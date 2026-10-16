package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

// Uses the retained controlled coordinator fixture: native Git/checks/HTTP,
// fake coding/container runtime. No Docker/provider acceptance is claimed.
func TestReviewedSourceJoinHandlerRejectsKindSubstitutionOnReservedReadback(t *testing.T) {
	for _, drift := range []string{"legacy-unchanged", "public-kind", "runtime-kind", "both-kinds", "source-request-under-legacy-plan"} {
		t.Run(drift, func(t *testing.T) {
			s, p, h, ctx, out, starts := coordinatorPreviewFixture(t, "preview", false)
			row := coordinatorReservePreviewFixture(t, s, p, ctx, coordinatorPreviewCheckedFixture(t, s, p, h, ctx, out))
			if err := s.invokeCoordinatorPreview(ctx, p, row); err != nil {
				t.Fatal(err)
			}
			releases, err := s.readReleases(p.ID)
			if err != nil || len(releases) != 1 || starts.Load() != 1 {
				t.Fatal("controlled preview did not create one retained receipt", err)
			}
			switch drift {
			case "public-kind":
				releases[0].Kind = previewruntime.ReviewedSourceKind
			case "runtime-kind":
				releases[0].Runtime.Kind = previewruntime.ReviewedSourceKind
			case "both-kinds":
				releases[0].Kind, releases[0].Runtime.Kind = previewruntime.ReviewedSourceKind, previewruntime.ReviewedSourceKind
			}
			if err := s.storeRelease(releases[0]); err != nil {
				t.Fatal(err)
			}
			before, err := s.readReleases(p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if drift == "source-request-under-legacy-plan" {
				raw, err := json.Marshal(map[string]any{"kind": previewruntime.ReviewedSourceKind, "target": "local", "expectedCommit": row.Delivery.Commit, "port": row.PreviewPlan.Port, "dockerfile": ""})
				if err != nil {
					t.Fatal(err)
				}
				origin := coordinatorPreviewOrigin{row.ID, row.Actor.ID, row.Delivery.Preview.ActionID, row.Delivery.Preview.ReleaseID, coordinatorPreviewBinding(row)}
				request, err := http.NewRequestWithContext(context.WithValue(ctx, coordinatorPreviewContextKey{}, origin), http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				request.SetPathValue("id", p.ID)
				request.Header.Set("Content-Type", "application/json")
				response := &coordinatorResponse{header: make(http.Header)}
				s.createLocalRelease(response, request)
				if response.status != 409 {
					t.Fatal("source handler inherited the legacy Dockerfile approval", response.status)
				}
			} else {
				err = s.invokeCoordinatorPreview(ctx, p, row)
				if (err == nil) != (drift == "legacy-unchanged") {
					t.Fatal("handler did not preserve legacy omission or refuse kind replacement", err)
				}
			}
			after, err := s.readReleases(p.ID)
			if err != nil || !reflect.DeepEqual(before, after) || starts.Load() != 1 {
				t.Fatal("readback allocated again or changed the retained receipt", err)
			}
		})
	}
}

func TestReviewedSourceJoinCASRejectsPublicAndRuntimeKindChanges(t *testing.T) {
	for _, drift := range []string{"stored-public", "next-public", "next-runtime", "publication"} {
		t.Run(drift, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "kind-cas")
			row := retainedPreviewFixture(t, s, p.ID)
			next := row
			copyRuntime := *row.Runtime
			next.Runtime = &copyRuntime
			switch drift {
			case "stored-public":
				changed := row
				changed.Kind = previewruntime.ReviewedSourceKind
				if err := s.storeRelease(changed); err != nil {
					t.Fatal(err)
				}
			case "next-public":
				next.Kind = previewruntime.ReviewedSourceKind
			case "next-runtime":
				next.Runtime.Kind = previewruntime.ReviewedSourceKind
			case "publication":
				out := *row.Runtime
				out.Kind = previewruntime.ReviewedSourceKind
				if err := s.publishPreviewOutcome(&row, out, nil); err == nil {
					t.Fatal("publication changed immutable preview kind")
				}
				return
			}
			before, err := s.readReleases(p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.storeAdmittedRelease(row, &next); err == nil {
				t.Fatal("preview CAS accepted a kind replacement")
			}
			after, err := s.readReleases(p.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("refused kind replacement changed retained history", err)
			}
		})
	}
}
