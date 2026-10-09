package api

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func terminalFixtureSession(s *registry.Session) {
	s.ProjectID, s.WorktreeID, s.BranchID = "project-terminal", "worktree-terminal", "branch-terminal"
}

func TestAttachTypedExitPublishesCurrentRoutesWithoutReaper(t *testing.T) {
	f := attachNewFixture(t, nil, terminalFixtureSession)
	// Serve the production router, not just the test attach registration. No
	// reaper runs in this fixture: the former implementation never converges.
	srv := httptest.NewServer(f.api)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/sessions/"+f.id+"/attach", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	f.sandboxd.push([]byte("\x03private terminal detail must not become REST metadata"))
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	attachWaitFor(t, 5*time.Second, func() bool {
		s, err := f.store.Get(f.id)
		return err == nil && s.Status == registry.StatusExited
	}, "typed Exit did not publish terminal status without a background sweep")
	for _, path := range []string{"/v1/sessions/" + f.id, "/v1/sessions"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatal("terminal read failed", res.StatusCode, err)
		}
		var row sessionJSON
		if path == "/v1/sessions" {
			var value sessionListResponse
			if err := json.Unmarshal(body, &value); err != nil || len(value.Sessions) != 1 {
				t.Fatal("invalid terminal list", err)
			}
			row = value.Sessions[0]
		} else if err := json.Unmarshal(body, &row); err != nil {
			t.Fatal(err)
		}
		if row.Status != string(registry.StatusExited) || row.ExitReason != "process exited" || strings.Contains(string(body), "private terminal detail") {
			t.Fatal("terminal REST projection did not preserve static process-only semantics")
		}
	}
	res, err := http.Get(srv.URL + "/v1/sessions/" + f.id + "/attach")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusConflict || f.be.destroyCount() != 0 {
		t.Fatal("ended attach was not refused or destroyed allocation", res.StatusCode)
	}
	f.sandboxd.close <- websocket.StatusNormalClosure
	_, _, err = c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatal("terminal transport lost normal close", err)
	}
}
