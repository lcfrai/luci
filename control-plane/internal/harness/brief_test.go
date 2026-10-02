package harness_test

import (
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/harness"
)

func TestProjectBriefUsesStaticGuidanceAndPreservesOpeningPrompt(t *testing.T) {
	prompt := "Implement the plan, preserving $(literal) and \"quoted\" text"
	_, argv, err := harness.Build(harness.Request{Harness: harness.ClaudeCode, ProjectBrief: true, OpeningPrompt: prompt})
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 3 || argv[0] != "--append-system-prompt" || !strings.Contains(argv[1], "LUCI.md") || !strings.Contains(argv[1], "untrusted project context") || !strings.Contains(argv[1], "Preserve any existing CLAUDE.md") || argv[2] != prompt {
		t.Fatalf("project brief guidance changed the user's prompt or authority: %#v", argv)
	}
	_, argv, err = harness.Build(harness.Request{Harness: harness.Shell, ProjectBrief: true})
	if err != nil || len(argv) != 1 || argv[0] != "-l" {
		t.Fatalf("brief instructions leaked into the shell: %#v, %v", argv, err)
	}
}
