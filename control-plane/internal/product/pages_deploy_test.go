package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type pagesFixture struct {
	site                            *pagesSite
	commit                          string
	private                         bool
	creates, builds, deletes, calls int
	fail                            bool
}

func pagesResponse(status int, value any) (string, error) {
	data, _ := json.Marshal(value)
	out := fmt.Sprintf("HTTP/2.0 %d Test\r\nContent-Type: application/json\r\n\r\n%s", status, data)
	if status >= 400 {
		return out, errors.New("provider detail must never leak")
	}
	return out, nil
}
func pagesFixtureServer(t *testing.T) (*Server, *http.ServeMux, project.Project, PagesReleaseRequest, *pagesFixture) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.registerPagesReleases(mux)
	s.registerCloudReleases(mux)
	p := createJourneyProject(t, mux, "static-demo")
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/brief", map[string]any{"revision": 0, "purpose": "Public static demo", "criteria": []string{"Site works over HTTPS"}}, 200)
	if err := os.Mkdir(filepath.Join(p.Path, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"Dockerfile": "FROM nginx:alpine\nCOPY docs /usr/share/nginx/html\n", "docs/index.html": "<!doctype html><title>Real static demo</title>\n", "docs/.nojekyll": ""} {
		if err := os.WriteFile(filepath.Join(p.Path, filepath.FromSlash(name)), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	commitPreviewFixture(t, p.Path)
	head, _ := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if _, err := s.journeyGit(context.Background(), p.Path, "remote", "add", "origin", "https://github.com/example/static-demo.git"); err != nil {
		t.Fatal(err)
	}
	f := &pagesFixture{commit: strings.TrimSpace(head)}
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "gh" {
			return RunCommand(ctx, exe, args, dir)
		}
		f.calls++
		if f.fail {
			return "", errors.New("ghp_PRIVATE_FAKE_DETAIL")
		}
		method, endpoint := "", ""
		fields := map[string]string{}
		for i, arg := range args {
			if arg == "--method" {
				method = args[i+1]
			}
			if strings.HasPrefix(arg, "repos/") {
				endpoint = arg
			}
			if arg == "-f" {
				k, v, _ := strings.Cut(args[i+1], "=")
				fields[k] = v
			}
		}
		switch method + " " + endpoint {
		case "GET repos/example/static-demo":
			return pagesResponse(200, map[string]any{"full_name": "example/static-demo", "private": f.private, "default_branch": "main"})
		case "GET repos/example/static-demo/git/ref/heads/main":
			return pagesResponse(200, map[string]any{"object": map[string]string{"sha": f.commit}})
		case "GET repos/example/static-demo/pages":
			if f.site == nil {
				return pagesResponse(404, map[string]string{"message": "Not Found"})
			}
			return pagesResponse(200, f.site)
		case "POST repos/example/static-demo/pages":
			if fields["build_type"] != "legacy" || fields["source[branch]"] != "main" || fields["source[path]"] != "/docs" {
				t.Errorf("wrong publishing source: %+v", fields)
			}
			f.creates++
			f.site = &pagesSite{HTMLURL: pagesAddress("example/static-demo"), BuildType: "legacy"}
			f.site.Source.Branch = "main"
			f.site.Source.Path = "/docs"
			return pagesResponse(201, f.site)
		case "POST repos/example/static-demo/pages/builds":
			f.builds++
			return pagesResponse(201, map[string]string{"status": "queued"})
		case "GET repos/example/static-demo/pages/builds/latest":
			return pagesResponse(200, pagesBuild{URL: "https://api.github.com/repos/example/static-demo/pages/builds/123", Commit: f.commit, Status: "queued"})
		case "DELETE repos/example/static-demo/pages":
			f.deletes++
			f.site = nil
			return pagesResponse(204, nil)
		default:
			t.Errorf("unexpected GitHub operation %s %s", method, endpoint)
			return "", errors.New("unexpected request")
		}
	}
	return s, mux, p, PagesReleaseRequest{Commit: f.commit, Branch: "main", AcknowledgePublic: true}, f
}
func TestPagesDeploymentPinsPublicCommitPersistsAndStopsOnlyOwnedSite(t *testing.T) {
	s, mux, p, req, f := pagesFixtureServer(t)
	base := "/v1/product/projects/" + p.ID + "/releases"
	var release Release
	created := journeyRequest(t, mux, "POST", base+"/pages", req, 202)
	if err := json.Unmarshal(created.Body.Bytes(), &release); err != nil {
		t.Fatal(err)
	}
	if release.Status != "queued" || release.Commit != req.Commit || release.BriefRevision != 1 || release.Target != "github-pages" || release.URL != "" {
		t.Fatalf("premature or unpinned release: %+v", release)
	}
	duplicate := journeyRequest(t, mux, "POST", base+"/pages", req, 202)
	if !strings.Contains(duplicate.Body.String(), release.ID) || f.creates != 1 || f.builds != 1 {
		t.Fatal("repeated request duplicated provider operations")
	}
	journeyRequest(t, mux, "POST", base+"/"+release.ID+"/refresh", nil, 200)
	rows, err := s.readPagesReleases(p.ID)
	if err != nil || len(rows) != 1 || !rows[0].OwnsConfiguration {
		t.Fatalf("missing ownership receipt: %+v %v", rows, err)
	}
	if strings.Contains(created.Body.String(), "indexSha256") || strings.Contains(created.Body.String(), "ownsConfiguration") {
		t.Fatal("internal receipt exposed")
	}
	if _, err = os.Stat(filepath.Join(p.Path, "pages-releases")); !os.IsNotExist(err) {
		t.Fatal("deployment receipt entered project")
	}
	if listed := journeyRequest(t, mux, "GET", base, nil, 200); !strings.Contains(listed.Body.String(), release.ID) {
		t.Fatal("Pages release missing from shared history")
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	saved, err := restarted.pagesReleases(p.ID)
	if err != nil || len(saved) != 1 || saved[0].Commit != req.Commit {
		t.Fatal("release provenance did not survive restart")
	}
	f.site.CNAME = "someone-else.example"
	journeyRequest(t, mux, "POST", base+"/"+release.ID+"/stop", nil, 409)
	if f.deletes != 0 {
		t.Fatal("changed site deleted")
	}
	f.site.CNAME = ""
	stopped := journeyRequest(t, mux, "POST", base+"/"+release.ID+"/stop", nil, 200)
	if !strings.Contains(stopped.Body.String(), `"status":"stopped"`) || f.deletes != 1 {
		t.Fatal("owned site not confirmed unpublished")
	}
	journeyRequest(t, mux, "POST", base+"/"+release.ID+"/stop", nil, 200)
	if f.deletes != 1 {
		t.Fatal("repeated stop deleted again")
	}
}
func TestPagesDeploymentRejectsPrivateWrongCommitDirtyAndUnownedConfiguration(t *testing.T) {
	s, mux, p, req, f := pagesFixtureServer(t)
	path := "/v1/product/projects/" + p.ID + "/releases/pages"
	noAck := req
	noAck.AcknowledgePublic = false
	journeyRequest(t, mux, "POST", path, noAck, 400)
	journeyRequest(t, workspaceAdminActor(mux, "super-admin"), "POST", path, req, 403)
	if f.calls != 0 {
		t.Fatal("provider called before authorization")
	}
	f.private = true
	journeyRequest(t, mux, "POST", path, req, 409)
	f.private = false
	f.commit = strings.Repeat("a", 40)
	journeyRequest(t, mux, "POST", path, req, 409)
	f.commit = req.Commit
	if err := os.WriteFile(filepath.Join(p.Path, "dirty.txt"), []byte("unreviewed"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", path, req, 409)
	if err := os.Remove(filepath.Join(p.Path, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
	f.site = &pagesSite{HTMLURL: pagesAddress("example/static-demo")}
	f.site.Source.Branch = "main"
	f.site.Source.Path = "/docs"
	journeyRequest(t, mux, "POST", path, req, 409)
	if f.creates != 0 || f.builds != 0 || f.deletes != 0 {
		t.Fatal("rejected request changed GitHub")
	}
	f.site = nil
	f.fail = true
	response := journeyRequest(t, mux, "POST", path, req, 409)
	if strings.Contains(response.Body.String(), "PRIVATE_FAKE") {
		t.Fatal("provider error secret leaked")
	}
	_ = s
}
func TestPagesDeploymentRequiresCommittedSafeStaticOutput(t *testing.T) {
	s, mux, p, req, f := pagesFixtureServer(t)
	if err := os.WriteFile(filepath.Join(p.Path, "docs", ".env"), []byte("PRIVATE=should-not-publish\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	head, _ := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	req.Commit = strings.TrimSpace(head)
	f.commit = req.Commit
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/releases/pages", req, 409)
	if f.calls != 0 {
		t.Fatal("invalid static tree reached GitHub")
	}
}
func TestPagesBuildRequiresExactCommitAndRealHTTPSContent(t *testing.T) {
	content := []byte("<!doctype html><title>Published benchmark</title>\n")
	sum := sha256.Sum256(content)
	row := pagesRelease{Release: Release{Commit: strings.Repeat("a", 40)}, Repository: "example/static-demo", SiteURL: pagesAddress("example/static-demo"), IndexSHA256: hex.EncodeToString(sum[:])}
	build := pagesBuild{URL: "https://api.github.com/repos/example/static-demo/pages/builds/7", Status: "built", Commit: row.Commit}
	health := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("credential reached public site")
		}
		_, _ = w.Write(content)
	}))
	defer health.Close()
	probe := func(ctx context.Context, address, digest string) error {
		if address != row.SiteURL {
			t.Error("site binding lost")
		}
		return probePagesIndex(ctx, health.URL, digest, health.Client())
	}
	ready := observePagesBuild(context.Background(), row, build, probe)
	if ready.Status != "ready" || ready.VerifiedAt == "" || ready.URL != row.SiteURL {
		t.Fatalf("real HTTPS content not verified: %+v", ready)
	}
	build.Commit = strings.Repeat("b", 40)
	called := false
	mismatch := observePagesBuild(context.Background(), row, build, func(context.Context, string, string) error { called = true; return nil })
	if called || mismatch.Status != "attention" || mismatch.URL != "" {
		t.Fatal("wrong commit accepted")
	}
	build.Commit = row.Commit
	stale := observePagesBuild(context.Background(), row, build, func(ctx context.Context, _ string, _ string) error {
		return probePagesIndex(ctx, health.URL, strings.Repeat("0", 64), health.Client())
	})
	if stale.Status != "verifying" || stale.URL != "" {
		t.Fatal("stale CDN content accepted")
	}
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, health.URL, 302) }))
	defer redirect.Close()
	if probePagesIndex(context.Background(), redirect.URL, row.IndexSHA256, redirect.Client()) == nil {
		t.Fatal("redirect accepted")
	}
	for _, address := range []string{"http://example.github.io/static-demo/", "https://127.0.0.1/", "https://example.github.io@localhost/", "https://example.github.io/static-demo/?token=x", "https://example.github.io:444/", "https://example.github.io/../private/", "https://example.github.io/static-demo/%2fadmin"} {
		if verifyPublicPagesIndex(context.Background(), address, row.IndexSHA256) == nil {
			t.Fatalf("unsafe URL accepted: %s", address)
		}
	}
}
