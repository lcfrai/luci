package product

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type previewArchiveEntry struct {
	name string
	body string
	mode int64
	kind byte
}

func previewArchiveBytes(t *testing.T, entries ...previewArchiveEntry) *bytes.Buffer {
	t.Helper()
	var data bytes.Buffer
	writer := tar.NewWriter(&data)
	for _, entry := range entries {
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Mode: entry.mode, Typeflag: entry.kind, Size: int64(len(entry.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &data
}

func assertPreviewArchiveMode(t *testing.T, name string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not implement POSIX access modes. Content, confinement and
	// no-overwrite assertions still run there; the Linux regression executes
	// these exact permission assertions, including under umask 077.
	if runtime.GOOS != "windows" && (info.Mode().Perm() != want || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0) {
		t.Fatalf("%s mode = %v; want canonical %v without special bits", filepath.Base(name), info.Mode(), want)
	}
}

func checkPreviewArchiveCanonicalModes(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	data := previewArchiveBytes(t,
		previewArchiveEntry{name: "index.html", body: "public page\n", mode: 0600, kind: tar.TypeReg},
		previewArchiveEntry{name: "implicit/deeper/start.sh", body: "#!/bin/sh\n", mode: 07777, kind: tar.TypeReg},
		previewArchiveEntry{name: "explicit/", mode: 01777, kind: tar.TypeDir},
		previewArchiveEntry{name: "explicit/config.json", body: "{}\n", mode: 0444, kind: tar.TypeReg},
		previewArchiveEntry{name: "implicit/", mode: 0700, kind: tar.TypeDir},
		previewArchiveEntry{name: ".env", body: "TOKEN=excluded\n", mode: 0777, kind: tar.TypeReg},
	)
	if err := extractPreviewArchive(data, root); err != nil {
		t.Fatal(err)
	}
	assertPreviewArchiveMode(t, root, 0700)
	for _, name := range []string{"implicit", "implicit/deeper", "explicit"} {
		assertPreviewArchiveMode(t, filepath.Join(root, filepath.FromSlash(name)), 0755)
	}
	for name, want := range map[string]os.FileMode{"index.html": 0644, "implicit/deeper/start.sh": 0755, "explicit/config.json": 0644} {
		assertPreviewArchiveMode(t, filepath.Join(root, filepath.FromSlash(name)), want)
	}
	if body, err := os.ReadFile(filepath.Join(root, "index.html")); err != nil || string(body) != "public page\n" {
		t.Fatalf("committed public content changed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".env")); !os.IsNotExist(err) {
		t.Fatal("private environment file reached the build context")
	}
}

func TestPreviewArchiveCanonicalModesInsidePrivateRoot(t *testing.T) {
	checkPreviewArchiveCanonicalModes(t)
}

func TestArchiveCommittedProjectPreservesNonRootCopyPermissions(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "archive-copy-permissions")
	if err := os.Mkdir(filepath.Join(p.Path, "public"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"public/index.html": "committed public page\n", "start.sh": "#!/bin/sh\n", ".env": "TOKEN=excluded\n"} {
		mode := os.FileMode(0600)
		if name == "start.sh" {
			mode = 0755
		}
		if err := os.WriteFile(filepath.Join(p.Path, filepath.FromSlash(name)), []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "--all"}, {"update-index", "--chmod=+x", "--", "start.sh"}} {
		if out, err := RunCommand(context.Background(), "git", args, p.Path); err != nil {
			t.Fatalf("Git fixture: %v: %s", err, out)
		}
	}
	commitPreviewFixture(t, p.Path)
	commit, err := RunCommand(context.Background(), "git", []string{"rev-parse", "HEAD"}, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := archiveCommittedProject(context.Background(), p.Path, string(bytes.TrimSpace([]byte(commit))), root); err != nil {
		t.Fatal(err)
	}
	assertPreviewArchiveMode(t, root, 0700)
	assertPreviewArchiveMode(t, filepath.Join(root, "public"), 0755)
	assertPreviewArchiveMode(t, filepath.Join(root, "public", "index.html"), 0644)
	assertPreviewArchiveMode(t, filepath.Join(root, "start.sh"), 0755)
	if body, err := os.ReadFile(filepath.Join(root, "public", "index.html")); err != nil || string(body) != "committed public page\n" {
		t.Fatalf("Git archive did not retain committed content: %v", err)
	}
	for _, name := range []string{".env", ".git", ".luci"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("private %s reached the build context", name)
		}
	}
}

func TestPreviewArchiveRefusesLinkedAncestorsAndPreservesOutside(t *testing.T) {
	for _, entry := range []previewArchiveEntry{
		{name: "linked/new/", mode: 0755, kind: tar.TypeDir},
		{name: "linked/new/index.html", body: "must not escape\n", mode: 0644, kind: tar.TypeReg},
		{name: "linked/", mode: 0755, kind: tar.TypeDir},
	} {
		t.Run(entry.name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(outside, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outside, "retained.txt"), []byte("keep outside\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if err := extractPreviewArchive(previewArchiveBytes(t, entry), root); err == nil {
				t.Fatal("linked archive ancestor was accepted")
			}
			assertPreviewArchiveMode(t, outside, 0700)
			if _, err := os.Lstat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
				t.Fatal("extraction created a directory outside its private root")
			}
			if body, err := os.ReadFile(filepath.Join(outside, "retained.txt")); err != nil || string(body) != "keep outside\n" {
				t.Fatalf("outside content changed: %v", err)
			}
		})
	}
}

func TestPreviewArchiveRefusesLinkedRootAndExistingFiles(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		parent, outside := t.TempDir(), t.TempDir()
		if err := os.Chmod(outside, 0700); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(parent, "linked")
		if err := os.Symlink(outside, root); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if err := extractPreviewArchive(previewArchiveBytes(t, previewArchiveEntry{name: "index.html", body: "bad\n", mode: 0644, kind: tar.TypeReg}), root); err == nil {
			t.Fatal("linked staging root was accepted")
		}
		assertPreviewArchiveMode(t, outside, 0700)
		if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
			t.Fatal("linked staging root changed outside resources")
		}
	})
	t.Run("existing-file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "index.html")
		if err := os.WriteFile(file, []byte("keep existing\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := extractPreviewArchive(previewArchiveBytes(t, previewArchiveEntry{name: "index.html", body: "bad\n", mode: 07777, kind: tar.TypeReg}), root); err == nil {
			t.Fatal("pre-existing file was overwritten")
		}
		assertPreviewArchiveMode(t, file, 0600)
		if body, err := os.ReadFile(file); err != nil || string(body) != "keep existing\n" {
			t.Fatalf("existing file content changed: %v", err)
		}
	})
}
