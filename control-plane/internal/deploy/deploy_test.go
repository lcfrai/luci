package deploy_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// TestValidCommit pins the one commit form phase 7 ever sends: 40 lowercase hex. A symbolic ref
// would ship whatever the branch holds when the pipeline clones, not what the user reviewed.
func TestValidCommit(t *testing.T) {
	hex40 := "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"40 lowercase hex", hex40, true},
		{"all digits", strings.Repeat("7", 40), true},
		{"39 characters", hex40[:39], false},
		{"41 characters", hex40 + "8", false},
		{"uppercase", strings.ToUpper(hex40), false},
		{"one uppercase letter", "A" + hex40[1:], false},
		{"40 characters, one not hex", "g" + hex40[1:], false},
		{"a branch name", "main", false},
		{"empty", "", false},
		{"40 characters with a space", " " + hex40[1:], false},
	}
	for _, c := range cases {
		if got := deploy.ValidCommit(c.in); got != c.want {
			t.Errorf("%s: ValidCommit(%q) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// TestParsePostureModeNeverPublic: only the exact word "none" opens a branch to the internet.
// Every other word, including case variants and plausible synonyms, fails closed to kpmg-sso.
func TestParsePostureModeNeverPublic(t *testing.T) {
	cases := []struct {
		in        string
		wantMode  deploy.PostureMode
		wantKnown bool
	}{
		{"panacea-sso", deploy.PosturePanaceaSSO, true},
		{"kpmg-sso", deploy.PostureKPMGSSO, true},
		{"password", deploy.PosturePassword, true},
		{"bypass-token", deploy.PostureBypassToken, true},
		{"none", deploy.PosturePublic, true},
		{"None", deploy.PostureKPMGSSO, false},
		{"NONE", deploy.PostureKPMGSSO, false},
		{" none", deploy.PostureKPMGSSO, false},
		{"KPMG-SSO", deploy.PostureKPMGSSO, false},
		{"", deploy.PostureKPMGSSO, false},
		{"public", deploy.PostureKPMGSSO, false},
		{"open", deploy.PostureKPMGSSO, false},
	}
	if len(cases) != 12 {
		t.Fatalf("the table is 12 words; got %d", len(cases))
	}
	for _, c := range cases {
		mode, known := deploy.ParsePostureMode(c.in)
		if mode != c.wantMode || known != c.wantKnown {
			t.Errorf("ParsePostureMode(%q) = (%q, %v), want (%q, %v)", c.in, mode, known, c.wantMode, c.wantKnown)
		}
		if mode == deploy.PosturePublic && c.in != "none" {
			t.Errorf("ParsePostureMode(%q) opened the branch to the internet", c.in)
		}
	}
	// The five words are the product's vocabulary, adopted on purpose (t01 PostureMode note).
	words := map[deploy.PostureMode]string{
		deploy.PosturePanaceaSSO: "panacea-sso", deploy.PostureKPMGSSO: "kpmg-sso",
		deploy.PosturePassword: "password", deploy.PostureBypassToken: "bypass-token",
		deploy.PosturePublic: "none",
	}
	for mode, word := range words {
		if string(mode) != word {
			t.Errorf("mode constant %q, want the word %q", mode, word)
		}
	}
}

// TestErrorNeverRendersCause: Error() is what reaches a log line. The cause can hold the request
// that failed, and so the credential; it is kept for errors.As and never formatted.
func TestErrorNeverRendersCause(t *testing.T) {
	cause := errors.New("Bearer pnc_abc")
	e := &deploy.Error{Kind: deploy.KindUnauthorized, Message: "invalid api key", Op: "whoami", Cause: cause}
	got := e.Error()
	if strings.Contains(got, "pnc_") || strings.Contains(got, "Bearer") {
		t.Fatalf("Error() rendered the cause: %q", got)
	}
	if want := "deploy: whoami: unauthorized: invalid api key"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(e, cause) || errors.Unwrap(e) != cause {
		t.Errorf("Unwrap() does not return the cause")
	}
	bare := &deploy.Error{Kind: deploy.KindUnavailable, Op: "ship", Status: 503, Cause: cause}
	if want := "deploy: ship: unavailable"; bare.Error() != want {
		t.Errorf("Error() with no message = %q, want %q", bare.Error(), want)
	}
	if got := deploy.ErrNotConnected.Error(); got != "deploy: connect: not_connected" {
		t.Errorf("ErrNotConnected.Error() = %q", got)
	}
}

// TestKindOf: an error that is not a deploy *Error is the target being unreachable, never a
// "not found" the lens would draw as a deleted project.
func TestKindOf(t *testing.T) {
	var typedNil *deploy.Error
	cases := []struct {
		name string
		err  error
		want deploy.Kind
	}{
		{"nil", nil, ""},
		{"a foreign error", io.EOF, deploy.KindUnavailable},
		{"a deploy error", &deploy.Error{Kind: deploy.KindNotFound}, deploy.KindNotFound},
		{"a wrapped deploy error", fmt.Errorf("watcher: %w", &deploy.Error{Kind: deploy.KindForbidden}), deploy.KindForbidden},
		{"twice wrapped", fmt.Errorf("a: %w", fmt.Errorf("b: %w", &deploy.Error{Kind: deploy.KindConflict})), deploy.KindConflict},
		{"ErrNotConnected", deploy.ErrNotConnected, deploy.KindNotConnected},
		// Two guards beyond the declaration (declared in the t01 report): a non-nil error never
		// reads as "" (no error), and a typed-nil *Error does not panic.
		{"a deploy error with no kind", &deploy.Error{Op: "ship"}, deploy.KindUnavailable},
		{"a typed-nil deploy error", typedNil, deploy.KindUnavailable},
	}
	for _, c := range cases {
		if got := deploy.KindOf(c.err); got != c.want {
			t.Errorf("%s: KindOf = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestBuildStateTerminal: the watcher leaves its fast cadence only on a state that cannot change.
func TestBuildStateTerminal(t *testing.T) {
	cases := map[deploy.BuildState]bool{
		deploy.BuildSucceeded:  true,
		deploy.BuildFailed:     true,
		deploy.BuildHistorical: true,
		deploy.BuildPending:    false,
		deploy.BuildRunning:    false,
		deploy.BuildUnknown:    false,
		deploy.BuildState(""):  false,
		deploy.BuildState("x"): false,
	}
	for state, want := range cases {
		if got := state.Terminal(); got != want {
			t.Errorf("BuildState(%q).Terminal() = %v, want %v", state, got, want)
		}
	}
}
