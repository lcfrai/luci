package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This explicit fixture helper follows the public review-before-commit contract;
// raw journeyRequest stays available to test missing and stale identities.
func reviewedCommitRequest(t *testing.T, handler http.Handler, commitRoute, message string) map[string]string {
	t.Helper()
	var review GitSummary
	response := journeyRequest(t, handler, "GET", strings.TrimSuffix(commitRoute, "/commit"), nil, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	if !hexDigest(review.ReviewDigest) {
		t.Fatal("Review has no bounded source identity")
	}
	return map[string]string{"message": message, "expectedReviewDigest": review.ReviewDigest}
}

func readReview(t *testing.T, handler http.Handler, base string) GitSummary {
	t.Helper()
	var review GitSummary
	if err := json.Unmarshal(journeyRequest(t, handler, "GET", base+"/git", nil, 200).Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	return review
}

func TestJourneyGitSkipsFilterProbeOnlyForExactMetadataReads(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "metadata-probe")
	original := s.runner
	probes := 0
	s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
		if executable == "git" && strings.Contains(strings.Join(args, "\x00"), "--get-regexp\x00^filter\\.") {
			probes++
		}
		return original(ctx, executable, args, dir)
	}
	for _, command := range [][]string{{"symbolic-ref", "--quiet", "HEAD"}, {"rev-parse", "--show-object-format"}, {"ls-files", "--stage", "-z"}} {
		if _, err := s.journeyGit(context.Background(), p.Path, command...); err != nil {
			t.Fatal(command, err)
		}
	}
	if probes != 0 {
		t.Fatal("metadata-only reads launched a filter-config Git child", probes)
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all"); err != nil || probes != 1 {
		t.Fatal("worktree status did not keep its filter guard", err, probes)
	}
	if journeyGitMetadataOnly([]string{"ls-files", "--eol"}) || journeyGitMetadataOnly([]string{"diff", "--no-ext-diff"}) {
		t.Fatal("unreviewed worktree command bypassed filter guard")
	}
}

func TestReviewedCommitRequiresIdentityAndRejectsEveryUnseenInputBeforeStaging(t *testing.T) {
	for _, mutation := range []string{"missing", "source", "untracked", "context", "index", "branch", "head", "delete", "mode"} {
		t.Run(mutation, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "review-"+mutation)
			base := "/v1/product/projects/" + p.ID
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial application\n"), 0600)
			initial := reviewedCommitRequest(t, mux, base+"/git/commit", "Initial reviewed source")
			journeyRequest(t, mux, "POST", base+"/git/commit", initial, 200)
			file := filepath.Join(p.Path, "README.md")
			if err := os.WriteFile(file, []byte("Reviewed application\n"), 0600); err != nil {
				t.Fatal(err)
			}
			req := reviewedCommitRequest(t, mux, base+"/git/commit", "Message to preserve")
			switch mutation {
			case "missing":
				delete(req, "expectedReviewDigest")
			case "source":
				os.WriteFile(file, []byte("Unseen application!\n"), 0600)
			case "untracked":
				os.WriteFile(filepath.Join(p.Path, "unseen.txt"), []byte("Unseen new feature\n"), 0600)
			case "context":
				journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Unseen context amendment", "audience": "Teams", "constraints": "", "criteria": []string{}}, 200)
			case "index":
				confinedFixtureGit(t, p.Path, "add", "README.md")
			case "branch":
				confinedFixtureGit(t, p.Path, "checkout", "-b", "another-review")
			case "head":
				tree := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD^{tree}")
				old := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
				next := confinedFixtureGit(t, p.Path, "-c", "user.name=Other person", "-c", "user.email=other@example.test", "commit-tree", tree, "-p", old, "-m", "Other parent")
				confinedFixtureGit(t, p.Path, "update-ref", "HEAD", next, old)
			case "delete":
				os.Remove(file)
			case "mode":
				if err := os.Chmod(file, 0700); err != nil {
					t.Fatal(err)
				}
			}
			indexBefore, err := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			if err != nil {
				t.Fatal(err)
			}
			headBefore := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
			writes := 0
			s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
				for _, arg := range args {
					if arg == "add" || arg == "read-tree" || arg == "commit-tree" || arg == "update-ref" {
						writes++
					}
				}
				return RunCommand(ctx, exe, args, dir)
			}
			want := 409
			if mutation == "missing" {
				want = 400
			}
			response := journeyRequest(t, mux, "POST", base+"/git/commit", req, want)
			expectedCode := "source_changed"
			if mutation == "missing" {
				expectedCode = "review_required"
			}
			if !strings.Contains(response.Body.String(), expectedCode) {
				t.Fatal(response.Body.String())
			}
			indexAfter, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			if writes != 0 || !bytes.Equal(indexBefore, indexAfter) || confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") != headBefore {
				t.Fatal("stale review mutated Git")
			}
		})
	}
}

