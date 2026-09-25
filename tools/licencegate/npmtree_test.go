package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// linkDir makes link point at target: a junction on Windows (what the
// program's worktrees use), a symlink elsewhere. It skips when it cannot.
func linkDir(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Skipf("cannot make a junction here: %v %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
}

// TestNpmDeps_linkedNodeModules is carried RR2-1: a worktree whose
// node_modules is a link into another tree ships that tree's packages, so
// the gate refuses to read it (exit 2), names the link, and says the remedy.
func TestNpmDeps_linkedNodeModules(t *testing.T) {
	for _, c := range []struct {
		mode fs.FileMode
		want bool
	}{{fs.ModeSymlink, true}, {fs.ModeIrregular, true}, {fs.ModeDir, false}, {0, false}} {
		if got := isLinkMode(c.mode); got != c.want {
			t.Errorf("isLinkMode(%v) = %v, want %v", c.mode, got, c.want)
		}
	}
	base := t.TempDir()
	mainTree := mkdirs(t, filepath.Join(base, "main", "desktop", "node_modules"))
	dir := filepath.Join(base, "wt", "desktop")
	pkgJSON(t, dir, "desktop", "0.1.0", "UNLICENSED")
	writeFile(t, filepath.Join(dir, "package-lock.json"), []byte(`{"packages":{}}`))
	linkDir(t, mainTree, filepath.Join(dir, "node_modules"))
	if fi, err := os.Lstat(filepath.Join(dir, "node_modules")); err == nil {
		t.Logf("Lstat mode of the link on %s: %v", runtime.GOOS, fi.Mode())
	}
	var calls [][]string
	_, err := NpmDeps(base, dir, "desktop", []string{dir}, npmRunner(base, nil, nil, nil, &calls))
	if err == nil {
		t.Fatal("NpmDeps read a linked node_modules")
	}
	for _, w := range []string{"node_modules is a link into", "never rm -rf through it", "run npm ci in"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q lacks %q", err, w)
		}
	}
	if len(calls) != 0 {
		t.Errorf("npm ran %v through the link", calls)
	}
}

// lsNode is the part of an npm ls --long node the gate reads, for trees a
// test composes where no npm run is needed to make its point.
type lsNode struct {
	Name         string             `json:"name,omitempty"`
	Version      string             `json:"version,omitempty"`
	Path         string             `json:"path,omitempty"`
	Dependencies map[string]*lsNode `json:"dependencies,omitempty"`
}

// npmProject is one composed project: its directory, its lockfile entries
// and the package.json each installed node carries.
type npmProject struct {
	t    *testing.T
	dir  string
	lock map[string]any
}

func newProject(t *testing.T, dir string) *npmProject {
	pkgJSON(t, dir, filepath.Base(dir), "0.1.0", "UNLICENSED")
	mkdirs(t, filepath.Join(dir, "src"), filepath.Join(dir, "node_modules"))
	return &npmProject{t: t, dir: dir, lock: map[string]any{"": map[string]any{"name": filepath.Base(dir)}}}
}

// node installs <dir>/<rel> (slash form) with a package.json and a lockfile
// entry, and returns its ls node with kids as its dependencies.
func (p *npmProject) node(rel, name, version, license string, kids map[string]*lsNode) *lsNode {
	path := filepath.Join(p.dir, filepath.FromSlash(rel))
	pkgJSON(p.t, path, name, version, license)
	p.lock[rel] = map[string]any{"version": version, "license": license}
	return &lsNode{Name: name, Version: version, Path: path, Dependencies: kids}
}

// link records rel as an npm link to target (relative to dir) in the lockfile.
func (p *npmProject) link(rel, target string) *lsNode {
	mkdirs(p.t, filepath.Join(p.dir, filepath.FromSlash(rel)))
	p.lock[rel] = map[string]any{"resolved": target, "link": true}
	return &lsNode{Name: filepath.Base(rel), Version: "0.1.0", Path: filepath.Join(p.dir, filepath.FromSlash(rel))}
}

