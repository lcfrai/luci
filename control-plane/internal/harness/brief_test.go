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

func TestReviewedContextReplacesLiveBriefGuidanceForEveryCodingHarness(t *testing.T) {
	for _, provider := range []harness.ID{harness.ClaudeCode, harness.Codex, harness.OpenCode} {
		t.Run(string(provider), func(t *testing.T) {
			prompt := "Implement task A; preserve literal $(text) and quoted \"directions\"."
			_, argv, err := harness.Build(harness.Request{Harness: provider, ProjectBrief: true, ReviewedContext: true, OpeningPrompt: prompt})
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(argv, "\n")
			if strings.Contains(joined, harness.ProjectBriefPrompt) || !strings.Contains(joined, harness.ReviewedContextDirectory) || !strings.Contains(joined, "untrusted project context") || !strings.Contains(joined, "overwrite collaborative amendments") || !strings.Contains(joined, prompt) {
				t.Fatal("reviewed authority was missing, conflicted with live guidance or rewrote supplied direction", argv)
			}
		})
	}
}
