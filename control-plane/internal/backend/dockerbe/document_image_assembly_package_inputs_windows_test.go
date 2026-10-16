//go:build windows

package dockerbe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Deferred real Windows TEMP-file/handle fixtures. These are NOT package bytes,
// parser/producer/signature checks or native process/Docker acceptance. Truncate
// gives the finite144257468 declared set without reading/hashing any payload.
func assemblyPackageInputFixture(t *testing.T) []backend.DocumentImageAssemblyEvidenceObject {
	t.Helper()
	directory := t.TempDir()
	const largest int64 = 56547048
	remaining := backend.DocumentImageAssemblyPackageInputBytes - largest
	base := remaining / 173
	objects := make([]backend.DocumentImageAssemblyEvidenceObject, 174)
	for ordinal := range objects {
		size := base
		if ordinal == 0 {
			size = largest
		} else if ordinal == 173 {
			size = remaining - base*172
		}
		path := filepath.Join(directory, fmt.Sprintf("original-%03d.deb", ordinal))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		err = file.Truncate(size)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		// Intentionally a declared digest, not a hash of the synthetic zero file.
		// Admission must not read payload or turn metadata into authentication.
		objects[ordinal] = backend.DocumentImageAssemblyEvidenceObject{Role: fmt.Sprintf("package.%03d", ordinal), Path: path, Bytes: size, Digest: strings.Repeat("a", 64)}
	}
	return objects
}

func TestAssemblyPackageInputsReadFreeExactLargeSetAndGeneration(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if err != nil {
		if guard != nil {
			_ = guard.Close()
		}
		t.Fatal(err)
	}
	defer func() { _ = guard.Close() }()
	if guard.CheckCurrent(context.Background()) != nil {
		t.Fatal("original metadata changed")
	}
	if os.WriteFile(objects[0].Path, []byte("changed"), 0600) == nil || os.Rename(objects[0].Path, objects[0].Path+"-moved") == nil {
		t.Fatal("held original was mutable")
	}
	old := guard
	next, err := old.Transfer(context.Background())
	if err != nil || next == nil {
		t.Fatal(err)
	}
	guard = next
	if old.Close() == nil || old.CheckCurrent(context.Background()) == nil {
		t.Fatal("old generation retained authority")
	}
	if guard.CheckCurrent(context.Background()) != nil || guard.Close() != nil || guard.Close() != nil {
		t.Fatal("moved exact owner failed closure")
	}
	if os.WriteFile(objects[0].Path, []byte("closed"), 0600) != nil {
		t.Fatal("positive closure left original locked")
	}
	// The distinct profile must not increase the existing small capture guard.
	if small, e := AcquireDocumentImageAssemblyEvidence(context.Background(), objects[:1]); e == nil || small != nil {
		if small != nil {
			_ = small.Close()
		}
		t.Fatal("default16MiB bound changed")
	}
}

func TestAssemblyPackageInputsRejectsFiniteScopeBeforeNativeAcquisition(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	for _, which := range []string{"count", "role", "duplicate-path", "bytes", "aggregate", "digest", "relative", "ads"} {
		t.Run(which, func(t *testing.T) {
			rows := append([]backend.DocumentImageAssemblyEvidenceObject(nil), objects...)
			switch which {
			case "count":
				rows = rows[:173]
			case "role":
				rows[0].Role = "raw:package"
			case "duplicate-path":
				rows[1].Path = rows[0].Path
			case "bytes":
				rows[0].Bytes = 64<<20 + 1
			case "aggregate":
				rows[0].Bytes++
			case "digest":
				rows[0].Digest = "not-a-digest"
			case "relative":
				rows[0].Path = "original.deb"
			case "ads":
				rows[0].Path += ":hidden"
			}
			guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), rows)
			if guard != nil {
				_ = guard.Close()
				t.Fatal("metadata refusal returned native ownership")
			}
			if err == nil {
				t.Fatal("bad scope accepted")
			}
		})
	}
}

func TestAssemblyPackageInputsPartialAndFailedCloseRetainOriginals(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	objects[173].Path += "-missing"
	guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if err == nil || guard == nil {
		t.Fatal("partial native set lost ownership", err)
	}
	token := guard.(*assemblyPackageInputToken)
	defer func() { token.owner.guard.closeHandle = nil; _ = guard.Close() }()
	if guard.CheckCurrent(context.Background()) == nil {
		t.Fatal("partial set was current")
	}
	if moved, e := guard.Transfer(context.Background()); e == nil || moved != nil {
		t.Fatal("partial set moved")
	}
	token.owner.guard.closeHandle = func(syscall.Handle) error { return errors.New("synthetic exact CloseHandle failure") }
	if guard.Close() == nil || len(token.owner.guard.handles) == 0 {
		t.Fatal("failed close erased ownership")
	}
	if os.WriteFile(objects[0].Path, []byte("changed"), 0600) == nil {
		t.Fatal("failed close released original")
	}
	token.owner.guard.closeHandle = nil
	if guard.Close() != nil || len(token.owner.guard.handles) != 0 {
		t.Fatal("remaining originals did not settle")
	}
}

