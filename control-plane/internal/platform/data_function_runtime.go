package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const dataFunctionRuntimeVersion = "1"
const dataFunctionHello = "// Luci's dependency-free example.\nexport default { fetch: () => Response.json({ message: 'Hello from Luci Functions!' }) };\n"

// Keep the licensed, pinned upstream JWT implementation. The replacement
// dispatcher never forwards private container credentials to customer code.
func dataFunctionMain() string {
	b, _ := dataAssets.ReadFile("supabase_assets/volumes/functions/main/index.ts")
	head, _, _ := strings.Cut(string(b), "Deno.serve(async (req: Request) => {")
	head = strings.ReplaceAll(head, "jsr:@panva/jose@6", "../_vendor/jose/dist/webapi/index.js")
	return head + dataFunctionDispatcher
}

const dataFunctionDispatcher = `
const base = '/home/deno/functions';
const manifestPath = base + '/_luci/routes.json';
const publicKeys = ['SUPABASE_URL','SUPABASE_PUBLIC_URL','SUPABASE_ANON_KEY','SUPABASE_FUNCTION_SLUG'];
const validName = (name: string) => /^[a-z][a-z0-9-]{0,62}$/.test(name) && name !== 'main';
const validRelease = (id: string) => /^function-release-[a-f0-9]{24}$/.test(id);
async function routes() {
 try { return JSON.parse(await Deno.readTextFile(manifestPath)); }
 catch { return { version: 1, routes: {}, candidates: {} }; }
}
async function worker(name: string, release: any) {
 const servicePath = name === 'hello' ? base + '/hello' : base + '/_luci/releases/' + release.releaseId;
 const envVars = publicKeys.filter(k => k !== 'SUPABASE_FUNCTION_SLUG').map(k => [k,Deno.env.get(k) || '']);
 envVars.push(['SUPABASE_FUNCTION_SLUG',name]);
 return await EdgeRuntime.userWorkers.create({
  servicePath, customModuleRoot: servicePath, allowRemoteModules: false, noNpm: true,
  memoryLimitMb: 150, workerTimeoutMs: 60000, cpuTimeSoftLimitMs: 10000, cpuTimeHardLimitMs: 20000,
  noModuleCache: false, envVars,
  permissions: {allow_all:false,allow_env:publicKeys,allow_net:['api-gw:8000'],allow_read:[servicePath],deny_write:[],deny_run:[],deny_ffi:[],deny_sys:[],deny_import:[]}
 });
}
async function verifiedClaims(req: Request) {
 const token = getAuthToken(req);
 if (typeof token !== 'string' || !JWT_SECRET) return null;
 try { return (await jose.jwtVerify(token,new TextEncoder().encode(JWT_SECRET),{algorithms:['HS256']})).payload; }
 catch { return null; }
}
Deno.serve(async (req: Request) => {
 const pathname = new URL(req.url).pathname;
 const pieces = pathname.split('/');
 const name = pieces[1];
 if (name === '_luci-health' || name === '_luci-probe') {
  if (req.headers.get('x-luci-function-probe') !== Deno.env.get('LUCI_FUNCTION_PROBE_TOKEN') || !Deno.env.get('LUCI_FUNCTION_PROBE_TOKEN')) return new Response(null,{status:404});
  const manifest = await routes();
  if (name === '_luci-health') {
   const raw = await Deno.readTextFile(manifestPath);
   const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(raw)))).map(b=>b.toString(16).padStart(2,'0')).join('');
   return Response.json({runtimeVersion:'1',manifestDigest:digest});
  }
  try {
   const check = await req.json();
   const candidate = manifest.candidates[check.releaseId];
   if (!candidate || !validName(candidate.name) || !validRelease(candidate.releaseId) || candidate.sourceDigest !== check.sourceDigest) return new Response(null,{status:409});
   await worker(candidate.name,candidate); // Compilation/boot only. Never call customer fetch.
   return Response.json({runtimeVersion:'1',releaseId:candidate.releaseId,sourceDigest:candidate.sourceDigest});
  } catch { return Response.json({error:'function_boot_failed'},{status:503}); }
 }
 if (!validName(name) || pieces.some(part => part === '..' || part === '.')) return new Response(null,{status:404});
 const manifest = await routes();
 const release = name === 'hello' ? {authMode:'project-key'} : manifest.routes[name];
 if (!release || (name !== 'hello' && !validRelease(release.releaseId))) return new Response(null,{status:404});
 if (req.method === 'OPTIONS') return new Response(null,{status:204});
 const claims = await verifiedClaims(req);
 if (!claims || !['anon','authenticated'].includes(String(claims.role)) || (release.authMode === 'authenticated' && (claims.role !== 'authenticated' || !claims.sub))) return Response.json({error:'application_auth_required'},{status:401});
 try { return await (await worker(name,release)).fetch(req); }
 catch { return Response.json({error:'function_execution_failed'},{status:503}); }
});
`