func TestReviewedCommitCapturesExactFilesAndLeavesConcurrentEditsUncommitted(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "captured-review")
	base := "/v1/product/projects/" + p.ID
	file := filepath.Join(p.Path, "README.md")
	os.WriteFile(file, []byte("Exactly reviewed\n"), 0600)
	req := reviewedCommitRequest(t, mux, base+"/git/commit", "Preserve exact reviewed source")
	changed := false
	privateDir := ""
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if selected, ok := ctx.Value(privateReviewGitKey{}).(privateReviewGit); ok {
			privateDir = filepath.Dir(selected.GitDir)
			if !changed {
				changed = true
				os.WriteFile(file, []byte("Newer unsent source\n"), 0600)
				os.WriteFile(filepath.Join(p.Path, "later.txt"), []byte("Later file\n"), 0600)
				// Normal branch changes cannot race through the original HEAD lock.
				if _, err := RunCommand(context.Background(), "git", []string{"symbolic-ref", "HEAD", "refs/heads/other"}, p.Path); err == nil {
					t.Error("branch movement bypassed owned HEAD lock")
				}
			}
		}
		return RunCommand(ctx, exe, args, dir)
	}
	response := journeyRequest(t, mux, "POST", base+"/git/commit", req, 200)
	var result struct {
		Commit        string
		SourceChanged bool
	}
	json.Unmarshal(response.Body.Bytes(), &result)
	if !changed || !result.SourceChanged {
		t.Fatal("concurrent edits were not reported", response.Body.String())
	}
	if got := confinedFixtureGit(t, p.Path, "show", result.Commit+":README.md"); got != "Exactly reviewed" {
		t.Fatal("swept newer content into commit", got)
	}
	if _, err := RunCommand(context.Background(), "git", []string{"show", result.Commit + ":later.txt"}, p.Path); err == nil {
		t.Fatal("newer file entered reviewed commit")
	}
	if got := confinedFixtureGit(t, p.Path, "diff", "--cached", "--name-only"); got != "" {
		t.Fatal("original index disagrees with published commit", got)
	}
	if got := confinedFixtureGit(t, p.Path, "status", "--porcelain"); !strings.Contains(got, "README.md") || !strings.Contains(got, "later.txt") {
		t.Fatal("newer edits lost", got)
	}
	for _, path := range []string{privateDir, filepath.Join(p.Path, ".git", "index.lock"), filepath.Join(p.Path, ".git", "HEAD.lock")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("owned temporary survived", path, err)
		}
	}
}

