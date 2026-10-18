package harness_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/harness"
)

func TestTaskPermissionsNeverChangeInteractiveConsent(t *testing.T) {
	for _, id := range []harness.ID{harness.Codex, harness.ClaudeCode, harness.OpenCode} {
		for _, mode := range []harness.PermissionMode{"", harness.PermissionAsk, harness.PermissionPlan} {
			r := harness.Request{Harness: id, Permission: mode, OpeningPrompt: "review this task"}
			if _, _, err := harness.Build(r); err != nil {
				t.Fatalf("interactive %s/%s must remain available: %v", id, mode, err)
			}
			cmd, argv, err := harness.BuildTask(r)
			if !errors.Is(err, harness.ErrTaskPermission) || cmd != "" || argv != nil {
				t.Fatalf("task %s/%s must refuse without partial argv: %q %v %v", id, mode, cmd, argv, err)
			}
		}
	}
}

func TestTaskUsesPinnedNativeOneShotAndRetainsCapturedContext(t *testing.T) {
	const prompt = "build, verify --literal $(not-a-shell-command)"
	for _, id := range []harness.ID{harness.Codex, harness.ClaudeCode, harness.OpenCode} {
		r := harness.Request{Harness: id, Model: "fixture-model", Permission: harness.PermissionBypass, ReviewedContext: true, Conversation: forkConversation, OpeningPrompt: prompt}
		cmd, argv, err := harness.BuildTask(r)
		if err != nil || cmd == "" {
			t.Fatalf("task %s: %v", id, err)
		}
		joined := strings.Join(argv, "\n")
		if !strings.Contains(joined, harness.ReviewedContextDirectory) || !strings.Contains(argv[len(argv)-1], prompt) || strings.Contains(joined, "--ephemeral") || strings.Contains(joined, "--resume") || strings.Contains(joined, "--continue") {
			t.Fatalf("task lost captured context, prompt or native persistence: %v", argv)
		}
		switch id {
		case harness.Codex:
			at := slices.Index(argv, "exec")
			if at < 0 || slices.Contains(argv[at+1:], "-c") || slices.Contains(argv, "--no-alt-screen") || !slices.Contains(argv, "--dangerously-bypass-approvals-and-sandbox") || argv[len(argv)-1] != prompt {
				t.Fatalf("wrong Codex task scope/flags: %v", argv)
			}
		case harness.ClaudeCode:
			if argv[0] != "--print" || !slices.Contains(argv, forkConversation) || !slices.Contains(argv, "bypassPermissions") || argv[len(argv)-1] != prompt {
				t.Fatalf("wrong Claude task flags/history: %v", argv)
			}
		case harness.OpenCode:
			if argv[0] != "run" || !slices.Contains(argv, "--auto") || slices.Contains(argv, "--interactive") || slices.Contains(argv, "--prompt") {
				t.Fatalf("wrong OpenCode task flags: %v", argv)
			}
		}
		r.Resume = sourceConversation
		if cmd, args, err := harness.BuildTask(r); err == nil || cmd != "" || args != nil {
			t.Fatalf("task silently continued an earlier attempt: %q %v %v", cmd, args, err)
		}
	}
}

func TestTaskCapabilityDoesNotLeakMutablePermissionSlices(t *testing.T) {
	for _, d := range harness.Catalogue() {
		want := d.ID == harness.Codex || d.ID == harness.ClaudeCode || d.ID == harness.OpenCode
		if d.AutonomousTask != want || d.TaskPermissionModes == nil {
			t.Fatalf("wrong task capability: %+v", d)
		}
		if want {
			if !slices.Equal(d.TaskPermissionModes, []harness.PermissionMode{harness.PermissionBypass}) || d.TaskPermissionNotice == "" {
				t.Fatalf("task permission must be explicit: %+v", d)
			}
			d.TaskPermissionModes[0] = harness.PermissionAsk
			next, _ := harness.Lookup(d.ID)
			if next.TaskPermissionModes[0] != harness.PermissionBypass {
				t.Fatal("caller changed authoritative task permissions")
			}
		}
	}
}

func TestTaskNativeDelegationUsesFixedOwnedCompatibilityEntry(t *testing.T) {
	r := harness.Request{Harness: harness.Codex, Model: "opaque-model", Permission: harness.PermissionBypass, DockerBoundary: true, ReviewedContext: true, NativeDelegation: "codex-native-helpers-2-v1", OpeningPrompt: "Implement only the captured coding task"}
	command, argv, err := harness.BuildTask(r)
	if err != nil || command != "/usr/local/bin/node" || len(argv) < 10 || argv[0] != "/usr/local/lib/luci/codex-native-task.cjs" { t.Fatal(command, argv, err) }
	for _, value := range []string{"agents.enabled=true", "agents.max_concurrent_threads_per_session=2", "features.multi_agent_v2=false"} {
		if !slices.Contains(argv, value) { t.Fatal("fixed helper policy missing", value, argv) }
	}
	if slices.Contains(argv, "agents.enabled=false") || argv[len(argv)-1] != r.OpeningPrompt || slices.Index(argv, "exec") < 8 { t.Fatal("conflicting policy or changed prompt", argv) }
	r.NativeDelegation = ""
	command, argv, err = harness.BuildTask(r)
	if err != nil || command != "codex" || !slices.Contains(argv, "agents.enabled=false") || !slices.Contains(argv, "features.multi_agent_v2=false") || slices.Contains(argv, "agents.max_concurrent_threads_per_session=2") { t.Fatal("default task child boundary changed", command, argv, err) }
}

