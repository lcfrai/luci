package api

// The table this file pins is the one place a deploy.Error becomes a status
// code, so it is tested where it lives rather than through whichever route
// happens to reach a given Kind. It is an internal test for the same reason:
// writeDeployError is unexported, and exporting it for a test would put a
// mapping on the package's surface that nothing outside it may re-decide.

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

func deployErrAPI() *API {
	return &API{deps: Deps{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}}
}

// deployErrWrite answers one error and decodes the problem it produced.
func deployErrWrite(t *testing.T, err error, extra ...map[string]any) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8730/v1/deploy/thing", nil)
	deployErrAPI().writeDeployError(rec, r, "test", err, extra...)
	var body map[string]any
	if e := json.Unmarshal(rec.Body.Bytes(), &body); e != nil {
		t.Fatalf("body %q is not a JSON object: %v", rec.Body.String(), e)
	}
	return rec.Code, body
}

func deployErrDetails(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	d, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("details = %v, want an object", body["details"])
	}
	return d
}

func TestWriteDeployErrorTable(t *testing.T) {
	for name, tc := range map[string]struct {
		err           error
		status        int
		code, message string
	}{
		"not connected": {&deploy.Error{Kind: deploy.KindNotConnected, Op: "connect"},
			http.StatusConflict, "deploy_not_connected", "Connect to Panacea first."},
		"unauthorized": {&deploy.Error{Kind: deploy.KindUnauthorized, Status: 401,
			Message: "api key revoked or expired"},
			http.StatusConflict, "deploy_token_rejected", "api key revoked or expired"},
		"forbidden": {&deploy.Error{Kind: deploy.KindForbidden, Status: 403,
			Message: "You cannot deploy this project."},
			http.StatusForbidden, "deploy_forbidden", "You cannot deploy this project."},
		"not found": {&deploy.Error{Kind: deploy.KindNotFound, Status: 404,
			Message: "No such project."},
			http.StatusNotFound, "deploy_not_found", "No such project."},
		"conflict": {&deploy.Error{Kind: deploy.KindConflict, Status: 409,
			Message: "A deployment is already running."},
			http.StatusConflict, "deploy_conflict", "A deployment is already running."},
		"rejected": {&deploy.Error{Kind: deploy.KindRejected, Status: 422,
			Message: "That address answered, but not as the Panacea API."},
			http.StatusUnprocessableEntity, "deploy_rejected",
			"That address answered, but not as the Panacea API."},
	} {
		t.Run(name, func(t *testing.T) {
			status, body := deployErrWrite(t, tc.err)
			if status != tc.status {
				t.Errorf("status = %d, want %d", status, tc.status)
			}
			if body["error"] != tc.code {
				t.Errorf("error = %v, want %q", body["error"], tc.code)
			}
			if body["message"] != tc.message {
				t.Errorf("message = %v, want %q", body["message"], tc.message)
			}
			d := deployErrDetails(t, body)
			if d["upstream"] != "panacea" {
				t.Errorf("details.upstream = %v, want panacea", d["upstream"])
			}
			want := float64(tc.err.(*deploy.Error).Status)
			if d["status"] != want {
				t.Errorf("details.status = %#v, want the JSON number %v", d["status"], want)
			}
		})
	}
}

// TestWriteDeployErrorPlainErrorIsInternal keeps the table honest at its edge.
// deploy.KindOf answers KindUnavailable for any non-*Error, which is right for
// a caller deciding whether to retry and wrong here: a bug in this process is
// not Panacea being down, and telling a user to try again later hides it.
func TestWriteDeployErrorPlainErrorIsInternal(t *testing.T) {
	status, body := deployErrWrite(t, errors.New("a nil map in the control plane"))
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
	if body["error"] != "internal" {
		t.Errorf("error = %v, want internal", body["error"])
	}
	if _, ok := body["details"]; ok {
		t.Errorf("details = %v; a 500 names nothing", body["details"])
	}
}

// TestUpstreamUnavailableCarriesStatus is the row the desktop reads to tell
// "Panacea never answered, so your Ship may have started" from "Panacea refused
// before acting, so nothing happened". The status must be a JSON NUMBER:
// === 0 never matches "0".
func TestUpstreamUnavailableCarriesStatus(t *testing.T) {
	const (
		silent  = "Panacea did not answer, so the cockpit cannot tell whether it acted."
		garbled = "Panacea answered, but not in a form the cockpit can use, so it cannot tell what Panacea did."
		errored = "Panacea answered with an error. Look at the history before trying again."
		refused = "Panacea, or a proxy in front of it, refused the request before acting, so nothing was changed."
	)
	for _, tc := range []struct {
		status   int
		sentence string
	}{
		{0, silent}, {502, silent}, {504, silent},
		{202, garbled}, {503, errored}, {500, errored}, {429, refused}, {400, refused},
	} {
		t.Run("status "+strconv.Itoa(tc.status), func(t *testing.T) {
			code, body := deployErrWrite(t, &deploy.Error{Kind: deploy.KindUnavailable, Status: tc.status})
			if code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", code)
			}
			if body["error"] != "upstream_unavailable" {
				t.Errorf("error = %v, want upstream_unavailable", body["error"])
			}
			if body["message"] != tc.sentence {
				t.Errorf("message =\n\t%v\nwant\n\t%q", body["message"], tc.sentence)
			}
			d := deployErrDetails(t, body)
			if d["upstream"] != "panacea" || d["status"] != float64(tc.status) {
				t.Errorf("details = %#v, want upstream panacea and the number %d", d, tc.status)
			}
		})
	}

	t.Run("extra merges and cannot overwrite", func(t *testing.T) {
		_, body := deployErrWrite(t, &deploy.Error{Kind: deploy.KindUnavailable, Status: 0,
			RetryAfter: 30 * time.Second}, map[string]any{"reserved": true, "status": 7})
		d := deployErrDetails(t, body)
		if d["reserved"] != true {
			t.Errorf("details.reserved = %#v, want true", d["reserved"])
		}
		if d["status"] != float64(0) {
			t.Errorf("details.status = %#v, want the error's own 0", d["status"])
		}
		if d["retryAfter"] != float64(30) {
			t.Errorf("details.retryAfter = %#v, want 30", d["retryAfter"])
		}
	})
}
