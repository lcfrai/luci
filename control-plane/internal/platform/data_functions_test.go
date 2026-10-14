package platform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These deterministic fixtures exercise admission, durable state, exact owned
// mount/engine proofs and route recovery. Actual execution is independently
// covered by TestDataFunctionPinnedRuntimeEvaluation.
type functionFixture struct {
	s                     *Service
	f                     *dataFake
	i                     dataInstallation
	source                DataFunctionSource
	engine                string
	foreignMount          bool
	env                   []string
	functionUps, probes   int
	sourceReads, invokes  int
	failProbe             bool
	failPromotionReadback bool
	reply                 string
	replyStatus           int
}

func newFunctionFixture(t *testing.T) *functionFixture {
	t.Helper()
	s, f := newDataFake(t)
	x := &functionFixture{s: s, f: f, engine: "owned-native-engine", reply: `{"ok":true}`, replyStatus: 200}
	x.source = DataFunctionSource{Name: "example", Path: "supabase/functions/example", AuthMode: "project-key", Commit: strings.Repeat("a", 40), Files: []DataFunctionFile{{Path: "index.ts", Content: "export default {fetch:()=>Response.json({version:1})};\n"}}}
	x.source.SourceDigest = DataFunctionDigest(x.source.AuthMode, x.source.Files)
	s.opts.FunctionSources = &DataFunctionSources{
		List: func(context.Context, string) ([]DataFunctionSource, error) {
			return []DataFunctionSource{x.source}, nil
		},
		Read: func(context.Context, string, string) (DataFunctionSource, error) { return x.source, nil },
		Save: func(_ context.Context, id, name string, in DataFunctionSave) (DataFunctionSource, error) {
			x.sourceReads++
			if in.ExpectedDigest != x.source.SourceDigest {
				return DataFunctionSource{}, errors.New("stale source")
			}
			return x.source, nil
		},
		Committed: func(_ context.Context, id, name, sha string) (DataFunctionSource, error) {
			x.sourceReads++
			if id != "owned" || name != "example" || sha != x.source.Commit {
				return DataFunctionSource{}, errors.New("foreign source")
			}
			return x.source, nil
		},
	}
	s.opts.Runner = x.run
	// Synchronous installation keeps this fixture independent of the old
	// three-second browser polling helper and Windows asset-write timing.
	if err := s.changeData("owned", func(v *DataStatus) { v.Operation = &DataOperation{Status: "running"} }); err != nil {
		t.Fatal(err)
	}
	if err := s.runDataAction(context.Background(), "owned", dataActionRequest{Action: "install", AcknowledgeResources: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.changeData("owned", func(v *DataStatus) { v.Operation.Status = "succeeded" }); err != nil {
		t.Fatal(err)
	}
	if err := s.read("owned-data-installation", &x.i); err != nil {
		t.Fatal(err)
	}
	s.opts.HTTPClient = &http.Client{Transport: dataTransport(x.transport)}
	return x
}

func (x *functionFixture) run(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
	if len(args) > 2 && args[0] == "info" && args[2] == "{{json .}}" {
		return json.Marshal(map[string]any{"ID": x.engine, "OSType": "linux", "MemTotal": int64(16 << 30), "NCPU": 8, "Swarm": map[string]string{"LocalNodeState": "inactive"}})
	}
	if len(args) == 2 && args[0] == "inspect" && strings.HasSuffix(args[1], "-functions") {
		x.f.mu.Lock()
		c, exists := x.f.containers[args[1]]
		x.f.mu.Unlock()
		if !exists {
			return nil, errors.New("missing runtime")
		}
		var i dataInstallation
		if err := x.s.read("owned-data-installation", &i); err != nil {
			return nil, err
		}
		root, _ := x.s.dataRoot(i)
		mount := filepath.Join(root, "volumes", "functions")
		if x.foreignMount {
			mount = filepath.Join(root, "foreign-functions")
		}
		b, _ := json.Marshal([]any{map[string]any{
			"Config":          map[string]any{"Image": c.Config.Image, "Labels": c.Config.Labels, "Env": x.env},
			"State":           c.State,
			"Mounts":          []any{map[string]any{"Source": mount, "Destination": "/home/deno/functions", "RW": false}},
			"NetworkSettings": map[string]any{"Networks": map[string]any{x.s.resource(i.ProjectID, i.StackID+"-net"): map[string]any{"NetworkID": "owned-network-id"}}},
		}})
		return b, nil
	}
	if len(args) == 3 && args[0] == "network" && args[1] == "inspect" {
		x.f.mu.Lock()
		labels, exists := x.f.networks[args[2]]
		x.f.mu.Unlock()
		if !exists {
			return nil, errors.New("missing network")
		}
		b, _ := json.Marshal([]any{map[string]any{"ID": "owned-network-id", "Labels": labels}})
		return b, nil
	}
	b, err := x.f.run(ctx, exe, args, dir, input)
	if err == nil && args[0] == "compose" && strings.Contains(" "+strings.Join(args, " ")+" ", " up ") {
		for n, arg := range args {
			if arg == "-f" {
				values, _ := os.ReadFile(filepath.Join(filepath.Dir(args[n+1]), "runtime.env"))
				x.env = strings.Split(strings.TrimSpace(string(values)), "\n")
				x.env = append(x.env, "LUCI_FUNCTION_RUNTIME_VERSION="+dataFunctionRuntimeVersion)
			}
		}
		if args[len(args)-1] == "functions" {
			x.functionUps++
			if !strings.Contains(" "+strings.Join(args, " ")+" ", " --no-deps ") {
				return nil, errors.New("unexpected whole-stack restart")
			}
		}
	}
	return b, err
}

func (x *functionFixture) manifest() (functionManifest, []byte, error) {
	root, err := x.s.dataRoot(x.i)
	if err != nil {
		return functionManifest{}, nil, err
	}
	b, err := os.ReadFile(filepath.Join(root, "volumes", "functions", "_luci", "routes.json"))
	var v functionManifest
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	return v, b, err
}
func (x *functionFixture) transport(r *http.Request) (*http.Response, error) {
	secret, err := x.s.dataCredentials(x.i)
	if err != nil {
		return nil, err
	}
	if r.Header.Get("apikey") != secret.Anon || r.Header.Get("Authorization") == "Bearer "+secret.ServiceRole {
		return nil, errors.New("private credentials forwarded")
	}
	status, value := x.replyStatus, x.reply
	if strings.HasPrefix(r.URL.Path, "/functions/v1/_luci-") {
		v, err := x.s.loadDataFunctions("owned")
		if err != nil || r.Header.Get("X-Luci-Function-Probe") != v.ProbeToken || v.ProbeToken == "" {
			return nil, errors.New("missing trusted nonce")
		}
		manifest, raw, err := x.manifest()
		if err != nil {
			return nil, err
		}
		status = 200
		if strings.HasSuffix(r.URL.Path, "_luci-health") {
			digest := dataSum(raw)
			if x.failPromotionReadback && v.Pending != nil && manifest.Routes["example"].ReleaseID == v.Pending.After["example"] {
				x.failPromotionReadback = false
				digest = "unverified"
			}
			b, _ := json.Marshal(map[string]string{"runtimeVersion": dataFunctionRuntimeVersion, "manifestDigest": digest})
			value = string(b)
		} else {
			x.probes++
			var check map[string]string
			if json.NewDecoder(r.Body).Decode(&check) != nil {
				return nil, errors.New("invalid probe")
			}
			candidate := manifest.Candidates[check["releaseId"]]
			if candidate.SourceDigest != check["sourceDigest"] {
				return nil, errors.New("foreign candidate")
			}
			b, _ := json.Marshal(map[string]string{"runtimeVersion": dataFunctionRuntimeVersion, "releaseId": candidate.ReleaseID, "sourceDigest": candidate.SourceDigest})
			value = string(b)
			if x.failProbe {
				status, value = 503, `{"error":"function_boot_failed"}`
			}
		}
	} else {
		x.invokes++
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(value))}, nil
}

func (x *functionFixture) action(t *testing.T, action string, customize func(*DataFunctionAction)) dataFunctionState {
	t.Helper()
	v, err := x.s.loadDataFunctions("owned")
	if err != nil {
		t.Fatal(err)
	}
	in := DataFunctionAction{Action: action, ExpectedRevision: v.Revision, Commit: x.source.Commit, AcknowledgeExecute: true, AcknowledgeRemove: true}
	if customize != nil {
		customize(&in)
	}
	w := request(t, x.s, "POST", "/v1/platform/projects/owned/data/functions/example/action", "admin", in)
	if w.Code != 202 {
		t.Fatalf("%s: %d %s", action, w.Code, w.Body)
	}
	x.s.wg.Wait()
	v, err = x.s.loadDataFunctions("owned")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestDataFunctionsSourceBeforeInstallationAndSelectedWorkspacePermissions(t *testing.T) {
	s, f := newDataFake(t)
	s.opts.FunctionSources = &DataFunctionSources{
		List: func(context.Context, string) ([]DataFunctionSource, error) { return []DataFunctionSource{}, nil },
		Read: func(context.Context, string, string) (DataFunctionSource, error) {
			return DataFunctionSource{}, errors.New("not found")
		},
		Save: func(context.Context, string, string, DataFunctionSave) (DataFunctionSource, error) {
			return DataFunctionSource{Name: "example"}, nil
		},
	}
	w := request(t, s, "GET", "/v1/platform/projects/owned/data/functions", "member", nil)
	var catalog DataFunctionCatalog
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil || catalog.CanManage || !catalog.CustomPublishing || len(catalog.Functions) != 1 || !catalog.Functions[0].Builtin {
		t.Fatal("read-only preinstall catalogue", w.Code, w.Body)
	}
	for _, tail := range []string{"source", "action"} {
		w = request(t, s, "POST", "/v1/platform/projects/owned/data/functions/example/"+tail, "member", map[string]any{})
		if w.Code != 403 {
			t.Fatal("workspace member manages source", w.Code)
		}
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/functions/example/source", "admin", DataFunctionSave{AuthMode: "authenticated", Files: []DataFunctionFile{{"index.ts", "export default {};"}}})
	if w.Code != 200 {
		t.Fatal("source editing depends on installed Data", w.Code, w.Body)
	}
	if len(f.calls) != 0 {
		t.Fatal("source metadata allocated resources")
	}
}

func TestDataFunctionsPublishFailureRecoveryRollbackRemoveAndRestoreIdentity(t *testing.T) {
	x := newFunctionFixture(t)
	v := x.action(t, "publish", nil)
	first := v.Active["example"]
	if v.Operation.Status != "succeeded" || first == "" || x.probes != 1 || x.invokes != 0 || x.functionUps != 1 {
		t.Fatalf("publication did not prove compile-only adoption: %+v", v.Operation)
	}
	x.source.Files[0].Content = "export default {fetch:()=>Response.json({version:2})};"
	x.source.SourceDigest = DataFunctionDigest(x.source.AuthMode, x.source.Files)
	x.failProbe = true
	v = x.action(t, "publish", nil)
	if v.Operation.Status != "failed" || v.Active["example"] != first || v.Pending != nil || v.Releases["example"][1].Status != "failed" {
		t.Fatal("failed probe replaced verified endpoint")
	}
	x.failProbe = false
	x.failPromotionReadback = true
	v = x.action(t, "publish", nil)
	manifest, _, err := x.manifest()
	if err != nil || v.Operation.Status != "failed" || v.Pending != nil || manifest.Routes["example"].ReleaseID != first || v.Active["example"] != first {
		t.Fatal("failed promotion did not restore and read back prior route", err)
	}
	// A merely retained candidate must never become a rollback target.
	candidate := v.Releases["example"][2]
	w := request(t, x.s, "POST", "/v1/platform/projects/owned/data/functions/example/action", "admin", DataFunctionAction{Action: "rollback", ExpectedRevision: v.Revision, ReleaseID: candidate.ReleaseID, AcknowledgeExecute: true})
	if w.Code != 409 {
		t.Fatal("unverified candidate admitted for rollback", w.Code)
	}
	v = x.action(t, "publish", nil)
	second := v.Active["example"]
	if v.Operation.Status != "succeeded" || first == second || x.functionUps != 1 || v.Releases["example"][2].Status != "candidate" {
		t.Fatal("new immutable release or candidate status wrong")
	}
	v = x.action(t, "rollback", func(in *DataFunctionAction) { in.ReleaseID = first })
	if v.Operation.Status != "succeeded" || v.Active["example"] != first {
		t.Fatal("retained release rollback failed")
	}
	// Data restore uses another stack with the durable tenant and real retained
	// source copies. The same release is regenerated rather than a hello-only reset.
	restored := x.i
	restored.StackID = "stack-restored"
	restored.StorageTenantID = x.i.storageTenant()
	secret, _ := x.s.dataCredentials(x.i)
	if err = x.s.prepareDataStack(restored, secret); err != nil {
		t.Fatal(err)
	}
	root, _ := x.s.dataRoot(restored)
	b, err := os.ReadFile(filepath.Join(root, "volumes", "functions", "_luci", "routes.json"))
	var restoredManifest functionManifest
	if err != nil || json.Unmarshal(b, &restoredManifest) != nil || restoredManifest.Routes["example"].ReleaseID != first {
		t.Fatal("restore lost stable function identity", err)
	}
	b, err = os.ReadFile(filepath.Join(root, "volumes", "functions", "_luci", "releases", first, "index.ts"))
	if err != nil || !strings.Contains(string(b), "version:1") {
		t.Fatal("restore lost immutable source", err)
	}
	v = x.action(t, "remove", nil)
	manifest, _, _ = x.manifest()
	if v.Operation.Status != "succeeded" || v.Active["example"] != "" || manifest.Routes["example"].ReleaseID != "" {
		t.Fatal("endpoint removal failed")
	}
	r, ok := functionRelease(v, "example", first)
	if !ok || r.Status != "stopped" {
		t.Fatal("removal lost source history")
	}
	if _, err = x.s.readFunctionArtifact("owned", r); err != nil {
		t.Fatal("removal deleted retained source", err)
	}
}

func TestDataFunctionAdmissionRejectsChangedEngineMountRevisionAndMissingAcknowledgement(t *testing.T) {
	x := newFunctionFixture(t)
	path := "/v1/platform/projects/owned/data/functions/example/action"
	in := DataFunctionAction{Action: "publish", Commit: x.source.Commit, AcknowledgeExecute: true}
	for _, test := range []struct {
		name   string
		mutate func()
		reset  func()
		in     DataFunctionAction
		code   int
	}{
		{"execute acknowledgement", func() {}, func() {}, DataFunctionAction{Action: "publish", Commit: x.source.Commit}, 400},
		{"remove acknowledgement", func() {}, func() {}, DataFunctionAction{Action: "remove"}, 400},
		{"stale revision", func() {}, func() {}, DataFunctionAction{Action: "publish", Commit: x.source.Commit, AcknowledgeExecute: true, ExpectedRevision: 7}, 409},
		{"foreign mount", func() { x.foreignMount = true }, func() { x.foreignMount = false }, in, 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.mutate()
			defer test.reset()
			w := request(t, x.s, "POST", path, "admin", test.in)
			if w.Code != test.code {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
	if x.sourceReads != 0 || x.probes != 0 || x.functionUps != 0 {
		t.Fatal("bad admission performed source/execution mutation")
	}
	v := x.action(t, "publish", nil)
	x.engine = "another-engine"
	w := request(t, x.s, "POST", path, "admin", DataFunctionAction{Action: "refresh", ExpectedRevision: v.Revision})
	if w.Code != 409 {
		t.Fatal("engine replacement accepted", w.Code)
	}
}

func TestDataFunctionInterruptedPromotionRefreshDoesNotRepeatExecution(t *testing.T) {
	x := newFunctionFixture(t)
	v := x.action(t, "publish", nil)
	before := copyFunctionRoutes(v.Active)
	v.Pending = &dataFunctionTransition{Before: before, After: map[string]string{}}
	if err := x.s.write("owned-data-functions", v); err != nil {
		t.Fatal(err)
	}
	if err := x.s.prepareFunctionFiles(x.i, v, v.Pending.After); err != nil {
		t.Fatal(err)
	}
	w := request(t, x.s, "GET", "/v1/platform/projects/owned/data/functions", "admin", nil)
	var catalog DataFunctionCatalog
	if json.Unmarshal(w.Body.Bytes(), &catalog) != nil || catalog.Functions[1].Active != nil {
		t.Fatal("uncertain transition shown as active")
	}
	probes := x.probes
	v = x.action(t, "refresh", nil)
	manifest, _, err := x.manifest()
	if err != nil || v.Pending != nil || v.Operation.Status != "succeeded" || manifest.Routes["example"].ReleaseID != before["example"] || x.probes != probes || x.functionUps != 1 {
		t.Fatal("refresh re-executed or did not reconcile exact prior route", err)
	}
}

func TestDataFunctionInvokePreservesRealStatusAndRejectsPrivateTokens(t *testing.T) {
	x := newFunctionFixture(t)
	x.source.AuthMode = "authenticated"
	x.source.SourceDigest = DataFunctionDigest(x.source.AuthMode, x.source.Files)
	x.action(t, "publish", nil)
	path := "/v1/platform/projects/owned/data/functions/example/invoke"
	jwt := func(role string) string {
		b, _ := json.Marshal(map[string]string{"role": role, "sub": "application-user"})
		return "header." + base64.RawURLEncoding.EncodeToString(b) + ".unverified-by-fixture-signature"
	}
	for _, token := range []string{"", "luci-control-token", jwt("service_role"), jwt("anon")} {
		w := request(t, x.s, "POST", path, "member", map[string]any{"input": map[string]any{}, "applicationAccessToken": token})
		if w.Code != 400 {
			t.Fatal("private/missing token admitted", w.Code)
		}
	}
	if x.invokes != 0 {
		t.Fatal("invalid token reached worker")
	}
	x.replyStatus = 422
	x.reply = `{"error":"application_validation_failed"}`
	w := request(t, x.s, "POST", path, "member", map[string]any{"input": map[string]any{"message": "test"}, "applicationAccessToken": jwt("authenticated")})
	var actual map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &actual) != nil || actual["statusCode"] != float64(422) || x.invokes != 1 {
		t.Fatal("application failure fabricated as success", w.Code, w.Body)
	}
	x.reply = strings.Repeat("x", (256<<10)+1)
	w = request(t, x.s, "POST", path, "member", map[string]any{"input": map[string]any{}, "applicationAccessToken": jwt("authenticated")})
	if w.Code != 502 || !strings.Contains(w.Body.String(), "function_outcome_uncertain") {
		t.Fatal("unbounded response or uncertain execution silently retried", w.Code)
	}
}

func TestDataFunctionBoundedSourceArtifactsRoundTripEscapedText(t *testing.T) {
	s, _ := newDataFake(t)
	src := DataFunctionSource{Name: "example", AuthMode: "authenticated", Commit: strings.Repeat("a", 40), Files: []DataFunctionFile{}}
	for n := 0; n < 16; n++ {
		name := "z" + strconv.Itoa(n) + ".ts"
		if n == 0 {
			name = "index.ts"
		}
		src.Files = append(src.Files, DataFunctionFile{Path: name, Content: strings.Repeat("\n", DataFunctionMaxFileBytes)})
	}
	if err := ValidateDataFunctionFiles(src.Files); err != nil {
		t.Fatal(err)
	}
	src.SourceDigest = DataFunctionDigest(src.AuthMode, src.Files)
	r := DataFunctionRelease{ReleaseID: randomID("function-release-"), Commit: src.Commit, SourceDigest: src.SourceDigest, AuthMode: src.AuthMode, Status: "candidate"}
	file, err := s.functionArtifactFile("owned", r.ReleaseID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.write(filepath.Base(file), src); err != nil {
		t.Fatal(err)
	}
	retained, err := s.readFunctionArtifact("owned", r)
	if err != nil || retained.SourceDigest != src.SourceDigest || len(retained.Files) != 16 {
		t.Fatal("valid bounded escaped source could not be retained", err)
	}
	// The read limit covers private JSON overhead; the source limit still
	// rejects an oversized customer bundle rather than accepting that overhead.
	src.Files = append(src.Files, DataFunctionFile{Path: "extra.ts", Content: "x"})
	if ValidateDataFunctionFiles(src.Files) == nil {
		t.Fatal("source byte limit was weakened")
	}
}
