package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// isLinkMode: a symlink, or a Windows junction, which Lstat reports as
// ModeIrregular since Go 1.23 (a mount point no longer sets ModeSymlink).
func isLinkMode(m fs.FileMode) bool { return m&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// refuseLinkedModules is carried RR2-1. In a worktree whose node_modules is a
// junction into the main tree, Node and Vite resolve the main tree's packages
// (its canvas/editor, not this one's), and npm ls exits 1 once a file:
// dependency is installed. Reading npm's JSON anyway would clear source that
// is not what ships, so the gate refuses, names the link and the remedy.
func refuseLinkedModules(dir string) error {
	nm := filepath.Join(dir, "node_modules")
	fi, err := os.Lstat(nm)
	if err != nil {
		return fmt.Errorf("%s: %w: the gate reads this project's installed tree; run npm ci in %s", nm, err, dir)
	}
	if !isLinkMode(fi.Mode()) {
		return nil
	}
	target, err := os.Readlink(nm)
	if err != nil {
		target = "another tree (" + err.Error() + ")"
	}
	return fmt.Errorf("%s is a link into %s: the gate reads this project's own installed tree; remove the link (never rm -rf through it) and run npm ci in %s", nm, target, dir)
}

// lockEntry is what package-lock.json states about one location: the only
// place the gate reads a link from (re-review RR-I9-1).
type lockEntry struct {
	Resolved string `json:"resolved"`
	Link     bool   `json:"link"`
}

func readLock(dir string) (map[string]lockEntry, error) {
	p := filepath.Join(dir, "package-lock.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("npm project %s: %w", dir, err)
	}
	var l struct {
		Packages map[string]lockEntry `json:"packages"`
	}
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return l.Packages, nil
}

// lsEntry is one node of npm ls --json --long.
type lsEntry struct {
	Version      string              `json:"version"`
	Path         string              `json:"path"`
	Missing      bool                `json:"missing"`
	Dependencies map[string]*lsEntry `json:"dependencies"`
}

// npmNode is one installed node, keyed by the canon form of its path.
type npmNode struct {
	name, version, path string
	kids                map[string]bool
}

// npmGraph holds every node of one npm ls tree, a node printed at one path
// under two parents being one node with its children unioned.
type npmGraph map[string]*npmNode

func (g npmGraph) add(name string, e *lsEntry) (string, error) {
	if e.Path == "" || e.Missing {
		return "", fmt.Errorf("npm ls lists %s with no installed path: run npm ci", name)
	}
	path, err := unredact(e.Path)
	if err != nil {
		return "", fmt.Errorf("npm ls node %s: %w", name, err)
	}
	key, err := canon(path)
	if err != nil {
		return "", fmt.Errorf("npm ls node %s: %w", name, err)
	}
	n := g[key]
	if n == nil {
		n = &npmNode{name: name, version: e.Version, path: path, kids: map[string]bool{}}
		g[key] = n
	}
	names := make([]string, 0, len(e.Dependencies))
	for k := range e.Dependencies {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		c := e.Dependencies[k]
		// npm prints an optional peer dependency that is not installed as {}
		// (npm 11.12.1, desktop's "@rolldown/plugin-babel", read 2026-09-25)
		// and exits 0. Nothing is installed, so nothing ships; an import of
		// it still fails "not installed". A missing required dep carries
		// "missing": true (npm ls exits 1) and stays an error.
		if c == nil || (c.Path == "" && c.Version == "" && !c.Missing && len(c.Dependencies) == 0) {
			continue
		}
		ck, err := g.add(k, c)
		if err != nil {
			return "", err
		}
		n.kids[ck] = true
	}
	return key, nil
}

// lsGraph runs one npm ls and returns its graph and the root's key.
func lsGraph(dir string, run Runner, args ...string) (npmGraph, string, error) {
	out, err := run(dir, "npm", args...)
	if err != nil {
		return nil, "", fmt.Errorf("npm %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	var root lsEntry
	if err := json.Unmarshal(out, &root); err != nil {
		return nil, "", fmt.Errorf("npm %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	if root.Path == "" {
		root.Path = dir
	}
	g := npmGraph{}
	key, err := g.add(filepath.Base(dir), &root)
	return g, key, err
}

// pkgLicence reads <dir>/package.json: its name, version and licence, the
// "license" string (or a legacy {"type": …} object), else the deprecated
// "licenses" array read as the AND of its entries.
func pkgLicence(dir string) (name, version, declared string, err error) {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", "", "", err
	}
	var p struct {
		Name     string            `json:"name"`
		Version  string            `json:"version"`
		License  json.RawMessage   `json:"license"`
		Licenses []json.RawMessage `json:"licenses"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return "", "", "", fmt.Errorf("%s: %w", filepath.Join(dir, "package.json"), err)
	}
	one := func(raw json.RawMessage) string {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		var o struct {
			Type string `json:"type"`
		}
		json.Unmarshal(raw, &o)
		return o.Type
	}
	if len(p.License) > 0 {
		return p.Name, p.Version, one(p.License), nil
	}
	var parts []string
	for _, raw := range p.Licenses {
		if s := one(raw); s != "" {
			parts = append(parts, s)
		}
	}
	return p.Name, p.Version, strings.Join(parts, " AND "), nil
}

var (
	npmLsProd = []string{"ls", "--all", "--omit=dev", "--json", "--long"}
	npmLsAll  = []string{"ls", "--all", "--include=dev", "--json", "--long"}
)

// npmState is one node's verdict inputs: its Dep, or skip (the project root,
// a link to a scanned project: listed nowhere, and never descended).
type npmState struct {
	dep  Dep
	skip bool
}

// npmScan is one NpmDeps run's context.
type npmScan struct {
	cdir, croot, from string
	scanned           []string
	locks             map[string]lockEntry
	all               npmGraph
	states            map[string]*npmState
}

// state classifies one node once, by structure (gate.go's rule): the link is
// read from package-lock.json at the node's location, never from npm ls.
func (s *npmScan) state(key string) (*npmState, error) {
	if st, ok := s.states[key]; ok {
		return st, nil
	}
	st := &npmState{}
	s.states[key] = st
	if sameCanon(key, s.cdir) {
		st.skip = true
		return st, nil
	}
	n := s.all[key]
	rel, err := filepath.Rel(s.cdir, key)
	if err != nil {
		return nil, err
	}
	loc := filepath.ToSlash(rel)
	read, broken := key, ""
	if e, ok := s.locks[loc]; ok && e.Link {
		target, err := canon(filepath.Join(s.cdir, filepath.FromSlash(e.Resolved)))
		if err != nil {
			return nil, fmt.Errorf("npm link %s -> %s: %w", loc, e.Resolved, err)
		}
		for _, sc := range s.scanned {
			if sameCanon(sc, target) {
				st.skip = true
				return st, nil
			}
		}
		if insideCanon(s.croot, target) {
			broken = linkWhy(target)
		}
		read = target
	}
	name, version, declared, err := pkgLicence(read)
	if err != nil {
		return nil, fmt.Errorf("npm node %s: %w", loc, err)
	}
	if version == "" {
		version = n.version
	}
	st.dep = Dep{Ecosystem: Npm, Name: n.name, Version: version, Declared: declared,
		Source: "package.json", Path: read, From: s.from}
	switch {
	case broken != "":
		st.dep.Broken, st.dep.Why = true, broken
	case name != "" && name != n.name && Denied(Npm, name):
		st.dep.Broken, st.dep.Why = true, "installed as "+n.name+", an alias of "+name+": "+deniedWhy
	}
	return st, nil
}

// reach walks g from start, never into a skipped node other than the
// project root, and calls mark on each node it reaches.
func (s *npmScan) reach(g npmGraph, start string, mark func(*npmState)) error {
	queue, seen := []string{start}, map[string]bool{}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		if seen[k] {
			continue
		}
		seen[k] = true
		st, err := s.state(k)
		if err != nil {
			return err
		}
		if st.skip && !sameCanon(k, s.cdir) {
			continue
		}
		if !st.skip {
			mark(st)
		}
		var ks []string
		if n := g[k]; n != nil {
			for c := range n.kids {
				ks = append(ks, c)
			}
		}
		sort.Strings(ks)
		queue = append(queue, ks...)
	}
	return nil
}

// NpmDeps reads one npm project (the part's contract, sources.go): the
// production closure of npm ls --omit=dev is Enforced, Via "production"; each
// package ShippedImports finds under <dir>/src, at <dir>/node_modules/<name>,
// and its closure, is Enforced, Via "imported by <from>/<file>"; an imported
// name with nothing installed fails "not installed"; every other node is
// reported. Nodes are keyed by path, each licence read from its own
// package.json. A link is read from package-lock.json only: a link to a
// scanned project's root is skipped and not descended; a link into -root
// that is not one fails; a link outside -root is classified from its
// target. A linked node_modules, a missing lockfile, a failed command or a
// path that is gone is an error: the gate could not run (exit 2).
func NpmDeps(root, dir, from string, scanned []string, run Runner) ([]Dep, error) {
	if err := refuseLinkedModules(dir); err != nil {
		return nil, err
	}
	s := &npmScan{from: from, states: map[string]*npmState{}}
	var err error
	if s.cdir, err = canon(dir); err != nil {
		return nil, err
	}
	if s.croot, err = canon(root); err != nil {
		return nil, err
	}
	for _, p := range scanned {
		c, err := canon(p)
		if err != nil {
			return nil, fmt.Errorf("scanned project %s: %w", p, err)
		}
		s.scanned = append(s.scanned, c)
	}
	if s.locks, err = readLock(dir); err != nil {
		return nil, err
	}
	prod, prodRoot, err := lsGraph(dir, run, npmLsProd...)
	if err != nil {
		return nil, err
	}
	all, allRoot, err := lsGraph(dir, run, npmLsAll...)
	if err != nil {
		return nil, err
	}
	for k, n := range prod {
		if all[k] == nil {
			all[k] = &npmNode{name: n.name, version: n.version, path: n.path, kids: map[string]bool{}}
		}
		for c := range n.kids {
			all[k].kids[c] = true
		}
	}
	s.all = all
	if err := s.reach(prod, prodRoot, func(st *npmState) {
		st.dep.Enforced, st.dep.Via = true, "production"
	}); err != nil {
		return nil, err
	}
	imports, err := ShippedImports(dir, filepath.Join(dir, "src"), run)
	if err != nil {
		return nil, err
	}
	var missing []Dep
	names := make([]string, 0, len(imports))
	for n := range imports {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		via := "imported by " + from + "/" + imports[name]
		p := filepath.Join(dir, "node_modules", filepath.FromSlash(name))
		key, cerr := canon(p)
		if cerr != nil || all[key] == nil {
			missing = append(missing, Dep{Ecosystem: Npm, Name: name, Source: "none", Path: p, Enforced: true,
				Via: via, From: from, Broken: true, Why: via + " but not installed"})
			continue
		}
		if err := s.reach(all, key, func(st *npmState) {
			if !st.dep.Enforced {
				st.dep.Enforced, st.dep.Via = true, via
			}
		}); err != nil {
			return nil, err
		}
	}
	if err := s.reach(all, allRoot, func(*npmState) {}); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(s.states))
	for k := range s.states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var deps []Dep
	for _, k := range keys {
		if st := s.states[k]; !st.skip {
			deps = append(deps, st.dep)
		}
	}
	return append(deps, missing...), nil
}

// linkWhy is the sentence a link into -root that is not a scanned project fails with.
func linkWhy(target string) string {
	return "first-party link " + target + " is not a scanned project: add it to the scan list, or install a vendored copy as a file: tarball, which is checked like a registry package"
}
