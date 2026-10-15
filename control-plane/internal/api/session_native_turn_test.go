package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func nativeTurnRow(endpoint string) registry.Session {
	row := sessionsFullRecord()
	row.SandboxID = "owned-allocation"
	row.AttachToken = base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	row.Harness = "opencode"
	row.ImplementationContext = &registry.ImplementationContext{ProjectID: row.ProjectID, WorktreeID: row.WorktreeID, TaskIDs: []string{"orchard-task"}}
	row.Status = registry.StatusIdle
	row.Backend, row.Endpoint = "docker", endpoint
	return row
}

func TestSessionListProjectionNeverReadsNativeEndpoint(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()
	row := nativeTurnRow(strings.TrimPrefix(server.URL, "http://"))
	started := time.Now()
	for i := 0; i < 8; i++ {
		row.ID = "s-native-" + strconv.Itoa(i)
		row.ProjectID = "p-project-" + strconv.Itoa(i)
		row.ImplementationContext.ProjectID = row.ProjectID
		view := (&API{}).sessionJSON(row, sessionBasePath)
		if view.ImplementationOutcome == nil || view.ImplementationOutcome.State != "quiet" {
			t.Fatalf("list row = %+v", view.ImplementationOutcome)
		}
	}
	if time.Since(started) > 200*time.Millisecond || calls.Load() != 0 {
		t.Fatalf("list made %d native reads in %v", calls.Load(), time.Since(started))
	}
}

func TestSelectedNativeTurnIsOwnedCancellableAndGenerationBound(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/native-turn" || r.Header.Get("Authorization") != "Bearer "+base64.RawURLEncoding.EncodeToString(make([]byte, 32)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"state":"finished","toolClass":"reads-only","turnId":"msg_67d5b111"}`))
	}))
	defer server.Close()
	row := nativeTurnRow(strings.TrimPrefix(server.URL, "http://"))
	turn := observeNativeOpenCodeTurn(context.Background(), row)
	if turn == nil || turn.ToolClass != "reads-only" || calls.Load() != 1 {
		t.Fatalf("selected turn = %+v, calls=%d", turn, calls.Load())
	}
	for name, change := range map[string]func(*registry.Session){
		"restart":    func(s *registry.Session) { s.SandboxID = "next-allocation" },
		"new-output": func(s *registry.Session) { s.LastOutputAt = s.LastOutputAt.Add(time.Second) },
		"status":     func(s *registry.Session) { s.Status = registry.StatusExited },
		"endpoint":   func(s *registry.Session) { s.Endpoint = "127.0.0.1:1" },
	} {
		t.Run(name, func(t *testing.T) {
			next := row
			change(&next)
			if nativeTurnSnapshotMatches(row, next) {
				t.Fatal("stale turn survived changed execution")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if observeNativeOpenCodeTurn(ctx, row) != nil {
		t.Fatal("canceled caller obtained native result")
	}
	if calls.Load() != 1 {
		t.Fatal("canceled request reached native endpoint")
	}
}

func TestSelectedNativeTurnRouteReturnsUnknownWhenBusyAndNeverPollsList(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"state":"finished","toolClass":"reads-only","turnId":"msg_67d5b111"}`))
	}))
	defer server.Close()
	env := sessionsNewEnv(t)
	row := nativeTurnRow(strings.TrimPrefix(server.URL, "http://"))
	if err := env.st.Create(row); err != nil {
		t.Fatal(err)
	}
	if out := env.do(t, http.MethodGet, "/v1/sessions", ""); out.Code != http.StatusOK || calls.Load() != 0 {
		t.Fatalf("list status=%d native calls=%d", out.Code, calls.Load())
	}
	read := func() nativeOpenCodeTurn {
		t.Helper()
		out := env.do(t, http.MethodGet, "/v1/sessions/"+row.ID+"/native-turn", "")
		if out.Code != http.StatusOK {
			t.Fatalf("native status=%d body=%s", out.Code, out.Body.String())
		}
		var turn nativeOpenCodeTurn
		if err := json.Unmarshal(out.Body.Bytes(), &turn); err != nil {
			t.Fatal(err)
		}
		return turn
	}
	if turn := read(); turn.State != "finished" || turn.ExecutionGeneration != sessionExecutionGeneration(row) || turn.LastOutputAt != sessionTime(row.LastOutputAt) {
		t.Fatalf("selected route = %+v", turn)
	}
	nativeTurnReads <- struct{}{}
	nativeTurnReads <- struct{}{}
	defer func() { <-nativeTurnReads; <-nativeTurnReads }()
	if turn := read(); turn.State != "unknown" || calls.Load() != 1 {
		t.Fatalf("busy route = %+v, calls=%d", turn, calls.Load())
	}
}
