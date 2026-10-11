package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

// StoreConfig describes storage on the Luci controller host. Distinct host IDs
// and a receipt are required configuration evidence, not a network topology
// proof. Wiring must verify this disk is outside the replaceable Embed host.
type StoreConfig struct {
	Directory               string
	OwnerID                 string
	RuntimeScope            string
	ControllerHostID        string
	RuntimeHostID           string
	SeparationReceiptDigest string
	MaxBytes                int64
	MaxObjects              int
}
type storeIdentity struct {
	Version                 int    `json:"version"`
	OwnerID                 string `json:"ownerId"`
	Scope                   string `json:"scope"`
	ControllerHostID        string `json:"controllerHostId"`
	RuntimeHostID           string `json:"runtimeHostId"`
	SeparationReceiptDigest string `json:"separationReceiptDigest"`
}
type storeHead struct {
	Version       int    `json:"version"`
	JournalDigest string `json:"journalDigest"`
}
type DirectoryStore struct {
	mu       sync.Mutex
	config   StoreConfig
	identity storeIdentity
	root     *os.Root
	lease    *os.File
	bytes    int64
	objects  int
	closed   bool
	writer   bool
	epoch    uint64
}

func NewDirectoryStore(cfg StoreConfig) (*DirectoryStore, error) {
	if !validID(cfg.OwnerID) || !validID(cfg.RuntimeScope) || !validID(cfg.ControllerHostID) || !validID(cfg.RuntimeHostID) || cfg.ControllerHostID == cfg.RuntimeHostID || !digestPattern.MatchString(cfg.SeparationReceiptDigest) || cfg.MaxBytes < maxResponse || cfg.MaxBytes > 10<<30 || cfg.MaxObjects < 1 || cfg.MaxObjects > 65536 {
		return nil, ErrUnsupported
	}
	lease, e := statelease.Acquire(cfg.Directory, "e2b-artifacts-v1")
	if e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(cfg.Directory)
	if e != nil {
		lease.Close()
		return nil, e
	}
	s := &DirectoryStore{config: cfg, root: root, lease: lease, identity: storeIdentity{1, cfg.OwnerID, cfg.RuntimeScope, cfg.ControllerHostID, cfg.RuntimeHostID, cfg.SeparationReceiptDigest}}
	fail := func(e error) (*DirectoryStore, error) { root.Close(); lease.Close(); return nil, e }
	if i, err := root.Lstat(".store-next"); err == nil {
		if !i.Mode().IsRegular() {
			return fail(ErrConflict)
		}
		if err = root.Remove(".store-next"); err != nil {
			return fail(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	metadata, e := s.readRegular("store.json", 4096)
	if errors.Is(e, os.ErrNotExist) {
		b, _ := json.Marshal(s.identity)
		if e = s.writeAtomic(context.Background(), "store.json", b); e != nil {
			return fail(e)
		}
	} else if e != nil {
		return fail(e)
	} else {
		var got storeIdentity
		if decodeStrict(metadata, &got) != nil || got != s.identity {
			return fail(ErrConflict)
		}
	}
	dir, e := root.Open(".")
	if e != nil {
		return fail(e)
	}
	defer dir.Close()
	entries, e := dir.ReadDir(cfg.MaxObjects + 5)
	if e != nil && e != io.EOF {
		return fail(e)
	}
	if len(entries) > cfg.MaxObjects+4 {
		return fail(ErrCapacity)
	}
	for _, entry := range entries {
		n := entry.Name()
		if n == statelease.FileName || n == "store.json" || n == "head.json" {
			continue
		}
		if n == ".store-next" {
			i, e := root.Lstat(n)
			if e != nil || !i.Mode().IsRegular() {
				return fail(ErrConflict)
			}
			if e = root.Remove(n); e != nil {
				return fail(e)
			}
			continue
		}
		if !validObjectName(n) {
			return fail(ErrConflict)
		}
		i, e := root.Lstat(n)
		if e != nil || !i.Mode().IsRegular() || (runtime.GOOS != "windows" && i.Mode().Perm()&0077 != 0) {
			return fail(ErrConflict)
		}
		s.bytes += i.Size()
		s.objects++
	}
	if s.bytes > cfg.MaxBytes || s.objects > cfg.MaxObjects {
		return fail(ErrCapacity)
	}
	return s, nil
}
func validObjectName(n string) bool {
	switch {
	case strings.HasPrefix(n, "journal-") && strings.HasSuffix(n, ".json"):
		return digestPattern.MatchString(strings.TrimSuffix(strings.TrimPrefix(n, "journal-"), ".json"))
	case strings.HasPrefix(n, "artifact-") && strings.HasSuffix(n, ".tar"):
		return digestPattern.MatchString(strings.TrimSuffix(strings.TrimPrefix(n, "artifact-"), ".tar"))
	}
	return false
}
func decodeStrict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return ErrConflict
	}
	return nil
}
func (s *DirectoryStore) readRegular(name string, limit int64) ([]byte, error) {
	i, e := s.root.Lstat(name)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > limit || (runtime.GOOS != "windows" && i.Mode().Perm()&0077 != 0) {
		return nil, ErrConflict
	}
	f, e := s.root.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, ErrCapacity
	}
	return b, nil
}
func (s *DirectoryStore) syncDir() error {
	f, e := s.root.Open(".")
	if e != nil {
		return e
	}
	defer f.Close()
	e = f.Sync()
	if runtime.GOOS == "windows" {
		return nil
	}
	return e
}
func (s *DirectoryStore) writeAtomic(ctx context.Context, name string, b []byte) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	f, e := s.root.OpenFile(".store-next", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer s.root.Remove(".store-next")
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = s.root.Rename(".store-next", name); e != nil {
		return e
	}
	return s.syncDir()
}
func (s *DirectoryStore) checkJournal(b []byte, hash string) error {
	if len(b) > maxResponse || !digestPattern.MatchString(hash) || bytesDigest(b) != hash {
		return ErrConflict
	}
	_, e := validateJournal(b, s.config.OwnerID, s.config.RuntimeScope)
	return e
}
func (s *DirectoryStore) commit(ctx context.Context, epoch uint64, hash string, b []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.writer || epoch != s.epoch {
		return "", ErrClosed
	}
	if e := ctx.Err(); e != nil {
		return "", e
	}
	if e := s.checkJournal(b, hash); e != nil {
		return "", e
	}
	name := "journal-" + hash + ".json"
	existing, e := s.readRegular(name, maxResponse)
	if e == nil {
		if !bytes.Equal(existing, b) {
			return "", ErrConflict
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return "", e
	} else {
		if s.objects >= s.config.MaxObjects || int64(len(b)) > s.config.MaxBytes-s.bytes {
			return "", ErrCapacity
		}
		if e = s.writeAtomic(ctx, name, b); e != nil {
			return "", e
		}
		s.objects++
		s.bytes += int64(len(b))
	}
	// Exact readback precedes the durable head and acknowledgement.
	read, e := s.readRegular(name, maxResponse)
	if e != nil || bytesDigest(read) != hash {
		return "", ErrUnknown
	}
	head, _ := json.Marshal(storeHead{1, hash})
	if e = s.writeAtomic(ctx, "head.json", head); e != nil {
		return "", e
	}
	return hash, nil
}
func (s *DirectoryStore) ReadCheckpoint(ctx context.Context, expectedHash string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if !digestPattern.MatchString(expectedHash) {
		return nil, ErrConflict
	}
	b, e := s.readRegular("journal-"+expectedHash+".json", maxResponse)
	if e != nil {
		return nil, e
	}
	if e = s.checkJournal(b, expectedHash); e != nil {
		return nil, e
	}
	return b, nil
}
func (s *DirectoryStore) Head(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", ErrClosed
	}
	if e := ctx.Err(); e != nil {
		return "", e
	}
	b, e := s.readRegular("head.json", 4096)
	if e != nil {
		return "", e
	}
	var h storeHead
	if decodeStrict(b, &h) != nil || h.Version != 1 || !digestPattern.MatchString(h.JournalDigest) {
		return "", ErrConflict
	}
	data, e := s.readRegular("journal-"+h.JournalDigest+".json", maxResponse)
	if e != nil || s.checkJournal(data, h.JournalDigest) != nil {
		return "", ErrConflict
	}
	return h.JournalDigest, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}

