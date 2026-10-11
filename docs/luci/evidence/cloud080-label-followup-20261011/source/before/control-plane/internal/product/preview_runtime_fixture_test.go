package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

// Only the typed boundary is substituted. Calls below reach each test's
// explicit runner fixture, never an actual Docker daemon. They retain existing
// committed archive, private configuration, health and ownership assertions;
// they make no shared admission or backend enforcement claim.
func installControlledPreviewRuntime(t *testing.T, s *Server) {
	t.Helper()
	initialRunner := reflect.ValueOf(s.runner).Pointer()
	read := func(id previewruntime.Identity) (previewruntime.Outcome, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		rows, err := s.readReleases(id.ProjectID)
		if err != nil {
			return previewruntime.Outcome{}, err
		}
		for _, row := range rows {
			if row.ID == id.ReleaseID && row.Runtime != nil && row.Runtime.Identity == id {
				return *row.Runtime, nil
			}
		}
		return previewruntime.Outcome{}, errors.New("exact fixture receipt missing")
	}
	s.SetPreviewRuntime(previewruntime.Runtime{
		CaptureActor: management.CaptureDeploymentActor,
		Start: func(ctx context.Context, req previewruntime.Request, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
			out := previewruntime.Outcome{Identity: req.Identity, Phase: "building", ResourcesHeld: true}
			if reflect.ValueOf(s.runner).Pointer() == initialRunner {
				t.Error("unexpected preview without explicit runner fixture")
				return out, errors.New("preview runner fixture missing")
			}
			if req.Actor.ProjectID != req.ProjectID || req.SourceDirectory == "" || req.SourceDigest == "" || req.ValidateSource == nil {
				t.Error("typed preview lost its source/actor binding")
				return out, errors.New("invalid preview fixture request")
			}
			if err := req.ValidateSource(ctx); err != nil {
				return out, err
			}
			out.BuildAllocationID, out.EngineID, out.InstanceID = "build-"+req.ReleaseID, "fixture-engine", "fixture-instance"
			if err := publish(out); err != nil {
				return out, err
			}
			tag := "luci/preview:" + req.ReleaseID
			if _, err := s.runner(ctx, "docker", []string{"build", "--tag", tag, "--file", filepath.Join(req.SourceDirectory, req.Dockerfile), req.SourceDirectory}, req.SourceDirectory); err != nil {
				// This fixture's build failure is deliberately settled. Unknown
				// transport failures are covered separately with a retained lease.
				out.Phase, out.ResourcesHeld, out.CleanupConfirmed, out.Message = "failed", false, true, err.Error()
				if e := publish(out); e != nil {
					return out, e
				}
				return out, err
			}
			out.Image = "sha256:" + strings.Repeat("c", 64)
			out.Phase, out.ResourcesHeld = "built", false
			if err := publish(out); err != nil {
				return out, err
			}
			if err := req.ValidateSource(ctx); err != nil {
				return out, err
			}
			out.AllocationID, out.Phase, out.ResourcesHeld = "serve-"+req.ReleaseID, "creating", true
			if err := publish(out); err != nil {
				return out, err
			}
			args := []string{"run", "--detach", "--init", "--name", "luci-preview-" + req.ReleaseID, "--label", "luci.preview=true", "--label", "luci.release-id=" + req.ReleaseID, "--label", "luci.project-id=" + req.ProjectID, "--publish", fmt.Sprintf("127.0.0.1::%d", req.Port)}
			file, err := s.writePreviewEnvironment(req.Environment)
			if err != nil {
				return out, err
			}
			if file != "" {
				defer os.Remove(file)
				args = append(args, "--env-file", file)
			}
			args = append(args, tag)
			ref, err := s.runner(ctx, "docker", args, "")
			if file != "" {
				if removeErr := os.Remove(file); err == nil {
					err = removeErr
				}
			}
			if err != nil {
				out.Phase, out.Message = "unknown", err.Error()
				if e := publish(out); e != nil {
					return out, e
				}
				return out, err
			}
			out.ContainerID = strings.TrimSpace(ref)
			if !previewContainerID.MatchString(out.ContainerID) {
				return out, errors.New("invalid controlled container ID")
			}
			out.Phase = "starting"
			if err := publish(out); err != nil {
				return out, err
			}
			mapping, err := s.runner(ctx, "docker", []string{"port", out.ContainerID, strconv.Itoa(req.Port) + "/tcp"}, "")
			if err != nil {
				return out, err
			}
			out.URL, err = localPreviewURL(mapping)
			if err != nil {
				return out, err
			}
			out.Phase = "running"
			if err := publish(out); err != nil {
				return out, err
			}
			return out, nil
		},
		Observe: func(ctx context.Context, id previewruntime.Identity) (previewruntime.Outcome, error) {
			out, err := read(id)
			if err != nil || out.Phase != "running" {
				return out, err
			}
			data, err := s.runner(ctx, "docker", []string{"inspect", "--format", `{"running":{{json .State.Running}},"labels":{{json .Config.Labels}},"ports":{{json .NetworkSettings.Ports}}}`, id.ContainerID}, "")
			if err != nil {
				return out, err
			}
			var inspection struct {
				Running bool `json:"running"`
			}
			if json.Unmarshal([]byte(data), &inspection) != nil || !inspection.Running {
				return out, errors.New("controlled preview not running")
			}
			return out, nil
		},
		Stop: func(ctx context.Context, id previewruntime.Identity, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
			out, err := read(id)
			if err != nil || out.Phase == "finished" {
				return out, err
			}
			data, err := s.runner(ctx, "docker", []string{"inspect", "--format", "{{json .Config.Labels}}", id.ContainerID}, "")
			if err != nil {
				return out, err
			}
			var labels map[string]string
			if json.Unmarshal([]byte(data), &labels) != nil || labels["luci.preview"] != "true" || labels["luci.release-id"] != id.ReleaseID || labels["luci.project-id"] != id.ProjectID {
				return out, errors.New("fixture ownership mismatch")
			}
			out.Phase = "cleaning"
			if err := publish(out); err != nil {
				return out, err
			}
			if _, err := s.runner(ctx, "docker", []string{"rm", "--force", id.ContainerID}, ""); err != nil {
				return out, err
			}
			out.Phase, out.ResourcesHeld, out.CleanupConfirmed, out.URL = "finished", false, true, ""
			if err := publish(out); err != nil {
				return out, err
			}
			return out, nil
		},
	})
}
