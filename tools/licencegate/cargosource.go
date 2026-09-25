package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// cargoPackage and cargoNode are the fields of cargo metadata the gate reads.
type cargoPackage struct {
	Name         string  `json:"name"`
	Version      string  `json:"version"`
	ID           string  `json:"id"`
	License      *string `json:"license"`
	LicenseFile  *string `json:"license_file"`
	ManifestPath string  `json:"manifest_path"`
	Targets      []struct {
		Kind []string `json:"kind"`
	} `json:"targets"`
}

type cargoNode struct {
	ID   string `json:"id"`
	Deps []struct {
		Pkg      string `json:"pkg"`
		DepKinds []struct {
			Kind *string `json:"kind"`
		} `json:"dep_kinds"`
	} `json:"deps"`
}

type cargoMetadata struct {
	Packages         []cargoPackage `json:"packages"`
	WorkspaceMembers []string       `json:"workspace_members"`
	Resolve          struct {
		Nodes []cargoNode `json:"nodes"`
	} `json:"resolve"`
}

// cargoSlash is Cargo's deprecated "/" licence separator, spaced or not.
var cargoSlash = regexp.MustCompile(`\s*/\s*`)

// CargoDeps reports one Cargo workspace (review I10), run in dir:
//
//	cargo metadata --offline --locked --format-version 1 --filter-platform x86_64-pc-windows-msvc
//
// Every package whose id is not in workspace_members is a Dep, Enforced =
// CargoEnforced, Declared its "license" with "/" read as OR (Cargo documents
// it so; ParseSPDX is unchanged), or "" with Source "license_file: <path>".
// Why is the crate's first chain from a member through normal edges, and
// "(proc-macro)" when its only target is a proc-macro. With CargoEnforced
// false, cargo missing or failing gives notScanned and err nil: the run
// prints "REPORT cargo: not scanned: <error>"; with it true, that is err.
func CargoDeps(root, dir string, run Runner) (deps []Dep, notScanned string, err error) {
	fail := func(e error) ([]Dep, string, error) {
		if CargoEnforced {
			return nil, "", e
		}
		return nil, e.Error(), nil
	}
	out, err := run(dir, "cargo", "metadata", "--offline", "--locked", "--format-version", "1",
		"--filter-platform", "x86_64-pc-windows-msvc")
	if err != nil {
		return fail(fmt.Errorf("cargo metadata in %s: %w", dir, err))
	}
	var m cargoMetadata
	if err := json.Unmarshal(out, &m); err != nil {
		return fail(fmt.Errorf("cargo metadata in %s: %w", dir, err))
	}
	from := filepath.Base(dir)
	if rel, err := filepath.Rel(root, dir); err == nil && !strings.HasPrefix(rel, "..") {
		from = filepath.ToSlash(rel)
	}
	byID := map[string]cargoPackage{}
	for _, p := range m.Packages {
		byID[p.ID] = p
	}
	chains := cargoChains(m, byID)
	members := map[string]bool{}
	for _, id := range m.WorkspaceMembers {
		members[id] = true
	}
	for _, p := range m.Packages {
		if members[p.ID] {
			continue
		}
		d := Dep{Ecosystem: Cargo, Name: p.Name, Version: p.Version, Source: "Cargo.toml",
			Path: filepath.Dir(p.ManifestPath), Enforced: CargoEnforced, From: from}
		switch {
		case p.License != nil && *p.License != "":
			d.Declared = cargoSlash.ReplaceAllString(*p.License, " OR ")
		case p.LicenseFile != nil:
			d.Source = "license_file: " + *p.LicenseFile
		default:
			d.Source = "none"
		}
		d.Why = chains[p.ID]
		if isProcMacroOnly(p) {
			d.Why += " (proc-macro)"
		}
		deps = append(deps, d)
	}
	return deps, "", nil
}

// isProcMacroOnly: the crate's library target is a proc-macro, which runs at
// compile time and does not link. Test, bench, example and build-script
// targets are ignored, because none links into a dependent: yoke-derive 0.8.3
// is [proc-macro] plus an [example], displaydoc 0.2.7 [proc-macro] plus
// [test]s (read 2026-09-26). Any other kind (lib, rlib, bin, …) is not.
func isProcMacroOnly(p cargoPackage) bool {
	macros := 0
	for _, t := range p.Targets {
		for _, k := range t.Kind {
			switch k {
			case "test", "bench", "example", "custom-build":
			case "proc-macro":
				macros++
			default:
				return false
			}
		}
	}
	return macros > 0
}

// cargoChains is each package's why: "reached by a 1.0 → b 2.0", the first
// chain breadth-first from a workspace member through normal edges (kind
// null) to the crate's parent; "a direct dependency of <member>"; or
// "reached through no normal edge (build or dev only)". Approximate: cargo
// metadata unifies features across build and normal use.
func cargoChains(m cargoMetadata, byID map[string]cargoPackage) map[string]string {
	edges := map[string][]string{}
	for _, n := range m.Resolve.Nodes {
		for _, d := range n.Deps {
			for _, k := range d.DepKinds {
				if k.Kind == nil {
					edges[n.ID] = append(edges[n.ID], d.Pkg)
					break
				}
			}
		}
	}
	label := func(id string) string { return byID[id].Name + " " + byID[id].Version }
	parent := map[string]string{}
	seen := map[string]bool{}
	queue := append([]string(nil), m.WorkspaceMembers...)
	for _, id := range queue {
		seen[id] = true
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, c := range edges[id] {
			if !seen[c] {
				seen[c] = true
				parent[c] = id
				queue = append(queue, c)
			}
		}
	}
	members := map[string]bool{}
	for _, id := range m.WorkspaceMembers {
		members[id] = true
	}
	why := map[string]string{}
	for id := range byID {
		p, ok := parent[id]
		switch {
		case members[id]:
		case !ok:
			why[id] = "reached through no normal edge (build or dev only)"
		case members[p]:
			why[id] = "a direct dependency of " + label(p)
		default:
			var chain []string
			for q := p; !members[q]; q = parent[q] {
				chain = append([]string{label(q)}, chain...)
			}
			why[id] = "reached by " + strings.Join(chain, " → ")
		}
	}
	return why
}
