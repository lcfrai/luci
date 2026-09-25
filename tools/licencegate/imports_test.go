package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// importRec is one record imports.cjs prints.
type importRec struct {
	File      string `json:"file"`
	Specifier string `json:"specifier"`
	TypeOnly  bool   `json:"typeOnly"`
	Builtin   bool   `json:"builtin,omitempty"`
}

// helperRunner stands in for "node imports.cjs <dir> <file>...": it answers
// each file from byFile (keyed by the path under dir, slash form) and records
// every file it was asked about.
func helperRunner(t *testing.T, byFile map[string][]importRec, asked *[]string) Runner {
	return func(dir, name string, args ...string) ([]byte, error) {
		if name != "node" || len(args) < 3 || !strings.HasSuffix(args[0], ".cjs") {
			return nil, errors.New("unexpected command " + name)
		}
		var out []importRec
		for _, f := range args[2:] {
			rel, err := filepath.Rel(args[1], f)
			if err != nil {
				t.Errorf("file %q not under %q", f, args[1])
				continue
			}
			rel = filepath.ToSlash(rel)
			*asked = append(*asked, rel)
			for _, r := range byFile[rel] {
				r.File = f
				out = append(out, r)
			}
		}
		return json.Marshal(out)
	}
}

func spec(s ...string) []importRec {
	var rs []importRec
	for _, x := range s {
		rs = append(rs, importRec{Specifier: x})
	}
	return rs
}

func TestShippedImports(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]importRec{
		"src/main.tsx": append(spec("react", "./export/png", "@s/n/sub", "n/sub", "x?raw", "./test/helper",
			"node:fs", "virtual:pwa", "/abs.js", "\x00id", "./style.css?inline", "#internal"),
			importRec{Specifier: "t-only", TypeOnly: true}, importRec{Specifier: "fs", Builtin: true},
			// A builtin name with the npm package installed is that package.
			importRec{Specifier: "events", Builtin: true}),
		"src/export/png.ts":   spec("html-to-image"),
		"src/a.test.tsx":      spec("vitest"),
		"src/a.spec.ts":       spec("spec-only"),
		"src/test/x.ts":       spec("testdir-only"),
		"src/test/helper.ts":  spec("helper-dep"),
		"src/latest/x.ts":     spec("latest-dep"),
		"src/contest.tsx":     spec("contest-dep"),
		"src/a.mts":           spec("mts-dep"),
		"src/a.cts":           spec("cts-dep"),
		"src/a.cjs":           spec("cjs-dep"),
		"src/b.mjs":           spec("mjs-dep"),
		"src/c.jsx":           spec("jsx-dep"),
		"src/d.js":            spec("js-dep"),
		"src/notes.md":        spec("markdown-is-not-shipped"),
		"src/test/setup.ts":   spec("vitest"),
		"src/deep/test/y.tsx": spec("deep-test-only"),
	}
	for rel := range files {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte("// fixture\n"))
	}
	writeFile(t, filepath.Join(dir, "src", "style.css"),
		[]byte("/* @import \"commented\"; */\n@import \"pkg/a.css\";\n@import url(\"urlpkg/b.css\");\n@import './local.css';\n"))
	writeFile(t, filepath.Join(dir, "src", "local.css"), []byte("@import \"localpkg\";\n"))
	writeFile(t, filepath.Join(dir, "node_modules", "events", "package.json"), []byte("{}\n"))

	var asked []string
	got, err := ShippedImports(dir, filepath.Join(dir, "src"), helperRunner(t, files, &asked))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"react": "src/main.tsx", "@s/n": "src/main.tsx", "n": "src/main.tsx", "x": "src/main.tsx",
		"html-to-image": "src/export/png.ts", "helper-dep": "src/test/helper.ts",
		"latest-dep": "src/latest/x.ts", "contest-dep": "src/contest.tsx",
		"mts-dep": "src/a.mts", "cts-dep": "src/a.cts", "cjs-dep": "src/a.cjs",
		"mjs-dep": "src/b.mjs", "jsx-dep": "src/c.jsx", "js-dep": "src/d.js",
		"pkg": "src/style.css", "urlpkg": "src/style.css", "localpkg": "src/local.css",
		"events": "src/main.tsx", "#internal": "src/main.tsx", // kept: fails not installed
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ShippedImports =\n  %v\nwant\n  %v", got, want)
	}
	for _, a := range asked {
		switch a {
		case "src/a.test.tsx", "src/a.spec.ts", "src/test/x.ts", "src/test/setup.ts", "src/deep/test/y.tsx", "src/notes.md":
			t.Errorf("%s was walked: it is a test file, or not a script", a)
		}
	}
}

func TestShippedImports_helperErrorIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "a.ts"), []byte("import x from 'x'\n"))
	fail := func(string, string, ...string) ([]byte, error) { return nil, errors.New("node: not found") }
	if _, err := ShippedImports(dir, filepath.Join(dir, "src"), fail); err == nil {
		t.Fatal("ShippedImports swallowed a failed helper")
	}
}
