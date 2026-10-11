package dockerbe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// These probes model SDK acknowledgements and their absence. They do not pull
// images, start Docker, or use the source checkout as a build context.
type previewBaseProbe struct {
	*previewProbe
	bases         map[string]image.InspectResponse
	inspectErrors map[string]error
	pullStreams   map[string]string
	pullBody      func(context.Context, string) io.ReadCloser
	pullError     error
	pulls         []string
	pullOptions   []image.PullOptions
	pullBudgets   []time.Duration
	buildBudgets  []time.Duration
	events        []string
	tags          [][2]string
}

func (f *previewBaseProbe) ImageInspect(ctx context.Context, ref string, options ...client.ImageInspectOption) (image.InspectResponse, error) {
	f.events = append(f.events, "inspect:"+ref)
	if err := f.inspectErrors[ref]; err != nil {
		return image.InspectResponse{}, err
	}
	return f.previewProbe.ImageInspect(ctx, ref, options...)
}

func (f *previewBaseProbe) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	f.events = append(f.events, "pull:"+ref)
	f.pulls = append(f.pulls, ref)
	f.pullOptions = append(f.pullOptions, options)
	budget := time.Duration(0)
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	f.pullBudgets = append(f.pullBudgets, budget)
	// Visibility alone does not prove that a pull's transport was acknowledged.
	if row, ok := f.bases[ref]; ok {
		f.images[ref], f.images[row.ID], f.images[previewBaseManifestReference(ref)] = row, row, row
	}
	if f.pullError != nil {
		return nil, f.pullError
	}
	if f.pullBody != nil {
		return f.pullBody(ctx, ref), nil
	}
	stream, ok := f.pullStreams[ref]
	if !ok {
		stream = previewBaseTerminal(ref, "Downloaded newer image")
	}
	return io.NopCloser(strings.NewReader(stream)), nil
}

func (f *previewBaseProbe) ImageBuild(ctx context.Context, source io.Reader, options build.ImageBuildOptions) (build.ImageBuildResponse, error) {
	f.events = append(f.events, "build")
	budget := time.Duration(0)
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	f.buildBudgets = append(f.buildBudgets, budget)
	return f.previewProbe.ImageBuild(ctx, source, options)
}

func (f *previewBaseProbe) ImageTag(_ context.Context, source, target string) error {
	f.events = append(f.events, "tag:"+target)
	f.tags = append(f.tags, [2]string{source, target})
	return errors.New("preview base preparation must not tag images")
}

func previewBaseTerminal(ref, status string) string {
	// Docker's create-image route drops a tag when a digest is present, then
	// emits FamiliarString(ref), not the fully qualified SDK request string.
	named, err := reference.ParseDockerRef(ref)
	if err != nil {
		panic(err)
	}
	_, digest, _ := strings.Cut(previewBaseManifestReference(ref), "@")
	digestFrame, _ := json.Marshal(map[string]string{"status": "Digest: " + digest})
	data, _ := json.Marshal(map[string]string{"status": "Status: " + status + " for " + reference.FamiliarString(named)})
	return string(digestFrame) + "\n" + string(data) + "\n"
}

func previewBaseManifestReference(ref string) string {
	named, err := reference.ParseDockerRef(ref)
	if err != nil {
		panic(err)
	}
	digest := "sha256:" + strings.Repeat("f", 64)
	if digested, ok := named.(reference.Digested); ok {
		digest = digested.Digest().String()
	}
	return reference.TrimNamed(named).Name() + "@" + digest
}

func newPreviewBaseProbe(t *testing.T, dockerfile string) (*Backend, *previewBaseProbe, backend.PreviewSpec) {
	t.Helper()
	b, original, spec := newPreviewProbe(t)
	f := &previewBaseProbe{previewProbe: original, bases: map[string]image.InspectResponse{}, inspectErrors: map[string]error{}, pullStreams: map[string]string{}}
	// Keep the established warm fixture visible under Docker's canonical name.
	f.images["docker.io/library/node:fixture"] = f.images["node:fixture"]
	f.images[f.image.ID] = f.image
	b.api = f
	if err := os.WriteFile(filepath.Join(spec.SourceDirectory, "Dockerfile"), []byte(dockerfile), 0600); err != nil {
		t.Fatal(err)
	}
	return b, f, spec
}