func TestReviewedCommitRefConflictAndStageFailurePreserveOriginalIndex(t *testing.T) {
	for _, failure := range []string{"stage", "ref"} {
		t.Run(failure, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "failure-"+failure)
			base := "/v1/product/projects/" + p.ID
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial application\n"), 0600)
			journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Initial"), 200)
			old := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Staged before review\n"), 0600)
			confinedFixtureGit(t, p.Path, "add", "README.md")
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed final content\n"), 0600)
			req := reviewedCommitRequest(t, mux, base+"/git/commit", "Retained failure message")
			before, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			other := ""
			privateDir := ""
			s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
				if selected, ok := ctx.Value(privateReviewGitKey{}).(privateReviewGit); ok {
					privateDir = filepath.Dir(selected.GitDir)
					for i, arg := range args {
						if arg == "add" && failure == "stage" {
							return "", errors.New("fixture stage interruption")
						}
						if arg == "update-ref" && failure == "ref" && other == "" && selected.ObjectsDir == "" {
							prefix := append([]string{}, args[:i]...)
							tree, e := RunCommand(ctx, exe, append(prefix, "rev-parse", old+"^{tree}"), dir)
							if e != nil {
								t.Fatal(e)
							}
							create := append(append([]string{}, prefix...), "-c", "user.name=Other worktree", "-c", "user.email=other@example.test", "commit-tree", strings.TrimSpace(tree), "-p", old, "-m", "Concurrent branch writer")
							other, e = RunCommand(ctx, exe, create, dir)
							if e != nil {
								t.Fatal(e)
							}
							other = strings.TrimSpace(other)
							if _, e := RunCommand(ctx, exe, append(prefix, "update-ref", "refs/heads/main", other, old), dir); e != nil {
								t.Fatal(e)
							}
						}
					}
				}
				return RunCommand(ctx, exe, args, dir)
			}
			want := 422
			if failure == "ref" {
				want = 409
			}
			journeyRequest(t, mux, "POST", base+"/git/commit", req, want)
			after, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			if !bytes.Equal(before, after) {
				t.Fatal("original staging changed on refusal")
			}
			expected := old
			if other != "" {
				expected = other
			}
			if confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") != expected {
				t.Fatal("refusal overwrote parent")
			}
			for _, path := range []string{privateDir, filepath.Join(p.Path, ".git", "index.lock"), filepath.Join(p.Path, ".git", "HEAD.lock")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("owned temporary survived", path, err)
				}
			}
		})
	}
}

func TestReviewedCommitExternalLocksAndDetachedHeadGiveRecoveryWithoutWrites(t *testing.T) {
	for _, condition := range []string{"index.lock", "HEAD.lock", "detached"} {
		t.Run(condition, func(t *testing.T) {
			_, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "busy-review")
			base := "/v1/product/projects/" + p.ID
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial application\n"), 0600)
			journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Initial"), 200)
			if condition == "detached" {
				confinedFixtureGit(t, p.Path, "checkout", "--detach")
			}
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed edit\n"), 0600)
			req := reviewedCommitRequest(t, mux, base+"/git/commit", "Retained busy message")
			before, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			head := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
			if condition != "detached" {
				os.WriteFile(filepath.Join(p.Path, ".git", condition), []byte("external owner"), 0600)
			}
			response := journeyRequest(t, mux, "POST", base+"/git/commit", req, 409)
			code := "git_busy"
			if condition == "detached" {
				code = "branch_required"
			}
			if !strings.Contains(response.Body.String(), code) {
				t.Fatal(response.Body.String())
			}
			after, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			if !bytes.Equal(before, after) || head != confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") {
				t.Fatal("busy refusal changed source")
			}
			if condition != "detached" {
				lock, _ := os.ReadFile(filepath.Join(p.Path, ".git", condition))
				if string(lock) != "external owner" {
					t.Fatal("removed another Git operation's lock")
				}
			}
		})
	}
}

