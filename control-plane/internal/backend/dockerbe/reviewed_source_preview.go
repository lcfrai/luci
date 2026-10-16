package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// These are whole-snapshot limits, before capacity reservation or Create. The
// normalized tar (including headers/padding) must fit as well as its file data.
// Static bytes are smaller so fixed argv fits below Linux's minimum ARG_MAX;
// each individual argument is also below MAX_ARG_STRLEN. No shell is involved.
const sourceSnapshotLimit = 512 << 10
const sourceSnapshotFiles = 200
const sourceAssetLimit = 64 << 10
const sourceArgumentChunk = 24 << 10
const sourceCommandLimit = 120 << 10

type sourceAsset struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data []byte `json:"data"`
}

// This program is Luci-owned. Project JavaScript is only returned as bytes to
// the browser, never required, evaluated, or executed on the serving runtime.
// No filesystem, child process, credential or provider access is used.
const sourcePreviewServer = `"use strict";
const http=require("node:http"),crypto=require("node:crypto");
const [port,payloadDigest,sourceDigest,snapshotDigest,...chunks]=process.argv.slice(1);
const raw=chunks.join("");
if(crypto.createHash("sha256").update(raw).digest("hex")!==payloadDigest)process.exit(2);
const files=new Map(JSON.parse(raw).map(f=>["/"+f.name,{type:f.type,data:Buffer.from(f.data||"","base64")}])) ;
const server=http.createServer({maxHeaderSize:8192},(req,res)=>{
  const target=req.url||"";const name=target.split("?")[0];
  if(target.length>4096||target.includes("#")||name.includes("%")||name.includes("\\")||
     (name!=="/"&&!/^\/[A-Za-z0-9][A-Za-z0-9._-]*(\/[A-Za-z0-9][A-Za-z0-9._-]*)*$/.test(name))||
     name.split("/").some(x=>x==="."||x==="..")){res.writeHead(400);res.end();return;}
  if(req.method!=="GET"&&req.method!=="HEAD"){res.writeHead(405,{Allow:"GET, HEAD"});res.end();return;}
  const file=files.get(name==="/"?"/index.html":name);
  if(!file){res.writeHead(404);res.end();return;}
  res.writeHead(200,{"Content-Type":file.type,"Content-Length":file.data.length,
    "Cache-Control":"no-store","X-Content-Type-Options":"nosniff",
    "X-Luci-Source-Digest":sourceDigest,"X-Luci-Snapshot-Digest":snapshotDigest});
  res.end(req.method==="HEAD"?undefined:file.data);
});
server.maxConnections=32;server.requestTimeout=5000;server.headersTimeout=5000;server.keepAliveTimeout=1000;
server.listen(Number(port),"0.0.0.0");
for(const signal of ["SIGTERM","SIGINT"])process.on(signal,()=>{server.close(()=>process.exit(0));server.closeAllConnections();});`

func sourceAssetType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return ""
	}
}

func sourceAssetPath(name string) bool {
	if !previewContextPath(name) || len(name) > 240 {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part[0] == '.' {
			return false
		}
		switch strings.ToLower(part) {
		case "credentials", "credentials.json", "secrets.json", "id_rsa", "id_ed25519", "id_dsa", "id_ecdsa":
			return false
		}
		for i, c := range []byte(part) {
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				continue
			}
			if i > 0 && (c == '.' || c == '_' || c == '-') {
				continue
			}
			return false
		}
	}
	return true
}

func sourceDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sourcePreviewCommand(p backend.PreparedPreview) []string {
	args := []string{"--max-old-space-size=128", "-e", sourcePreviewServer, "--",
		strconv.Itoa(p.Spec.Port), sourceDigest(p.SourcePayload), p.Spec.Identity.SourceDigest, p.ContextDigest}
	for raw := string(p.SourcePayload); len(raw) > 0; {
		n := len(raw)
		if n > sourceArgumentChunk {
			n = sourceArgumentChunk
		}
		args = append(args, raw[:n])
		raw = raw[n:]
	}
	return args
}

func sourceCommandDigest(args []string) string {
	raw, _ := json.Marshal(args)
	return sourceDigest(raw)
}