func previewBaseAvailableAfterPull(f *previewBaseProbe, ref string, digit string, size int64) string {
	row := f.image
	row.ID, row.Size = "sha256:"+strings.Repeat(digit, 64), size
	row.RepoDigests = []string{previewBaseManifestReference(ref)}
	cfg := *row.Config
	row.Config = &cfg
	f.bases[ref] = row
	return row.ID
}

func previewBaseBuildSuccess(f *previewBaseProbe, prepared backend.PreparedPreview) string {
	id := previewBuiltImage(f.previewProbe, prepared)
	f.stream = previewAuxFrame(id) + previewStreamFrame("Successfully built "+strings.TrimPrefix(id, "sha256:")[:12]+"\n")
	return id
}

func previewBaseGuard(f *previewBaseProbe) func(context.Context) error {
	return func(ctx context.Context) error {
		f.events = append(f.events, "guard")
		return ctx.Err()
	}
}

func assertPreviewBaseNoMutation(t *testing.T, f *previewBaseProbe) {
	t.Helper()
	if len(f.pulls)+len(f.buildOptions)+len(f.tags)+len(f.creates)+len(f.starts)+len(f.removes) != 0 {
		t.Fatalf("read-only refusal mutated resources: pulls=%v builds=%d tags=%v", f.pulls, len(f.buildOptions), f.tags)
	}
}

func assertPreviewBaseMutationGuards(t *testing.T, f *previewBaseProbe) {
	t.Helper()
	for i, event := range f.events {
		if event == "build" || strings.HasPrefix(event, "pull:") {
			if i == 0 || f.events[i-1] != "guard" {
				t.Fatalf("mutation lacked an immediately preceding guard: %v", f.events)
			}
		}
	}
}

func TestPreviewColdBasesArePlannedThenPulledAndPinned(t *testing.T) {
	digest := "sha256:" + strings.Repeat("f", 64)
	for _, tc := range []struct{ name, source, canonical string }{
		{"tag", "alpine:3.22", "docker.io/library/alpine:3.22"},
		{"digest", "alpine@" + digest, "docker.io/library/alpine@" + digest},
		{"tag-and-digest", "alpine:3.22@" + digest, "docker.io/library/alpine@" + digest},
	} {
		for _, terminal := range []string{"Downloaded newer image", "Image is up to date"} {
			t.Run(tc.name+"/"+terminal, func(t *testing.T) {
				b, f, spec := newPreviewBaseProbe(t, "FROM "+tc.source+"\nRUN echo compile\n")
				baseID := previewBaseAvailableAfterPull(f, tc.canonical, "b", 64<<20)
				f.pullStreams[tc.canonical] = previewBaseTerminal(tc.canonical, terminal)
				prepared, err := b.PreparePreview(context.Background(), spec)
				if err != nil {
					t.Fatal(err)
				}
				assertPreviewBaseNoMutation(t, f)
				wantPlan := []backend.PreviewBaseImage{{Reference: tc.canonical, VirtualSizeBytes: 2 << 30}}
				if !reflect.DeepEqual(prepared.BasePlan, wantPlan) || len(prepared.BasePlanDigest) != 64 {
					t.Fatalf("cold plan lacks canonical reference, cap, or digest: %+v %q", prepared.BasePlan, prepared.BasePlanDigest)
				}
				wantDisk := uint64(len(prepared.PreparedContext))*2 + (1 << 30) + 2*(2<<30)
				// Immutable IDs can grow FROM across tar record boundaries.
				// Admission must cover that bounded padding as well as the cap.
				if prepared.RequiredDiskBytes < wantDisk || prepared.RequiredDiskBytes > wantDisk+2*2048 {
					t.Fatalf("cold build demand=%d outside bounded estimate [%d,%d]", prepared.RequiredDiskBytes, wantDisk, wantDisk+2*2048)
				}
				builtID := previewBaseBuildSuccess(f, prepared)
				result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
				if err != nil || !result.Settled || result.ImageID != builtID {
					t.Fatalf("cold build failed: result=%+v err=%v", result, err)
				}
				if !reflect.DeepEqual(f.pulls, []string{tc.canonical}) || len(f.buildOptions) != 1 || len(f.tags) != 0 {
					t.Fatalf("unexpected mutations: pulls=%v builds=%d tags=%v", f.pulls, len(f.buildOptions), f.tags)
				}
				if !reflect.DeepEqual(f.pullOptions, []image.PullOptions{{}}) || f.buildOptions[0].PullParent || len(f.buildOptions[0].Tags) != 0 {
					t.Fatal("base resolution delegated an unplanned pull, authentication, platform override, or tag")
				}
				if got := tarDockerfile(t, f.contexts[0]); !strings.Contains(got, "FROM "+baseID+"\n") || strings.Contains(got, tc.source) {
					t.Fatalf("mutable base reached builder: %q", got)
				}
				assertPreviewBaseMutationGuards(t, f)
			})
		}
	}
}

