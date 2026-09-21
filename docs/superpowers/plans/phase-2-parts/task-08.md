### Task 8: The harness catalogue, the worktree seam, and read-only project enumeration

Three small packages that together answer the only question the New-session button asks: *what can I start, and where can I start it*. `harness` says what can be started and turns a choice into argv. `worktree` says where, as a one-method seam Phase 3 will implement properly. `project` is the read-only `git` enumeration that fills the rail and happens to satisfy both that seam and Phase 4's naming interface.

`Build` is where known defect 2's sibling — the comma defect — is finally dead. Group A task 3 replaced sandboxd's comma-joined `PANACEA_APP_ARGS` with `PANACEA_APP_ARGS_JSON` so a prompt could survive transport; this task is the other half, the producer that emits an opening prompt as **one** argv element and never splits it. The argv tests pin a prompt containing a comma, because that is the input that used to become two arguments and a confused agent.

Three decisions the contract left open, made here so no later reader re-derives them. **First, an option a harness does not accept is rejected, not ignored.** `Build` returns `ErrUnsupportedOption` wrapped with the offending field name. Ignore-and-report has nowhere to report to: `Build`'s signature is `(string, []string, error)`, there is no warnings channel, and adding one forces every caller to plumb it to the UI or silently drop it — which is how a user ends up believing they started a high-effort Codex session that never had an effort flag. Rejecting surfaces at the API boundary as a `400 bad_request` naming the field. Concretely: effort is Claude Code only (`docs/FUNCTIONAL-SPEC.md` line 27), so any non-empty `Effort` on `shell` is an error, and `max` on Claude Code is an error too because the product offers four levels and the CLI's fifth is not one of them. **Second, validation order is fixed**: unknown harness, then unavailable, then options in field order — model, effort, permission, resume, prompt. A request with two faults must fail the same way every run or the API's error body is nondeterministic. **Third, catalogue display order is available-first** — `claude-code`, `shell`, then the three unverified entries alphabetically — rather than the spec's prose order, which would seat three disabled rows between the two that work.

`project` shells out to `git`, so its logic tests must not depend on whatever repositories happen to sit on the machine running them. The `Runner` interface takes that dependency out: the enumeration tests drive a scripted fake and need neither `git` nor a checkout. One separate file, `gitrunner_test.go`, exercises the real `exec.Command` runner against a git repository **the test itself creates in `t.TempDir()`**, with a skip when `git` is not on `PATH`. Enumeration is read-only in Phase 2 in the strongest sense: no code in this task runs a git subcommand that writes, and no code creates or removes a worktree. Phase 3 owns that, and a Phase 2 that half-owned it would be building Phase 3 badly.

**Files:**
- `control-plane/internal/harness/harness.go` — the catalogue: IDs, `Descriptor`, `Catalogue`, `Lookup`, sentinel errors (new)
- `control-plane/internal/harness/argv.go` — `Build`: request → `(command, argv)`, and the option validators (new)
- `control-plane/internal/harness/harness_test.go` — catalogue shape, availability, defensive copying (new)
- `control-plane/internal/harness/argv_test.go` — every argv rule, including the comma prompt (new)
- `control-plane/internal/harness/model_internal_test.go` — white-box test of `validateModel`, the one path `ErrUnknownModel` lives on (new)
- `control-plane/internal/worktree/worktree.go` — the Phase 3 seam: `Provider`, `Static`, `ErrUnknownID`, `ErrNotADir` (new)
- `control-plane/internal/worktree/worktree_test.go` — resolution, the missing-directory failure, context cancellation (new)
- `control-plane/internal/project/project.go` — `Project`/`Branch`/`Worktree`, derived ids, `Scanner`, `Path`, `ProjectName`/`BranchName` (new)
- `control-plane/internal/project/gitrunner.go` — `ExecRunner`, the only place `exec.CommandContext` runs git (new)
- `control-plane/internal/project/project_test.go` — enumeration against a scripted fake `Runner`; no git, no repo (new)
- `control-plane/internal/project/gitrunner_test.go` — `ExecRunner` and `Scanner` against a temp repo the test creates (new)

The file-structure table lists one file for `project`. `gitrunner.go` is a deliberate split from it: keeping `exec.CommandContext` out of `project.go` is what lets the enumeration tests run with no git binary and no repository on disk, which is the difference between a suite that passes on a clean CI image and one that passes only on a developer's laptop.

**Interfaces:**
- Produces: `harness.ID` and the five constants (`ClaudeCode`, `Codex`, `OpenCode`, `GeminiCLI`, `Shell`), `harness.PermissionMode` (`PermissionAsk`/`PermissionBypass`/`PermissionPlan`), `harness.Effort` (`EffortLow`/`EffortMedium`/`EffortHigh`/`EffortXHigh`), `harness.Model`, `harness.Descriptor` with `Available()`, `harness.Request`, `harness.Catalogue()`, `harness.Lookup()`, `harness.Build()`, `harness.ErrUnknownHarness`/`ErrUnavailable`/`ErrUnsupportedOption`/`ErrUnknownModel`; `worktree.Provider`, `worktree.Static`, `worktree.Verify`, `worktree.ErrUnknownID`, `worktree.ErrNotADir`; `project.Project`, `project.Branch`, `project.Worktree`, `project.Runner`, `project.Scanner`, `project.NewScanner`, `project.ExecRunner`, `project.NewExecRunner`, `project.GitTimeout`, `project.ProjectID`, `project.WorktreeID`, `project.BranchID`, `project.ErrUnknownProject`
- Note on three of those. `project.ErrUnknownProject` is a sentinel this task invents because the contract gives `Branches` and `Worktrees` an error return but names none: **task 10 maps it to `404 project_not_found` via `errors.Is`, and must never string-match the message or declare a second sentinel of its own.** `worktree.Verify` is the absolute-and-exists check shared by `worktree.Static` and `project.Scanner.Path`, exported so the two cannot drift. `project.GitTimeout` is the per-invocation bound the HTTP layer inherits.
- Consumes: nothing at compile time. The output is consumed downstream: task 11 puts `Build`'s `(command, argv)` into `backend.Spec.Command` and `backend.Spec.Args` (task 4), and resolves `backend.Mount.HostPath` from `worktree.Provider.Path`; task 10 serves `Catalogue()` at `GET /v1/harnesses` and `Scanner`'s three enumerations at `GET /v1/projects`, `…/branches` and `…/worktrees`.

- [ ] **Step 1: Write the failing catalogue tests**

These pin the four properties the picker depends on and nothing else. That `Shell` is present and available is the load-bearing one: `docs/FUNCTIONAL-SPEC.md` line 25 lists a plain terminal as one of five harnesses, so a catalogue that models "no agent" as the absence of a harness cannot render the picker at all. The copy test exists because `Descriptor` holds three slices; a package-level slice handed straight to a JSON encoder is fine, but handed to a caller that appends to `Models` it corrupts every later request in the process.

Create `control-plane/internal/harness/harness_test.go`:

```go
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
```

- [ ] **Step 2: Run the catalogue tests and watch them fail**

Run: `go test ./control-plane/internal/harness/...`

Expected: FAIL, and specifically:

```
control-plane/internal/harness: no non-test Go files in C:\...\control-plane\internal\harness
FAIL    github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness [build failed]
```

That is the right failure. The directory now holds exactly one file, an external `harness_test.go` in package `harness_test`, and the package it imports has no source: there is nothing to compile, and nothing has been asserted about behaviour that does not exist.

Do **not** expect `no required module provides package …`. That message belongs to an import path *outside* the module; `internal/harness` is inside `control-plane` (module `github.com/kpmg-au-emu/panacea-app/control-plane`), so the toolchain resolves the path and then finds it empty. The distinction matters because `no required module provides package` invites a `go get`, and the Global Constraints forbid it — it can move the `go 1.23` directive and it needs the TLS-intercepted proxy. If you ever do see that message here, the import path in the test file is misspelled; fix the spelling, do not fetch anything.

Failure mode to watch for: if the error is instead `directory prefix . does not contain modules listed in go.work`, the command was run as `go test ./...` from the workspace root. The root is not a module. Use the explicit package path.

- [ ] **Step 3: Write the catalogue**

Create `control-plane/internal/harness/harness.go`:

```go
// Package harness decides which binary a session runs and with which arguments.
// It never wraps, parses or reimplements any CLI; it constructs argv and stops.
//
// Two things in here are product decisions rather than technical ones, and both
// are deliberate. A shell is a harness — docs/FUNCTIONAL-SPEC.md lists a plain
// terminal as one of five choices in the picker, so "no agent" is an entry in
// this catalogue and not a special case elsewhere. And a harness whose flags
// have never been checked against its own --help ships as unavailable with a
// note, rather than being started with guessed flags: a wrong flag produces a
// usage error inside a terminal the user then has to read and interpret.
package harness

import "errors"

// ID names a harness. These strings are the wire values in GET /v1/harnesses
// and in registry.Session.Harness, so they are stable.
type ID string

const (
	ClaudeCode ID = "claude-code"
	Codex      ID = "codex"
	OpenCode   ID = "opencode"
	GeminiCLI  ID = "gemini-cli"
	Shell      ID = "shell"
)

// PermissionMode is the product's vocabulary, from docs/FUNCTIONAL-SPEC.md
// ("bypass / ask / plan"). Each harness maps it to its own flag, or to nothing.
type PermissionMode string

const (
	PermissionAsk    PermissionMode = "ask"
	PermissionBypass PermissionMode = "bypass"
	PermissionPlan   PermissionMode = "plan"
)

// Effort is Claude Code's effort level, and Claude Code's alone
// (docs/FUNCTIONAL-SPEC.md line 27: "low / medium / high / xhigh (Claude Code
// only)"). The CLI also accepts "max"; the product does not offer it, so Build
// rejects it rather than passing through a level no picker can show.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortXHigh  Effort = "xhigh"
)

// Model is one selectable model for a harness.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Descriptor is everything the cockpit needs to render the picker and
// everything Build needs to construct argv.
type Descriptor struct {
	ID      ID     `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`

	// Installed is whether images/sandbox/Dockerfile actually puts this binary
	// in the sandbox image. Only Claude Code and the shell are, today.
	Installed bool `json:"installed"`

	// FlagsVerified is whether this package's argv was checked against the real
	// CLI's --help.
	FlagsVerified bool `json:"flagsVerified"`

	Models          []Model          `json:"models"`
	DefaultModel    string           `json:"defaultModel"`
	Efforts         []Effort         `json:"efforts"`
	PermissionModes []PermissionMode `json:"permissionModes"`

	SupportsOpeningPrompt bool `json:"supportsOpeningPrompt"`
	SupportsResume        bool `json:"supportsResume"`

	// Note is shown in the picker when the harness is unavailable. It says why.
	Note string `json:"note"`

	// freeTextModel is unexported on purpose: it changes no JSON and the
	// cockpit has no use for it. Claude Code documents full model names as well
	// as the three aliases, so pinning its list here would make the app stale
	// the week a new model ships. The three unverified harnesses do not get
	// that latitude — if one is ever enabled, an unlisted model is a typo.
	freeTextModel bool
}

// Available reports whether a session may actually be started on this harness.
func (d Descriptor) Available() bool { return d.Installed && d.FlagsVerified }

// Request is one session's configuration.
type Request struct {
	Harness       ID
	Model         string
	Effort        Effort
	Permission    PermissionMode
	OpeningPrompt string
	Resume        string // a prior agent session id; "" starts fresh
}

var (
	ErrUnknownHarness    = errors.New("harness: unknown harness")
	ErrUnavailable       = errors.New("harness: harness is not available in the sandbox image")
	ErrUnsupportedOption = errors.New("harness: this harness does not accept that option")
	ErrUnknownModel      = errors.New("harness: model is not offered by this harness")
)
```

