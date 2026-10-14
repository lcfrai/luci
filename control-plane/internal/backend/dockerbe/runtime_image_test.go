package dockerbe

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// These fixtures exercise SDK calls and response ownership only. They never
// open a Docker endpoint, invoke a process, or compile the bundled source.
type runtimeImageProbe struct {
	*checkProbe
	images       map[string]image.InspectResponse
	bases        map[string]image.InspectResponse
	pullStreams  map[string]string
	pullBody     func(context.Context, string) io.ReadCloser
	pullError    error
	pulls        []string
	pullOptions  []image.PullOptions
	buildStream  string
	buildBody    io.ReadCloser
	buildError   error
	buildHook    func(context.Context)
	buildOptions []build.ImageBuildOptions
	contexts     [][]byte
	buildBudget  time.Duration
	tags         [][2]string
	tagError     error
	tagNoEffect  bool
	mutations    []string
	lists        []container.ListOptions
	listError    error
	killed       []string
	killHook     func(string) error
}

func (f *runtimeImageProbe) ContainerKill(_ context.Context, id, signal string) error {
	if signal != "SIGKILL" {
		return errors.New("unexpected cancellation signal")
	}
	f.killed = append(f.killed, id)
	if f.killHook != nil {
		return f.killHook(id)
	}
	return checkMissing{}
}

func (f *runtimeImageProbe) ImageInspect(_ context.Context, ref string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
	if row, ok := f.images[ref]; ok {
		return row, nil
	}
	return image.InspectResponse{}, checkMissing{}
}

func (f *runtimeImageProbe) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	f.pulls = append(f.pulls, ref)
	f.pullOptions = append(f.pullOptions, options)
	f.mutations = append(f.mutations, "pull:"+ref)
	if f.pullError != nil {
		return nil, f.pullError
	}
	// An image may become visible even when its response is lost. Tests must
	// not confuse that visibility with acknowledged pull completion.
	if row, ok := f.bases[ref]; ok {
		f.images[ref], f.images[row.ID] = row, row
	}
	if f.pullBody != nil {
		return f.pullBody(ctx, ref), nil
	}
	stream, exists := f.pullStreams[ref]
	if !exists {
		data, _ := json.Marshal(map[string]string{"status": "Status: Downloaded newer image for " + ref})
		stream = string(data) + "\n"
	}
	return io.NopCloser(strings.NewReader(stream)), nil
}

func (f *runtimeImageProbe) ImageBuild(ctx context.Context, source io.Reader, options build.ImageBuildOptions) (build.ImageBuildResponse, error) {
	f.mutations = append(f.mutations, "build")
	f.buildOptions = append(f.buildOptions, options)
	data, err := io.ReadAll(source)
	if err != nil {
		return build.ImageBuildResponse{}, err
	}
	f.contexts = append(f.contexts, data)
	if deadline, ok := ctx.Deadline(); ok {
		f.buildBudget = time.Until(deadline)
	}
	if f.buildHook != nil {
		f.buildHook(ctx)
	}
	if f.buildError != nil {
		return build.ImageBuildResponse{}, f.buildError
	}
	body := f.buildBody
	if body == nil {
		body = io.NopCloser(strings.NewReader(f.buildStream))
	}
	return build.ImageBuildResponse{Body: body, OSType: "linux"}, nil
}

func (f *runtimeImageProbe) ImageTag(_ context.Context, source, target string) error {
	f.mutations = append(f.mutations, "tag:"+target)
	f.tags = append(f.tags, [2]string{source, target})
	if f.tagError != nil {
		return f.tagError
	}
	row, ok := f.images[source]
	if !ok {
		return checkMissing{}
	}
	if !f.tagNoEffect {
		f.images[target] = row
	}
	return nil
}

func (f *runtimeImageProbe) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	f.lists = append(f.lists, options)
	if f.listError != nil {
		return nil, f.listError
	}
	return f.checkProbe.ContainerList(ctx, options)
}

const runtimeFixtureDockerfile = `FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY sandboxd ./sandboxd
RUN printf prepare \
    && true
FROM node:24-bookworm-slim
RUN true
COPY --from=build /out/sandboxd /usr/local/bin/sandboxd
COPY --from=build /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
USER dev
WORKDIR /home/dev
RUN true
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
`

