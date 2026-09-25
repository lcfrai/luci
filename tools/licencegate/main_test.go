package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRepo answers run()'s commands for a composed repository: go list,
// npm ls per project directory, and cargo metadata.
type fakeRepo struct {
	goOut, cargoOut []byte
	goErr, npmErr   error
	cargoErr        error
	npm             map[string][]byte
}

func (f *fakeRepo) run(dir, name string, args ...string) ([]byte, error) {
	switch name {
	case "go":
		return f.goOut, f.goErr
	case "cargo":
		return f.cargoOut, f.cargoErr
	case "npm":
		if f.npmErr != nil {
			return nil, f.npmErr
		}
		for d, b := range f.npm {
			if samePath(d, dir) {
				return b, nil
			}
		}
		return nil, errors.New("no npm fixture for " + dir)
	}
	return nil, errors.New("unexpected command " + name + " " + strings.Join(args, " "))
}

// cleanRepo lays out every scanned project under a temporary root: the three
// Go roots, desktop and canvas/editor (react MIT in desktop's production
// closure), and desktop/src-tauri with one MIT crate. mpl adds a production
// mpl-fixture 1.0.0 declaring MPL-2.0 to desktop.
func cleanRepo(t *testing.T, mpl bool) (string, *fakeRepo) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "canvas"), filepath.Join(root, "control-plane", "cmd"), filepath.Join(root, "sandboxd", "cmd"))
	mod := moduleDir(t, map[string][]byte{"LICENSE": fixture(t, "MIT.txt")})
	goOut, _ := json.Marshal(goPackage{ImportPath: "example.com/mit", Module: &goModule{Path: "example.com/mit", Version: "v1.0.0", Dir: mod}})
	desk := newProject(t, filepath.Join(root, "desktop"))
	top := kids("react", desk.node("node_modules/react", "react", "19.1.0", "MIT", nil))
	if mpl {
		top["mpl-fixture"] = desk.node("node_modules/mpl-fixture", "mpl-fixture", "1.0.0", "MPL-2.0", nil)
	}
	ed := newProject(t, filepath.Join(root, "canvas", "editor"))
	writeFile(t, filepath.Join(root, "desktop", "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"))
	m := &cargoMeta{root: root, nodes: map[string][]map[string]any{}}
	member := m.pkg("desktop", "0.1.0", "path", "bin")
	m.dep(member, m.pkg("fnv", "1.0.7", "Apache-2.0 / MIT"), "")
	return root, &fakeRepo{goOut: goOut, cargoOut: m.json(member), npm: map[string][]byte{
		desk.dir: desk.write(top),
		ed.dir:   ed.write(nil),
	}}
}

