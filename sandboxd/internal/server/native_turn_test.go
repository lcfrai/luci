package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNativeTurnLiveProcessFinishedReadsOnly(t *testing.T) {
	now := time.Now().UnixMilli()
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic b3BlbmNvZGU6dGVzdC10b2tlbg==" {
			t.Error("upstream basic auth missing")
		}
		switch r.URL.Path {
		case "/session":
			_, _ = w.Write([]byte(`[{"id":"ses_native1"}]`))
		case "/session/ses_native1/message":
			if r.URL.RawQuery != "limit=64" {
				t.Error("native message page must be bounded")
			}
			_, _ = w.Write([]byte(`[{"info":{"id":"msg_assistant1","role":"assistant","finish":"stop","time":{"completed":` + jsonNumber(now) + `}},"parts":[{"type":"tool","tool":"read","state":{"status":"completed","input":{"filePath":"PRIVATE PATH"},"output":"PRIVATE OUTPUT"}},{"type":"text","text":"PRIVATE REPORT"}]},{"info":{"id":"msg_user1","role":"user"},"parts":[{"type":"text","text":"PRIVATE PROMPT"}]}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer peer.Close()
	s := &Server{cfg: Config{AttachToken: "test-token"}, nativeURL: peer.URL, startedAt: now - 1000}
	req := httptest.NewRequest(http.MethodGet, "/native-turn", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	out := httptest.NewRecorder()
	s.nativeTurn(out, req)
	if out.Code != http.StatusOK || !strings.Contains(out.Body.String(), `"toolClass":"reads-only"`) || !strings.Contains(out.Body.String(), `"turnId":"msg_assistant1"`) {
		t.Fatalf("native turn = %d %s", out.Code, out.Body.String())
	}
	for _, secret := range []string{"PRIVATE PROMPT", "PRIVATE PATH", "PRIVATE OUTPUT", "PRIVATE REPORT"} {
		if strings.Contains(out.Body.String(), secret) {
			t.Fatalf("native summary leaked %s", secret)
		}
	}
}

func jsonNumber(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestClassifyNativeTurnRefusesUnfinishedStaleAndPending(t *testing.T) {
	now := time.Now().UnixMilli()
	var user, assistant upstreamMessage
	user.Info.Role = "user"
	assistant.Info.ID = "msg_assistant1"
	assistant.Info.Role = "assistant"
	assistant.Info.Finish = "stop"
	assistant.Info.Time.Completed = now
	if got, ok := classifyNativeTurn([]upstreamMessage{user, assistant}, now-100); !ok || got.ToolClass != "none" {
		t.Fatalf("complete no-tool turn = %#v %v", got, ok)
	}
	if _, ok := classifyNativeTurn([]upstreamMessage{user, assistant}, now+10_000); ok {
		t.Fatal("old assistant turn was accepted for a new sandbox generation")
	}
	assistant.Info.Finish = "tool-calls"
	if _, ok := classifyNativeTurn([]upstreamMessage{user, assistant}, now-100); ok {
		t.Fatal("intermediate assistant turn was accepted as terminal")
	}
	assistant.Info.Finish = "stop"
	if err := json.Unmarshal([]byte(`[{"type":"tool","tool":"edit","state":{"status":"running"}}]`), &assistant.Parts); err != nil {
		t.Fatal(err)
	}
	assistant.Parts[0].Type, assistant.Parts[0].Tool, assistant.Parts[0].State.Status = "tool", "edit", "running"
	if _, ok := classifyNativeTurn([]upstreamMessage{user, assistant}, now-100); ok {
		t.Fatal("pending edit was accepted as completed")
	}
	assistant.Parts[0].State.Status = "completed"
	if got, ok := classifyNativeTurn([]upstreamMessage{user, assistant}, now-100); !ok || got.ToolClass != "effects-unverified" {
		t.Fatalf("completed edit = %#v %v", got, ok)
	}
	assistant.Parts[0].Type = "patch"
	assistant.Parts[0].Tool = ""
	if got, ok := classifyNativeTurn([]upstreamMessage{user, assistant}, now-100); !ok || got.ToolClass != "effects-unverified" {
		t.Fatalf("patch part was misclassified as no native effect: %#v %v", got, ok)
	}
}

func TestNativeTurnRequiresExactOwnershipAndBoundedRead(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"ses_first"},{"id":"ses_second"}]`))
	}))
	defer peer.Close()
	s := &Server{cfg: Config{AttachToken: "test-token"}, nativeURL: peer.URL, startedAt: time.Now().UnixMilli()}
	for _, tc := range []struct {
		token string
		want  int
	}{{"", 401}, {"Bearer wrong", 401}, {"Bearer test-token", 503}} {
		req := httptest.NewRequest(http.MethodGet, "/native-turn", nil)
		req.Header.Set("Authorization", tc.token)
		out := httptest.NewRecorder()
		s.nativeTurn(out, req)
		if out.Code != tc.want || out.Body.Len() != 0 {
			t.Fatalf("token %q: %d %q", tc.token, out.Code, out.Body.String())
		}
	}
	if nativeOpenCodeCommand("bash", []string{"--hostname", "127.0.0.1", "--port", "4096"}) || nativeOpenCodeCommand("opencode", []string{"--hostname", "0.0.0.0", "--port", "4096"}) || !nativeOpenCodeCommand("opencode", []string{"--hostname", "127.0.0.1", "--port", "4096"}) {
		t.Fatal("native observer was enabled for an unsupported process or address")
	}
}

func TestNativeTurnRefusesRedirectWithoutForwardingOwnedSecret(t *testing.T) {
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
	}))
	defer target.Close()
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer peer.Close()
	s := &Server{cfg: Config{AttachToken: "test-token"}, nativeURL: peer.URL, startedAt: time.Now().UnixMilli()}
	req := httptest.NewRequest(http.MethodGet, "/native-turn", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	out := httptest.NewRecorder()
	s.nativeTurn(out, req)
	if out.Code != http.StatusServiceUnavailable || redirected {
		t.Fatalf("redirect status=%d followed=%v", out.Code, redirected)
	}
}