func runtimeFixtureWrite(t *testing.T, root, relative, contents string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func newRuntimeImageProbe(t *testing.T) (*Backend, *runtimeImageProbe, backend.RuntimeImageSpec) {
	t.Helper()
	b, old, _ := newCheckProbe(t)
	f := &runtimeImageProbe{
		checkProbe:  old,
		images:      map[string]image.InspectResponse{},
		bases:       map[string]image.InspectResponse{},
		pullStreams: map[string]string{},
	}
	for i, base := range []string{"golang:1.26-bookworm", "node:24-bookworm-slim"} {
		row := old.image
		row.ID = "sha256:" + strings.Repeat([]string{"b", "c"}[i], 64)
		cfg := *row.Config
		row.Config = &cfg
		f.bases[base] = row
	}
	b.api = f
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtimeFixtureWrite(t, directory, "images/sandbox/Dockerfile", runtimeFixtureDockerfile)
	runtimeFixtureWrite(t, directory, "images/sandbox/entrypoint.sh", "#!/bin/sh\nexec sandboxd\n")
	runtimeFixtureWrite(t, directory, "sandboxd/go.mod", "module fixture/sandboxd\n\ngo 1.25.0\n")
	runtimeFixtureWrite(t, directory, "sandboxd/vendor/modules.txt", "# controlled vendored fixture\n")
	runtimeFixtureWrite(t, directory, "sandboxd/cmd/sandboxd/main.go", "package main\nfunc main() {}\n")
	spec := backend.RuntimeImageSpec{
		Identity:        backend.RuntimeImageIdentity{Protocol: backend.RuntimeImageExecutionProtocol, AllocationID: "runtime-allocation-one", JobID: "runtime-job-one", ScopeDigest: strings.Repeat("a", 64)},
		Resources:       backend.Resources{MemoryMiB: 1536, CPUMillis: 1250},
		SourceDirectory: directory,
		ImageTag:        "luci/sandbox:dev",
	}
	return b, f, spec
}

func runtimeFixturePrepare(t *testing.T, b *Backend, spec backend.RuntimeImageSpec) backend.PreparedRuntimeImage {
	t.Helper()
	p, err := b.PrepareRuntimeImage(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRuntimeImageColdPullDemandIsSeparateFromRunCap(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	cold := runtimeFixturePrepare(t, b, spec)
	if cold.AdmissionMemoryMiB != spec.Resources.MemoryMiB+2*runtimeColdBaseAdmissionMiB {
		t.Fatal("two absent pinned bases were not charged before build", cold.AdmissionMemoryMiB)
	}
	for base, row := range f.bases {
		f.images[base] = row
	}
	warm := runtimeFixturePrepare(t, b, spec)
	if warm.AdmissionMemoryMiB != spec.Resources.MemoryMiB || warm.Spec.Resources != spec.Resources {
		t.Fatal("warm bases or hard RUN limits changed", warm)
	}
}

func runtimeFixtureOutput(f *runtimeImageProbe, p backend.PreparedRuntimeImage) string {
	row := f.image
	row.ID = "sha256:" + strings.Repeat("d", 64)
	cfg := *row.Config
	cfg.Labels = runtimeIdentityLabels(p.Spec.Identity)
	row.Config = &cfg
	f.images[row.ID] = row
	return row.ID
}

func runtimeFixtureJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func runtimeFixtureAux(t *testing.T, id string) string {
	t.Helper()
	return runtimeFixtureJSON(t, map[string]any{"aux": map[string]string{"ID": id}})
}

func runtimeFixtureSuccess(t *testing.T, id string) string {
	t.Helper()
	stage := "sha256:" + strings.Repeat("e", 64)
	return runtimeFixtureAux(t, stage) + runtimeFixtureAux(t, id) + runtimeFixtureJSON(t, map[string]string{"stream": "Successfully built " + strings.TrimPrefix(id, "sha256:")[:12] + "\n"})
}

func runtimeFixtureTar(t *testing.T, data []byte) map[string]string {
	t.Helper()
	files := map[string]string{}
	r := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := r.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		body, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		files[h.Name] = string(body)
	}
}

func assertRuntimeNoMutation(t *testing.T, f *runtimeImageProbe) {
	t.Helper()
	if len(f.mutations)+len(f.creates)+len(f.starts)+len(f.removes) != 0 {
		t.Fatalf("unexpected mutation: SDK=%v creates=%v starts=%v removes=%v", f.mutations, f.creates, f.starts, f.removes)
	}
}

func TestRuntimeImagePrepareColdIsReadOnlyAndExcludesSurroundingCheckout(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	runtimeFixtureWrite(t, spec.SourceDirectory, ".git/config", "unrelated git state")
	runtimeFixtureWrite(t, spec.SourceDirectory, "docs/private.txt", "unrelated document")
	p := runtimeFixturePrepare(t, b, spec)
	if p.RequiredDiskBytes != 32<<30 || p.ContextDigest == "" || p.Spec.Identity.SourceDigest != p.ContextDigest || p.Spec.Identity.EngineID != f.info.ID || p.Spec.Identity.InstanceID != b.instance || p.Spec.ImageTag != "docker.io/luci/sandbox:dev" {
		t.Fatalf("incomplete cold preparation: %+v", p)
	}
	if !reflect.DeepEqual(p.BaseImages, []string{"golang:1.26-bookworm", "node:24-bookworm-slim"}) {
		t.Fatalf("unexpected allowed bases: %v", p.BaseImages)
	}
	files := runtimeFixtureTar(t, p.PreparedContext)
	if len(files) != 5 || files[".git/config"] != "" || files["docs/private.txt"] != "" {
		t.Fatalf("context included unrelated files: %v", files)
	}
	dockerfile := files["images/sandbox/Dockerfile"]
	if !strings.Contains(dockerfile, "FROM golang:1.26-bookworm AS build\nLABEL ") || !strings.Contains(dockerfile, "FROM node:24-bookworm-slim\nLABEL ") || strings.Count(dockerfile, `luci.runtime-image-allocation="runtime-allocation-one"`) != 2 {
		t.Fatalf("ownership does not precede each stage: %s", dockerfile)
	}
	runtimeFixtureWrite(t, spec.SourceDirectory, "docs/private.txt", "unrelated changed document")
	again := runtimeFixturePrepare(t, b, spec)
	if !reflect.DeepEqual(p, again) {
		t.Fatal("unrelated checkout content changed the trusted context")
	}
	assertRuntimeNoMutation(t, f)
}

func TestRuntimeImageColdBuildPullsOnlyApprovedBasesThenPinsAndBoundsBuild(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	p := runtimeFixturePrepare(t, b, spec)
	id := runtimeFixtureOutput(f, p)
	f.buildStream = runtimeFixtureSuccess(t, id)
	result, err := b.BuildRuntimeImage(context.Background(), p)
	if err != nil || !result.Settled || result.ImageID != id {
		t.Fatalf("cold build: %+v, %v", result, err)
	}
	if !reflect.DeepEqual(f.mutations, []string{"pull:golang:1.26-bookworm", "pull:node:24-bookworm-slim", "build"}) || len(f.buildOptions) != 1 {
		t.Fatalf("wrong dispatch order or replay: %v", f.mutations)
	}
	for _, options := range f.pullOptions {
		if options.All || options.RegistryAuth != "" || options.PrivilegeFunc != nil {
			t.Fatalf("pull broadened its authority/options: %+v", options)
		}
	}
	opts := f.buildOptions[0]
	if opts.Version != build.BuilderV1 || len(opts.Tags) != 0 || opts.Dockerfile != "images/sandbox/Dockerfile" || !opts.Remove || !opts.ForceRemove || !opts.NoCache || opts.PullParent || opts.Memory != int64(1536)<<20 || opts.MemorySwap != opts.Memory || opts.CPUPeriod != 100000 || opts.CPUQuota != 125000 {
		t.Fatalf("build changed limits, ownership or publication: %+v", opts)
	}
	if opts.SecurityOpt != nil || len(opts.Ulimits) != 1 || opts.Ulimits[0].Name != "nproc" || opts.Ulimits[0].Hard != 256 || opts.Ulimits[0].Soft != 256 || opts.NetworkMode != "default" {
		t.Fatalf("build execution options changed: %+v", opts)
	}
	if f.buildBudget <= 0 || f.buildBudget > 30*time.Minute {
		t.Fatalf("missing trusted build deadline: %s", f.buildBudget)
	}
	dockerfile := runtimeFixtureTar(t, f.contexts[0])["images/sandbox/Dockerfile"]
	for base, row := range f.bases {
		if strings.Contains(dockerfile, "FROM "+base) || !strings.Contains(dockerfile, "FROM "+row.ID) {
			t.Fatalf("base %s remained mutable: %s", base, dockerfile)
		}
	}
	if !strings.Contains(dockerfile, "COPY --from=build /usr/local/go /usr/local/go") || !strings.Contains(dockerfile, "RUN printf prepare \\\n    && true") {
		t.Fatal("trusted stage references or shell continuation changed")
	}
	if !strings.Contains(runtimeFixtureTar(t, p.PreparedContext)["images/sandbox/Dockerfile"], "FROM golang:1.26-bookworm AS build") {
		t.Fatal("build mutated the retained prepared receipt")
	}
	if len(f.lists) != 1 || !f.lists[0].All || f.lists[0].Limit != 0 || !reflect.DeepEqual(f.lists[0].Filters.Get("label"), []string{"luci.runtime-image-allocation=runtime-allocation-one"}) {
		t.Fatalf("incomplete or overfiltered intermediate inventory: %+v", f.lists)
	}
}

func TestRuntimeImageBuildPreservesShorterCallerDeadlineAndWarmBases(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	for base, row := range f.bases {
		f.images[base] = row
	}
	p := runtimeFixturePrepare(t, b, spec)
	f.buildStream = runtimeFixtureSuccess(t, runtimeFixtureOutput(f, p))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := b.BuildRuntimeImage(ctx, p)
	if err != nil || !result.Settled || len(f.pulls) != 0 || f.buildBudget <= 0 || f.buildBudget > time.Minute {
		t.Fatalf("warm build/deadline: %+v %v, pulls=%v budget=%s", result, err, f.pulls, f.buildBudget)
	}
}

func TestRuntimeImageBuildRefusesChangedSourceOrPreparedContextBeforeMutation(t *testing.T) {
	for _, mode := range []string{"source", "prepared-context", "digest"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			p := runtimeFixturePrepare(t, b, spec)
			switch mode {
			case "source":
				runtimeFixtureWrite(t, spec.SourceDirectory, "sandboxd/cmd/sandboxd/main.go", "package main\nfunc main() { panic(1) }\n")
			case "prepared-context":
				p.PreparedContext = append(append([]byte(nil), p.PreparedContext...), 'x')
			case "digest":
				p.ContextDigest = strings.Repeat("f", 64)
			}
			result, err := b.BuildRuntimeImage(context.Background(), p)
			if err == nil || result.Settled {
				t.Fatalf("changed input accepted: %+v %v", result, err)
			}
			assertRuntimeNoMutation(t, f)
		})
	}
}

