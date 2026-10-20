package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func TestDocumentAssemblyStageWritesExactGeneratedInputsWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	live := &liveDocumentAssemblyStage{}
	data := []byte("derived-closed-lock\n")
	if e := writeAssemblyStageFile(context.Background(), live, root, "verification/installed-0000.json", nil, int64(len(data)), assemblyCatalogDigest(data), data, false); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "verification", "installed-0000.json")
	got, e := os.ReadFile(path)
	if e != nil || string(got) != string(data) {
		t.Fatal("exact generated source not retained", e)
	}
	if e = writeAssemblyStageFile(context.Background(), live, root, "verification/installed-0000.json", nil, int64(len(data)), assemblyCatalogDigest(data), data, false); e == nil {
		t.Fatal("existing staging bytes overwritten")
	}
	if live.writing || live.writerUnknown {
		t.Fatal("a returned positive file close retained an active writer")
	}
}

func TestDocumentAssemblyStageRefusesEscapesWrongDigestAndCancelledWrites(t *testing.T) {
	for _, name := range []string{"escape", "wrong-digest", "cancelled", "retiring"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			live := &liveDocumentAssemblyStage{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			path := "payload"
			digest := assemblyCatalogDigest([]byte("payload"))
			switch name {
			case "escape":
				path = "../outside"
			case "wrong-digest":
				digest = strings.Repeat("a", 64)
			case "cancelled":
				cancel()
			case "retiring":
				live.retiring = true
			}
			if e := writeAssemblyStageFile(ctx, live, root, path, nil, 7, digest, []byte("payload"), false); e == nil {
				t.Fatal("unusable source became staged input")
			}
			if live.writing {
				t.Fatal("synchronous writer remained active after return")
			}
		})
	}
}

func TestDocumentAssemblyRetirementCannotInventReturnOfOriginalAcquisitionOrWriter(t *testing.T) {
	for _, name := range []string{"opening", "writing", "unknown-close"} {
		t.Run(name, func(t *testing.T) {
			live := &liveDocumentAssemblyStage{}
			switch name {
			case "opening":
				live.opening = true
			case "writing":
				live.writing = true
			case "unknown-close":
				live.writerUnknown = true
			}
			g := &executionAdmission{}
			if e := g.closeDocumentAssemblyStage(context.Background(), documentImageAssemblyLease{}, live); e == nil || !live.retiring {
				t.Fatal("deadline/nil-owner falsely settled an original operation")
			}
		})
	}
}

func TestDocumentAssemblyStagingLeaseBindsCurrentJobAndSeparateDemand(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	row := assemblySharedAdmit(t, g, req, f, h)
	spec := req.Spec
	spec.Identity = row.Identity
	selection := runtimeinstall.DocumentAssemblyArtifact{InputClosureSHA256: row.InputClosureRef.Digest, ProfileSHA256: strings.Repeat("a", 64), SourceSHA256: strings.Repeat("b", 64)}
	row.Spec = &spec
	row.DocumentSelection = &selection
	row.DocumentJobID = "assembly-job"
	row.State = "staging"
	demand := backend.DocumentDomainInputDemand{Inputs: 1, OriginalHandles: 3, DeclaredBytes: 1, ReadCalls: backend.DocumentDomainInputReadCalls, ReadBytes: backend.DocumentDomainInputReadBytes, BlockBytes: backend.DocumentDomainInputBlockBytes, ReadBufferBytes: 2 * backend.DocumentDomainInputBlockBytes, AcquisitionWorkers: 1, RetirementWorkers: 1, CancellationWorkers: 1, CancellationCallbacks: 3}
	row.Stage = &documentAssemblyStageBinding{InputsDigest: strings.Repeat("c", 64), Demand: demand, DirectoryBytes: 1, State: "reserved", WriterHandles: 1, WriterBufferBytes: 64 << 10, StageWorkers: 1, StageRetirementWorkers: 1}
	if !validDocumentImageAssemblyLease(row) {
		t.Fatal("closed staging row refused")
	}
	for _, name := range []string{"missing-job", "caller-artifact", "opened-without-charge", "writer", "past-stage"} {
		t.Run(name, func(t *testing.T) {
			copy := row
			binding := *row.Stage
			selected := selection
			copy.Stage = &binding
			copy.DocumentSelection = &selected
			switch name {
			case "missing-job":
				copy.DocumentJobID = ""
			case "caller-artifact":
				selected.Ref = strings.Repeat("d", 64)
			case "opened-without-charge":
				binding.Demand.OriginalHandles = 0
			case "writer":
				binding.WriterHandles = 2
			case "past-stage":
				copy.State = "reserved"
			}
			if validDocumentImageAssemblyLease(copy) {
				t.Fatal("uncaptured or unretired staging authority accepted")
			}
		})
	}
}

