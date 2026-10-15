package dockerbe

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type documentProbe struct {
	*checkProbe
	archive []byte
	stat    container.PathStat
}

func (f *documentProbe) CopyFromContainer(context.Context, string, string) (io.ReadCloser, container.PathStat, error) {
	return io.NopCloser(bytes.NewReader(f.archive)), f.stat, nil
}
func newDocumentProbe(t *testing.T) (*DocumentExtractor, *documentProbe, backend.DocumentExtractionSpec) {
	t.Helper()
	b, base, _ := newCheckProbe(t)
	f := &documentProbe{checkProbe: base}
	b.api = f
	extractor, err := NewDocumentExtractor(b, f.image.ID)
	if err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(t.TempDir(), "original")
	data := []byte("%PDF-controlled-fixture")
	if err = os.WriteFile(original, data, 0600); err != nil {
		t.Fatal(err)
	}
	spec := backend.DocumentExtractionSpec{UserID: 1001, GroupID: 1001, Identity: backend.DocumentExtractionIdentity{Protocol: 1, Profile: backend.DocumentExtractionProfile, AllocationID: "document-allocation", ScopeDigest: strings.Repeat("b", 64), ActorDigest: strings.Repeat("c", 64), ProjectID: "project-one", DocumentID: "doc-" + strings.Repeat("d", 64), AttemptID: "extract-one", OriginalDigest: documentFixtureDigest(data)}, ImageID: f.image.ID, OriginalPath: original, OriginalBytes: len(data), MimeType: "application/pdf"}
	spec.Identity.CommandDigest = backend.DocumentExtractionCommandDigest(spec)
	return extractor, f, spec
}
func documentFixtureDigest(data []byte) string {
	result := sha256.Sum256(data)
	return hex.EncodeToString(result[:])
}
func documentFixtureArchive(t *testing.T, name string, kind byte, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	archive := tar.NewWriter(&out)
	header := &tar.Header{Name: name, Typeflag: kind, Mode: 0600, Size: int64(len(data))}
	if kind == tar.TypeSymlink {
		header.Size = 0
		header.Linkname = "/outside"
	}
	if err := archive.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if header.Size > 0 {
		if _, err := archive.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestDocumentExtractorRefusesChangedOriginalAndNeverUsesCheckShape(t *testing.T) {
	b, f, spec := newDocumentProbe(t)
	prepared, err := b.PrepareDocument(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.creates) != 0 || len(f.starts) != 0 {
		t.Fatal("prepare allocated work")
	}
	cfg, host := documentConfiguration(prepared)
	if cfg.User != "1001:1001" || !cfg.NetworkDisabled || host.NetworkMode != "none" || !host.Mounts[0].ReadOnly || host.Mounts[0].Source != spec.OriginalPath || host.Memory != 512<<20 || host.MemorySwap != host.Memory || *host.PidsLimit != 32 || host.NanoCPUs != 1_000_000_000 || cfg.Labels[checkLabel] != "" {
		t.Fatal("document isolation shape changed")
	}
	if err = os.WriteFile(spec.OriginalPath, []byte("changed original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = b.CreateDocument(context.Background(), prepared); err == nil || len(f.creates) != 0 {
		t.Fatal("changed original allocated a parser")
	}
}
func TestDocumentExtractorBindsOutputAndPositiveExactRetirement(t *testing.T) {
	b, f, spec := newDocumentProbe(t)
	prepared, err := b.PrepareDocument(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreateDocument(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	identity := prepared.Spec.Identity
	row := f.rows[ref]
	row.Config.Labels[checkLabel] = "true"
	f.rows[ref] = row
	if b.StartDocument(context.Background(), identity, ref) == nil || len(f.starts) != 0 {
		t.Fatal("check ownership adopted as a document")
	}
	delete(row.Config.Labels, checkLabel)
	f.rows[ref] = row
	if err = b.StartDocument(context.Background(), identity, ref); err != nil {
		t.Fatal(err)
	}
	row = f.rows[ref]
	row.State.Status = "exited"
	row.State.Running = false
	row.State.Pid = 0
	row.State.ExitCode = 0
	f.rows[ref] = row
	data := []byte(`{"protocol":1}`)
	f.archive = documentFixtureArchive(t, "result.json", tar.TypeReg, data)
	f.stat = container.PathStat{Name: "result.json", Size: int64(len(data)), Mode: 0600}
	out, err := b.DocumentResult(context.Background(), identity, ref)
	if err != nil || !bytes.Equal(out.Data, data) || out.Digest != documentFixtureDigest(data) {
		t.Fatal("exact complete result unavailable", err)
	}
	f.removeNoEffect = true
	if b.RetireDocument(context.Background(), identity, ref) == nil {
		t.Fatal("remove acknowledgment without absence accepted")
	}
	f.removeNoEffect = false
	if err = b.RetireDocument(context.Background(), identity, ref); err != nil {
		t.Fatal(err)
	}
	observed, err := b.InspectDocument(context.Background(), identity, ref)
	if err != nil || observed.State != "absent" {
		t.Fatal("positive retirement unavailable", err)
	}
}
func TestDocumentResultArchiveRejectsForeignPathsLinksAndIncompleteBytes(t *testing.T) {
	data := []byte(`{"protocol":1}`)
	stat := container.PathStat{Name: "result.json", Size: int64(len(data)), Mode: 0600}
	for _, test := range []struct {
		name string
		kind byte
	}{{"../result.json", tar.TypeReg}, {"result.json", tar.TypeSymlink}, {"other.json", tar.TypeReg}} {
		if _, err := readDocumentResult(context.Background(), bytes.NewReader(documentFixtureArchive(t, test.name, test.kind, data)), stat); err == nil {
			t.Fatal("unsafe result archive accepted", test.name)
		}
	}
	valid := documentFixtureArchive(t, "result.json", tar.TypeReg, data)
	if _, err := readDocumentResult(context.Background(), bytes.NewReader(valid[:515]), stat); err == nil {
		t.Fatal("truncated result accepted")
	}
	stat.Size = backend.DocumentResultMaxBytes + 1
	if _, err := readDocumentResult(context.Background(), bytes.NewReader(valid), stat); err == nil {
		t.Fatal("oversized declared result accepted")
	}
}