func TestRuntimeImageContextRejectsExtraBuildAuthority(t *testing.T) {
	cases := map[string]string{
		"alternate-base": strings.Replace(runtimeFixtureDockerfile, "golang:1.26-bookworm", "other:latest", 1),
		"dynamic-base":   strings.Replace(runtimeFixtureDockerfile, "golang:1.26-bookworm", "$BASE", 1),
		"third-stage":    runtimeFixtureDockerfile + "FROM node:24-bookworm-slim\n",
		"external-copy":  strings.Replace(runtimeFixtureDockerfile, "COPY --from=build /out/sandboxd", "COPY --from=foreign:latest /out/sandboxd", 1),
		"label-override": runtimeFixtureDockerfile + "LABEL luci.runtime-image-allocation=foreign\n",
		"remote-add":     runtimeFixtureDockerfile + "ADD https://example.invalid/payload /payload\n",
		"onbuild":        runtimeFixtureDockerfile + "ONBUILD RUN true\n",
		"buildkit":       "# syntax=docker/dockerfile:1\n" + runtimeFixtureDockerfile,
		"escape":         "# escape=`\n" + runtimeFixtureDockerfile,
		"heredoc":        runtimeFixtureDockerfile + "RUN <<EOF\ntrue\nEOF\n",
		"volume":         runtimeFixtureDockerfile + "VOLUME /hidden\n",
		"missing-copy":   strings.Replace(runtimeFixtureDockerfile, "COPY sandboxd ./sandboxd\n", "", 1),
	}
	for name, dockerfile := range cases {
		t.Run(name, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			runtimeFixtureWrite(t, spec.SourceDirectory, "images/sandbox/Dockerfile", dockerfile)
			if _, err := b.PrepareRuntimeImage(context.Background(), spec); err == nil {
				t.Fatal("unsupported Dockerfile reached admission")
			}
			assertRuntimeNoMutation(t, f)
		})
	}
}

