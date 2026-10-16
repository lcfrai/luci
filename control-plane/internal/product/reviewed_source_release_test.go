package product

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func TestReviewedSourceReleaseRequiresExplicitKindAndExactCommit(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "source-kind")
	starts := 0
	rt := s.PreviewRuntime()
	rt.Start = func(context.Context, previewruntime.Request, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		starts++
		return previewruntime.Outcome{}, errors.New("must not start")
	}
	s.SetPreviewRuntime(rt)
	for _, request := range []map[string]any{
		{"kind": "unknown", "target": "local"},
		{"kind": "dockerfile", "target": "local"},
		{"kind": previewruntime.ReviewedSourceKind, "target": "local"},
		{"kind": previewruntime.ReviewedSourceKind, "target": "local", "expectedCommit": strings.Repeat("a", 40), "dockerfile": "Dockerfile"},
	} {
		journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/releases", request, 400)
	}
	rows, err := s.readReleases(p.ID)
	if err != nil || len(rows) != 0 || starts != 0 {
		t.Fatal("invalid source kind created runtime or history", err)
	}
}

func TestReviewedSourceReleaseUsesReviewedSanitizedCommitWithoutDockerfile(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "source-reviewed-commit")
	base := "/v1/product/projects/" + p.ID
	for name, data := range map[string]string{"index.html": "<h1>synthetic reviewed fixture</h1>", "app.js": "console.log('browser bytes only')", ".env": "SYNTHETIC_PRIVATE=not-for-snapshot"} {
		if err := os.WriteFile(filepath.Join(p.Path, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	head, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head = strings.TrimSpace(head)
	starts := 0
	rt := s.PreviewRuntime()
	rt.Start = func(ctx context.Context, req previewruntime.Request, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		starts++
		if req.Kind != previewruntime.ReviewedSourceKind || req.Commit != head || req.Dockerfile != "" || len(req.Environment) != 0 || req.BuildAllocationID != "" || req.SourceDigest == "" {
			t.Fatal("normal source request lost its exact reviewed profile")
		}
		if err := req.ValidateSource(ctx); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(req.SourceDirectory, "index.html")); err != nil || string(data) != "<h1>synthetic reviewed fixture</h1>" {
			t.Fatal("reviewed immutable archive missing", err)
		}
		if _, err := os.Stat(filepath.Join(req.SourceDirectory, ".env")); !os.IsNotExist(err) {
			t.Fatal("private project configuration entered static source snapshot")
		}
		out := previewruntime.Outcome{Identity: req.Identity, Phase: "blocked", Message: "Synthetic typed-boundary stop before any allocation."}
		if err := publish(out); err != nil {
			t.Fatal(err)
		}
		return out, errors.New("synthetic boundary; no container or server")
	}
	s.SetPreviewRuntime(rt)
	journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"kind": previewruntime.ReviewedSourceKind, "target": "local", "expectedCommit": head, "port": 3000}, 422)
	rows, err := s.readReleases(p.ID)
	if err != nil || len(rows) != 1 || starts != 1 || rows[0].Kind != previewruntime.ReviewedSourceKind || rows[0].Runtime.Kind != previewruntime.ReviewedSourceKind || rows[0].Runtime.Retained() || rows[0].Status != "failed" {
		t.Fatal("source release fabricated image/resource success or lost kind", err)
	}
}

func TestReviewedSourceReleaseRefusesPrivateEnvironmentBeforeReservation(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerApplicationConfiguration(mux)
	p := createJourneyProject(t, mux, "source-no-private-environment")
	base := "/v1/product/projects/" + p.ID
	if err := os.WriteFile(filepath.Join(p.Path, "index.html"), []byte("synthetic fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	head, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/configuration", map[string]any{"expectedRevision": 0, "updates": []map[string]any{{"name": "SYNTHETIC_PRIVATE", "value": "fixture-only"}}}, 200)
	starts := 0
	rt := s.PreviewRuntime()
	rt.Start = func(context.Context, previewruntime.Request, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		starts++
		return previewruntime.Outcome{}, errors.New("must not start")
	}
	s.SetPreviewRuntime(rt)
	journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"kind": previewruntime.ReviewedSourceKind, "target": "local", "expectedCommit": strings.TrimSpace(head)}, 422)
	rows, err := s.readReleases(p.ID)
	if err != nil || len(rows) != 0 || starts != 0 {
		t.Fatal("private environment reached source allocation", err)
	}
}

func TestReviewedSourceReadinessRequiresExactBytesAndFullSnapshotReceipt(t *testing.T) {
	index := []byte("<h1>reviewed static fixture</h1>")
	for _, name := range []string{"exact", "wrong-body", "wrong-source", "wrong-snapshot", "missing-snapshot", "wrong-type", "redirect", "overflow"} {
		t.Run(name, func(t *testing.T) {
			response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}, "X-Luci-Source-Digest": []string{"source"}, "X-Luci-Snapshot-Digest": []string{"snapshot"}}, Body: io.NopCloser(bytes.NewReader(index))}
			switch name {
			case "wrong-body":
				response.Body = io.NopCloser(strings.NewReader("other content"))
			case "wrong-source":
				response.Header.Set("X-Luci-Source-Digest", "different")
			case "wrong-snapshot":
				response.Header.Set("X-Luci-Snapshot-Digest", "different")
			case "missing-snapshot":
				response.Header.Del("X-Luci-Snapshot-Digest")
			case "wrong-type":
				response.Header.Set("Content-Type", "text/plain")
			case "redirect":
				response.StatusCode = 302
			case "overflow":
				response.Body = io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), (64<<10)+1)))
			}
			defer response.Body.Close()
			if reviewedSourceResponseMatches(response, index, "source", "snapshot") != (name == "exact") {
				t.Fatal("static readiness accepted a generic/wrong HTTP receipt")
			}
		})
	}
}