func TestPreviewColdBasePlanDeduplicatesReferencesButAccountsEachStage(t *testing.T) {
	const ref = "docker.io/library/alpine:3.22"
	dockerfile := "FROM alpine:3.22 AS compile\nRUN echo first\nFROM compile AS packaged\nFROM docker.io/library/alpine:3.22 AS final\nCOPY --from=packaged /app /app\n"
	b, f, spec := newPreviewBaseProbe(t, dockerfile)
	baseID := previewBaseAvailableAfterPull(f, ref, "b", 64<<20)
	prepared, err := b.PreparePreview(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.BasePlan) != 1 || prepared.BasePlan[0].Reference != ref {
		t.Fatalf("alias or repeated reference added a pull: %+v", prepared.BasePlan)
	}
	wantDisk := uint64(len(prepared.PreparedContext))*2 + (1 << 30) + 2*2*(2<<30)
	if prepared.RequiredDiskBytes < wantDisk || prepared.RequiredDiskBytes > wantDisk+2*2048 {
		t.Fatalf("repeated external stage demand=%d outside bounded estimate [%d,%d]", prepared.RequiredDiskBytes, wantDisk, wantDisk+2*2048)
	}
	previewBaseBuildSuccess(f, prepared)
	result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
	if err != nil || !result.Settled || !reflect.DeepEqual(f.pulls, []string{ref}) || len(f.contexts) != 1 {
		t.Fatalf("deduplicated build failed: pulls=%v result=%+v err=%v", f.pulls, result, err)
	}
	got := tarDockerfile(t, f.contexts[0])
	if strings.Count(got, "FROM "+baseID+" AS ") != 2 || !strings.Contains(got, "FROM compile AS packaged") || !strings.Contains(got, "COPY --from=packaged /app /app") {
		t.Fatalf("stage relationships or immutable external bases changed: %q", got)
	}
	assertPreviewBaseMutationGuards(t, f)
}

func TestPreviewColdBaseBuildRequiresLiveGuardBeforeAnyMutation(t *testing.T) {
	for _, mode := range []string{"nil", "denied", "canceled-before-call", "canceled-in-guard"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			guard := previewBaseGuard(f)
			switch mode {
			case "nil":
				guard = nil
			case "denied":
				guard = func(context.Context) error { return errors.New("admission withdrawn") }
			case "canceled-before-call":
				cancel()
			case "canceled-in-guard":
				guard = func(context.Context) error { cancel(); return nil }
			}
			if _, err = b.BuildPreview(ctx, prepared, guard); err == nil {
				t.Fatal("missing, denied, or canceled authority was accepted")
			}
			assertPreviewBaseNoMutation(t, f)
		})
	}
}