The rest of `control-plane/internal/harness/harness.go`, appended immediately below that `var (…)` error block, in the same file:

```go
// unverifiedNote is one string because three descriptors say the same thing and
// the cockpit renders it verbatim. It names the file to change, so the note is
// an instruction rather than an apology.
const unverifiedNote = "Not installed in the sandbox image. Add it to images/sandbox/Dockerfile and verify its flags against `--help` before enabling."

// catalogue is the single source of truth, in display order: the two harnesses
// that work, then the three that do not, alphabetically. Never handed out
// directly — see clone.
var catalogue = []Descriptor{
	{
		ID:      ClaudeCode,
		Name:    "Claude Code",
		Command: "claude",
		// Verified against `claude --help` on this machine, version 2.1.267.
		// images/sandbox/Dockerfile pins @anthropic-ai/claude-code@2.1.274.
		Installed:     true,
		FlagsVerified: true,
		Models: []Model{
			{ID: "opus", Name: "Opus"},
			{ID: "sonnet", Name: "Sonnet"},
			{ID: "fable", Name: "Fable"},
		},
		DefaultModel:          "opus",
		Efforts:               []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh},
		PermissionModes:       []PermissionMode{PermissionAsk, PermissionBypass, PermissionPlan},
		SupportsOpeningPrompt: true,
		SupportsResume:        true,
		freeTextModel:         true,
	},
	{
		ID:      Shell,
		Name:    "Shell",
		Command: "/bin/bash",
		// Verified trivially: bash is in the image and -l is POSIX-stable.
		Installed:     true,
		FlagsVerified: true,
		// No models, no efforts, no permission modes, and no opening prompt: a
		// shell has no way to receive a sentence that is not "execute this",
		// and silently executing what the user typed into a prompt box is a
		// footgun, not a convenience.
		SupportsOpeningPrompt: false,
		SupportsResume:        false,
	},
	{
		ID:            Codex,
		Name:          "Codex",
		Command:       "codex",
		Installed:     false,
		FlagsVerified: false,
		Note:          unverifiedNote,
	},
	{
		ID:            GeminiCLI,
		Name:          "Gemini CLI",
		Command:       "gemini",
		Installed:     false,
		FlagsVerified: false,
		Note:          unverifiedNote,
	},
	{
		ID:            OpenCode,
		Name:          "OpenCode",
		Command:       "opencode",
		Installed:     false,
		FlagsVerified: false,
		Note:          unverifiedNote,
	},
}

// clone deep-copies the three slices a Descriptor carries. Without it a handler
// that appends to Models poisons the package variable for every later request
// in the process, and the symptom shows up in an unrelated session.
func clone(d Descriptor) Descriptor {
	out := d
	if d.Models != nil {
		out.Models = append([]Model(nil), d.Models...)
	}
	if d.Efforts != nil {
		out.Efforts = append([]Effort(nil), d.Efforts...)
	}
	if d.PermissionModes != nil {
		out.PermissionModes = append([]PermissionMode(nil), d.PermissionModes...)
	}
	return out
}

// Catalogue returns every harness, in display order.
func Catalogue() []Descriptor {
	out := make([]Descriptor, 0, len(catalogue))
	for _, d := range catalogue {
		out = append(out, clone(d))
	}
	return out
}

// Lookup returns one descriptor. The second result is false for an id the
// catalogue does not carry, and the first is then the zero Descriptor.
func Lookup(id ID) (Descriptor, bool) {
	for _, d := range catalogue {
		if d.ID == id {
			return clone(d), true
		}
	}
	return Descriptor{}, false
}
```

- [ ] **Step 4: Run the catalogue tests and watch them pass**

Run: `go test ./control-plane/internal/harness/... -race -count=1 -v`

Expected: PASS — eight tests, `ok github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness`.

Failure mode: if `TestCatalogueHandsOutCopiesCallersCannotCorrupt` fails on `Efforts`, `clone` is copying the struct but not the slices — assigning `out := d` copies three slice headers that still point at the package variable's arrays.

- [ ] **Step 5: Write the failing argv tests**

`TestBuildKeepsACommaInTheOpeningPromptAsOneArgument` is the point of this file. Group A task 3 killed the comma-joined `PANACEA_APP_ARGS` inside sandboxd; this is the producer side of the same defect, and the only way it comes back is if someone "helpfully" splits or joins the prompt here. The remaining tests pin the flag mapping against the recorded `claude --help` evidence, the rejection rules, and — importantly — the *order* in which rejections happen, so a request with two faults produces the same error every run.

Create `control-plane/internal/harness/argv_test.go`:

```go
package harness_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
)

func argvEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argv = %#v (%d elements), want %#v (%d elements)", got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q (full argv %#v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildClaudeCodeMapsEveryFieldToItsVerifiedFlag(t *testing.T) {
	cmd, argv, err := harness.Build(harness.Request{
		Harness:       harness.ClaudeCode,
		Model:         "opus",
		Effort:        harness.EffortHigh,
		Permission:    harness.PermissionBypass,
		Resume:        "abc123",
		OpeningPrompt: "add tests",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if cmd != "claude" {
		t.Fatalf("command = %q, want %q", cmd, "claude")
	}
	argvEqual(t, argv, []string{
		"--model", "opus",
		"--effort", "high",
		"--permission-mode", "bypassPermissions",
		"--resume", "abc123",
		"add tests",
	})
}

// THE COMMA DEFECT. The legacy PANACEA_APP_ARGS was comma-joined, so this
// prompt arrived inside the sandbox as two arguments and the agent was started
// with a truncated instruction plus a stray one. Group A task 3 replaced the
// transport with PANACEA_APP_ARGS_JSON; this asserts the producer never does
// the splitting itself.
func TestBuildKeepsACommaInTheOpeningPromptAsOneArgument(t *testing.T) {
	const prompt = "add tests, then refactor, and finally update the README"

	_, argv, err := harness.Build(harness.Request{
		Harness:       harness.ClaudeCode,
		Model:         "opus",
		OpeningPrompt: prompt,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	argvEqual(t, argv, []string{"--model", "opus", prompt})

	for _, a := range argv {
		if a != prompt && strings.Contains(a, ",") {
			t.Fatalf("argv element %q contains a comma; the prompt was split (full argv %#v)", a, argv)
		}
	}
}

// Usage: claude [options] [command] [prompt]. A prompt emitted before the
// flags is read by the CLI as the command name.
func TestBuildPutsTheOpeningPromptLast(t *testing.T) {
	_, argv, err := harness.Build(harness.Request{
		Harness:       harness.ClaudeCode,
		Model:         "sonnet",
		Effort:        harness.EffortLow,
		OpeningPrompt: "why is this flaky?",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if argv[len(argv)-1] != "why is this flaky?" {
		t.Fatalf("argv = %#v, want the prompt last", argv)
	}
}
```

The rest of `control-plane/internal/harness/argv_test.go`, appended immediately below `TestBuildPutsTheOpeningPromptLast`, in the same file:

```go
// `claude --permission-mode` offers acceptEdits, auto, bypassPermissions,
// manual, dontAsk and plan — there is no `default`, and --help does not say
// which is the out-of-the-box behaviour. Omitting the flag gets the CLI's own
// default, which IS the ask behaviour, and is provably correct without
// guessing a mapping. Open question 1 in the contract tracks confirming it.
func TestBuildAskEmitsNoPermissionFlag(t *testing.T) {
	for _, p := range []harness.PermissionMode{harness.PermissionAsk, ""} {
		_, argv, err := harness.Build(harness.Request{Harness: harness.ClaudeCode, Permission: p})
		if err != nil {
			t.Fatalf("Build with Permission=%q: %v", p, err)
		}
		for _, a := range argv {
			if a == "--permission-mode" {
				t.Fatalf("Permission=%q emitted --permission-mode (argv %#v); it must emit nothing", p, argv)
			}
		}
	}
}

func TestBuildPlanMapsToThePlanMode(t *testing.T) {
	_, argv, err := harness.Build(harness.Request{Harness: harness.ClaudeCode, Permission: harness.PermissionPlan})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	argvEqual(t, argv, []string{"--permission-mode", "plan"})
}

// --model documents full model names as well as aliases. Pinning the list
// would make the app stale the week a new model ships, so an unlisted string
// is passed through unchanged for Claude Code specifically.
func TestBuildPassesAnUnlistedClaudeModelThroughUnchanged(t *testing.T) {
	_, argv, err := harness.Build(harness.Request{Harness: harness.ClaudeCode, Model: "claude-fable-5-20260401"})
	if err != nil {
		t.Fatalf("Build with a full model name: %v", err)
	}
	argvEqual(t, argv, []string{"--model", "claude-fable-5-20260401"})
}

func TestBuildEmitsNothingForFieldsLeftEmpty(t *testing.T) {
	cmd, argv, err := harness.Build(harness.Request{Harness: harness.ClaudeCode})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if cmd != "claude" {
		t.Fatalf("command = %q, want %q", cmd, "claude")
	}
	if len(argv) != 0 {
		t.Fatalf("argv = %#v, want no arguments for a request that sets nothing", argv)
	}
}

func TestBuildShellIsALoginShellAndNothingElse(t *testing.T) {
	cmd, argv, err := harness.Build(harness.Request{Harness: harness.Shell})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if cmd != "/bin/bash" {
		t.Fatalf("command = %q, want %q", cmd, "/bin/bash")
	}
	argvEqual(t, argv, []string{"-l"})
}

func TestBuildAcceptsAskOnTheShell(t *testing.T) {
	if _, _, err := harness.Build(harness.Request{Harness: harness.Shell, Permission: harness.PermissionAsk}); err != nil {
		t.Fatalf("Build(shell, ask) = %v, want nil: ask is the absence of a flag everywhere", err)
	}
}

// Effort is Claude Code only (docs/FUNCTIONAL-SPEC.md line 27), and a shell
// takes none of the other options either. Reject rather than ignore: Build
// returns (string, []string, error) and has nowhere to report an option it
// quietly dropped, so ignoring would let a user believe they started a
// high-effort shell session.
func TestBuildRejectsEveryAgentOptionOnTheShell(t *testing.T) {
	cases := map[string]harness.Request{
		"model":      {Harness: harness.Shell, Model: "opus"},
		"effort":     {Harness: harness.Shell, Effort: harness.EffortHigh},
		"permission": {Harness: harness.Shell, Permission: harness.PermissionBypass},
		"resume":     {Harness: harness.Shell, Resume: "abc123"},
		"prompt":     {Harness: harness.Shell, OpeningPrompt: "do the thing"},
	}
	for name, req := range cases {
		_, argv, err := harness.Build(req)
		if err == nil {
			// A nil error here is the diagnostic step 10 names — a descriptor
			// still advertising the option. Report it and move on: the
			// err.Error() below would panic and lose every other case.
			t.Errorf("Build(shell with %s) = nil error and argv %#v, want an error wrapping ErrUnsupportedOption", name, argv)
			continue
		}
		if !errors.Is(err, harness.ErrUnsupportedOption) {
			t.Errorf("Build(shell with %s) = %v, want an error wrapping ErrUnsupportedOption", name, err)
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Build(shell with %s) error %q does not name the field; the API cannot tell the user which one to clear", name, err)
		}
		if argv != nil {
			t.Errorf("Build(shell with %s) returned argv %#v alongside an error; a partial argv is how a bad request becomes a started session", name, argv)
		}
	}
}
```

