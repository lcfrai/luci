package dockerbe

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Existing in-memory SDK probe only; never a daemon, socket, image or package.
func newAssemblyProbe(t *testing.T) (*DocumentImageAssembler, *documentProbe, backend.DocumentImageAssemblySpec) {
	t.Helper()
	base, f, _ := newCheckProbe(t)
	probe := &documentProbe{checkProbe: f, dial: func(context.Context) (net.Conn, error) { return nil, errors.New("no inert connection configured") }}
	base.api = probe
	spec := backend.DocumentImageAssemblySpec{BaseImageID: backend.DocumentImageAssemblyBaseID, InputPath: t.TempDir(), InputBytes: 1, HelperPath: filepath.Join(t.TempDir(), "helper"), HelperBytes: 1, Identity: backend.DocumentImageAssemblyIdentity{Protocol: 1, Profile: backend.DocumentImageAssemblyProfile, AllocationID: "assembly-one", ScopeDigest: strings.Repeat("b", 64), ActorDigest: strings.Repeat("c", 64), SourceCommit: strings.Repeat("d", 40), SourceDigest: strings.Repeat("e", 64), InputDigest: strings.Repeat("f", 64), HelperDigest: strings.Repeat("a", 64), RecipeDigest: backend.DocumentImageAssemblyRecipeDigest()}}
	spec.Identity.CommandDigest = backend.DocumentImageAssemblyCommandDigest(spec)
	b, err := NewDocumentImageAssembler(base, spec, DocumentAttachConfiguration{Headers: map[string]string{"X-Fixture-Auth": "scoped"}})
	if err != nil {
		t.Fatal(err)
	}
	return b, probe, spec
}
func assemblyCreatedProbe(t *testing.T) (*DocumentImageAssembler, *documentProbe, backend.DocumentImageAssemblyIdentity, string) {
	b, f, spec := newAssemblyProbe(t)
	spec.Identity.EngineID, spec.Identity.InstanceID = f.info.ID, b.instance
	cfg, host := assemblyConfiguration(backend.PreparedDocumentImageAssembly{Spec: spec})
	created, err := f.ContainerCreate(context.Background(), cfg, host, nil, nil, assemblyName(spec.Identity))
	if err != nil {
		t.Fatal(err)
	}
	return b, f, spec.Identity, created.ID
}
func assemblyProducerFrame(data []byte) []byte {
	var b bytes.Buffer
	b.WriteString(assemblyResultMagic)
	_ = binary.Write(&b, binary.BigEndian, uint64(len(data)))
	h := sha256.Sum256(data)
	b.Write(h[:])
	b.Write(data)
	b.WriteString(assemblyResultEnd)
	return b.Bytes()
}
func TestAssemblyRefusesUnclosedInputsBeforeAnyDaemonEffect(t *testing.T) {
	b, f, spec := newAssemblyProbe(t)
	if _, err := b.PrepareDocumentImageAssembly(context.Background(), spec); err == nil {
		t.Fatal("missing closed manifest admitted")
	}
	if len(f.creates)+len(f.starts)+len(f.removes)+f.imageCalls+f.infoCalls != 0 {
		t.Fatal("unclosed input refusal reached daemon")
	}
	spec.Identity.HelperDigest = strings.Repeat("9", 64)
	spec.Identity.CommandDigest = backend.DocumentImageAssemblyCommandDigest(spec)
	if b.pinnedSpec(spec) {
		t.Fatal("caller substituted another pinned helper")
	}
}
func TestAssemblyOwnedProfileRefusesForeignPurposeAndEscalatedConfiguration(t *testing.T) {
	b, f, id, ref := assemblyCreatedProbe(t)
	row := f.rows[ref]
	if !b.assemblyOwnedConfiguration(id, ref, row) {
		t.Fatal("fixed inert profile not recognized")
	}
	if row.HostConfig.Memory != 2<<30 || row.HostConfig.MemorySwap != row.HostConfig.Memory || *row.HostConfig.PidsLimit != 64 || row.HostConfig.Tmpfs["/work"] != "rw,nosuid,exec,dev,size=2g,mode=0700" || len(row.HostConfig.CapAdd) != 8 {
		t.Fatal("fixed quota/cap profile changed")
	}
	for _, mutate := range []func(*container.InspectResponse){
		func(r *container.InspectResponse) { r.Config.Labels[documentLabel] = "true" },
		func(r *container.InspectResponse) { r.Config.Labels[checkLabel] = "true" },
		func(r *container.InspectResponse) { r.HostConfig.CapAdd = append(r.HostConfig.CapAdd, "SYS_ADMIN") },
		func(r *container.InspectResponse) { r.HostConfig.Privileged = true },
		func(r *container.InspectResponse) { r.HostConfig.Mounts[0].ReadOnly = false },
		func(r *container.InspectResponse) { r.HostConfig.Tmpfs["/work"] = "rw,size=8g" },
		func(r *container.InspectResponse) { r.Config.Entrypoint = []string{"/bin/sh"} },
	} {
		changed := copyCheckRow(row)
		mutate(&changed)
		if b.assemblyOwnedConfiguration(id, ref, changed) {
			t.Fatal("foreign/escalated configuration adopted")
		}
	}
	if b.StartDocumentImageAssembly(context.Background(), id, ref) == nil || len(f.starts) != 0 {
		t.Fatal("assembly started without its exact pre-Start stream")
	}
}
func TestAssemblyStreamBoundedChunkedDataAndHostileFraming(t *testing.T) {
	data := bytes.Repeat([]byte("OCI-data"), 20000)
	inner := assemblyProducerFrame(data)
	var framed []byte
	for len(inner) > 0 {
		n := 313
		if len(inner) < n {
			n = len(inner)
		}
		framed = append(framed, documentDockerFrame(1, inner[:n])...)
		inner = inner[n:]
	}
	var sink bytes.Buffer
	out, err := readAssemblyResultStream(context.Background(), bytes.NewReader(framed), &sink)
	if err != nil || out.Bytes != int64(len(data)) || out.Digest != documentFixtureDigest(data) || !bytes.Equal(sink.Bytes(), data) {
		t.Fatal("valid split frames refused", err)
	}
	valid := documentDockerFrame(1, assemblyProducerFrame([]byte("one")))
	large := make([]byte, 8)
	large[0] = 1
	binary.BigEndian.PutUint32(large[4:], ^uint32(0))
	changed := append([]byte{}, valid...)
	changed[len(changed)-len(assemblyResultEnd)-1] ^= 1
	oversize := assemblyProducerFrame([]byte("one"))
	binary.BigEndian.PutUint64(oversize[len(assemblyResultMagic):], backend.DocumentImageAssemblyOutputMaxBytes+1)
	for _, body := range [][]byte{large, documentDockerFrame(2, []byte("foreign")), documentDockerFrame(1, nil), valid[:len(valid)-1], changed, append(append([]byte{}, valid...), 1), append(append([]byte{}, valid...), documentDockerFrame(1, []byte("extra"))...), documentDockerFrame(1, oversize)} {
		if _, e := readAssemblyResultStream(context.Background(), bytes.NewReader(body), io.Discard); e == nil {
			t.Fatal("foreign/trailing/oversized/incomplete stream accepted")
		}
	}
}