func TestReviewedCommitPreservesUnchangedProtectedTrackedFiles(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "protected-existing")
	base := "/v1/product/projects/" + p.ID
	os.WriteFile(filepath.Join(p.Path, ".env"), []byte("FIXTURE_PRIVATE_VALUE=unchanged\n"), 0600)
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial\n"), 0600)
	// Native Git intentionally owns the existing protected file; the Luci
	// one-click action must neither expose it nor silently delete it.
	confinedFixtureGit(t, p.Path, "add", "--all")
	confinedFixtureGit(t, p.Path, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "Existing private file")
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed improvement\n"), 0600)
	review := readReview(t, mux, base)
	if strings.Contains(review.Diff, "FIXTURE_PRIVATE_VALUE") {
		t.Fatal("review exposed protected bytes")
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", map[string]string{"message": "Improve source", "expectedReviewDigest": review.ReviewDigest}, 200)
	if got := confinedFixtureGit(t, p.Path, "show", "HEAD:.env"); got != "FIXTURE_PRIVATE_VALUE=unchanged" {
		t.Fatal("unchanged protected file was removed", got)
	}
	if got := confinedFixtureGit(t, p.Path, "diff", "HEAD^", "HEAD", "--name-only"); got != "README.md" {
		t.Fatal("commit changed an unreviewed path", got)
	}
}

func TestReviewedCommitFreezesGitNormalizationAndIgnoresInheritedRedirection(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "frozen-attributes")
	base := "/v1/product/projects/" + p.ID
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed CRLF\r\n"), 0600)
	os.MkdirAll(filepath.Join(p.Path, ".git", "info"), 0700)
	os.WriteFile(filepath.Join(p.Path, ".git", "info", "attributes"), []byte("*.md -text\n"), 0600)
	req := reviewedCommitRequest(t, mux, base+"/git/commit", "Frozen transform inputs")
	foreign := ownedFixtureTempDir(t)
	confinedFixtureGit(t, foreign, "init", "-b", "main")
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreign, "redirect-index"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(foreign, ".git", "objects"))
	before := auditTree(t, foreign)
	changed := false
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if selected, ok := ctx.Value(privateReviewGitKey{}).(privateReviewGit); ok && selected.ObjectsDir != "" && !changed {
			changed = true
			os.WriteFile(filepath.Join(p.Path, ".git", "info", "attributes"), []byte("*.md text eol=lf\n"), 0600)
			// A native configuration edit after capture cannot reinterpret this copy.
			confinedFixtureGit(t, p.Path, "config", "core.autocrlf", "true")
		}
		return RunCommand(ctx, exe, args, dir)
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", req, 200)
	out, err := runCommandEnvironment(context.Background(), "git", []string{"show", "HEAD:README.md"}, p.Path, journeyGitEnvironment(context.WithValue(context.Background(), managedJourneyGitContextKey{}, true), os.Environ()))
	if err != nil || out != "Reviewed CRLF\r\n" {
		t.Fatal("mutable live attributes changed reviewed bytes", out, err)
	}
	if !reflect.DeepEqual(before, auditTree(t, foreign)) {
		t.Fatal("inherited Git environment changed foreign repository")
	}
}

func TestReviewedCommitSupportsSHA256ParentsAndLiteralPaths(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "sha256-review")
	base := "/v1/product/projects/" + p.ID
	// Replace only this empty fixture's Git store before any reviewed commit.
	if err := os.RemoveAll(filepath.Join(p.Path, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := RunCommand(context.Background(), "git", []string{"init", "--object-format=sha256", "--initial-branch=main"}, p.Path); err != nil {
		t.Skip("installed Git lacks SHA-256 repositories")
	}
	names := []string{"README.md", "[literal].txt", "--option.txt", "space name.txt"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(p.Path, name), []byte("First "+name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "First SHA-256 commit"), 200)
	first := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
	if len(first) != 64 {
		t.Fatal(first)
	}
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Second\n"), 0600)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Second SHA-256 commit"), 200)
	if parent := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD^"); parent != first {
		t.Fatal("wrong reviewed SHA-256 parent", parent)
	}
	if status := confinedFixtureGit(t, p.Path, "status", "--porcelain"); status != "" {
		t.Fatal("wrong final index", status)
	}
}