The last block of `control-plane/internal/harness/argv_test.go`, appended below `TestBuildRejectsEveryAgentOptionOnTheShell`, in the same file:

```go
// The CLI accepts "max"; the product offers four levels. Passing a level the
// picker cannot render means the cockpit and the running session disagree
// about what was started.
func TestBuildRejectsAnEffortTheProductDoesNotOffer(t *testing.T) {
	_, _, err := harness.Build(harness.Request{Harness: harness.ClaudeCode, Effort: harness.Effort("max")})
	if !errors.Is(err, harness.ErrUnsupportedOption) {
		t.Fatalf("Build(claude-code, effort=max) = %v, want an error wrapping ErrUnsupportedOption", err)
	}
	if !strings.Contains(err.Error(), "effort") || !strings.Contains(err.Error(), "max") {
		t.Errorf("error %q names neither the field nor the offending value", err)
	}
}

func TestBuildRejectsAPermissionModeTheProductDoesNotOffer(t *testing.T) {
	_, _, err := harness.Build(harness.Request{Harness: harness.ClaudeCode, Permission: harness.PermissionMode("acceptEdits")})
	if !errors.Is(err, harness.ErrUnsupportedOption) {
		t.Fatalf("Build(claude-code, permission=acceptEdits) = %v, want an error wrapping ErrUnsupportedOption", err)
	}
}

func TestBuildRejectsAnUnknownHarness(t *testing.T) {
	_, _, err := harness.Build(harness.Request{Harness: harness.ID("aider")})
	if !errors.Is(err, harness.ErrUnknownHarness) {
		t.Fatalf("Build(aider) = %v, want an error wrapping ErrUnknownHarness", err)
	}
}

func TestBuildRejectsAHarnessThatIsNotInTheImage(t *testing.T) {
	for _, id := range []harness.ID{harness.Codex, harness.OpenCode, harness.GeminiCLI} {
		_, _, err := harness.Build(harness.Request{Harness: id})
		if !errors.Is(err, harness.ErrUnavailable) {
			t.Errorf("Build(%q) = %v, want an error wrapping ErrUnavailable", id, err)
		}
	}
}

// Validation order is fixed so a request with two faults fails the same way
// every run: unknown harness, then unavailable, then options in field order.
// Reporting "unsupported effort" for a harness that is not even installed
// sends the user to fix the wrong thing.
func TestBuildChecksAvailabilityBeforeOptions(t *testing.T) {
	_, _, err := harness.Build(harness.Request{
		Harness: harness.Codex,
		Effort:  harness.EffortHigh,
		Model:   "nonsense",
	})
	if !errors.Is(err, harness.ErrUnavailable) {
		t.Fatalf("Build(codex with bad options) = %v, want ErrUnavailable to win", err)
	}
	if errors.Is(err, harness.ErrUnsupportedOption) {
		t.Fatal("Build reported an unsupported option for a harness that is not installed; fix the order")
	}
}

// The shell is the only available harness on which both validators can fault,
// which is what makes the order observable: Claude Code's model is free text,
// so a request carrying a bad model and a bad effort faults on effort whichever
// way round Build calls them and proves nothing.
func TestBuildChecksTheModelBeforeTheEffort(t *testing.T) {
	_, _, err := harness.Build(harness.Request{
		Harness: harness.Shell,
		Model:   "opus",
		Effort:  harness.Effort("max"),
	})
	if err == nil {
		t.Fatal("Build(shell with both a model and an effort) = nil error, want the model fault")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Fatalf("error = %q, want it to name model, the first failing field in order", err)
	}
	if strings.Contains(err.Error(), "effort") {
		t.Fatalf("error = %q reports effort; model is validated first and its error is the one the API returns", err)
	}
}
```

- [ ] **Step 6: Run the argv tests and watch them fail**

Run: `go test ./control-plane/internal/harness/...`

Expected: FAIL to build — `undefined: harness.Build`. The catalogue tests from step 1 no longer run either, because a package that does not compile has no passing tests; that is expected and resolves at step 8.

- [ ] **Step 7: Write the failing white-box model test**

`ErrUnknownModel` is in the contract but is unreachable through `Build` in Phase 2: the only available harness with a model list accepts free text, and the other three fail earlier with `ErrUnavailable`. Deleting it is wrong — task 11's error switch is written against the contract's sentinel set, and the rule bites the moment a Dockerfile line makes one of the three available. So it is tested directly, from inside the package, against a descriptor with a closed list. This is the only white-box test in the package and it earns its place by keeping a forward-declared sentinel from rotting into a lie.

Create `control-plane/internal/harness/model_internal_test.go`:

```go
package harness

import (
	"errors"
	"testing"
)

func TestValidateModelEnforcesAClosedList(t *testing.T) {
	d := Descriptor{
		ID:            Codex,
		Models:        []Model{{ID: "o4", Name: "o4"}},
		freeTextModel: false,
	}
	if err := validateModel(d, "o4"); err != nil {
		t.Fatalf("validateModel(listed model) = %v, want nil", err)
	}
	err := validateModel(d, "gpt-99")
	if !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("validateModel(unlisted model) = %v, want an error wrapping ErrUnknownModel", err)
	}
}

func TestValidateModelAcceptsAnyStringWhenTheHarnessTakesFreeText(t *testing.T) {
	d, ok := Lookup(ClaudeCode)
	if !ok {
		t.Fatal("Lookup(ClaudeCode) reported it is not in the catalogue")
	}
	if err := validateModel(d, "claude-fable-5-20260401"); err != nil {
		t.Fatalf("validateModel(full model name) = %v, want nil", err)
	}
}

func TestValidateModelAcceptsAnEmptyModel(t *testing.T) {
	d, _ := Lookup(Shell)
	if err := validateModel(d, ""); err != nil {
		t.Fatalf("validateModel(\"\") = %v, want nil: an unset field is not an option", err)
	}
}

func TestValidateModelRejectsAModelOnAHarnessWithNoModels(t *testing.T) {
	d, _ := Lookup(Shell)
	err := validateModel(d, "opus")
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Fatalf("validateModel(shell, \"opus\") = %v, want an error wrapping ErrUnsupportedOption", err)
	}
}
```

- [ ] **Step 8: Run the harness tests and watch them fail**

Run: `go test ./control-plane/internal/harness/...`

Expected: FAIL to build, naming both gaps — `undefined: harness.Build` from `argv_test.go` and `undefined: validateModel` from `model_internal_test.go`.

Failure mode: if the compiler instead reports `d.freeTextModel undefined`, `model_internal_test.go` was created with `package harness_test` rather than `package harness`. Both files are legal in one directory; only the internal one can see an unexported field.

- [ ] **Step 9: Write Build**

Create `control-plane/internal/harness/argv.go`:

```go
package harness

import "fmt"

// claudePermissionFlag maps the product's three-word vocabulary onto the CLI's
// --permission-mode choices. Evidence: `claude --help` 2.1.267 lists
// acceptEdits, auto, bypassPermissions, manual, dontAsk and plan.
//
// "ask" is deliberately absent. The choice list has no `default`, and --help
// does not say which of manual/auto/dontAsk is the out-of-the-box behaviour.
// Emitting no flag gets the CLI's own default, which IS the ask behaviour, and
// is provably correct without guessing a mapping. Contract open question 1
// tracks confirming it against a real session before Phase 8 adds an admin
// control over permitted modes.
var claudePermissionFlag = map[PermissionMode]string{
	PermissionBypass: "bypassPermissions",
	PermissionPlan:   "plan",
}

// Build turns a Request into the command and argv a sandbox runs.
//
// The opening prompt is appended as ONE argv element and is never split,
// joined, quoted or escaped. That is the whole point of this function: the
// legacy comma-joined PANACEA_APP_ARGS turned "add tests, then refactor" into
// two arguments, and Group A task 3 replaced the transport with
// PANACEA_APP_ARGS_JSON so the producer could stay honest.
//
// Validation order is fixed — unknown harness, unavailable, then options in
// field order — so a request with two faults fails the same way every run.
// On any error both return values are zero: a partial argv is how a rejected
// request becomes a started session.
func Build(r Request) (command string, argv []string, err error) {
	d, ok := Lookup(r.Harness)
	if !ok {
		return "", nil, fmt.Errorf("%w: %q", ErrUnknownHarness, r.Harness)
	}
	if !d.Available() {
		return "", nil, fmt.Errorf("%w: %q: %s", ErrUnavailable, d.ID, d.Note)
	}
	if err := validateModel(d, r.Model); err != nil {
		return "", nil, err
	}
	if err := validateEffort(d, r.Effort); err != nil {
		return "", nil, err
	}
	if err := validatePermission(d, r.Permission); err != nil {
		return "", nil, err
	}
	if err := validateResume(d, r.Resume); err != nil {
		return "", nil, err
	}
	if err := validatePrompt(d, r.OpeningPrompt); err != nil {
		return "", nil, err
	}

	switch d.ID {
	case ClaudeCode:
		return d.Command, claudeArgv(r), nil
	case Shell:
		// -l gives a login shell, so /etc/profile and the image's PATH are in
		// place. Nothing else: a terminal takes no options from this request,
		// which validatePrompt and friends have already enforced.
		return d.Command, []string{"-l"}, nil
	default:
		// Unreachable while Available() is true only for the two above. It is a
		// loud failure rather than a silent empty argv so that enabling a
		// harness in the catalogue without writing its builder is caught by the
		// first request instead of by a container that starts and exits.
		return "", nil, fmt.Errorf("%w: %q is available but has no argv builder", ErrUnavailable, d.ID)
	}
}

func claudeArgv(r Request) []string {
	argv := make([]string, 0, 9)
	if r.Model != "" {
		argv = append(argv, "--model", r.Model)
	}
	if r.Effort != "" {
		argv = append(argv, "--effort", string(r.Effort))
	}
	if flag, ok := claudePermissionFlag[r.Permission]; ok {
		argv = append(argv, "--permission-mode", flag)
	}
	if r.Resume != "" {
		argv = append(argv, "--resume", r.Resume)
	}
	if r.OpeningPrompt != "" {
		// Usage: claude [options] [command] [prompt]. Last, and whole.
		argv = append(argv, r.OpeningPrompt)
	}
	return argv
}
```

