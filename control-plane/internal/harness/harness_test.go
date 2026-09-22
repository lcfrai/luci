package harness_test

import (
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
)

func TestCatalogueCarriesEveryHarnessIncludingTheShell(t *testing.T) {
	want := map[harness.ID]bool{
		harness.ClaudeCode: false,
		harness.Codex:      false,
		harness.OpenCode:   false,
		harness.GeminiCLI:  false,
		harness.Shell:      false,
	}
	for _, d := range harness.Catalogue() {
		if _, known := want[d.ID]; !known {
			t.Errorf("Catalogue returned an unexpected harness %q", d.ID)
			continue
		}
		want[d.ID] = true
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("Catalogue is missing %q", id)
		}
	}
}

// docs/FUNCTIONAL-SPEC.md line 25: "Claude Code, Codex, OpenCode, Gemini CLI,
// or Shell (a plain terminal, no agent)". A shell is a choice in the picker,
// not the absence of one.
func TestShellIsAFirstClassAvailableHarness(t *testing.T) {
	d, ok := harness.Lookup(harness.Shell)
	if !ok {
		t.Fatal("Lookup(Shell) reported the shell is not in the catalogue")
	}
	if !d.Available() {
		t.Fatalf("Shell.Available() = false (installed=%v flagsVerified=%v), want true", d.Installed, d.FlagsVerified)
	}
	if d.Command != "/bin/bash" {
		t.Errorf("Shell.Command = %q, want %q", d.Command, "/bin/bash")
	}
	if len(d.Models) != 0 || len(d.Efforts) != 0 || len(d.PermissionModes) != 0 {
		t.Errorf("Shell offers models/efforts/permission modes (%v/%v/%v); a terminal accepts none of them", d.Models, d.Efforts, d.PermissionModes)
	}
	if d.SupportsOpeningPrompt {
		t.Error("Shell.SupportsOpeningPrompt = true; a shell would execute the sentence as a command")
	}
}

// images/sandbox/Dockerfile installs @anthropic-ai/claude-code and nothing
// else. Offering an enabled button for a binary that is not in the image
// produces a usage error inside a terminal the user then has to read.
func TestOnlyClaudeCodeAndTheShellAreAvailable(t *testing.T) {
	available := map[harness.ID]bool{}
	for _, d := range harness.Catalogue() {
		if d.Available() {
			available[d.ID] = true
		}
	}
	if len(available) != 2 || !available[harness.ClaudeCode] || !available[harness.Shell] {
		t.Fatalf("available harnesses = %v, want exactly claude-code and shell", available)
	}
}

func TestUnverifiedHarnessesCarryANoteSayingWhy(t *testing.T) {
	for _, id := range []harness.ID{harness.Codex, harness.OpenCode, harness.GeminiCLI} {
		d, ok := harness.Lookup(id)
		if !ok {
			t.Fatalf("Lookup(%q) reported it is not in the catalogue", id)
		}
		if d.Available() {
			t.Errorf("%q is available; its flags have never been checked against --help", id)
		}
		if !strings.Contains(d.Note, "images/sandbox/Dockerfile") {
			t.Errorf("%q.Note = %q, want it to name the file that would enable it", id, d.Note)
		}
		if d.Command == "" {
			t.Errorf("%q has no Command; the conventional binary name records what would be run", id)
		}
	}
}

// Display order is a product decision, not an accident of a map iteration.
// docs/FUNCTIONAL-SPEC.md lists the shell last in prose; the picker puts it
// second, so the two harnesses that actually work sit together at the top
// instead of being separated by three disabled rows.
func TestCatalogueOrdersAvailableHarnessesFirst(t *testing.T) {
	got := make([]harness.ID, 0, 5)
	for _, d := range harness.Catalogue() {
		got = append(got, d.ID)
	}
	want := []harness.ID{harness.ClaudeCode, harness.Shell, harness.Codex, harness.GeminiCLI, harness.OpenCode}
	if len(got) != len(want) {
		t.Fatalf("Catalogue() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Catalogue() = %v, want %v", got, want)
		}
	}
}

// Descriptor holds three slices. Handing a caller the package-level backing
// array means one handler that appends to Models poisons every later request
// in the process, and the symptom appears in an unrelated session.
func TestCatalogueHandsOutCopiesCallersCannotCorrupt(t *testing.T) {
	first := harness.Catalogue()
	for i := range first {
		first[i].Name = "mutated"
		first[i].Models = append(first[i].Models, harness.Model{ID: "injected", Name: "injected"})
		first[i].Efforts = append(first[i].Efforts, harness.Effort("injected"))
	}

	second := harness.Catalogue()
	for _, d := range second {
		if d.Name == "mutated" {
			t.Fatalf("%q.Name survived a caller's mutation; Catalogue shares its backing data", d.ID)
		}
		for _, m := range d.Models {
			if m.ID == "injected" {
				t.Fatalf("%q kept an injected model; Catalogue shares its Models slice", d.ID)
			}
		}
		for _, e := range d.Efforts {
			if e == "injected" {
				t.Fatalf("%q kept an injected effort; Catalogue shares its Efforts slice", d.ID)
			}
		}
	}
}

func TestLookupReportsAnUnknownHarnessRatherThanReturningAZeroDescriptor(t *testing.T) {
	d, ok := harness.Lookup(harness.ID("aider"))
	if ok {
		t.Fatalf("Lookup(\"aider\") = %+v, true; want the zero descriptor and false", d)
	}
	if d.ID != "" {
		t.Errorf("Lookup of an unknown harness returned %+v, want the zero Descriptor", d)
	}
}

// The CLI accepts a fifth level, "max". The product offers four
// (docs/FUNCTIONAL-SPEC.md line 27), so the catalogue advertises four and
// Build rejects the fifth.
func TestClaudeCodeOffersExactlyTheFourProductEfforts(t *testing.T) {
	d, ok := harness.Lookup(harness.ClaudeCode)
	if !ok {
		t.Fatal("Lookup(ClaudeCode) reported it is not in the catalogue")
	}
	want := []harness.Effort{harness.EffortLow, harness.EffortMedium, harness.EffortHigh, harness.EffortXHigh}
	if len(d.Efforts) != len(want) {
		t.Fatalf("ClaudeCode.Efforts = %v, want %v", d.Efforts, want)
	}
	for i := range want {
		if d.Efforts[i] != want[i] {
			t.Fatalf("ClaudeCode.Efforts = %v, want %v", d.Efforts, want)
		}
	}
	for _, e := range d.Efforts {
		if e == "max" {
			t.Fatal("ClaudeCode advertises the CLI's \"max\" effort; the product does not offer it")
		}
	}
	if d.DefaultModel == "" {
		t.Error("ClaudeCode has no DefaultModel; the one-click New-session path has nothing to send")
	}
	if !d.SupportsOpeningPrompt || !d.SupportsResume {
		t.Errorf("ClaudeCode SupportsOpeningPrompt=%v SupportsResume=%v, want both true", d.SupportsOpeningPrompt, d.SupportsResume)
	}
}
