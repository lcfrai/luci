package registry

import "time"

const TaskTerminalResultMethod = "docker-task-normal-zero-v1"

// Private controller fact from the exact opted-in daemon allocation. It is not
// a worker report, child-code/signal claim, source check or permission receipt.
type TaskTerminalResult struct {
	Method            string    `json:"method"`
	SessionCreatedAt  time.Time `json:"sessionCreatedAt"`
	Ref               string    `json:"ref"`
	Image             string    `json:"image"`
	ContractDigest    string    `json:"contractDigest"`
	CreatedAt         time.Time `json:"createdAt"`
	StartedAt         time.Time `json:"startedAt"`
	FinishedAt        time.Time `json:"finishedAt"`
	NormalZero        bool      `json:"normalZero"`
	OOMKilled         bool      `json:"oomKilled"`
	ContainerExitCode int       `json:"containerExitCode"`
}

func (s Session) HasTaskContainerResult() bool {
	r := s.TaskTerminalResult
	return s.Command == "/usr/local/bin/node" && len(s.Args) >= 2 && s.Args[0] == "/usr/local/lib/luci/codex-task-result.cjs" &&
		r != nil && r.Method == TaskTerminalResultMethod && !s.CreatedAt.IsZero() && r.SessionCreatedAt.Equal(s.CreatedAt) &&
		r.Ref != "" && r.Ref == s.SandboxRef && r.Image != "" && r.Image == s.TaskTerminalImage && r.ContractDigest == s.Args[1] &&
		!s.TaskTerminalCreatedAt.IsZero() && r.CreatedAt.Equal(s.TaskTerminalCreatedAt) &&
		!s.TaskTerminalStartedAt.IsZero() && r.StartedAt.Equal(s.TaskTerminalStartedAt) && !r.FinishedAt.IsZero() &&
		r.ContainerExitCode >= 0 && r.ContainerExitCode <= 255 &&
		!r.StartedAt.Before(r.CreatedAt) && !r.FinishedAt.Before(r.StartedAt) &&
		(!r.NormalZero || !r.OOMKilled && r.ContainerExitCode == 0)
}
