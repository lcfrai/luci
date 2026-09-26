package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// npmScans are the npm projects the gate scans, and the npm first-party list
// (gate.go): a link is exempt only when its target is one of these roots. A
// new first-party package is added here, in one line, or the gate fails its
// link as "not a scanned project". Each ships its src/ directory.
var npmScans = []string{"desktop", "canvas/editor"}

// cargoDir is the one Cargo workspace (reported while CargoEnforced is false).
const cargoDir = "desktop/src-tauri"

// exitCode is 1 when any enforced dep failed, else 0. A report never counts.
func exitCode(fs []Finding) int {
	for _, f := range fs {
		if f.Verdict == Fail {
			return 1
		}
	}
	return 0
}

// execRunner runs name in dir, with env set over the process's own
// environment, and returns stdout; stderr goes in the error.
func execRunner(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...) // a later NAME= wins (os/exec dedupes, keeping the last)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 2000 {
			msg = msg[:2000] + " …"
		}
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
	}
	return out, nil
}

// loadExceptions reads the exceptions file. With no -exceptions flag it is
// <root>/tools/licencegate/exceptions.json, and none when that is absent (a
// fixture root has none); a named file must exist. Unknown fields are an
// error, so a misspelt "reviewedBy" is not silently a review.
func loadExceptions(root, named string) ([]Exception, error) {
	p := named
	if p == "" {
		p = filepath.Join(root, "tools", "licencegate", "exceptions.json")
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var f struct {
		Exceptions []Exception `json:"exceptions"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return f.Exceptions, nil
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, execRunner)) }

// run is the gate: exit 0 when every enforced dep passes or is excepted, 1
// when one fails, 2 when the gate could not run. A scanned project absent
// from -root prints a SKIP line (a fixture root holds only some); none
// present at all is exit 2, so a mistyped -root never passes vacuously.
func run(args []string, stdout, stderr io.Writer, runner Runner) int {
	fl := flag.NewFlagSet("licencegate", flag.ContinueOnError)
	fl.SetOutput(stderr)
	rootFlag := fl.String("root", ".", "the repository root")
	exFlag := fl.String("exceptions", "", "the exceptions file (default <root>/tools/licencegate/exceptions.json)")
	asJSON := fl.Bool("json", false, "print the findings as JSON")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	cannot := func(err error) int {
		fmt.Fprintln(stderr, "licencegate: could not run:", err)
		return 2
	}
	root, err := canon(*rootFlag)
	if err != nil {
		return cannot(err)
	}
	exceptions, err := loadExceptions(root, *exFlag)
	if err != nil {
		return cannot(err)
	}
	var deps []Dep
	var notes []string
	present := 0

	var goHave, goMissing []string
	for _, s := range goScans {
		d := strings.TrimSuffix(strings.TrimPrefix(s.pattern, "./"), "/...")
		if exists(filepath.Join(root, filepath.FromSlash(d))) {
			goHave = append(goHave, s.pattern)
		} else {
			goMissing = append(goMissing, s.pattern)
		}
	}
	switch {
	case len(goMissing) == 0:
		d, err := GoDeps(root, runner)
		if err != nil {
			return cannot(err)
		}
		deps, present = append(deps, d...), present+1
	case len(goHave) == 0:
		notes = append(notes, "SKIP go: no shipped Go root under -root")
	default:
		return cannot(fmt.Errorf("go: %s under -root, but not %s", strings.Join(goHave, ", "), strings.Join(goMissing, ", ")))
	}

	var scanned []string
	for _, p := range npmScans {
		if dir := filepath.Join(root, filepath.FromSlash(p)); exists(filepath.Join(dir, "package.json")) {
			scanned = append(scanned, dir)
		}
	}
	for _, p := range npmScans {
		dir := filepath.Join(root, filepath.FromSlash(p))
		if !exists(filepath.Join(dir, "package.json")) {
			notes = append(notes, "SKIP npm "+p+": no package.json under -root")
			continue
		}
		d, err := NpmDeps(root, dir, p, scanned, runner)
		if err != nil {
			return cannot(err)
		}
		deps, present = append(deps, d...), present+1
	}

	notScanned := "no " + cargoDir + "/Cargo.toml under -root"
	if cdir := filepath.Join(root, filepath.FromSlash(cargoDir)); exists(filepath.Join(cdir, "Cargo.toml")) {
		d, ns, err := CargoDeps(root, cdir, runner)
		if err != nil {
			return cannot(err)
		}
		deps, notScanned, present = append(deps, d...), ns, present+1
	}
	if present == 0 {
		return cannot(fmt.Errorf("no scanned project under %s", root))
	}
	findings, err := Evaluate(deps, exceptions)
	if err != nil {
		return cannot(err)
	}
	code := exitCode(findings)
	if *asJSON {
		b, _ := json.MarshalIndent(map[string]any{"findings": findings, "notes": notes,
			"cargoNotScanned": notScanned, "exit": code}, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return code
	}
	report(stdout, findings, notes, notScanned, code)
	return code
}