// write writes package-lock.json and returns the ls JSON for deps.
func (p *npmProject) write(deps map[string]*lsNode) []byte {
	lock, _ := json.Marshal(map[string]any{"lockfileVersion": 3, "packages": p.lock})
	writeFile(p.t, filepath.Join(p.dir, "package-lock.json"), lock)
	b, err := json.Marshal(&lsNode{Name: filepath.Base(p.dir), Version: "0.1.0", Path: p.dir, Dependencies: deps})
	if err != nil {
		p.t.Fatal(err)
	}
	return b
}

func kids(kv ...any) map[string]*lsNode {
	m := map[string]*lsNode{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1].(*lsNode)
	}
	return m
}

// TestNpmDeps_importedDevDepIsEnforced is review I9's hole 1: Vite bundles
// what shipped source imports, whichever package.json list names it.
func TestNpmDeps_importedDevDepIsEnforced(t *testing.T) {
	base := t.TempDir()
	editor := filepath.Join(base, "canvas", "editor")
	newProject(t, editor)
	p := newProject(t, filepath.Join(base, "desktop"))
	react := p.node("node_modules/react", "react", "19.1.0", "MIT", nil)
	font := p.node("node_modules/font-lib", "font-lib", "2.0.0", "MIT", nil)
	hti := p.node("node_modules/html-to-image", "html-to-image", "1.11.13", "GPL-3.0", kids("font-lib", font))
	vite := p.node("node_modules/vite", "vite", "8.3.0", "MIT", kids("lightningcss", p.node("node_modules/lightningcss", "lightningcss", "1.30.1", "MPL-2.0", nil)))
	vitest := p.node("node_modules/vitest", "vitest", "5.0.1", "MPL-2.0", kids("vite", vite))
	ed := p.link("node_modules/@panacea-app/canvas-editor", "../canvas/editor")
	prod := p.write(kids("react", react, "@panacea-app/canvas-editor", ed))
	all := p.write(kids("react", react, "@panacea-app/canvas-editor", ed, "html-to-image", hti, "vitest", vitest))
	files := map[string][]importRec{
		"src/main.tsx":      spec("react", "./export/png", "ghost-pkg", "@panacea-app/canvas-editor"),
		"src/export/png.ts": spec("html-to-image"),
		"src/a.test.ts":     spec("vitest"),
		"src/test/setup.ts": spec("vitest"),
	}
	for rel := range files {
		writeFile(t, filepath.Join(p.dir, filepath.FromSlash(rel)), []byte("// fixture\n"))
	}
	var asked []string
	var calls [][]string
	run := npmRunner(base, prod, all, helperRunner(t, files, &asked), &calls)
	deps, err := NpmDeps(base, p.dir, "desktop", []string{p.dir, editor}, run)
	if err != nil {
		t.Fatal(err)
	}
	got := npmDeps(t, deps)
	v := verdicts(t, deps)
	const via = "imported by desktop/src/export/png.ts"
	if d := got["html-to-image@1.11.13"]; !d.Enforced || d.Via != via || v["html-to-image@1.11.13"].Verdict != Fail {
		t.Errorf("html-to-image = %+v, %q: want enforced %q and fail", d, v["html-to-image@1.11.13"].Verdict, via)
	}
	if d := got["font-lib@2.0.0"]; !d.Enforced || d.Via != via {
		t.Errorf("font-lib (html-to-image's production child) = %+v: want enforced %q", d, via)
	}
	for _, k := range []string{"vitest@5.0.1", "lightningcss@1.30.1"} {
		if d := got[k]; d.Enforced || v[k].Verdict != Report {
			t.Errorf("%s = %+v, %q: imported only by test files, want report", k, d, v[k].Verdict)
		}
	}
	if d := got["react@19.1.0"]; !d.Enforced || d.Via != "production" {
		t.Errorf("react = %+v: want production", d)
	}
	ghost, ok := got["ghost-pkg@"]
	if !ok || v["ghost-pkg@"].Verdict != Fail || v["ghost-pkg@"].Why != "imported by desktop/src/main.tsx but not installed" {
		t.Errorf("ghost-pkg = %+v, %v, %+v: want fail, not installed", ghost, ok, v["ghost-pkg@"])
	}
	if _, ok := got["@panacea-app/canvas-editor@0.1.0"]; ok || len(got) != 7 {
		t.Errorf("deps = %v: want 7, and the scanned editor link skipped", got)
	}
}