func TestMain_exitCodes(t *testing.T) {
	cases := []struct {
		name     string
		mpl      bool
		edit     func(t *testing.T, root string, f *fakeRepo)
		code     int
		inOut    string // a line stdout must hold
		inErr    string // text stderr must hold
		notInOut string
	}{
		{name: "clean", code: 0, inOut: "PASS npm react 19.1.0 MIT — MIT is allowed", notInOut: "FAIL"},
		{name: "an MPL package fails", mpl: true, code: 1, inOut: "FAIL npm mpl-fixture 1.0.0 MPL-2.0 — "},
		{name: "go list fails", code: 2, inErr: "could not run", edit: func(t *testing.T, _ string, f *fakeRepo) {
			f.goErr = errors.New("go: cannot find main module")
		}},
		{name: "npm ls fails", code: 2, inErr: "could not run", edit: func(t *testing.T, _ string, f *fakeRepo) {
			f.npmErr = errors.New("exit status 1: npm error code ELSPROBLEMS")
		}},
		{name: "an MPL crate is reported", code: 0, inOut: "REPORT cargo cssparser 0.36.0 MPL-2.0 — ", edit: func(t *testing.T, root string, f *fakeRepo) {
			m := &cargoMeta{root: root, nodes: map[string][]map[string]any{}}
			member := m.pkg("desktop", "0.1.0", "path", "bin")
			m.dep(member, m.pkg("cssparser", "0.36.0", "MPL-2.0"), "")
			f.cargoOut = m.json(member)
		}},
		{name: "cargo failing is reported", code: 0, inOut: "REPORT cargo: not scanned: ", edit: func(t *testing.T, _ string, f *fakeRepo) {
			f.cargoErr = errors.New(`exec: "cargo": executable file not found in %PATH%`)
		}},
		{name: "a linked node_modules (carried RR2-1)", code: 2, inErr: "node_modules is a link into", edit: func(t *testing.T, root string, f *fakeRepo) {
			nm := filepath.Join(root, "desktop", "node_modules")
			other := mkdirs(t, filepath.Join(t.TempDir(), "main", "desktop", "node_modules"))
			if err := os.RemoveAll(nm); err != nil {
				t.Fatal(err)
			}
			linkDir(t, other, nm)
		}},
		{name: "an exception for tldraw", code: 2, inErr: "OD-11", edit: func(t *testing.T, root string, f *fakeRepo) {
			writeFile(t, filepath.Join(root, "tools", "licencegate", "exceptions.json"), []byte(`{"exceptions":[{"ecosystem":"npm","name":"tldraw","version":"3.0.0","declared":"MIT","resolved":"MIT","reason":"r","reviewedBy":"x","reviewedOn":"2026-09-25"}]}`))
		}},
		// Mutation A3 survived without this row: a partial Go tree is a
		// broken checkout, never three roots with one quietly skipped.
		{name: "a partial Go tree", code: 2, inErr: "go: ./canvas/..., ./control-plane/cmd/... under -root, but not ./sandboxd/cmd/...", edit: func(t *testing.T, root string, f *fakeRepo) {
			if err := os.RemoveAll(filepath.Join(root, "sandboxd")); err != nil {
				t.Fatal(err)
			}
		}},
		// Mutation A5 survived without this row. encoding/json matches names
		// case-insensitively, so the typo is a real one; without the check the
		// run still exits 2 (reviewedBy is required), but not naming the typo.
		{name: "a misspelt exception field", code: 2, inErr: "unknown field", edit: func(t *testing.T, root string, f *fakeRepo) {
			writeFile(t, filepath.Join(root, "tools", "licencegate", "exceptions.json"), []byte(`{"exceptions":[{"ecosystem":"npm","name":"x","version":"1.0.0","declared":"MPL-2.0","resolved":"MIT","reason":"r","reviewdBy":"x","reviewedOn":"2026-09-25"}]}`))
		}},
		{name: "no scanned project under -root", code: 2, inErr: "no scanned project", edit: func(t *testing.T, root string, f *fakeRepo) {
			for _, d := range []string{"canvas", "control-plane", "sandboxd", "desktop"} {
				if err := os.RemoveAll(filepath.Join(root, d)); err != nil {
					t.Fatal(err)
				}
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, f := cleanRepo(t, c.mpl)
			if c.edit != nil {
				c.edit(t, root, f)
			}
			code, out, errOut := runGate(t, []string{"-root", root}, f)
			if code != c.code {
				t.Errorf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, c.code, out, errOut)
			}
			if !strings.Contains(out, c.inOut) || !strings.Contains(errOut, c.inErr) {
				t.Errorf("stdout lacks %q or stderr lacks %q\nstdout:\n%s\nstderr:\n%s", c.inOut, c.inErr, out, errOut)
			}
			if c.notInOut != "" && strings.Contains(out, c.notInOut) {
				t.Errorf("stdout holds %q:\n%s", c.notInOut, out)
			}
			if code != 2 && !strings.Contains(out, "REPORT cargo") && !strings.Contains(out, "cargo: ") {
				t.Errorf("no cargo line: the Cargo report always prints\n%s", out)
			}
		})
	}
}

// TestMain_repoWithMPL is the spec's done-when (§9.1 item 13) with the real
// runner: the committed fixture repository's desktop has one production
// package declaring MPL-2.0, and the gate run on it exits 1 naming it.
func TestMain_repoWithMPL(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is not on PATH: the done-when needs npm ls")
	}
	var out, errOut bytes.Buffer
	code := run([]string{"-root", filepath.Join("testdata", "repo-with-mpl")}, &out, &errOut, execRunner)
	if code != 1 || !strings.Contains(out.String(), "FAIL npm mpl-fixture 1.0.0 MPL-2.0 — ") {
		t.Fatalf("exit %d, want 1 with the MPL fixture named\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
}

func runGate(t *testing.T, args []string, f *fakeRepo) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut, f.run)
	return code, out.String(), errOut.String()
}
