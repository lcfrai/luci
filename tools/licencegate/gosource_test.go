package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type goCall struct {
	dir  string
	args []string
}

// goListRunner answers "go list" with a JSON stream per pattern, and records
// every call.
func goListRunner(byPattern map[string][]goPackage, calls *[]goCall) Runner {
	return func(dir, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, goCall{dir, append([]string{name}, args...)})
		if name != "go" || len(args) < 4 || args[0] != "list" {
			return nil, errors.New("unexpected command " + name + " " + strings.Join(args, " "))
		}
		var out []byte
		for _, p := range byPattern[args[len(args)-1]] {
			b, _ := json.MarshalIndent(p, "", "\t")
			out = append(append(out, b...), '\n')
		}
		return out, nil
	}
}

func depsByName(deps []Dep) map[string]Dep {
	m := map[string]Dep{}
	for _, d := range deps {
		m[d.Name] = d
	}
	return m
}

// Hole 2 (RR-I9-2): first party is .Module.Main, never a module path, and a
// directory replace is classified from where it points, inside -root or not.
func TestGoDeps_firstPartyIsStructural(t *testing.T) {
	base := t.TempDir()
	root := mkdirs(t, filepath.Join(base, "repo"), filepath.Join(base, "repo", "control-plane"))
	published := moduleDir(t, map[string][]byte{"LICENSE": fixture(t, "MIT.txt")})
	gplFork := mkdirs(t, filepath.Join(root, "third_party", "gpl-lib"))
	writeFile(t, filepath.Join(gplFork, "COPYING"), fixture(t, "GPL-3.0.txt"))
	outside := mkdirs(t, filepath.Join(base, "elsewhere", "lib"))
	writeFile(t, filepath.Join(outside, "LICENSE"), fixture(t, "BSD-3-Clause.txt"))

	main := &goModule{Path: "github.com/kpmg-au-emu/panacea-app/control-plane", Main: true, Dir: filepath.Join(root, "control-plane")}
	x := &goModule{Path: "github.com/kpmg-au-emu/panacea-app/x", Version: "v1.0.0", Dir: published}
	pkgs := map[string][]goPackage{
		"./control-plane/cmd/...": {
			{ImportPath: "github.com/kpmg-au-emu/panacea-app/control-plane/cmd/cp", Module: main},
			{ImportPath: "fmt", Standard: true},
			{ImportPath: "github.com/kpmg-au-emu/panacea-app/x/a", Module: x},
			{ImportPath: "github.com/kpmg-au-emu/panacea-app/x/b", Module: x},
			{ImportPath: "example.com/gpl-lib", Module: &goModule{Path: "example.com/gpl-lib", Version: "v1.2.0",
				Replace: &goModule{Path: "./third_party/gpl-lib", Dir: gplFork}}},
			{ImportPath: "example.com/outside", Module: &goModule{Path: "example.com/outside", Version: "v0.1.0",
				Replace: &goModule{Path: "../elsewhere/lib", Dir: outside}}},
		},
		"./sandboxd/cmd/...": {{ImportPath: "github.com/kpmg-au-emu/panacea-app/x/a", Module: x}},
	}
	var calls []goCall
	deps, err := GoDeps(root, goListRunner(pkgs, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != len(goScans) {
		t.Fatalf("%d go list calls, want one per scanned Go root (%d)", len(calls), len(goScans))
	}
	for _, c := range calls {
		if c.dir != root || strings.Join(c.args[:4], " ") != "go list -deps -json" {
			t.Fatalf("go list ran as %q in %q", c.args, c.dir)
		}
	}
	if len(deps) != 3 {
		t.Fatalf("GoDeps returned %d deps, want 3 (x once, gpl-lib, outside): %+v", len(deps), deps)
	}
	m := depsByName(deps)
	if d := m["github.com/kpmg-au-emu/panacea-app/x"]; d.Declared != "MIT" || d.From != "control-plane, sandboxd" {
		t.Fatalf("the published first-party-named module: %+v", d)
	}
	g := m["example.com/gpl-lib"]
	if g.Path != gplFork || g.Declared != "GPL-3.0-only" || g.Version != "v1.2.0" || !g.Enforced || g.Via != "production" {
		t.Fatalf("the vendored GPL fork inside -root: %+v", g)
	}
	if d := m["example.com/outside"]; d.Path != outside || d.Declared != "BSD-3-Clause" {
		t.Fatalf("the directory replace outside -root: %+v", d)
	}
	fs, err := Evaluate(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode(fs) != 1 {
		t.Fatal("a GPL-3.0 fork inside -root passed the gate")
	}
}

// Hole 3: a module with no licence file fails, with its why, and exit 1.
func TestGoDeps_noLicenceFileFails(t *testing.T) {
	root := t.TempDir()
	empty := moduleDir(t, map[string][]byte{"go.mod": []byte("module example.com/empty\n")})
	pkgs := map[string][]goPackage{"./canvas/...": {{ImportPath: "example.com/empty",
		Module: &goModule{Path: "example.com/empty", Version: "v1.0.0", Dir: empty}}}}
	var calls []goCall
	deps, err := GoDeps(root, goListRunner(pkgs, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].Source != "none" || deps[0].Why != "no licence file in "+empty {
		t.Fatalf("deps = %+v", deps)
	}
	fs, err := Evaluate(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fs[0].Verdict != Fail || !strings.HasPrefix(fs[0].Why, "no licence file in ") || exitCode(fs) != 1 {
		t.Fatalf("finding %+v, exit %d", fs[0], exitCode(fs))
	}
}

func TestGoDeps_runnerErrorIsAnError(t *testing.T) {
	fail := func(string, string, ...string) ([]byte, error) { return nil, errors.New("go: not found") }
	if _, err := GoDeps(t.TempDir(), fail); err == nil {
		t.Fatal("GoDeps swallowed a failed go list")
	}
}