func TestAssemblyPackageInputsHardlinkAndSizeDriftStayRetirementOnly(t *testing.T) {
	for _, which := range []string{"hardlink", "size-drift"} {
		t.Run(which, func(t *testing.T) {
			objects := assemblyPackageInputFixture(t)
			if which == "hardlink" {
				if err := os.Link(objects[0].Path, objects[0].Path+"-hardlink"); err != nil {
					t.Fatal("fixture hardlink unavailable", err)
				}
			} else {
				file, err := os.OpenFile(objects[0].Path, os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				err = file.Truncate(objects[0].Bytes - 1)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatal(err, closeErr)
				}
			}
			guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
			if err == nil || guard == nil {
				t.Fatal("native identity refusal lost original ownership", err)
			}
			defer guard.Close()
			if guard.CheckCurrent(context.Background()) == nil {
				t.Fatal("foreign/native drift remained current")
			}
			if moved, err := guard.Transfer(context.Background()); moved != nil || err == nil {
				t.Fatal("refused original set moved")
			}
			if guard.Close() != nil {
				t.Fatal("refused exact original failed closure")
			}
		})
	}
}

func TestAssemblyPackageInputsRetirementLatchesBeforeBlockedTransfer(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if err != nil {
		if guard != nil {
			_ = guard.Close()
		}
		t.Fatal(err)
	}
	token := guard.(*assemblyPackageInputToken)
	native := token.owner.guard
	native.mu.Lock()
	locked, settled := true, false
	type outcome struct {
		next backend.DocumentImageAssemblyPackageInputGuard
		err  error
	}
	done := make(chan outcome, 1)
	defer func() {
		if locked {
			native.mu.Unlock()
		}
		if !settled {
			select {
			case result := <-done:
				if result.next != nil {
					_ = result.next.Close()
				}
			case <-time.After(3 * time.Second):
				t.Error("blocked original check did not settle; ownership unknown")
				return
			}
		}
		if err := guard.Close(); err != nil {
			t.Error("exact final guard closure unknown", err)
		}
	}()
	go func() { next, err := guard.Transfer(context.Background()); done <- outcome{next, err} }()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		token.owner.mu.Lock()
		busy := token.owner.busy
		token.owner.mu.Unlock()
		if busy {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("transfer did not enter exact native boundary")
		case <-ticker.C:
		}
	}
	if guard.Close() == nil {
		t.Fatal("busy native owner falsely settled")
	}
	token.owner.mu.Lock()
	retiring, closed := token.owner.retiring, token.owner.closed
	token.owner.mu.Unlock()
	if !retiring || closed {
		t.Fatal("retirement latch waited for original check")
	}
	native.mu.Unlock()
	locked = false
	select {
	case result := <-done:
		settled = true
		if result.err == nil || result.next != nil {
			if result.next != nil {
				_ = result.next.Close()
			}
			t.Fatal("late transfer promoted after retirement")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("original check failed to settle")
	}
}

func TestAssemblyPackageInputsCancelledTransferAndFiniteMovesPreserveClose(t *testing.T) {
	objects := assemblyPackageInputFixture(t)
	guard, err := AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if err != nil {
		if guard != nil {
			_ = guard.Close()
		}
		t.Fatal(err)
	}
	defer func() { _ = guard.Close() }()
	for n := 0; n < backend.DocumentImageAssemblyPackageInputMaxTransfers; n++ {
		next, err := guard.Transfer(context.Background())
		if err != nil || next == nil {
			t.Fatal("finite transfer", n, err)
		}
		guard = next
	}
	if next, err := guard.Transfer(context.Background()); err == nil || next != nil {
		t.Fatal("transfer budget reset")
	}
	if guard.Close() != nil {
		t.Fatal("exhausted moves lost exact close")
	}
	guard, err = AcquireDocumentImageAssemblyPackageInputs(context.Background(), objects)
	if err != nil {
		if guard != nil {
			_ = guard.Close()
		}
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if next, err := guard.Transfer(ctx); err == nil || next != nil {
		t.Fatal("cancelled transfer promoted")
	}
	if guard.CheckCurrent(context.Background()) == nil {
		t.Fatal("cancelled attempt stayed eligible")
	}
	if guard.Close() != nil {
		t.Fatal("cancellation stranded original owner")
	}
}