func TestRuntimeImageContextRefusesLinkedMissingAndOversizedInputs(t *testing.T) {
	for _, mode := range []string{"symlink", "hardlink", "missing-vendor", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			switch mode {
			case "symlink", "hardlink":
				outside := filepath.Join(t.TempDir(), "outside.go")
				if err := os.WriteFile(outside, []byte("private outside bytes"), 0600); err != nil {
					t.Fatal(err)
				}
				link := os.Link
				if mode == "symlink" {
					link = os.Symlink
				}
				if err := link(outside, filepath.Join(spec.SourceDirectory, "sandboxd", "linked.go")); err != nil {
					t.Skipf("host does not support this controlled %s fixture: %v", mode, err)
				}
			case "missing-vendor":
				if err := os.Remove(filepath.Join(spec.SourceDirectory, "sandboxd", "vendor", "modules.txt")); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				file, err := os.Create(filepath.Join(spec.SourceDirectory, "sandboxd", "large.go"))
				if err != nil {
					t.Fatal(err)
				}
				err = file.Truncate((64 << 20) + 1)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("sparse size fixture: %v %v", err, closeErr)
				}
			}
			if _, err := b.PrepareRuntimeImage(context.Background(), spec); err == nil {
				t.Fatal("unsafe/incomplete context reached admission")
			}
			assertRuntimeNoMutation(t, f)
		})
	}
}

