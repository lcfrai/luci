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

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Deferred Windows temporary original-file handle tests only. No assemblySpec,
// Backend, SDK, native process, package, key verifier or Docker daemon exists.
func assemblyEvidenceFixture(t *testing.T) backend.DocumentImageAssemblyEvidenceObject {
	t.Helper()
	path := filepath.Join(t.TempDir(), "original.json")
	if err := os.WriteFile(path, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	return backend.DocumentImageAssemblyEvidenceObject{Role: "raw:one", Path: path, Bytes: 3, Digest: documentFixtureDigest([]byte("one"))}
}

func TestAssemblyEvidenceReadsOriginalHandleAndMovesSingleOwner(t *testing.T) {
	object := assemblyEvidenceFixture(t)
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = guard.Close() }()
	for i := 0; i < 2; i++ {
		data, err := guard.ReadBounded(context.Background(), object.Role, 3)
		if err != nil || string(data) != "one" {
			t.Fatal("original seek/read/hash failed", string(data), err)
		}
	}
	if os.WriteFile(object.Path, []byte("two"), 0600) == nil || os.Rename(object.Path, object.Path+"-moved") == nil {
		t.Fatal("source changed while original handle held")
	}
	old := guard
	next, err := old.Transfer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	guard = next
	if old.Close() == nil || old.CheckCurrent(context.Background()) == nil {
		t.Fatal("old token retained close/read authority")
	}
	if _, err = old.ReadBounded(context.Background(), object.Role, 3); err == nil {
		t.Fatal("stale token read current owner's handle")
	}
	if guard.CheckCurrent(context.Background()) != nil {
		t.Fatal("transfer replaced or closed original handle")
	}
	if guard.Close() != nil || guard.Close() != nil {
		t.Fatal("exact positive closure failed")
	}
	if os.WriteFile(object.Path, []byte("two"), 0600) != nil {
		t.Fatal("positive closure left handle locked")
	}
}

func TestAssemblyEvidencePartialAcquisitionAndFailedCloseRetainExactOwner(t *testing.T) {
	object := assemblyEvidenceFixture(t)
	missing := object
	missing.Role, missing.Path = "raw:missing", object.Path+"-missing"
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object, missing})
	if err == nil || guard == nil {
		t.Fatal("partial acquisition lost ownership")
	}
	token := guard.(*assemblyEvidenceToken)
	defer func() { token.owner.guard.closeHandle = nil; _ = guard.Close() }()
	if guard.CheckCurrent(context.Background()) == nil {
		t.Fatal("partial set became eligible")
	}
	if _, err := guard.Transfer(context.Background()); err == nil {
		t.Fatal("partial set transferred")
	}
	token.owner.guard.closeHandle = func(syscall.Handle) error { return errors.New("inert exact close failed") }
	if guard.Close() == nil || len(token.owner.guard.handles) == 0 {
		t.Fatal("failed close lost exact handles")
	}
	if _, err := guard.ReadBounded(context.Background(), object.Role, 3); err == nil {
		t.Fatal("closing set read bytes")
	}
	if os.WriteFile(object.Path, []byte("two"), 0600) == nil {
		t.Fatal("failed close released original object")
	}
	token.owner.guard.closeHandle = nil
	if guard.Close() != nil || len(token.owner.guard.handles) != 0 {
		t.Fatal("exact remaining handles not retired")
	}
}

func TestAssemblyEvidenceDigestFailureAndReadBudgetNeverAcceptPartialBytes(t *testing.T) {
	object := assemblyEvidenceFixture(t)
	wrong := object
	wrong.Digest = strings.Repeat("9", 64)
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{wrong})
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	if data, err := guard.ReadBounded(context.Background(), object.Role, 3); err == nil || data != nil {
		t.Fatal("wrong hash accepted data", data, err)
	}
	if guard.CheckCurrent(context.Background()) == nil {
		t.Fatal("failed evidence read stayed eligible")
	}
	if _, err := guard.Transfer(context.Background()); err == nil {
		t.Fatal("poisoned bytes transferred")
	}
	if guard.Close() != nil {
		t.Fatal("failed read lost close ownership")
	}
	guard, err = AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object})
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	token := guard.(*assemblyEvidenceToken)
	if data, err := guard.ReadBounded(context.Background(), object.Role, 2); err == nil || data != nil || token.owner.readBytes != 0 {
		t.Fatal("maximum checked after I/O")
	}
	token.owner.readBytes = backend.DocumentImageAssemblyEvidenceMaxBytes - 2
	if data, err := guard.ReadBounded(context.Background(), object.Role, 3); err == nil || data != nil || token.owner.readBytes != backend.DocumentImageAssemblyEvidenceMaxBytes-2 {
		t.Fatal("lifetime budget exceeded before refusal")
	}
}

