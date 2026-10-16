//go:build windows

package dockerbe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Deferred Windows temporary-file/handle acceptance only. documentProbe is an
// in-memory SDK; these tests never connect to a daemon or run native payloads.
func assemblyGuardFixture(t *testing.T) (*DocumentImageAssembler, *documentProbe, backend.DocumentImageAssemblySpec, backend.DocumentImageAssemblyRetainedObject) {
	t.Helper()
	b, probe, spec := newAssemblyProbe(t)
	if err := os.WriteFile(spec.HelperPath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	object := backend.DocumentImageAssemblyRetainedObject{Path: filepath.Join(t.TempDir(), "source-object"), Bytes: 3, Digest: strings.Repeat("1", 64)}
	if err := os.WriteFile(object.Path, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	return b, probe, spec, object
}

func TestAssemblyPrePrepareGuardOwnsOriginalHandlesAndExactTransfer(t *testing.T) {
	b, probe, spec, object := assemblyGuardFixture(t)
	view, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, []backend.DocumentImageAssemblyRetainedObject{object})
	if err != nil || view == nil || view.CheckCurrent(context.Background()) != nil {
		t.Fatal("guard not acquired", err)
	}
	defer b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec)
	if err := os.WriteFile(object.Path, []byte("two"), 0600); err == nil {
		t.Fatal("retained source changed under guard")
	}
	if _, err := b.PrepareDocumentImageAssembly(context.Background(), spec); err == nil {
		t.Fatal("guard syntax became evidence/manifest acceptance")
	}
	if probe.infoCalls+probe.imageCalls+len(probe.creates) != 0 {
		t.Fatal("incomplete input reached daemon")
	}
	prepared := spec
	prepared.Identity.EngineID, prepared.Identity.InstanceID = probe.info.ID, b.instance
	foreign := prepared
	foreign.Identity.AllocationID = "foreign-allocation"
	if b.bindAssemblyInputs(context.Background(), foreign) == nil {
		t.Fatal("foreign allocation borrowed guard")
	}
	original := b.sourceClaim.guard
	if b.bindAssemblyInputs(context.Background(), prepared) != nil || b.sourceClaim.guard != original {
		t.Fatal("transfer reopened/replaced original handles")
	}
	if _, ok := b.guards.Load(spec.Identity); ok {
		t.Fatal("old unprepared guard key remained")
	}
	if got, ok := b.guards.Load(prepared.Identity); !ok || got != original {
		t.Fatal("prepared identity did not own exact handles")
	}
	if err := b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if view.CheckCurrent(context.Background()) == nil {
		t.Fatal("borrowed closed view stayed eligible")
	}
	if _, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, nil); err == nil {
		t.Fatal("same allocation reused closed handles")
	}
}

func TestAssemblyUnknownCreateCannotUseUnallocatedReleaseAndFailedCloseStaysOwned(t *testing.T) {
	b, probe, spec, object := assemblyGuardFixture(t)
	view, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, []backend.DocumentImageAssemblyRetainedObject{object})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { b.sourceClaim.guard.closeHandle = nil; _ = b.releaseAssemblySources(b.sourceClaim.identity) }()
	prepared := spec
	prepared.Identity.EngineID, prepared.Identity.InstanceID = probe.info.ID, b.instance
	if b.bindAssemblyInputs(context.Background(), prepared) != nil {
		t.Fatal("exact transfer failed")
	}
	// Model the irreversible edge before a lost ContainerCreate ACK. No SDK
	// effect is invoked. Production sets this flag BEFORE ContainerCreate.
	b.sourceClaim.createAttempted = true
	if b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec) == nil {
		t.Fatal("uncertain Create released original inputs")
	}
	if os.WriteFile(object.Path, []byte("two"), 0600) == nil {
		t.Fatal("uncertain allocation lost source ownership")
	}
	guard := b.sourceClaim.guard
	guard.closeHandle = func(syscall.Handle) error { return errors.New("inert exact handle close unavailable") }
	if b.releaseAssemblySources(prepared.Identity) == nil || b.sourceClaim.closed {
		t.Fatal("failed closure marked settled")
	}
	if _, ok := b.guards.Load(prepared.Identity); !ok {
		t.Fatal("unknown handles became unreachable")
	}
	if view.CheckCurrent(context.Background()) == nil {
		t.Fatal("partially closing view stayed eligible")
	}
	guard.closeHandle = nil
	// Exact positive retirement is simulated only by this private cleanup call;
	// normal production callers first verify owned retirement/fence/absence.
	if b.releaseAssemblySources(prepared.Identity) != nil || !b.sourceClaim.closed {
		t.Fatal("exact close did not settle")
	}
	if os.WriteFile(object.Path, []byte("two"), 0600) != nil {
		t.Fatal("positive close left handle held")
	}
}

