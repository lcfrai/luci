package api

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexAccountErrorsDistinguishGrantStorageAndUnconfirmedSave(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"incomplete grant", errCodexSignInIncomplete, 409, "codex_sign_in_incomplete"},
		{"still active", errCodexSignInPending, 409, "codex_sign_in_pending"},
		{"unreadable session", errCodexSignInUnreadable, 409, "codex_sign_in_state_unavailable"},
		{"unconfirmed save", errCodexSaveFailed, 500, "codex_account_save_failed"},
		{"canonical incomplete state", errCodexAuthIncomplete, 409, "codex_account_unavailable"},
		{"private failure", errors.New("private/path fixture-token raw-json"), 409, "codex_account_unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRecorder()
			codexAccountProblem(r, tt.err)
			if sessionsProblemCode(t, r, tt.status) != tt.code {
				t.Fatal("wrong error category")
			}
			for _, private := range []string{"private/path", "fixture-token", "raw-json"} {
				if strings.Contains(r.Body.String(), private) {
					t.Fatal("private error was exposed")
				}
			}
		})
	}
}

func TestCodexAccountAdoptionDistinguishesMissingIncompleteAndUnreadableState(t *testing.T) {
	for _, kind := range []string{"missing", "incomplete", "malformed", "directory"} {
		t.Run(kind, func(t *testing.T) {
			e := codexTestEnv(t)
			sessionsStubReady(t, nil)
			s, dir := codexTestSignIn(t, e, sessionsWatchDone(t))
			path := filepath.Join(dir, "auth.json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			want := "codex_sign_in_incomplete"
			switch kind {
			case "incomplete":
				nativeWrite(t, dir, "auth.json", `{"auth_mode":"chatgpt","tokens":{}}`)
			case "malformed":
				nativeWrite(t, dir, "auth.json", "fixture-invalid-private-json")
				want = "codex_sign_in_state_unavailable"
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				want = "codex_sign_in_state_unavailable"
			}
			r := e.do(t, "POST", codexAccountRoute, `{"sessionId":"`+s.ID+`"}`)
			if sessionsProblemCode(t, r, 409) != want {
				t.Fatal("wrong adoption failure")
			}
			if strings.Contains(r.Body.String(), dir) || strings.Contains(r.Body.String(), "fixture-invalid") {
				t.Fatal("private state exposed")
			}
			status := e.do(t, "GET", codexAccountRoute, "")
			if status.Code != 200 || strings.Contains(status.Body.String(), `"configured":true`) {
				t.Fatal("failed adoption reported a saved connection")
			}
		})
	}
}
