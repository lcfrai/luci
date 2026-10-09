package dockerbe_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Get is not List's ownership-filtered inventory. A foreign object occupying a
// manual worker's reusable name must remain visible, with its actual immutable
// ref, so trusted lease observation cannot treat it as confirmed absence.
func TestGetManualAbsenceNeverFiltersForeignStoppedOrRemovingObject(t *testing.T) {
	for _, state := range []string{"running", "exited", "removing"} {
		t.Run(state, func(t *testing.T) {
			labels := labelsFor("s-other", "session-other")
			labels[backend.LabelInstance] = "foreign-instance"
			f := &fakeDocker{inspects: []inspectResult{{resp: inspectOf("foreign-immutable-ref", state, "", "", labels)}}}
			b := newBackend(t, f)
			got, err := b.Get(context.Background(), "s-manual")
			if err != nil || errors.Is(err, backend.ErrNotFound) || got.Ref != "foreign-immutable-ref" || got.Instance != "foreign-instance" {
				t.Fatal("foreign present object was projected as absence")
			}
			if len(f.inspectNames) != 1 || f.inspectNames[0] != "luci-s-manual" || len(f.snapshotRemoves()) != 0 || len(f.creates) != 0 {
				t.Fatal("exact lookup performed effects or inspected another allocation")
			}
		})
	}
}

func TestAllocationAbsentRequiresBothNameAndImmutableReferenceAbsent(t *testing.T) {
	ref := strings.Repeat("a", 64)
	missing := inspectResult{err: notFoundErr{"controlled-absent"}}
	present := inspectResult{resp: inspectOf(ref, "exited", "", "", labelsFor("s-manual", "s-manual"))}
	foreignLabels := labelsFor("s-foreign", "s-foreign")
	foreignLabels[backend.LabelInstance] = "foreign-instance"
	foreign := inspectResult{resp: inspectOf(strings.Repeat("b", 64), "removing", "", "", foreignLabels)}
	for _, tc := range []struct {
		name    string
		results []inspectResult
		absent  bool
		failure bool
		calls   int
	}{
		{"both absent", []inspectResult{missing, missing}, true, false, 2},
		{"name absent original renamed", []inspectResult{missing, present}, false, false, 2},
		{"reference absent name foreign", []inspectResult{foreign, missing}, false, false, 1},
		{"original removing", []inspectResult{missing, foreign}, false, false, 2},
		{"name transport failure", []inspectResult{{err: errors.New("controlled transport failure")}}, false, true, 1},
		{"reference transport failure", []inspectResult{missing, {err: errors.New("controlled transport failure")}}, false, true, 2},
		{"joined name absence", []inspectResult{{err: errors.Join(notFoundErr{"controlled"}, backend.ErrUnavailable)}}, false, true, 1},
		{"joined reference absence", []inspectResult{missing, {err: errors.Join(notFoundErr{"controlled"}, context.Canceled)}}, false, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeDocker{inspects: tc.results}
			b := newBackend(t, f)
			absent, err := b.AllocationAbsent(context.Background(), "s-manual", ref)
			if absent != tc.absent || (err != nil) != tc.failure || len(f.inspectNames) != tc.calls {
				t.Fatal("incorrect exact absence result", absent, err, len(f.inspectNames))
			}
			if len(f.inspectNames) > 0 && f.inspectNames[0] != "luci-s-manual" || len(f.inspectNames) > 1 && f.inspectNames[1] != ref {
				t.Fatal("absence did not bind name and immutable reference")
			}
			if len(f.snapshotRemoves()) != 0 || len(f.creates) != 0 || len(f.starts) != 0 {
				t.Fatal("read-only absence observation mutated Docker")
			}
		})
	}
}

func TestAllocationAbsentRejectsUnverifiableReferenceAndCancellation(t *testing.T) {
	for _, scenario := range []string{"empty ref", "short ref", "nonhex ref", "invalid id", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			id, ref := "s-manual", strings.Repeat("a", 64)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "empty ref":
				ref = ""
			case "short ref":
				ref = "abc123"
			case "nonhex ref":
				ref = strings.Repeat("x", 64)
			case "invalid id":
				id = "../foreign"
			case "cancelled":
				cancel()
			}
			f := &fakeDocker{}
			b := newBackend(t, f)
			absent, err := b.AllocationAbsent(ctx, id, ref)
			if absent || err == nil || len(f.inspectNames) != 0 {
				t.Fatal("invalid proof triggered lookup or absence")
			}
		})
	}
}
