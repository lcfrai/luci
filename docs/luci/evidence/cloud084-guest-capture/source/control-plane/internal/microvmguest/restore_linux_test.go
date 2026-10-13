//go:build linux

package microvmguest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func protectedCaptureFixture(t *testing.T) (Prepared, FrozenTask) {
	t.Helper()
	p, frozen, _, _ := frozenSourceFixture(t)
	if err := ReserveLaunch(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(p.directory, kernelLaunchReceipt), frozen.identity); err != nil {
		t.Fatal(err)
	}
	return p, frozen
}
func TestGuestCollectorRestoresProtectedOriginalAfterWorkerChanges(t *testing.T) {
	p, frozen := protectedCaptureFixture(t)
	if err := os.WriteFile(filepath.Join(p.directory, "source", "AGENTS.md"), []byte("amended by real upstream tool"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := restoreCaptureInput(context.Background(), p.directory, frozen.identity, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(p.receipt())
	restored, _ := json.Marshal(loaded.receipt())
	if !bytes.Equal(original, restored) {
		t.Fatal("mutated source replaced original baseline")
	}
	if err := verifyPreparedFiles(context.Background(), loaded); err == nil {
		t.Fatal("test failed to modify source")
	}
}
func TestGuestCollectorRefusesUntrustedOrWrongLaunchWithoutMutatingSource(t *testing.T) {
	for _, name := range []string{"wrong-vm", "wrong-pid", "kernel-changed", "input-tampered", "marker-changed", "marker-symlink", "input-hardlink", "marker-readable", "writable-parent", "linked-parent", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			p, frozen := protectedCaptureFixture(t)
			before, err := os.ReadFile(filepath.Join(p.directory, "source", "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			dir := p.directory
			expected := frozen.identity
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "wrong-vm":
				expected.SandboxID = "another-vm"
			case "wrong-pid":
				expected.ProcessPID++
			case "kernel-changed":
				b, _ := json.Marshal(struct {
					Version int `json:"version"`
				}{2})
				if err = os.WriteFile(filepath.Join(dir, kernelLaunchReceipt), b, 0600); err != nil {
					t.Fatal(err)
				}
			case "input-tampered":
				if err = os.WriteFile(filepath.Join(dir, "input.tar"), []byte("not reviewed archive"), 0600); err != nil {
					t.Fatal(err)
				}
			case "marker-changed":
				if err = os.WriteFile(filepath.Join(dir, "launch-reserved.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "marker-symlink":
				target := filepath.Join(dir, "launch-reserved.json")
				if err = os.Rename(target, target+".original"); err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(target+".original", target); err != nil {
					t.Fatal(err)
				}
			case "input-hardlink":
				if err = os.Link(filepath.Join(dir, "input.tar"), filepath.Join(dir, "extra-link")); err != nil {
					t.Fatal(err)
				}
			case "marker-readable":
				if err = os.Chmod(filepath.Join(dir, "prepared.json"), 0644); err != nil {
					t.Fatal(err)
				}
			case "writable-parent":
				parent := filepath.Dir(dir)
				if err = os.Chmod(parent, 0777); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(parent, 0700)
			case "linked-parent":
				link := filepath.Join(t.TempDir(), "parent-link")
				if err = os.Symlink(filepath.Dir(dir), link); err != nil {
					t.Fatal(err)
				}
				dir = filepath.Join(link, filepath.Base(dir))
			case "cancelled":
				cancel()
			}
			_, err = restoreCaptureInput(ctx, dir, expected, uint32(os.Getuid()))
			if err == nil {
				t.Fatal("unsafe collector baseline accepted")
			}
			if name == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(p.directory, "source", "AGENTS.md"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refused collection changed source", err)
			}
			if _, err = os.Lstat(filepath.Join(p.directory, "capture-intent.json")); !os.IsNotExist(err) {
				t.Fatal("restore reserved capture", err)
			}
		})
	}
}