The validators, appended below `claudeArgv` in the same `control-plane/internal/harness/argv.go`:

```go
// The five validators share one rule: an unset field is not an option, so ""
// always passes. Anything else must be something the descriptor advertises,
// because the descriptor is what the picker rendered and the two must agree.

func validateModel(d Descriptor, model string) error {
	if model == "" {
		return nil
	}
	if len(d.Models) == 0 {
		return fmt.Errorf("%w: %q does not accept a model", ErrUnsupportedOption, d.ID)
	}
	if d.freeTextModel {
		return nil
	}
	for _, m := range d.Models {
		if m.ID == model {
			return nil
		}
	}
	return fmt.Errorf("%w: %q does not offer the model %q", ErrUnknownModel, d.ID, model)
}

func validateEffort(d Descriptor, effort Effort) error {
	if effort == "" {
		return nil
	}
	if len(d.Efforts) == 0 {
		// docs/FUNCTIONAL-SPEC.md line 27: effort is Claude Code only.
		return fmt.Errorf("%w: %q does not accept an effort level", ErrUnsupportedOption, d.ID)
	}
	for _, e := range d.Efforts {
		if e == effort {
			return nil
		}
	}
	return fmt.Errorf("%w: %q does not offer the effort %q", ErrUnsupportedOption, d.ID, effort)
}

func validatePermission(d Descriptor, mode PermissionMode) error {
	// "ask" is the absence of a flag on every harness, including a shell,
	// which is why it passes before the advertised-modes check.
	if mode == "" || mode == PermissionAsk {
		return nil
	}
	if len(d.PermissionModes) == 0 {
		return fmt.Errorf("%w: %q does not accept a permission mode", ErrUnsupportedOption, d.ID)
	}
	for _, m := range d.PermissionModes {
		if m == mode {
			return nil
		}
	}
	return fmt.Errorf("%w: %q does not offer the permission mode %q", ErrUnsupportedOption, d.ID, mode)
}

func validateResume(d Descriptor, resume string) error {
	if resume == "" {
		return nil
	}
	if !d.SupportsResume {
		return fmt.Errorf("%w: %q cannot resume a prior session", ErrUnsupportedOption, d.ID)
	}
	return nil
}

func validatePrompt(d Descriptor, prompt string) error {
	if prompt == "" {
		return nil
	}
	if !d.SupportsOpeningPrompt {
		// A shell has no way to receive a sentence that is not "execute this
		// as a command", and silently executing what the user typed into a
		// prompt box is a footgun.
		return fmt.Errorf("%w: %q does not accept an opening prompt", ErrUnsupportedOption, d.ID)
	}
	return nil
}
```

- [ ] **Step 10: Run the whole harness package and watch it pass**

Run: `go test ./control-plane/internal/harness/... -race -count=1 -v`

Expected: PASS — 28 tests in total: 8 catalogue, 16 argv, 4 model. `ok github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness`.

Failure modes, with causes: `TestBuildKeepsACommaInTheOpeningPromptAsOneArgument` reporting 4 elements instead of 3 means something in `claudeArgv` is splitting the prompt — the only correct treatment is a single `append`. `TestBuildRejectsEveryAgentOptionOnTheShell` failing on the `resume` case with a nil error means `SupportsResume` was left true on the shell descriptor. `TestBuildAskEmitsNoPermissionFlag` failing means `claudePermissionFlag` gained an entry for `PermissionAsk`; it must have exactly two keys. `TestBuildChecksTheModelBeforeTheEffort` reporting an error that names effort means `validateModel` and `validateEffort` are called the other way round in `Build`.

- [ ] **Step 11: Write the failing worktree-seam tests**

`worktree` is twenty lines of code carrying a contract restated in three plans, so the tests are mostly about the contract. The one behavioural rule worth the file is that `Path` **stats**: a provider that returns a plausible path for a directory that is not there pushes the failure into Docker's bind-mount error, which surfaces to the user as an unreadable daemon message about an invalid mount spec rather than "that worktree is gone".

Create `control-plane/internal/worktree/worktree_test.go`:

```go
package worktree_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// Phase 4's contextwrite.WorktreeResolver declares exactly this method set, so
// one type satisfies both with no adapter. If this line stops compiling, the
// seam has drifted.
var _ worktree.Provider = worktree.Static{}

func TestStaticResolvesAKnownIdToItsDirectory(t *testing.T) {
	dir := t.TempDir()
	s := worktree.Static{"w-4a77b1c9": dir}

	got, err := s.Path(context.Background(), "w-4a77b1c9")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != dir {
		t.Fatalf("Path = %q, want %q", got, dir)
	}
}

func TestStaticRejectsAnUnknownId(t *testing.T) {
	s := worktree.Static{"w-4a77b1c9": t.TempDir()}

	_, err := s.Path(context.Background(), "w-deadbeef")
	if !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("Path of an unknown id = %v, want an error wrapping ErrUnknownID", err)
	}
}

func TestNilStaticReportsUnknownRatherThanPanicking(t *testing.T) {
	var s worktree.Static

	_, err := s.Path(context.Background(), "w-4a77b1c9")
	if !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("Path on a nil Static = %v, want an error wrapping ErrUnknownID", err)
	}
}

// The reason Path stats. Without it the bad path reaches Docker's bind-mount
// validation and the user sees a daemon error about an invalid mount spec.
func TestStaticRejectsADirectoryThatIsNotThere(t *testing.T) {
	s := worktree.Static{"w-4a77b1c9": filepath.Join(t.TempDir(), "removed")}

	_, err := s.Path(context.Background(), "w-4a77b1c9")
	if !errors.Is(err, worktree.ErrNotADir) {
		t.Fatalf("Path of a missing directory = %v, want an error wrapping ErrNotADir", err)
	}
}

func TestStaticRejectsAPathThatIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s := worktree.Static{"w-4a77b1c9": file}

	_, err := s.Path(context.Background(), "w-4a77b1c9")
	if !errors.Is(err, worktree.ErrNotADir) {
		t.Fatalf("Path of a regular file = %v, want an error wrapping ErrNotADir", err)
	}
}

// The contract says ABSOLUTE and it means it: Phase 4 writes CLAUDE.md at this
// path and Docker binds it, and neither resolves a relative path against the
// control plane's working directory in the way a caller would assume.
func TestStaticReturnsACleanedAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	s := worktree.Static{"w-4a77b1c9": filepath.Join(dir, "sub", "..") + string(filepath.Separator) + "."}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got, err := s.Path(context.Background(), "w-4a77b1c9")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("Path = %q, want an absolute path", got)
	}
	if got != dir {
		t.Fatalf("Path = %q, want the cleaned %q", got, dir)
	}
}

func TestStaticHonoursACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := worktree.Static{"w-4a77b1c9": t.TempDir()}

	_, err := s.Path(ctx, "w-4a77b1c9")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Path with a cancelled context = %v, want context.Canceled", err)
	}
}
```

- [ ] **Step 12: Run the worktree tests and watch them fail**

Run: `go test ./control-plane/internal/worktree/...`

Expected: FAIL — `control-plane/internal/worktree: no non-test Go files in <abs path>`, then `FAIL github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree [build failed]`. Same shape as step 2: the directory holds only an external `_test.go`, so the package resolves and is empty. Not `no required module provides package …`; see step 2 for why that difference is worth holding on to.

Failure mode to watch for: `directory prefix . does not contain modules listed in go.work` means `go test ./...` was run from the workspace root, which is not a module. Use the explicit package path.

- [ ] **Step 13: Write the seam**

Create `control-plane/internal/worktree/worktree.go`:

```go
// Package worktree resolves a worktree id to a directory on disk.
//
// PHASE 3 SEAM — this contract is restated in the Phase 3 plan and, in
// identical words, in Phase 4's contextwrite package:
//  1. Path returns an ABSOLUTE path to an EXISTING directory that is the root
//     of a checked-out worktree. It never creates, moves or removes anything.
//  2. Phase 3 owns creating, listing and removing worktrees. Phase 2 owns only
//     resolution and read-only enumeration.
//  3. Anything that must be on disk before an agent starts — Phase 4 writes
//     CLAUDE.md here — is written BEFORE the sandbox is created, never after.
//     Claude Code reads CLAUDE.md at session start; writing it afterwards means
//     the session inherits nothing, silently, with no error anywhere.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrUnknownID is returned for an id the provider does not know.
var ErrUnknownID = errors.New("worktree: unknown worktree id")

// ErrNotADir is returned when the resolved path is not an existing directory.
var ErrNotADir = errors.New("worktree: path is not an existing directory")

// Provider resolves worktree ids. The method set is deliberately one method:
// Phase 4's contextwrite.WorktreeResolver declares exactly this signature, so
// one type satisfies both with no adapter.
type Provider interface {
	Path(ctx context.Context, worktreeID string) (string, error)
}

// Static is the implementation Phase 2 ships when no projects directory is
// configured, and the one every test uses.
type Static map[string]string

// Path implements Provider. It verifies the directory exists, because a
// Provider that returns a plausible path for a directory that is not there
// pushes the failure into Docker's bind-mount error, which is unreadable.
func (s Static) Path(ctx context.Context, worktreeID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	raw, ok := s[worktreeID]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownID, worktreeID)
	}
	return Verify(raw)
}

// Verify cleans a path, makes it absolute and confirms it is a directory. It
// is exported because project.Scanner resolves ids a completely different way
// and must end with exactly this check — one implementation, so the two
// providers cannot disagree about what "resolvable" means.
func Verify(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrNotADir, path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrNotADir, abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %q is a file", ErrNotADir, abs)
	}
	return abs, nil
}
```

`Verify` is an addition to the contract's declared surface. It is justified: `project.Scanner` must apply the identical existence check after deriving a path from `git worktree list`, and two copies of "absolute, cleaned, is a directory" is two chances for the two providers to disagree about what resolution means. It adds no behaviour the contract did not already require of `Static.Path`.

- [ ] **Step 14: Run the worktree tests and watch them pass**

Run: `go test ./control-plane/internal/worktree/... -race -count=1 -v`

Expected: PASS — seven tests, `ok github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree`.

Failure mode: `TestStaticReturnsACleanedAbsolutePath` failing with a path ending in `\.` on Windows means `Verify` stats the raw string and returns it rather than returning the `filepath.Abs` result.

- [ ] **Step 15: Write the failing enumeration tests**

These drive a scripted fake `Runner`, so the suite needs neither `git` on `PATH` nor a repository anywhere on the machine — only directories it creates in `t.TempDir()`. Two of them are the load-bearing ones. `TestProjectsFindsDepth1RepositoriesOnly` pins the scan depth, because recursing a developer's whole disk is how a rail takes forty seconds to paint. `TestEveryGitCommandIsReadOnly` is a whitelist assertion over the recorded calls: Phase 3 owns worktree lifecycle, and the cheapest way to keep Phase 2 out of it is to fail the build the moment a write subcommand appears.

