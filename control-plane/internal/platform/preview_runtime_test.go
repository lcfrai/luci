package platform

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func TestRetainedCrashedDeploymentKeepsOwnedRedactedLogs(t *testing.T) {
	var s *Service
	identity := previewruntime.Identity{ProjectID: "owned", ReleaseID: "deploy-crashed", Target: "native-docker", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("d", 64), AllocationID: "serve-crashed", ContainerID: strings.Repeat("b", 64), Image: "sha256:" + strings.Repeat("c", 64)}
	s = fixture(t, func(_ context.Context, exe string, args []string, _ string, _ []byte) ([]byte, error) {
		if exe != "docker" || len(args) < 2 || args[len(args)-1] != identity.ContainerID {
			return nil, errors.New("unexpected container target")
		}
		switch args[0] {
		case "inspect":
			info := containerInfo{ID: identity.ContainerID, Image: identity.Image}
			info.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: identity.ReleaseID}
			return json.Marshal([]containerInfo{info})
		case "logs":
			return []byte("crashed with fixture-private-value"), nil
		default:
			t.Error("logs attempted a mutation")
			return nil, errors.New("unexpected operation")
		}
	})
	out := previewruntime.Outcome{Identity: identity, Phase: "exited", ResourcesHeld: true}
	runtime := s.previewRuntime()
	runtime.Observe = func(_ context.Context, id previewruntime.Identity) (previewruntime.Outcome, error) {
		if id != identity {
			return previewruntime.Outcome{}, errors.New("wrong allocation")
		}
		return out, nil
	}
	s.opts.PreviewRuntime = func() previewruntime.Runtime { return runtime }
	d := Deployment{ID: identity.ReleaseID, ProjectID: "owned", Provider: "docker", Status: "unknown", Container: identity.ContainerID, Image: identity.Image, Runtime: &out}
	if err := s.saveDeployment(d); err != nil {
		t.Fatal(err)
	}
	if err := s.write(d.ID+"-redactions", map[string]string{"APP_SECRET": "fixture-private-value"}); err != nil {
		t.Fatal(err)
	}
	w := request(t, s, "GET", "/v1/platform/projects/owned/deployments/"+d.ID+"/logs", "member", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "[redacted]") || strings.Contains(w.Body.String(), "fixture-private-value") {
		t.Fatal("crashed owned container logs were unavailable or leaked", w.Code, w.Body.String())
	}
}

func TestBrowserDeploymentDoesNotExposeOrMutatePrivateRuntimeOrigin(t *testing.T) {
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) {
		t.Error("receipt projection invoked Docker")
		return nil, errors.New("unexpected")
	})
	out := previewruntime.Outcome{Identity: previewruntime.Identity{ProjectID: "owned", ReleaseID: "deploy-private"}, URL: "http://127.0.0.1:45678", Phase: "unknown", ResourcesHeld: true}
	d := Deployment{ID: "deploy-private", ProjectID: "owned", Provider: "docker", Status: "unknown", Runtime: &out}
	public := s.browserDeployment(d)
	if public.Runtime == nil || public.Runtime.URL != "" || d.Runtime.URL != "http://127.0.0.1:45678" {
		t.Fatal("public receipt exposed private runtime origin or mutated durable outcome")
	}
}
