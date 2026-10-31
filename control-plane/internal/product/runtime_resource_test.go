package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

type runtimeModelTestBody struct {
	io.Reader
	closeErr error
	closed   int
}

func (b *runtimeModelTestBody) Close() error { b.closed++; return b.closeErr }
func runtimeModelReaderFixture(body *runtimeModelTestBody, expected string, record func(context.Context, RuntimeModelDownloadProof) error) *runtimeModelReader {
	sum := sha256.Sum256([]byte(expected))
	return &runtimeModelReader{ctx: context.Background(), body: body, expected: RuntimeModelSource{ServiceID: "model", URL: "https://models.example/file.gguf", Bytes: int64(len(expected)), SHA256: hex.EncodeToString(sum[:])}, hash: sha256.New(), authorize: func(context.Context) error { return nil }, record: record}
}
func TestRuntimeModelBodyRequiresExactEOFHashCheckedCloseAndDurableControllerProof(t *testing.T) {
	for _, tc := range []struct {
		name, body          string
		closeErr, recordErr error
		pass                bool
	}{{"exact", "model", nil, nil, true}, {"short", "mode", nil, nil, false}, {"long", "model!", nil, nil, false}, {"wrong-hash", "MODEL", nil, nil, false}, {"close-error", "model", errors.New("close"), nil, false}, {"receipt-write-error", "model", nil, errors.New("durable write"), false}} {
		t.Run(tc.name, func(t *testing.T) {
			records := 0
			body := &runtimeModelTestBody{Reader: strings.NewReader(tc.body), closeErr: tc.closeErr}
			r := runtimeModelReaderFixture(body, "model", func(_ context.Context, p RuntimeModelDownloadProof) error {
				records++
				if body.closed != 1 || p.Bytes != 5 {
					t.Fatal("EOF reached before original body checked Close")
				}
				return tc.recordErr
			})
			_, err := io.ReadAll(r)
			if tc.pass != (err == nil) || tc.pass != r.verified {
				t.Fatal("incorrect completion", err)
			}
			if tc.pass && records != 1 {
				t.Fatal("missing exact controller proof")
			}
			_ = r.Close()
		})
	}
	revoked := false
	body := &runtimeModelTestBody{Reader: strings.NewReader("model")}
	r := runtimeModelReaderFixture(body, "model", func(context.Context, RuntimeModelDownloadProof) error { t.Fatal("revoked body promoted"); return nil })
	r.authorize = func(context.Context) error {
		if revoked {
			return ErrRuntimePreparation
		}
		return nil
	}
	var first [1]byte
	if _, err := r.Read(first[:]); err != nil {
		t.Fatal(err)
	}
	revoked = true
	if _, err := io.ReadAll(r); err == nil {
		t.Fatal("continued stream after fresh grant refusal")
	}
	_ = r.Close()
}
func TestRuntimeModelSourceAndHelperProfileAreExplicitBoundedAndPlatformPinned(t *testing.T) {
	for _, url := range []string{"http://models.example/model", "https://user:secret@models.example/a", "https://models.example/a?token=secret", "https://models.example/a#fragment", "https://models.example:444/a"} {
		if _, err := runtimeModelURL(url, false); err == nil {
			t.Fatal("unsafe original URL accepted", url)
		}
	}
	if _, err := runtimeModelURL("https://cdn.example/file?Signature=ephemeral", true); err != nil {
		t.Fatal("provider redirect signature rejected", err)
	}
	image := "registry.example/build/go@sha256:" + strings.Repeat("a", 64)
	dockerfile, err := runtimeModelSinkDockerfile(image)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{image, "FROM scratch", "USER 1000:1000", "GOOS=linux GOARCH=amd64", "GOPROXY=off", "GOTOOLCHAIN=local", "COPY main.go"} {
		if !strings.Contains(dockerfile, part) {
			t.Fatal("missing fixed profile binding", part)
		}
	}
	if _, err = runtimeModelSinkDockerfile("go:latest"); err == nil {
		t.Fatal("mutable builder admitted")
	}
	if !hexDigest(runtimeModelSinkProfile()) {
		t.Fatal("missing embedded source identity")
	}
}
func TestRuntimeResourceReceiptsCannotReplaceUIDVolumeOriginalContainerOrPublishedImage(t *testing.T) {
	original := &RuntimeResourceOperation{Source: RuntimeBuildAttempt{RunID: "run", CheckID: "check", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), RunInputDigest: strings.Repeat("c", 64), CheckDigest: strings.Repeat("d", 64), Attempted: true}, ApplyAttempted: true, HelperBuildAttempted: true, HelperBuild: &previewruntime.Outcome{Identity: previewruntime.Identity{ProjectID: "p", ReleaseID: "prep-model-sink", BuildAllocationID: "build-original"}, ImmutableImage: "registry.example/model@sha256:" + strings.Repeat("e", 64)}, Resources: []RuntimeOwnedResource{{Kind: "PersistentVolumeClaim", Name: "claim", UID: "pvc-original", VolumeName: "pv-original", DesiredDigest: strings.Repeat("f", 64), Attempted: true}}, Models: []RuntimeModelTransfer{{ServiceID: "model", PodUID: "pod-original", PVCUID: "pvc-original", ContainerID: "containerd://original", ContainerImageID: "image-original", UploadAttempted: true}}}
	for _, change := range []func(*RuntimeResourceOperation){func(v *RuntimeResourceOperation) { v.ApplyAttempted = false }, func(v *RuntimeResourceOperation) { v.Resources[0].UID = "replacement" }, func(v *RuntimeResourceOperation) { v.Resources[0].VolumeName = "replacement" }, func(v *RuntimeResourceOperation) { v.Models[0].ContainerID = "replacement" }, func(v *RuntimeResourceOperation) { v.Models[0].UploadAttempted = false }, func(v *RuntimeResourceOperation) {
		v.HelperBuild.ImmutableImage = "registry.example/other@sha256:" + strings.Repeat("a", 64)
	}, func(v *RuntimeResourceOperation) { v.Source.CheckID = "replacement" }} {
		next, err := cloneRuntimeResourceOperation(original)
		if err != nil {
			t.Fatal(err)
		}
		change(next)
		if runtimeResourceContinues(original, next) {
			t.Fatal("original receipt replaced")
		}
	}
	next, _ := cloneRuntimeResourceOperation(original)
	next.Resources[0].ResourceVersion = "new-observation"
	if !runtimeResourceContinues(original, next) {
		t.Fatal("same original observation rejected")
	}
}
func TestRuntimeResourceOptionalPhasePreservesImageOnlyRecoveryAndUnknownOriginal(t *testing.T) {
	s, ctx, actor, prepared := runtimePreparationFixture(t)
	got, err := s.ObserveRuntimeResourcePreparation(ctx, actor, prepared.Plan.ID)
	if err != nil || got.Revision != prepared.Revision {
		t.Fatal("absent optional resource phase blocked image-only recovery", err)
	}
	got, err = s.RunRuntimeResourcePreparation(ctx, actor, prepared.Plan.ID, "unused", "unused")
	if err != nil || got.Revision != prepared.Revision {
		t.Fatal("absent phase allocated work", err)
	}
	// The helper's advertised source profile is not evidence a replacement can
	// be allocated for a retained attempt with no original acknowledgement.
	row := storedRuntimePreparation{RuntimePreparation: RuntimePreparation{Plan: RuntimePreparationPlan{Resources: &RuntimeResourcePlan{}}, Resources: &RuntimeResourceOperation{HelperBuildAttempted: true}}}
	if _, err = runtimeHelperImage(row); err == nil {
		t.Fatal("missing original helper result promoted")
	}
}
func TestRuntimeResourcePublicProjectionOmitsPrivateValuesAndLegacyFields(t *testing.T) {
	private := storedRuntimePreparation{PrivateSecrets: []runtimePrivateSecret{{Name: "private", Data: map[string][]byte{"tls.key": []byte("never-return-key-material")}}}}
	raw, err := json.Marshal(private.RuntimePreparation)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "never-return") || strings.Contains(string(raw), "privateSecrets") || strings.Contains(string(raw), "\"resources\"") {
		t.Fatal("private or optional zero fields escaped", string(raw))
	}
	raw, err = json.Marshal(deploymentplan.Service{ID: "legacy"})
	if err != nil || strings.Contains(string(raw), "imagePullSecrets") {
		t.Fatal("legacy service bytes changed")
	}
	source := RuntimeModelSource{ServiceID: "model", Bytes: 5, SHA256: strings.Repeat("a", 64)}
	state := RuntimeModelTransfer{OperationID: "operation", PVCUID: "original"}
	row := storedRuntimePreparation{RuntimePreparation: RuntimePreparation{Plan: RuntimePreparationPlan{ID: "prep", Resources: &RuntimeResourcePlan{ModelsRelativePaths: map[string]string{"model": "weights/model.gguf"}}}}}
	receipt := deploymentkube.ModelSinkReceipt{Protocol: 1, PreparationID: "prep", OperationID: "operation", PVCUID: "replacement", Path: "weights/model.gguf", Bytes: 5, SHA256: source.SHA256, State: "stored-reported"}
	if runtimeReceiptMatches(receipt, row, source, state) {
		t.Fatal("same file hash adopted replacement PVC")
	}
}