type runtimeFixtureBody struct {
	io.Reader
	readError, closeError error
	closed                int
}

func (r *runtimeFixtureBody) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF && r.readError != nil {
		return n, r.readError
	}
	return n, err
}

func (r *runtimeFixtureBody) Close() error {
	r.closed++
	return r.closeError
}

func TestRuntimeImageBuildRequiresTerminalTransportAndExactSettlement(t *testing.T) {
	for _, mode := range []string{"success", "aux-only", "no-terminal", "mismatched-marker", "later-aux", "truncated", "transport-error", "close-error", "oversized", "error-eof", "error-after-success", "intermediate-remains", "error-intermediate-remains", "inventory-error", "wrong-output-label", "wrong-output-id", "engine-change", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			p := runtimeFixturePrepare(t, b, spec)
			id := runtimeFixtureOutput(f, p)
			stream := runtimeFixtureSuccess(t, id)
			body := &runtimeFixtureBody{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "aux-only":
				stream = runtimeFixtureAux(t, id)
			case "no-terminal":
				stream = runtimeFixtureJSON(t, map[string]string{"stream": "working\n"})
			case "mismatched-marker":
				stream = runtimeFixtureAux(t, id) + runtimeFixtureJSON(t, map[string]string{"stream": "Successfully built ffffffffffff\n"})
			case "later-aux":
				stream += runtimeFixtureAux(t, "sha256:"+strings.Repeat("f", 64))
			case "truncated":
				stream += "{"
			case "transport-error":
				body.readError = errors.New("controlled stream transport loss")
			case "close-error":
				body.closeError = errors.New("controlled response close failure")
			case "oversized":
				stream += strings.Repeat(" ", (8<<20)+1)
			case "error-eof":
				stream = runtimeFixtureJSON(t, map[string]string{"error": "controlled compile failure"})
			case "error-after-success":
				stream += runtimeFixtureJSON(t, map[string]any{"errorDetail": map[string]string{"message": "controlled terminal failure"}})
			case "intermediate-remains", "error-intermediate-remains":
				ref := strings.Repeat("f", 64)
				// Other labels deliberately disagree. Filtering by every identity
				// field would conceal this surviving allocation container.
				f.rows[ref] = container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: ref, State: &container.State{Status: "exited"}}, Config: &container.Config{Labels: map[string]string{"luci.runtime-image-allocation": p.Spec.Identity.AllocationID, "luci.runtime-image-job": "different-job"}}}
				if mode == "error-intermediate-remains" {
					stream = runtimeFixtureJSON(t, map[string]string{"error": "controlled failed cleanup"})
				}
			case "inventory-error":
				f.listError = errors.New("controlled incomplete inventory")
			case "wrong-output-label":
				f.images[id].Config.Labels["luci.runtime-image-source"] = strings.Repeat("f", 64)
			case "wrong-output-id":
				row := f.images[id]
				row.ID = "sha256:" + strings.Repeat("f", 64)
				f.images[id] = row
			case "engine-change":
				f.buildHook = func(context.Context) { f.info.ID = "replacement-engine" }
			case "cancelled":
				f.buildHook = func(context.Context) { cancel() }
			}
			body.Reader = strings.NewReader(stream)
			f.buildBody = body
			result, err := b.BuildRuntimeImage(ctx, p)
			if mode == "success" {
				if err != nil || !result.Settled || result.ImageID != id {
					t.Fatalf("successful receipt: %+v %v", result, err)
				}
			} else if mode == "error-eof" || mode == "error-after-success" {
				if err == nil || !result.Settled || result.ImageID != "" {
					t.Fatalf("complete failed build did not settle honestly: %+v %v", result, err)
				}
			} else if mode == "cancelled" {
				if err != nil || !result.Settled || result.ImageID != id {
					t.Fatalf("cancelled caller still received an exact terminal build: %+v %v", result, err)
				}
			} else if err == nil || result.Settled || result.ImageID != "" {
				t.Fatalf("uncertain/unowned result acknowledged: %+v %v", result, err)
			}
			if body.closed != 1 || len(f.buildOptions) != 1 || len(f.tags) != 0 || len(f.removes) != 0 {
				t.Fatalf("response/replay/cleanup ownership: closes=%d builds=%d tags=%v removes=%v", body.closed, len(f.buildOptions), f.tags, f.removes)
			}
		})
	}
}

