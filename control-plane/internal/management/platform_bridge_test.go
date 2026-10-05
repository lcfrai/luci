package management

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedPlatformBridgeEncodedStorageRequiresPrivateBoundary(t *testing.T) {
	initial := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v1/platform/projects/p1/browser/data/stable-data/storage/v1/object/bucket/a%2Fb" || r.Header.Get("X-Luci-Preview-App-Authorization") != "Bearer application-user-token" {
			t.Error("encoded storage path or separated client authorization lost")
		}
		jsonReply(w, 200, map[string]bool{"ok": true})
	})
	_, handler, token, _ := fixture(t, initial)
	for _, tc := range []struct {
		marker, browser, authenticated bool
		code                           int
	}{{true, false, true, 200}, {true, true, true, 400}, {false, false, true, 400}, {true, false, false, 401}} {
		r := httptest.NewRequest("POST", "/v1/platform/projects/p1/browser/data/stable-data/storage/v1/object/bucket/a%2Fb", nil)
		if tc.marker {
			r.Header.Set("X-Luci-Preview-Bridge", "1")
		}
		if tc.browser {
			r.Header.Set("Sec-Fetch-Mode", "cors")
		}
		if tc.authenticated {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Luci-Preview-App-Authorization", "Bearer application-user-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("platform private boundary returned %d want %d", w.Code, tc.code)
		}
	}
}
