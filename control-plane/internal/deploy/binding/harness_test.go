package binding_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

func classifyHubStub(err error) binding.HubErrorClass {
	switch {
	case errors.Is(err, errHubBadRepo):
		return binding.HubRefusedKey
	case errors.Is(err, errHubSignedIn):
		return binding.HubSignedOut
	}
	return binding.HubBlip
}

type harness struct {
	locals     *fakeLocals
	remotes    *fakeRemotes
	fake       *deploytest.Fake
	hub        *hubStub
	dir        string
	origin     string
	useHub     bool
	hubTimeout time.Duration
	accepted   func(string) bool
	logs       io.Writer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{
		locals:  &fakeLocals{},
		remotes: &fakeRemotes{origins: map[string]string{}},
		fake:    deploytest.NewFake(func() time.Time { return time.Unix(1700000000, 0).UTC() }),
		hub:     &hubStub{byKey: map[string]string{}},
		dir:     t.TempDir(),
		origin:  "https://dev.panacea.kpmg",
	}
}

// local seeds one phase 2 project with an origin URL. "" means no origin.
func (h *harness) local(id, origin string) string {
	path := "/w/" + id
	h.locals.list = append(h.locals.list, project.Project{ID: id, Name: id, Path: path})
	if origin != "" {
		h.remotes.origins[path] = origin
	}
	return id
}

// panaceaProject seeds one deploy-target project with the repoUrl it reports.
func (h *harness) panaceaProject(ref, repoURL string) {
	h.fake.AddProject(deploy.Project{
		Ref:         deploy.ProjectRef(ref),
		DisplayName: ref,
		RepoURL:     repoURL,
	})
}

func (h *harness) withHub(accepted func(string) bool) *harness {
	h.useHub, h.accepted = true, accepted
	return h
}

func (h *harness) cfg() binding.Config {
	cfg := binding.Config{
		Projects:         h.locals,
		Remotes:          h.remotes,
		Deployer:         h.fake,
		ParseRemote:      testParse,
		ClassifyHubError: classifyHubStub,
		PanaceaOrigin:    func() string { return h.origin },
		ChoicesDir:       h.dir,
		Now:              func() time.Time { return time.Unix(1700000000, 0).UTC() },
		Log:              slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if h.useHub {
		cfg.Hub, cfg.HubKeyAccepted, cfg.HubReadTimeout = h.hub, h.accepted, h.hubTimeout
	}
	return cfg
}

func (h *harness) build(t *testing.T) *binding.Resolver {
	t.Helper()
	r, err := binding.New(h.cfg())
	if err != nil {
		t.Fatalf("binding.New: %v", err)
	}
	return r
}