func TestRuntimeImageCancelledOwnedBuilderRequiresTerminalFailureAndAbsence(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	p := runtimeFixturePrepare(t, b, spec)
	ref := strings.Repeat("d", 64)
	f.rows[ref] = container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
		ID: ref, State: &container.State{Status: "running"}},
		Config: &container.Config{Labels: runtimeIdentityLabels(p.Spec.Identity)}}
	reader, writer := io.Pipe()
	f.buildBody = reader
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.buildHook = func(context.Context) { cancel() }
	f.killHook = func(id string) error {
		if id != ref {
			return errors.New("foreign intermediate")
		}
		delete(f.rows, ref)
		_, err := io.WriteString(writer, runtimeFixtureJSON(t, map[string]string{"error": "owned RUN stopped"}))
		_ = writer.Close()
		return err
	}
	result, err := b.BuildRuntimeImage(ctx, p)
	if err == nil || !result.Settled || result.ImageID != "" || len(f.killed) != 1 || f.killed[0] != ref {
		t.Fatalf("exact cancelled build did not settle from terminal transport: %+v %v killed=%v", result, err, f.killed)
	}
}

func TestRuntimeImageCancelRefusesCollidingContainerWithoutKillingIt(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	p := runtimeFixturePrepare(t, b, spec)
	ref := strings.Repeat("e", 64)
	labels := runtimeIdentityLabels(p.Spec.Identity)
	labels[runtimeImagePrefix+"job"] = "unrelated-job"
	f.rows[ref] = container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
		ID: ref, State: &container.State{Status: "running"}},
		Config: &container.Config{Labels: labels}}
	request, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	monitor, stopMonitor := context.WithCancel(context.Background())
	defer stopMonitor()
	cancelled := false
	b.cancelOwnedRuntimeBuild(request, monitor, p.Spec.Identity, make(chan struct{}), func() { cancelled = true })
	if !cancelled || len(f.killed) != 0 || f.rows[ref].State.Status != "running" {
		t.Fatal("foreign/colliding container was mutated or treated as settled", cancelled, f.killed)
	}
}