// TestNpmDeps_redactedPath: npm 11.12.1 passes what it prints through
// @npmcli/redact 4.0.0, which turns a UUID into "***", so a project under a
// directory named by one (this program's own scratchpad) gets node paths
// that do not exist. Measured 2026-09-26: the gate exited 2 on its own
// mutation copy ("CreateFile …\***\scratchpad\…: The filename, directory
// name, or volume label syntax is incorrect"). The gate restores each "***"
// element from the one directory entry that redacts to it.
func TestNpmDeps_redactedPath(t *testing.T) {
	const uuid = "2ce0c1a9-63c3-42f2-bf4e-e6ba45d14fab"
	base := filepath.Join(t.TempDir(), uuid)
	p := newProject(t, filepath.Join(base, "desktop"))
	x := p.node("node_modules/x", "x", "1.0.0", "GPL-3.0", nil)
	tree := []byte(strings.ReplaceAll(string(p.write(kids("x", x))), uuid, "***"))
	if !strings.Contains(string(tree), "***") {
		t.Fatal("the fixture holds no redacted path")
	}
	var calls [][]string
	deps, err := NpmDeps(base, p.dir, "desktop", []string{p.dir}, npmRunner(base, tree, tree, nil, &calls))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := npmDeps(t, deps)["x@1.0.0"]
	if !ok || d.Declared != "GPL-3.0" || !strings.Contains(d.Path, uuid) {
		t.Fatalf("x = %+v, %v: want it read at its real path", d, ok)
	}
	t.Run("two entries that redact alike are an error", func(t *testing.T) {
		mkdirs(t, filepath.Join(filepath.Dir(base), "0e2cc1a9-63c3-42f2-bf4e-e6ba45d14fab"))
		if _, err := NpmDeps(base, p.dir, "desktop", []string{p.dir}, npmRunner(base, tree, tree, nil, &calls)); err == nil {
			t.Fatal("an ambiguous redacted path was guessed")
		}
	})
}

