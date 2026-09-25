package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// cargoMeta composes cargo metadata --format-version 1 output in the shape
// the real one has (read 2026-09-25 in desktop/src-tauri: packages with id,
// license, license_file, source, targets[].kind and manifest_path;
// workspace_members; resolve.nodes[].deps[] with pkg and dep_kinds[].kind,
// null for a normal edge).
type cargoMeta struct {
	root     string
	packages []map[string]any
	nodes    map[string][]map[string]any
	order    []string
}

func (m *cargoMeta) pkg(name, version string, license any, kinds ...string) string {
	id := "registry+https://github.com/rust-lang/crates.io-index#" + name + "@" + version
	var source any = "registry+https://github.com/rust-lang/crates.io-index"
	manifest := filepath.Join(m.root, "registry", name+"-"+version, "Cargo.toml")
	if license == "path" {
		id = "path+file:///" + filepath.ToSlash(m.root) + "/crates/" + name + "#" + name + "@" + version
		source, license = nil, nil
		manifest = filepath.Join(m.root, "crates", name, "Cargo.toml")
	}
	// Each argument is one target; "+" joins the kinds of one target, as the
	// real member's ["staticlib", "cdylib", "rlib"].
	if len(kinds) == 0 {
		kinds = []string{"lib"}
	}
	var targets []map[string]any
	for _, k := range kinds {
		targets = append(targets, map[string]any{"kind": strings.Split(k, "+")})
	}
	p := map[string]any{"name": name, "version": version, "id": id, "license": license,
		"license_file": nil, "source": source, "manifest_path": manifest, "targets": targets}
	if license == nil && source == nil {
		p["license_file"] = "LICENSE.txt"
	}
	m.packages = append(m.packages, p)
	m.order = append(m.order, id)
	return id
}

// dep adds an edge from -> to of kind ("" is normal: kind null).
func (m *cargoMeta) dep(from, to, kind string) {
	var k any
	if kind != "" {
		k = kind
	}
	m.nodes[from] = append(m.nodes[from], map[string]any{"name": to, "pkg": to,
		"dep_kinds": []map[string]any{{"kind": k, "target": nil}}})
}

func (m *cargoMeta) json(member string) []byte {
	var nodes []map[string]any
	for _, id := range m.order {
		deps := m.nodes[id]
		if deps == nil {
			deps = []map[string]any{}
		}
		nodes = append(nodes, map[string]any{"id": id, "deps": deps})
	}
	b, _ := json.Marshal(map[string]any{"packages": m.packages, "workspace_members": []string{member},
		"resolve": map[string]any{"nodes": nodes, "root": member}, "version": 1})
	return b
}

var cargoArgs = []string{"metadata", "--offline", "--locked", "--format-version", "1", "--filter-platform", "x86_64-pc-windows-msvc"}

