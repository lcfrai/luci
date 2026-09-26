package main

// Phase 4 task 10. TASK 14 DELETES THIS FILE when it wires contextwrite.

import (
	"context"
	"log/slog"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
)

// materialiserUntilTask14 is the Materialiser the binary runs between phase 4
// task 10, which makes Deps.Materialiser required, and task 14, which wires
// contextwrite and deletes this file (review I1). Its name is its deadline,
// so it cannot pass for the real thing: it writes nothing. It is not a
// default inside api: api.New still refuses a nil Materialiser, and only
// main chooses this value.
type materialiserUntilTask14 struct{}

// newMaterialiserUntilTask14 logs ONE line, at start-up, that sessions
// inherit nothing until phase 4 task 14. Materialise itself never logs.
func newMaterialiserUntilTask14(log *slog.Logger) materialiserUntilTask14 {
	log.Warn("sessions inherit no brief until phase 4 task 14 wires contextwrite")
	return materialiserUntilTask14{}
}

// Materialise returns "" (nothing inherited), or "none:declined" when the
// person declined the brief, as MaterialiseRequest.WithoutBrief says. No code
// before task 14 writes a managed block (nothing in control-plane imports
// claudemd or contextwrite yet), so there is no block to strip.
func (materialiserUntilTask14) Materialise(ctx context.Context, req api.MaterialiseRequest) (api.MaterialiseResult, error) {
	if req.WithoutBrief {
		return api.MaterialiseResult{ContextRef: "none:declined"}, nil
	}
	return api.MaterialiseResult{}, nil
}

var _ api.Materialiser = materialiserUntilTask14{}