func TestReviewedCommitDoesNotRemoveNextWritersIndexLock(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "index-handoff")
	base := "/v1/product/projects/" + p.ID
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed\n"), 0600)
	req := reviewedCommitRequest(t, mux, base+"/git/commit", "Publish owned index")
	published := false
	nextLock := false
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		for _, arg := range args {
			if selected, ok := ctx.Value(privateReviewGitKey{}).(privateReviewGit); arg == "update-ref" && ok && selected.ObjectsDir == "" {
				out, err := RunCommand(ctx, exe, args, dir)
				published = err == nil
				return out, err
			}
			if arg == "status" && published && !nextLock {
				if _, private := ctx.Value(privateReviewGitKey{}).(privateReviewGit); !private {
					file, err := os.OpenFile(filepath.Join(p.Path, ".git", "index.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
					if err != nil {
						t.Fatal(err)
					}
					file.WriteString("next writer owns this lock")
					file.Close()
					nextLock = true
				}
			}
		}
		return RunCommand(ctx, exe, args, dir)
	}
	journeyRequest(t, mux, "POST", base+"/git/commit", req, 200)
	data, err := os.ReadFile(filepath.Join(p.Path, ".git", "index.lock"))
	if err != nil || string(data) != "next writer owns this lock" {
		t.Fatal("commit removed a subsequent writer's lock", err)
	}
}

func TestGitReviewDisplaysUntrackedTextAndPortableContextWithoutCredentialBytes(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "review-visible-files")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "Review shared project meaning", "audience": "Teams", "criteria": []string{}}, 200)
	os.MkdirAll(filepath.Join(p.Path, "new-folder"), 0700)
	os.WriteFile(filepath.Join(p.Path, "new-folder", "new.txt"), []byte("Visible new feature\n"), 0600)
	os.WriteFile(filepath.Join(p.Path, ".env"), []byte("FIXTURE_MUST_NOT_APPEAR=private\n"), 0600)
	os.WriteFile(filepath.Join(p.Path, "binary.dat"), []byte{0, 1, 2}, 0600)
	review := readReview(t, mux, base)
	for _, want := range []string{"new-folder/new.txt", "Visible new feature", ".luci/state.json", "Review shared project meaning", "[Binary file: contents are not displayed]"} {
		if !strings.Contains(review.Diff, want) {
			t.Fatal("review omitted inspectable input", want)
		}
	}
	if strings.Contains(review.Diff, "FIXTURE_MUST_NOT_APPEAR") {
		t.Fatal("credential bytes exposed")
	}
	for _, change := range review.Changes {
		if change.Path == "new-folder/" {
			t.Fatal("untracked files collapsed to unseen directory")
		}
	}
}

func TestGitReviewRefusesConcurrentDisplayChangeAndOversizeFiles(t *testing.T) {
	for _, kind := range []string{"concurrent", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "bounded-review")
			base := "/v1/product/projects/" + p.ID
			file := filepath.Join(p.Path, "README.md")
			os.WriteFile(file, []byte("Initial\n"), 0600)
			if kind == "oversize" {
				f, err := os.OpenFile(file, os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if err = f.Truncate((100 << 20) + 1); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			changed := false
			s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
				if kind == "concurrent" && !changed {
					for _, arg := range args {
						if arg == "diff" {
							changed = true
							os.WriteFile(file, []byte("Changed during rendered review\n"), 0600)
						}
					}
				}
				return RunCommand(ctx, exe, args, dir)
			}
			result := journeyRequest(t, mux, "GET", base+"/git", nil, 422)
			if strings.Contains(result.Body.String(), "reviewDigest") {
				t.Fatal("incoherent or oversized review received commit identity")
			}
		})
	}
}

