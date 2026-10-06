package api

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// Codex 0.160.0 creates these aliases in its normal mounted CODEX_HOME. A
// stopped real installed home reproduced snapshot failures for both resume and
// clean restart; the temporary executables are not conversation or auth state.
func nativeCodexTransientAliases(t *testing.T, source string) {
	t.Helper()
	transient := filepath.Join(source, "tmp", "arg0", "codex-arg0-fixture")
	if err := os.MkdirAll(transient, 0700); err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, transient, ".lock", "")
	for _, name := range []string{"apply_patch", "applypatch", "codex-execve-wrapper", "codex-linux-sandbox"} {
		if err := os.Symlink("/sandbox-only/executable", filepath.Join(transient, name)); err != nil {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
	}
}

func TestCodexNativeSnapshotOmitsTransientAliasesAndPreservesIndependentDurableState(t *testing.T) {
	for _, credentialsOnly := range []bool{false, true} {
		name := "resume"
		if credentialsOnly {
			name = "restart-credentials"
		}
		t.Run(name, func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			durable := map[string][]byte{
				"auth.json":   []byte(`{"tokens":{"access_token":"fixture-only"}}`),
				"config.toml": []byte("model = \"fixture-model\"\n"),
				"sessions/2026/10/06/rollout-original.jsonl": []byte("original provider-owned conversation\n"),
				"state_5.sqlite":              append([]byte("SQLite format 3\x00"), 0, 1, 255),
				"state_5.sqlite-wal":          {0, 255, 1, 2, 3},
				"state_5.sqlite-shm":          {4, 5, 0, 255},
				"thread_history_1.sqlite":     []byte("provider-owned database bytes"),
				"thread_history_1.sqlite-wal": []byte("uncheckpointed provider bytes"),
			}
			for path, data := range durable {
				nativeWrite(t, source, path, string(data))
			}
			nativeCodexTransientAliases(t, source)
			if err := copyNativeState(source, target, "codex", credentialsOnly); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(target, "tmp")); !os.IsNotExist(err) {
				t.Fatal("transient alias tree copied", err)
			}
			if _, err := os.Lstat(filepath.Join(source, "tmp/arg0/codex-arg0-fixture/apply_patch")); err != nil {
				t.Fatal("source transient state mutated", err)
			}
			for path, want := range durable {
				got, err := os.ReadFile(filepath.Join(target, path))
				if credentialsOnly && path != "auth.json" {
					if !os.IsNotExist(err) {
						t.Fatal("clean restart retained conversation state", path, err)
					}
					continue
				}
				if err != nil || !bytes.Equal(got, want) {
					t.Fatal("durable bytes changed or were lost", path, err)
				}
				before, _ := os.Stat(filepath.Join(source, path))
				after, _ := os.Stat(filepath.Join(target, path))
				if os.SameFile(before, after) {
					t.Fatal("copy shared deletable ownership", path)
				}
			}
			nativeWrite(t, target, "auth.json", "changed only in resumed home")
			original, err := os.ReadFile(filepath.Join(source, "auth.json"))
			if err != nil || !bytes.Equal(original, durable["auth.json"]) {
				t.Fatal("resumed auth changed original", err)
			}
			if err := os.RemoveAll(source); err != nil {
				t.Fatal(err)
			}
			if got, err := os.ReadFile(filepath.Join(target, "auth.json")); err != nil || string(got) != "changed only in resumed home" {
				t.Fatal("source removal lost independent copy", err)
			}
		})
	}
}

func TestCodexNativeSnapshotNeverFollowsTransientRootLink(t *testing.T) {
	for _, credentialsOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "resume", true: "restart-credentials"}[credentialsOnly], func(t *testing.T) {
			source, target, outside := t.TempDir(), t.TempDir(), t.TempDir()
			nativeWrite(t, source, "auth.json", "retained auth")
			nativeWrite(t, outside, "private.txt", "outside must remain untouched")
			// A second dangling link would fail a walked target. No target is opened.
			if err := os.Symlink(filepath.Join(outside, "absent"), filepath.Join(outside, "dangling")); err != nil {
				t.Skipf("symlink fixture unavailable: %v", err)
			}
			if err := os.Symlink(outside, filepath.Join(source, "tmp")); err != nil {
				t.Skipf("symlink fixture unavailable: %v", err)
			}
			if err := copyNativeState(source, target, "codex", credentialsOnly); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(target, "tmp")); !os.IsNotExist(err) {
				t.Fatal("temporary root link adopted", err)
			}
			if got, err := os.ReadFile(filepath.Join(outside, "private.txt")); err != nil || string(got) != "outside must remain untouched" {
				t.Fatal("outside changed", err)
			}
			if got, err := os.ReadFile(filepath.Join(target, "auth.json")); err != nil || string(got) != "retained auth" {
				t.Fatal("auth lost", err)
			}
		})
	}
}

