package platform

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type dataTransport func(*http.Request) (*http.Response, error)

func (f dataTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type dataFake struct {
	mu              sync.Mutex
	s               *Service
	calls           [][]string
	sql             []string
	containers      map[string]containerInfo
	networks        map[string]map[string]string
	volumes         map[string]map[string]string
	attachments     map[string]map[string]bool
	restores        int
	restoreSchemas  map[string]bool
	failRestoreData bool
	stops           []string
}

func newDataFake(t *testing.T) (*Service, *dataFake) {
	f := &dataFake{containers: map[string]containerInfo{}, networks: map[string]map[string]string{}, volumes: map[string]map[string]string{}, attachments: map[string]map[string]bool{}, restoreSchemas: map[string]bool{}}
	s := fixture(t, f.run)
	f.s = s
	s.opts.MemoryMiB = 8192
	s.opts.CPUMillis = 4000
	s.opts.HTTPClient = &http.Client{Transport: dataTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(r.URL.Host, "127.0.0.1:") {
			t.Error("non-loopback data request")
		}
		if r.Header.Get("apikey") == "" || r.Header.Get("Authorization") == "" {
			t.Error("missing private admin authorization")
		}
		value := `{}`
		if r.URL.Path == "/auth/v1/admin/users" && r.Method == "GET" {
			value = `{"users":[{"id":"real-user","email":"test@example.test","created_at":"today","email_confirmed_at":"today","password":"must-not-expose","app_metadata":{"service_role":"must-not-expose"}}]}`
		}
		if r.URL.Path == "/auth/v1/admin/users" && r.Method == "POST" {
			value = `{"id":"created-user","email":"new@example.test","password":"must-not-expose"}`
		}
		if r.URL.Path == "/storage/v1/bucket" {
			value = `[]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(value)), Header: http.Header{}}, nil
	})}
	return s, f
}
func (f *dataFake) run(_ context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{exe}, args...))
	if exe != "docker" {
		return nil, errors.New("unexpected command")
	}
	encoded := func(v any) []byte { b, _ := json.Marshal(v); return b }
	switch args[0] {
	case "info":
		return []byte(`{"OSType":"linux","MemTotal":17179869184,"NCPU":8}`), nil
	case "ps":
		return []byte(""), nil
	case "inspect":
		name := args[1]
		v, ok := f.containers[name]
		if !ok {
			return nil, errors.New("not found")
		}
		if len(args) > 2 {
			return encoded(f.attachments[name]), nil
		}
		return encoded([]containerInfo{v}), nil
	case "network":
		name := args[2]
		if args[1] == "inspect" {
			v, ok := f.networks[name]
			if !ok {
				return nil, errors.New("not found")
			}
			return encoded(v), nil
		}
		if args[1] == "create" {
			f.networks[args[len(args)-1]] = map[string]string{labelInstance: f.s.instance, labelProject: "owned", labelResource: "network"}
			return []byte("network"), nil
		}
		if args[1] == "connect" {
			name = args[len(args)-2]
			container := args[len(args)-1]
			if f.attachments[container] == nil {
				f.attachments[container] = map[string]bool{}
			}
			f.attachments[container][name] = true
			return nil, nil
		}
		if args[1] == "disconnect" {
			delete(f.attachments[args[3]], name)
			return nil, nil
		}
	case "volume":
		if args[1] == "inspect" {
			v, ok := f.volumes[args[2]]
			if !ok {
				return nil, errors.New("not found")
			}
			return encoded(v), nil
		}
		if args[1] == "create" {
			labels := map[string]string{}
			for index, arg := range args {
				if arg == "--label" {
					key, value, _ := strings.Cut(args[index+1], "=")
					labels[key] = value
				}
			}
			f.volumes[args[len(args)-1]] = labels
			return nil, nil
		}
	case "compose":
		if args[1] == "version" {
			return []byte("2.40.0"), nil
		}
		file := ""
		index := 0
		for n, arg := range args {
			if arg == "-f" {
				file = args[n+1]
				index = n + 2
			}
		}
		b, e := os.ReadFile(file)
		if e != nil {
			return nil, e
		}
		var config map[string]any
		if yaml.Unmarshal(b, &config) != nil {
			return nil, errors.New("bad compose")
		}
		operation := args[index]
		if operation == "pull" {
			return nil, nil
		}
		services := config["services"].(map[string]any)
		if operation == "stop" {
			for _, raw := range services {
				service := raw.(map[string]any)
				name := service["container_name"].(string)
				v := f.containers[name]
				v.State.Running = false
				f.containers[name] = v
				f.stops = append(f.stops, name)
			}
			return nil, nil
		}
		if operation == "up" {
			for _, raw := range config["volumes"].(map[string]any) {
				volume := raw.(map[string]any)
				labels := map[string]string{}
				for key, value := range volume["labels"].(map[string]any) {
					labels[key] = value.(string)
				}
				f.volumes[volume["name"].(string)] = labels
			}
			network := config["networks"].(map[string]any)["default"].(map[string]any)
			labels := map[string]string{}
			for key, value := range network["labels"].(map[string]any) {
				labels[key] = value.(string)
			}
			f.networks[network["name"].(string)] = labels
			for _, raw := range services {
				service := raw.(map[string]any)
				v := containerInfo{ID: "actual-container", Image: "sha256:actual-image"}
				v.Config.Image = service["image"].(string)
				v.Config.Labels = map[string]string{}
				for key, value := range service["labels"].(map[string]any) {
					v.Config.Labels[key] = value.(string)
				}
				v.State.Running = true
				v.State.Health.Status = "healthy"
				name := service["container_name"].(string)
				f.containers[name] = v
				if f.attachments[name] == nil {
					f.attachments[name] = map[string]bool{}
				}
				f.attachments[name][network["name"].(string)] = true
			}
			return nil, nil
		}
	case "exec":
		if strings.Contains(strings.Join(args, " "), "pg_dump") {
			return []byte("PGDMP-owned-auth-storage-public"), nil
		}
		if strings.Contains(strings.Join(args, " "), "pg_restore") {
			if !bytes.HasPrefix(input, []byte("PGDMP")) {
				return nil, errors.New("bad restore input")
			}
			joined := " " + strings.Join(args, " ") + " "
			if !strings.Contains(joined, " -U supabase_admin ") || strings.Contains(joined, " --no-owner ") {
				return nil, errors.New("Auth tables require their bootstrap owner and preserved service-role ownership")
			}
			preData := strings.Contains(joined, " --section=pre-data ")
			loadData := strings.Contains(joined, " --section=data ")
			postData := strings.Contains(joined, " --section=post-data ")
			if strings.Contains(joined, " --clean ") && (!preData || loadData || postData) {
				// PostgreSQL cannot DROP POLICY IF EXISTS ON an absent
				// application table in a fresh Supabase installation.
				return nil, errors.New("policy parent relation absent")
			}
			if preData {
				f.restoreSchemas[args[2]] = true
			} else if !f.restoreSchemas[args[2]] || f.failRestoreData {
				return nil, errors.New("archive data could not be restored")
			}
			f.restores++
			return nil, nil
		}
		sql := string(input)
		f.sql = append(f.sql, sql)
		if strings.Contains(sql, "SELECT 1") || strings.Contains(sql, "CASE WHEN EXISTS") {
			return []byte("1\n"), nil
		}
		if strings.Contains(sql, "json_agg") {
			return []byte("[]\n"), nil
		}
		if strings.Contains(sql, "row_to_json") {
			return []byte(`{"id":"inserted"}`), nil
		}
		if strings.Contains(sql, "json_build_object") {
			return []byte(`{"enabled":true,"policies":[]}`), nil
		}
		return []byte(""), nil
	case "run":
		if bytes.Contains(input, []byte{0x1f, 0x8b}) {
			return nil, nil
		}
		return dataTar("object.txt", []byte("stored file")), nil
	}
	return nil, errors.New("unexpected Docker operation")
}
func dataTar(name string, content []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(content))})
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gz.Close()
	return b.Bytes()
}
func TestStorageBackupAcceptsTarDirectoriesAndRejectsUnsafePaths(t *testing.T) {
	archive := func(headers []tar.Header) []byte {
		var out bytes.Buffer
		gz := gzip.NewWriter(&out)
		tw := tar.NewWriter(gz)
		for _, h := range headers {
			if e := tw.WriteHeader(&h); e != nil {
				t.Fatal(e)
			}
		}
		if e := tw.Close(); e != nil {
			t.Fatal(e)
		}
		if e := gz.Close(); e != nil {
			t.Fatal(e)
		}
		return out.Bytes()
	}
	if e := validateStorageArchive(archive([]tar.Header{{Name: "./", Typeflag: tar.TypeDir}, {Name: "./bucket/", Typeflag: tar.TypeDir}, {Name: "./bucket/folder/", Typeflag: tar.TypeDir}, {Name: "./bucket/folder/file.txt", Typeflag: tar.TypeReg}})); e != nil {
		t.Fatalf("normal nested GNU tar archive rejected: %v", e)
	}
	for _, h := range []tar.Header{{Name: "../escape/", Typeflag: tar.TypeDir}, {Name: "/absolute/", Typeflag: tar.TypeDir}, {Name: "./bucket/../escape/", Typeflag: tar.TypeDir}, {Name: "./bucket/symlink", Typeflag: tar.TypeSymlink, Linkname: "../../escape"}, {Name: "./bucket//file", Typeflag: tar.TypeReg}} {
		if e := validateStorageArchive(archive([]tar.Header{h})); e == nil {
			t.Fatalf("unsafe archive entry accepted: %q", h.Name)
		}
	}
}
func waitData(t *testing.T, s *Service) DataStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		s.mu.Lock()
		state, e := s.loadData("owned")
		busy := s.busy["owned"]
		s.mu.Unlock()
		if e != nil {
			t.Fatal(e)
		}
		if !busy && (state.Operation == nil || state.Operation.Status != "running") {
			return state
		}
		if time.Now().After(deadline) {
			t.Fatal("data operation did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func installData(t *testing.T, s *Service) DataStatus {
	t.Helper()
	w := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "install", AcknowledgeResources: true})
	if w.Code != 202 {
		t.Fatalf("install %d %s", w.Code, w.Body)
	}
	state := waitData(t, s)
	if state.Status != "ready" || state.Operation.Status != "succeeded" {
		t.Fatalf("install state %+v", state)
	}
	return state
}

func TestDataAssetsArePinnedAndProjectStackHasPrivateLoopbackBindings(t *testing.T) {
	s, _ := newDataFake(t)
	i := dataInstallation{ProjectID: "owned", StackID: "stack-test", APIPort: 45123}
	secret := newDataSecret()
	if e := s.prepareDataStack(i, secret); e != nil {
		t.Fatal(e)
	}
	root, _ := s.dataRoot(i)
	b, _ := os.ReadFile(filepath.Join(root, "compose.yaml"))
	var config map[string]any
	if e := yaml.Unmarshal(b, &config); e != nil {
		t.Fatal(e)
	}
	services := config["services"].(map[string]any)
	totalMemory, totalCPU := 0, 0
	for name, raw := range services {
		v := raw.(map[string]any)
		if !strings.Contains(v["image"].(string), "@sha256:") {
			t.Fatal("mutable image")
		}
		if name == "api-gw" {
			if v["ports"].([]any)[0] != "127.0.0.1:45123:8000" {
				t.Fatal("public bind")
			}
		} else if _, ok := v["ports"]; ok {
			t.Fatal("management or database port exposed")
		}
		totalMemory += dataMemory[name]
		totalCPU += dataCPU[name]
		if bytes.Contains(b, []byte(secret.Password)) {
			t.Fatal("password in Compose source")
		}
	}
	if totalMemory != 4096 || totalCPU != 2000 {
		t.Fatal("stack budget differs from requirements")
	}
	env, _ := os.ReadFile(filepath.Join(root, "runtime.env"))
	if !bytes.Contains(env, []byte("FUNCTIONS_VERIFY_JWT=true")) || bytes.Contains(env, []byte("sk-proj-xxxxxxxx")) {
		t.Fatal("insecure config inherited")
	}
	main, _ := os.ReadFile(filepath.Join(root, "volumes/functions/main/index.ts"))
	if bytes.Contains(main, []byte("jsr:@panva/jose@6")) || !bytes.Contains(main, []byte("../_vendor/jose/dist/webapi/index.js")) {
		t.Fatal("floating worker dependency")
	}
	if _, e := os.Stat(filepath.Join(root, "volumes/functions/_vendor/jose/LICENSE.md")); e != nil {
		t.Fatal("vendored grant absent")
	}
}
func TestDataReadIsCachedAndDurableInterruptedOperationsAreNotRerun(t *testing.T) {
	s, f := newDataFake(t)
	if e := s.changeData("owned", func(v *DataStatus) {
		v.Status = "installing"
		v.Operation = &DataOperation{ID: "op", Status: "running", Action: "install"}
	}); e != nil {
		t.Fatal(e)
	}
	w := request(t, s, "GET", "/v1/platform/projects/owned/data", "member", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"interrupted"`) {
		t.Fatal(w.Body)
	}
	if len(f.calls) != 0 {
		t.Fatal("GET started or inspected Docker")
	}
	w = request(t, s, "GET", "/v1/platform/projects/foreign/data", "member", nil)
	if w.Code != 404 || len(f.calls) != 0 {
		t.Fatal("foreign project read")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data", "member", map[string]string{"action": "install"})
	if w.Code != 400 || len(f.calls) != 0 {
		t.Fatal("missing resource consent")
	}
}
func TestDataLifecycleAndBindingsPersistWithoutReturningPrivateKeys(t *testing.T) {
	s, f := newDataFake(t)
	installData(t, s)
	var i dataInstallation
	_ = s.read("owned-data-installation", &i)
	secret, _ := s.dataCredentials(i)
	w := request(t, s, "GET", "/v1/platform/projects/owned/data", "member", nil)
	for _, private := range []string{secret.Password, secret.ServiceRole, secret.JWT, secret.Dashboard} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("private data value in DTO")
		}
	}
	_, bindings, e := s.environment("owned")
	if e != nil || bindings["SUPABASE_SERVICE_ROLE_KEY"] != secret.ServiceRole || bindings["SUPABASE_URL"] != "http://supabase-api:8000" {
		t.Fatal("private app bindings missing")
	}
	for _, action := range []string{"stop", "start"} {
		w = request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: action, AcknowledgeResources: true})
		if w.Code != 202 {
			t.Fatal(w.Body)
		}
		state := waitData(t, s)
		expected := "stopped"
		if action == "start" {
			expected = "ready"
		}
		if state.Status != expected || state.Operation.Status != "succeeded" {
			t.Fatalf("%s %+v", action, state)
		}
	}
	other, e := New(s.opts)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	w = request(t, other, "GET", "/v1/platform/projects/owned/data", "member", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ready"`) {
		t.Fatal("state did not survive new Service")
	}
	for _, call := range f.calls {
		joined := strings.Join(call, " ")
		for _, value := range []string{secret.Password, secret.JWT, secret.ServiceRole} {
			if strings.Contains(joined, value) {
				t.Fatal("private key in process argument")
			}
		}
	}
}
func TestDataResourceOwnershipCollisionCannotBeAdopted(t *testing.T) {
	s, f := newDataFake(t)
	i := dataInstallation{ProjectID: "owned", StackID: "stack-test", APIPort: 45123}
	if e := s.write("owned-data-installation", i); e != nil {
		t.Fatal(e)
	}
	secret := newDataSecret()
	_ = s.write(i.StackID+"-secret", secret)
	_ = s.prepareDataStack(i, secret)
	foreign := containerInfo{}
	foreign.Config.Labels = map[string]string{labelInstance: "another-owner"}
	f.containers[s.dataName(i, "db")] = foreign
	w := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "start", AcknowledgeResources: true})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	state := waitData(t, s)
	if state.Operation.Status != "failed" {
		t.Fatal("foreign resource adopted")
	}
	for _, call := range f.calls {
		if strings.Contains(strings.Join(call, " "), " up ") {
			t.Fatal("Compose started despite collision")
		}
	}
}
func TestDataControlsUseRealServiceContractsAndBoundedSQLTemplates(t *testing.T) {
	s, f := newDataFake(t)
	installData(t, s)
	w := request(t, s, "GET", "/v1/platform/projects/owned/data/users", "member", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "must-not-expose") {
		t.Fatal("raw Auth user exposed")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/tables", "member", map[string]any{"name": "tasks", "columns": []DataColumn{{Name: "title", Type: "text"}}, "enableRls": true})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/tables", "member", map[string]any{"name": "tasks; DROP TABLE auth.users", "columns": []DataColumn{{Name: "title", Type: "text"}}, "enableRls": true})
	if w.Code != 400 {
		t.Fatal("SQL identifier injection accepted")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/tables/tasks/rows", "member", map[string]any{"title": "'); DROP TABLE auth.users; --"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/tables/tasks/rls", "member", map[string]any{"enabled": true, "policy": map[string]string{"name": "owner_access", "operation": "all", "role": "authenticated", "using": "owner", "ownerColumn": "owner_id"}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/realtime", "member", map[string]any{"table": "tasks", "enabled": true})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	joined := strings.Join(f.sql, "\n")
	if !strings.Contains(joined, `ENABLE ROW LEVEL SECURITY`) || !strings.Contains(joined, `auth.uid() = "owner_id"`) || !strings.Contains(joined, `''); DROP TABLE auth.users; --`) {
		t.Fatal("fixed SQL template/escaping missing")
	}
	for _, bad := range []string{"../escape", "/absolute", "folder//empty", "x\\y"} {
		if _, ok := objectPath(bad); ok {
			t.Fatal("unsafe storage path accepted")
		}
	}
}
func TestDataBackupChecksumAndStagedRestoreRetainPreviousStack(t *testing.T) {
	s, f := newDataFake(t)
	installData(t, s)
	var prior dataInstallation
	_ = s.read("owned-data-installation", &prior)
	w := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "backup"})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	state := waitData(t, s)
	if state.Operation.Status != "succeeded" || len(state.Backups) != 1 {
		t.Fatal(state)
	}
	backup := state.Backups[0]
	_, b, e := s.readDataBackup("owned", backup.ID)
	if e != nil {
		t.Fatal(e)
	}
	meta, dump, files, e := unpackDataBackup("owned", b)
	if e != nil || meta.ProjectID != "owned" || !bytes.HasPrefix(dump, []byte("PGDMP")) || validateStorageArchive(files) != nil {
		t.Fatal("invalid data backup")
	}
	secret, _ := s.dataCredentials(prior)
	if bytes.Contains(b, []byte(secret.ServiceRole)) || bytes.Contains(b, []byte(secret.Password)) {
		t.Fatal("private restore keys in downloaded backup")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "restore", BackupID: backup.ID, AcknowledgeResources: true, ConfirmRestore: true})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	state = waitData(t, s)
	if state.Operation.Status != "succeeded" || !state.PriorStackRetained || f.restores != 2 {
		t.Fatalf("restore %+v count %d", state, f.restores)
	}
	var active dataInstallation
	_ = s.read("owned-data-installation", &active)
	if active.StackID == prior.StackID || len(active.Previous) != 1 || active.Previous[0] != prior.StackID || active.StorageTenantID != prior.storageTenant() {
		t.Fatal("previous stack overwritten")
	}
	root, _ := s.dataRoot(active)
	env, e := os.ReadFile(filepath.Join(root, "runtime.env"))
	if e != nil || !bytes.Contains(env, []byte("STORAGE_TENANT_ID="+prior.storageTenant()+"\n")) {
		t.Fatal("restored storage uses a different file namespace")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "restore", BackupID: backup.ID, AcknowledgeResources: true, ConfirmRestore: true})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	state = waitData(t, s)
	if state.Operation.Status != "succeeded" {
		t.Fatal("original backup cannot be restored again after switching stacks")
	}
	var repeated dataInstallation
	if e := s.read("owned-data-installation", &repeated); e != nil || repeated.StorageTenantID != prior.storageTenant() || repeated.StackID == active.StackID {
		t.Fatal("repeated restore changed the file namespace or reused the active stack")
	}
	if _, e := s.dataCredentials(prior); e != nil {
		t.Fatal("previous credentials removed")
	}
	if e := validateStorageArchive(dataTar("../escape", []byte("bad"))); e == nil {
		t.Fatal("malicious storage archive accepted")
	}
	path, _ := s.path(backup.ID)
	_ = os.WriteFile(path, []byte("changed"), 0600)
	if _, _, e := s.readDataBackup("owned", backup.ID); e == nil {
		t.Fatal("tampered backup accepted")
	}
}

