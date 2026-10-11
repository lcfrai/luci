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
	"strings"
	"testing"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type previewProbe struct {
	*checkProbe
	buildOptions []build.ImageBuildOptions
	contexts     [][]byte
	stream       string
	buildError   error
	images       map[string]image.InspectResponse
	stops        []string
}

func (f *previewProbe) ImageInspect(_ context.Context, id string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
	if r, ok := f.images[id]; ok {
		return r, nil
	}
	return image.InspectResponse{}, checkMissing{}
}
func (f *previewProbe) ImageBuild(_ context.Context, r io.Reader, o build.ImageBuildOptions) (build.ImageBuildResponse, error) {
	f.buildOptions = append(f.buildOptions, o)
	v, e := io.ReadAll(r)
	if e != nil {
		return build.ImageBuildResponse{}, e
	}
	f.contexts = append(f.contexts, v)
	return build.ImageBuildResponse{Body: io.NopCloser(strings.NewReader(f.stream)), OSType: "linux"}, f.buildError
}
func (f *previewProbe) ContainerStop(_ context.Context, ref string, _ container.StopOptions) error {
	f.stops = append(f.stops, ref)
	r, ok := f.rows[ref]
	if !ok {
		return checkMissing{}
	}
	r.State.Status, r.State.Running, r.State.Pid = "exited", false, 0
	f.rows[ref] = r
	return nil
}
func newPreviewProbe(t *testing.T) (*Backend, *previewProbe, backend.PreviewSpec) {
	t.Helper()
	b, old, _ := newCheckProbe(t)
	f := &previewProbe{checkProbe: old, images: map[string]image.InspectResponse{"node:fixture": old.image}}
	b.api = f
	dir := t.TempDir()
	dir, e := filepath.EvalSymlinks(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:fixture\nCOPY . /app\nRUN echo compile\nCMD [\"node\",\"app.js\"]\n"), 0600); e != nil {
		t.Fatal(e)
	}
	s := backend.PreviewSpec{Identity: backend.PreviewIdentity{Protocol: 1, AllocationID: "previewbuild-one", ReleaseID: "rel-one", ScopeDigest: strings.Repeat("a", 64), ProjectID: "project-one", SourceDigest: strings.Repeat("b", 64), Commit: strings.Repeat("c", 40)}, Kind: "preview-build", SourceDirectory: dir, Dockerfile: "Dockerfile", Port: 3000, Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}}
	return b, f, s
}
func previewBuiltImage(f *previewProbe, p backend.PreparedPreview) string {
	id := "sha256:" + strings.Repeat("d", 64)
	row := f.image
	row.ID = id
	cfg := *row.Config
	cfg.Labels = previewIdentityLabels(p.Spec.Identity)
	cfg.Cmd = []string{"node", "app.js"}
	row.Config = &cfg
	f.images[id] = row
	return id
}
func previewServe(t *testing.T, b *Backend, f *previewProbe, s backend.PreviewSpec) backend.PreparedPreview {
	t.Helper()
	p, e := b.PreparePreview(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	id := previewBuiltImage(f, p)
	s = p.Spec
	s.Kind = "preview"
	s.Identity.AllocationID = "preview-one"
	s.SourceDirectory = ""
	s.SourceArchive = nil
	s.ContextDigest = ""
	s.ImageID = id
	p, e = b.PreparePreview(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestPreviewPreparationPinsContextWithoutMutation(t *testing.T) {
	b, f, s := newPreviewProbe(t)
	p, e := b.PreparePreview(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	if p.Spec.Identity.EngineID != f.info.ID || p.Spec.Identity.InstanceID != b.instance || p.RequiredDiskBytes < 1<<30 || p.ContextDigest == "" || len(p.BaseImages) != 1 {
		t.Fatalf("incomplete preparation: %+v", p)
	}
	if !bytes.Contains(p.PreparedContext, []byte("FROM "+f.image.ID)) || !bytes.Contains(p.PreparedContext, []byte(previewPrefix+"allocation=\"previewbuild-one\"")) {
		t.Fatal("base image or ownership not pinned")
	}
	if len(f.creates)+len(f.starts)+len(f.buildOptions) != 0 {
		t.Fatal("read-only preparation mutated runtime")
	}
	if err := os.WriteFile(filepath.Join(s.SourceDirectory, "new.txt"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, e = b.BuildPreview(context.Background(), p); e == nil || len(f.buildOptions) != 0 {
		t.Fatal("changed context reached builder")
	}
}
func TestPreviewPreparationRefusesUnbudgetedInputs(t *testing.T) {
	for _, tc := range []struct{ name, text string }{{"dynamic-base", "ARG BASE=node:fixture\nFROM $BASE\n"}, {"buildkit", "# syntax=docker/dockerfile:1\nFROM node:fixture\n"}, {"host-platform", "FROM --platform=linux/amd64 node:fixture\n"}, {"reserved-label", "FROM node:fixture\nLABEL luci.preview-allocation=other\n"}, {"missing-image", "FROM no-local-image\n"}, {"heredoc", "FROM node:fixture\nRUN <<EOF\nx\nEOF\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			b, f, s := newPreviewProbe(t)
			if e := os.WriteFile(filepath.Join(s.SourceDirectory, "Dockerfile"), []byte(tc.text), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := b.PreparePreview(context.Background(), s); e == nil {
				t.Fatal("unsafe/unbudgeted context accepted")
			}
			if len(f.buildOptions)+len(f.creates) != 0 {
				t.Fatal("mutation during refusal")
			}
		})
	}
}
func TestPreviewArchiveRefusesTraversalLinksAndDuplicates(t *testing.T) {
	for _, tc := range []struct {
		name      string
		typ       byte
		duplicate bool
	}{{"../outside", tar.TypeReg, false}, {"/absolute", tar.TypeReg, false}, {"Dockerfile", tar.TypeSymlink, false}, {"Dockerfile", tar.TypeLink, false}, {"Dockerfile", tar.TypeReg, true}} {
		t.Run(tc.name+string(tc.typ), func(t *testing.T) {
			_, _, s := newPreviewProbe(t)
			var buf bytes.Buffer
			w := tar.NewWriter(&buf)
			for i := 0; i < 1+boolInt(tc.duplicate); i++ {
				if e := w.WriteHeader(&tar.Header{Name: tc.name, Typeflag: tc.typ, Linkname: "outside", Mode: 0600}); e != nil {
					t.Fatal(e)
				}
			}
			if e := w.Close(); e != nil {
				t.Fatal(e)
			}
			s.SourceDirectory = ""
			s.SourceArchive = buf.Bytes()
			if _, _, e := previewContext(s); e == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func TestPreviewBuilderRequiresTerminalTransportAndExactSettlement(t *testing.T) {
	for _, mode := range []string{"success", "error-terminal", "truncated", "no-terminal", "transport-error", "intermediate-remains", "wrong-image-label", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			b, f, s := newPreviewProbe(t)
			p, e := b.PreparePreview(context.Background(), s)
			if e != nil {
				t.Fatal(e)
			}
			id := previewBuiltImage(f, p)
			aux, _ := json.Marshal(map[string]any{"aux": map[string]string{"ID": id}})
			f.stream = string(aux) + "\n"
			switch mode {
			case "error-terminal":
				f.stream = `{"error":"build failed"}`
			case "truncated":
				f.stream = string(aux) + "\n{"
			case "no-terminal":
				f.stream = `{"stream":"working"}`
			case "transport-error":
				f.buildError = context.DeadlineExceeded
			case "intermediate-remains":
				f.rows[strings.Repeat("e", 64)] = container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: strings.Repeat("e", 64), State: &container.State{Status: "running"}}, Config: &container.Config{Labels: previewIdentityLabels(p.Spec.Identity)}}
			case "wrong-image-label":
				r := f.images[id]
				r.Config.Labels[previewPrefix+"scope"] = "wrong"
				f.images[id] = r
			case "oversized":
				f.stream = string(aux) + strings.Repeat(" ", 4<<20)
			}
			result, e := b.BuildPreview(context.Background(), p)
			if mode == "success" {
				if e != nil || !result.Settled || result.ImageID != id {
					t.Fatalf("success: %+v %v", result, e)
				}
			} else if mode == "error-terminal" {
				if e == nil || !result.Settled {
					t.Fatal("complete failed build must settle")
				}
			} else if e == nil || result.Settled {
				t.Fatalf("ambiguous build released: %+v %v", result, e)
			}
			if len(f.buildOptions) != 1 {
				t.Fatalf("build replayed: %d", len(f.buildOptions))
			}
			o := f.buildOptions[0]
			if o.Version != build.BuilderV1 || o.Memory != 1<<30 || o.MemorySwap != o.Memory || o.CPUQuota != 100000 || o.CPUPeriod != 100000 || !o.ForceRemove || !o.Remove || o.PullParent {
				t.Fatalf("limits/protocol weakened: %+v", o)
			}
			if len(f.starts) != 0 {
				t.Fatal("builder started serving container")
			}
		})
	}
}
func TestPreviewServingLifecycleUsesExactReferencesAndLimits(t *testing.T) {
	b, f, s := newPreviewProbe(t)
	p := previewServe(t, b, f, s)
	ref, e := b.CreatePreview(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	r := f.rows[ref]
	r.Path = "node"
	r.Config.Cmd = []string{"node", "app.js"}
	r.NetworkSettings = &container.NetworkSettings{NetworkSettingsBase: container.NetworkSettingsBase{Ports: map[nat.Port][]nat.PortBinding{nat.Port("3000/tcp"): {{HostIP: "127.0.0.1", HostPort: "49100"}}}}}
	f.rows[ref] = r
	if e = b.StartPreview(context.Background(), p.Spec.Identity, ref); e != nil {
		t.Fatal(e)
	}
	obs, e := b.InspectPreview(context.Background(), p.Spec.Identity, ref)
	if e != nil || obs.State != "running" || obs.URL != "http://127.0.0.1:49100" {
		t.Fatalf("observation: %+v %v", obs, e)
	}
	if e = b.StartPreview(context.Background(), p.Spec.Identity, ref); e == nil {
		t.Fatal("running container replayed")
	}
	if e = b.StopPreview(context.Background(), p.Spec.Identity, ref); e != nil {
		t.Fatal(e)
	}
	if e = b.StartPreview(context.Background(), p.Spec.Identity, ref); e != nil {
		t.Fatal(e)
	}
	if e = b.RemovePreview(context.Background(), p.Spec.Identity, ref); e != nil {
		t.Fatal(e)
	}
	if len(f.rows) != 0 || len(f.creates) != 1 || len(f.starts) != 2 || len(f.removes) != 1 || f.removeOptions[0].Force {
		t.Fatalf("unexpected mutation counts: %d/%d/%d", len(f.creates), len(f.starts), len(f.removes))
	}
}
func TestPreviewLostCreateACKDoesNotRetryStartOrClean(t *testing.T) {
	b, f, s := newPreviewProbe(t)
	p := previewServe(t, b, f, s)
	f.loseCreateACK = true
	ref, e := b.CreatePreview(context.Background(), p)
	if e == nil || ref != "" || len(f.creates) != 1 || len(f.starts)+len(f.removes) != 0 || len(f.rows) != 1 {
		t.Fatal("unknown Create was adopted/replayed/removed")
	}
}
func TestPreviewStartRejectsChangedOwnershipLimitsAndPublication(t *testing.T) {
	for _, mode := range []string{"foreign-instance", "changed-cpu", "changed-swap", "privileged", "wrong-image", "extra-port", "unsupported-kernel", "wrong-engine"} {
		t.Run(mode, func(t *testing.T) {
			b, f, s := newPreviewProbe(t)
			p := previewServe(t, b, f, s)
			ref, e := b.CreatePreview(context.Background(), p)
			if e != nil {
				t.Fatal(e)
			}
			r := f.rows[ref]
			switch mode {
			case "foreign-instance":
				r.Config.Labels[backend.LabelInstance] = "other"
			case "changed-cpu":
				r.HostConfig.NanoCPUs *= 2
			case "changed-swap":
				r.HostConfig.MemorySwap = -1
			case "privileged":
				r.HostConfig.Privileged = true
			case "wrong-image":
				r.Image = "sha256:" + strings.Repeat("f", 64)
			case "extra-port":
				r.HostConfig.PortBindings[nat.Port("9000/tcp")] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9000"}}
			case "unsupported-kernel":
				f.info.MemoryLimit = false
			case "wrong-engine":
				f.info.ID = "different"
			}
			f.rows[ref] = r
			if e = b.StartPreview(context.Background(), p.Spec.Identity, ref); e == nil || len(f.starts) != 0 {
				t.Fatal("changed configuration started")
			}
		})
	}
}
func TestPreviewRemovalRejectsForeignAndRetainsFailedAbsence(t *testing.T) {
	for _, mode := range []string{"foreign", "remove-fails", "still-present"} {
		t.Run(mode, func(t *testing.T) {
			b, f, s := newPreviewProbe(t)
			p := previewServe(t, b, f, s)
			ref, e := b.CreatePreview(context.Background(), p)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "foreign":
				r := f.rows[ref]
				r.Config.Labels[previewPrefix+"scope"] = "other"
				f.rows[ref] = r
			case "remove-fails":
				f.removeError = errors.New("busy")
			case "still-present":
				f.removeNoEffect = true
			}
			if e = b.RemovePreview(context.Background(), p.Spec.Identity, ref); e == nil {
				t.Fatal("cleanup claimed without exact absence")
			}
			if mode == "foreign" && len(f.removes)+len(f.stops) != 0 {
				t.Fatal("foreign mutation")
			}
			if len(f.rows) != 1 {
				t.Fatal("lost retained object")
			}
		})
	}
}
