package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// The local side, in the smallest shape that makes the route real: one project
// with one origin. The key spelling is phase 3's job, so this test's parser is
// the identity - what the route is asked to prove is the WIRE.
type bindLocals struct{ path, origin string }

func (b bindLocals) Projects(context.Context) ([]project.Project, error) {
	return []project.Project{{ID: "p-8c1d0e42", Name: "portal", Path: b.path}}, nil
}

func (b bindLocals) OriginURL(context.Context, string) (string, error) { return b.origin, nil }

func bindResolver(t *testing.T) *binding.Resolver {
	t.Helper()
	local := bindLocals{path: "/w/portal", origin: "github.com/lcfrai/panaceaapp"}
	fake := deploytest.NewFake(func() time.Time { return time.Unix(1700000000, 0).UTC() })
	for _, ref := range []string{"portal-dev", "portal-uat"} {
		fake.AddProject(deploy.Project{Ref: deploy.ProjectRef(ref), DisplayName: ref, RepoURL: "github.com/lcfrai/panaceaapp"})
	}
	r, err := binding.New(binding.Config{
		Projects:      local,
		Remotes:       local,
		Deployer:      fake,
		ParseRemote:   func(u string) (binding.Remote, error) { return binding.Remote{RepoKey: u}, nil },
		PanaceaOrigin: func() string { return connTestOrigin },
		ChoicesDir:    t.TempDir(),
		Log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("binding.New: %v", err)
	}
	return r
}

func bindAPI(t *testing.T, r *binding.Resolver) http.Handler {
	t.Helper()
	return catalogNewAPI(t, func(d *api.Deps) { d.DeployBinding = r })
}

// The mutant: a free-text binding that points Ship at any project name, and an
// unknown local project answered as a deploy failure rather than a 404.
func TestDeployBindingChooseAndRefuse(t *testing.T) {
	h := bindAPI(t, bindResolver(t))
	const path = "/v1/projects/p-8c1d0e42/deploy/binding"

	rec := catalogDo(t, h, "PUT", path, `{"panaceaProject":"portal-uat"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d, body %s", rec.Code, rec.Body.String())
	}
	body := catalogDecodeObject(t, rec.Body.String())
	if body["state"] != "bound" || body["panaceaProject"] != "portal-uat" || body["chosenBy"] != "local" {
		t.Errorf("body = %s", rec.Body.String())
	}

	rec = catalogDo(t, h, "PUT", path, `{"panaceaProject":"portal-anything"}`, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("outside the candidates = %d, body %s", rec.Code, rec.Body.String())
	}
	if code := catalogDecodeObject(t, rec.Body.String())["error"]; code != "deploy_rejected" {
		t.Errorf("error = %v, want deploy_rejected", code)
	}

	rec = catalogDo(t, h, "PUT", "/v1/projects/p-nope/deploy/binding", `{"panaceaProject":"portal-uat"}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown project = %d, body %s", rec.Code, rec.Body.String())
	}
	if code := catalogDecodeObject(t, rec.Body.String())["error"]; code != "project_not_found" {
		t.Errorf("error = %v, want project_not_found", code)
	}
}

// The mutant: a json:"hubMiss" tag, or no tag at all, putting the Hub's cause
// on a wire this task says it is not on. The assertion looks for the KEY, so a
// struct that grew one under any spelling is still caught.
func TestHubMissIsNotOnTheBindingWire(t *testing.T) {
	h := bindAPI(t, bindResolver(t))
	rec := catalogDo(t, h, "PUT", "/v1/projects/p-8c1d0e42/deploy/binding", `{"panaceaProject":"portal-dev"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d, body %s", rec.Code, rec.Body.String())
	}
	for k := range catalogDecodeObject(t, rec.Body.String()) {
		if strings.EqualFold(k, "hubMiss") {
			t.Fatalf("HubMiss is on the wire: %s", rec.Body.String())
		}
	}
}

// The mutant: a route that disappears when phase 7 is unwired, which the
// cockpit reads as "this control plane is too old".
func TestDeployBindingUnwiredIsNotConnected(t *testing.T) {
	h := catalogNewAPI(t, nil)
	rec := catalogDo(t, h, "PUT", "/v1/projects/p-8c1d0e42/deploy/binding", `{"panaceaProject":"x"}`, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unwired = %d, body %s", rec.Code, rec.Body.String())
	}
	if code := catalogDecodeObject(t, rec.Body.String())["error"]; code != "deploy_not_connected" {
		t.Errorf("error = %v, want deploy_not_connected", code)
	}
}
