package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var (
	lsProd = []string{"ls", "--all", "--omit=dev", "--json", "--long"}
	lsAll  = []string{"ls", "--all", "--include=dev", "--json", "--long"}
)

// npmRunner answers the two npm ls commands with prod and all, "@ROOT@"
// replaced by base in slash form, and hands any "node" call to node (nil:
// an error). Every npm command line is recorded in calls.
func npmRunner(base string, prod, all []byte, node Runner, calls *[][]string) Runner {
	sub := func(b []byte) []byte {
		return []byte(strings.ReplaceAll(string(b), "@ROOT@", filepath.ToSlash(base)))
	}
	return func(dir, name string, args ...string) ([]byte, error) {
		switch {
		case name == "npm" && reflect.DeepEqual(args, lsProd):
			*calls = append(*calls, args)
			return sub(prod), nil
		case name == "npm" && reflect.DeepEqual(args, lsAll):
			*calls = append(*calls, args)
			return sub(all), nil
		case name == "node" && node != nil:
			return node(dir, name, args...)
		}
		return nil, errors.New("unexpected command " + name + " " + strings.Join(args, " "))
	}
}

// pkgJSON writes <dir>/package.json with a name, version and license.
func pkgJSON(t *testing.T, dir, name, version, license string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"name": name, "version": version, "license": license})
	writeFile(t, filepath.Join(dir, "package.json"), b)
}

func npmDeps(t *testing.T, deps []Dep) map[string]Dep {
	t.Helper()
	m := map[string]Dep{}
	for _, d := range deps {
		if d.Ecosystem != Npm {
			t.Errorf("%s: ecosystem %q", d.Name, d.Ecosystem)
		}
		key := d.Name + "@" + d.Version
		if _, dup := m[key]; dup {
			t.Errorf("%s listed twice", key)
		}
		m[key] = d
	}
	return m
}

func verdicts(t *testing.T, deps []Dep) map[string]Finding {
	t.Helper()
	fs, err := Evaluate(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]Finding{}
	for _, f := range fs {
		m[f.Dep.Name+"@"+f.Dep.Version] = f
	}
	return m
}

// firstPartyTree lays out, under base, the directories the real npm run
// behind testdata/npm/firstparty installed (see TestNpmDeps_firstPartyIsStructural),
// with each node's package.json. Link locations are plain directories: the
// gate reads a link from package-lock.json, never from the filesystem.
func firstPartyTree(t *testing.T, base string, lock []byte) (root, proj string) {
	root = filepath.Join(base, "fx")
	proj = filepath.Join(root, "proj")
	nm := filepath.Join(proj, "node_modules")
	pkgJSON(t, proj, "proj", "1.0.0", "UNLICENSED")
	writeFile(t, filepath.Join(proj, "package-lock.json"), lock)
	mkdirs(t, filepath.Join(proj, "src"), filepath.Join(nm, "@panacea-app", "b"), filepath.Join(nm, "forkx"), filepath.Join(nm, "vx"))
	pkgJSON(t, filepath.Join(nm, "desktop-notify"), "picocolors", "1.1.1", "ISC")
	pkgJSON(t, filepath.Join(nm, "@panacea-app", "x"), "siginfo", "2.0.0", "GPL-3.0")
	pkgJSON(t, filepath.Join(nm, "g"), "g", "1.0.0", "MIT")
	pkgJSON(t, filepath.Join(nm, "gpl-dev"), "gpl-dev", "1.0.0", "GPL-3.0")
	pkgJSON(t, filepath.Join(root, "b"), "@panacea-app/b", "0.1.0", "UNLICENSED")
	pkgJSON(t, filepath.Join(root, "b", "node_modules", "bchild"), "siginfo", "2.0.0", "GPL-3.0")
	pkgJSON(t, filepath.Join(root, "vendor", "x"), "vx", "1.0.0", "GPL-3.0")
	pkgJSON(t, filepath.Join(base, "fx-fork", "x"), "forkx", "1.0.0", "UNLICENSED")
	return root, proj
}

