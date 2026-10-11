package product

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func retainedPreviewFixture(t *testing.T, s *Server, projectID string) storedRelease {
	t.Helper()
	identity := previewruntime.Identity{ProjectID: projectID, ReleaseID: "rel-retained", Target: "local", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), BuildAllocationID: "build-retained", EngineID: "engine-retained", InstanceID: "instance-retained"}
	row := storedRelease{Release: Release{ID: identity.ReleaseID, ProjectID: projectID, Target: "local", Commit: identity.Commit, Status: "building", ConfigurationRevision: 3, Revision: 1}, Runtime: &previewruntime.Outcome{Identity: identity, Phase: "building", ResourcesHeld: true}}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestPreviewReceiptRetainsUnknownBuildWithoutContainerAndNeverReplays(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "retained-preview")
	row := retainedPreviewFixture(t, s, p.ID)
	out := *row.Runtime
	out.Phase, out.Message = "unknown", "The build acknowledgement was lost."
	if err := s.publishPreviewOutcome(&row, out, nil); err != nil {
		t.Fatal(err)
	}
	if row.Status != "unknown" || row.ContainerID != "" || !s.browserRelease(row).CanStop {
		t.Fatal("unknown pre-container build lost its stop affordance")
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), Runner: func(context.Context, string, []string, string) (string, error) {
		t.Error("restart observation attempted a resource operation")
		return "", errors.New("no runtime")
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	rows, err := reopened.readReleases(p.ID)
	if err != nil || len(rows) != 1 || rows[0].Runtime == nil || *rows[0].Runtime != out {
		t.Fatal("restart lost the exact uncertain allocation", err)
	}
	before := auditTree(t, s.stateDir)
	views := reopened.observeLocalReleases(context.Background(), rows)
	if len(views) != 1 || views[0].Status != "unknown" || !reopened.browserRelease(views[0]).CanStop {
		t.Fatal("restart manufactured a ready or settled build")
	}
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("observation rewrote the retained receipt")
	}
}

func TestPreviewReceiptCASPreventsStaleReadyFromReplacingExactStop(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "preview-cas")
	row := retainedPreviewFixture(t, s, p.ID)
	stale := row
	finished := *row.Runtime
	finished.Phase, finished.ResourcesHeld, finished.CleanupConfirmed = "finished", false, true
	if err := s.publishPreviewOutcome(&row, finished, nil); err != nil {
		t.Fatal(err)
	}
	late := *stale.Runtime
	late.Phase, late.AllocationID, late.ContainerID = "running", "serve-stale", strings.Repeat("c", 64)
	if err := s.publishPreviewOutcome(&stale, late, nil); err == nil {
		t.Fatal("stale launch replaced a newer exact stop")
	}
	rows, err := s.readReleases(p.ID)
	if err != nil || len(rows) != 1 || rows[0].Status != "stopped" || rows[0].Runtime.ResourcesHeld || rows[0].Runtime.AllocationID != "" {
		t.Fatal("stale result changed the stopped receipt", err)
	}
	foreign := finished
	foreign.EngineID = "another-engine"
	if err := s.publishPreviewOutcome(&row, foreign, nil); err == nil {
		t.Fatal("known engine identity was replaceable")
	}
}

func TestLocalPreviewHasNoUnadmittedRunnerFallback(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "preview-runtime-required")
	s.SetPreviewRuntime(previewruntime.Runtime{})
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Error("missing runtime reached a command runner")
		return "", errors.New("forbidden")
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/releases", map[string]any{"target": "local"}, 503)
	rows, err := s.readReleases(p.ID)
	if err != nil || len(rows) != 0 {
		t.Fatal("unadmitted preview created a release receipt", err)
	}
}