func TestReviewedCommitNormalizesSplitIndexAndPreservesNativeRecoveryHistory(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "split-index-review")
	base := "/v1/product/projects/" + p.ID
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial\n"), 0600)
	firstReq := reviewedCommitRequest(t, mux, base+"/git/commit", "Initial reviewed source")
	journeyRequest(t, mux, "POST", base+"/git/commit", firstReq, 200)
	first := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
	confinedFixtureGit(t, p.Path, "config", "core.splitIndex", "true")
	confinedFixtureGit(t, p.Path, "update-index", "--split-index")
	if shared := confinedFixtureGit(t, p.Path, "rev-parse", "--shared-index-path"); shared == "" {
		t.Fatal("fixture did not create a shared index")
	}
	confinedFixtureGit(t, p.Path, "config", "user.name", "Untrusted local identity")
	confinedFixtureGit(t, p.Path, "config", "user.email", "untrusted@example.test")
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed next source\n"), 0600)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Reviewed recovery entry"), 200)
	second := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
	if got := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD@{1}"); got != first {
		t.Fatal("HEAD recovery lost previous reviewed commit", got)
	}
	if got := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD@{0}"); got != second {
		t.Fatal("HEAD recovery omitted new reviewed commit", got)
	}
	for _, ref := range []string{"HEAD", "main"} {
		log := confinedFixtureGit(t, p.Path, "reflog", "show", "-1", "--format=%gn <%ge> %gs", ref)
		if log != "Luci Test Person <person@example.com> commit: Reviewed recovery entry" {
			t.Fatal("Git recovery used repository's spoof identity", ref, log)
		}
	}
	if shared := confinedFixtureGit(t, p.Path, "rev-parse", "--shared-index-path"); shared != "" && !strings.HasSuffix(shared, "sharedindex."+strings.Repeat("0", 40)) {
		t.Fatal("published index depends on deleted private shared index", shared)
	}
	if status := confinedFixtureGit(t, p.Path, "status", "--porcelain"); status != "" {
		t.Fatal("split index normalization changed source", status)
	}
	readReview(t, mux, base)
}

func TestGitReviewRefusesHiddenIndexFlagsWithRecoveryGuidance(t *testing.T) {
	for _, flag := range []string{"--skip-worktree", "--assume-unchanged"} {
		t.Run(flag, func(t *testing.T) {
			_, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "index-flags")
			base := "/v1/product/projects/" + p.ID
			os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial\n"), 0600)
			journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Initial"), 200)
			confinedFixtureGit(t, p.Path, "update-index", flag, "README.md")
			if flag == "--skip-worktree" {
				os.Remove(filepath.Join(p.Path, "README.md"))
			} else {
				os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Hidden change\n"), 0600)
			}
			before, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			head := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
			response := journeyRequest(t, mux, "GET", base+"/git", nil, 422)
			if !strings.Contains(response.Body.String(), "Clear Git skip-worktree and assume-unchanged flags") {
				t.Fatal("missing actionable recovery", response.Body.String())
			}
			after, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
			if !bytes.Equal(before, after) || head != confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") {
				t.Fatal("read normalized or deleted hidden source")
			}
		})
	}
}

func TestReviewedCommitReflogLockRefusalPreservesSourceAndHistory(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "reflog-locked")
	base := "/v1/product/projects/" + p.ID
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Initial\n"), 0600)
	journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Initial"), 200)
	os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("Reviewed later\n"), 0600)
	req := reviewedCommitRequest(t, mux, base+"/git/commit", "Message stays")
	before := confinedFixtureGit(t, p.Path, "rev-parse", "HEAD")
	index, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
	log, _ := os.ReadFile(filepath.Join(p.Path, ".git", "logs", "HEAD"))
	lock := filepath.Join(p.Path, ".git", "logs", "HEAD.lock")
	os.WriteFile(lock, []byte("another history writer"), 0600)
	response := journeyRequest(t, mux, "POST", base+"/git/commit", req, 422)
	if !strings.Contains(response.Body.String(), "reflog_unavailable") {
		t.Fatal(response.Body.String())
	}
	afterIndex, _ := os.ReadFile(filepath.Join(p.Path, ".git", "index"))
	afterLog, _ := os.ReadFile(filepath.Join(p.Path, ".git", "logs", "HEAD"))
	afterLock, _ := os.ReadFile(lock)
	if before != confinedFixtureGit(t, p.Path, "rev-parse", "HEAD") || !bytes.Equal(index, afterIndex) || !bytes.Equal(log, afterLog) || string(afterLock) != "another history writer" {
		t.Fatal("reflog lock refusal changed owned history")
	}
}
