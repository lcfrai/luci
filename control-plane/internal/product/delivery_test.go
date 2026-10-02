package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeliveryPushesExactCommitToRealBareRemote(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerDelivery(mux)
	p := createJourneyProject(t, mux, "delivery")
	if err := os.WriteFile(filepath.Join(p.Path, "README.md"), []byte("real Git delivery\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commit := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/commit", map[string]string{"message": "initial"}, 200)
	var committed struct {
		Commit string `json:"commit"`
	}
	json.Unmarshal(commit.Body.Bytes(), &committed)
	bare := filepath.Join(t.TempDir(), "remote.git")
	if _, err := RunCommand(context.Background(), "git", []string{"init", "--bare", bare}, ""); err != nil {
		t.Fatal(err)
	}
	const address = "https://example.com/owner/project.git"
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/remote", map[string]string{"url": address}, 200)
	networkCalls := 0
	// The production boundary accepts HTTPS only. This fixture translates that
	// reviewed URL after validation, so real Git proves push and ls-remote
	// without sending credentials or creating a GitHub repository.
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		args = append([]string{}, args...)
		for i, arg := range args {
			if arg == address {
				args[i] = bare
				networkCalls++
			}
		}
		if strings.Contains(strings.Join(args, " "), "protocol.https.allow=always") {
			args = append([]string{"-c", "protocol.file.allow=always"}, args...)
		}
		return RunCommand(ctx, exe, args, dir)
	}
	w := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/push", map[string]string{"commit": committed.Commit, "branch": "main"}, 200)
	var result GitPublication
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Verified || result.Commit != committed.Commit || result.Repository != address || result.Status != "pushed" || result.UpdatedAt == "" {
		t.Fatalf("unverified result: %+v", result)
	}
	remote, err := RunCommand(context.Background(), "git", []string{"--git-dir=" + bare, "rev-parse", "refs/heads/main"}, "")
	if err != nil || strings.TrimSpace(remote) != committed.Commit {
		t.Fatalf("remote=%q err=%v", remote, err)
	}
	if networkCalls < 2 {
		t.Fatal("push and remote verification were not exercised")
	}
	if err := os.WriteFile(filepath.Join(p.Path, "dirty.txt"), []byte("not committed"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/push", map[string]string{"commit": committed.Commit, "branch": "main"}, 409)
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/push", map[string]string{"commit": strings.Repeat("a", 40), "branch": "main"}, 409)
}

func TestDeliveryGitHubCreationPrivateDefaultAndExplicitPublicAcknowledgement(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerDelivery(mux)
	p := createJourneyProject(t, mux, "creation")
	created := false
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "gh" {
			return RunCommand(ctx, exe, args, dir)
		}
		command := strings.Join(args, " ")
		switch command {
		case "api --hostname github.com user --jq .login":
			return "owner\n", nil
		case "repo create owner/new-repo --private":
			created = true
			return "https://github.com/owner/new-repo", nil
		case "api --hostname github.com repos/owner/new-repo":
			return `{"full_name":"owner/new-repo","private":true,"clone_url":"https://github.com/owner/new-repo.git"}`, nil
		default:
			t.Fatalf("unexpected GitHub command %q", command)
			return "", nil
		}
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/github", map[string]any{"name": "new-repo", "visibility": "public"}, 400)
	if created {
		t.Fatal("public creation was attempted without acknowledgement")
	}
	w := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/github", map[string]string{"name": "new-repo"}, 201)
	if !created || !strings.Contains(w.Body.String(), `"visibility":"private"`) {
		t.Fatalf("private creation not proven: %s", w.Body.String())
	}
	origin, err := s.rawOrigin(context.Background(), p.Path)
	if err != nil || origin != "https://github.com/owner/new-repo.git" {
		t.Fatalf("origin=%q err=%v", origin, err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/github", map[string]string{"name": "new-repo"}, 409)
}

func TestDeliveryUnknownCreationIsNotBlindlyRetriedOrLeaked(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerDelivery(mux)
	p := createJourneyProject(t, mux, "unknown")
	creates := 0
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "gh" {
			return RunCommand(ctx, exe, args, dir)
		}
		creates++
		return "secret-token", errors.New("token=secret-token")
	}
	path := "/v1/product/projects/" + p.ID + "/git/github"
	w := journeyRequest(t, mux, "POST", path, map[string]string{"owner": "owner", "name": "uncertain"}, 422)
	if strings.Contains(w.Body.String(), "secret-token") {
		t.Fatal("raw command output leaked")
	}
	journeyRequest(t, mux, "POST", path, map[string]string{"owner": "owner", "name": "uncertain"}, 409)
	if creates != 1 {
		t.Fatalf("ambiguous creation repeated %d times", creates)
	}
}

func TestDeliveryRefusesCredentialsUnsafeOriginsAndLocalHelpers(t *testing.T) {
	for _, address := range []string{"file:///tmp/repo", "https://token@example.com/a.git", "https://example.com/a.git?secret=1", "https://example.com/%2e%2e/a", "https://example.com/a/../b", "https://example.com:443/a.git", "https://example.com/a\n.git", "ssh://example.com/a"} {
		if _, err := safeDeliveryURL(address); err == nil {
			t.Errorf("accepted %q", address)
		}
	}
	s, mux := journeyFixture(t)
	s.registerDelivery(mux)
	p := createJourneyProject(t, mux, "helpers")
	marker := filepath.Join(t.TempDir(), "helper-ran")
	if _, err := s.journeyGit(context.Background(), p.Path, "config", "credential.helper", "!echo secret > "+marker); err != nil {
		t.Fatal(err)
	}
	var calls []string
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		line := strings.Join(args, " ")
		calls = append(calls, line)
		if strings.Contains(line, "ls-remote") {
			if strings.Contains(line, "!echo secret") || !strings.Contains(line, "credential.helper=") {
				t.Fatalf("unsafe helpers in %s", line)
			}
			return "", nil
		}
		return RunCommand(ctx, exe, args, dir)
	}
	if _, err := s.deliveryNetworkGit(context.Background(), p.Path, "https://example.com/a.git", "ls-remote", "https://example.com/a.git"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("repository helper executed")
	}
	if len(calls) == 0 {
		t.Fatal("no command evidence")
	}
}

type githubFakeProcess struct{ run func() error }

func TestDeliveryPrivateDestinationNeedsLiveMatchingGitHubEvidence(t *testing.T) {
	s, _ := journeyFixture(t)
	for _, test := range []struct {
		address, output string
		want            bool
	}{
		{"https://github.com/owner/repo.git", `{"private":true,"clone_url":"https://github.com/owner/repo.git"}`, true},
		{"https://github.com/owner/repo.git", `{"private":false,"clone_url":"https://github.com/owner/repo.git"}`, false},
		{"https://github.com/owner/repo.git", `{"private":true,"clone_url":"https://github.com/owner/other.git"}`, false},
		{"https://example.com/owner/repo.git", `{"private":true}`, false},
	} {
		s.runner = func(context.Context, string, []string, string) (string, error) { return test.output, nil }
		if got := s.privateGitHubDestination(context.Background(), test.address); got != test.want {
			t.Errorf("address%s output%s: got%v", test.address, test.output, got)
		}
	}
}

func TestJourneyGitPinsWorktreeAgainstRepositoryConfigEscape(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "confined")
	if err := os.WriteFile(filepath.Join(p.Path, "account.txt"), []byte("project content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/commit", map[string]string{"message": "initial"}, 200)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "account.txt"), []byte("outside private secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"config", "core.worktree", outside}, {"config", "core.bare", "true"}} {
		if _, err := RunCommand(context.Background(), "git", args, p.Path); err != nil {
			t.Fatal(err)
		}
	}
	w := journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/git", nil, 200)
	if strings.Contains(w.Body.String(), "outside private secret") {
		t.Fatal("outside worktree content leaked")
	}
	var result GitSummary
	json.Unmarshal(w.Body.Bytes(), &result)
	if !result.Clean {
		t.Fatalf("trusted project should remain clean: %s", w.Body.String())
	}
	if err := os.WriteFile(filepath.Join(p.Path, "account.txt"), []byte("updated own content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/commit", map[string]string{"message": "own update"}, 200)
	got, err := s.journeyGit(context.Background(), p.Path, "show", "HEAD:account.txt")
	if err != nil || got != "updated own content\n" {
		t.Fatalf("Git staged outside workspace: %q %v", got, err)
	}
}

func (p githubFakeProcess) Run() error { return p.run() }
func TestGitHubLoginUsesOwnedCLIAndNeverReturnsRawOutput(t *testing.T) {
	s, _ := journeyFixture(t)
	release := make(chan struct{})
	done := make(chan struct{})
	job := s.startGitHubLogin(func(ctx context.Context, command ProviderLoginCommand, w io.Writer) (ProviderLoginProcess, error) {
		if command.Name != "gh" || strings.Join(command.Args, " ") != "auth login --hostname github.com --git-protocol https --web --skip-ssh-key" {
			t.Fatalf("command %+v", command)
		}
		return githubFakeProcess{run: func() error {
			defer close(done)
			io.WriteString(w, "secret-token\n! First copy your one-time code: ABCD-1234\nhttps://evil.example/token\n")
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}}, nil
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		current := *s.authJobs[job.ID]
		s.mu.RUnlock()
		if current.Status == "waiting" {
			if current.UserCode != "ABCD-1234" || current.VerificationURL != "https://github.com/login/device" {
				t.Fatalf("instructions %+v", current)
			}
			break
		}
		time.Sleep(time.Millisecond)
	}
	duplicate := s.startGitHubLogin(func(context.Context, ProviderLoginCommand, io.Writer) (ProviderLoginProcess, error) {
		t.Fatal("duplicate process started")
		return nil, nil
	})
	if duplicate.ID != job.ID {
		t.Fatal("duplicate login not reused")
	}
	close(release)
	<-done
	mux := http.NewServeMux()
	s.registerDelivery(mux)
	w := journeyRequest(t, mux, "GET", "/v1/setup/github/login", nil, 200)
	if strings.Contains(w.Body.String(), "secret-token") || strings.Contains(w.Body.String(), "evil.example") {
		t.Fatal("CLI output leaked")
	}
}