Create `control-plane/internal/project/project_test.go`:

```go
package project_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// project.Scanner must satisfy the Phase 3 seam with no adapter; Phase 4's
// contextwrite.NewWriter takes it directly.
var _ worktree.Provider = (*project.Scanner)(nil)

type fakeGit struct {
	mu      sync.Mutex
	replies map[string]string
	fails   map[string]error
	calls   []string
}

func newFakeGit() *fakeGit {
	return &fakeGit{replies: map[string]string{}, fails: map[string]error{}}
}

func gitKey(dir string, args []string) string { return dir + "\x00" + strings.Join(args, " ") }

func (f *fakeGit) reply(dir, out string, args ...string) {
	f.replies[gitKey(dir, args)] = out
}

func (f *fakeGit) fail(dir string, err error, args ...string) {
	f.fails[gitKey(dir, args)] = err
}

func (f *fakeGit) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls = append(f.calls, strings.Join(args, " "))
	k := gitKey(dir, args)
	if err, ok := f.fails[k]; ok {
		return nil, err
	}
	out, ok := f.replies[k]
	if !ok {
		return nil, fmt.Errorf("fakeGit: no scripted reply for %q in %q", strings.Join(args, " "), dir)
	}
	return []byte(out), nil
}

func (f *fakeGit) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// makeRepo creates root/name with a .git directory in it. No git binary is
// involved: Scanner decides "is this a repo" from the presence of .git.
func makeRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	return dir
}

// An unconfigured projects directory is a normal state, not a failure: a fresh
// install has one, and the cockpit renders an empty rail rather than an error.
func TestAnUnconfiguredRootReturnsEmptyResultsAndNeverRunsGit(t *testing.T) {
	g := newFakeGit()
	s := project.NewScanner("", g)
	ctx := context.Background()

	projects, err := s.Projects(ctx)
	if err != nil || len(projects) != 0 {
		t.Fatalf("Projects() = %v, %v; want an empty slice and no error", projects, err)
	}
	branches, err := s.Branches(ctx, "p-4a77b1c9")
	if err != nil || len(branches) != 0 {
		t.Fatalf("Branches() = %v, %v; want an empty slice and no error", branches, err)
	}
	worktrees, err := s.Worktrees(ctx, "p-4a77b1c9")
	if err != nil || len(worktrees) != 0 {
		t.Fatalf("Worktrees() = %v, %v; want an empty slice and no error", worktrees, err)
	}
	if calls := g.recorded(); len(calls) != 0 {
		t.Fatalf("git ran %v with no root configured; there is nothing to enumerate", calls)
	}
}
```

Continuing `control-plane/internal/project/project_test.go`, appended below `TestAnUnconfiguredRootReturnsEmptyResultsAndNeverRunsGit`:

```go
// Depth-1 under root, and nothing else. Deeper recursion over a developer's
// whole disk is how a rail takes forty seconds to paint.
func TestProjectsFindsDepth1RepositoriesOnly(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "panacea")
	makeRepo(t, root, "eunomia")
	// A linked worktree has .git as a FILE, not a directory. It is still a
	// repository as far as enumeration is concerned.
	linked := filepath.Join(root, "linked")
	if err := os.MkdirAll(linked, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte("gitdir: /elsewhere\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// A dot-prefixed directory holding .git is a repository like any other. It
	// is here to pin the absence of a hidden-name filter: the contract's rule
	// is .git, full stop, and a filter would drop a project the user put there
	// with no way to tell that it had been dropped.
	makeRepo(t, root, ".cache")
	// Not repositories, and must not be reported.
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeRepo(t, root, filepath.Join("nested", "deep")) // depth 2
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := project.NewScanner(root, newFakeGit()).Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, p.Name)
		if !filepath.IsAbs(p.Path) {
			t.Errorf("project %q has a relative path %q", p.Name, p.Path)
		}
		if !strings.HasPrefix(p.ID, "p-") || len(p.ID) != 10 {
			t.Errorf("project %q has id %q, want \"p-\" plus 8 hex characters", p.Name, p.ID)
		}
	}
	// os.ReadDir sorts by filename, and "." sorts before every letter.
	want := []string{".cache", "eunomia", "linked", "panacea"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("Projects() names = %v, want %v in that order", names, want)
	}
}

// Ids are derived, not stored: a restart produces the same ids with no storage
// and no migration, which is why registry.Session can hold a worktreeId across
// a reboot.
func TestProjectIdsAreDerivedFromThePathAndAreStable(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")

	first, err := project.NewScanner(root, newFakeGit()).Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	second, err := project.NewScanner(root, newFakeGit()).Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("two scanners produced different ids %q and %q for the same path", first[0].ID, second[0].ID)
	}
	if got := project.ProjectID(dir); got != first[0].ID {
		t.Fatalf("ProjectID(%q) = %q, want %q", dir, got, first[0].ID)
	}
	// Windows paths are case-insensitive, so the id is computed from the
	// lowercased path and C:\Users\x and c:\users\x must not be two projects.
	if got := project.ProjectID(strings.ToUpper(dir)); got != first[0].ID {
		t.Fatalf("ProjectID is case-sensitive: %q != %q", got, first[0].ID)
	}
}

func TestBranchesMarksTheCheckedOutBranch(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	g := newFakeGit()
	g.reply(dir, "main\nevidence-upload\nfeat/ready-flag\n", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	g.reply(dir, "evidence-upload\n", "rev-parse", "--abbrev-ref", "HEAD")

	s := project.NewScanner(root, g)
	got, err := s.Branches(context.Background(), project.ProjectID(dir))
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Branches() = %v, want 3", got)
	}
	head := 0
	for _, b := range got {
		if b.IsHead {
			head++
			if b.Name != "evidence-upload" {
				t.Errorf("IsHead is set on %q, want evidence-upload", b.Name)
			}
		}
		if b.ProjectID != project.ProjectID(dir) {
			t.Errorf("branch %q carries projectId %q", b.Name, b.ProjectID)
		}
	}
	if head != 1 {
		t.Errorf("%d branches are marked IsHead, want exactly 1", head)
	}
	// The id is slugged; the display name is not touched.
	if got[2].Name != "feat/ready-flag" {
		t.Errorf("Branches()[2].Name = %q, want the real ref name %q", got[2].Name, "feat/ready-flag")
	}
	wantID := project.BranchID(project.ProjectID(dir), "feat/ready-flag")
	if got[2].ID != wantID || !strings.HasSuffix(wantID, "-feat-ready-flag") {
		t.Errorf("Branches()[2].ID = %q, want %q ending in the slug", got[2].ID, wantID)
	}
}
```

Continuing `control-plane/internal/project/project_test.go`, appended below `TestBranchesMarksTheCheckedOutBranch`:

```go
func TestBranchesOnAnUnknownProjectIsASentinelTheApiCanMapTo404(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "panacea")

	_, err := project.NewScanner(root, newFakeGit()).Branches(context.Background(), "p-deadbeef")
	if !errors.Is(err, project.ErrUnknownProject) {
		t.Fatalf("Branches of an unknown project = %v, want an error wrapping ErrUnknownProject", err)
	}
}

// git emits forward slashes in porcelain output on every platform, including
// Windows. The scanner must clean them to the OS separator or every derived
// worktree id differs from the one Path computes from a real directory.
func worktreePorcelain(dirs ...string) string {
	var b strings.Builder
	for i, d := range dirs {
		fmt.Fprintf(&b, "worktree %s\n", filepath.ToSlash(d))
		fmt.Fprintf(&b, "HEAD %040d\n", i)
		fmt.Fprintf(&b, "branch refs/heads/wt-%d\n", i)
		if i == 2 {
			// git emits either a branch line or "detached", never both. Record
			// 2 carries both on purpose: with only the realistic shape, a
			// parser that ignores the marker entirely still reports an empty
			// BranchID, so the test could not tell a parser that handles a
			// detached HEAD from one that has never heard of it.
			b.WriteString("detached\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestWorktreesParsesPorcelainOutput(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	linked := filepath.Join(t.TempDir(), "evidence-upload")
	detached := filepath.Join(t.TempDir(), "spike")
	for _, d := range []string{linked, detached} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, linked, detached), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Worktrees(context.Background(), project.ProjectID(dir))
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Worktrees() = %#v, want 3 records", got)
	}
	if !got[0].IsPrimary {
		t.Error("the first record is not marked primary; git lists the main worktree first")
	}
	if got[1].IsPrimary || got[2].IsPrimary {
		t.Error("more than one worktree is marked primary")
	}
	if got[1].Name != "evidence-upload" || got[1].Path != linked {
		t.Errorf("Worktrees()[1] = {Name:%q Path:%q}, want {evidence-upload %q}", got[1].Name, got[1].Path, linked)
	}
	if got[1].BranchID != project.BranchID(project.ProjectID(dir), "wt-1") {
		t.Errorf("Worktrees()[1].BranchID = %q, want the derived id for wt-1", got[1].BranchID)
	}
	// A detached HEAD belongs to no branch. Inventing one would make the rail
	// claim a branch that does not exist, and the fixture puts a branch line in
	// that record so only a parser that acts on "detached" can clear it.
	if got[2].BranchID != "" {
		t.Errorf("a detached worktree carries BranchID %q, want empty", got[2].BranchID)
	}
	if got[2].ID != project.WorktreeID(detached) {
		t.Errorf("Worktrees()[2].ID = %q, want %q", got[2].ID, project.WorktreeID(detached))
	}
}

func TestPathResolvesAWorktreeIdThroughTheProvider(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	linked := filepath.Join(t.TempDir(), "evidence-upload")
	if err := os.MkdirAll(linked, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, linked), "worktree", "list", "--porcelain")
	s := project.NewScanner(root, g)

	got, err := s.Path(context.Background(), project.WorktreeID(linked))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != linked {
		t.Fatalf("Path = %q, want %q", got, linked)
	}

	if _, err := s.Path(context.Background(), "w-deadbeef"); !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("Path of an unknown id = %v, want an error wrapping worktree.ErrUnknownID", err)
	}
}

// git's metadata can outlive the directory: delete a worktree folder by hand
// and `git worktree list` still reports it until someone prunes. Resolving it
// must fail here, not inside Docker's bind-mount validation.
func TestPathOnAWorktreeGitStillListsButDiskHasLostIsErrNotADir(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	ghost := filepath.Join(t.TempDir(), "pruned-by-hand")
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, ghost), "worktree", "list", "--porcelain")

	_, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(ghost))
	if !errors.Is(err, worktree.ErrNotADir) {
		t.Fatalf("Path of a listed-but-absent worktree = %v, want an error wrapping worktree.ErrNotADir", err)
	}
}
```

The last block of `control-plane/internal/project/project_test.go`, appended below `TestPathOnAWorktreeGitStillListsButDiskHasLostIsErrNotADir`:

