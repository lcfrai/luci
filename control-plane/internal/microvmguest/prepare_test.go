//go:build linux

package microvmguest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func archive(t *testing.T) ([]byte, e2bbe.TransferBinding, string) {
	t.Helper()
	binding := e2bbe.TransferBinding{Identity: e2bbe.Identity{OwnerID: "owner-one", ProjectID: "project-one", TaskID: "task-one", AttemptID: "attempt-one", OperationID: "operation-one"}, SourceDigest: strings.Repeat("a", 64), ContextDigest: strings.Repeat("b", 64), Direction: "input"}
	var buf bytes.Buffer
	files := []e2bbe.Payload{}
	for _, row := range []struct {
		name, body string
		mode       uint32
	}{{"source/AGENTS.md", "preserved upstream instructions", 0644}, {"source/build.sh", "#!/bin/sh\nprintf 'build'\n", 0755}, {"context/reviewed.json", "{\"revision\":7}", 0644}} {
		body := []byte(row.body)
		digest := sha256.Sum256(body)
		files = append(files, e2bbe.Payload{TransferFile: e2bbe.TransferFile{Path: row.name, Size: int64(len(body)), SHA256: hex.EncodeToString(digest[:]), Mode: row.mode}, Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }})
	}
	_, hash, e := e2bbe.WriteTransfer(&buf, binding, files)
	if e != nil {
		t.Fatal(e)
	}
	return buf.Bytes(), binding, hash
}
func TestGuestPreparePreservesExactInputAndPreventsSecondExecution(t *testing.T) {
	data, binding, digest := archive(t)
	root := filepath.Join(t.TempDir(), "attempt")
	p, e := Prepare(context.Background(), root, bytes.NewReader(data), binding, digest)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"source/AGENTS.md", "source/build.sh", "context/reviewed.json"} {
		if info, e := os.Lstat(filepath.Join(root, name)); e != nil || !info.Mode().IsRegular() {
			t.Fatal(name, e)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "source/AGENTS.md")); string(b) != "preserved upstream instructions" {
		t.Fatal("upstream instructions changed")
	}
	if _, e = os.Stat(filepath.Join(root, "launch-reserved.json")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("Prepare granted launch", e)
	}
	if e = ReserveLaunch(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(root, "launch-reserved.json"))
	if e = ReserveLaunch(context.Background(), p); !errors.Is(e, os.ErrExist) {
		t.Fatal("replayed launch", e)
	}
	if _, e = Prepare(context.Background(), root, bytes.NewReader(data), binding, digest); !errors.Is(e, ErrRefused) {
		t.Fatal("replayed prepare", e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "launch-reserved.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("changed exact launch intent")
	}
}
func TestGuestRejectsChangedBindingOrArchiveBeforeSourceMaterialization(t *testing.T) {
	for _, kind := range []string{"project", "attempt", "context", "direction", "digest", "bytes", "tail"} {
		t.Run(kind, func(t *testing.T) {
			data, binding, digest := archive(t)
			switch kind {
			case "project":
				binding.Identity.ProjectID = "another"
			case "attempt":
				binding.Identity.AttemptID = "another"
			case "context":
				binding.ContextDigest = strings.Repeat("c", 64)
			case "direction":
				binding.Direction = "output"
			case "digest":
				digest = strings.Repeat("d", 64)
			case "bytes":
				at := bytes.Index(data, []byte("preserved upstream"))
				data[at] = 'X'
			case "tail":
				data = append(data, 'X')
			}
			root := filepath.Join(t.TempDir(), "attempt")
			if _, e := Prepare(context.Background(), root, bytes.NewReader(data), binding, digest); e == nil {
				t.Fatal("accepted altered input")
			}
			if _, e := os.Stat(filepath.Join(root, "source")); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("source exposed before verification", e)
			}
			if _, e := os.Stat(filepath.Join(root, "launch-reserved.json")); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("launch granted", e)
			}
		})
	}
}

type cancelReader struct {
	cancel context.CancelFunc
	body   *bytes.Reader
}

func (r cancelReader) Read(b []byte) (int, error) { n, e := r.body.Read(b); r.cancel(); return n, e }
func TestGuestCancellationAndExistingDestinationNeverGrantLaunch(t *testing.T) {
	data, binding, digest := archive(t)
	parent := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(parent, "cancelled")
	if _, e := Prepare(ctx, path, bytes.NewReader(data), binding, digest); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := os.Stat(path); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("cancelled request reserved work")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	path = filepath.Join(parent, "interrupted")
	if _, e := Prepare(ctx, path, cancelReader{cancel, bytes.NewReader(data)}, binding, digest); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(path, "prepared.json")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("published interrupted work", e)
	}
	path = filepath.Join(parent, "existing")
	if e := os.WriteFile(path, []byte("preserve"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Prepare(context.Background(), path, bytes.NewReader(data), binding, digest); !errors.Is(e, ErrRefused) {
		t.Fatal(e)
	}
	if b, _ := os.ReadFile(path); string(b) != "preserve" {
		t.Fatal("overwrote existing content")
	}
}

func TestFabricatedPreparedCannotWriteLaunchOrChangeOwnership(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "preserve")
	if e := os.WriteFile(sentinel, []byte("untouched"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []Prepared{{}, {directory: root}, {directory: "."}} {
		if e := ReserveLaunch(context.Background(), p); !errors.Is(e, ErrRefused) {
			t.Fatal("forged launch accepted", e)
		}
		if e := Protect(p, 1001, 1001); !errors.Is(e, ErrRefused) {
			t.Fatal("forged permission mutation", e)
		}
	}
	if b, _ := os.ReadFile(sentinel); string(b) != "untouched" {
		t.Fatal("sentinel changed")
	}
	if _, e := os.Stat(filepath.Join(root, "launch-reserved.json")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("unowned receipt created", e)
	}
}

func TestGuestLaunchRefusesLostChangedOrLinkedPreparedInput(t *testing.T) {
	for _, kind := range []string{"missing-source", "changed-source", "changed-context", "linked-parent", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			data, binding, digest := archive(t)
			parent := t.TempDir()
			p, err := Prepare(context.Background(), filepath.Join(parent, "attempt"), bytes.NewReader(data), binding, digest)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "missing-source":
				err = os.Remove(filepath.Join(p.Directory(), "source", "AGENTS.md"))
			case "changed-source":
				err = os.WriteFile(filepath.Join(p.Directory(), "source", "AGENTS.md"), []byte("not the reviewed source"), 0600)
			case "changed-context":
				err = os.WriteFile(filepath.Join(p.Directory(), "context", "reviewed.json"), []byte(`{"revision":8}`), 0600)
			case "linked-parent":
				outside := filepath.Join(parent, "original-context")
				if err = os.Rename(filepath.Join(p.Directory(), "context"), outside); err == nil {
					err = os.Symlink(outside, filepath.Join(p.Directory(), "context"))
				}
			case "cancelled":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := ReserveLaunch(ctx, p); err == nil {
				t.Fatal("changed prepared input granted launch")
			}
			if _, err := os.Stat(filepath.Join(p.Directory(), "launch-reserved.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("launch marker was written", err)
			}
		})
	}
}