func TestAssemblyEvidenceFiniteRolesSizesPathsAndCancelledTransfer(t *testing.T) {
	object := assemblyEvidenceFixture(t)
	for name, objects := range map[string][]backend.DocumentImageAssemblyEvidenceObject{
		"duplicate-role": {object, object},
		"duplicate-path": {object, {Role: "other", Path: object.Path, Bytes: 3, Digest: object.Digest}},
		"bad-role":       {{Role: "https://example.invalid", Path: object.Path, Bytes: 3, Digest: object.Digest}},
		"too-large":      {{Role: "one", Path: object.Path, Bytes: backend.DocumentImageAssemblyEvidenceMaxObjectBytes + 1, Digest: object.Digest}},
		"count":          make([]backend.DocumentImageAssemblyEvidenceObject, backend.DocumentImageAssemblyEvidenceMaxObjects+1),
	} {
		t.Run(name, func(t *testing.T) {
			guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), objects)
			if guard != nil {
				defer guard.Close()
			}
			if err == nil || guard != nil {
				t.Fatal("invalid set acquired native handles", err)
			}
		})
	}
	var aggregate []backend.DocumentImageAssemblyEvidenceObject
	for i := 0; i < 5; i++ {
		aggregate = append(aggregate, backend.DocumentImageAssemblyEvidenceObject{Role: fmt.Sprintf("object-%d", i), Path: object.Path + fmt.Sprintf("-%d", i), Bytes: backend.DocumentImageAssemblyEvidenceMaxObjectBytes, Digest: object.Digest})
	}
	if guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), aggregate); err == nil || guard != nil {
		t.Fatal("aggregate byte bound did not precede open")
	}
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object})
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if moved, err := guard.Transfer(ctx); err == nil || moved != nil {
		t.Fatal("cancelled transfer moved ownership")
	}
	if guard.CheckCurrent(context.Background()) != nil {
		t.Fatal("failed transfer invalidated original owner")
	}
	if data, err := guard.ReadBounded(ctx, object.Role, 3); err == nil || data != nil {
		t.Fatal("cancelled read accepted bytes")
	}
}

func TestAssemblyEvidenceNoAliasStreamOrSizeFallback(t *testing.T) {
	for _, mode := range []string{"hardlink", "alternate-stream", "size", "missing"} {
		t.Run(mode, func(t *testing.T) {
			object := assemblyEvidenceFixture(t)
			switch mode {
			case "hardlink":
				if err := os.Link(object.Path, object.Path+"-alias"); err != nil {
					t.Fatal(err)
				}
			case "alternate-stream":
				object.Path += ":alias"
			case "size":
				object.Bytes++
			case "missing":
				object.Path += "-absent"
			}
			guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object})
			if guard != nil {
				defer guard.Close()
			}
			if err == nil {
				t.Fatal("unsafe original source accepted")
			}
			if guard != nil && guard.CheckCurrent(context.Background()) == nil {
				t.Fatal("unsafe partial guard became current")
			}
		})
	}
}

func TestAssemblyEvidenceTransferBoundNeverDuplicatesOrClosesCurrentHandles(t *testing.T) {
	object := assemblyEvidenceFixture(t)
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = guard.Close() }()
	for i := 0; i < 256; i++ {
		old := guard
		next, moveErr := old.Transfer(context.Background())
		if moveErr != nil {
			t.Fatal(i, moveErr)
		}
		guard = next
		if old.Close() == nil {
			t.Fatal("old generation closed transferred native handles")
		}
	}
	if next, err := guard.Transfer(context.Background()); err == nil || next != nil {
		t.Fatal("transfer bound exceeded")
	}
	if guard.CheckCurrent(context.Background()) != nil {
		t.Fatal("refused transfer lost current owner")
	}
}

func TestAssemblyEvidenceEmptyNativeLogsRequireExactEOFAndHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.stdout.log")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	object := backend.DocumentImageAssemblyEvidenceObject{Role: "child-0:stdout", Path: path, Bytes: 0, Digest: documentFixtureDigest(nil)}
	guard, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{object})
	if err != nil {
		if guard != nil {
			_ = guard.Close()
		}
		t.Fatal(err)
	}
	defer guard.Close()
	data, err := guard.ReadBounded(context.Background(), object.Role, 1)
	if err != nil || data == nil || len(data) != 0 {
		t.Fatal("exact empty log refused", data, err)
	}
	token := guard.(*assemblyEvidenceToken)
	if token.owner.readBytes != 1 || token.owner.readCalls != 1 {
		t.Fatal("empty read bypassed lifetime accounting")
	}
	token.owner.readCalls = backend.DocumentImageAssemblyEvidenceMaxReads
	if data, err := guard.ReadBounded(context.Background(), object.Role, 1); err == nil || data != nil {
		t.Fatal("empty rereads bypassed finite call budget")
	}
	if guard.Close() != nil {
		t.Fatal("empty log native handle not closed")
	}
	for _, mode := range []string{"wrong-hash", "negative", "nonempty-original"} {
		t.Run(mode, func(t *testing.T) {
			changed := object
			switch mode {
			case "wrong-hash":
				changed.Digest = strings.Repeat("9", 64)
			case "negative":
				changed.Bytes = -1
			case "nonempty-original":
				changed.Path = assemblyEvidenceFixture(t).Path
			}
			partial, err := AcquireDocumentImageAssemblyEvidence(context.Background(), []backend.DocumentImageAssemblyEvidenceObject{changed})
			if partial != nil {
				defer partial.Close()
			}
			if err == nil {
				t.Fatal("invalid empty-log claim accepted")
			}
			if partial != nil && partial.CheckCurrent(context.Background()) == nil {
				t.Fatal("failed exact0size became current")
			}
		})
	}
}