```go
// Phase 4's Naming contract: never an error, and fall back to the id, because
// a missing display name must not fail a brief. The cache warms as a side
// effect of enumeration, so a cold scanner returns the id and a warm one
// returns the name.
func TestNamesFallBackToTheIdUntilEnumerationHasWarmedTheCache(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	pid := project.ProjectID(dir)
	bid := project.BranchID(pid, "feat/ready-flag")

	g := newFakeGit()
	g.reply(dir, "main\nfeat/ready-flag\n", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	g.reply(dir, "main\n", "rev-parse", "--abbrev-ref", "HEAD")
	s := project.NewScanner(root, g)

	if got := s.ProjectName(pid); got != pid {
		t.Errorf("cold ProjectName = %q, want the id %q", got, pid)
	}
	if got := s.BranchName(bid); got != bid {
		t.Errorf("cold BranchName = %q, want the id %q", got, bid)
	}
	if got := s.ProjectName("p-deadbeef"); got != "p-deadbeef" {
		t.Errorf("ProjectName of an unknown id = %q, want the id back", got)
	}

	if _, err := s.Projects(context.Background()); err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if _, err := s.Branches(context.Background(), pid); err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if got := s.ProjectName(pid); got != "panacea" {
		t.Errorf("warm ProjectName = %q, want %q", got, "panacea")
	}
	if got := s.BranchName(bid); got != "feat/ready-flag" {
		t.Errorf("warm BranchName = %q, want %q", got, "feat/ready-flag")
	}
}

// Phase 2 owns resolution and read-only enumeration; docs/PROGRAM.md phase 3
// owns worktree lifecycle. This is the cheap mechanical guard: a whitelist over
// every git invocation the package makes.
func TestEveryGitCommandIsReadOnly(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	pid := project.ProjectID(dir)
	g := newFakeGit()
	g.reply(dir, "main\n", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	g.reply(dir, "main\n", "rev-parse", "--abbrev-ref", "HEAD")
	g.reply(dir, worktreePorcelain(dir), "worktree", "list", "--porcelain")

	s := project.NewScanner(root, g)
	ctx := context.Background()
	if _, err := s.Projects(ctx); err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if _, err := s.Branches(ctx, pid); err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if _, err := s.Worktrees(ctx, pid); err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if _, err := s.Path(ctx, project.WorktreeID(dir)); err != nil {
		t.Fatalf("Path: %v", err)
	}

	allowed := map[string]bool{
		"for-each-ref --format=%(refname:short) refs/heads": true,
		"rev-parse --abbrev-ref HEAD":                       true,
		"worktree list --porcelain":                         true,
	}
	calls := g.recorded()
	if len(calls) == 0 {
		t.Fatal("no git commands were recorded; the fake is not wired to the scanner")
	}
	for _, c := range calls {
		if !allowed[c] {
			t.Errorf("git %s is not on the read-only whitelist; Phase 3 owns anything that writes", c)
		}
	}
}

// A git failure must name git's own message, or the rail shows "exit status
// 128" and the user has nothing to act on.
func TestAGitFailureCarriesTheMessageUpward(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	g := newFakeGit()
	g.fail(dir, errors.New("fatal: not a git repository"), "for-each-ref", "--format=%(refname:short)", "refs/heads")

	_, err := project.NewScanner(root, g).Branches(context.Background(), project.ProjectID(dir))
	if err == nil {
		t.Fatal("Branches on a failing git returned no error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %q, want git's own message in it", err)
	}
}
```

- [ ] **Step 16: Run the enumeration tests and watch them fail**

Run: `go test ./control-plane/internal/project/...`

Expected: FAIL — `control-plane/internal/project: no non-test Go files in <abs path>`, then `FAIL github.com/kpmg-au-emu/panacea-app/control-plane/internal/project [build failed]`. Same shape as steps 2 and 12: only an external `_test.go` is present, so the package resolves inside the module and is empty. Not `no required module provides package …` — that one would mean a misspelled import path, never a missing dependency, and never a reason to run `go get`.

Failure mode to watch for: `directory prefix . does not contain modules listed in go.work` means `go test ./...` was run from the workspace root, which is not a module. Use the explicit package path.

- [ ] **Step 17: Write the enumeration**

Four decisions the contract did not make, all visible in this file. **`ErrUnknownProject` is new**: the contract gives `Branches` and `Worktrees` an error return but no sentinel, and task 10 must map "no such project" to `404 project_not_found` without string-matching. **`ProjectName`/`BranchName` read a cache warmed by enumeration**, rather than shelling out: the contract forbids them an error return and a `context`, so a live lookup would have to invent both, and the documented fallback — return the id — is exactly the right answer for a cold cache. **`Path` skips a project whose `git worktree list` fails** instead of failing the whole resolution, because one broken checkout under the projects root must not make every other worktree unresolvable. **A configured root that does not exist yet returns an empty slice**, like an unconfigured one: a laptop that has not created `~/Projects` is not in an error state.

Create `control-plane/internal/project/project.go`:

```go
// Package project enumerates the projects, branches and worktrees the cockpit's
// rail shows. It is READ-ONLY by design: docs/PROGRAM.md phase 3 owns worktree
// lifecycle, and a Phase 2 that could create or delete one would be building
// Phase 3 badly. Three git subcommands are used, all of them reads —
// for-each-ref, rev-parse and worktree list — and project_test.go asserts that
// whitelist mechanically.
package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// ErrUnknownProject is returned for a project id under no configured root. The
// contract leaves the error unnamed; task 10 needs a sentinel to map to
// 404 project_not_found without matching on message text.
var ErrUnknownProject = errors.New("project: unknown project id")

type Project struct {
	ID   string `json:"id"`   // "p-" + first 8 hex of sha256(absolute path)
	Name string `json:"name"` // the directory's base name
	Path string `json:"path"` // absolute
}

type Branch struct {
	ID        string `json:"id"` // "b-" + project hash + "-" + slug(name)
	ProjectID string `json:"projectId"`
	Name      string `json:"name"` // e.g. "evidence-upload"
	IsHead    bool   `json:"isHead"`
}

type Worktree struct {
	ID        string `json:"id"` // "w-" + first 8 hex of sha256(absolute path)
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"` // empty for a detached or bare worktree
	Name      string `json:"name"`     // base name of the worktree directory
	Path      string `json:"path"`     // absolute
	IsPrimary bool   `json:"isPrimary"`
}

// Runner runs git. It is an interface so tests need neither git nor a repo.
//
// An implementation returns git's STANDARD OUTPUT ONLY on success. Everything
// in this file splits that on newlines and trusts every line, so stderr — where
// git puts non-fatal warnings while still exiting 0 — must not be in it. On
// failure the error should carry git's own message; see gitrunner.go.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Scanner enumerates one root directory of repositories.
type Scanner struct {
	root string
	git  Runner

	// names is the display-name cache behind ProjectName and BranchName. Those
	// two satisfy Phase 4's Naming, which forbids an error return and takes no
	// context, so they cannot shell out. Enumeration warms this as a side
	// effect; a cold read returns the id, which is the documented fallback.
	mu    sync.RWMutex
	names map[string]string
}

// NewScanner returns a scanner over root. root may be "", in which case every
// method returns an empty slice and no error — an unconfigured projects
// directory is a normal state, not a failure.
func NewScanner(root string, git Runner) *Scanner {
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		root = filepath.Clean(root)
	}
	return &Scanner{root: root, git: git, names: map[string]string{}}
}

// hashPath is the id derivation: absolute, cleaned, lowercased, sha256, first
// 8 hex. Lowercasing is for Windows, where C:\Users\x and c:\users\x are one
// directory and must not become two rail entries. On a case-sensitive
// filesystem it can merge two genuinely different paths; these are rail ids,
// not a security boundary, and the cost of the alternative is a rail that
// duplicates a project after the user types a path with different casing.
func hashPath(path string) string {
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	return hex.EncodeToString(sum[:])[:8]
}

// ProjectID derives a project id from a directory path.
func ProjectID(path string) string { return "p-" + hashPath(path) }

// WorktreeID derives a worktree id from a directory path.
func WorktreeID(path string) string { return "w-" + hashPath(path) }

// BranchID derives a branch id. It reuses the project's hash so a branch id
// carries its project, which is what lets BranchName work without a lookup.
func BranchID(projectID, name string) string {
	return "b-" + strings.TrimPrefix(projectID, "p-") + "-" + slug(name)
}

// slug lowercases and replaces every run of characters outside [a-z0-9] with a
// single "-", and does nothing else. It is lossy — "feat/x" and "feat-x" slug
// the same — which is why Branch.Name carries the real ref name and only the id
// is slugged.
//
// No trimming of leading or trailing dashes and no fallback for an empty
// result, tempting as both are: a b- id is reproducible from the contract's
// rule with no storage, so an extra rule here means any other phase deriving an
// id for the same branch computes a different one. "-wip-" slugs to "-wip-".
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return b.String()
}
```

Continuing `control-plane/internal/project/project.go`, appended below `slug`:

```go
func (s *Scanner) remember(id, name string) {
	if id == "" || name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.names[id] = name
}

func (s *Scanner) cached(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n, ok := s.names[id]; ok && n != "" {
		return n
	}
	return id
}

// ProjectName implements Phase 4's discoverapi.Naming. It never returns an
// error and falls back to the id, because a missing display name must not fail
// a brief.
func (s *Scanner) ProjectName(projectID string) string { return s.cached(projectID) }

// BranchName implements Phase 4's discoverapi.Naming, on the same terms.
func (s *Scanner) BranchName(branchID string) string { return s.cached(branchID) }

// Projects lists the repositories directly under root. Scanning is depth-1:
// subdirectories of root that contain a .git entry. Deeper recursion over a
// developer's whole disk is how a rail takes forty seconds to paint.
//
// The .git entry is the whole test. A dot-prefixed directory is not skipped:
// the rail would be missing a project the user can see in the folder, with
// nothing anywhere saying why.
func (s *Scanner) Projects(ctx context.Context) ([]Project, error) {
	if s.root == "" {
		return []Project{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// A configured root the user has not created yet is the same
			// normal state as an unconfigured one.
			return []Project{}, nil
		}
		return nil, fmt.Errorf("project: read %s: %w", s.root, err)
	}
	out := make([]Project, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.root, name)
		// A linked worktree has .git as a FILE, a normal clone as a directory.
		// Lstat accepts both and costs no subprocess.
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		p := Project{ID: ProjectID(dir), Name: name, Path: dir}
		s.remember(p.ID, p.Name)
		out = append(out, p)
	}
	return out, nil
}