func TestTaskNativeDelegationRejectsUncapturedUnsupportedAndInteractiveRequests(t *testing.T) {
	for _, name := range []string{"unknown", "non-codex", "non-docker", "uncaptured", "ask", "resume"} {
		t.Run(name, func(t *testing.T) {
			r := harness.Request{Harness: harness.Codex, Permission: harness.PermissionBypass, DockerBoundary: true, ReviewedContext: true, NativeDelegation: "codex-native-helpers-2-v1", OpeningPrompt: "Captured task"}
			switch name {
			case "unknown": r.NativeDelegation = "other"
			case "non-codex": r.Harness = harness.ClaudeCode
			case "non-docker": r.DockerBoundary = false
			case "uncaptured": r.ReviewedContext = false
			case "ask": r.Permission = harness.PermissionAsk
			case "resume": r.Resume, r.Conversation = sourceConversation, forkConversation
			}
			command, argv, err := harness.BuildTask(r)
			if err == nil || command != "" || argv != nil { t.Fatal("unsupported helper request produced executable argv", command, argv, err) }
		})
	}
}

func TestTaskJSONOutputIsLimitedToCapturedNativeHelpers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		id          harness.ID
		model       string
		helpers     bool
		interactive bool
	}{
		{"helper-model", harness.Codex, "opaque/json-model", true, false},
		{"helper-default-model", harness.Codex, "default", true, false},
		{"default-codex", harness.Codex, "opaque/json-model", false, false},
		{"default-claude", harness.ClaudeCode, "opaque-model", false, false},
		{"default-opencode", harness.OpenCode, "opaque-model", false, false},
		{"interactive-codex", harness.Codex, "opaque-model", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const prompt = "Captured task: literal --json remains prompt text"
			r := harness.Request{Harness: tc.id, Model: tc.model, Permission: harness.PermissionBypass, DockerBoundary: true, ReviewedContext: true, OpeningPrompt: prompt}
			if tc.helpers {
				r.NativeDelegation = "codex-native-helpers-2-v1"
			}
			build := harness.BuildTask
			if tc.interactive {
				build = harness.Build
			}
			command, argv, err := build(r)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, arg := range argv {
				if arg == "--json" {
					count++
				}
			}
			if !tc.helpers {
				if count != 0 || command == "/usr/local/bin/node" {
					t.Fatal("ordinary or interactive route gained helper JSON output", command, argv)
				}
				return
			}
			at := slices.Index(argv, "exec")
			want := []string{"exec", "--json", "--color", "never", "--dangerously-bypass-approvals-and-sandbox"}
			if tc.model != "default" {
				want = append(want, "--model", tc.model)
			}
			want = append(want, "--", prompt)
			if command != "/usr/local/bin/node" || count != 1 || at < 9 || !slices.Equal(argv[at:], want) {
				t.Fatal("captured helper lost its exact JSON ABI or opaque input", command, argv)
			}
		})
	}
}

func TestCodexTaskDisablesUncapturedChildrenOnlyAtRoot(t *testing.T) {
	const prompt = "implement the captured task --literal agents.enabled=false"
	for _, tc := range []struct {
		name     string
		brief    bool
		reviewed bool
	}{
		{"no-context", false, false},
		{"project-brief", true, false},
		{"reviewed-context", false, true},
		{"reviewed-over-brief", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := harness.Request{Harness: harness.Codex, Model: "fixture-model", Permission: harness.PermissionBypass, ProjectBrief: tc.brief, ReviewedContext: tc.reviewed, OpeningPrompt: prompt}
			context := []string{}
			if tc.brief || tc.reviewed {
				instructions := harness.ProjectBriefPrompt
				if tc.reviewed {
					instructions = harness.ReviewedContextPrompt
				}
				context = []string{"-c", "developer_instructions=" + strconv.Quote(instructions)}
			}
			want := []string{"-c", `cli_auth_credentials_store="file"`, "-c", "agents.enabled=false", "-c", "features.multi_agent_v2=false"}
			want = append(want, context...)
			want = append(want, "exec", "--color", "never", "--dangerously-bypass-approvals-and-sandbox", "--model", r.Model, "--", prompt)
			command, argv, err := harness.BuildTask(r)
			if err != nil || command != "codex" || !slices.Equal(argv, want) {
				t.Fatalf("controller-owned task must disable both upstream child paths without changing its captured inputs: %q %v %v", command, argv, err)
			}
			for _, setting := range []string{"agents.enabled=false", "features.multi_agent_v2=false"} {
				count := 0
				for i, arg := range argv {
					if arg == setting {
						count++
						if i == 0 || argv[i-1] != "-c" || i >= slices.Index(argv, "exec") {
							t.Fatalf("task setting escaped root config: %v", argv)
						}
					}
				}
				if count != 1 {
					t.Fatalf("task setting must occur exactly once: %s in %v", setting, argv)
				}
			}
			interactive := []string{"--no-alt-screen", "--no-daemon", "-c", `cli_auth_credentials_store="file"`}
			interactive = append(interactive, context...)
			interactive = append(interactive, "--model", r.Model, "--dangerously-bypass-approvals-and-sandbox", "--", prompt)
			command, argv, err = harness.Build(r)
			if err != nil || command != "codex" || !slices.Equal(argv, interactive) {
				t.Fatalf("interactive Codex argv changed with task-only child bounds: %q %v %v", command, argv, err)
			}
		})
	}
}
