//go:build linux

package microvmguest

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func frozenSourceFixture(t *testing.T) (Prepared, FrozenTask, *controlledTaskKernel, string) {
	t.Helper()
	raw, binding, hash := archive(t)
	p, e := Prepare(context.Background(), filepath.Join(t.TempDir(), "attempt"), bytes.NewReader(raw), binding, hash)
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range p.manifest.Files {
		if strings.HasPrefix(file.Path, "source/") {
			if e = os.Chmod(filepath.Join(p.directory, file.Path), os.FileMode(file.Mode)); e != nil {
				t.Fatal(e)
			}
		}
	}
	id, k := taskKernelFixture()
	id.Identity = binding.Identity
	id.InputBundleDigest = hash
	id.SourceDigest = binding.SourceDigest
	id.ContextDigest = binding.ContextDigest
	id.CgroupRelativePath = "luci-tasks/" + binding.Identity.OperationID
	k.target = "/sys/fs/cgroup/" + id.CgroupRelativePath
	k.events = []string{"populated 1\nfrozen 1"}
	frozen, e := freezeTaskGroup(context.Background(), id, k)
	if e != nil {
		t.Fatal(e)
	}
	return p, frozen, k, t.TempDir()
}
func TestGuestCaptureExportsCompleteSourceChangesWithoutRunningGit(t *testing.T) {
	p, frozen, k, dest := frozenSourceFixture(t)
	source := filepath.Join(p.directory, "source")
	writes := map[string]string{"AGENTS.md": "amended project guidance", "app.txt": "changed implementation", ".gitignore": "AGENTS.md\napp.txt\n", "node_modules/large/secret": "not application source", ".env": "test-only-never-export", ".git/config": "untrusted hooks and remotes never executed"}
	for name, body := range writes {
		target := filepath.Join(source, name)
		if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(target, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Remove(filepath.Join(source, "build.sh")); e != nil {
		t.Fatal(e)
	}
	receipt, e := captureTaskOutput(context.Background(), p, frozen, dest, uint32(os.Getuid()))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(dest, "output.tar"))
	if e != nil {
		t.Fatal(e)
	}
	binding := p.manifest.Binding
	binding.Direction = "output"
	manifest, e := e2bbe.VerifyTransfer(bytes.NewReader(raw), binding, receipt.ArchiveSHA256)
	if e != nil || receipt.ArchiveBytes != int64(len(raw)) {
		t.Fatal(receipt, e)
	}
	files := map[string][]byte{}
	reader := tar.NewReader(bytes.NewReader(raw))
	for {
		header, e := reader.Next()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		files[header.Name], e = io.ReadAll(reader)
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(manifest.Files) != 5 || string(files["output/source/AGENTS.md"]) != "amended project guidance" || string(files["output/source/app.txt"]) != "changed implementation" {
		t.Fatal("modified ignores hid required source", manifest)
	}
	for _, name := range []string{"output/source/build.sh", "output/source/.env", "output/source/node_modules/large/secret", "output/source/.git/config"} {
		if _, ok := files[name]; ok {
			t.Fatal("deleted/dependency/credential/Git path returned", name)
		}
	}
	if hashBytes(files["output/source-manifest.json"]) != receipt.SourceManifestSHA256 || hashBytes(files["output/changes.json"]) != receipt.ChangeManifestSHA256 {
		t.Fatal("metadata not bound to exact bytes")
	}
	var changes changeSnapshot
	if e = json.Unmarshal(files["output/changes.json"], &changes); e != nil {
		t.Fatal(e)
	}
	observed := map[string]string{}
	for _, change := range changes.Changes {
		observed[change.Path] = change.Kind
	}
	expected := map[string]string{"AGENTS.md": "modified", "build.sh": "deleted", "app.txt": "added", ".gitignore": "added"}
	if !reflect.DeepEqual(observed, expected) {
		t.Fatal(observed)
	}
	before := append([]byte{}, raw...)
	if _, e = captureTaskOutput(context.Background(), p, frozen, dest, uint32(os.Getuid())); e == nil {
		t.Fatal("exclusive capture intent replayed")
	}
	after, _ := os.ReadFile(filepath.Join(dest, "output.tar"))
	if !bytes.Equal(before, after) || k.requested != "1" || k.writes != 1 {
		t.Fatal("replay changed output or unfroze task")
	}
}
func TestGuestCaptureRefusesLinkedSpecialOversizedOrChangedIdentitySource(t *testing.T) {
	for _, name := range []string{"symlink", "hardlink", "fifo", "oversized", "different-attempt", "source-root-link"} {
		t.Run(name, func(t *testing.T) {
			p, frozen, _, dest := frozenSourceFixture(t)
			source := filepath.Join(p.directory, "source")
			outside := filepath.Join(t.TempDir(), "foreign")
			if e := os.WriteFile(outside, []byte("preserve foreign bytes"), 0600); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "symlink":
				if e := os.Symlink(outside, filepath.Join(source, "linked")); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(outside, filepath.Join(source, "linked")); e != nil {
					t.Fatal(e)
				}
			case "fifo":
				if e := syscall.Mkfifo(filepath.Join(source, "pipe"), 0600); e != nil {
					t.Fatal(e)
				}
			case "oversized":
				f, e := os.Create(filepath.Join(source, "big"))
				if e != nil {
					t.Fatal(e)
				}
				if e = f.Truncate(e2bbe.MaxTransferBytes); e != nil {
					t.Fatal(e)
				}
				f.Close()
			case "different-attempt":
				frozen.identity.Identity.AttemptID = "other"
			case "source-root-link":
				if e := os.Rename(source, source+"-retained"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(filepath.Dir(outside), source); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := captureTaskOutput(context.Background(), p, frozen, dest, uint32(os.Getuid())); e == nil {
				t.Fatal("unsafe/incomplete source exported")
			}
			if _, e := os.Stat(filepath.Join(dest, "output.json")); !os.IsNotExist(e) {
				t.Fatal("failed capture published success metadata", e)
			}
			bytes, _ := os.ReadFile(outside)
			if string(bytes) != "preserve foreign bytes" {
				t.Fatal("foreign source changed")
			}
		})
	}
}
func TestGuestCaptureLossOfFreezeRetainsUnconfirmedIntentWithoutReceipt(t *testing.T) {
	p, frozen, k, dest := frozenSourceFixture(t)
	k.afterEvents = func() {
		if info, e := os.Stat(filepath.Join(dest, "output.tar")); e == nil && info.Size() > 0 {
			k.events = []string{"populated 1\nfrozen 0"}
		}
	}
	if _, e := captureTaskOutput(context.Background(), p, frozen, dest, uint32(os.Getuid())); e == nil {
		t.Fatal("capture continued after freeze disappeared")
	}
	if _, e := os.Stat(filepath.Join(p.directory, "capture-intent.json")); e != nil {
		t.Fatal("failure lost reservation", e)
	}
	if _, e := os.Stat(filepath.Join(dest, "output.json")); !os.IsNotExist(e) {
		t.Fatal("unknown freeze published receipt")
	}
	if k.writes != 1 {
		t.Fatal("failed capture replayed or unfroze")
	}
}

func TestGuestCaptureExcludesNewGeneratedSiblingsButKeepsOriginalExcludedTree(t *testing.T) {
	_, binding, _ := archive(t)
	payloads := []e2bbe.Payload{metadataPayload("source/.cache/kept.txt", []byte("original reviewed source")), metadataPayload("source/AGENTS.md", []byte("original instructions")), metadataPayload("context/reviewed.json", []byte(`{"revision":7}`))}
	var data bytes.Buffer
	_, hash, err := e2bbe.WriteTransfer(&data, binding, payloads)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(context.Background(), filepath.Join(t.TempDir(), "attempt"), bytes.NewReader(data.Bytes()), binding, hash)
	if err != nil {
		t.Fatal(err)
	}
	id, k := taskKernelFixture()
	id.Identity = binding.Identity
	id.InputBundleDigest = hash
	id.SourceDigest = binding.SourceDigest
	id.ContextDigest = binding.ContextDigest
	id.CgroupRelativePath = "luci-tasks/" + binding.Identity.OperationID
	k.target = "/sys/fs/cgroup/" + id.CgroupRelativePath
	k.events = []string{"populated 1\nfrozen 1"}
	frozen, err := freezeTaskGroup(context.Background(), id, k)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(p.directory, "source")
	for name, body := range map[string]string{".cache/kept.txt": "changed reviewed source", ".cache/generated.bin": "new generated content"} {
		if err = os.WriteFile(filepath.Join(source, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, changes, _, err := snapshotTaskSource(context.Background(), p, frozen, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 2 || snapshot.Files[0].Path != ".cache/kept.txt" || snapshot.Files[0].SHA256 != hashBytes([]byte("changed reviewed source")) {
		t.Fatal("original tracked source or fixed new-path exclusions lost", snapshot.Files)
	}
	if len(changes.Changes) != 1 || changes.Changes[0].Path != ".cache/kept.txt" || changes.Changes[0].Kind != "modified" {
		t.Fatal(changes.Changes)
	}
	if len(changes.Exclusions) != 1 || changes.Exclusions[0].Path != ".cache/generated.bin" || changes.Exclusions[0].Reason != "fixed-new-path-exclusion" {
		t.Fatal(changes.Exclusions)
	}
}
