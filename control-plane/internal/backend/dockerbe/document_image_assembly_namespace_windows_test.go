//go:build windows

package dockerbe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// These sources require actual Windows temporary directory handles unless
// explicitly labelled metadata-only. No spool, child process or Docker exists.
func assemblyNamespaceFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "namespace")
	if err := os.Mkdir(path, 0700); err != nil { t.Fatal(err) }
	return path
}

func TestAssemblyNamespacePrepareIsReadFreeAndRetirementStopsUnstarted(t *testing.T) {
	// This leaf does not exist. Successful Prepare therefore cannot be an open
	// or a positive directory identity observation.
	directory := filepath.Join(t.TempDir(), "not-created")
	a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(directory)
	if err != nil { t.Fatal(err) }
	lease, err := a.RegistryLease(); if err != nil { t.Fatal(err) }
	charge, err := lease.Charge(); if err != nil { t.Fatal(err) }
	n, err := charge.Slots(); if err != nil || n < 1 || n > 65 { t.Fatal(n, err) }
	copyShell := *a
	other, _ := copyShell.RegistryLease(); same, _ := other.Charge()
	if !charge.SameRegistry(same) || charge.SameRegistry(DocumentImageAssemblyOriginalRegistryCharge{}) { t.Fatal("registry identity forged or lost") }
	foreign, err := PrepareDocumentImageAssemblyNamespaceAcquisition(directory); if err != nil { t.Fatal(err) }
	foreignLease, _ := foreign.RegistryLease(); foreignCharge, _ := foreignLease.Charge()
	if charge.SameRegistry(foreignCharge) { t.Fatal("equal path forged the original registry lifetime") }
	foreign.RequestRetirement()
	state, _ := lease.RetirementState()
	if state.CollectorStarted || state.CollectorStopped || state.Planned != n || len(a.state.guard.handles) != 0 { t.Fatal("Prepare performed original effects", state) }
	copyShell.RequestRetirement()
	state, _ = lease.RetirementState()
	if !state.Retiring || !state.CollectorStopped || !state.AllSlotsClosed || state.CollectorStarted { t.Fatal("unstarted slots not positively discarded", state) }
	if g, err := a.Acquire(context.Background()); g != nil || err == nil { t.Fatal("retired shell started") }
	if _, err := os.Lstat(directory); !os.IsNotExist(err) { t.Fatal("Prepare created namespace", err) }
}

func TestAssemblyNamespaceRejectsBoundedMalformedMetadata(t *testing.T) {
	for _, path := range []string{"", "relative", `\\server\share\directory`, `C:\a\..\b`, `C:\a:stream`, "C:\\a\x00b", "C:\\a\xff", "C:\\"+strings.Repeat("x", 32761), "C:\\"+strings.Repeat("a\\", 65)+"leaf"} {
		if a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(path); a != nil || err == nil { t.Fatal("malformed namespace planned", path, err) }
	}
}

func TestAssemblyNamespaceOriginalDirectoryTransferKeepsStableRegistry(t *testing.T) {
	path := assemblyNamespaceFixture(t)
	a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(path); if err != nil { t.Fatal(err) }
	lease, _ := a.RegistryLease(); charge, _ := lease.Charge()
	g, err := a.Acquire(context.Background()); if err != nil { if g != nil { _ = g.Close() }; t.Fatal(err) }
	defer func() { _ = g.Close() }()
	copyShell := *a
	if extra, err := copyShell.Acquire(context.Background()); extra != nil || err == nil { t.Fatal("copied shell duplicated collection") }
	if err := os.WriteFile(filepath.Join(path, "sibling.txt"), []byte("allowed sibling"), 0600); err != nil { t.Fatal(err) }
	if err := g.CheckCurrent(context.Background()); err != nil { t.Fatal("directory content timestamps became identity", err) }
	if os.Rename(path, path+"-moved") == nil { t.Fatal("held directory allowed replacement") }
	old := g
	g, err = old.Transfer(context.Background()); if err != nil { g = old; t.Fatal(err) }
	if old.Close() == nil || old.CheckCurrent(context.Background()) == nil { t.Fatal("stale token retained custody") }
	currentLease, _ := a.RegistryLease(); currentCharge, _ := currentLease.Charge()
	if !charge.SameRegistry(currentCharge) || g.CheckCurrent(context.Background()) != nil { t.Fatal("transfer replaced registry/originals") }
	if g.Close() != nil || g.Close() != nil { t.Fatal("positive close failed") }
	state, _ := lease.RetirementState(); if !state.AllSlotsClosed || !state.CollectorStopped { t.Fatal(state) }
	if os.Rename(path, path+"-moved") != nil { t.Fatal("closed namespace still locked") }
}

