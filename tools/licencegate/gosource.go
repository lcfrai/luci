package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Runner runs a command in a directory and returns stdout; tests inject one.
type Runner func(dir string, name string, args ...string) ([]byte, error)

// goScan is one shipped Go root: the go list pattern, and the From it marks.
type goScan struct{ pattern, from string }

// goScans are the shipped Go packages. tools/ is not among them: the gate is
// a CI tool and never ships.
var goScans = []goScan{
	{"./canvas/...", "canvas"},
	{"./control-plane/cmd/...", "control-plane"},
	{"./sandboxd/cmd/...", "sandboxd"},
}

// goPackage and goModule are the fields of "go list -json" the gate reads.
type goPackage struct {
	ImportPath string
	Standard   bool      `json:",omitempty"`
	Module     *goModule `json:",omitempty"`
}

type goModule struct {
	Path    string
	Version string    `json:",omitempty"`
	Main    bool      `json:",omitempty"`
	Dir     string    `json:",omitempty"`
	Replace *goModule `json:",omitempty"`
}

// GoDeps lists every module linked into the shipped packages, one
//
//	go list -deps -json <pattern>
//
// per goScans root, run from the repository root (under go.work, every
// workspace module is Main). It keeps each package's .Module, never the
// standard library (.Standard) and never a Main module (first party is
// structural: review I9), de-duplicated by path@version with every From that
// reached it, and classifies .Module.Replace.Dir when .Module.Replace is set
// (a directory replace included, wherever it points: re-review RR-I9-2),
// else .Module.Dir. All Enforced. A module whose directory holds no licence
// file fails with "no licence file in <dir>".
func GoDeps(root string, run Runner) ([]Dep, error) {
	var deps []Dep
	index := map[string]int{}
	for _, s := range goScans {
		out, err := run(root, "go", "list", "-deps", "-json", s.pattern)
		if err != nil {
			return nil, fmt.Errorf("go list %s: %w", s.pattern, err)
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var p goPackage
			if err := dec.Decode(&p); err == io.EOF {
				break
			} else if err != nil {
				return nil, fmt.Errorf("go list %s: %w", s.pattern, err)
			}
			if p.Standard || p.Module == nil || p.Module.Main {
				continue
			}
			m := p.Module
			version, dir := m.Version, m.Dir
			if m.Replace != nil {
				dir = m.Replace.Dir
				if m.Replace.Version != "" {
					version = m.Replace.Version
				}
			}
			key := m.Path + "@" + version
			if i, ok := index[key]; ok {
				if !strings.Contains(", "+deps[i].From+", ", ", "+s.from+", ") {
					deps[i].From += ", " + s.from
				}
				continue
			}
			if dir == "" {
				return nil, fmt.Errorf("go list %s: module %s has no directory (not in the module cache?)", s.pattern, key)
			}
			expr, source, err := Classify(dir)
			if err != nil {
				return nil, fmt.Errorf("module %s: %w", key, err)
			}
			d := Dep{Ecosystem: Go, Name: m.Path, Version: version, Declared: expr, Source: source,
				Path: dir, Enforced: true, Via: "production", From: s.from}
			if source == "none" {
				d.Why = "no licence file in " + dir
			}
			index[key] = len(deps)
			deps = append(deps, d)
		}
	}
	return deps, nil
}
