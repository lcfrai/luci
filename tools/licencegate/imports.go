package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// importsHelper is imports.cjs, embedded so the gate runs from any -root
// and any working directory; it is written to a temporary file per call.
//
//go:embed imports.cjs
var importsHelper []byte

// scriptExts are the eight script forms Vite bundles (re-review RR-M4).
var scriptExts = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

// helperBatch caps the files per node call: Windows caps a command line at
// 32767 characters, and desktop/src holds hundreds of files.
const helperBatch = 40

func isScript(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	for _, e := range scriptExts {
		if ext == e {
			return true
		}
	}
	return false
}

func isCSS(p string) bool { return strings.EqualFold(filepath.Ext(p), ".css") }

// isTestFile: the name matches *.test.* or *.spec.*, or the file lies under
// a directory named exactly "test" (not "latest", not "contest").
func isTestFile(srcDir, p string) bool {
	base := strings.ToLower(filepath.Base(p))
	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	rel, err := filepath.Rel(srcDir, filepath.Dir(p))
	if err != nil {
		return false
	}
	for _, el := range strings.Split(filepath.ToSlash(rel), "/") {
		if el == "test" {
			return true
		}
	}
	return false
}

// packageName is the installed package a bare specifier names: "@s/n/sub"
// names "@s/n", "n/sub" names "n", and a "?query" is dropped. ok is false
// for every specifier that names no package. A "#x" subpath import is not
// skipped: package.json "imports" can map it to any package, so it stays a
// name, finds nothing installed, and fails "not installed" (fail closed; no
// shipped file uses one today).
func packageName(spec string) (string, bool) {
	s, _, _ := strings.Cut(spec, "?")
	for _, p := range []string{".", "/", "node:", "virtual:", "\x00"} {
		if strings.HasPrefix(s, p) {
			return "", false
		}
	}
	if s == "" {
		return "", false
	}
	parts := strings.Split(s, "/")
	if strings.HasPrefix(s, "@") {
		if len(parts) < 2 || parts[1] == "" {
			return "", false
		}
		return parts[0] + "/" + parts[1], true
	}
	return parts[0], true
}

// resolveRelative finds the file a relative specifier from file names, the
// way Vite's resolver would: the path itself, with each script extension, a
// ".js"-style name standing for its TypeScript source, then an index file.
func resolveRelative(file, spec string) (string, bool) {
	s, _, _ := strings.Cut(spec, "?")
	base := filepath.Join(filepath.Dir(file), filepath.FromSlash(s))
	var tries []string
	tries = append(tries, base)
	for _, e := range scriptExts {
		tries = append(tries, base+e)
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	switch strings.ToLower(ext) {
	case ".js", ".jsx":
		tries = append(tries, stem+".ts", stem+".tsx")
	case ".mjs":
		tries = append(tries, stem+".mts")
	case ".cjs":
		tries = append(tries, stem+".cts")
	}
	for _, e := range scriptExts {
		tries = append(tries, filepath.Join(base, "index"+e))
	}
	for _, t := range tries {
		if fi, err := os.Stat(t); err == nil && !fi.IsDir() {
			return filepath.Clean(t), true
		}
	}
	return "", false
}

var (
	cssComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssQuoted   = regexp.MustCompile(`@import\s+(?:url\(\s*)?["']([^"']+)["']`)
	cssUnquoted = regexp.MustCompile(`@import\s+url\(\s*([^"'\s)]+)\s*\)`)
)

// cssImports are a stylesheet's @import "x" and @import url("x")
// specifiers, after /* */ comments are removed.
func cssImports(b []byte) []string {
	text := cssComment.ReplaceAllString(string(b), " ")
	var out []string
	for _, re := range []*regexp.Regexp{cssQuoted, cssUnquoted} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// importRecord is one record imports.cjs prints. Builtin marks a Node core
// module ("fs"), which Vite replaces with an empty browser stub: no package.
type importRecord struct {
	File      string `json:"file"`
	Specifier string `json:"specifier"`
	TypeOnly  bool   `json:"typeOnly"`
	Builtin   bool   `json:"builtin"`
}

func within(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ShippedImports returns each bare package name the shipped source of one
// npm project imports, with the first file (relative to dir, slash form)
// that imports it (review I9, hole 1). The walk starts at every script and
// .css file under srcDir that is not a test file, and follows relative
// imports to a fixed point, so a test helper that shipped code imports is
// walked too. Script specifiers come from imports.cjs, which skips nothing
// itself: here `import type` / `export type` (typeOnly) and a Node builtin
// with no package of its name installed are dropped; `import { type A, b }`
// is kept, because b ships.
func ShippedImports(dir, srcDir string, run Runner) (map[string]string, error) {
	var queue []string
	walked := map[string]bool{}
	err := filepath.WalkDir(srcDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if (isScript(p) || isCSS(p)) && !isTestFile(srcDir, p) {
			walked[p] = true
			queue = append(queue, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", srcDir, err)
	}
	helper, err := os.CreateTemp("", "licencegate-imports-*.cjs")
	if err != nil {
		return nil, err
	}
	defer os.Remove(helper.Name())
	if _, err := helper.Write(importsHelper); err != nil {
		helper.Close()
		return nil, err
	}
	if err := helper.Close(); err != nil {
		return nil, err
	}
	found := map[string]string{}
	for len(queue) > 0 {
		sort.Strings(queue)
		round := queue
		queue = nil
		var specs []importRecord
		var scripts []string
		for _, f := range round {
			if !isCSS(f) {
				scripts = append(scripts, f)
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			for _, s := range cssImports(b) {
				specs = append(specs, importRecord{File: f, Specifier: s})
			}
		}
		for i := 0; i < len(scripts); i += helperBatch {
			batch := scripts[i:min(i+helperBatch, len(scripts))]
			out, err := run(dir, "node", append([]string{helper.Name(), dir}, batch...)...)
			if err != nil {
				return nil, fmt.Errorf("imports.cjs in %s: %w", dir, err)
			}
			var recs []importRecord
			if err := json.Unmarshal(out, &recs); err != nil {
				return nil, fmt.Errorf("imports.cjs in %s: %w", dir, err)
			}
			for _, r := range recs {
				if r.TypeOnly {
					continue
				}
				// Vite resolves a package first and stubs a builtin only
				// when none is installed: "events" can be the npm package.
				if r.Builtin {
					if _, err := os.Stat(filepath.Join(dir, "node_modules", filepath.FromSlash(r.Specifier))); err != nil {
						continue
					}
				}
				specs = append(specs, r)
			}
		}
		sort.SliceStable(specs, func(i, j int) bool { return specs[i].File < specs[j].File })
		for _, s := range specs {
			if strings.HasPrefix(s.Specifier, ".") {
				t, ok := resolveRelative(s.File, s.Specifier)
				if ok && !walked[t] && within(dir, t) && (isScript(t) || isCSS(t)) {
					walked[t] = true
					queue = append(queue, t)
				}
				continue
			}
			if name, ok := packageName(s.Specifier); ok {
				if _, seen := found[name]; !seen {
					rel, err := filepath.Rel(dir, s.File)
					if err != nil {
						return nil, err
					}
					found[name] = filepath.ToSlash(rel)
				}
			}
		}
	}
	return found, nil
}
