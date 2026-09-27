// Package canvas holds no code. It exists so that the module's boundary —
// OD-11 rule 4, "the canvas is its own package ... with a clean interface to
// the cockpit, so it can ship and be tested alone" — is pinned by a test that
// runs at the module root. See docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md.
package canvas

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// goList runs the go command in this directory (the canvas module root) and
// returns its stdout lines. A failure is fatal: a boundary test that cannot
// run must not look like a boundary that holds.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = "."
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		t.Fatalf("go %s printed nothing", strings.Join(args, " "))
	}
	return lines
}

// TestBoundary: nothing the canvas module builds may reach the cockpit's
// control plane or its sandbox daemon (OD-11 rule 4).
func TestBoundary(t *testing.T) {
	const modulePrefix = "github.com/kpmg-au-emu/panacea-app/"
	banned := []string{modulePrefix + "control-plane", modulePrefix + "sandboxd"}
	deps := goList(t, "list", "-deps", "-f", "{{.ImportPath}}", "./...")

	sawOwn := false
	for _, dep := range deps {
		if dep == modulePrefix+"canvas/doc" {
			sawOwn = true
		}
		for _, b := range banned {
			if dep == b || strings.HasPrefix(dep, b+"/") {
				t.Errorf("the canvas module depends on %q, which OD-11 rule 4 forbids", dep)
			}
		}
	}
	if !sawOwn {
		t.Errorf("go list -deps ./... did not name %q, so this test proves nothing: %v", modulePrefix+"canvas/doc", deps)
	}
}

// TestBoundary_editorModule: canvas/editor is a separate module so that the
// ./canvas/... walk stops at the npm package, and the workspace must not use
// it (plan review M6, task-02.md "Why a second go.mod").
func TestBoundary_editorModule(t *testing.T) {
	const editorPath = "github.com/kpmg-au-emu/panacea-app/canvas/editor"
	b, err := os.ReadFile("editor/go.mod")
	if err != nil {
		t.Fatalf("canvas/editor/go.mod is the module boundary and must exist: %v", err)
	}
	var moduleLine string
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "module ") {
			moduleLine = strings.TrimSpace(l)
			break
		}
	}
	if want := "module " + editorPath; moduleLine != want {
		t.Errorf("canvas/editor/go.mod module line = %q, want %q", moduleLine, want)
	}

	// The workspace must not `use` it: with a use line, go list -m names it,
	// while build, vet and test never would (task 02's p2s finding).
	for _, m := range goList(t, "list", "-m", "-f", "{{.Path}}") {
		if m == editorPath {
			t.Errorf("go.work uses %q; the editor module is a boundary, not a workspace module", m)
		}
	}
}