func TestAssemblyNamespacePartialValidACKBeforeErrorRetainsExactOriginal(t *testing.T) {
	a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(assemblyNamespaceFixture(t)); if err != nil { t.Fatal(err) }
	a.state.guard.openFile = func(name *uint16, access, share uint32, sa *syscall.SecurityAttributes, mode, flags uint32, template int32) (syscall.Handle, error) {
		h, err := syscall.CreateFile(name, access, share, sa, mode, flags, template)
		if err != nil { return h, err }
		return h, errors.New("synthetic error accompanying original ACK")
	}
	g, err := a.Acquire(context.Background()); if err == nil || g == nil { t.Fatal("partial original lost", err) }
	defer g.Close()
	lease, _ := a.RegistryLease(); state, _ := lease.RetirementState()
	if !state.CollectorStopped || state.ValidACK != 1 || state.AllSlotsClosed || len(a.state.guard.handles) != 1 { t.Fatal(state) }
	if g.CheckCurrent(context.Background()) == nil { t.Fatal("partial namespace eligible") }
	if next, err := g.Transfer(context.Background()); next != nil || err == nil { t.Fatal("partial namespace moved") }
	if g.Close() != nil { t.Fatal("exact partial original could not close") }
}

func TestAssemblyNamespaceBlockedCollectorRetiresBeforeLateACKAndClose(t *testing.T) {
	a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(assemblyNamespaceFixture(t)); if err != nil { t.Fatal(err) }
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	returned := make(chan struct{ g backend.DocumentImageAssemblyNamespaceGuard; err error }, 1)
	calls := 0
	a.state.guard.openFile = func(name *uint16, access, share uint32, sa *syscall.SecurityAttributes, mode, flags uint32, template int32) (syscall.Handle, error) {
		calls++
		h, err := syscall.CreateFile(name, access, share, sa, mode, flags, template)
		close(entered); <-release
		return h, err
	}
	consumed := false
	defer func() {
		releaseOnce.Do(func() { close(release) })
		if !consumed { result := <-returned; if result.g != nil { _ = result.g.Close() } }
	}()
	go func() { g, err := a.Acquire(context.Background()); returned <- struct{ g backend.DocumentImageAssemblyNamespaceGuard; err error }{g, err} }()
	select { case <-entered: case <-time.After(time.Second): t.Fatal("collector did not enter original open") }
	a.RequestRetirement() // cannot wait for the blocked collector/native ACK
	lease, _ := a.RegistryLease(); state, _ := lease.RetirementState()
	if !state.Retiring || state.CollectorStopped || state.PendingOpen != 1 { t.Fatal(state) }
	// Lower boundary only: the external caller does not yet receive this token.
	beforeReturn := &assemblyNamespaceToken{a.state.namespace, 1}
	if beforeReturn.Close() == nil { t.Fatal("collector-inflight originals closed") }
	releaseOnce.Do(func() { close(release) })
	result := <-returned; consumed = true
	if result.g == nil || result.err == nil || calls != 1 { t.Fatal("late ACK became success/new open", result.err, calls) }
	defer result.g.Close()
	state, _ = lease.RetirementState()
	if !state.CollectorStopped || state.ValidACK != 1 || state.AllSlotsClosed { t.Fatal("late original discarded", state) }
	if result.g.Close() != nil { t.Fatal("late exact original not closed") }
}