func TestLegacyDataBackupNamespaceMustBelongToActiveOrRetainedInstallation(t *testing.T) {
	i := dataInstallation{ProjectID: "owned", StackID: "stack-current", Previous: []string{"stack-original"}}
	for _, tenant := range []string{"stack-current", "stack-original"} {
		actual, e := legacyDataStorageTenant(dataTar("luci/"+tenant+"/private/folder/object", []byte("stored file")), i)
		if e != nil || actual != tenant {
			t.Fatalf("owned legacy namespace %q not preserved", tenant)
		}
	}
	if _, e := legacyDataStorageTenant(dataTar("luci/stack-foreign/private/object", []byte("foreign")), i); e == nil {
		t.Fatal("foreign file namespace accepted")
	}
	if ownedDataStorageTenant("../escape", i) {
		t.Fatal("invalid namespace accepted")
	}
	// Explicit durable tenant identity survives replacement of Docker resources.
	i.StorageTenantID = "stack-original"
	if i.storageTenant() != "stack-original" || !ownedDataStorageTenant("stack-original", i) {
		t.Fatal("durable namespace was replaced with the container identity")
	}
}

func TestFailedArchiveLoadKeepsActiveDataStackAndRetainsCandidate(t *testing.T) {
	s, f := newDataFake(t)
	installData(t, s)
	var original dataInstallation
	if e := s.read("owned-data-installation", &original); e != nil {
		t.Fatal(e)
	}
	w := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "backup"})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	state := waitData(t, s)
	if state.Operation.Status != "succeeded" || len(state.Backups) != 1 {
		t.Fatal("backup failed")
	}
	f.failRestoreData = true
	w = request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "restore", BackupID: state.Backups[0].ID, AcknowledgeResources: true, ConfirmRestore: true})
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	state = waitData(t, s)
	if state.Operation.Status != "failed" || !state.PriorStackRetained {
		t.Fatal("failed restore did not retain recovery state")
	}
	var active dataInstallation
	if e := s.read("owned-data-installation", &active); e != nil || active.StackID != original.StackID {
		t.Fatal("failed candidate replaced the active installation")
	}
	for _, name := range dataServiceNames {
		if !f.containers[s.dataName(original, name)].State.Running {
			t.Fatalf("failed restore stopped the active %s service", name)
		}
	}
	var retained []string
	if e := s.read("owned-data-retained", &retained); e != nil || len(retained) != 1 || retained[0] == original.StackID {
		t.Fatal("failed candidate was not recorded for inspection")
	}
	if _, _, e := s.readDataBackup("owned", state.Backups[0].ID); e != nil {
		t.Fatal("failed restore lost the original backup")
	}
}

