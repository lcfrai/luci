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
	env  []string
	args []string
}

// goListRunner answers "go list" with a JSON stream per pattern, the same
// under every target, and records every call.
func goListRunner(byPattern map[string][]goPackage, calls *[]goCall) Runner {
	return goListRunnerOn(byPattern, nil, calls)
}

// goListRunnerOn is goListRunner plus, for each NAME=value pair a call's
// environment holds ("GOOS=windows", "CGO_ENABLED=1"), extra[pair]'s
// packages for the pattern: what go list adds under that setting alone.
func goListRunnerOn(byPattern map[string][]goPackage, extra map[string]map[string][]goPackage, calls *[]goCall) Runner {
	return func(dir string, env []string, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, goCall{dir, env, append([]string{name}, args...)})
		if name != "go" || len(args) < 4 || args[0] != "list" {
			return nil, errors.New("unexpected command " + name + " " + strings.Join(args, " "))
		}
		pattern := args[len(args)-1]
		pkgs := append([]goPackage(nil), byPattern[pattern]...)
		for _, e := range env {
			pkgs = append(pkgs, extra[e][pattern]...)
		}
		var out []byte
		for _, p := range pkgs {
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
	if len(calls) != len(goScans)*len(goTargets) {
		t.Fatalf("%d go list calls, want one per scanned Go root per target (%d)", len(calls), len(goScans)*len(goTargets))
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

// TestGoDeps_everyTarget is fix round 1, review I2: go list selects files by
// GOOS, GOARCH and CGO_ENABLED, so a listing under the host's environment
// alone missed github.com/Microsoft/go-winio on the ubuntu agent and
// github.com/creack/pty on Windows (measured by the review). Every root is
// listed under every goTargets environment and the modules unioned, so a
// GPL module reached only from a windows file fails on any host.
func TestGoDeps_everyTarget(t *testing.T) {
	mit := moduleDir(t, map[string][]byte{"LICENSE": fixture(t, "MIT.txt")})
	gpl := moduleDir(t, map[string][]byte{"COPYING": fixture(t, "GPL-3.0.txt")})
	both := goPackage{ImportPath: "example.com/both", Module: &goModule{Path: "example.com/both", Version: "v1.0.0", Dir: mit}}
	pkgs := map[string][]goPackage{"./control-plane/cmd/...": {both}, "./sandboxd/cmd/...": {both}}
	extra := map[string]map[string][]goPackage{
		"GOOS=windows": {"./control-plane/cmd/...": {{ImportPath: "example.com/wingpl",
			Module: &goModule{Path: "example.com/wingpl", Version: "v0.6.2", Dir: gpl}}}},
		"GOOS=linux": {"./sandboxd/cmd/...": {{ImportPath: "example.com/pty",
			Module: &goModule{Path: "example.com/pty", Version: "v1.1.24", Dir: mit}}}},
		"CGO_ENABLED=1": {"./canvas/...": {{ImportPath: "example.com/cgoonly",
			Module: &goModule{Path: "example.com/cgoonly", Version: "v2.0.0", Dir: mit}}}},
	}
	root := t.TempDir()
	var calls []goCall
	deps, err := GoDeps(root, goListRunnerOn(pkgs, extra, &calls))
	if err != nil {
		t.Fatal(err)
	}
	m := depsByName(deps)
	if len(deps) != 4 || m["example.com/both"].From != "control-plane, sandboxd" {
		t.Errorf("deps = %+v: want both (once, from control-plane and sandboxd), wingpl, pty and cgoonly", deps)
	}
	for _, n := range []string{"example.com/wingpl", "example.com/pty", "example.com/cgoonly"} {
		if _, ok := m[n]; !ok {
			t.Errorf("%s, listed under one target only, is missing", n)
		}
	}
	fs, err := Evaluate(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode(fs) != 1 {
		t.Error("a GPL module reached only from a windows build passed the gate")
	}
	// Each root under each target exactly once, its environment naming the
	// platform and cgo, never left to the host.
	seen := map[string]int{}
	for _, c := range calls {
		env := strings.Join(c.env, " ")
		for _, w := range []string{"GOOS=", "GOARCH=", "CGO_ENABLED="} {
			if !strings.Contains(env, w) {
				t.Errorf("go list %s ran with env %q: no %s", c.args[len(c.args)-1], env, w)
			}
		}
		seen[c.args[len(c.args)-1]+" | "+env]++
	}
	for _, s := range goScans {
		for _, want := range []string{"GOOS=windows GOARCH=amd64 CGO_ENABLED=0", "GOOS=windows GOARCH=amd64 CGO_ENABLED=1",
			"GOOS=linux GOARCH=amd64 CGO_ENABLED=0", "GOOS=linux GOARCH=amd64 CGO_ENABLED=1"} {
			if n := seen[s.pattern+" | "+want]; n != 1 {
				t.Errorf("go list %s under %s ran %d times, want 1", s.pattern, want, n)
			}
		}
	}
}

// TestGoDeps_replaceOntoDenied is fix round 1, review M3: Denied saw only
// the replaced path, so "replace example.com/x => github.com/tldraw/tldraw-go
// v1.0.0" was judged on the replacement's licence text alone.
func TestGoDeps_replaceOntoDenied(t *testing.T) {
	mit := moduleDir(t, map[string][]byte{"LICENSE": fixture(t, "MIT.txt")})
	pkgs := map[string][]goPackage{"./canvas/...": {
		{ImportPath: "example.com/x", Module: &goModule{Path: "example.com/x", Version: "v0.1.0",
			Replace: &goModule{Path: "github.com/TLDraw/tldraw-go", Version: "v1.0.0", Dir: mit}}},
		{ImportPath: "example.com/y", Module: &goModule{Path: "example.com/y", Version: "v0.1.0",
			Replace: &goModule{Path: "github.com/tldrawx/y", Version: "v1.0.0", Dir: mit}}},
	}}
	var calls []goCall
	deps, err := GoDeps(t.TempDir(), goListRunner(pkgs, &calls))
	if err != nil {
		t.Fatal(err)
	}
	fs, err := Evaluate(deps, []Exception{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Finding{}
	for _, f := range fs {
		got[f.Dep.Name] = f
	}
	if f := got["example.com/x"]; f.Verdict != Fail || !strings.Contains(f.Why, "replaced by github.com/TLDraw/tldraw-go v1.0.0") || !strings.Contains(f.Why, "OD-11") {
		t.Errorf("a replace onto tldraw = %+v: want fail, naming the replacement and OD-11", f)
	}
	if f := got["example.com/y"]; f.Verdict != Pass {
		t.Errorf("a replace onto github.com/tldrawx/y (MIT) = %+v: want pass, the deny match is exact", f)
	}
}

func TestGoDeps_runnerErrorIsAnError(t *testing.T) {
	fail := func(string, []string, string, ...string) ([]byte, error) { return nil, errors.New("go: not found") }
	if _, err := GoDeps(t.TempDir(), fail); err == nil {
		t.Fatal("GoDeps swallowed a failed go list")
	}
}
