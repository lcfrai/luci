package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

func flipCase(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		return unicode.ToUpper(r)
	}, s)
}

// mkdirs makes each path, and returns the first.
func mkdirs(t *testing.T, paths ...string) string {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return paths[0]
}

func TestCanon(t *testing.T) {
	base := t.TempDir()
	root := mkdirs(t, filepath.Join(base, "repo"), filepath.Join(base, "repo", "desktop"), filepath.Join(base, "repo-fork", "x"))

	t.Run("case", func(t *testing.T) {
		got := samePath(root, flipCase(root))
		if want := runtime.GOOS == "windows"; got != want {
			t.Fatalf("samePath(%q, case-flipped) = %v, want %v", root, got, want)
		}
	})
	t.Run("a sibling with the root as its prefix is outside", func(t *testing.T) {
		if insideRoot(root, filepath.Join(base, "repo-fork", "x")) {
			t.Fatal(`insideRoot("<r>", "<r>-fork/x") = true`)
		}
		if !insideRoot(root, filepath.Join(root, "desktop")) || !insideRoot(root, root) {
			t.Fatal("insideRoot misses the root or a child of it")
		}
	})
	// Carried RR2-M1: on every platform, not Windows alone.
	t.Run("a missing path is an error", func(t *testing.T) {
		if c, err := canon(filepath.Join(base, "gone")); err == nil {
			t.Fatalf("canon of a missing path = %q, nil error", c)
		}
		if insideRoot(root, filepath.Join(root, "gone")) {
			t.Fatal("insideRoot of a missing child = true")
		}
	})
	t.Run("an 8.3 root", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("8.3 short names are Windows only")
		}
		short := os.TempDir()
		if !strings.Contains(short, "~") {
			t.Skipf("TEMP %q has no short element on this machine", short)
		}
		long, err := canon(short)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(long, "~") {
			t.Fatalf("canon(%q) = %q, still short", short, long)
		}
		child, err := canon(base)
		if err != nil {
			t.Fatal(err)
		}
		if !insideRoot(short, child) {
			t.Fatalf("insideRoot(%q, %q) = false", short, child)
		}
	})
	t.Run("a junction is kept", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("junctions are Windows only")
		}
		target := mkdirs(t, filepath.Join(base, "target"))
		if err := os.WriteFile(filepath.Join(target, "f"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		j := filepath.Join(base, "j")
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", j, target).CombinedOutput(); err != nil {
			t.Skipf("cannot make a junction here: %v %s", err, out)
		}
		c, err := canon(filepath.Join(j, "f"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(c, `\j\f`) {
			t.Fatalf("canon through a junction = %q, want it to end \\j\\f", c)
		}
	})
}