func npmFixture(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "npm", dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestNpmDeps_firstPartyIsStructural reads testdata/npm/firstparty, copied
// from a real run of npm 11.12.1 on Windows (scratchpad p5/t04npm/run2):
// "npm install --offline" in fx/b, then in fx/proj, then in fx/proj
//
//	npm ls --all --omit=dev --json --long   (ls-prod.json)
//	npm ls --all --include=dev --json --long (ls-all.json)
//
// and fx/proj/package-lock.json as npm wrote it. Only the machine prefix
// changed: the run's directory became "@ROOT@" and "\" became "/". A fixture
// composed from the rule's own words agrees with a wrong rule; this one is
// what npm prints. -root is fx; fx/proj and fx/b are the scanned projects.
func TestNpmDeps_firstPartyIsStructural(t *testing.T) {
	prod := npmFixture(t, "firstparty", "ls-prod.json")
	all := npmFixture(t, "firstparty", "ls-all.json")
	lock := npmFixture(t, "firstparty", "package-lock.json")

	base := t.TempDir()
	root, proj := firstPartyTree(t, base, lock)
	var calls [][]string
	deps, err := NpmDeps(root, proj, "proj", []string{proj, filepath.Join(root, "b")}, npmRunner(base, prod, all, nil, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, [][]string{lsProd, lsAll}) {
		t.Errorf("npm calls = %v, want the two ls --long commands", calls)
	}
	got := npmDeps(t, deps)
	v := verdicts(t, deps)
	for _, gone := range []string{"proj@1.0.0", "@panacea-app/b@0.1.0", "bchild@2.0.0"} {
		if _, ok := got[gone]; ok {
			t.Errorf("%s is a Dep: the root, a link to a scanned project and its children are skipped", gone)
		}
	}
	want := map[string]Verdict{
		"desktop-notify@1.1.1": Pass, // a registry package named like ours: checked
		"@panacea-app/x@2.0.0": Fail, // a registry package in our scope, GPL-3.0
		"vx@1.0.0":             Fail, // a link into -root that is not a scanned project
		"forkx@1.0.0":          Fail, // a link into <root>-fork: third party, UNLICENSED
		"g@1.0.0":              Pass, // a file: tarball (no "link"): checked, MIT (RR2-M2)
		"gpl-dev@1.0.0":        Report,
	}
	for k, wv := range want {
		if f, ok := v[k]; !ok || f.Verdict != wv {
			t.Errorf("%s: verdict %q (%v), want %q", k, f.Verdict, ok, wv)
		}
	}
	if len(got) != len(want) {
		t.Errorf("deps = %v, want exactly %d", got, len(want))
	}
	if w := v["vx@1.0.0"].Why; !strings.Contains(w, "is not a scanned project: add it to the scan list") {
		t.Errorf("vx why = %q", w)
	}
	fk := got["forkx@1.0.0"]
	if fk.Declared != "UNLICENSED" || !samePath(fk.Path, filepath.Join(base, "fx-fork", "x")) || !fk.Enforced {
		t.Errorf("forkx = %+v: want it classified from its target's package.json", fk)
	}
	if x := got["@panacea-app/x@2.0.0"]; x.Declared != "GPL-3.0" || x.Via != "production" || x.From != "proj" {
		t.Errorf("@panacea-app/x = %+v: want GPL-3.0 read from its own path, production, from proj", x)
	}

	t.Run("a node with no lockfile entry is checked", func(t *testing.T) {
		var l map[string]any
		if err := json.Unmarshal(lock, &l); err != nil {
			t.Fatal(err)
		}
		delete(l["packages"].(map[string]any), "node_modules/desktop-notify")
		trimmed, _ := json.Marshal(l)
		base := t.TempDir()
		root, proj := firstPartyTree(t, base, trimmed)
		var calls [][]string
		deps, err := NpmDeps(root, proj, "proj", []string{proj, filepath.Join(root, "b")}, npmRunner(base, prod, all, nil, &calls))
		if err != nil {
			t.Fatal(err)
		}
		if d, ok := npmDeps(t, deps)["desktop-notify@1.1.1"]; !ok || d.Declared != "ISC" {
			t.Fatalf("desktop-notify with no lockfile entry = %+v, %v: want it checked", d, ok)
		}
	})
	t.Run("no lockfile is an error", func(t *testing.T) {
		base := t.TempDir()
		root, proj := firstPartyTree(t, base, lock)
		if err := os.Remove(filepath.Join(proj, "package-lock.json")); err != nil {
			t.Fatal(err)
		}
		var calls [][]string
		if _, err := NpmDeps(root, proj, "proj", []string{proj}, npmRunner(base, prod, all, nil, &calls)); err == nil {
			t.Fatal("NpmDeps with no package-lock.json returned no error")
		}
	})
}
