package product

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func TestPreviewCancellationReopenDoesNotLeaveInactiveNoDemandCancelling(t *testing.T) {
	for _, mode := range []string{"before-allocation", "finished-builder", "live-cancellation", "confirmed-cancelled"} {
		t.Run(mode, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "cancel-reopen")
			identity := previewruntime.Identity{ProjectID: p.ID, ReleaseID: "cancel-before-reply", Target: "local", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64)}
			row := storedRelease{Release: Release{ID: identity.ReleaseID, ProjectID: p.ID, Commit: identity.Commit, Target: "local", Status: "cancelling", Revision: 2, CancelRequested: true}, CancelRevision: 1, Runtime: &previewruntime.Outcome{Identity: identity, Phase: "reserved"}}
			want := "unknown"
			if mode == "finished-builder" {
				row.Runtime.BuildAllocationID, row.Runtime.EngineID, row.Runtime.InstanceID = "finished-build", "engine", "instance"
				row.Runtime.Phase, row.Runtime.CleanupConfirmed = "built", true
			}
			if mode == "confirmed-cancelled" {
				row.Status, row.Runtime.Phase, row.Runtime.CleanupConfirmed = "cancelled", "failed", true
				want = "cancelled"
			}
			if err := s.storeRelease(row); err != nil {
				t.Fatal(err)
			}
			observer := s
			if mode == "live-cancellation" {
				_, cancel := context.WithCancel(context.Background())
				op, err := s.beginLocalPreview(&row, cancel)
				if err != nil {
					t.Fatal(err)
				}
				defer s.finishLocalPreview(row.ID, op)
				defer cancel()
				want = "cancelling"
			} else {
				var err error
				observer, err = New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
				if err != nil {
					t.Fatal(err)
				}
				defer observer.Close()
			}
			observer.SetPreviewRuntime(previewruntime.Runtime{
				Observe: func(context.Context, previewruntime.Identity) (previewruntime.Outcome, error) {
					return previewruntime.Outcome{}, errors.New("no positive runtime settlement proof")
				},
				Start: func(context.Context, previewruntime.Request, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
					t.Error("history replayed a build")
					return previewruntime.Outcome{}, errors.New("forbidden")
				},
				Stop: func(context.Context, previewruntime.Identity, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
					t.Error("history attempted cleanup")
					return previewruntime.Outcome{}, errors.New("forbidden")
				},
			})
			readMux := http.NewServeMux()
			observer.registerReleases(observer.guardProjectSourceRoutes(readMux))
			before, err := os.ReadFile(s.releaseFile(p.ID))
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				views := observedReleaseHistory(t, readMux, p.ID)
				if len(views) != 1 || views[0].Status != want || views[0].CanCancel || !views[0].CancelRequested || views[0].Revision != row.Revision || views[0].URL != "" {
					t.Fatalf("lost ownership must not leave cancellation pending or invent settlement: want %s, got %+v", want, views)
				}
			}
			after, err := os.ReadFile(s.releaseFile(p.ID))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("read-only interruption projection rewrote the retained receipt", err)
			}
		})
	}
}