func TestRuntimeImageCancellationAfterAcknowledgedPullStopsBeforeBuild(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	p := runtimeFixturePrepare(t, b, spec)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.pullBody = func(_ context.Context, base string) io.ReadCloser {
		cancel()
		return io.NopCloser(strings.NewReader(runtimeFixtureJSON(t,
			map[string]string{"status": "Status: Downloaded newer image for " + base})))
	}
	result, err := b.BuildRuntimeImage(ctx, p)
	if err == nil || !result.Settled || len(f.pulls) != 1 || len(f.buildOptions) != 0 {
		t.Fatalf("completed pull did not stop before an image build: %+v %v pulls=%v", result, err, f.pulls)
	}
}

func TestRuntimeImageFailedPullNeverSettlesFromImageVisibilityOrEmptyInventory(t *testing.T) {
	for _, mode := range []string{"error-eof", "no-terminal", "truncated", "transport-error", "close-error", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			p := runtimeFixturePrepare(t, b, spec)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body := &runtimeFixtureBody{}
			f.pullBody = func(_ context.Context, base string) io.ReadCloser {
				stream := runtimeFixtureJSON(t, map[string]string{"status": "Status: Downloaded newer image for " + base})
				switch mode {
				case "error-eof":
					stream = runtimeFixtureJSON(t, map[string]string{"error": "controlled pull failure"})
				case "no-terminal":
					stream = runtimeFixtureJSON(t, map[string]string{"status": "Extracting"})
				case "truncated":
					stream += "{"
				case "transport-error":
					body.readError = errors.New("controlled pull response loss")
				case "close-error":
					body.closeError = errors.New("controlled pull close error")
				case "cancelled":
					cancel()
					stream = runtimeFixtureJSON(t, map[string]string{"status": "Extracting"})
				}
				body.Reader = strings.NewReader(stream)
				return body
			}
			result, err := b.BuildRuntimeImage(ctx, p)
			if err == nil || result.Settled || len(f.pulls) != 1 || len(f.buildOptions) != 0 || len(f.tags) != 0 || len(f.lists) != 0 || body.closed != 1 {
				t.Fatalf("incomplete pull was promoted/replayed: %+v %v mutations=%v inventories=%d closes=%d", result, err, f.mutations, len(f.lists), body.closed)
			}
			if _, visible := f.images["golang:1.26-bookworm"]; !visible {
				t.Fatal("fixture did not expose the image despite incomplete transport")
			}
		})
	}
}

func TestRuntimeImageRefusesBaseOnBuildAndVolumeBeforeBuildDispatch(t *testing.T) {
	for _, cold := range []bool{false, true} {
		for _, metadata := range []string{"onbuild", "volume"} {
			name := metadata + "-warm"
			if cold {
				name = metadata + "-cold"
			}
			t.Run(name, func(t *testing.T) {
				b, f, spec := newRuntimeImageProbe(t)
				row := f.bases["golang:1.26-bookworm"]
				if metadata == "onbuild" {
					row.Config.OnBuild = []string{"RUN echo before-label"}
				} else {
					row.Config.Volumes = map[string]struct{}{"/unowned-volume": {}}
				}
				if cold {
					p := runtimeFixturePrepare(t, b, spec)
					result, err := b.BuildRuntimeImage(context.Background(), p)
					if err == nil || result.Settled || len(f.pulls) != 1 {
						t.Fatalf("unsafe pulled base accepted: %+v %v pulls=%v", result, err, f.pulls)
					}
				} else {
					f.images["golang:1.26-bookworm"] = row
					if _, err := b.PrepareRuntimeImage(context.Background(), spec); err == nil {
						t.Fatal("unsafe local base accepted")
					}
					assertRuntimeNoMutation(t, f)
				}
				if len(f.buildOptions) != 0 {
					t.Fatal("base metadata could execute before injected ownership labels")
				}
			})
		}
	}
}

