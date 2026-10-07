package product

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/platform"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestPlatformDataDomainAdapterRejectsRemoteEngineBeforeAllocation(t *testing.T) {
	f := &dataDomainProviderFixture{}
	s, req, pre := dataDomainFixture(t, f)
	commands := 0
	native, err := platform.New(platform.Options{
		StateDir: t.TempDir(), DockerHost: "tcp://private-engine:2375",
		ResolveProject: func(string) (project.Project, error) {
			return project.Project{}, errors.New("unexpected project lookup")
		},
		Runner: func(context.Context, string, []string, string, []byte) ([]byte, error) {
			commands++
			return nil, errors.New("unexpected command")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(native.Close)
	bridge := s.PlatformDataDomains(func() *platform.Service { return native })
	host := platform.DataDomainHost{ProjectID: req.ProjectID, StableTenant: req.StableTenant, ApplicationNetwork: pre.Data.ApplicationNetwork, NetworkID: pre.Data.NetworkID, DockerEngineID: pre.Data.DockerEngineID, PublishableKey: pre.Data.PublishableKey}
	input := platform.DataDomainRequest{ProjectID: req.ProjectID, StableTenant: req.StableTenant, TargetID: req.TargetID, Domain: req.Domain, AppOrigin: req.AppOrigin, AcknowledgePublic: true, ServerUUID: "browser-cannot-replace-server"}
	if err := bridge.Preflight(context.Background(), input, host); err == nil {
		t.Fatal("remote engine admitted as local native provider")
	}
	if commands != 0 || f.projectCreates != 0 || f.creates != 0 || f.starts != 0 {
		t.Fatal("failed host preflight allocated or ran native commands")
	}
	input.ProjectID = "another-project"
	if err := bridge.Preflight(context.Background(), input, host); err == nil {
		t.Fatal("foreign project request admitted")
	}
}

func TestPlatformDataAdapterMissingIntegrationAndUnusedData(t *testing.T) {
	ctx := context.Background()
	for _, resolve := range []func() *platform.Service{nil, func() *platform.Service { return nil }} {
		if _, err := PlatformCloudDataBindings(resolve)(ctx, CloudDataBindingRequest{ProjectID: "owned"}); err == nil {
			t.Fatal("missing native integration accepted")
		}
		bridge := (&Server{}).PlatformDataDomains(resolve)
		if _, err := bridge.Refresh(ctx, "owned", platform.DataDomainHost{ProjectID: "owned"}); err == nil {
			t.Fatal("missing native provider accepted")
		}
	}
	commands := 0
	native, err := platform.New(platform.Options{
		StateDir:       t.TempDir(),
		ResolveProject: func(string) (project.Project, error) { return project.Project{}, nil },
		Runner: func(context.Context, string, []string, string, []byte) ([]byte, error) {
			commands++
			return nil, errors.New("unexpected command")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(native.Close)
	read := PlatformCloudDataBindings(func() *platform.Service { return native })
	v, err := read(ctx, CloudDataBindingRequest{ProjectID: "owned", Framework: "none"})
	if err != nil || v.Installed || v.ProjectID != "owned" || v.DomainStatus != "not-configured" || commands != 0 {
		t.Fatalf("unused native data requires installation or engine access: %+v, %v", v, err)
	}
}
