package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func storeConfig(t *testing.T) StoreConfig {
	return StoreConfig{Directory: filepath.Join(t.TempDir(), "checkpoint-store"), OwnerID: "owner-a", RuntimeScope: "scope-one", ControllerHostID: "controller-host", RuntimeHostID: "embed-host", SeparationReceiptDigest: strings.Repeat("e", 64), MaxBytes: 16 << 20, MaxObjects: 256}
}
func openStore(t *testing.T, cfg StoreConfig) *DirectoryStore {
	t.Helper()
	s, e := NewDirectoryStore(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := s.Close(); e != nil {
			t.Error(e)
		}
	})
	return s
}
func storedTransfer(t *testing.T, s *DirectoryStore, b TransferBinding) ([]byte, string) {
	t.Helper()
	var data bytes.Buffer
	p := payload("source/main.go", "package main")
	if b.Direction == "output" {
		p = payload("output/result.patch", "untrusted task output")
	}
	_, hash, e := WriteTransfer(&data, b, []Payload{p})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.PutTransfer(context.Background(), b, hash, bytes.NewReader(data.Bytes())); e != nil {
		t.Fatal(e)
	}
	return data.Bytes(), hash
}
func TestDirectoryStoreRestoresExactJournalAndArtifactsWithoutReplayingCreate(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	sc := storeConfig(t)
	s := openStore(t, sc)
	cfg.Checkpoint = s
	c := openController(t, cfg)
	req := taskRequest("alpha", "one")
	r, e := c.Allocate(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	binding := transferBinding()
	original, inputHash := storedTransfer(t, s, binding)
	returned := binding
	returned.Direction = "output"
	output, outputHash := storedTransfer(t, s, returned)
	head, e := s.Head(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s = openStore(t, sc)
	cfg.Checkpoint = s
	cfg.StateDir = filepath.Join(t.TempDir(), "restored")
	c, e = s.RestoreController(context.Background(), cfg, head)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	after, e := c.Reconcile(context.Background(), req.Identity.OperationID)
	if e != nil || after.SandboxID != r.SandboxID || p.creates != 1 {
		t.Fatal(after, e, p.creates)
	}
	var got bytes.Buffer
	if _, e = s.CopyTransfer(context.Background(), binding, inputHash, &got); e != nil || !bytes.Equal(got.Bytes(), original) {
		t.Fatal("source changed", e)
	}
	got.Reset()
	if _, e = s.CopyTransfer(context.Background(), returned, outputHash, &got); e != nil || !bytes.Equal(got.Bytes(), output) {
		t.Fatal("output changed", e)
	}
	after, e = c.Cancel(context.Background(), req.Identity.OperationID, after.Revision)
	requireAbsent(t, after, e)
}

func TestRestoredControllerReadsEveryOwnedVMBeforeNewCreate(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	sc := storeConfig(t)
	s := openStore(t, sc)
	cfg.Checkpoint = s
	c := openController(t, cfg)
	first := taskRequest("alpha", "one")
	second := taskRequest("beta", "two")
	if _, err := c.Allocate(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Allocate(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	head, err := s.Head(context.Background())
	if err != nil || c.Close(context.Background()) != nil || s.Close() != nil {
		t.Fatal("could not settle original controller", err)
	}
	s = openStore(t, sc)
	cfg.Checkpoint = s
	cfg.StateDir = filepath.Join(t.TempDir(), "restored")
	c, err = s.RestoreController(context.Background(), cfg, head)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	third := taskRequest("gamma", "three")
	if !c.RestoredPending(first.Identity.OperationID) || !c.RestoredPending(second.Identity.OperationID) {
		t.Fatal("restored ownership was not visible before provider readback")
	}
	if _, err = c.Allocate(context.Background(), first); !errors.Is(err, ErrUnknown) {
		t.Fatal("same-attempt Start could proceed before restored identity readback", err)
	}
	if _, err = c.Allocate(context.Background(), third); !errors.Is(err, ErrUnknown) || p.creates != 2 {
		t.Fatal("fresh Create preceded restored identity readback", err, p.creates)
	}
	p.foreign = true
	remaining, err := c.ReconcileRestored(context.Background(), 1)
	if remaining != 2 || !errors.Is(err, ErrConflict) {
		t.Fatal("foreign first VM did not stay charged", remaining, err)
	}
	p.foreign = false
	p.mu.Lock()
	delete(p.rows, "vm-2") // Exact provider absence settles only the healthy peer's VM charge.
	p.mu.Unlock()
	remaining, err = c.ReconcileRestored(context.Background(), 1)
	if remaining != 1 || err != nil {
		t.Fatal("healthy peer was starved by first VM", remaining, err)
	}
	if c.RestoredPending(second.Identity.OperationID) || !c.RestoredPending(first.Identity.OperationID) {
		t.Fatal("readback cleared another VM's recovery status")
	}
	_, usage := c.Snapshot()
	if usage.Active != 1 {
		t.Fatal("missing peer did not release only its exact VM reservation", usage)
	}
	if _, err = c.Allocate(context.Background(), third); !errors.Is(err, ErrUnknown) || p.creates != 2 {
		t.Fatal("new VM admitted while one restored identity remains", err, p.creates)
	}
	remaining, err = c.ReconcileRestored(context.Background(), 1)
	if remaining != 0 || err != nil {
		t.Fatal("exact restored readback did not release admission fence", remaining, err)
	}
	if _, err = c.Allocate(context.Background(), third); err != nil || p.creates != 3 || p.deletes != 0 {
		t.Fatal("fresh admission replayed or deleted prior VMs", err, p.creates, p.deletes)
	}
}
func TestDirectoryStoreRejectsLiveSecondWriterAndStaleRestore(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	s := openStore(t, storeConfig(t))
	cfg.Checkpoint = s
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	old, _ := s.Head(context.Background())
	otherCfg := cfg
	otherCfg.StateDir = filepath.Join(t.TempDir(), "clone")
	if other, e := New(otherCfg); other != nil || !errors.Is(e, ErrConflict) {
		t.Fatal(other, e)
	}
	if other, e := s.RestoreController(context.Background(), otherCfg, old); other != nil || !errors.Is(e, ErrConflict) {
		t.Fatal(other, e)
	}
	if e = s.Close(); !errors.Is(e, ErrConflict) {
		t.Fatal("closed store under active writer", e)
	}
	r, e = c.Cancel(context.Background(), r.Request.Identity.OperationID, r.Revision)
	requireAbsent(t, r, e)
	latest, _ := s.Head(context.Background())
	if old == latest {
		t.Fatal("head not advanced")
	}
	c.Close(context.Background())
	if other, e := s.RestoreController(context.Background(), otherCfg, old); other != nil || !errors.Is(e, ErrConflict) {
		t.Fatal("stale restore accepted", e)
	}
	if p.creates != 1 {
		t.Fatal("duplicate provider effect")
	}
}
func TestDirectoryStoreRefusesOverwriteAndImplicitRestore(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	s := openStore(t, storeConfig(t))
	cfg.Checkpoint = s
	c := openController(t, cfg)
	_, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	c.Close(context.Background())
	head, _ := s.Head(context.Background())
	before, _ := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if other, e := s.RestoreController(context.Background(), cfg, head); other != nil || !errors.Is(e, ErrConflict) {
		t.Fatal(other, e)
	}
	after, _ := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("restore overwrote existing journal")
	}
	cfg.StateDir = filepath.Join(t.TempDir(), "empty")
	if other, e := New(cfg); other != nil || !errors.Is(e, ErrConflict) {
		t.Fatal("empty clone ignored retained store head", e)
	}
}
func TestDirectoryStoreRuntimeHostReplacementRetainsUncertainCharge(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	s := openStore(t, storeConfig(t))
	cfg.Checkpoint = s
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	head, _ := s.Head(context.Background())
	c.Close(context.Background())
	cfg.StateDir = filepath.Join(t.TempDir(), "restored")
	cfg.Deployment.HostGeneration = "replacement-host"
	cfg.Deployment.RestoreReceiptDigest = strings.Repeat("f", 64)
	c, e = s.RestoreController(context.Background(), cfg, head)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	r, e = c.Reconcile(context.Background(), r.Request.Identity.OperationID)
	if !errors.Is(e, ErrUnknown) || !r.ResourcesHeld {
		t.Fatal(r, e)
	}
	if _, e = c.Allocate(context.Background(), taskRequest("beta", "one")); !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	if p.creates != 1 || p.deletes != 0 {
		t.Fatal("replacement host erased ownership")
	}
}
func TestDirectoryStoreRejectsSameHostMissingReceiptForeignOwnerAndCorruption(t *testing.T) {
	sc := storeConfig(t)
	bad := sc
	bad.RuntimeHostID = bad.ControllerHostID
	if s, e := NewDirectoryStore(bad); s != nil || !errors.Is(e, ErrUnsupported) {
		t.Fatal(s, e)
	}
	bad = sc
	bad.SeparationReceiptDigest = ""
	if s, e := NewDirectoryStore(bad); s != nil || !errors.Is(e, ErrUnsupported) {
		t.Fatal(s, e)
	}
	s := openStore(t, sc)
	b := transferBinding()
	data, hash := storedTransfer(t, s, b)
	foreign := b
	foreign.Identity.OwnerID = "someone-else"
	var out bytes.Buffer
	if _, e := s.CopyTransfer(context.Background(), foreign, hash, &out); !errors.Is(e, ErrConflict) || out.Len() != 0 {
		t.Fatal(e, out.Len())
	}
	file := filepath.Join(sc.Directory, "artifact-"+hash+".tar")
	changed := append([]byte(nil), data...)
	at := bytes.Index(changed, []byte("package main"))
	changed[at] = 'X'
	if e := os.WriteFile(file, changed, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.CopyTransfer(context.Background(), b, hash, &out); !errors.Is(e, ErrTransfer) || out.Len() != 0 {
		t.Fatal("corrupt artifact copied", e)
	}
	s.Close()
	bad = sc
	bad.OwnerID = "someone-else"
	if s, e := NewDirectoryStore(bad); s != nil || !errors.Is(e, ErrConflict) {
		t.Fatal(s, e)
	}
}
func TestDirectoryStoreBoundedBudgetRefusesBeforeAllocationAndPreservesPriorObjects(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	sc := storeConfig(t)
	sc.MaxObjects = 1
	s := openStore(t, sc)
	b := transferBinding()
	prior, hash := storedTransfer(t, s, b)
	cfg.Checkpoint = s
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if !errors.Is(e, ErrUnknown) || !r.ResourcesHeld || p.creates != 0 {
		t.Fatal(r, e, p.creates)
	}
	var copy bytes.Buffer
	if _, e = s.CopyTransfer(context.Background(), b, hash, &copy); e != nil || !bytes.Equal(copy.Bytes(), prior) {
		t.Fatal("prior object lost", e)
	}
	if _, e = os.Lstat(filepath.Join(sc.Directory, ".store-next")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("partial object retained", e)
	}
}
func TestDirectoryStoreCancelledAndMalformedTransfersDoNotPublish(t *testing.T) {
	sc := storeConfig(t)
	s := openStore(t, sc)
	b := transferBinding()
	var data bytes.Buffer
	_, hash, e := WriteTransfer(&data, b, []Payload{payload("source/a", "data")})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.PutTransfer(ctx, b, hash, bytes.NewReader(data.Bytes())); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = s.PutTransfer(context.Background(), b, hash, strings.NewReader("invalid")); !errors.Is(e, ErrTransfer) {
		t.Fatal(e)
	}
	if _, e = os.Lstat(filepath.Join(sc.Directory, "artifact-"+hash+".tar")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("failed transfer published", e)
	}
	if _, e = s.Head(context.Background()); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("failed transfer created a head", e)
	}
}
func TestDirectoryStoreReopensOwnedInterruptedTemporaryFileAndRejectsSecondProcessOwner(t *testing.T) {
	sc := storeConfig(t)
	s := openStore(t, sc)
	if other, e := NewDirectoryStore(sc); other != nil || e == nil {
		t.Fatal(other, e)
	}
	s.Close()
	if e := os.WriteFile(filepath.Join(sc.Directory, ".store-next"), []byte("interrupted unacknowledged bytes"), 0600); e != nil {
		t.Fatal(e)
	}
	s = openStore(t, sc)
	if _, e := os.Lstat(filepath.Join(sc.Directory, ".store-next")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("owned partial file remains", e)
	}
}

func TestDirectoryStoreFencesReleasedWriterWithoutReleasingSuccessor(t *testing.T) {
	s := openStore(t, storeConfig(t))
	initial := journal{Version: 1, OwnerID: "owner-a", Scope: "scope-one", Records: map[string]Record{}}
	b, e := json.Marshal(initial)
	if e != nil {
		t.Fatal(e)
	}
	hash := bytesDigest(b)
	claim := CheckpointClaim{OwnerID: "owner-a", Scope: "scope-one", JournalDigest: hash}
	old, e := s.Claim(context.Background(), claim)
	if e != nil {
		t.Fatal(e)
	}
	if ack, e := old.Commit(context.Background(), hash, b); e != nil || ack != hash {
		t.Fatal(ack, e)
	}
	old.Close()
	claim.HasJournal = true
	current, e := s.Claim(context.Background(), claim)
	if e != nil {
		t.Fatal(e)
	}
	defer current.Close()
	if _, e := old.Commit(context.Background(), hash, b); !errors.Is(e, ErrClosed) {
		t.Fatal("released writer published", e)
	}
	old.Close()
	if _, e := s.Claim(context.Background(), claim); !errors.Is(e, ErrConflict) {
		t.Fatal("old Close released successor", e)
	}
	if ack, e := current.Commit(context.Background(), hash, b); e != nil || ack != hash {
		t.Fatal(ack, e)
	}
}
func TestDirectoryStoreByteLimitLeavesNoPublishedPartialTransfer(t *testing.T) {
	sc := storeConfig(t)
	sc.MaxBytes = maxResponse
	s := openStore(t, sc)
	b := transferBinding()
	var data bytes.Buffer
	_, hash, e := WriteTransfer(&data, b, []Payload{payload("source/large", strings.Repeat("a", maxResponse))})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.PutTransfer(context.Background(), b, hash, bytes.NewReader(data.Bytes())); e == nil {
		t.Fatal("storage byte budget ignored")
	}
	entries, e := os.ReadDir(sc.Directory)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "artifact-") || entry.Name() == ".store-next" {
			t.Fatal("partial transfer published", entry.Name())
		}
	}
}