type boundedWriter struct {
	w       io.Writer
	left, n int64
}

func (w *boundedWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.left {
		return 0, ErrCapacity
	}
	n, e := w.w.Write(b)
	w.left -= int64(n)
	w.n += int64(n)
	return n, e
}

// PutTransfer stores already scoped input/output data, without extracting or
// executing it. HTTP/file readers supplied by wiring must have their own I/O
// deadlines; a context cannot preempt an arbitrary blocking Reader.
func (s *DirectoryStore) PutTransfer(ctx context.Context, binding TransferBinding, hash string, r io.Reader) (TransferManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m TransferManifest
	if s.closed {
		return m, ErrClosed
	}
	if binding.Identity.OwnerID != s.config.OwnerID || !validBinding(binding) || !digestPattern.MatchString(hash) {
		return m, ErrConflict
	}
	if e := ctx.Err(); e != nil {
		return m, e
	}
	name := "artifact-" + hash + ".tar"
	if _, e := s.root.Lstat(name); e == nil {
		if _, e = VerifyTransfer(contextReader{ctx, r}, binding, hash); e != nil {
			return m, e
		}
		return s.verifyTransfer(ctx, name, binding, hash)
	} else if !errors.Is(e, os.ErrNotExist) {
		return m, e
	}
	if s.objects >= s.config.MaxObjects {
		return m, ErrCapacity
	}
	f, e := s.root.OpenFile(".store-next", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return m, e
	}
	defer s.root.Remove(".store-next")
	writer := &boundedWriter{w: f, left: s.config.MaxBytes - s.bytes}
	m, e = VerifyTransfer(io.TeeReader(contextReader{ctx, r}, writer), binding, hash)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return m, e
	}
	if e = ctx.Err(); e != nil {
		return m, e
	}
	if e = s.root.Rename(".store-next", name); e != nil {
		return m, e
	}
	s.objects++
	s.bytes += writer.n
	if e = s.syncDir(); e != nil {
		return m, e
	}
	return s.verifyTransfer(ctx, name, binding, hash)
}
func (s *DirectoryStore) verifyTransfer(ctx context.Context, name string, binding TransferBinding, hash string) (TransferManifest, error) {
	var m TransferManifest
	i, e := s.root.Lstat(name)
	if e != nil {
		return m, e
	}
	if !i.Mode().IsRegular() || (runtime.GOOS != "windows" && i.Mode().Perm()&0077 != 0) {
		return m, ErrConflict
	}
	f, e := s.root.Open(name)
	if e != nil {
		return m, e
	}
	defer f.Close()
	return VerifyTransfer(contextReader{ctx, f}, binding, hash)
}