func TestNativeSnapshotExclusionIsCodexOnlyAndNeverCoversAuthOrHistory(t *testing.T) {
	for _, provider := range []string{"codex", "opencode"} {
		for _, credentialsOnly := range []bool{false, true} {
			for _, path := range []string{"auth.json", "sessions/2026/rollout.jsonl", "sessions/tmp/alias", "cache/alias"} {
				for _, kind := range []string{"symlink", "hardlink"} {
					t.Run(provider+"/"+map[bool]string{false: "resume", true: "restart"}[credentialsOnly]+"/"+kind+"/"+path, func(t *testing.T) {
						source, target, outside := t.TempDir(), t.TempDir(), t.TempDir()
						nativeWrite(t, outside, "private.txt", "never copy this")
						dest := filepath.Join(source, path)
						if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
							t.Fatal(err)
						}
						link := os.Symlink
						if kind == "hardlink" {
							link = os.Link
						}
						if err := link(filepath.Join(outside, "private.txt"), dest); err != nil {
							t.Skipf("link fixture unavailable: %v", err)
						}
						err := copyNativeState(source, target, provider, credentialsOnly)
						if err == nil {
							t.Fatal("unsafe durable entry accepted")
						}
						wantCode := "native_state_link"
						if kind == "hardlink" {
							wantCode = "native_state_hardlink"
							if !errors.Is(err, hostpath.ErrHardLinked) {
								t.Fatal("hard-link cause lost", err)
							}
						}
						nativeSnapshotDiagnosticCode(t, err, wantCode)
						if _, err := os.Lstat(filepath.Join(target, path)); !os.IsNotExist(err) {
							t.Fatal("unsafe entry copied", err)
						}
					})
				}
			}
		}
	}
	t.Run("other-provider-tmp-is-durable", func(t *testing.T) {
		source, target := t.TempDir(), t.TempDir()
		nativeWrite(t, source, "tmp/provider-record", "other provider durable bytes")
		if err := copyNativeState(source, target, "opencode", false); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(target, "tmp/provider-record")); err != nil || string(got) != "other provider durable bytes" {
			t.Fatal("excluded another provider's data", err)
		}
	})
	t.Run("other-provider-aliases-refuse", func(t *testing.T) {
		source, target := t.TempDir(), t.TempDir()
		nativeCodexTransientAliases(t, source)
		if err := copyNativeState(source, target, "opencode", false); err == nil {
			t.Fatal("Codex exception leaked into another provider")
		}
	})
	t.Run("special-durable-entry-refuses", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix socket fixture")
		}
		source, target := t.TempDir(), t.TempDir()
		listener, err := net.Listen("unix", filepath.Join(source, "provider.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		nativeSnapshotDiagnosticCode(t, copyNativeState(source, target, "codex", false), "native_state_special")
	})
}

func nativeSnapshotDiagnosticCode(t *testing.T, err error, want string) {
	t.Helper()
	var classified interface{ DiagnosticCode() string }
	if !errors.As(err, &classified) || classified.DiagnosticCode() != want {
		t.Fatalf("native failure classification: got %T, want %s", err, want)
	}
}

func TestNativeSnapshotClassifiesIOAndSizeRefusalsWithoutLosingCause(t *testing.T) {
	t.Run("missing-source", func(t *testing.T) {
		err := copyNativeState(filepath.Join(t.TempDir(), "absent"), t.TempDir(), "codex", false)
		nativeSnapshotDiagnosticCode(t, err, "native_state_io")
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal("filesystem cause lost", err)
		}
	})
	t.Run("oversized-source", func(t *testing.T) {
		source, target := t.TempDir(), t.TempDir()
		file, err := os.Create(filepath.Join(source, "provider.sqlite"))
		if err != nil {
			t.Fatal(err)
		}
		err = file.Truncate(256<<20 + 1)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		nativeSnapshotDiagnosticCode(t, copyNativeState(source, target, "codex", false), "native_state_limit")
		if _, err := os.Stat(filepath.Join(target, "provider.sqlite")); !os.IsNotExist(err) {
			t.Fatal("oversized source partially copied", err)
		}
	})
}