func TestAssemblyGuardPartialAcquisitionHardlinkAndByteMismatchRetainOwnership(t *testing.T) {
	for _, mode := range []string{"hardlink", "bytes", "missing", "alternate-stream"} {
		t.Run(mode, func(t *testing.T) {
			b, _, spec, object := assemblyGuardFixture(t)
			switch mode {
			case "hardlink":
				if err := os.Link(object.Path, object.Path+"-alias"); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				object.Bytes++
			case "missing":
				object.Path += "-absent"
			case "alternate-stream":
				object.Path += ":stream"
			}
			view, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, []backend.DocumentImageAssemblyRetainedObject{object})
			if err == nil || view == nil || b.sourceClaim == nil {
				t.Fatal("partial claim missing or unsafe source accepted", err)
			}
			if view.CheckCurrent(context.Background()) == nil {
				t.Fatal("incomplete acquisition became an eligible borrowed view")
			}
			if b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec) != nil {
				t.Fatal("positively unallocated partial guard could not close")
			}
			if view.CheckCurrent(context.Background()) == nil {
				t.Fatal("released partial view became eligible")
			}
		})
	}
}

func TestAssemblyGuardAggregateBoundAndProcessRestartHaveNoImplicitAdoption(t *testing.T) {
	b, probe, spec, object := assemblyGuardFixture(t)
	tooMany := make([]backend.DocumentImageAssemblyRetainedObject, assemblySourceHandleLimit+1)
	if _, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, tooMany); err == nil || b.sourceClaim != nil {
		t.Fatal("unbounded request allocated handles")
	}
	view, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, []backend.DocumentImageAssemblyRetainedObject{object})
	if err != nil {
		t.Fatal(err)
	}
	defer b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec)
	restarted, err := NewDocumentImageAssembler(b.Backend, spec, DocumentAttachConfiguration{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.PrepareDocumentImageAssembly(context.Background(), spec); err == nil || restarted.sourceClaim != nil {
		t.Fatal("restart invented old kernel guards")
	}
	if view.CheckCurrent(context.Background()) != nil || probe.infoCalls+probe.imageCalls != 0 {
		t.Fatal("restart altered original owner or reached daemon")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if view.CheckCurrent(ctx) == nil {
		t.Fatal("cancelled check eligible")
	}
}

func TestAssemblyGuardOriginalIdentityDistinguishesFilesAndMutableAncestorContents(t *testing.T) {
	file := syscall.ByHandleFileInformation{FileAttributes: syscall.FILE_ATTRIBUTE_NORMAL, NumberOfLinks: 1, FileIndexLow: 10, VolumeSerialNumber: 4, FileSizeLow: 3}
	for _, change := range []func(*syscall.ByHandleFileInformation){
		func(v *syscall.ByHandleFileInformation) { v.FileIndexLow++ },
		func(v *syscall.ByHandleFileInformation) { v.NumberOfLinks++ },
		func(v *syscall.ByHandleFileInformation) { v.FileSizeLow++ },
		func(v *syscall.ByHandleFileInformation) { v.LastWriteTime.LowDateTime++ },
		func(v *syscall.ByHandleFileInformation) { v.FileAttributes |= syscall.FILE_ATTRIBUTE_REPARSE_POINT },
	} {
		changed := file
		change(&changed)
		if assemblySourceIdentityEqual(file, changed) {
			t.Fatal("original file identity change ignored")
		}
	}
	dir := file
	dir.FileAttributes = syscall.FILE_ATTRIBUTE_DIRECTORY
	changed := dir
	changed.LastWriteTime.LowDateTime++
	if !assemblySourceIdentityEqual(dir, changed) {
		t.Fatal("unrelated ancestor child activity invalidated source identity")
	}
}

func TestAssemblyGuardAggregateIncludesPayloadAncestorsAndRetainedObjects(t *testing.T) {
	b, _, spec, _ := assemblyGuardFixture(t)
	root := t.TempDir()
	objects := make([]backend.DocumentImageAssemblyRetainedObject, assemblySourceHandleLimit)
	for i := range objects {
		path := filepath.Join(root, fmt.Sprintf("retained-%04d", i))
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		objects[i] = backend.DocumentImageAssemblyRetainedObject{path, 1, strings.Repeat("1", 64)}
	}
	view, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, objects)
	if err == nil || view == nil || b.sourceClaim == nil {
		t.Fatal("aggregate acquisition exceeded actual handle budget", err)
	}
	defer b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec)
	if len(b.sourceClaim.guard.handles) > assemblySourceHandleLimit || view.CheckCurrent(context.Background()) == nil {
		t.Fatal("partial aggregate became eligible")
	}
	if err := b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec); err != nil {
		t.Fatal("bounded partial claim remained held", err)
	}
}

