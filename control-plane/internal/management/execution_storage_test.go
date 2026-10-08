package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type executionStorageFixture struct {
	status   resourcebudget.LocalStorageStatus
	prepares atomic.Int32
	prepare  func()
	read     func()
}

func (f *executionStorageFixture) LocalStorageStatus() resourcebudget.LocalStorageStatus {
	if f.read != nil {
		f.read()
	}
	return f.status
}
func (f *executionStorageFixture) PrepareLocalStorageConfirmation(_ context.Context, path string) (resourcebudget.LocalStorageConfirmation, error) {
	f.prepares.Add(1)
	if f.prepare != nil {
		f.prepare()
	}
	return resourcebudget.LocalStorageConfirmation{Version: 1, EngineID: "controlled-engine", DesktopVersion: "Docker Desktop controlled fixture", WindowsSID: "S-1-5-21-100", VHDPath: path, FileIdentity: "controlled-file-identity", VolumeGUID: `\\?\Volume{11111111-2222-3333-4444-555555555555}\`, SettingsSHA256: strings.Repeat("a", 64)}, nil
}
func storageConfirmationBody(revision uint64) map[string]any {
	return map[string]any{"expectedRevision": revision, "vhdPath": `C:\Docker\data.vhdx`, "confirmActiveAndPendingLocal": true}
}
func attachExecutionStorageFixture(g *Gateway) *executionStorageFixture {
	f := &executionStorageFixture{status: resourcebudget.LocalStorageStatus{Platform: "windows", Scope: "local-docker-desktop", Required: true, State: "confirmation-required", ReasonCode: "confirmation_required"}}
	g.opts.ExecutionStorage = f
	return f
}

func TestExecutionStorageHTTPRequiresSuperAdminAndExplicitConsent(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	f := attachExecutionStorageFixture(g)
	for _, role := range []string{"member", "admin"} {
		addUser(t, h, token, "storage-"+role, role)
		other := loginAs(t, h, "storage-"+role)
		for _, method := range []string{"GET", "POST"} {
			w := request(t, h, method, "/v1/admin/execution-storage", other, storageConfirmationBody(0), 403)
			if strings.Contains(w.Body.String(), "vhdx") {
				t.Fatal("private path escaped administrator boundary")
			}
		}
	}
	request(t, h, "GET", "/v1/admin/execution-storage", "", nil, 401)
	body := storageConfirmationBody(0)
	body["confirmActiveAndPendingLocal"] = false
	request(t, h, "POST", "/v1/admin/execution-storage", token, body, 400)
	body = storageConfirmationBody(0)
	body["unexpected"] = "field"
	request(t, h, "POST", "/v1/admin/execution-storage", token, body, 400)
	if f.prepares.Load() != 0 {
		t.Fatal("unauthorized or unconfirmed request reached native probe")
	}
	before := request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200)
	if f.prepares.Load() != 0 || !strings.Contains(before.Body.String(), `"revision":0`) {
		t.Fatal("read-only GET performed a native probe")
	}
	if _, err := os.Stat(g.opts.ExecutionStateDir); !os.IsNotExist(err) {
		t.Fatal("GET created host state", err)
	}
	result := request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 200)
	var status resourcebudget.LocalStorageStatus
	if json.Unmarshal(result.Body.Bytes(), &status) != nil || status.Platform != "windows" || status.Scope != "local-docker-desktop" || status.State != "pending-observation" || status.Revision != 1 || status.HostAvailableBytes != nil {
		t.Fatal(result.Body.String())
	}
	saved, err := resourcebudget.ReadLocalStorageConfirmation(g.executionStoragePath())
	if err != nil || saved.ConfirmedBy != owner.ID || saved.Revision != 1 {
		t.Fatal("private publication lost host attribution", saved, err)
	}
	data, err := os.ReadFile(g.executionStoragePath())
	if err != nil {
		t.Fatal(err)
	}
	request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 409)
	after, err := os.ReadFile(g.executionStoragePath())
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("stale revision rewrote confirmation")
	}
	w := request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200)
	if !strings.Contains(w.Body.String(), `"state":"pending-observation"`) || strings.Contains(w.Body.String(), saved.WindowsSID) || strings.Contains(w.Body.String(), saved.SettingsSHA256) {
		t.Fatal("GET exposed private pins or claimed an unobserved confirmation", w.Body.String())
	}
	audit := request(t, h, "GET", "/v1/admin/audit", token, nil, 200).Body.String()
	if !strings.Contains(audit, "execution-storage.confirm") || !strings.Contains(audit, "succeeded") || strings.Contains(audit, "vhdx") {
		t.Fatal("audit attribution missing or leaking operator path")
	}
	// Corrupt state is preserved for diagnosis, never treated as revision zero.
	if err := os.WriteFile(g.executionStoragePath(), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 409)
	if after, err := os.ReadFile(g.executionStoragePath()); err != nil || string(after) != "{}" {
		t.Fatal("corrupt configuration silently replaced")
	}
}

func TestExecutionStorageHTTPFallbackAndUnavailableScopesMatchClient(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	w := request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200)
	var status resourcebudget.LocalStorageStatus
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || status.Scope != "not-required" || status.Required || status.State != "not-required" {
		t.Fatal("unsupported backend advertised the wrong wire contract", w.Body.String())
	}
	f := attachExecutionStorageFixture(g)
	f.status.State, f.status.ReasonCode, f.status.Message = "unavailable", "runtime_unavailable", "Start Docker Desktop."
	w = request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200)
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || status.Scope != "local-docker-desktop" || status.State != "unavailable" || status.ReasonCode != "runtime_unavailable" {
		t.Fatal("missing confirmation hid the unavailable runtime", w.Body.String())
	}
}

func TestExecutionStorageHTTPNeverReusesMeasurementsAcrossConfirmationRevisions(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	f := attachExecutionStorageFixture(g)
	available := uint64(1 << 30)
	f.status.State = "confirmed"
	f.status.ObservedAt = time.Now().UTC()
	f.status.HostAvailableBytes, f.status.GuestAvailableBytes = &available, &available
	f.status.MountedVolumeCount, f.status.LimitingVolume = 2, "C:"
	check := func(w *httptest.ResponseRecorder, want string) {
		t.Helper()
		var status resourcebudget.LocalStorageStatus
		if json.Unmarshal(w.Body.Bytes(), &status) != nil || status.State != want || status.HostAvailableBytes != nil || status.GuestAvailableBytes != nil || status.MountedVolumeCount != 0 || status.LimitingVolume != "" || !status.ObservedAt.IsZero() {
			t.Fatal("old measurement crossed its confirmation revision", w.Body.String())
		}
	}
	check(request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200), "confirmation-required")
	request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 200)
	check(request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200), "pending-observation")
}

func TestExecutionStorageRechecksAuthorityAfterNativeProbe(t *testing.T) {
	for _, change := range []string{"downgraded", "disabled", "logout"} {
		t.Run(change, func(t *testing.T) {
			g, h, ownerToken, _ := fixture(t, nil)
			f := attachExecutionStorageFixture(g)
			operator := addUser(t, h, ownerToken, "operator", "super-admin")
			token := loginAs(t, h, "operator")
			f.prepare = func() {
				switch change {
				case "downgraded":
					request(t, h, "PATCH", "/v1/admin/users/"+operator.ID, ownerToken, map[string]string{"role": "admin"}, 200)
				case "disabled":
					request(t, h, "PATCH", "/v1/admin/users/"+operator.ID, ownerToken, map[string]bool{"disabled": true}, 200)
				case "logout":
					request(t, h, "POST", "/v1/auth/logout", token, map[string]any{}, 200)
				}
			}
			request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 403)
			if _, err := os.Stat(g.executionStoragePath()); !os.IsNotExist(err) {
				t.Fatal("revoked operator persisted host confirmation", err)
			}
		})
	}
}

func TestExecutionStorageReadRechecksAuthorityBeforeReturningPrivatePath(t *testing.T) {
	g, h, ownerToken, _ := fixture(t, nil)
	f := attachExecutionStorageFixture(g)
	operator := addUser(t, h, ownerToken, "operator", "super-admin")
	token := loginAs(t, h, "operator")
	request(t, h, "POST", "/v1/admin/execution-storage", ownerToken, storageConfirmationBody(0), 200)
	f.read = func() {
		request(t, h, "PATCH", "/v1/admin/users/"+operator.ID, ownerToken, map[string]string{"role": "admin"}, 200)
	}
	w := request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 403)
	if strings.Contains(w.Body.String(), "vhdx") {
		t.Fatal("revoked operator read private path")
	}
}

func TestExecutionStorageAuditFailurePreventsConfirmation(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	attachExecutionStorageFixture(g)
	before := cloneIdentityState(g.state)
	blockIdentityCommit(t, g)
	request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 500)
	assertIdentityUnchanged(t, g, before)
	if _, err := os.Stat(g.executionStoragePath()); !os.IsNotExist(err) {
		t.Fatal("unattributed confirmation was saved", err)
	}
}

func TestExecutionStorageCASAcrossGateways(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	f := attachExecutionStorageFixture(g)
	other, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close(context.Background())
	other.opts.ExecutionStorage = f
	responses := make(chan *httptest.ResponseRecorder, 2)
	data, _ := json.Marshal(storageConfirmationBody(0))
	for _, handler := range []http.Handler{h, other.Handler()} {
		go func(h http.Handler) {
			r := httptest.NewRequest("POST", "/v1/admin/execution-storage", bytes.NewReader(data))
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			responses <- w
		}(handler)
	}
	counts := map[int]int{}
	for range 2 {
		result := <-responses
		counts[result.Code]++
		if result.Code != 200 && result.Code != 409 {
			t.Fatal(result.Code, result.Body.String())
		}
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("CAS published two competing confirmations", counts)
	}
	record, err := resourcebudget.ReadLocalStorageConfirmation(g.executionStoragePath())
	if err != nil || record.Revision != 1 {
		t.Fatal(record, err)
	}
}

// The browser imports this exact golden too. Compare the complete emitted HTTP
// document, not a second hand-written projection of the status DTO. Only the
// server-assigned confirmation timestamp is normalized for repeatability.
func TestExecutionStorageHTTPMatchesSharedClientFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/execution-storage-http.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]map[string]any
	if err = json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	check := func(name string, w *httptest.ResponseRecorder) {
		t.Helper()
		var actual map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &actual); err != nil {
			t.Fatal(err)
		}
		if stamp, exists := actual["confirmedAt"]; exists {
			text, ok := stamp.(string)
			parsed, err := time.Parse(time.RFC3339Nano, text)
			if !ok || err != nil || parsed.IsZero() {
				t.Fatal("HTTP confirmation has an invalid timestamp", stamp)
			}
			actual["confirmedAt"] = "2026-10-08T00:00:00Z"
		}
		if !reflect.DeepEqual(actual, expected[name]) {
			t.Fatalf("%s HTTP contract changed: actual=%s expected=%v", name, w.Body.String(), expected[name])
		}
	}
	g, h, token, _ := fixture(t, nil)
	f := attachExecutionStorageFixture(g)
	f.status = resourcebudget.LocalStorageStatus{Platform: "linux", Scope: "not-required", State: "not-required", ReasonCode: "not_required", Message: "This host does not require Windows backing-storage confirmation.", Basis: resourcebudget.LocalStorageBasis}
	check("notRequired", request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200))
	check("pendingObservation", request(t, h, "POST", "/v1/admin/execution-storage", token, storageConfirmationBody(0), 200))
	host, guest := uint64(0), uint64(1<<30)
	stamp, _ := time.Parse(time.RFC3339, "2026-10-08T00:00:00Z")
	f.status = resourcebudget.LocalStorageStatus{Platform: "windows", Scope: "local-docker-desktop", Required: true, State: "confirmed", Revision: 1, Message: "Local layout is operator-confirmed. Physical headroom is the lowest available space across every mounted writable local volume, including unrelated drives; guest capacity is measured separately.", Basis: resourcebudget.LocalStorageBasis, ObservedAt: stamp, DesktopVersion: "Docker Desktop controlled fixture", MountedVolumeCount: 2, LimitingVolume: "C:", HostAvailableBytes: &host, GuestAvailableBytes: &guest}
	check("confirmed", request(t, h, "GET", "/v1/admin/execution-storage", token, nil, 200))
}
