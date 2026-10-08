package dockerbe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// The narrowed interface deliberately lacks Info: an old agent-only backend
// cannot claim it enforces Docker RAM/swap simply because Create exists.
type checkProbeWithoutInfo struct{ APIClient }

func TestWorkerCPUAndProcessLimitsRefuseBeforeCreate(t *testing.T) {
	for _, capability := range []string{"period", "quota", "pids"} {
		t.Run(capability, func(t *testing.T) {
			b, f, _ := newCheckProbe(t)
			switch capability {
			case "period":
				f.info.CPUCfsPeriod = false
			case "quota":
				f.info.CPUCfsQuota = false
			case "pids":
				f.info.PidsLimit = false
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			spec := backend.Spec{ID: "cap-worker", SessionID: "cap-session", Image: "fixture:image", Command: "codex", AttachToken: "fixture-token", WorkDir: "/workspace", Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}}
			_, err := b.Create(ctx, spec)
			if !errors.Is(err, backend.ErrResourceLimitsUnsupported) || len(f.creates)+len(f.starts)+len(f.removes) != 0 {
				t.Fatalf("unsupported %s reached allocation: error=%v creates=%d starts=%d removes=%d", capability, err, len(f.creates), len(f.starts), len(f.removes))
			}
		})
	}
}

func TestWorkerZeroMemoryStillRequiresProcessLimitIdentity(t *testing.T) {
	b, f, _ := newCheckProbe(t)
	f.info.MemoryLimit, f.info.SwapLimit, f.info.CPUCfsPeriod, f.info.CPUCfsQuota = false, false, false, false
	if err := b.ValidateResourceLimits(context.Background(), backend.Resources{}); err != nil {
		t.Fatal("unrequested RAM/CPU ceilings must not be required", err)
	}
	f.info.PidsLimit = false
	if err := b.ValidateResourceLimits(context.Background(), backend.Resources{}); !errors.Is(err, backend.ErrResourceLimitsUnsupported) {
		t.Fatal("the always-requested process ceiling was ignored", err)
	}
	if len(f.creates)+len(f.starts)+len(f.removes) != 0 {
		t.Fatal("capability observation allocated")
	}
}

func TestCheckResourceCapabilitiesRefuseBeforeMutatingRPC(t *testing.T) {
	for _, name := range []string{"missing-info", "no-memory", "no-swap", "no-cpu-period", "no-cpu-quota", "no-pids", "bad-engine", "negative-memory", "negative-cpu"} {
		t.Run(name, func(t *testing.T) {
			b, f, spec := newCheckProbe(t)
			switch name {
			case "missing-info":
				b.api = checkProbeWithoutInfo{f}
			case "no-memory":
				f.info.MemoryLimit = false
			case "no-swap":
				f.info.SwapLimit = false
			case "no-cpu-period":
				f.info.CPUCfsPeriod = false
			case "no-cpu-quota":
				f.info.CPUCfsQuota = false
			case "no-pids":
				f.info.PidsLimit = false
			case "negative-cpu":
				spec.Resources.CPUMillis = -1
			case "bad-engine":
				f.info.ID = ""
			case "negative-memory":
				spec.Resources.MemoryMiB = -1
			}
			if err := b.ValidateResourceLimits(context.Background(), spec.Resources); err == nil {
				t.Fatal("unsupported limits reported as supported")
			}
			if len(f.creates)+len(f.starts)+len(f.removes) != 0 {
				t.Fatal("read-only capability check mutated the backend")
			}
		})
	}
}

func TestCheckCancelledPreparationDoesNotCreate(t *testing.T) {
	b, f, spec := newCheckProbe(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.PrepareCheck(ctx, spec); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatal("cancelled preparation was accepted", err)
	}
	if len(f.creates)+len(f.starts)+len(f.removes) != 0 {
		t.Fatal("pre-cancelled call emitted a mutation")
	}
}

func TestWorkerCreatedCapsAreCheckedBeforeStart(t *testing.T) {
	for _, name := range []string{"memory", "swap", "cpu", "pid", "foreign-reference", "engine-replaced"} {
		t.Run(name, func(t *testing.T) {
			b, f, _ := newCheckProbe(t)
			spec := backend.Spec{ID: "s-cap-check", SessionID: "session-cap-check", Image: "fixture:image", Command: "codex", AttachToken: "test-only-token", WorkDir: "/workspace", Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}}
			f.inspectHook = func(_ string, row container.InspectResponse) (container.InspectResponse, error) {
				switch name {
				case "memory":
					row.HostConfig.Memory = 0
				case "swap":
					row.HostConfig.MemorySwap = -1
				case "cpu":
					row.HostConfig.NanoCPUs = 0
				case "pid":
					row.HostConfig.PidsLimit = nil
				case "foreign-reference":
					row.ID = strings.Repeat("f", 64)
				}
				return row, nil
			}
			if name == "engine-replaced" {
				f.infoHook = func(n int, row *system.Info) {
					if n >= 3 {
						row.ID = "replacement-engine"
					}
				}
			}
			if _, err := b.Create(context.Background(), spec); err == nil {
				t.Fatal("invalid post-Create limits were accepted")
			}
			if len(f.creates) != 1 || len(f.starts) != 0 {
				t.Fatal("worker started before actual caps were proved", f.creates, f.starts)
			}
			if name == "foreign-reference" || name == "engine-replaced" {
				if len(f.removes) != 0 {
					t.Fatal("unknown owner/engine was cleaned implicitly")
				}
			} else if len(f.removes) != 1 || f.removes[0] != strings.Repeat("0", 63)+"1" {
				t.Fatal("known owned invalid allocation was not cleaned by immutable ID", f.removes)
			}
		})
	}
}