func TestDataCapacityAccountsForOtherWorkloadsAndAdditionalRestoreStack(t *testing.T) {
	s, _ := newDataFake(t)
	s.opts.Runner = func(_ context.Context, _ string, args []string, _ string, _ []byte) ([]byte, error) {
		switch args[0] {
		case "info":
			return []byte(`{"OSType":"linux","MemTotal":8589934592,"NCPU":4}`), nil
		case "compose":
			return []byte("2.40.0"), nil
		case "ps":
			return []byte("existing-data legacy-postgres"), nil
		case "inspect":
			return json.Marshal([]any{map[string]any{"Config": map[string]any{"Labels": map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: "data-stack-db"}}, "HostConfig": map[string]any{"Memory": int64(4 << 30), "NanoCpus": int64(2e9)}}, map[string]any{"Config": map[string]any{"Labels": map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: "postgres"}}, "HostConfig": map[string]any{"Memory": int64(1 << 30), "NanoCpus": int64(1e9)}}})
		}
		return nil, errors.New("unexpected")
	}
	if e := s.dataCapacity(context.Background(), "owned", false); e != nil {
		t.Fatalf("existing stack restart rejected: %v", e)
	}
	if e := s.dataCapacity(context.Background(), "owned", true); e == nil {
		t.Fatal("restore did not reserve capacity for both stacks and legacy PG")
	}
}

