//go:build windows

package management

// File-integrity tests only: toy bytes are never loaded or presented as an
// actual model, and no provider/process/Job/GPU operation occurs.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagedModelContentPinsDigestClosureUntilCheckedStop(t *testing.T) {
	root := t.TempDir()
	if e := os.MkdirAll(filepath.Join(root, "blobs"), 0700); e != nil {
		t.Fatal(e)
	}
	blob := func(raw []byte) managedModelDescriptor {
		sum := sha256.Sum256(raw)
		hash := hex.EncodeToString(sum[:])
		if e := os.WriteFile(filepath.Join(root, "blobs", "sha256-"+hash), raw, 0600); e != nil {
			t.Fatal(e)
		}
		return managedModelDescriptor{Digest: "sha256:" + hash, Size: int64(len(raw))}
	}
	config := blob([]byte(`{"test":true}`))
	weights := blob([]byte("inert integrity-test bytes"))
	manifest := managedModelManifest{Version: 2, Config: config, Layers: []managedModelDescriptor{weights}}
	raw, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(root, "manifests-v2", "ollama.com", "library", "test", "latest")
	if e := os.MkdirAll(filepath.Dir(manifestPath), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(manifestPath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pins, digest, e := pinManagedModelContent(ctx, root, "test:latest")
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	if digest != hex.EncodeToString(sum[:]) || len(pins) != 3 {
		t.Fatal("manifest identity or full content closure lost")
	}
	path := filepath.Join(root, "blobs", "sha256-"+weights.Digest[len("sha256:"):])
	if e = os.WriteFile(path, []byte("tamper"), 0600); e == nil {
		t.Fatal("held model content remained writable")
	}
	service := &windowsManagedModelService{content: pins}
	if e = service.stop(ctx); e != nil {
		t.Fatal("original content handles not closed", e)
	}
	if e = os.WriteFile(path, []byte("tamper"), 0600); e != nil {
		t.Fatal(e)
	}
	bad, _, e := pinManagedModelContent(ctx, root, "test:latest")
	if e == nil {
		t.Fatal("content digest mutation was accepted")
	}
	if e = (&windowsManagedModelService{content: bad}).stop(ctx); e != nil {
		t.Fatal("failed content read lost its original handles", e)
	}
}