func (s *Scanner) findProject(ctx context.Context, projectID string) (Project, error) {
	projects, err := s.Projects(ctx)
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.ID == projectID {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("%w: %q", ErrUnknownProject, projectID)
}

// Branches lists local heads and marks the checked-out one.
func (s *Scanner) Branches(ctx context.Context, projectID string) ([]Branch, error) {
	if s.root == "" {
		return []Branch{}, nil
	}
	p, err := s.findProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out, err := s.git.Run(ctx, p.Path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("project: list branches of %s: %w", p.Name, err)
	}
	// A failure here is tolerated: a repository with no commits has no HEAD to
	// resolve, and losing the IsHead marker is better than losing the list.
	head := ""
	if raw, err := s.git.Run(ctx, p.Path, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		head = strings.TrimSpace(string(raw))
	}

	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	branches := make([]Branch, 0, len(lines))
	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		b := Branch{
			ID:        BranchID(p.ID, name),
			ProjectID: p.ID,
			Name:      name,
			IsHead:    name == head,
		}
		s.remember(b.ID, b.Name)
		branches = append(branches, b)
	}
	return branches, nil
}

// Worktrees lists the repository's worktrees. It reports what git reports and
// verifies nothing on disk: Path is where a listed-but-absent worktree is
// caught, because that is the call that is about to hand a path to a mount.
func (s *Scanner) Worktrees(ctx context.Context, projectID string) ([]Worktree, error) {
	if s.root == "" {
		return []Worktree{}, nil
	}
	p, err := s.findProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out, err := s.git.Run(ctx, p.Path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("project: list worktrees of %s: %w", p.Name, err)
	}
	return s.parseWorktrees(p, string(out)), nil
}
```

The last block of `control-plane/internal/project/project.go`, appended below `Worktrees`:

```go
// parseWorktrees reads `git worktree list --porcelain`. Records are separated
// by a blank line and each begins with a "worktree <path>" line; git emits
// forward slashes on every platform, including Windows, so the path is cleaned
// to the OS separator before it is hashed — otherwise the derived id would not
// match the one Path computes from a real directory.
func (s *Scanner) parseWorktrees(p Project, out string) []Worktree {
	var res []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			res = append(res, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			path := filepath.Clean(strings.TrimPrefix(line, "worktree "))
			cur = &Worktree{
				ID:        WorktreeID(path),
				ProjectID: p.ID,
				Name:      filepath.Base(path),
				Path:      path,
				// git lists the main worktree first, always.
				IsPrimary: len(res) == 0,
			}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "branch refs/heads/"):
			name := strings.TrimPrefix(line, "branch refs/heads/")
			cur.BranchID = BranchID(p.ID, name)
			s.remember(cur.BranchID, name)
		case line == "detached" || line == "bare":
			// No branch. Inventing one would make the rail claim a branch that
			// does not exist.
			cur.BranchID = ""
		}
	}
	flush()
	return res
}

// Path implements worktree.Provider: it resolves a worktree id to an absolute,
// existing directory and never creates, moves or removes anything.
//
// A project whose `git worktree list` fails is skipped rather than failing the
// whole resolution — one broken checkout under the root must not make every
// other worktree unresolvable.
func (s *Scanner) Path(ctx context.Context, worktreeID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.root == "" {
		return "", fmt.Errorf("%w: %q (no projects directory is configured)", worktree.ErrUnknownID, worktreeID)
	}
	projects, err := s.Projects(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		worktrees, err := s.Worktrees(ctx, p.ID)
		if err != nil {
			continue
		}
		for _, w := range worktrees {
			if w.ID == worktreeID {
				return worktree.Verify(w.Path)
			}
		}
	}
	return "", fmt.Errorf("%w: %q", worktree.ErrUnknownID, worktreeID)
}
```

- [ ] **Step 18: Run the enumeration tests and watch them pass**

Run: `go test ./control-plane/internal/project/... -race -count=1 -v`

Expected: PASS — eleven tests, `ok github.com/kpmg-au-emu/panacea-app/control-plane/internal/project`.

Failure modes, with causes. `TestWorktreesParsesPorcelainOutput` failing on `Worktrees()[1].Path` with a forward-slash path on Windows means `parseWorktrees` is not calling `filepath.Clean` on the parsed path; the derived id will not match `WorktreeID` of a real directory either, so `TestPathResolvesAWorktreeIdThroughTheProvider` fails with `ErrUnknownID` in the same run. `TestEveryGitCommandIsReadOnly` reporting `no scripted reply` rather than a whitelist violation means the implementation's argument list differs from the test's by a character — the format string is `%(refname:short)` with no spaces. `TestNamesFallBackToTheIdUntilEnumerationHasWarmedTheCache` failing on the warm read means `remember` is being called on a copy of the scanner; `Scanner` must be used through the pointer `NewScanner` returns, which is also what makes the `sync.RWMutex` safe. `TestWorktreesParsesPorcelainOutput` failing only on `Worktrees()[2].BranchID` means `parseWorktrees` has no `detached`/`bare` arm; the fixture's third record carries a branch line before the marker so a missing arm shows up here rather than on a user's detached worktree. `TestProjectsFindsDepth1RepositoriesOnly` reporting three names instead of four means a hidden-name filter crept into the `os.ReadDir` loop; the only test is `.git`.

- [ ] **Step 19: Write the failing ExecRunner tests**

These are the only tests in the task that touch a real `git`, and they still touch no repository that already exists on the machine: each one builds a repository from nothing inside `t.TempDir()`, with `GIT_CONFIG_GLOBAL` and `GIT_CONFIG_SYSTEM` pointed at empty files so the developer's own git config — `init.defaultBranch`, hooks, templates, a signing key that prompts — cannot change the outcome. `t.Skip` when `git` is absent keeps the suite green on a container image that has none.

The fixture builder does run write subcommands (`init`, `add`, `commit`, `worktree add`). That is the test constructing its own world, not the package mutating the user's: nothing in `project.go` or `gitrunner.go` can reach them, and `TestEveryGitCommandIsReadOnly` proves it.

Create `control-plane/internal/project/gitrunner_test.go`:

```go
package project_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH; skipping the ExecRunner tests")
	}
}

