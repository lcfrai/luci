package dockerinfo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeUsesSelectedDockerTransportWithoutInfoFormatter(t *testing.T) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("Docker CLI is unavailable for its read-only selected-transport command")
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			// Docker CLI verifies its selected transport before proxying the
			// one application-level engine observation.
			w.WriteHeader(http.StatusOK)
			return
		}
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/info" || r.Host != "api.moby.localhost" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		// A daemon response can arrive after the request bytes; premature stdin
		// EOF has broken Docker Desktop's named-pipe dial-stdio transport.
		time.Sleep(25 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]any{"ID": "selected-private-engine", "ServerVersion": "28.5.1", "OSType": "linux", "MemTotal": int64(8 << 30), "NCPU": 4, "Swarm": map[string]string{"LocalNodeState": "inactive"}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	info, err := Probe(ctx, docker, "tcp://"+strings.TrimPrefix(server.URL, "http://"), []string{"PATH=" + os.Getenv("PATH"), "DOCKER_HOST=tcp://127.0.0.1:1"}, true)
	if err != nil || requests.Load() != 1 || info.ID != "selected-private-engine" || info.ServerVersion != "28.5.1" || info.MemTotal != 8<<30 || info.NCPU != 4 || info.Swarm.LocalNodeState != "inactive" {
		t.Fatalf("selected read-only Docker info failed: info=%+v calls=%d err=%v", info, requests.Load(), err)
	}
}

func TestSelectedEnvironmentDoesNotCarryUnrelatedSecrets(t *testing.T) {
	env := selectedEnvironment([]string{"PATH=/trusted", "DOCKER_HOST=tcp://foreign:2375", "OPENAI_API_KEY=secret", "DOCKER_CONTEXT=private-context"}, "tcp://selected:2375", true)
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "foreign") || !strings.Contains(joined, "DOCKER_HOST=tcp://selected:2375") || !strings.Contains(joined, "DOCKER_CONTEXT=private-context") {
		t.Fatal("selected transport environment lost its context or retained unrelated credentials")
	}
}
