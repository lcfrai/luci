package main

import (
	"strings"
	"testing"
)

func npmDep(name, version, declared string, enforced bool) Dep {
	d := Dep{Ecosystem: Npm, Name: name, Version: version, Declared: declared,
		Source: "package.json", Enforced: enforced, From: "desktop"}
	if enforced {
		d.Via = "production"
	}
	return d
}

func onlyFinding(t *testing.T, deps []Dep, ex []Exception) Finding {
	t.Helper()
	fs, err := Evaluate(deps, ex)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != len(deps) {
		t.Fatalf("Evaluate returned %d findings for %d deps", len(fs), len(deps))
	}
	return fs[0]
}

// The spec's done-when (§9.1 item 13): a gate that passes MPL is no gate.
func TestEvaluate_mplFails(t *testing.T) {
	f := onlyFinding(t, []Dep{npmDep("left-pad-mpl", "1.0.0", "MPL-2.0", true)}, nil)
	if f.Verdict != Fail {
		t.Fatalf("MPL-2.0 enforced dep: verdict %q, want fail", f.Verdict)
	}
	if f.Dep.Name != "left-pad-mpl" {
		t.Fatalf("finding names %q", f.Dep.Name)
	}
	if exitCode([]Finding{f}) != 1 {
		t.Fatalf("exit code %d, want 1", exitCode([]Finding{f}))
	}
}

func TestEvaluate_tldrawDeniedWhateverItDeclares(t *testing.T) {
	f := onlyFinding(t, []Dep{npmDep("@tldraw/editor", "3.0.0", "MIT", true)}, nil)
	if f.Verdict != Fail {
		t.Fatalf("@tldraw/editor declaring MIT: verdict %q, want fail", f.Verdict)
	}
	if !strings.Contains(f.Why, "OD-11") {
		t.Fatalf("why %q does not name OD-11", f.Why)
	}
	// Denied fails a non-enforced copy too: it is reported, never silent.
	f = onlyFinding(t, []Dep{npmDep("tldraw", "3.0.0", "MIT", false)}, nil)
	if f.Verdict != Report {
		t.Fatalf("reported tldraw: verdict %q, want report", f.Verdict)
	}
}

func TestEvaluate_exceptionCannotCoverDenied(t *testing.T) {
	ex := []Exception{{Ecosystem: Npm, Name: "tldraw", Version: "3.0.0",
		Declared: "LicenseRef-tldraw", Resolved: "MIT", Reason: "x",
		ReviewedBy: "r", ReviewedOn: "2026-09-25"}}
	if _, err := Evaluate([]Dep{npmDep("tldraw", "3.0.0", "LicenseRef-tldraw", true)}, ex); err == nil {
		t.Fatal("an exception naming tldraw was accepted")
	}
	// Refused whether or not the tree holds the package.
	if _, err := Evaluate(nil, ex); err == nil {
		t.Fatal("an exception naming tldraw was accepted with no dep present")
	}
}

func TestEvaluate_exceptionPinsVersion(t *testing.T) {
	ex := []Exception{{Ecosystem: Npm, Name: "x", Version: "1.0.0",
		Declared: "LicenseRef-unclassified", Resolved: "MIT", Reason: "read the file",
		ReviewedBy: "r", ReviewedOn: "2026-09-25"}}
	f := onlyFinding(t, []Dep{npmDep("x", "1.0.1", "LicenseRef-unclassified", true)}, ex)
	if f.Verdict != Fail {
		t.Fatalf("x@1.0.1 under an x@1.0.0 exception: verdict %q, want fail", f.Verdict)
	}
	f = onlyFinding(t, []Dep{npmDep("x", "1.0.0", "LicenseRef-unclassified", true)}, ex)
	if f.Verdict != Excepted {
		t.Fatalf("x@1.0.0 under its exception: verdict %q, want excepted", f.Verdict)
	}
	// The ecosystem is part of the pin: a Go module named x is not covered.
	g := Dep{Ecosystem: Go, Name: "x", Version: "1.0.0", Declared: "LicenseRef-unclassified", Enforced: true, Via: "production"}
	f = onlyFinding(t, []Dep{g}, ex)
	if f.Verdict != Fail {
		t.Fatalf("go x@1.0.0 under an npm exception: verdict %q, want fail", f.Verdict)
	}
}

