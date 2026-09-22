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