func TestPreviewColdBaseAdmissionIsRecheckedBetweenMutations(t *testing.T) {
	for _, mode := range []string{"before-build", "before-second-pull", "cancel-before-build"} {
		t.Run(mode, func(t *testing.T) {
			dockerfile := "FROM alpine:3.22\n"
			if mode == "before-second-pull" {
				dockerfile += "FROM busybox:1.37\n"
			}
			b, f, spec := newPreviewBaseProbe(t, dockerfile)
			previewBaseAvailableAfterPull(f, "docker.io/library/alpine:3.22", "b", 64<<20)
			previewBaseAvailableAfterPull(f, "docker.io/library/busybox:1.37", "c", 64<<20)
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			guard := func(context.Context) error {
				f.events = append(f.events, "guard")
				calls++
				if calls == 2 {
					if mode == "cancel-before-build" {
						cancel()
						return nil
					}
					return errors.New("admission withdrawn after acknowledged pull")
				}
				return nil
			}
			result, err := b.BuildPreview(ctx, prepared, guard)
			if err == nil || !result.Settled || calls != 2 || len(f.pulls) != 1 || len(f.buildOptions)+len(f.tags) != 0 {
				t.Fatalf("authority changed after pull: calls=%d pulls=%v result=%+v err=%v", calls, f.pulls, result, err)
			}
			assertPreviewBaseMutationGuards(t, f)
		})
	}
}

type previewBasePullBody struct {
	io.Reader
	readError, closeError error
	onEOF                 func()
	closed                int
}

func (r *previewBasePullBody) Read(data []byte) (int, error) {
	n, err := r.Reader.Read(data)
	if err == io.EOF {
		if r.onEOF != nil {
			r.onEOF()
		}
		if r.readError != nil {
			return n, r.readError
		}
	}
	return n, err
}

func (r *previewBasePullBody) Close() error {
	r.closed++
	return r.closeError
}

func TestPreviewColdBasePullNeedsMatchingTerminalEOFAndClose(t *testing.T) {
	const ref = "docker.io/library/alpine:3.22"
	for _, mode := range []string{"malformed", "missing-terminal", "missing-manifest-digest", "malformed-manifest-digest", "conflicting-manifest-digest", "invalid-duplicate-manifest-digest", "wrong-reference", "extra-terminal-text", "late-error", "late-error-detail", "truncated", "read-error", "close-error", "lost-ack", "canceled-at-eof", "oversized-stream", "post-pull-inspect-unavailable", "mismatched-repo-digests", "missing-repo-digests"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
			baseID := previewBaseAvailableAfterPull(f, ref, "b", 64<<20)
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream := previewBaseTerminal(ref, "Downloaded newer image")
			body := &previewBasePullBody{}
			switch mode {
			case "malformed":
				stream = "not-json\n"
			case "missing-terminal":
				stream = `{"status":"Pull complete","id":"layer"}` + "\n"
			case "missing-manifest-digest":
				_, stream, _ = strings.Cut(stream, "\n")
			case "malformed-manifest-digest":
				stream = strings.Replace(stream, "Digest: sha256:"+strings.Repeat("f", 64), "Digest: invalid", 1)
			case "conflicting-manifest-digest", "invalid-duplicate-manifest-digest":
				value := "sha256:" + strings.Repeat("e", 64)
				if mode == "invalid-duplicate-manifest-digest" {
					value = "invalid"
				}
				duplicate, _ := json.Marshal(map[string]string{"status": "Digest: " + value})
				stream = strings.Replace(stream, "\n", "\n"+string(duplicate)+"\n", 1)
			case "wrong-reference":
				stream = previewBaseTerminal("docker.io/library/busybox:1.37", "Downloaded newer image")
			case "extra-terminal-text":
				stream = strings.Replace(stream, `alpine:3.22"`, `alpine:3.22 extra"`, 1)
			case "late-error":
				stream += `{"error":"late pull failure"}` + "\n"
			case "late-error-detail":
				stream += `{"errorDetail":{"message":"late pull failure"}}` + "\n"
			case "truncated":
				stream += "{"
			case "read-error":
				body.readError = io.ErrUnexpectedEOF
			case "close-error":
				body.closeError = errors.New("lost transport completion")
			case "lost-ack":
				f.pullError = context.DeadlineExceeded
			case "canceled-at-eof":
				body.onEOF = cancel
			case "oversized-stream":
				stream += strings.Repeat(" ", 8<<20)
			case "post-pull-inspect-unavailable":
				body.onEOF = func() { f.inspectErrors[previewBaseManifestReference(ref)] = errors.New("inspection unavailable") }
			case "mismatched-repo-digests", "missing-repo-digests":
				row := f.bases[ref]
				row.RepoDigests = nil
				if mode == "mismatched-repo-digests" {
					row.RepoDigests = []string{"docker.io/library/alpine@sha256:" + strings.Repeat("e", 64)}
				}
				f.bases[ref] = row
			}
			body.Reader = strings.NewReader(stream)
			f.pullBody = func(context.Context, string) io.ReadCloser { return body }
			result, err := b.BuildPreview(ctx, prepared, previewBaseGuard(f))
			// Inspection can fail after the pull itself was positively
			// acknowledged. That read-only failure leaves no active mutation.
			wantSettled := mode == "post-pull-inspect-unavailable" || mode == "mismatched-repo-digests" || mode == "missing-repo-digests"
			if err == nil || result.Settled != wantSettled || result.ImageID != "" || len(f.pulls) != 1 || len(f.buildOptions)+len(f.tags) != 0 {
				t.Fatalf("pull acknowledgement or refusal misclassified: pulls=%v result=%+v err=%v", f.pulls, result, err)
			}
			if f.images[ref].ID != baseID {
				t.Fatal("fixture did not expose the image independently of the lost acknowledgement")
			}
			if mode != "lost-ack" && body.closed != 1 {
				t.Fatalf("pull body closed %d times, want exactly once", body.closed)
			}
			assertPreviewBaseMutationGuards(t, f)
		})
	}
}