func TestEvaluate_exceptionMustResolveAllowed(t *testing.T) {
	ex := []Exception{{Ecosystem: Cargo, Name: "cssparser", Version: "0.36.0",
		Declared: "MPL-2.0", Resolved: "MPL-2.0", Reason: "x",
		ReviewedBy: "r", ReviewedOn: "2026-09-25"}}
	if _, err := Evaluate(nil, ex); err == nil {
		t.Fatal("an exception resolving to MPL-2.0 was accepted")
	}
}

func TestEvaluate_devDepReportsOnly(t *testing.T) {
	deps := []Dep{npmDep("gpl-dev-tool", "2.0.0", "GPL-3.0", false), npmDep("ok", "1.0.0", "MIT", true)}
	fs, err := Evaluate(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fs[0].Verdict != Report {
		t.Fatalf("non-enforced GPL-3.0: verdict %q, want report", fs[0].Verdict)
	}
	if fs[1].Verdict != Pass {
		t.Fatalf("enforced MIT: verdict %q, want pass", fs[1].Verdict)
	}
	if c := exitCode(fs); c != 0 {
		t.Fatalf("exit code %d, want 0", c)
	}
}

// The deny list matches exactly: a name, a scope, a Go path segment. Never a
// string prefix, never a substring.
func TestEvaluate_denyMatchesExactly(t *testing.T) {
	cases := []struct {
		eco      Ecosystem
		name     string
		declared string
		want     Verdict
	}{
		{Npm, "tldraw", "MIT", Fail},
		{Npm, "tldraw", "LicenseRef-tldraw", Fail},
		{Npm, "@tldraw/editor", "MIT", Fail},
		{Npm, "@tldraw/editor", "Apache-2.0", Fail},
		{Npm, "tldraw-helpers", "MIT", Pass},
		{Npm, "@tldrawx/y", "MIT", Pass},
		{Npm, "not-tldraw", "MIT", Pass},
		{Npm, "@x/tldraw", "MIT", Pass},
		{Go, "github.com/tldraw/tldraw-go", "MIT", Fail},
		{Go, "github.com/tldraw", "MIT", Fail},
		{Go, "github.com/tldrawx/y", "MIT", Pass},
		{Go, "github.com/x/tldraw", "MIT", Pass},
		// GitHub owners are case-insensitive: the same repository.
		{Go, "github.com/TLDraw/x", "MIT", Fail},
		// The npm scope rule is npm's alone: a Go path is not an npm name.
		{Go, "tldraw", "MIT", Pass},
	}
	for _, c := range cases {
		d := Dep{Ecosystem: c.eco, Name: c.name, Version: "1.0.0", Declared: c.declared,
			Enforced: true, Via: "production"}
		fs, err := Evaluate([]Dep{d}, nil)
		if err != nil {
			t.Fatalf("%s %s: %v", c.eco, c.name, err)
		}
		if fs[0].Verdict != c.want {
			t.Errorf("%s %s declaring %s: verdict %q, want %q", c.eco, c.name, c.declared, fs[0].Verdict, c.want)
		}
	}
}

// A structural fault a source found fails whatever licence the dep declares,
// and carries the source's why.
func TestEvaluate_brokenFailsWithItsWhy(t *testing.T) {
	d := npmDep("x", "1.0.0", "MIT", true)
	d.Broken, d.Why = true, "first-party link vendor/x is not a scanned project"
	f := onlyFinding(t, []Dep{d}, nil)
	if f.Verdict != Fail || f.Why != d.Why {
		t.Fatalf("broken MIT dep: %q %q", f.Verdict, f.Why)
	}
	ex := []Exception{{Ecosystem: Npm, Name: "x", Version: "1.0.0", Declared: "MIT",
		Resolved: "MIT", Reason: "x", ReviewedBy: "r", ReviewedOn: "2026-09-25"}}
	if f = onlyFinding(t, []Dep{d}, ex); f.Verdict != Fail {
		t.Fatalf("an exception covered a structural fault: %q", f.Verdict)
	}
}
