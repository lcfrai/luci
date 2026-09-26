package main

// Phase 4 task 10. TASK 14 DELETES THIS FILE with materialiser_interim.go.

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
)

func TestMaterialiserUntilTask14InheritsNothingAndSaysSoOnce(t *testing.T) {
	var buf bytes.Buffer
	m := newMaterialiserUntilTask14(slog.New(slog.NewTextHandler(&buf, nil)))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "level=WARN") || !strings.Contains(lines[0], "task 14") {
		t.Fatalf("construction logged %q; want exactly one Warn line naming task 14", buf.String())
	}
	logged := buf.Len()

	got, err := m.Materialise(context.Background(), api.MaterialiseRequest{
		WorktreeID: "w-4a77b1c9", WorktreePath: t.TempDir(), ProjectID: "p-8c1d0e42", BranchID: "b-8c1d0e42-main",
	})
	if err != nil || got.ContextRef != "" {
		t.Errorf("Materialise = %+v, %v; want \"\", nil: nothing is written, so nothing may be claimed", got, err)
	}
	got, err = m.Materialise(context.Background(), api.MaterialiseRequest{WorktreeID: "w-4a77b1c9", WithoutBrief: true})
	if err != nil || got.ContextRef != "none:declined" {
		t.Errorf("Materialise(WithoutBrief) = %+v, %v; want none:declined, nil", got, err)
	}
	if buf.Len() != logged {
		t.Errorf("Materialise logged %q; want no line per session start", buf.String()[logged:])
	}
}