type checkWireTransport func(*http.Request) (*http.Response, error)

func (fn checkWireTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

// Exercise the real pinned Docker SDK's serialized requests through an in-memory
// HTTP transport. This is a wire contract check, not daemon/container execution.
func TestCheckSDKWirePinsImageLimitsAndImmutableLifecycle(t *testing.T) {
	_, f, spec := newCheckProbe(t)
	var calls []string
	var createWire map[string]any
	var removedRef string
	transport := checkWireTransport(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		status := http.StatusOK
		var value any
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		switch {
		case strings.HasSuffix(req.URL.Path, "/_ping"):
			return &http.Response{StatusCode: 200, Header: http.Header{"Api-Version": []string{"1.51"}}, Body: io.NopCloser(strings.NewReader("OK")), Request: req}, nil
		case req.Method == "GET" && req.URL.Path == "/v1.51/info":
			value = f.info
		case req.Method == "GET" && req.URL.Path == "/v1.51/images/fixture:image/json":
			value = f.image
		case req.Method == "POST" && req.URL.Path == "/v1.51/containers/create":
			raw, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err = json.Unmarshal(raw, &createWire); err != nil {
				return nil, err
			}
			var body struct {
				container.Config
				HostConfig *container.HostConfig
			}
			if err = json.Unmarshal(raw, &body); err != nil {
				return nil, err
			}
			if req.URL.Query().Get("name") != "luci-check-allocation-one" {
				t.Errorf("wrong Create name: %s", req.URL.RawQuery)
			}
			value, err = f.ContainerCreate(req.Context(), &body.Config, body.HostConfig, nil, nil, req.URL.Query().Get("name"))
			if err != nil {
				return nil, err
			}
			status = 201
		case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/json") && strings.HasPrefix(req.URL.Path, "/v1.51/containers/"):
			ref := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1.51/containers/"), "/json")
			row, err := f.ContainerInspect(req.Context(), ref)
			if err != nil {
				status = 404
				value = map[string]string{"message": "controlled exact absence"}
			} else {
				value = row
			}
		case req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/start"):
			ref := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1.51/containers/"), "/start")
			if err := f.ContainerStart(req.Context(), ref, container.StartOptions{}); err != nil {
				return nil, err
			}
			status = 204
		case req.Method == "DELETE" && strings.HasPrefix(req.URL.Path, "/v1.51/containers/"):
			removedRef = strings.TrimPrefix(req.URL.Path, "/v1.51/containers/")
			if req.URL.Query().Get("force") != "1" {
				t.Errorf("explicit forced cancellation was lost")
			}
			if err := f.ContainerRemove(req.Context(), removedRef, container.RemoveOptions{Force: true}); err != nil {
				return nil, err
			}
			status = 204
		default:
			t.Errorf("unexpected SDK RPC: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected SDK RPC")
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(raw))), Request: req}, nil
	})
	sdk, err := client.NewClientWithOpts(client.WithHost(f.endpoint), client.WithVersion("1.51"), client.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Close()
	b, err := New(Config{Instance: "check-instance", ResourceImage: "fixture:image", Client: sdk})
	if err != nil {
		t.Fatal(err)
	}
	p, ref := prepareAndCreateCheck(t, b, spec)
	if createWire["Image"] != f.image.ID || !reflect.DeepEqual(createWire["Entrypoint"], []any{"node"}) || !reflect.DeepEqual(createWire["Cmd"], []any{"verify.mjs", "final"}) || createWire["User"] != "1000:1000" {
		t.Fatal("wire image/argv/user differs", createWire)
	}
	host, ok := createWire["HostConfig"].(map[string]any)
	if !ok {
		t.Fatal("missing wire HostConfig")
	}
	for key, want := range map[string]float64{"Memory": 1 << 30, "MemorySwap": 1 << 30, "NanoCpus": 1e9, "PidsLimit": 256} {
		if host[key] != want {
			t.Fatalf("wire %s=%v, want %v", key, host[key], want)
		}
	}
	if host["AutoRemove"] != false || host["ReadonlyRootfs"] != true || host["Privileged"] != false {
		t.Fatal("wire isolation changed", host)
	}
	labels := createWire["Labels"].(map[string]any)
	if labels["luci.check"] != "true" || labels["luci.check-engine"] != f.info.ID || labels["luci.check-command-digest"] != spec.Identity.CommandDigest {
		t.Fatal("wire ownership binding changed", labels)
	}
	for _, key := range []string{backend.LabelManaged, backend.LabelSessionID, backend.LabelSandboxID} {
		if _, ok := labels[key]; ok {
			t.Fatal("check masqueraded as an agent", key)
		}
	}
	if err = b.StartCheck(context.Background(), p.Spec.Identity, ref); err != nil {
		t.Fatal(err)
	}
	if err = b.RemoveCheck(context.Background(), p.Spec.Identity, ref, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.starts, []string{ref}) || removedRef != ref || len(f.rows) != 0 {
		t.Fatal("lifecycle escaped its immutable reference", calls)
	}
	for _, call := range calls {
		if strings.Contains(call, "/images/create") || strings.HasPrefix(call, "DELETE /v1.51/containers/luci-") {
			t.Fatal("hidden pull/name cleanup", call)
		}
	}
}