// makeGitRepo builds a real repository from nothing. The developer's own git
// config is neutralised so init.defaultBranch, templates, hooks and signing
// cannot change what the test observes. Requires git >= 2.28 for `init -b`.
func makeGitRepo(t *testing.T, dir string) {
	t.Helper()
	requireGit(t)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cfg := t.TempDir()
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(cfg, "gitconfig"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(cfg, "gitconfig-system"),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial")
	run("branch", "evidence-upload")
}

func TestExecRunnerReturnsGitsOutput(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "panacea")
	makeGitRepo(t, dir)

	out, err := project.NewExecRunner().Run(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "main" {
		t.Fatalf("rev-parse --abbrev-ref HEAD = %q, want %q", got, "main")
	}
}

// "exit status 128" in the rail tells the user nothing. On FAILURE — and only
// on failure — the runner folds both streams into the error so it carries
// git's own sentence. On success the caller gets stdout alone; see
// TestExecRunnerDoesNotLetAGitWarningOnStderrBecomeABranch.
func TestExecRunnerErrorCarriesGitsOwnMessage(t *testing.T) {
	requireGit(t)

	_, err := project.NewExecRunner().Run(context.Background(), t.TempDir(), "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatal("Run in a directory that is not a repository returned no error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %q, want git's own message in it", err)
	}
}

func TestExecRunnerHonoursACancelledContext(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "panacea")
	makeGitRepo(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if _, err := project.NewExecRunner().Run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		t.Fatal("Run with a cancelled context returned no error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run took %s to notice a cancelled context", elapsed)
	}
}
```

The next block of `control-plane/internal/project/gitrunner_test.go`, appended below `TestExecRunnerHonoursACancelledContext`. This is the one test that puts real git output through the real parser, which is the only way to learn that the porcelain format assumed in `parseWorktrees` is the porcelain format git emits.

It is also the one test that compares a path *this test built* against a path *git printed*, and those two are not the same string by default. `os.TempDir()` on the Windows development machine is `C:\Users\AU-DEV~1\AppData\Local\Temp` — the 8.3 short form, because the account name is longer than eight characters — so everything `t.TempDir()` returns carries `AU-DEV~1`, while `git worktree list --porcelain` normalises and prints `worktree C:/Users/au-dev-dmackie1/AppData/Local/Temp/…`. `parseWorktrees` runs only `filepath.Clean` and `worktree.Verify` runs only `filepath.Abs`; neither expands a short component, and neither should — expanding it in production code would mean a syscall per worktree per rail poll to fix a problem only a test has. So the test normalises instead: `filepath.EvalSymlinks` on the root and on the linked worktree, which resolves the short form to the long one on Windows and `/var` to `/private/var` on macOS. Skip it and the final `Path` assertion fails on a correct implementation:

```go
// Two spellings of one directory, without the normalisation below.
got:  "C:\\Users\\au-dev-dmackie1\\AppData\\Local\\Temp\\...\\evidence-upload"
want: "C:\\Users\\AU-DEV~1\\AppData\\Local\\Temp\\...\\evidence-upload"
```

```go
func TestScannerAgainstARealRepositoryAndARealLinkedWorktree(t *testing.T) {
	requireGit(t)

	// Normalise the temp root before a single path is built from it. On the
	// Windows development machine os.TempDir() is the 8.3 short form
	// C:\Users\AU-DEV~1\AppData\Local\Temp, while git normalises every path it
	// prints and emits the long form C:/Users/au-dev-dmackie1/...; on macOS the
	// same call resolves /var to /private/var. parseWorktrees only Cleans and
	// worktree.Verify only Abs's, and neither expands a short component or a
	// symlink, so without this the string this test builds and the string git
	// reports are two spellings of one directory and every path comparison
	// below fails.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(root): %v", err)
	}
	repo := filepath.Join(root, "panacea")
	makeGitRepo(t, repo)

	// A real linked worktree, created OUTSIDE root so it is not also
	// enumerated as a project. The env neutralisation repeats makeGitRepo's on
	// purpose, so this test reads standalone.
	linked := filepath.Join(t.TempDir(), "evidence-upload")
	cfg := t.TempDir()
	add := exec.Command("git", "worktree", "add", linked, "evidence-upload")
	add.Dir = repo
	add.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(cfg, "gitconfig"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(cfg, "gitconfig-system"),
	)
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	// The same normalisation for the worktree, and it has to come after the
	// directory exists: EvalSymlinks fails on a path that is not there yet.
	// This is the value the Path assertion at the bottom compares against.
	linked, err = filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatalf("EvalSymlinks(linked): %v", err)
	}

	s := project.NewScanner(root, project.NewExecRunner())
	ctx := context.Background()

	projects, err := s.Projects(ctx)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "panacea" {
		t.Fatalf("Projects() = %#v, want exactly the panacea repository", projects)
	}
	pid := projects[0].ID

	branches, err := s.Branches(ctx, pid)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	seen := map[string]bool{}
	for _, b := range branches {
		seen[b.Name] = true
		if b.Name == "main" && !b.IsHead {
			t.Error("main is not marked IsHead in the primary worktree's repository")
		}
	}
	if !seen["main"] || !seen["evidence-upload"] {
		t.Fatalf("Branches() = %#v, want main and evidence-upload", branches)
	}

	worktrees, err := s.Worktrees(ctx, pid)
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("Worktrees() = %#v, want the primary and the linked one", worktrees)
	}
	if !worktrees[0].IsPrimary || worktrees[1].IsPrimary {
		t.Errorf("IsPrimary = %v, %v; want only the first record", worktrees[0].IsPrimary, worktrees[1].IsPrimary)
	}
	if worktrees[1].Name != "evidence-upload" {
		t.Errorf("Worktrees()[1].Name = %q, want evidence-upload", worktrees[1].Name)
	}
	if worktrees[1].BranchID != project.BranchID(pid, "evidence-upload") {
		t.Errorf("Worktrees()[1].BranchID = %q, want the derived id", worktrees[1].BranchID)
	}

	got, err := s.Path(ctx, worktrees[1].ID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != linked {
		t.Fatalf("Path = %q, want %q — both sides are EvalSymlinks-normalised, so a "+
			"difference here is a real resolution bug and not a short-name or "+
			"separator artefact: filepath.Clean in parseWorktrees already turns "+
			"git's forward slashes into the platform separator", got, linked)
	}
	if name := s.ProjectName(pid); name != "panacea" {
		t.Errorf("ProjectName after enumeration = %q, want panacea", name)
	}
}
```

The last block of `gitrunner_test.go`, appended below it. Every other test in this file runs a git that either succeeds silently or fails loudly, and neither shape can catch the runner folding stderr into stdout. This one scripts the third shape — **git exits 0 and still writes to stderr** — because that is the one a line-by-line parser gets wrong. A corrupt loose ref makes `for-each-ref` print `warning: ignoring broken ref refs/heads/garbage` on stderr, the real branches on stdout, and exit 0; a runner returning combined output would put that sentence through `Scanner.Branches` and the rail would grow a branch called `warning: ignoring broken ref refs/heads/garbage` with a matching derived id, while `rev-parse --abbrev-ref HEAD` would return `"warning: …\nmain"` and match nothing, so no branch would be marked `IsHead`. Without this test that defect ships green:

```go
func TestExecRunnerDoesNotLetAGitWarningOnStderrBecomeABranch(t *testing.T) {
	requireGit(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(root): %v", err)
	}
	repo := filepath.Join(root, "panacea")
	makeGitRepo(t, repo)

	// A corrupt loose ref. git reports it and carries on: the complaint goes
	// to stderr, the good refs to stdout, and the exit status is 0.
	broken := filepath.Join(repo, ".git", "refs", "heads", "garbage")
	if err := os.WriteFile(broken, []byte("not-a-sha\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Confirm the premise before asserting on it. git's diagnostics are not a
	// stable interface; if this build refuses rather than warns, or says
	// nothing at all, the test has nothing to prove and skips rather than
	// failing for a reason that is not the product's fault.
	probe := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	probe.Dir = repo
	var pout, perr bytes.Buffer
	probe.Stdout, probe.Stderr = &pout, &perr
	if err := probe.Run(); err != nil {
		t.Skipf("this git fails rather than warns on a broken loose ref: %v", err)
	}
	if !strings.Contains(perr.String(), "warning") {
		t.Skip("this git emits no stderr warning for a broken loose ref")
	}
	if strings.Contains(pout.String(), "warning") {
		t.Fatalf("premise broken: git put its warning on stdout:\n%s", pout.String())
	}

	// The runner's own output must be stdout alone.
	out, err := project.NewExecRunner().Run(context.Background(), repo,
		"for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(string(out), "warning") {
		t.Fatalf("Run folded git's stderr into its result:\n%s", out)
	}

	// And the same through the parser, which is where a user would see it.
	s := project.NewScanner(root, project.NewExecRunner())
	ctx := context.Background()
	projects, err := s.Projects(ctx)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("Projects() = %#v, want exactly the panacea repository", projects)
	}
	branches, err := s.Branches(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	names := map[string]bool{}
	head := ""
	for _, b := range branches {
		names[b.Name] = true
		if b.IsHead {
			head = b.Name
		}
	}
	if len(names) != 2 || !names["main"] || !names["evidence-upload"] {
		t.Fatalf("Branches() = %#v, want exactly main and evidence-upload; a stderr warning leaked into the parse", branches)
	}
	if head != "main" {
		t.Fatalf("IsHead branch = %q, want main; rev-parse's stdout carried a stderr warning", head)
	}
}
```

- [ ] **Step 20: Run the ExecRunner tests and watch them fail**

Run: `go test ./control-plane/internal/project/... -run ExecRunner -count=1`

Expected: FAIL to build — `undefined: project.NewExecRunner`.

Failure mode: if the output is `ok … [no tests to run]`, the `-run` pattern matched nothing because the file was not saved. `-run ExecRunner` selects four of the five tests in the file; `TestScannerAgainstARealRepositoryAndARealLinkedWorktree` does not match it and is meant not to — step 22 runs the whole package.

- [ ] **Step 21: Write the git runner**

Create `control-plane/internal/project/gitrunner.go`:

```go
package project

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitTimeout bounds one git invocation. Enumeration runs behind an HTTP
// handler that the rail polls, so a git that blocks — a network remote, a
// credential prompt, an index.lock held by another process — must fail rather
// than hold a request open.
const GitTimeout = 10 * time.Second

// ExecRunner is the production Runner: the only place in the product that
// starts a git process.
type ExecRunner struct {
	// Timeout overrides GitTimeout when non-zero. Tests use it; nothing else
	// needs to.
	Timeout time.Duration
}

var _ Runner = (*ExecRunner)(nil)

// NewExecRunner returns a Runner that shells out to git on PATH.
func NewExecRunner() *ExecRunner { return &ExecRunner{Timeout: GitTimeout} }

// Run executes `git <args...>` in dir and returns its STANDARD OUTPUT ONLY.
//
// The two streams are captured separately and that separation is the whole
// point of this function. git writes non-fatal warnings to stderr and still
// exits 0 — "warning: ignoring broken ref refs/heads/x" is the common one —
// and every caller here parses the result line by line. CombinedOutput on the
// success path would hand that warning to the parser, and the rail would grow
// a branch literally named "warning: ignoring broken ref refs/heads/x" with a
// derived id to match, while rev-parse --abbrev-ref HEAD would return
// "warning: …\nmain" and match no branch, so nothing would be flagged IsHead.
//
// On FAILURE it returns both streams in the error, stderr first. Without that
// the rail shows "exit status 128" and the user has nothing to act on; with it
// the error carries git's own sentence, e.g. "fatal: not a git repository".
func (r *ExecRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = GitTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Headless. A credential helper, an editor or a pager would block until
	// the timeout, and on Windows the Git Credential Manager pops a window.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_PAGER=cat",
		"GCM_INTERACTIVE=never",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// stderr first: on a failure git's own sentence is there, and stdout
		// is usually empty. Both are included because a few subcommands
		// diagnose on stdout, and an error naming nothing is useless.
		detail := strings.TrimSpace(stderr.String() + "\n" + stdout.String())
		return nil, fmt.Errorf("git %s in %s: %v: %s",
			strings.Join(args, " "), dir, err, detail)
	}
	return stdout.Bytes(), nil
}
```

`GIT_OPTIONAL_LOCKS=0` matters more than it looks: without it a plain `git worktree list` can try to refresh the index and contend with an editor's language server for `index.lock`, which turns a read into an intermittent failure on exactly the repository the user is working in.

Two buffers instead of `cmd.CombinedOutput()` is the other line to resist simplifying. The contract's words are "returns the combined output **on failure** so the error names the git message" — on failure, not always — and the asymmetry is load-bearing, because git's exit status and git's chattiness are independent. A warning on stderr with exit 0 is normal git behaviour, and everything downstream of `Run` splits the result on newlines and trusts each line, so the only safe success-path result is `stdout.Bytes()`. The failure path keeps both streams because that is where the user-facing sentence lives.

- [ ] **Step 22: Run the whole project package and watch it pass**

Run: `go test ./control-plane/internal/project/... -race -count=1 -v`

Expected: PASS — sixteen tests, `ok github.com/kpmg-au-emu/panacea-app/control-plane/internal/project`. On a machine without git, the five in `gitrunner_test.go` report `--- SKIP` and the package is still `ok`. One more `--- SKIP` is legitimate even with git present: `TestExecRunnerDoesNotLetAGitWarningOnStderrBecomeABranch` skips itself when the installed git does not warn-and-exit-0 on a broken loose ref, because its premise would be untestable rather than violated.

This task needs no Docker. Git is the only external binary, and on the Windows development machine it is already on `PATH` via Git Bash. Three failure modes worth naming:

1. `TestScannerAgainstARealRepositoryAndARealLinkedWorktree` fails at `git worktree add` with `fatal: invalid reference: evidence-upload` — the fixture's `git branch evidence-upload` ran before the initial commit, and a repository with no commits has no ref to branch from.
2. The same test fails only on the final `Path` comparison, with two paths that look identical apart from `AU-DEV~1` versus `au-dev-dmackie1` — an `EvalSymlinks` call was dropped. Both `root` and `linked` must be normalised; see the note above step 19's last block.
3. `TestExecRunnerDoesNotLetAGitWarningOnStderrBecomeABranch` fails with a branch named `warning: …` in the `Branches()` dump — `ExecRunner.Run` is still returning `CombinedOutput()` on the success path. It must return `stdout.Bytes()`.

- [ ] **Step 23: Run the full Go suite and confirm the toolchain did not move**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected: every package `ok` or `[no test files]`, including the three new ones. Never `go test ./...` — the workspace root is not a module and that form errors with `directory prefix . does not contain modules listed in go.work`.

This task adds no dependency: `harness`, `worktree` and `project` import only the standard library and each other, so no step ran `go get`. Confirm it anyway, because a stray `go mod tidy` in an adjacent task can move the directive and this is the last gate before the commit.

Run: `git diff --stat control-plane/go.mod sandboxd/go.mod go.work`

Expected: **no output at all.** If any of the three appears, run `git diff` on it and check the `go` directive still reads `go 1.23`; CI pins 1.23 and a higher directive makes a network-restricted agent attempt a toolchain download and fail.

- [ ] **Step 24: Commit**

```bash
git add control-plane/internal/harness control-plane/internal/worktree control-plane/internal/project
git commit -m "feat(control-plane): harness catalogue, worktree seam, read-only project enumeration

Build is the producer half of the comma defect. Group A task 3 replaced
sandboxd's comma-joined PANACEA_APP_ARGS with PANACEA_APP_ARGS_JSON so a
prompt could survive transport; this is the end that must never split it in
the first place, and a prompt containing two commas is pinned as a single
argv element.

A shell is a harness, not the absence of one, because the picker offers it as
one of five choices. Effort is Claude Code only, so Build rejects it elsewhere
rather than dropping it silently: there is no channel to report an ignored
option through, and a user who believes they started a high-effort session
that has no effort flag has been lied to by the UI.

worktree.Provider is one method on purpose — Phase 4's contextwrite resolver
declares the same signature, so project.Scanner satisfies both with no
adapter. Enumeration is read-only: Phase 3 owns worktree lifecycle, and a test
asserts the whitelist of git subcommands this package may run.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

