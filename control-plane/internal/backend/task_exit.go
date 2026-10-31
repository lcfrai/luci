package backend

import (
	"context"
	"encoding/hex"
	"time"
)

const TaskNormalZeroCapability = "ai.lcfr.task-exit-normal-zero"
const TaskExitProtocolLabel = "luci.task-exit-protocol"
const TaskExitContractLabel = "luci.task-exit-contract"
const TaskExitImageLabel = "luci.task-exit-image"
const TaskExitProtocol = "normal-zero-v1"
const TaskResultWrapper = "/usr/local/lib/luci/codex-task-result.cjs"

func StructuredTaskSpec(spec Spec) bool {
	if spec.Command != "/usr/local/bin/node" || len(spec.Args) < 4 || spec.Args[0] != TaskResultWrapper || len(spec.Args[1]) != 64 {
		return false
	}
	_, err := hex.DecodeString(spec.Args[1])
	return err == nil
}

// This is private original allocation identity, not a client assertion or a
// worker report. Timestamps are the acknowledged daemon values, not labels.
type TaskExitIdentity struct {
	SessionID      string
	SandboxID      string
	Ref            string
	Image          string
	ContractDigest string
	CreatedAt      time.Time
	StartedAt      time.Time
}

type TaskExitObservation struct {
	Ref            string    `json:"ref"`
	Image          string    `json:"image"`
	ContractDigest string    `json:"contractDigest"`
	CreatedAt      time.Time `json:"createdAt"`
	StartedAt      time.Time `json:"startedAt"`
	FinishedAt     time.Time `json:"finishedAt"`
	NormalZero     bool      `json:"normalZero"`
	OOMKilled      bool      `json:"oomKilled"`
	// Nonzero is the container's diagnostic status; it is NOT claimed as the
	// exact child code/signal. Only opted-in normal-zero has positive meaning.
	ContainerExitCode int `json:"containerExitCode"`
}

type TaskExitObserver interface {
	ObserveTaskExit(context.Context, TaskExitIdentity) (TaskExitObservation, error)
}
