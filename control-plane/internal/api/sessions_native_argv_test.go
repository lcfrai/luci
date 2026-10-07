package api

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/harness"
)

func TestCodexContextAndSelectedAuthenticationStayInOneConfigScope(t *testing.T) {
	for _, auth := range []string{"api-key", "chatgpt"} {
		for _, context := range []string{"project", "reviewed"} {
			for _, continuation := range []string{"fresh", "resume"} {
				t.Run(auth+"/"+context+"/"+continuation, func(t *testing.T) {
					const prompt = "--config model_provider=untrusted\nKeep this as prompt text."
					request := harness.Request{Harness: harness.Codex, Conversation: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Model: "configured-model", Permission: harness.PermissionAsk, DockerBoundary: true, OpeningPrompt: prompt, ProjectBrief: true, ReviewedContext: context == "reviewed"}
					if continuation == "resume" {
						request.Resume = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
					}
					command, args, err := harness.Build(request)
					if err != nil || command != "codex" {
						t.Fatal("Codex launch could not be built", err)
					}
					env := map[string]string{"OPENAI_API_KEY": "fixture-secret-kept-in-env", "LUCI_CODEX_BASE_URL": "http://host.docker.internal:4321/v1"}
					args = codexSubscriptionArgs(args, env, auth)
					args = nativeArgs("codex", args, env, false)
					if strings.Contains(strings.Join(args, " "), "fixture-secret-kept-in-env") {
						t.Fatal("model credential entered argv")
					}
					if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != prompt {
						t.Fatal("opening prompt lost its argument boundary")
					}
					fork := slices.Index(args[:len(args)-2], "fork")
					if (fork >= 0) != (continuation == "resume") || (fork >= 0 && args[fork+1] != "--last") {
						t.Fatal("native conversation selection changed")
					}
					config := map[string]string{}
					for index := 0; index < len(args)-2; index++ {
						if args[index] != "-c" {
							continue
						}
						// Pinned Codex uses Clap's global Append argument: a
						// child -c vector replaces the root vector before the
						// CLI can merge it. Test the complete composed launch,
						// including API/subscription settings and context.
						if fork >= 0 && index > fork {
							t.Fatal("fork-level context config would discard root authentication/provider settings")
						}
						index++
						key, value, ok := strings.Cut(args[index], "=")
						if !ok || config[key] != "" {
							t.Fatal("malformed or duplicate launch configuration")
						}
						config[key] = value
					}
					if config["cli_auth_credentials_store"] != `"file"` {
						t.Fatal("native private credential storage was lost")
					}
					instructions, err := strconv.Unquote(config["developer_instructions"])
					want := harness.ProjectBriefPrompt
					if context == "reviewed" {
						want = harness.ReviewedContextPrompt
					}
					if err != nil || instructions != want {
						t.Fatal("captured context guidance changed or was lost")
					}
					if auth == "api-key" {
						provider := config["model_providers.luci-openai"]
						if config["model_provider"] != `"luci-openai"` || !strings.Contains(provider, `env_key="OPENAI_API_KEY"`) || !strings.Contains(provider, `base_url="http://host.docker.internal:4321/v1"`) || env["OPENAI_API_KEY"] == "" || config["forced_login_method"] != "" {
							t.Fatal("selected API route or environment credential was lost")
						}
					} else if config["model_provider"] != `"openai"` || config["forced_login_method"] != `"chatgpt"` || env["OPENAI_API_KEY"] != "" || config["model_providers.luci-openai"] != "" {
						t.Fatal("subscription route inherited API authentication")
					}
					for option, value := range map[string]string{"--model": "configured-model", "--sandbox": "danger-full-access", "--ask-for-approval": "on-request"} {
						index := slices.Index(args, option)
						if index < 0 || args[index+1] != value {
							t.Fatalf("%s changed while preserving config scope", option)
						}
					}
					if !slices.Contains(args, "--no-daemon") || !slices.Contains(args, "--no-alt-screen") || slices.Contains(args, "--dangerously-bypass-approvals-and-sandbox") {
						t.Fatal("interactive isolation or approval mode changed")
					}
				})
			}
		}
	}
}