type assemblyShortWriter struct{}

func (assemblyShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestAssemblyStreamRefusesShortSinkAndRawHeaderBudgetBeforeAllocation(t *testing.T) {
	body := documentDockerFrame(1, assemblyProducerFrame([]byte("one")))
	if _, err := readAssemblyResultStream(context.Background(), bytes.NewReader(body), assemblyShortWriter{}); err == nil {
		t.Fatal("short artifact write accepted")
	}
	r := &assemblyDockerReader{ctx: context.Background(), reader: bytes.NewReader(documentDockerFrame(1, []byte("x"))), raw: backend.DocumentImageAssemblyRawMaxBytes - 7}
	var one [1]byte
	if _, err := r.Read(one[:]); err == nil {
		t.Fatal("raw header crossed bound")
	}
	r = &assemblyDockerReader{ctx: context.Background(), reader: bytes.NewReader(documentDockerFrame(1, []byte("x"))), frames: 65536}
	if _, err := r.Read(one[:]); err == nil {
		t.Fatal("frame count crossed bound")
	}
}
func TestAssemblyAttachTimeoutClosesExactInertConnectionAndNeverStarts(t *testing.T) {
	b, f, id, ref := assemblyCreatedProbe(t)
	clientEnd, serverEnd := net.Pipe()
	defer serverEnd.Close()
	f.dial = func(ctx context.Context) (net.Conn, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return clientEnd, nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = http.ReadRequest(bufio.NewReader(serverEnd))
		_, _ = io.Copy(io.Discard, serverEnd)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if stream, err := b.AttachDocumentImageAssembly(ctx, id, ref); err == nil || stream != nil {
		t.Fatal("missing handshake accepted")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("exact connection remained open")
	}
	if len(f.starts) != 0 {
		t.Fatal("failed attach executed helper")
	}
}
func TestAssemblyFenceRefusesLostACKAndForeignIdentity(t *testing.T) {
	b, f, spec := newAssemblyProbe(t)
	spec.Identity.EngineID, spec.Identity.InstanceID = f.info.ID, b.instance
	req := backend.DocumentImageAssemblyFenceRequest{Identity: spec.Identity, IntentID: strings.Repeat("a", 32)}
	f.loseCreateACK = true
	proof, err := b.FenceDocumentImageAssembly(context.Background(), req)
	if !errors.Is(err, backend.ErrLaunchFencePending) || proof != (backend.DocumentImageAssemblyFenceProof{}) {
		t.Fatal("lost marker ACK became proof", err)
	}
	if len(f.starts) != 0 || len(f.creates) != 1 {
		t.Fatal("fence started or automatically replayed work")
	}
	row := f.rows[strings.Repeat("0", 63)+"1"]
	if !inertDocumentImageAssemblyFenceLabels(row) {
		t.Fatal("dedicated marker identity not recognized")
	}
	row.Config.Labels[documentLabel] = "true"
	if inertDocumentImageAssemblyFenceLabels(row) {
		t.Fatal("mixed-purpose marker recognized")
	}
}
func TestAssemblyEmergencyRetirementDoesNotAdoptForeignPurpose(t *testing.T) {
	b, f, id, ref := assemblyCreatedProbe(t)
	row := f.rows[ref]
	row.Config.Labels[checkLabel] = "true"
	f.rows[ref] = row
	if b.RetireDocumentImageAssembly(context.Background(), id, ref) == nil || len(f.removes) != 0 {
		t.Fatal("foreign object retired")
	}
	delete(row.Config.Labels, checkLabel)
	row.State.Status = "running"
	row.State.Running = true
	row.State.Pid = 42
	f.rows[ref] = row
	f.removeNoEffect = true
	if b.RetireDocumentImageAssembly(context.Background(), id, ref) == nil {
		t.Fatal("unknown cleanup settled")
	}
	if len(f.removes) != 1 || !f.removeOptions[0].Force {
		t.Fatal("exact owned emergency retirement absent")
	}
}

type assemblyCloseFailure struct {
	net.Conn
	fail bool
}

func (c *assemblyCloseFailure) Close() error {
	if c.fail {
		c.fail = false
		return errors.New("unknown native close")
	}
	return c.Conn.Close()
}
func TestAssemblyFailedHandshakeCloseRetainsExactNativeOwnership(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	connection := &assemblyCloseFailure{Conn: client, fail: true}
	owner := &assemblyAttachOwner{connections: []net.Conn{connection}}
	if owner.Close() == nil || len(owner.connections) != 1 {
		t.Fatal("failed native close discarded ownership")
	}
	if owner.Close() != nil || len(owner.connections) != 0 {
		t.Fatal("exact positive close failed to settle ownership")
	}
}
func TestAssemblySourceDirectoryWalkRefusesBeyondFiniteEntryBudget(t *testing.T) {
	root := t.TempDir()
	for n := 0; n < 1025; n++ {
		if os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%04d", n)), nil, 0600) != nil {
			t.Fatal("fixture file")
		}
	}
	visited := 0
	err := walkAssemblySources(context.Background(), root, func(string, fs.DirEntry, error) error { visited++; return nil })
	if err == nil || visited != 1024 {
		t.Fatal("unbounded directory reached source visitor", visited, err)
	}
}
func TestAssemblyStaticEntryCheckRefusesInterpreterDynamicAndForeignArchitecture(t *testing.T) {
	data := make([]byte, 120)
	copy(data, "\x7fELF\x02\x01\x01")
	binary.LittleEndian.PutUint16(data[16:], 2)
	binary.LittleEndian.PutUint16(data[18:], 62)
	binary.LittleEndian.PutUint32(data[20:], 1)
	binary.LittleEndian.PutUint64(data[24:], 0x400000)
	binary.LittleEndian.PutUint64(data[32:], 64)
	binary.LittleEndian.PutUint16(data[52:], 64)
	binary.LittleEndian.PutUint16(data[54:], 56)
	binary.LittleEndian.PutUint16(data[56:], 1)
	binary.LittleEndian.PutUint32(data[64:], 1)
	binary.LittleEndian.PutUint32(data[68:], 5)
	binary.LittleEndian.PutUint64(data[80:], 0x400000)
	binary.LittleEndian.PutUint64(data[96:], 120)
	binary.LittleEndian.PutUint64(data[104:], 120)
	name := filepath.Join(t.TempDir(), "inert-elf-headers")
	if os.WriteFile(name, data, 0600) != nil || !staticAssemblyHelper(name, 120) {
		t.Fatal("bounded static header fixture refused; never executed")
	}
	for _, mutate := range []func([]byte){func(d []byte) { binary.LittleEndian.PutUint32(d[64:], 3) }, func(d []byte) { binary.LittleEndian.PutUint32(d[64:], 2) }, func(d []byte) { binary.LittleEndian.PutUint16(d[18:], 183) }, func(d []byte) { binary.LittleEndian.PutUint64(d[32:], ^uint64(0)) }} {
		changed := append([]byte{}, data...)
		mutate(changed)
		if os.WriteFile(name, changed, 0600) != nil {
			t.Fatal("fixture change")
		}
		if staticAssemblyHelper(name, 120) {
			t.Fatal("foreign/dynamic/overflowed helper header accepted")
		}
	}
}