func TestCargoDeps(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "desktop", "src-tauri")
	m := &cargoMeta{root: root, nodes: map[string][]map[string]any{}}
	desktop := m.pkg("desktop", "0.1.0", "path", "staticlib+cdylib+rlib", "bin", "custom-build")
	tauri := m.pkg("tauri", "2.11.5", "Apache-2.0 OR MIT")
	utils := m.pkg("tauri-utils", "2.9.3", "Apache-2.0 OR MIT")
	dom := m.pkg("dom_query", "0.27.0", "MIT")
	css := m.pkg("cssparser", "0.36.0", "MPL-2.0", "lib", "custom-build")
	macros := m.pkg("cssparser-macros", "0.6.1", "MPL-2.0", "proc-macro")
	// Read 2026-09-26: yoke-derive 0.8.3 is [proc-macro] plus an [example]
	// target, and displaydoc 0.2.7 [proc-macro] plus [test] targets; neither
	// links, so the mark ignores test, bench, example and build-script targets.
	yoke := m.pkg("yoke-derive", "0.8.3", "Unicode-3.0", "proc-macro", "example")
	m.dep(dom, yoke, "")
	same := m.pkg("same-file", "1.0.6", "Unlicense/MIT")
	fnv := m.pkg("fnv", "1.0.7", "Apache-2.0 / MIT")
	brotli := m.pkg("brotli-decompressor", "5.0.3", "BSD-3-Clause/MIT")
	local := m.pkg("local-helper", "0.1.0", "path")
	build := m.pkg("build-only", "1.0.0", "GPL-3.0")
	m.dep(desktop, tauri, "")
	m.dep(desktop, local, "")
	m.dep(tauri, build, "build")
	m.dep(tauri, utils, "")
	m.dep(tauri, fnv, "")
	m.dep(utils, dom, "")
	m.dep(utils, same, "")
	m.dep(dom, css, "")
	m.dep(css, macros, "")
	m.dep(css, brotli, "")
	var calls []string
	deps, notScanned, err := CargoDeps(root, dir, cargoRunner(m.json(desktop), nil, &calls))
	if err != nil || notScanned != "" {
		t.Fatalf("CargoDeps: %v, notScanned %q", err, notScanned)
	}
	if want := []string{dir + " cargo " + strings.Join(cargoArgs, " ")}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
	got := map[string]Dep{}
	for _, d := range deps {
		if d.Ecosystem != Cargo || d.Enforced != CargoEnforced || d.From != "desktop/src-tauri" {
			t.Errorf("%s: %+v: want cargo, Enforced = CargoEnforced, from desktop/src-tauri", d.Name, d)
		}
		got[d.Name] = d
	}
	if _, ok := got["desktop"]; ok || len(got) != 11 {
		t.Errorf("deps = %v: want the 11 non-members, the member skipped", got)
	}
	fs := verdicts(t, deps)
	want := map[string]Verdict{"cssparser@0.36.0": Report, "cssparser-macros@0.6.1": Report,
		"same-file@1.0.6": Pass, "fnv@1.0.7": Pass, "brotli-decompressor@5.0.3": Pass,
		"local-helper@0.1.0": Report, "build-only@1.0.0": Report, "tauri@2.11.5": Pass}
	for k, w := range want {
		if fs[k].Verdict != w {
			t.Errorf("%s: %q (%s), want %q", k, fs[k].Verdict, fs[k].Dep.Declared, w)
		}
	}
	if d := got["same-file"].Declared; d != "Unlicense OR MIT" {
		t.Errorf("Unlicense/MIT read as %q", d)
	}
	if d := got["local-helper"]; d.Declared != "" || d.Source != "license_file: LICENSE.txt" {
		t.Errorf("a path crate with only license_file = %+v", d)
	}
	if w := fs["cssparser@0.36.0"].Why; w != "reached by tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0" {
		t.Errorf("cssparser why = %q", w)
	}
	if w := fs["cssparser-macros@0.6.1"].Why; !strings.Contains(w, "dom_query 0.27.0 → cssparser 0.36.0") || !strings.HasSuffix(w, "(proc-macro)") {
		t.Errorf("cssparser-macros why = %q", w)
	}
	if w := fs["yoke-derive@0.8.3"].Why; !strings.HasSuffix(w, "dom_query 0.27.0 (proc-macro)") {
		t.Errorf("yoke-derive (proc-macro and an example target) why = %q", w)
	}
	if w := fs["build-only@1.0.0"].Why; !strings.Contains(w, "no normal edge") {
		t.Errorf("build-only why = %q", w)
	}
	if exitCode(mustEvaluate(t, deps)) != 0 {
		t.Error("a Cargo report changed the exit code while CargoEnforced is false")
	}

	t.Run("cargo failing is not scanned, never an abort", func(t *testing.T) {
		var calls []string
		deps, notScanned, err := CargoDeps(root, dir, cargoRunner(nil, errors.New(`exec: "cargo": executable file not found`), &calls))
		if err != nil || len(deps) != 0 || !strings.Contains(notScanned, "cargo") {
			t.Fatalf("= %v, %q, %v: want no deps, notScanned set, err nil", deps, notScanned, err)
		}
	})
}

func mustEvaluate(t *testing.T, deps []Dep) []Finding {
	t.Helper()
	fs, err := Evaluate(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func cargoRunner(out []byte, err error, calls *[]string) Runner {
	return func(dir, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, dir+" "+name+" "+strings.Join(args, " "))
		if name != "cargo" || !reflect.DeepEqual(args, cargoArgs) {
			return nil, errors.New("unexpected command " + name + " " + strings.Join(args, " "))
		}
		return out, err
	}
}
