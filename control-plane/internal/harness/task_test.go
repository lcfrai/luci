package harness_test

import (
	"errors"
	"slices"
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
