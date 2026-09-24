package hubclient

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

func TestErrorCarriesTheProblemUnchanged(t *testing.T) {
	e := &Error{Status: 409, Problem: httpx.Problem{
		Error:   "revision_conflict",
		Message: "Someone saved this brief first.",
		Details: map[string]string{"headRevision": "6"},
	}}
	if got := e.Error(); !strings.Contains(got, "revision_conflict") || !strings.Contains(got, "Someone saved this brief first.") {
		t.Errorf("Error() = %q, want it to name the code and the message", got)
	}
	wrapped := fmt.Errorf("x: %w", e)
	if got := Code(wrapped); got != "revision_conflict" {
		t.Errorf("Code(wrapped) = %q, want revision_conflict", got)
	}
	var he *Error
	if !errors.As(wrapped, &he) || he.Status != 409 || he.Problem.Details["headRevision"] != "6" {
		t.Errorf("errors.As(wrapped) = %+v, want the Problem unchanged", he)
	}
	if got := Code(io.EOF); got != "" {
		t.Errorf("Code(io.EOF) = %q, want empty", got)
	}
	if got := Code(nil); got != "" {
		t.Errorf("Code(nil) = %q, want empty", got)
	}
	// The three sentinels are distinct values: the relay maps each to its own answer.
	for _, pair := range [][2]error{{ErrUnavailable, ErrSignedOut}, {ErrUnavailable, ErrNotConfigured}, {ErrSignedOut, ErrNotConfigured}} {
		if errors.Is(pair[0], pair[1]) {
			t.Errorf("%v is %v; the sentinels must be distinct", pair[0], pair[1])
		}
	}
}