func TestPreviewColdBaseExceedingVirtualCapSettlesWithoutBuild(t *testing.T) {
	b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
	previewBaseAvailableAfterPull(f, "docker.io/library/alpine:3.22", "b", (2<<30)+1)
	prepared, err := b.PreparePreview(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
	if err == nil || !result.Settled || result.ImageID != "" || len(f.pulls) != 1 || len(f.buildOptions)+len(f.tags) != 0 {
		t.Fatalf("oversized acknowledged base was built or retained as uncertain: result=%+v err=%v", result, err)
	}
}

func TestPreviewDigestBaseRejectsDifferentManifestAcknowledgement(t *testing.T) {
	ref := "docker.io/library/alpine@sha256:" + strings.Repeat("f", 64)
	b, f, spec := newPreviewBaseProbe(t, "FROM "+ref+"\n")
	previewBaseAvailableAfterPull(f, ref, "b", 64<<20)
	prepared, err := b.PreparePreview(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	f.pullStreams[ref] = strings.Replace(previewBaseTerminal(ref, "Downloaded newer image"), "Digest: sha256:"+strings.Repeat("f", 64), "Digest: sha256:"+strings.Repeat("e", 64), 1)
	result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
	if err == nil || result.Settled || len(f.pulls) != 1 || len(f.buildOptions)+len(f.tags) != 0 {
		t.Fatalf("different digest acknowledged the requested immutable base: result=%+v err=%v", result, err)
	}
}

func TestPreviewColdBaseRetagRaceUsesAcknowledgedManifest(t *testing.T) {
	const ref = "docker.io/library/alpine:3.22"
	b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
	baseID := previewBaseAvailableAfterPull(f, ref, "b", 64<<20)
	prepared, err := b.PreparePreview(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	body := &previewBasePullBody{Reader: strings.NewReader(previewBaseTerminal(ref, "Downloaded newer image")), onEOF: func() {
		// The tag changes after the daemon's acknowledged manifest, while that
		// immutable manifest and its config image remain available locally.
		changed := f.bases[ref]
		changed.ID = "sha256:" + strings.Repeat("c", 64)
		changed.RepoDigests = []string{"docker.io/library/alpine@sha256:" + strings.Repeat("e", 64)}
		f.images[ref], f.images[changed.ID] = changed, changed
	}}
	f.pullBody = func(context.Context, string) io.ReadCloser { return body }
	builtID := previewBaseBuildSuccess(f, prepared)
	result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
	if err != nil || !result.Settled || result.ImageID != builtID || len(f.pulls) != 1 || len(f.contexts) != 1 || body.closed != 1 {
		t.Fatalf("acknowledged immutable base lost to tag race: result=%+v err=%v", result, err)
	}
	if got := tarDockerfile(t, f.contexts[0]); !strings.Contains(got, "FROM "+baseID+"\n") || strings.Contains(got, f.images[ref].ID) {
		t.Fatalf("retagged image replaced acknowledged base: %q", got)
	}
	afterPull, inspectedManifest := false, false
	for _, event := range f.events {
		if event == "pull:"+ref {
			afterPull = true
		}
		if afterPull && event == "inspect:"+ref {
			t.Fatal("mutable tag was consulted after pull acknowledgement")
		}
		if afterPull && event == "inspect:"+previewBaseManifestReference(ref) {
			inspectedManifest = true
		}
	}
	if !inspectedManifest || len(f.tags) != 0 {
		t.Fatal("build did not verify its acknowledged immutable repository digest")
	}
	assertPreviewBaseMutationGuards(t, f)
}

func TestPreviewColdBasePlanAndContextTamperingNeverPulls(t *testing.T) {
	for _, mode := range []string{"reference", "image-id", "virtual-size", "plan-digest", "drop-plan", "duplicate-plan", "context-digest", "prepared-context", "source-context", "disk-demand"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "reference":
				prepared.BasePlan[0].Reference = "docker.io/library/unreviewed:latest"
			case "image-id":
				prepared.BasePlan[0].ImageID = "sha256:" + strings.Repeat("e", 64)
			case "virtual-size":
				prepared.BasePlan[0].VirtualSizeBytes--
			case "plan-digest":
				prepared.BasePlanDigest = strings.Repeat("0", 64)
			case "drop-plan":
				prepared.BasePlan = nil
			case "duplicate-plan":
				prepared.BasePlan = append(prepared.BasePlan, prepared.BasePlan[0])
			case "context-digest":
				prepared.ContextDigest = strings.Repeat("0", 64)
			case "prepared-context":
				prepared.PreparedContext = append([]byte(nil), prepared.PreparedContext...)
				prepared.PreparedContext[0] ^= 1
			case "source-context":
				if err := os.WriteFile(filepath.Join(spec.SourceDirectory, "changed.txt"), []byte("changed after admission"), 0600); err != nil {
					t.Fatal(err)
				}
			case "disk-demand":
				prepared.RequiredDiskBytes--
			}
			if _, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f)); err == nil {
				t.Fatal("altered admission input was accepted")
			}
			assertPreviewBaseNoMutation(t, f)
		})
	}
}