type assemblyBlockedCreateProbe struct {
	*documentProbe
	entered chan struct{}
	release chan struct{}
}

func (f *assemblyBlockedCreateProbe) ContainerCreate(ctx context.Context, config *container.Config, host *container.HostConfig, networking *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error) {
	created, err := f.documentProbe.ContainerCreate(ctx, config, host, networking, platform, name)
	if err != nil || config.Labels[backend.LabelLaunchFence] == "1" {
		return created, err
	}
	// Simulate actual daemon allocation with a blocked/lost client ACK. The
	// independent marker Create below still returns a real acknowledged result.
	close(f.entered)
	<-f.release
	return container.CreateResponse{}, context.DeadlineExceeded
}

func TestAssemblyBlockedCreateACKCannotBlockAcknowledgedFenceGuardSettlement(t *testing.T) {
	b, probe, spec, object := assemblyGuardFixture(t)
	view, err := b.AcquireDocumentImageAssemblyInputs(context.Background(), spec, []backend.DocumentImageAssemblyRetainedObject{object})
	if err != nil {
		t.Fatal(err)
	}
	prepared := backend.PreparedDocumentImageAssembly{Spec: spec}
	prepared.Spec.Identity.EngineID, prepared.Spec.Identity.InstanceID = probe.info.ID, b.instance
	if err := b.bindAssemblyInputs(context.Background(), prepared.Spec); err != nil {
		t.Fatal(err)
	}
	blocked := &assemblyBlockedCreateProbe{documentProbe: probe, entered: make(chan struct{}), release: make(chan struct{})}
	b.api = blocked
	var once sync.Once
	release := func() { once.Do(func() { close(blocked.release) }) }
	defer func() {
		release()
		_ = b.releaseAssemblySources(prepared.Spec.Identity)
	}()
	created := make(chan error, 1)
	go func() {
		// Call the exact production allocation edge. The fake spec is NOT a
		// closed-input fixture and makes no preparation/domain acceptance claim.
		_, err := b.createOwnedAssemblyAllocation(context.Background(), prepared)
		created <- err
	}()
	select {
	case <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("owned SDK Create was not entered")
	}
	if b.ReleaseUnallocatedDocumentImageAssemblyInputs(context.Background(), spec) == nil {
		t.Fatal("live lost-ACK Create was released as unallocated")
	}
	if _, err := b.createOwnedAssemblyAllocation(context.Background(), prepared); err == nil {
		t.Fatal("duplicate allocation reached SDK")
	}
	type result struct {
		proof backend.DocumentImageAssemblyFenceProof
		err   error
	}
	fenced := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		proof, err := b.FenceDocumentImageAssembly(ctx, backend.DocumentImageAssemblyFenceRequest{Identity: prepared.Spec.Identity, IntentID: strings.Repeat("a", 32)})
		fenced <- result{proof, err}
	}()
	select {
	case outcome := <-fenced:
		if outcome.err != nil || outcome.proof.FenceRef == "" || outcome.proof.AcknowledgedAt == "" {
			t.Fatal("real acknowledged fence did not settle", outcome)
		}
	case <-time.After(time.Second):
		release()
		<-created
		select {
		case <-fenced:
		case <-time.After(time.Second):
		}
		t.Fatal("source mutex stranded acknowledged fence behind blocked SDK Create")
	}
	// The old SDK call is still blocked, but exact source closure is proven.
	select {
	case <-created:
		t.Fatal("fixture released old Create early")
	default:
	}
	if !b.sourceClaim.closed || view.CheckCurrent(context.Background()) == nil {
		t.Fatal("fence lost source closure ownership")
	}
	if err := os.WriteFile(object.Path, []byte("two"), 0600); err != nil {
		t.Fatal("positive guard closure remained blocked", err)
	}
	if len(probe.creates) != 2 || len(probe.starts) != 0 {
		t.Fatal("duplicate work or marker execution", probe.creates, probe.starts)
	}
	release()
	select {
	case err := <-created:
		if !errors.Is(err, backend.ErrUnavailable) {
			t.Fatal("late lost ACK became success", err)
		}
	case <-time.After(time.Second):
		t.Fatal("old SDK caller did not settle")
	}
}