func TestRuntimeImagePublicationRefusesForeignTargetAndAcknowledgesExactOutput(t *testing.T) {
	for _, mode := range []string{"absent", "same-output", "foreign-target", "tag-error", "tag-no-effect", "wrong-output-label"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			p := runtimeFixturePrepare(t, b, spec)
			id := runtimeFixtureOutput(f, p)
			foreign := f.image
			foreign.ID = "sha256:" + strings.Repeat("f", 64)
			switch mode {
			case "same-output":
				f.images[p.Spec.ImageTag] = f.images[id]
			case "foreign-target":
				f.images[p.Spec.ImageTag] = foreign
			case "tag-error":
				f.tagError = errors.New("controlled tag acknowledgement loss")
			case "tag-no-effect":
				f.tagNoEffect = true
			case "wrong-output-label":
				f.images[id].Config.Labels["luci.runtime-image-job"] = "other-job"
			}
			result, err := b.PublishRuntimeImage(context.Background(), p, id)
			if mode == "absent" || mode == "same-output" {
				if err != nil || !result.Acknowledged || result.ImageID != id || result.ImageTag != p.Spec.ImageTag {
					t.Fatalf("publication: %+v %v", result, err)
				}
				wantTags := 1
				if mode == "same-output" {
					wantTags = 0
				}
				if len(f.tags) != wantTags {
					t.Fatalf("wrong initial tag dispatch count: %v", f.tags)
				}
				again, repeatErr := b.PublishRuntimeImage(context.Background(), p, id)
				if repeatErr != nil || !again.Acknowledged || len(f.tags) != wantTags {
					t.Fatalf("same-output acknowledgement replayed mutation: %+v %v tags=%v", again, repeatErr, f.tags)
				}
			} else {
				if err == nil || result.Acknowledged {
					t.Fatalf("unsafe/unacknowledged publication accepted: %+v %v", result, err)
				}
				if mode == "foreign-target" && (len(f.tags) != 0 || f.images[p.Spec.ImageTag].ID != foreign.ID) {
					t.Fatal("observed foreign tag was overwritten")
				}
				if mode == "wrong-output-label" && len(f.tags) != 0 {
					t.Fatal("unowned output reached tag mutation")
				}
			}
			if len(f.pulls)+len(f.buildOptions)+len(f.lists)+len(f.removes) != 0 {
				t.Fatal("publication performed build/recovery work")
			}
		})
	}
}

func TestRuntimeImageObserveIsReadOnlyAndDoesNotInferBuildSettlement(t *testing.T) {
	for _, mode := range []string{"published", "image-absent", "tag-absent", "tag-different", "wrong-output-label", "engine-change"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newRuntimeImageProbe(t)
			p := runtimeFixturePrepare(t, b, spec)
			id := runtimeFixtureOutput(f, p)
			switch mode {
			case "published":
				f.images[p.Spec.ImageTag] = f.images[id]
			case "image-absent":
				delete(f.images, id)
			case "tag-different":
				f.images[p.Spec.ImageTag] = f.image
			case "wrong-output-label":
				f.images[id].Config.Labels["luci.runtime-image-scope"] = strings.Repeat("f", 64)
			case "engine-change":
				f.info.ID = "replacement-engine"
			}
			observation, err := b.ObserveRuntimeImage(context.Background(), p.Spec.Identity, id, p.Spec.ImageTag)
			if mode == "wrong-output-label" || mode == "engine-change" {
				if err == nil || observation.Publication != "" {
					t.Fatalf("foreign identity observed as owned: %+v %v", observation, err)
				}
			} else if err != nil || observation.Publication != mode || observation.ImageID != id || observation.ImageTag != p.Spec.ImageTag {
				t.Fatalf("publication observation: %+v %v", observation, err)
			}
			assertRuntimeNoMutation(t, f)
			if len(f.lists) != 0 {
				t.Fatal("publication observation attempted to infer settlement from container absence")
			}
		})
	}
}
