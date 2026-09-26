package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Runner runs a command in a directory and returns stdout; tests inject one.
// env is NAME=value pairs set over the process's own environment (nil: none):
// go list needs it, because GOOS and GOARCH have no flag (fix round 1, I2).
type Runner func(dir string, env []string, name string, args ...string) ([]byte, error)

// goScan is one shipped Go root: the go list pattern, and the From it marks.
type goScan struct{ pattern, from string }

// goScans are the shipped Go packages. tools/ is not among them: the gate is
// a CI tool and never ships.
var goScans = []goScan{
	{"./canvas/...", "canvas"},
	{"./control-plane/cmd/...", "control-plane"},
	{"./sandboxd/cmd/...", "sandboxd"},
}

// goTarget is one platform a shipped Go binary is built for, and the go
// list environment that selects its files.
type goTarget struct {
	name string
	env  []string
}

// goTargets are the platforms this program ships Go binaries for (fix round
// 1, review I2): windows/amd64, the control plane on the owner's machines
// (github.com/Microsoft/go-winio, "which Windows builds need":
// docs/superpowers/plans/phase-2-parts/task-05.md:1428), and linux/amd64,
// sandboxd in the sandbox image (images/sandbox/Dockerfile:1, :8) and the
// ubuntu CI agent. Each is listed with cgo off and on: a cross listing turns
// cgo off by default and a native one turns it on where a C compiler is
// found, so without both the set would still depend on the host, and a
// module reached only from a cgo file must not pass unseen. GoDeps lists
// every root under every one and unions the modules, so the enforced set is
// the same on any host.
var goTargets = []goTarget{
	{"windows/amd64", []string{"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0"}},
	{"windows/amd64 cgo", []string{"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=1"}},
	{"linux/amd64", []string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0"}},
	{"linux/amd64 cgo", []string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1"}},
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
// per goScans root per goTargets platform (its environment set: fix round 1,
// review I2), run from the repository root (under go.work, every workspace
// module is Main). It keeps each package's .Module, never the standard
// library (.Standard) and never a Main module (first party is structural:
// review I9), de-duplicated by path@version across roots and platforms with
// every From that reached it, and classifies .Module.Replace.Dir when
// .Module.Replace is set (a directory replace included, wherever it points:
// re-review RR-I9-2), else .Module.Dir. All Enforced. A module whose
// directory holds no licence file fails with "no licence file in <dir>". A
// module replaced by a denied module path fails as denied, whatever the
// replacement's licence text reads (fix round 1, review M3).
func GoDeps(root string, run Runner) ([]Dep, error) {
	var deps []Dep
	index := map[string]int{}
	for _, s := range goScans {
		for _, tg := range goTargets {
			out, err := run(root, tg.env, "go", "list", "-deps", "-json", s.pattern)
			if err != nil {
				return nil, fmt.Errorf("go list %s (%s): %w", s.pattern, tg.name, err)
			}
			dec := json.NewDecoder(bytes.NewReader(out))
			for {
				var p goPackage
				if err := dec.Decode(&p); err == io.EOF {
					break
				} else if err != nil {
					return nil, fmt.Errorf("go list %s (%s): %w", s.pattern, tg.name, err)
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
					return nil, fmt.Errorf("go list %s (%s): module %s has no directory (not in the module cache?)", s.pattern, tg.name, key)
				}
				expr, source, err := Classify(dir)
				if err != nil {
					return nil, fmt.Errorf("module %s: %w", key, err)
				}
				d := Dep{Ecosystem: Go, Name: m.Path, Version: version, Declared: expr, Source: source,
					Path: dir, Enforced: true, Via: "production", From: s.from}
				switch {
				case m.Replace != nil && m.Replace.Version != "" && Denied(Go, m.Replace.Path):
					d.Broken, d.Why = true, "replaced by "+m.Replace.Path+" "+m.Replace.Version+": "+deniedWhy
				case source == "none":
					d.Why = "no licence file in " + dir
				}
				index[key] = len(deps)
				deps = append(deps, d)
			}
		}
	}
	return deps, nil
}