func TestAssemblyNamespaceFailedCloseKeepsCurrentTokenAndCharge(t *testing.T) {
	a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(assemblyNamespaceFixture(t)); if err != nil { t.Fatal(err) }
	g, err := a.Acquire(context.Background()); if err != nil { if g != nil { _ = g.Close() }; t.Fatal(err) }
	defer func() { a.state.guard.closeHandle = nil; _ = g.Close() }()
	a.state.guard.closeHandle = func(syscall.Handle) error { return errors.New("synthetic close uncertainty") }
	if g.Close() == nil { t.Fatal("failed native close became positive") }
	lease, _ := a.RegistryLease(); state, _ := lease.RetirementState()
	if state.AllSlotsClosed || state.UnknownClose == 0 || len(a.state.guard.handles) == 0 { t.Fatal("failed handles/charge lost", state) }
	if next, err := g.Transfer(context.Background()); next != nil || err == nil { t.Fatal("failed close moved token") }
	a.RequestRetirement()
	a.state.guard.closeHandle = nil
	if g.Close() != nil { t.Fatal("retained exact originals cannot settle") }
	state, _ = lease.RetirementState(); if !state.AllSlotsClosed { t.Fatal(state) }
}

func TestAssemblyNamespaceRetirementDoesNotWaitForIdentityIO(t *testing.T) {
	a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(assemblyNamespaceFixture(t)); if err != nil { t.Fatal(err) }
	g, err := a.Acquire(context.Background()); if err != nil { if g != nil { _ = g.Close() }; t.Fatal(err) }
	defer g.Close()
	native := a.state.guard
	native.mu.Lock(); locked := true
	finished := make(chan error, 1)
	defer func() { if locked { native.mu.Unlock(); <-finished } }()
	go func() { finished <- g.CheckCurrent(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for {
		a.state.namespace.mu.Lock(); busy := a.state.namespace.busy; a.state.namespace.mu.Unlock()
		if busy { break }
		if time.Now().After(deadline) { t.Fatal("identity operation did not enter") }
		time.Sleep(time.Millisecond)
	}
	a.RequestRetirement()
	lease, _ := a.RegistryLease(); state, _ := lease.RetirementState(); if !state.Retiring || state.AllSlotsClosed { t.Fatal(state) }
	if next, err := g.Transfer(context.Background()); next != nil || err == nil { t.Fatal("retirement permitted transfer") }
	if g.Close() == nil { t.Fatal("identity borrower was closed") }
	native.mu.Unlock(); locked = false
	if err := <-finished; err == nil { t.Fatal("retired identity operation succeeded") }
	if g.Close() != nil { t.Fatal("returned identity borrower cannot retire") }
}

func TestAssemblyNamespaceOrdinaryTypeAndReparseAncestorsRefuse(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0600); err != nil { t.Fatal(err) }
	for _, path := range []string{file, filepath.Join(file, "child")} {
		a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(path); if err != nil { t.Fatal(err) }
		g, err := a.Acquire(context.Background()); if err == nil || g == nil { t.Fatal("ordinary-directory requirement weakened", err) }
		if g.Close() != nil { t.Fatal("refused ordinary-type partial originals lost") }
	}
	actual := filepath.Join(root, "actual"); if err := os.Mkdir(actual, 0700); err != nil { t.Fatal(err) }
	link := filepath.Join(root, "redirect")
	// A setup error fails this true Windows fixture; it is never a skipped pass
	// or simulated reparse identity when symlink privilege is unavailable.
	if err := os.Symlink(actual, link); err != nil { t.Fatal("Windows reparse fixture setup unavailable", err) }
	for _, path := range []string{link, filepath.Join(link, "child")} {
		a, err := PrepareDocumentImageAssemblyNamespaceAcquisition(path); if err != nil { t.Fatal(err) }
		g, err := a.Acquire(context.Background()); if err == nil || g == nil { t.Fatal("reparse path accepted", err) }
		if g.Close() != nil { t.Fatal("reparse refusal lost exact originals") }
	}
}
