package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalReleaseExpectedCommitRejectsChangedSourceBeforeReservation(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "checked-preview")
	base := "/v1/product/projects/" + p.ID
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	head, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head = strings.TrimSpace(head)
	dockerCalls := 0
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "docker" {
			return RunCommand(ctx, exe, args, dir)
		}
		dockerCalls++
		return "", errors.New("stop at the build boundary")
	}
	before := auditTree(t, s.stateDir)
	for _, value := range []struct {
		commit string
		status int
	}{
		{strings.Repeat("f", 40), 409}, {head[:12], 400}, {strings.ToUpper(head), 400}, {" " + head, 400},
	} {
		journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"target": "local", "expectedCommit": value.commit}, value.status)
	}
	if dockerCalls != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("changed or invalid checked source allocated release provenance or Docker work")
	}
	journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"target": "local", "expectedCommit": head}, 422)
	rows, err := s.readReleases(p.ID)
	if err != nil || dockerCalls != 1 || len(rows) != 1 || rows[0].Commit != head || rows[0].Status != "failed" {
		t.Fatal("matching commit did not reach exactly one recorded build attempt", dockerCalls, rows, err)
	}
}
