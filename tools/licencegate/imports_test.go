package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	return func(dir string, _ []string, name string, args ...string) ([]byte, error) {
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

// TestShippedImports_batches: every real run takes a second batch (desktop/src
// held 55 shipped script files on 2026-09-26), so a loop that stops after the
// first helperBatch files stops enforcing the rest, and the gate still exits
// 0 (fix round 1, review I4: mutant V2 survived every other test).
func TestShippedImports_batches(t *testing.T) {
	dir := t.TempDir()
	n := 2*helperBatch + 1 // three calls: two full, one holding the last file
	files := map[string][]importRec{}
	want := map[string]string{}
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("src/f%03d.ts", i)
		pkg := fmt.Sprintf("pkg-%03d", i)
		files[rel] = spec(pkg)
		want[pkg] = rel
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), []byte("// fixture\n"))
	}
	var asked []string
	var perCall []int
	inner := helperRunner(t, files, &asked)
	run := func(d string, env []string, name string, args ...string) ([]byte, error) {
		perCall = append(perCall, len(args)-2)
		return inner(d, env, name, args...)
	}
	got, err := ShippedImports(dir, filepath.Join(dir, "src"), run)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ShippedImports found %d of %d packages; the last, pkg-%03d: %q", len(got), n, n-1, got[fmt.Sprintf("pkg-%03d", n-1)])
	}
	if len(asked) != n {
		t.Errorf("imports.cjs was asked about %d files, want %d", len(asked), n)
	}
	for i, c := range perCall {
		if c > helperBatch {
			t.Errorf("call %d passed %d files, over helperBatch %d (the Windows command-line cap)", i, c, helperBatch)
		}
	}
	if len(perCall) != 3 {
		t.Errorf("imports.cjs ran %d times (%v), want 3", len(perCall), perCall)
	}
}

// TestShippedImports_cssAndURLForms is fix round 1, review M4. Vite resolves
// a CSS @import relative first (its css resolver: extensions [".css"],
// tryIndex false, preferRelative true; vite 8.3.0,
// desktop/node_modules/vite/dist/node/chunks/node.js:29701-29707, read
// 2026-09-26), so a bare "theme.css" beside theme.css is that file, walked,
// and never a package. A URL names no package but loads at run time, where
// no gate checks it ("https://esm.sh/tldraw" would ship tldraw), so it stays
// a name, whole, and NpmDeps fails it with a why that says so.
func TestShippedImports_cssAndURLForms(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "a.css"), []byte(
		"@import url(https://fonts.example.com/css2?family=Inter);\n@import \"theme.css\";\n@import \"tokens\";\n@import \"pkgcss/x.css\";\n@import \"test/helper.css\";\n"))
	writeFile(t, filepath.Join(dir, "src", "theme.css"), []byte("@import \"deeppkg\";\n"))
	// Under src/test/, so walked only because a.css imports it.
	writeFile(t, filepath.Join(dir, "src", "test", "helper.css"), []byte("@import \"helperpkg\";\n"))
	writeFile(t, filepath.Join(dir, "src", "tokens.css"), []byte("@import \"tokpkg/t.css\";\n"))
	writeFile(t, filepath.Join(dir, "src", "remote.ts"), []byte("// fixture\n"))
	files := map[string][]importRec{"src/remote.ts": spec("https://esm.sh/tldraw@3.0.0", "react")}
	var asked []string
	got, err := ShippedImports(dir, filepath.Join(dir, "src"), helperRunner(t, files, &asked))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"https://fonts.example.com/css2?family=Inter": "src/a.css",
		"pkgcss": "src/a.css", "deeppkg": "src/theme.css", "tokpkg": "src/tokens.css", "helperpkg": "src/test/helper.css",
		"https://esm.sh/tldraw@3.0.0": "src/remote.ts", "react": "src/remote.ts",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ShippedImports =\n  %v\nwant\n  %v", got, want)
	}
}

// TestIsURLSpec pins what counts as a URL: a scheme of two or more
// characters. One letter and a colon is a Windows drive, a file path, never a
// URL, so the why a person reads must not call it one (fix round 1's
// predicted survivor S2: no other test holds a drive-letter specifier).
func TestIsURLSpec(t *testing.T) {
	for _, c := range []struct {
		spec string
		want bool
	}{
		{"https://esm.sh/tldraw@3.0.0", true},
		{"http://x.example/a.js", true},
		{"data:text/javascript,export default 1", true},
		{"blob:https://x.example/0f1e", true},
		{"git+ssh://x.example/r.git", true},
		{"C:/Users/x/a.js", false},
		{`c:\x\a.js`, false},
		{"react", false},
		{"@s/n/sub", false},
		{"#internal", false},
		{"x?raw", false},
	} {
		if got := isURLSpec(c.spec); got != c.want {
			t.Errorf("isURLSpec(%q) = %v, want %v", c.spec, got, c.want)
		}
	}
}

func TestShippedImports_helperErrorIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "a.ts"), []byte("import x from 'x'\n"))
	fail := func(string, []string, string, ...string) ([]byte, error) { return nil, errors.New("node: not found") }
	if _, err := ShippedImports(dir, filepath.Join(dir, "src"), fail); err == nil {
		t.Fatal("ShippedImports swallowed a failed helper")
	}
}