func TestDataBucketPrefixesAndReviewedFunctionsHaveBoundedRealContracts(t *testing.T) {
	s, _ := newDataFake(t)
	installData(t, s)
	var prefix string
	s.opts.HTTPClient = &http.Client{Transport: dataTransport(func(r *http.Request) (*http.Response, error) {
		input, _ := io.ReadAll(r.Body)
		result := `[]`
		if strings.HasPrefix(r.URL.Path, "/storage/v1/object/list/") {
			var value map[string]any
			_ = json.Unmarshal(input, &value)
			prefix, _ = value["prefix"].(string)
		}
		if r.URL.Path == "/functions/v1/hello" {
			result = `{"message":"Hello from Luci Functions!"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(result)), Header: http.Header{}}, nil
	})}
	w := request(t, s, "GET", "/v1/platform/projects/owned/data/buckets/files/objects?prefix=documents%2F", "member", nil)
	if w.Code != 200 || prefix != "documents/" {
		t.Fatal(w.Code, w.Body)
	}
	w = request(t, s, "GET", "/v1/platform/projects/owned/data/buckets/files/objects?prefix=..%2Fprivate%2F", "member", nil)
	if w.Code != 400 {
		t.Fatal("unsafe nested prefix accepted")
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/functions/hello/invoke", "member", map[string]string{"name": "test"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Hello from Luci Functions!") {
		t.Fatal(w.Code, w.Body)
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/data/functions/arbitrary/invoke", "member", map[string]string{"code": "Deno.env.toObject()"})
	if w.Code != 404 {
		t.Fatal("unapproved function invocation accepted")
	}
}
