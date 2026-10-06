package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFailureDiagnosticsCannotReplaceOrExposeTheOriginalAPIError(t *testing.T) {
	secret := "synthetic-private-credential-never-return"
	original := errors.New(secret)
	calls := 0
	a := &API{deps: Deps{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), RecordFailure: func(operation string, cause error) error {
		calls++
		if operation != "prepare private provider state" || cause != original {
			t.Fatal("recorder lost the internal operation or cause")
		}
		return errors.New("diagnostic storage unavailable")
	}}}
	w := httptest.NewRecorder()
	a.internal(w, httptest.NewRequest("POST", "/v1/sessions?private="+secret, nil), "prepare private provider state", original)
	if calls != 1 || w.Code != 500 || strings.Contains(w.Body.String(), secret) || strings.Contains(w.Body.String(), "storage unavailable") {
		t.Fatal("diagnostics obscured the original generic API failure", w.Code, w.Body.String())
	}
}