func TestPreviewBaseInspectionFailureIsNotACacheMiss(t *testing.T) {
	b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
	f.inspectErrors["alpine:3.22"] = errors.New("Docker inspection unavailable")
	f.inspectErrors["docker.io/library/alpine:3.22"] = errors.New("Docker inspection unavailable")
	if _, err := b.PreparePreview(context.Background(), spec); err == nil {
		t.Fatal("unknown base inspection was treated as confirmed absence")
	}
	assertPreviewBaseNoMutation(t, f)
}

func TestPreviewWarmBaseIdentityDriftNeverPullsOrBuilds(t *testing.T) {
	b, f, spec := newPreviewBaseProbe(t, "FROM node:fixture\n")
	prepared, err := b.PreparePreview(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.BasePlan) != 1 || prepared.BasePlan[0].ImageID != f.image.ID || prepared.BasePlan[0].VirtualSizeBytes != uint64(f.image.Size) {
		t.Fatalf("warm identity and size were not pinned: %+v", prepared.BasePlan)
	}
	changed := f.image
	changed.ID = "sha256:" + strings.Repeat("c", 64)
	f.images["node:fixture"], f.images["docker.io/library/node:fixture"] = changed, changed
	if _, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f)); err == nil {
		t.Fatal("warm tag drift was accepted")
	}
	assertPreviewBaseNoMutation(t, f)
}