// CopyTransfer writes only to a caller-owned staging destination. The caller
// must discard partial output on error and independently verify before use.
func (s *DirectoryStore) CopyTransfer(ctx context.Context, binding TransferBinding, hash string, w io.Writer) (TransferManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m TransferManifest
	if s.closed {
		return m, ErrClosed
	}
	if binding.Identity.OwnerID != s.config.OwnerID || !validBinding(binding) || !digestPattern.MatchString(hash) {
		return m, ErrConflict
	}
	name := "artifact-" + hash + ".tar"
	m, e := s.verifyTransfer(ctx, name, binding, hash)
	if e != nil {
		return m, e
	}
	f, e := s.root.Open(name)
	if e != nil {
		return m, e
	}
	defer f.Close()
	return VerifyTransfer(io.TeeReader(contextReader{ctx, f}, w), binding, hash)
}
func (s *DirectoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if s.writer {
		return ErrConflict
	}
	s.closed = true
	first := s.root.Close()
	second := s.lease.Close()
	if first != nil {
		return first
	}
	return second
}

// Claim binds an exact current journal to one fenced live writer. The process
// statelease also excludes another Store instance. Old writer handles cannot
// publish or release ownership after a newer claim.
func (s *DirectoryStore) Claim(ctx context.Context, claim CheckpointClaim) (CheckpointWriter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if s.writer {
		return nil, ErrConflict
	}
	if claim.OwnerID != s.config.OwnerID || claim.Scope != s.config.RuntimeScope || !digestPattern.MatchString(claim.JournalDigest) {
		return nil, ErrConflict
	}
	head, e := s.readRegular("head.json", 4096)
	if errors.Is(e, os.ErrNotExist) {
		if claim.HasJournal {
			return nil, ErrConflict
		}
	} else if e != nil {
		return nil, e
	} else {
		var h storeHead
		if decodeStrict(head, &h) != nil || h.Version != 1 || h.JournalDigest != claim.JournalDigest || !claim.HasJournal {
			return nil, ErrConflict
		}
		b, e := s.readRegular("journal-"+h.JournalDigest+".json", maxResponse)
		if e != nil || s.checkJournal(b, h.JournalDigest) != nil {
			return nil, ErrConflict
		}
	}
	s.writer = true
	s.epoch++
	return &directoryWriter{store: s, epoch: s.epoch}, nil
}

type directoryWriter struct {
	store *DirectoryStore
	epoch uint64
}

func (w *directoryWriter) Commit(ctx context.Context, hash string, b []byte) (string, error) {
	return w.store.commit(ctx, w.epoch, hash, b)
}
func (w *directoryWriter) Close() error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if w.store.epoch == w.epoch {
		w.store.writer = false
	}
	return nil
}

// RestoreController requires the caller's exact reviewed current head and a
// fresh destination. It refuses stale snapshots, existing journals, another
// live writer and owner/scope mismatch. Changed runtime generation remains
// uncertain in the restored controller; this never replays a Create.
func (s *DirectoryStore) RestoreController(ctx context.Context, cfg Config, expectedHash string) (*Controller, error) {
	if cfg.Checkpoint != s || cfg.OwnerID != s.config.OwnerID || cfg.Deployment.Scope != s.config.RuntimeScope {
		return nil, ErrConflict
	}
	head, e := s.Head(ctx)
	if e != nil {
		return nil, e
	}
	if head != expectedHash {
		return nil, ErrConflict
	}
	b, e := s.ReadCheckpoint(ctx, expectedHash)
	if e != nil {
		return nil, e
	}
	return newController(cfg, b, ctx)
}
