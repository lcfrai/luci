package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// All tests are inert source/byte/temp-file fixtures. No assemble/runChroot,
// native payload execution, Mknod, real /work, Docker, network or installation.
func TestPureWireMirrorMatchesAuthoritativeBackendSource(t *testing.T) {
	backend, err := os.ReadFile("../../../control-plane/internal/backend/document_image_assembly.go")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile("wire.go")
	if err != nil {
		t.Fatal(err)
	}
	b, h := strings.ReplaceAll(string(backend), "\r\n", "\n"), strings.ReplaceAll(string(helper), "\r\n", "\n")
	between := func(source, start, end string) string {
		at := strings.Index(source, start)
		if at < 0 {
			t.Fatal("wire source missing", start)
		}
		value := source[at:]
		if end != "" {
			finish := strings.Index(value, end)
			if finish < 0 {
				t.Fatal("wire source missing", end)
			}
			value = value[:finish]
		}
		return strings.TrimSpace(value)
	}
	for _, pair := range [][2]string{{"const (", "type DocumentImageAssemblyIdentity"}, {"type DocumentImageAssemblyInput", "// The trusted constructor"}, {"func DocumentImageAssemblyRecipeDigest", "func ValidateDocumentImageAssemblyIdentity"}, {"func ValidateDocumentImageAssemblyManifest", ""}} {
		if pair[0] == "const (" {
			actual := between(h, pair[0], "type DocumentImageAssemblyInput")
			expected := between(b, pair[0], pair[1])
			if actual != expected {
				t.Fatal("fixed profile/recipe mirror diverged")
			}
			continue
		}
		var helperEnd string
		switch pair[0] {
		case "type DocumentImageAssemblyInput":
			helperEnd = "var assemblyCommit"
		case "func DocumentImageAssemblyRecipeDigest":
			helperEnd = "func ValidateDocumentImageAssemblyManifest"
		}
		if between(b, pair[0], pair[1]) != between(h, pair[0], helperEnd) {
			t.Fatal("wire schema/validation mirror diverged", pair[0])
		}
	}
}
func TestManifestRejectsDuplicateNestedKeysAndUnboundedDepth(t *testing.T) {
	for _, body := range []string{`{"a":1,"a":2}`, `{"a":{"same":1,"same":2}}`, strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18), `{"a":1} {"b":2}`, `{"a":NaN}`} {
		if uniqueJSON([]byte(body)) == nil {
			t.Fatal("ambiguous/unbounded manifest accepted")
		}
	}
	if uniqueJSON([]byte(`{"a":[{"b":1},{"b":2}]}`)) != nil {
		t.Fatal("distinct nested objects refused")
	}
}
func TestUnresolvedDependencyLockCannotBeRelabelledByManifestFlags(t *testing.T) {
	for _, body := range []string{`{"state":"unresolved","unresolved":[]}`, `{"state":"closed-reviewed","unresolved":["native unknown"]}`, `{"state":"closed-reviewed"}`, `{"state":"closed-reviewed","unresolved":null}`, `{"state":"closed-reviewed","state":"unresolved","unresolved":[]}`} {
		if validClosedDependencyLock([]byte(body)) {
			t.Fatal("unresolved/ambiguous lock accepted")
		}
	}
	if !validClosedDependencyLock([]byte(`{"state":"closed-reviewed","unresolved":[]}`)) {
		t.Fatal("closed-state syntax fixture refused; not an attestation")
	}
}
func TestUnixRootfsColonNamesDoNotRelaxHostStaging(t *testing.T) {
	name := "var/lib/dpkg/info/libc6:amd64.list"
	if inputName(name) || !relativeUnixName(name) {
		t.Fatal("Unix rootfs and strict staging path policies conflated")
	}
	for _, bad := range []string{"../foreign", "/host/absolute", "safe\nforeign", "safe\x00foreign", "safe/../foreign"} {
		if relativeUnixName(bad) || inputName(bad) {
			t.Fatal("unsafe member path accepted", bad)
		}
	}
	if runtime.GOOS != "linux" {
		t.Skip("colon-bearing rootfs fixture is Linux-only; never create Windows ADS")
	}
	root := t.TempDir()
	payload := []byte("/usr/lib/x86_64-linux-gnu/libc.so.6\n")
	if unpackUSTAR(bytes.NewReader(fixtureUSTAR(t, name, tar.TypeReg, payload)), root) != nil {
		t.Fatal("valid multiarch Linux archive member refused")
	}
	full, err := rooted(root, name)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(full)
	if err != nil || !bytes.Equal(actual, payload) {
		t.Fatal("multiarch member did not remain confined", err)
	}
	names, err := rootfsNames(context.Background(), root)
	if err != nil {
		t.Fatal("multiarch export inventory refused", err)
	}
	found := false
	for _, entry := range names {
		if entry == name {
			found = true
		}
	}
	if !found {
		t.Fatal("multiarch output member omitted")
	}
}
func TestRootedTreatsAbsoluteLinksAsVirtualAndRefusesEscapeCycleAndFileParent(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("/inside", filepath.Join(root, "absolute")); err != nil {
		t.Skip("fixture symlinks unavailable", err)
	}
	if actual, err := rooted(root, "absolute/file"); err != nil || actual != filepath.Join(root, "inside", "file") {
		t.Fatal("absolute link escaped virtual root", actual, err)
	}
	if os.Symlink("../escape", filepath.Join(root, "escape")) != nil || os.Symlink("cycle", filepath.Join(root, "cycle")) != nil || os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0600) != nil {
		t.Fatal("fixture creation")
	}
	for _, name := range []string{"escape/file", "cycle/file", "file/child", "../foreign", "/host", "safe\\foreign", "safe\nforeign"} {
		if _, err := rooted(root, name); err == nil {
			t.Fatal("unsafe virtual path accepted", name)
		}
	}
}
func fixtureUSTAR(t *testing.T, name string, kind byte, body []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	header := &tar.Header{Name: name, Typeflag: kind, Mode: 0644, Size: int64(len(body)), Uid: os.Getuid(), Gid: os.Getgid(), ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}
	if kind == tar.TypeDir {
		header.Size = 0
	}
	if err := w.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestUSTARRefusesTraversalDevicesCorruptionAndTrailingData(t *testing.T) {
	valid := fixtureUSTAR(t, "file", tar.TypeReg, []byte("text"))
	if err := unpackUSTAR(bytes.NewReader(valid), t.TempDir()); err != nil {
		t.Fatal("valid bounded source fixture refused", err)
	}
	corrupt := append([]byte{}, valid...)
	corrupt[1] ^= 1
	for _, body := range [][]byte{fixtureUSTAR(t, "../foreign", tar.TypeReg, []byte("text")), fixtureUSTAR(t, "device", tar.TypeChar, nil), corrupt, valid[:len(valid)-1], append(append([]byte{}, valid...), byte('x'))} {
		if unpackUSTAR(bytes.NewReader(body), t.TempDir()) == nil {
			t.Fatal("unsafe/incomplete base archive accepted")
		}
	}
}
func TestPAXRejectsUnknownDuplicateAndMalformedFeature(t *testing.T) {
	for _, body := range []string{"12 unknown=x\n", "9 path=x\n9 path=y\n", "999 path=x\n", "0 x=y\n"} {
		if _, err := parsePAX([]byte(body)); err == nil {
			t.Fatal("unsupported PAX accepted")
		}
	}
	if values, err := parsePAX([]byte("9 path=x\n")); err != nil || values["path"] != "x" {
		t.Fatal("valid bounded PAX refused")
	}
}

type countAssemblyWrites struct{ calls int }

func (w *countAssemblyWrites) Write(p []byte) (int, error) { w.calls++; return len(p), nil }
func TestOutputBoundRejectsBeforeSinkAndCancellationStopsWrites(t *testing.T) {
	sink := &countAssemblyWrites{}
	writer := &assemblyBoundedWriter{ctx: context.Background(), writer: sink, max: 3}
	if _, err := writer.Write([]byte("four")); err == nil || sink.calls != 0 {
		t.Fatal("over-bound bytes reached artifact sink")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer.ctx = ctx
	if _, err := writer.Write([]byte("x")); err == nil || sink.calls != 0 {
		t.Fatal("cancelled output reached sink")
	}
}
func TestOCIArchiveIsDeterministicAndVerifiesActualLayerDigest(t *testing.T) {
	data := []byte("inert-layer-bytes")
	sum := sha256.Sum256(data)
	name := filepath.Join(t.TempDir(), "layer")
	if os.WriteFile(name, data, 0600) != nil {
		t.Fatal("fixture file")
	}
	layer := ociDescriptor{"application/vnd.oci.image.layer.v1.tar+gzip", "sha256:" + hex.EncodeToString(sum[:]), int64(len(data))}
	metadata := map[string][]byte{"oci-layout": []byte(`{"imageLayoutVersion":"1.0.0"}`), "index.json": []byte(`{"schemaVersion":2,"manifests":[]}`)}
	var first, second bytes.Buffer
	if writeOCITar(context.Background(), &first, metadata, layer, name) != nil || writeOCITar(context.Background(), &second, metadata, layer, name) != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("owned OCI encoding is not deterministic")
	}
	r := tar.NewReader(bytes.NewReader(first.Bytes()))
	count := 0
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
		if header.Name == "blobs/sha256/"+hex.EncodeToString(sum[:]) {
			actual, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(actual, data) {
				t.Fatal("wrong actual layer bytes")
			}
		}
	}
	if count != 3 {
		t.Fatal("extra/missing OCI members")
	}
	if os.WriteFile(name, []byte("changed-layer-data"), 0600) != nil {
		t.Fatal("fixture change")
	}
	if writeOCITar(context.Background(), io.Discard, metadata, layer, name) == nil {
		t.Fatal("changed layer was accepted")
	}
}