// TestNpmDeps_readsEachCopyAtItsPath is hole 4: nodes are keyed by path.
func TestNpmDeps_readsEachCopyAtItsPath(t *testing.T) {
	base := t.TempDir()
	p := newProject(t, filepath.Join(base, "desktop"))
	x1 := p.node("node_modules/x", "x", "1.0.0", "MIT", nil)
	x2 := p.node("node_modules/y/node_modules/x", "x", "2.0.0", "GPL-3.0", nil)
	w := p.node("node_modules/w", "w", "1.0.0", "MIT", nil)
	b := p.node("node_modules/b", "b", "1.0.0", "MIT", nil)
	// z is printed under y with its child and again at the root without it;
	// a is printed at the root without its child and again under y with it.
	// Whichever print a walk meets first, the children are unioned.
	zFull := p.node("node_modules/z", "z", "1.0.0", "MIT", kids("w", w))
	zBare := &lsNode{Name: "z", Version: "1.0.0", Path: zFull.Path}
	aFull := p.node("node_modules/a", "a", "1.0.0", "MIT", kids("b", b))
	aBare := &lsNode{Name: "a", Version: "1.0.0", Path: aFull.Path}
	y := p.node("node_modules/y", "y", "1.0.0", "MIT", kids("x", x2, "z", zFull, "a", aFull))
	tree := p.write(kids("a", aBare, "x", x1, "y", y, "z", zBare))
	var calls [][]string
	deps, err := NpmDeps(base, p.dir, "desktop", []string{p.dir}, npmRunner(base, tree, tree, nil, &calls))
	if err != nil {
		t.Fatal(err)
	}
	got := npmDeps(t, deps)
	v := verdicts(t, deps)
	if got["x@1.0.0"].Declared != "MIT" || v["x@1.0.0"].Verdict != Pass {
		t.Errorf("node_modules/x = %+v", got["x@1.0.0"])
	}
	if got["x@2.0.0"].Declared != "GPL-3.0" || v["x@2.0.0"].Verdict != Fail {
		t.Errorf("node_modules/y/node_modules/x = %+v, %q: want GPL-3.0 read at its own path, fail", got["x@2.0.0"], v["x@2.0.0"].Verdict)
	}
	for _, k := range []string{"w@1.0.0", "b@1.0.0"} {
		if d, ok := got[k]; !ok || !d.Enforced {
			t.Errorf("%s = %+v, %v: a child of a node printed twice at one path is lost", k, d, ok)
		}
	}
	if len(got) != 7 {
		t.Errorf("deps = %v, want 7", got)
	}
}

// TestNpmDeps: the production closure is enforced, dev-only is reported,
// and each package.json licence form is read.
func TestNpmDeps(t *testing.T) {
	base := t.TempDir()
	p := newProject(t, filepath.Join(base, "desktop"))
	two := p.node("node_modules/two", "two", "1.0.0", "", nil)
	writeFile(t, filepath.Join(two.Path, "package.json"), []byte(`{"name":"two","version":"1.0.0","licenses":[{"type":"MIT"},{"type":"ISC"}]}`))
	obj := p.node("node_modules/obj", "obj", "1.0.0", "", nil)
	writeFile(t, filepath.Join(obj.Path, "package.json"), []byte(`{"name":"obj","version":"1.0.0","license":{"type":"ISC"}}`))
	alias := p.node("node_modules/my-canvas", "tldraw", "3.0.0", "MIT", nil)
	// npm 11.12.1 prints an optional peer dependency that is not installed as
	// {} (desktop's @vitejs/plugin-react -> "@rolldown/plugin-babel": {},
	// read 2026-09-25): nothing is installed, so nothing ships; skipped.
	dev := p.node("node_modules/dev-tool", "dev-tool", "1.0.0", "MIT", kids("opt-peer", &lsNode{}))
	prod := p.write(kids("two", two, "obj", obj, "my-canvas", alias))
	all := p.write(kids("two", two, "obj", obj, "my-canvas", alias, "dev-tool", dev))
	var calls [][]string
	deps, err := NpmDeps(base, p.dir, "desktop", []string{p.dir}, npmRunner(base, prod, all, nil, &calls))
	if err != nil {
		t.Fatal(err)
	}
	got := npmDeps(t, deps)
	v := verdicts(t, deps)
	if d := got["two@1.0.0"]; d.Declared != "MIT AND ISC" || !d.Enforced || v["two@1.0.0"].Verdict != Pass {
		t.Errorf("licenses array = %+v: want MIT AND ISC, enforced, pass", d)
	}
	if d := got["obj@1.0.0"]; d.Declared != "ISC" {
		t.Errorf("license object = %+v: want ISC", d)
	}
	if d := got["dev-tool@1.0.0"]; d.Enforced || d.Via != "" {
		t.Errorf("dev-only, not imported = %+v: want reported", d)
	}
	if f := v["my-canvas@3.0.0"]; f.Verdict != Fail || !strings.Contains(f.Why, "OD-11") {
		t.Errorf("an alias of tldraw = %+v: want denied whatever its installed name", f)
	}
}