func (b *Backend) prepareReviewedSource(ctx context.Context, p backend.PreparedPreview) (backend.PreparedPreview, error) {
	s := p.Spec
	if s.Identity.Kind != backend.ReviewedSourcePreview || s.Dockerfile != "" || len(s.Environment) != 0 || strings.TrimSpace(b.resourceImage) == "" {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	files, digest, err := previewContextBounded(s, sourceSnapshotLimit, sourceSnapshotFiles)
	if err != nil {
		return backend.PreparedPreview{}, err
	}
	raw, err := previewTar(files)
	if err != nil || len(raw) > sourceSnapshotLimit {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	if s.Identity.SnapshotDigest != "" && s.Identity.SnapshotDigest != digest {
		return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
	}
	assets, size, index := []sourceAsset{}, 0, false
	for _, file := range files {
		if file.directory {
			continue
		}
		typ := sourceAssetType(file.name)
		if typ == "" {
			continue
		}
		// Hidden metadata and private configuration never become browser assets.
		if strings.HasPrefix(file.name, ".") || strings.Contains(file.name, "/.") {
			continue
		}
		if !sourceAssetPath(file.name) {
			return backend.PreparedPreview{}, backend.ErrUnsupported
		}
		size += len(file.data)
		if size > sourceAssetLimit {
			return backend.PreparedPreview{}, backend.ErrUnsupported
		}
		index = index || file.name == "index.html"
		assets = append(assets, sourceAsset{Name: file.name, Type: typ, Data: file.data})
	}
	if !index {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	p.SourcePayload, err = json.Marshal(assets)
	if err != nil {
		return backend.PreparedPreview{}, err
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	// ResourceImage is the server-selected trusted Luci sandbox runtime. Its
	// production Dockerfile is Node based. Never pull, accept a browser image, or
	// mistake this runtime image for an application image built from the source.
	image, err := images.ImageInspect(ctx, b.resourceImage)
	if err != nil {
		return backend.PreparedPreview{}, backend.ErrUnavailable
	}
	if !previewImageID(image.ID) || image.Os != "linux" || image.Size <= 0 || image.Config == nil || len(image.Config.Volumes) != 0 || len(image.Config.OnBuild) != 0 {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	// The supported trusted runtime is the repository's Node 24 sandbox image.
	// This is an immutable image-metadata capability check, not evidence that a
	// process has executed. Reject alternate image environments before allocation.
	node24, environmentBytes := false, 0
	seenEnvironment := map[string]bool{}
	for _, env := range image.Config.Env {
		environmentBytes += len(env) + 1
		key, value, ok := strings.Cut(env, "=")
		if !ok || seenEnvironment[key] || environmentBytes > 4096 || strings.ContainsAny(env, "\x00\r\n") {
			return backend.PreparedPreview{}, backend.ErrUnsupported
		}
		seenEnvironment[key] = true
		switch key {
		case "NODE_VERSION":
			parts := strings.Split(value, ".")
			node24 = len(parts) == 3 && parts[0] == "24"
			for _, part := range parts {
				if part == "" {
					node24 = false
				}
				for _, ch := range part {
					if ch < '0' || ch > '9' {
						node24 = false
					}
				}
			}
		case "PATH", "HOME", "YARN_VERSION", "NODE_ENV":
		case "NODE_OPTIONS", "NODE_PATH":
			if value != "" {
				return backend.PreparedPreview{}, backend.ErrUnsupported
			}
		default:
			return backend.PreparedPreview{}, backend.ErrUnsupported
		}
	}
	if !node24 {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	// Docker otherwise merges image Env into ContainerConfig.Env. Resolve the
	// complete declared environment now, bind its exact unique-key multiset and
	// pass every entry explicitly. Runtime version entries remain intact.
	environment := append([]string(nil), image.Config.Env...)
	for _, override := range sourcePreviewEnvironment {
		key, _, _ := strings.Cut(override, "=")
		filtered := environment[:0]
		for _, entry := range environment {
			if !strings.HasPrefix(entry, key+"=") {
				filtered = append(filtered, entry)
			}
		}
		environment = append(filtered, override)
	}
	var validEnvironment bool
	p.SourceEnvironment, validEnvironment = canonicalSourceEnvironment(environment)
	if !validEnvironment {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	environmentDigest := sourceEnvironmentDigest(p.SourceEnvironment)
	if s.Identity.EnvironmentDigest != "" && s.Identity.EnvironmentDigest != environmentDigest {
		return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
	}
	p.Spec.Identity.EnvironmentDigest = environmentDigest
	if s.ImageID != "" && s.ImageID != image.ID || s.Identity.RuntimeImageID != "" && s.Identity.RuntimeImageID != image.ID {
		return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
	}
	p.Spec.ImageID, p.Spec.Identity.SnapshotDigest, p.Spec.Identity.RuntimeImageID = image.ID, digest, image.ID
	p.ContextDigest, p.Spec.ContextDigest, p.PreparedContext = digest, digest, raw
	// The immutable in-memory snapshot outlives the product's temporary archive.
	// A repeat Prepare compares these exact bytes, not a mutable project path.
	p.Spec.SourceDirectory, p.Spec.SourceArchive = "", raw
	p.RequiredDiskBytes = uint64(image.Size) + (256 << 20)
	command := sourcePreviewCommand(p)
	commandDigest := sourceCommandDigest(command)
	if s.Identity.CommandDigest != "" && s.Identity.CommandDigest != commandDigest {
		return backend.PreparedPreview{}, backend.ErrLaunchFenceOwnership
	}
	p.Spec.Identity.CommandDigest = commandDigest
	total := 0
	for _, arg := range command {
		total += len(arg) + 1
	}
	if total > sourceCommandLimit {
		return backend.PreparedPreview{}, backend.ErrUnsupported
	}
	return p, ctx.Err()
}

var sourcePreviewEnvironment = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/nonexistent", "NODE_OPTIONS=", "NODE_PATH=", "NODE_ENV=production"}

func canonicalSourceEnvironment(environment []string) ([]string, bool) {
	seen, total := map[string]bool{}, 0
	canonical := make([]string, 0, len(environment))
	for _, entry := range environment {
		key, _, ok := strings.Cut(entry, "=")
		total += len(entry) + 1
		if !ok || seen[key] || total > 4096 || strings.ContainsAny(entry, "\x00\r\n") {
			return nil, false
		}
		switch key {
		case "PATH", "HOME", "NODE_OPTIONS", "NODE_PATH", "NODE_ENV", "NODE_VERSION", "YARN_VERSION":
		default:
			return nil, false
		}
		seen[key] = true
		canonical = append(canonical, entry)
	}
	sort.Strings(canonical)
	return canonical, true
}

func sourceEnvironmentDigest(environment []string) string {
	raw, _ := json.Marshal(environment)
	return sourceDigest(raw)
}

func configureReviewedSource(p backend.PreparedPreview, config *container.Config, host *container.HostConfig) {
	config.Entrypoint, config.Cmd = []string{"/usr/local/bin/node"}, sourcePreviewCommand(p)
	config.User, config.WorkingDir, config.Env = "1001:1001", "/", append([]string(nil), p.SourceEnvironment...)
	config.Healthcheck, config.StopSignal = &container.HealthConfig{Test: []string{"NONE"}}, "SIGTERM"
	config.Labels[previewPrefix+"command"] = sourceCommandDigest(config.Cmd)
	host.ReadonlyRootfs = true
}

func ownedReviewedSource(v backend.PreviewIdentity, cfg *container.Config, host *container.HostConfig, port int) bool {
	if v.Kind != backend.ReviewedSourcePreview || cfg.Image != v.RuntimeImageID || !fenceHex(v.SnapshotDigest, 32) || !host.ReadonlyRootfs || cfg.User != "1001:1001" || cfg.WorkingDir != "/" || cfg.StopSignal != "SIGTERM" || cfg.Healthcheck == nil || !reflect.DeepEqual(cfg.Healthcheck.Test, []string{"NONE"}) || !reflect.DeepEqual([]string(cfg.Entrypoint), []string{"/usr/local/bin/node"}) || len(cfg.Cmd) < 9 {
		return false
	}
	args := []string(cfg.Cmd)
	if args[0] != "--max-old-space-size=128" || args[1] != "-e" || args[2] != sourcePreviewServer || args[3] != "--" || args[4] != strconv.Itoa(port) || args[6] != v.SourceDigest || args[7] != v.SnapshotDigest || cfg.Labels[previewPrefix+"command"] != v.CommandDigest || v.CommandDigest != sourceCommandDigest(args) {
		return false
	}
	var payload strings.Builder
	total := 0
	for i, arg := range args {
		total += len(arg) + 1
		if total > sourceCommandLimit {
			return false
		}
		if i >= 8 {
			if len(arg) > sourceArgumentChunk {
				return false
			}
			payload.WriteString(arg)
		}
	}
	if sourceDigest([]byte(payload.String())) != args[5] {
		return false
	}
	environment, valid := canonicalSourceEnvironment(cfg.Env)
	if !valid || sourceEnvironmentDigest(environment) != v.EnvironmentDigest {
		return false
	}
	for _, want := range sourcePreviewEnvironment {
		key, _, _ := strings.Cut(want, "=")
		got := ""
		for _, env := range environment {
			if strings.HasPrefix(env, key+"=") {
				got = env
				break
			}
		}
		if got != want {
			return false
		}
	}
	return true
}
