package harness_test

import (
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"slices"
	"strings"
	"testing"
)

func TestNativeProviderArgumentsPreservePromptsAndForkWithoutSharingIDs(t *testing.T) {
	for _, id := range []harness.ID{harness.Codex, harness.OpenCode} {
		t.Run(string(id), func(t *testing.T) {
			prompt := "--help; do not execute this, use it as a prompt"
			_, args, err := harness.Build(harness.Request{Harness: id, Model: "default", Permission: harness.PermissionAsk, OpeningPrompt: prompt, Conversation: "c0ffee00-0000-4000-8000-5ec2e7c0ffee", Resume: "ea71e700-0000-4000-8000-5ec2e7ea71e7"})
			if err != nil {
				t.Fatal(err)
			}
			if args[len(args)-1] != prompt || slices.Contains(args, "default") {
				t.Fatalf("prompt or provider default was corrupted: %q", args)
			}
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "c0ffee") || strings.Contains(joined, "ea71e700") {
				t.Fatal("Luci home identity was treated as a native conversation ID")
			}
			if id == harness.Codex && (!strings.Contains(joined, "fork --last") || !strings.Contains(joined, "workspace-write") || !strings.Contains(joined, "on-request")) {
				t.Fatalf("Codex continuation/permission mismatch: %q", args)
			}
			if id == harness.OpenCode && (!slices.Contains(args, "--continue") || !slices.Contains(args, "--fork") || !slices.Contains(args, "--pure") || slices.Contains(args, "--auto") || args[1] != "--hostname" || args[2] != "127.0.0.1" || args[3] != "--port" || args[4] != "4096") {
				t.Fatalf("OpenCode continuation/permission mismatch: %q", args)
			}
		})
	}
}

func TestNativePlanModesDoNotEnableUnattendedExecution(t *testing.T) {
	for _, id := range []harness.ID{harness.Codex, harness.OpenCode} {
		_, args, err := harness.Build(harness.Request{Harness: id, Permission: harness.PermissionPlan})
		if err != nil {
			t.Fatal(err)
		}
		if id == harness.Codex && !slices.Contains(args, "read-only") || id == harness.OpenCode && !slices.Contains(args, "plan") {
			t.Fatalf("plan mode missing: %q", args)
		}
		if slices.Contains(args, "--auto") || slices.Contains(args, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatalf("plan allowed unattended execution: %q", args)
		}
	}
}