func TestDocumentAssemblyStageDeadlineRetainsBlockedOriginalUntilActualReturn(t *testing.T) {
	for _, phase := range []string{"open", "write", "sync", "close"} {
		t.Run(phase, func(t *testing.T) {
			g, req, f, h := assemblySharedFixture(t)
			row := assemblySharedAdmit(t, g, req, f, h)
			spec := req.Spec
			spec.Identity = row.Identity
			selection := runtimeinstall.DocumentAssemblyArtifact{InputClosureSHA256: row.InputClosureRef.Digest, ProfileSHA256: strings.Repeat("a", 64), SourceSHA256: strings.Repeat("b", 64)}
			row.Spec = &spec
			row.DocumentSelection = &selection
			row.DocumentJobID = "assembly-job"
			row.State = "staging"
			row.Stage = &documentAssemblyStageBinding{InputsDigest: strings.Repeat("c", 64), Demand: backend.DocumentDomainInputDemand{Inputs: 1, OriginalHandles: 3, DeclaredBytes: 1, ReadCalls: backend.DocumentDomainInputReadCalls, ReadBytes: backend.DocumentDomainInputReadBytes, BlockBytes: backend.DocumentDomainInputBlockBytes, ReadBufferBytes: 2 * backend.DocumentDomainInputBlockBytes, AcquisitionWorkers: 1, RetirementWorkers: 1, CancellationWorkers: 1, CancellationCallbacks: 3}, DirectoryBytes: 1, State: "reserved", WriterHandles: 1, WriterBufferBytes: 64 << 10, StageWorkers: 1, StageRetirementWorkers: 1}
			if e := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
				i, e := assemblyLeaseIndex(l, row.Identity)
				if e == nil {
					l.ImageAssemblies[i] = row
				}
				return e
			}); e != nil {
				t.Fatal(e)
			}
			live := &liveDocumentAssemblyStage{}
			entered, release := make(chan struct{}), make(chan struct{})
			returned := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				returned <- g.runDocumentAssemblyStageWork(ctx, row, live, func() error {
					live.mu.Lock()
					if phase == "open" {
						live.opening = true
					} else {
						live.writing = true
					}
					live.mu.Unlock()
					close(entered)
					<-release
					live.mu.Lock()
					live.opening = false
					live.writing = false
					live.mu.Unlock()
					return ctx.Err()
				})
			}()
			<-entered
			cancel()
			select {
			case e := <-returned:
				if e == nil {
					t.Fatal("deadline accepted pending original")
				}
			case <-time.After(time.Second):
				t.Fatal("caller waited for synchronous original IO")
			}
			live.mu.Lock()
			pending := live.workerRunning && live.retiring
			retired := live.retirementDone
			live.mu.Unlock()
			if !pending {
				t.Fatal("timer discarded original worker custody")
			}
			select {
			case <-retired:
				t.Fatal("retirement acknowledged before actual return")
			default:
			}
			ledger, e := g.readReservationLedger(context.Background())
			if e != nil || ledger.ImageAssemblies[0].State != "staging" {
				t.Fatal("pending original lost durable charge", e)
			}
			close(release)
			select {
			case <-retired:
			case <-time.After(time.Second):
				t.Fatal("returned original never reached settlement")
			}
			ledger, e = g.readReservationLedger(context.Background())
			if e != nil || ledger.ImageAssemblies[0].State != "finished" {
				t.Fatal("actual positive original return did not settle exact stage", e)
			}
		})
	}
}

func TestDocumentAssemblyStageHelperModeDoesNotAlterItsPinnedBytes(t *testing.T) {
	root := t.TempDir()
	live := &liveDocumentAssemblyStage{}
	payload := []byte("synthetic-helper-not-executed")
	if e := writeAssemblyStageFile(context.Background(), live, root, "helper", nil, int64(len(payload)), assemblyCatalogDigest(payload), payload, true); e != nil {
		t.Fatal(e)
	}
	file, e := os.Stat(filepath.Join(root, "helper"))
	if e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" && file.Mode().Perm() != 0700 {
		t.Fatal("Linux assembly helper not executable by its owner")
	}
	actual, e := os.ReadFile(filepath.Join(root, "helper"))
	if e != nil || string(actual) != string(payload) {
		t.Fatal("mode change replaced pinned helper bytes", e)
	}
}

func TestDocumentAssemblyOriginalWholeDeadlineDoesNotScheduleLateCleanup(t *testing.T) {
	g := &executionAdmission{}
	original := make(chan struct{})
	live := &liveDocumentAssemblyStage{workerDone: original, workerRunning: true}
	row := documentImageAssemblyLease{WholeDeadline: time.Now().Add(-time.Second)}
	done, e := g.beginDocumentAssemblyStageRetirement(context.Background(), row, live, true, false)
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expired original cleanup waited for blocked stage")
	}
	live.mu.Lock()
	e = live.retirementErr
	retained := live.workerRunning && live.retiring
	live.mu.Unlock()
	if !errors.Is(e, context.DeadlineExceeded) || !retained {
		t.Fatal("original deadline manufactured return or fresh cleanup window", e)
	}
	// A late actual return does not restart the old cleanup worker. Only a later
	// explicitly authorized reconcile can request its separate cleanup window.
	live.mu.Lock()
	live.workerRunning = false
	close(original)
	same := live.retirementDone
	live.mu.Unlock()
	if same != done {
		t.Fatal("late return silently created another cleanup worker")
	}
}

func TestDocumentAssemblyRecoveryRetainsOneOriginalStageAcrossObservers(t *testing.T) {
	g := &executionAdmission{path: filepath.Join(t.TempDir(), "ledger"), scope: strings.Repeat("a", 64)}
	first := g.retainRecoveredDocumentAssemblyStage("assembly-one")
	defer liveDocumentAssemblyStages.Delete(g.assemblyStageKey("assembly-one"))
	first.mu.Lock()
	first.retiring = true
	first.retirementDone = make(chan struct{})
	first.mu.Unlock()
	second := g.retainRecoveredDocumentAssemblyStage("assembly-one")
	if second != first || second.retirementDone != first.retirementDone {
		t.Fatal("recovery replaced an outstanding original retirement worker")
	}
	other := g.retainRecoveredDocumentAssemblyStage("assembly-two")
	defer liveDocumentAssemblyStages.Delete(g.assemblyStageKey("assembly-two"))
	if other == first {
		t.Fatal("different exact allocations shared retirement custody")
	}
}