func TestPreviewColdBaseReservationAndStageCountsAreFinite(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stages     int
		cold, same bool
		allowed    bool
	}{
		{"seven-cold-stages", 7, true, false, true},
		{"eight-cold-stages-exceed-reservation", 8, true, false, false},
		{"repeated-cold-stage-still-charged", 8, true, true, false},
		{"sixteen-warm-stages", 16, false, false, true},
		{"seventeenth-warm-stage", 17, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var dockerfile strings.Builder
			for i := 0; i < tc.stages; i++ {
				name := fmt.Sprintf("base%d:fixture", i)
				if tc.same {
					name = "base0:fixture"
				}
				fmt.Fprintf(&dockerfile, "FROM %s\n", name)
			}
			b, f, spec := newPreviewBaseProbe(t, dockerfile.String())
			if !tc.cold {
				for i := 0; i < tc.stages; i++ {
					name := fmt.Sprintf("base%d:fixture", i)
					f.images[name], f.images["docker.io/library/"+name] = f.image, f.image
				}
			}
			prepared, err := b.PreparePreview(context.Background(), spec)
			if (err == nil) != tc.allowed {
				t.Fatalf("bounded preparation allowed=%v want=%v: %v", err == nil, tc.allowed, err)
			}
			if tc.allowed && (len(prepared.BasePlan) > 16 || prepared.RequiredDiskBytes == 0 || prepared.RequiredDiskBytes > 32<<30) {
				t.Fatalf("unbounded plan: bases=%d bytes=%d", len(prepared.BasePlan), prepared.RequiredDiskBytes)
			}
			assertPreviewBaseNoMutation(t, f)
		})
	}
}

func TestPreviewColdBasePullAndBuildShareFiniteCallerBound(t *testing.T) {
	for _, callerBudget := range []time.Duration{0, 3 * time.Second} {
		t.Run(callerBudget.String(), func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM alpine:3.22\n")
			previewBaseAvailableAfterPull(f, "docker.io/library/alpine:3.22", "b", 64<<20)
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			previewBaseBuildSuccess(f, prepared)
			ctx := context.Background()
			if callerBudget != 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, callerBudget)
				defer cancel()
			}
			result, err := b.BuildPreview(ctx, prepared, previewBaseGuard(f))
			if err != nil || !result.Settled || len(f.pullBudgets) != 1 || len(f.buildBudgets) != 1 {
				t.Fatalf("bounded build failed: result=%+v err=%v", result, err)
			}
			limit := 15 * time.Minute
			if callerBudget != 0 {
				limit = callerBudget
			}
			for _, budget := range []time.Duration{f.pullBudgets[0], f.buildBudgets[0]} {
				if budget <= 0 || budget > limit {
					t.Fatalf("mutation budget %v exceeds caller/build limit %v", budget, limit)
				}
			}
			if f.buildBudgets[0] > f.pullBudgets[0] {
				t.Fatal("build reset the pull/build budget")
			}
		})
	}
}

func TestPreviewWarmBaseDemandKeepsExistingSizePolicy(t *testing.T) {
	b, f, spec := newPreviewBaseProbe(t, "FROM node:fixture AS build\nFROM node:fixture\n")
	row := f.image
	row.Size = 9 << 30
	f.images["node:fixture"], f.images["docker.io/library/node:fixture"], f.images[row.ID] = row, row, row
	prepared, err := b.PreparePreview(context.Background(), spec)
	if err != nil {
		t.Fatalf("cold-image policy changed a warm-only build: %v", err)
	}
	wantDisk := uint64(len(prepared.PreparedContext))*2 + (1 << 30) + uint64(row.Size)*2*2
	if prepared.RequiredDiskBytes != wantDisk || prepared.RequiredDiskBytes <= 32<<30 || len(prepared.BasePlan) != 1 {
		t.Fatalf("warm stage demand changed: bytes=%d want=%d plan=%+v", prepared.RequiredDiskBytes, wantDisk, prepared.BasePlan)
	}
	previewBaseBuildSuccess(f, prepared)
	result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
	if err != nil || !result.Settled || len(f.pulls) != 0 || len(f.buildOptions) != 1 || len(f.tags) != 0 {
		t.Fatalf("warm-only build pulled or failed: result=%+v err=%v", result, err)
	}
	assertPreviewBaseMutationGuards(t, f)
}

