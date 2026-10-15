package dockerbe

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Inert in-memory transport only: no daemon, real socket or native process.
type documentProbe struct {
	*checkProbe
	dial func(context.Context) (net.Conn, error)
}

func (f *documentProbe) ClientVersion() string                           { return "1.51" }
func (f *documentProbe) Dialer() func(context.Context) (net.Conn, error) { return f.dial }
func (f *documentProbe) HTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _ string, _ string) (net.Conn, error) { return f.dial(ctx) }}}
}
func newDocumentProbe(t *testing.T) (*DocumentExtractor, *documentProbe, backend.DocumentExtractionSpec) {
	t.Helper()
	b, base, _ := newCheckProbe(t)
	f := &documentProbe{checkProbe: base, dial: func(context.Context) (net.Conn, error) { return nil, errors.New("no fixture connection configured") }}
	b.api = f
	extractor, err := NewDocumentExtractor(b, f.image.ID, DocumentAttachConfiguration{Headers: map[string]string{"X-Fixture-Auth": "scoped"}})
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
func documentProducerFrame(data []byte) []byte {
	var output bytes.Buffer
	output.WriteString(documentResultMagic)
	_ = binary.Write(&output, binary.BigEndian, uint32(len(data)))
	sum := sha256.Sum256(data)
	output.Write(sum[:])
	output.Write(data)
	output.WriteString(documentResultEnd)
	return output.Bytes()
}
func documentDockerFrame(stream byte, data []byte) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	return append(header, data...)
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
	if cfg.User != "1001:1001" || !cfg.NetworkDisabled || !cfg.AttachStdout || host.NetworkMode != "none" || !host.Mounts[0].ReadOnly || host.Mounts[0].Source != spec.OriginalPath || host.Memory != 512<<20 || host.MemorySwap != host.Memory || *host.PidsLimit != 32 || host.NanoCPUs != 1_000_000_000 || cfg.Labels[checkLabel] != "" {
		t.Fatal("document isolation shape changed")
	}
	if err = os.WriteFile(spec.OriginalPath, []byte("changed original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = b.CreateDocument(context.Background(), prepared); err == nil || len(f.creates) != 0 {
		t.Fatal("changed original allocated a parser")
	}
}
func TestDocumentStreamSurvivesExitWithoutPostStopTmpfsRead(t *testing.T) {
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
	if b.StartDocument(context.Background(), identity, ref) == nil || len(f.starts) != 0 {
		t.Fatal("an extraction started without its owned pre-Start stream")
	}
	clientEnd, serverEnd := net.Pipe()
	defer serverEnd.Close()
	f.dial = func(context.Context) (net.Conn, error) { return clientEnd, nil }
	release := make(chan struct{})
	served := make(chan error, 1)
	data := []byte(`{"protocol":1}`)
	go func() {
		request, e := http.ReadRequest(bufio.NewReader(serverEnd))
		if e != nil {
			served <- e
			return
		}
		if request.Header.Get("X-Fixture-Auth") != "scoped" || request.URL.Query().Get("stdin") != "" || request.URL.Query().Get("logs") != "" {
			served <- errors.New("selected attach semantics changed")
			serverEnd.Close()
			return
		}
		if _, e = io.WriteString(serverEnd, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.multiplexed-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"); e != nil {
			served <- e
			return
		}
		<-release
		_, e = serverEnd.Write(documentDockerFrame(1, documentProducerFrame(data)))
		serverEnd.Close()
		served <- e
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := b.AttachDocumentResult(ctx, identity, ref)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	defer stream.Close()
	if err = b.StartDocument(ctx, identity, ref); err != nil {
		close(release)
		t.Fatal(err)
	}
	captured := make(chan backend.DocumentExtractionOutput, 1)
	captureError := make(chan error, 1)
	go func() { out, e := stream.Capture(ctx); captured <- out; captureError <- e }()
	close(release)
	// A real Docker tmpfs no longer exists after stop. This fixture deliberately
	// offers no CopyFromContainer/filesystem result API at all.
	row := f.rows[ref]
	row.State.Status = "exited"
	row.State.Running = false
	row.State.Pid = 0
	row.State.ExitCode = 0
	f.rows[ref] = row
	select {
	case out := <-captured:
		if e := <-captureError; e != nil || !bytes.Equal(out.Data, data) || out.Digest != documentFixtureDigest(data) {
			t.Fatal("live capture unavailable after exit", e)
		}
	case <-ctx.Done():
		t.Fatal("exact stream reader did not settle")
	}
	if e := <-served; e != nil {
		t.Fatal(e)
	}
	f.removeNoEffect = true
	if b.RetireDocument(ctx, identity, ref) == nil {
		t.Fatal("remove ACK without absence accepted")
	}
	f.removeNoEffect = false
	if err = b.RetireDocument(ctx, identity, ref); err != nil {
		t.Fatal(err)
	}
}
func TestDocumentAttachTimeoutClosesExactInertConnectionAndSettlesHandshake(t *testing.T) {
	b, f, spec := newDocumentProbe(t)
	prepared, err := b.PrepareDocument(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreateDocument(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	clientEnd, serverEnd := net.Pipe()
	defer serverEnd.Close()
	f.dial = func(ctx context.Context) (net.Conn, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return clientEnd, nil
	}
	settled := make(chan struct{})
	go func() {
		defer close(settled)
		_, _ = http.ReadRequest(bufio.NewReader(serverEnd))
		_, _ = io.Copy(io.Discard, serverEnd)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if stream, e := b.AttachDocumentResult(ctx, prepared.Spec.Identity, ref); e == nil || stream != nil {
		t.Fatal("missing attach handshake accepted")
	}
	select {
	case <-settled:
	case <-time.After(time.Second):
		t.Fatal("exact handshake connection remained open")
	}
	if len(f.starts) != 0 {
		t.Fatal("failed attachment started extraction")
	}
}
func TestDocumentResultStreamRejectsExtraForeignIncompleteAndUnboundedFraming(t *testing.T) {
	data := []byte(`{"protocol":1}`)
	valid := documentDockerFrame(1, documentProducerFrame(data))
	if out, err := readDocumentResultStream(context.Background(), bytes.NewReader(valid)); err != nil || out.Digest != documentFixtureDigest(data) {
		t.Fatal("complete bounded frame refused", err)
	}
	changed := append([]byte{}, valid...)
	changed[len(changed)-len(documentResultEnd)-1] ^= 1
	large := make([]byte, 8)
	large[0] = 1
	binary.BigEndian.PutUint32(large[4:], ^uint32(0))
	for _, hostile := range [][]byte{documentDockerFrame(2, documentProducerFrame(data)), append(append([]byte{}, valid...), documentDockerFrame(1, []byte("extra"))...), append(append([]byte{}, valid...), 1), valid[:len(valid)-1], changed, large, documentDockerFrame(1, nil)} {
		if _, err := readDocumentResultStream(context.Background(), bytes.NewReader(hostile)); err == nil {
			t.Fatal("foreign/extra/incomplete framing accepted")
		}
	}
}