type functionRoute struct {
	Name         string `json:"name"`
	ReleaseID    string `json:"releaseId"`
	SourceDigest string `json:"sourceDigest"`
	AuthMode     string `json:"authMode"`
}
type functionManifest struct {
	Version    int                      `json:"version"`
	Routes     map[string]functionRoute `json:"routes"`
	Candidates map[string]functionRoute `json:"candidates"`
}

func manifestForFunctions(v dataFunctionState, active map[string]string) (functionManifest, error) {
	m := functionManifest{Version: 1, Routes: map[string]functionRoute{}, Candidates: map[string]functionRoute{}}
	for name, releases := range v.Releases {
		for _, r := range releases {
			if r.Status == "failed" {
				continue
			}
			if !ValidDataFunctionName(name) || !functionReleaseName.MatchString(r.ReleaseID) || !ValidDataFunctionAuth(r.AuthMode) {
				return m, errors.New("invalid function release")
			}
			route := functionRoute{name, r.ReleaseID, r.SourceDigest, r.AuthMode}
			m.Candidates[r.ReleaseID] = route
			if active[name] == r.ReleaseID {
				m.Routes[name] = route
			}
		}
	}
	return m, nil
}

var functionReleaseName = regexpFunctionRelease()

func regexpFunctionRelease() *regexp.Regexp {
	return regexp.MustCompile(`^function-release-[a-f0-9]{24}$`)
}
func (s *Service) functionArtifactFile(id, release string) (string, error) {
	if !identifier.MatchString(id) || !functionReleaseName.MatchString(release) {
		return "", errors.New("invalid function artifact binding")
	}
	return s.path(id + "-" + release + "-source")
}
func (s *Service) readFunctionArtifact(id string, r DataFunctionRelease) (DataFunctionSource, error) {
	file, e := s.functionArtifactFile(id, r.ReleaseID)
	if e != nil {
		return DataFunctionSource{}, e
	}
	f, e := os.Open(file)
	if e != nil {
		return DataFunctionSource{}, errors.New("function artifact unavailable")
	}
	defer f.Close()
	// JSON may escape a source byte to six bytes. Bound the read itself while
	// allowing every valid source bundle to round-trip without truncation.
	b, e := io.ReadAll(io.LimitReader(f, 8*DataFunctionMaxBytes+1))
	if e != nil || len(b) > 8*DataFunctionMaxBytes {
		return DataFunctionSource{}, errors.New("function artifact unavailable")
	}
	var src DataFunctionSource
	if json.Unmarshal(b, &src) != nil || src.SourceDigest != r.SourceDigest || src.Commit != r.Commit || src.AuthMode != r.AuthMode || !ValidDataFunctionName(src.Name) || DataFunctionDigest(src.AuthMode, src.Files) != src.SourceDigest || ValidateDataFunctionFiles(src.Files) != nil {
		return src, errors.New("function artifact differs from its receipt")
	}
	return src, nil
}
func ValidDataFunctionPath(name string) bool {
	if name == "" || len(name) > 240 || path.Clean(name) != name || path.IsAbs(name) || strings.ContainsAny(name, "\\:\x00\r\n") || strings.HasPrefix(name, "../") {
		return false
	}
	for _, p := range strings.Split(name, "/") {
		if strings.HasPrefix(p, ".") || strings.EqualFold(p, "node_modules") || strings.TrimRight(p, ". ") != p || strings.Contains(strings.ToLower(p), "secret") || strings.Contains(strings.ToLower(p), "credential") {
			return false
		}
	}
	return (strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".json")) && name != "luci.function.json"
}
func ValidateDataFunctionFiles(files []DataFunctionFile) error {
	total := 0
	seen := map[string]bool{}
	directories := map[string]bool{}
	entry := false
	if len(files) < 1 || len(files) > DataFunctionMaxFiles {
		return errors.New("bounded function files required")
	}
	for _, f := range files {
		for parent := path.Dir(f.Path); parent != "." && parent != "/"; parent = path.Dir(parent) {
			directories[parent] = true
		}
		if !ValidDataFunctionPath(f.Path) || seen[strings.ToLower(f.Path)] || len(f.Content) > DataFunctionMaxFileBytes || !utf8.ValidString(f.Content) || strings.ContainsRune(f.Content, '\x00') {
			return errors.New("unsupported function source")
		}
		seen[strings.ToLower(f.Path)] = true
		total += len(f.Content)
		if f.Path == "index.ts" {
			entry = true
		}
	}
	if !entry || total > DataFunctionMaxBytes || len(directories)+len(files)+2 > 96 {
		return errors.New("index.ts and bounded function source required")
	}
	return nil
}
func DataFunctionDigest(auth string, files []DataFunctionFile) string {
	copy := append([]DataFunctionFile(nil), files...)
	sort.Slice(copy, func(i, j int) bool { return copy[i].Path < copy[j].Path })
	b, _ := json.Marshal(struct {
		Auth  string
		Files []DataFunctionFile
	}{auth, copy})
	return dataSum(b)
}
func (s *Service) functionEngine(ctx context.Context, i dataInstallation, v *dataFunctionState) error {
	info, e := s.engineInfo(ctx)
	id := strings.TrimSpace(info.ID)
	if e != nil || id == "" || len(id) > 128 || strings.ContainsAny(id, "\r\n\x00 \t") || (v.EngineID != "" && v.EngineID != id) || (v.StableTenant != "" && v.StableTenant != i.storageTenant()) {
		return errors.New("function host identity changed")
	}
	v.EngineID = id
	v.StableTenant = i.storageTenant()
	return nil
}
func (s *Service) prepareFunctionFiles(i dataInstallation, v dataFunctionState, active map[string]string) error {
	root, e := s.dataRoot(i)
	if e != nil {
		return e
	}
	assetRoot := filepath.Join(root, "volumes", "functions")
	dirs := map[string]bool{}
	for _, releases := range v.Releases {
		for _, r := range releases {
			if r.Status == "failed" {
				continue
			}
			src, e := s.readFunctionArtifact(i.ProjectID, r)
			if e != nil {
				return e
			}
			for _, f := range src.Files {
				target := filepath.Join(assetRoot, "_luci", "releases", r.ReleaseID, filepath.FromSlash(f.Path))
				if e = dataPublicAssetDirectories(root, filepath.Dir(target), dirs); e != nil {
					return e
				}
				if e = writeFunctionPublicFile(root, target, []byte(f.Content)); e != nil {
					return e
				}
			}
		}
	}
	m, e := manifestForFunctions(v, active)
	if e != nil {
		return e
	}
	b, _ := json.Marshal(m)
	target := filepath.Join(assetRoot, "_luci", "routes.json")
	if e = dataPublicAssetDirectories(root, filepath.Dir(target), dirs); e != nil {
		return e
	}
	return writeFunctionPublicFile(root, target, b)
}
func writeFunctionPublicFile(root, target string, b []byte) error {
	if _, e := os.Lstat(target); e == nil {
		if _, e = hostpath.ConfineToMounts([]string{root}, target); e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	file, e := os.CreateTemp(filepath.Dir(target), ".function-write-")
	if e != nil {
		return e
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, e = file.Write(b); e == nil {
		e = file.Sync()
	}
	if close := file.Close(); e == nil {
		e = close
	}
	if e != nil {
		return e
	}
	if e = os.Chmod(tmp, 0644); e != nil {
		return e
	}
	if _, e = hostpath.ConfineToMounts([]string{root}, filepath.Dir(target)); e != nil {
		return e
	}
	return os.Rename(tmp, target)
}
func (s *Service) dataFunctionConfiguration(i dataInstallation, values map[string]string) error {
	v, e := s.loadDataFunctions(i.ProjectID)
	if e != nil {
		return e
	}
	if v.StableTenant != "" && v.StableTenant != i.storageTenant() {
		return errors.New("function tenant changed")
	}
	values["LUCI_FUNCTION_PROBE_TOKEN"] = v.ProbeToken
	// A restart or data restore reinstates the last durable verified route set.
	return s.prepareFunctionFiles(i, v, v.Active)
}
func (s *Service) verifyFunctionContainer(ctx context.Context, i dataInstallation, v dataFunctionState) error {
	return s.verifyFunctionHost(ctx, i, v, true)
}
func (s *Service) verifyFunctionHost(ctx context.Context, i dataInstallation, v dataFunctionState, strict bool) error {
	root, e := s.dataRoot(i)
	if e != nil {
		return e
	}
	b, e := s.docker(ctx, "inspect", s.dataName(i, "functions"))
	if e != nil {
		return e
	}
	var all []struct {
		Config struct {
			Image  string
			Labels map[string]string
			Env    []string
		}
		State struct {
			Running bool
			Health  struct{ Status string }
		}
		Mounts []struct {
			Source, Destination string
			RW                  bool
		}
		NetworkSettings struct {
			Networks map[string]struct{ NetworkID string }
		}
	}
	if json.Unmarshal(b, &all) != nil || len(all) != 1 {
		return errors.New("functions identity unavailable")
	}
	c := all[0]
	assets, e := reviewedDataAssets()
	if e != nil {
		return e
	}
	if c.Config.Image != assets.Images["supabase/edge-runtime:v1.76.2"] || !c.State.Running || c.State.Health.Status != "healthy" || c.Config.Labels[labelInstance] != s.instance || c.Config.Labels[labelProject] != i.ProjectID || c.Config.Labels[labelResource] != "data-"+i.StackID+"-functions" {
		return errors.New("owned function runtime not healthy")
	}
	mounted := false
	for _, m := range c.Mounts {
		if m.Destination == "/home/deno/functions" {
			if filepath.Clean(m.Source) != filepath.Join(root, "volumes", "functions") || (strict && m.RW) {
				return errors.New("function source mount changed")
			}
			mounted = true
		}
		if m.Destination != "/home/deno/functions" && m.Destination != "/root/.cache/deno" {
			return errors.New("private runtime mount refused")
		}
	}
	if !mounted {
		return errors.New("function source mount missing")
	}
	env := map[string]string{}
	for _, line := range c.Config.Env {
		k, value, ok := strings.Cut(line, "=")
		if ok {
			env[k] = value
		}
	}
	if strict && (env["LUCI_FUNCTION_RUNTIME_VERSION"] != dataFunctionRuntimeVersion || env["LUCI_FUNCTION_PROBE_TOKEN"] != v.ProbeToken || env["SUPABASE_SERVICE_ROLE_KEY"] != "" || env["SUPABASE_DB_URL"] != "" || env["SUPABASE_SECRET_KEYS"] != "") {
		return errors.New("function credential isolation unavailable")
	}
	if !strict && ((env["LUCI_FUNCTION_RUNTIME_VERSION"] != "" && env["LUCI_FUNCTION_RUNTIME_VERSION"] != dataFunctionRuntimeVersion) || (env["LUCI_FUNCTION_PROBE_TOKEN"] != "" && env["LUCI_FUNCTION_PROBE_TOKEN"] != v.ProbeToken)) {
		return errors.New("function runtime binding changed")
	}
	network := s.resource(i.ProjectID, i.StackID+"-net")
	member, exists := c.NetworkSettings.Networks[network]
	if !exists || member.NetworkID == "" {
		return errors.New("function project network missing")
	}
	b, e = s.docker(ctx, "network", "inspect", network)
	if e != nil {
		return e
	}
	var networks []struct {
		ID     string
		Labels map[string]string
	}
	if json.Unmarshal(b, &networks) != nil || len(networks) != 1 || networks[0].ID != member.NetworkID || networks[0].Labels[labelInstance] != s.instance || networks[0].Labels[labelProject] != i.ProjectID || networks[0].Labels[labelResource] != "data-"+i.StackID {
		return errors.New("function network ownership changed")
	}
	return nil
}
func (s *Service) functionRequest(ctx context.Context, i dataInstallation, secret dataSecret, name string, input []byte, token, probe string) (int, string, any, error) {
	req, e := http.NewRequestWithContext(ctx, "POST", dataURL(i)+"/functions/v1/"+name, bytes.NewReader(input))
	if e != nil {
		return 0, "", nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", secret.Anon)
	req.Header.Set("Authorization", "Bearer "+token)
	if probe != "" {
		req.Header.Set("X-Luci-Function-Probe", probe)
	}
	res, e := s.opts.HTTPClient.Do(req)
	if e != nil {
		return 0, "", nil, e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, (256<<10)+1))
	if e != nil || len(b) > 256<<10 || !utf8.Valid(b) {
		return 0, "", nil, errors.New("bounded function response unavailable")
	}
	var result any
	if json.Unmarshal(b, &result) != nil {
		result = string(b)
	}
	return res.StatusCode, res.Header.Get("Content-Type"), result, nil
}
func (s *Service) checkFunctionManifest(ctx context.Context, i dataInstallation, v dataFunctionState, active map[string]string) error {
	secret, e := s.dataCredentials(i)
	if e != nil {
		return e
	}
	status, _, result, e := s.functionRequest(ctx, i, secret, "_luci-health", []byte(`{}`), secret.Anon, v.ProbeToken)
	m, e2 := manifestForFunctions(v, active)
	b, _ := json.Marshal(m)
	obj, ok := result.(map[string]any)
	if e != nil || e2 != nil || status != 200 || !ok || obj["runtimeVersion"] != dataFunctionRuntimeVersion || obj["manifestDigest"] != dataSum(b) {
		return errors.New("function route readback failed")
	}
	return nil
}
func (s *Service) probeFunction(ctx context.Context, i dataInstallation, v dataFunctionState, r DataFunctionRelease) error {
	secret, e := s.dataCredentials(i)
	if e != nil {
		return e
	}
	b, _ := json.Marshal(map[string]string{"releaseId": r.ReleaseID, "sourceDigest": r.SourceDigest})
	status, _, result, e := s.functionRequest(ctx, i, secret, "_luci-probe", b, secret.Anon, v.ProbeToken)
	obj, ok := result.(map[string]any)
	if e != nil || status != 200 || !ok || obj["runtimeVersion"] != dataFunctionRuntimeVersion || obj["releaseId"] != r.ReleaseID || obj["sourceDigest"] != r.SourceDigest {
		return errors.New("function compilation failed")
	}
	return nil
}
func copyFunctionRoutes(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (s *Service) runFunctionAction(ctx context.Context, i dataInstallation, in DataFunctionAction, name string, src DataFunctionSource) error {
	v, e := s.loadDataFunctions(i.ProjectID)
	if e != nil {
		return e
	}
	if e = s.functionEngine(ctx, i, &v); e != nil {
		return e
	}
	if e = s.verifyDataOwnership(ctx, i, false); e != nil {
		return e
	}
	if e = s.verifyFunctionHost(ctx, i, v, false); e != nil {
		return e
	}
	secret, e := s.dataCredentials(i)
	if e != nil {
		return e
	}
	if v.Pending != nil {
		// Interrupted promotion never repeats customer code. Restore the prior
		// durable map and require actual readback before clearing the journal.
		if e = s.prepareFunctionFiles(i, v, v.Pending.Before); e != nil {
			return e
		}
		if e = s.checkFunctionManifest(ctx, i, v, v.Pending.Before); e != nil {
			return e
		}
		v.Active = v.Pending.Before
		v.Pending = nil
		if e = s.write(i.ProjectID+"-data-functions", v); e != nil {
			return e
		}
	}
	if in.Action == "refresh" {
		if e = s.verifyFunctionContainer(ctx, i, v); e != nil {
			return e
		}
		return s.checkFunctionManifest(ctx, i, v, v.Active)
	}
	if in.Action == "publish" {
		if src.Name != name || src.Commit != in.Commit || !ValidDataFunctionAuth(src.AuthMode) || ValidateDataFunctionFiles(src.Files) != nil || DataFunctionDigest(src.AuthMode, src.Files) != src.SourceDigest {
			return errors.New("function source receipt changed")
		}
		r := DataFunctionRelease{ReleaseID: randomID("function-release-"), Commit: src.Commit, SourceDigest: src.SourceDigest, AuthMode: src.AuthMode, PublishedAt: now(), Status: "candidate", Message: "Committed source retained; activation is not verified yet."}
		file, e := s.functionArtifactFile(i.ProjectID, r.ReleaseID)
		if e != nil {
			return e
		}
		b, _ := json.Marshal(src)
		if e = s.write(filepath.Base(file), json.RawMessage(b)); e != nil {
			return e
		}
		v.Releases[name] = append(v.Releases[name], r)
		if e = s.write(i.ProjectID+"-data-functions", v); e != nil {
			return e
		}
	}
	// Regenerate only trusted files and recreate only Functions when adopting
	// the hardened dispatcher. Auth, DB, Storage and data volumes stay running.
	if s.verifyFunctionContainer(ctx, i, v) != nil {
		if e = s.prepareDataStack(i, secret); e != nil {
			return e
		}
		if _, e = s.compose(ctx, i, "up", "-d", "--no-deps", "--wait", "--wait-timeout", "240", "functions"); e != nil {
			return e
		}
	} else if e = s.prepareFunctionFiles(i, v, v.Active); e != nil {
		return e
	}
	if e = s.verifyFunctionContainer(ctx, i, v); e != nil {
		return e
	}
	if e = s.checkFunctionManifest(ctx, i, v, v.Active); e != nil {
		return e
	}
	var candidate DataFunctionRelease
	if in.Action == "publish" {
		candidate = v.Releases[name][len(v.Releases[name])-1]
	} else if in.Action == "rollback" {
		candidate, _ = functionRelease(v, name, in.ReleaseID)
	}
	if in.Action != "remove" {
		if e = s.probeFunction(ctx, i, v, candidate); e != nil {
			_ = s.changeDataFunctions(i.ProjectID, func(current *dataFunctionState) error {
				for n := range current.Releases[name] {
					if current.Releases[name][n].ReleaseID == candidate.ReleaseID && in.Action == "publish" {
						current.Releases[name][n].Status = "failed"
						current.Releases[name][n].Message = "Compilation or boot failed. The prior endpoint remains active."
					}
				}
				return nil
			})
			return e
		}
	}
	next := copyFunctionRoutes(v.Active)
	if in.Action == "remove" {
		delete(next, name)
	} else {
		next[name] = candidate.ReleaseID
	}
	before := copyFunctionRoutes(v.Active)
	v.Pending = &dataFunctionTransition{Before: before, After: next}
	if e = s.write(i.ProjectID+"-data-functions", v); e != nil {
		return e
	}
	if e = s.prepareFunctionFiles(i, v, next); e == nil {
		e = s.checkFunctionManifest(ctx, i, v, next)
	}
	if e != nil {
		recovery, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		if restore := s.prepareFunctionFiles(i, v, before); restore == nil {
			if restore = s.checkFunctionManifest(recovery, i, v, before); restore == nil {
				v.Pending = nil
				_ = s.write(i.ProjectID+"-data-functions", v)
			}
		}
		return e
	}
	v.Active = next
	v.Pending = nil
	for key, releases := range v.Releases {
		for n := range releases {
			if releases[n].Status == "failed" || (releases[n].Status == "candidate" && next[key] != releases[n].ReleaseID) {
				continue
			}
			releases[n].Status = "superseded"
			if next[key] == releases[n].ReleaseID {
				releases[n].Status = "active"
				releases[n].Message = "Runtime compilation and endpoint readback verified."
			} else if in.Action == "remove" && key == name && before[name] == releases[n].ReleaseID {
				releases[n].Status = "stopped"
				releases[n].Message = "Endpoint removed; source retained."
			}
		}
		v.Releases[key] = releases
	}
	return s.write(i.ProjectID+"-data-functions", v)
}

type functionClaims struct {
	Role string `json:"role"`
	Sub  string `json:"sub"`
}

func decodeFunctionClaims(encoded string) (functionClaims, error) {
	var c functionClaims
	b, e := base64.RawURLEncoding.DecodeString(encoded)
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