func TestPreviewCopyFromOnlyPreviouslyDeclaredStagesReachesBuild(t *testing.T) {
	for _, tc := range []struct{ name, instruction string }{
		{"alias", "COPY --from=compile /artifact /artifact\n"},
		{"index", "COPY --from=0 /artifact /artifact\n"},
		{"continued-alias", "COPY --from=compile \\\n  /artifact /artifact\n"},
		{"continued-index", "COPY \\\n  --from=0 /artifact /artifact\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM node:fixture AS compile\nRUN touch /artifact\nFROM scratch\n"+tc.instruction)
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatalf("ordinary prior-stage COPY refused: %v", err)
			}
			if len(prepared.BasePlan) != 1 || prepared.BasePlan[0].ImageID != f.image.ID {
				t.Fatalf("COPY stage was mistaken for an external base: %+v", prepared.BasePlan)
			}
			previewBaseBuildSuccess(f, prepared)
			result, err := b.BuildPreview(context.Background(), prepared, previewBaseGuard(f))
			if err != nil || !result.Settled || len(f.pulls)+len(f.tags) != 0 || len(f.contexts) != 1 {
				t.Fatalf("prior-stage COPY build failed: result=%+v err=%v", result, err)
			}
			got := tarDockerfile(t, f.contexts[0])
			if !strings.Contains(got, "FROM "+f.image.ID+" AS compile") || !strings.Contains(got, "FROM scratch") || !strings.Contains(got, "/artifact /artifact") {
				t.Fatalf("immutable base or COPY relationship lost: %q", got)
			}
			assertPreviewBaseMutationGuards(t, f)
		})
	}
}

func TestPreviewCopyFromExternalAndNonPreviousStagesRefusedBeforeInspection(t *testing.T) {
	for _, tc := range []struct{ name, dockerfile string }{
		{"uncached-external", "FROM scratch\nCOPY --from=alpine:3.22 /artifact /artifact\n"},
		{"cached-external", "FROM scratch\nCOPY --from=node:fixture /artifact /artifact\n"},
		{"external-after-cold-base", "FROM alpine:3.22\nCOPY --from=busybox:1.37 /artifact /artifact\n"},
		{"dynamic", "FROM scratch\nCOPY --from=$BASE /artifact /artifact\n"},
		{"missing-alias", "FROM scratch\nCOPY --from=missing /artifact /artifact\n"},
		{"self-alias", "FROM scratch AS current\nCOPY --from=current /artifact /artifact\n"},
		{"future-alias", "FROM scratch\nCOPY --from=later /artifact /artifact\nFROM scratch AS later\n"},
		{"current-first-index", "FROM scratch\nCOPY --from=0 /artifact /artifact\n"},
		{"current-second-index", "FROM scratch\nFROM scratch\nCOPY --from=1 /artifact /artifact\n"},
		{"future-index", "FROM scratch\nCOPY --from=1 /artifact /artifact\nFROM scratch\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, tc.dockerfile)
			if _, err := b.PreparePreview(context.Background(), spec); err == nil {
				t.Fatal("COPY introduced an unplanned image or non-previous stage")
			}
			assertPreviewBaseNoMutation(t, f)
			for _, event := range f.events {
				if strings.HasPrefix(event, "inspect:") {
					t.Fatalf("COPY source was not validated before base inspection: %v", f.events)
				}
			}
		})
	}
}
