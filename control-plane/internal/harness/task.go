package harness

import (
	"errors"
	"fmt"
	"strconv"
)

var ErrTaskPermission = errors.New("task_permission_requires_interactive")

const TaskPermissionNotice = "Autonomous tasks require an explicitly saved Bypass permission mode. The pinned task CLIs cannot present interactive Ask or Plan approvals. Keep those modes in an interactive agent, or deliberately review and save a separate Bypass run. Task processes exit after their response; changes still require review and verification."

// ValidateTask checks native, noninteractive task capability without changing
// the saved route. In particular, Codex 0.160.0 exec forces Never approvals and
// rejects interactive requests; neither that mode nor automatic review is an
// equivalent of Ask. Do not silently turn an interactive permission into either.
func ValidateTask(id ID, permission PermissionMode) error {
	d, ok := Lookup(id)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownHarness, id)
	}
	if !d.Available() || id != Codex && id != ClaudeCode && id != OpenCode {
		return fmt.Errorf("%w: %q has no verified autonomous task mode", ErrUnavailable, id)
	}
	if permission != PermissionBypass {
		return fmt.Errorf("%w: %s", ErrTaskPermission, TaskPermissionNotice)
	}
	return nil
}

// BuildTask is only for a controller-owned swarm operation. Direct terminals
// continue to use Build. The pinned CLIs finish naturally after their response,
// while their existing conversation mounts preserve native history. Exit is an
// execution observation, never evidence that the task is correct.
//
// Flags verified in the pinned sandbox: Codex 0.160.0 exec, Claude 2.1.274
// --print, and OpenCode 1.18.34 run. Swarm retries are separate reviewed attempts;
// resuming a CLI here would silently replay an earlier operation.
func BuildTask(r Request) (command string, argv []string, err error) {
	if r.NativeDelegation != "" && (r.NativeDelegation != "codex-native-helpers-2-v1" || r.Harness != Codex || !r.DockerBoundary || !r.ReviewedContext) {
		return "", nil, fmt.Errorf("%w: native helpers require their captured local Codex task profile", ErrUnsupportedOption)
	}
	command, _, err = Build(r)
	if err != nil {
		return "", nil, err
	}
	if err = ValidateTask(r.Harness, r.Permission); err != nil {
		return "", nil, err
	}
	if r.Resume != "" {
		return "", nil, fmt.Errorf("%w: autonomous tasks require a fresh reviewed attempt", ErrUnsupportedOption)
	}
	if r.OpeningPrompt == "" {
		return "", nil, fmt.Errorf("%w: autonomous tasks require their captured task prompt", ErrUnsupportedOption)
	}
	switch r.Harness {
	case Codex:
		// All config overrides stay before the subcommand: the pinned parser
		// otherwise replaces earlier root -c values with the subcommand's -c.
		// These controller-owned tasks have no captured child-agent allowance.
		// Disable both pinned upstream agent paths; V2 overrides agents.enabled.
		argv = []string{"-c", `cli_auth_credentials_store="file"`,
			"-c", "agents.enabled=false", "-c", "features.multi_agent_v2=false"}
		if r.NativeDelegation != "" {
			command = "/usr/local/bin/node"
			argv = []string{"/usr/local/lib/luci/codex-native-task.cjs", "-c", `cli_auth_credentials_store="file"`,
				"-c", "agents.enabled=true", "-c", "agents.max_concurrent_threads_per_session=2", "-c", "features.multi_agent_v2=false"}
		}
		if r.ProjectBrief || r.ReviewedContext {
			argv = append(argv, "-c", "developer_instructions="+strconv.Quote(projectContextPrompt(r)))
		}
		argv = append(argv, "exec", "--color", "never", "--dangerously-bypass-approvals-and-sandbox")
		if r.Model != "" && r.Model != "default" {
			argv = append(argv, "--model", r.Model)
		}
		argv = append(argv, "--", r.OpeningPrompt)
	case ClaudeCode:
		argv = append([]string{"--print"}, claudeArgv(r)...)
	case OpenCode:
		prompt := r.OpeningPrompt
		if r.ReviewedContext {
			prompt = ReviewedContextPrompt + "\n\n" + prompt
		}
		argv = []string{"run", "--pure", "--agent", "build", "--auto"}
		if r.Model != "" && r.Model != "default" {
			argv = append(argv, "--model", r.Model)
		}
		argv = append(argv, "--", prompt)
	}
	return command, argv, nil
}
