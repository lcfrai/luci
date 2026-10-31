package product

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Inspect only public capability labels. This never reads image environment,
// starts a container, downloads an image or claims that task execution passed.
const runtimeReadinessFormat = `{"id":{{json .Id}},"os":{{json .Os}},"architecture":{{json .Architecture}},"result":{{json (index .Config.Labels "ai.lcfr.codex-task-result")}},"schema":{{json (index .Config.Labels "ai.lcfr.codex-task-result.schema-sha256")}},"normalZero":{{json (index .Config.Labels "ai.lcfr.task-exit-normal-zero")}},"harnesses":{{json (index .Config.Labels "ai.lcfr.harness-task-result")}}}`

type runtimeReadinessIdentity struct {
	ID           string `json:"id"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Result       string `json:"result"`
	Schema       string `json:"schema"`
	NormalZero   string `json:"normalZero"`
	Harnesses    string `json:"harnesses"`
}

func runtimeReadinessCheck(output string) Check {
	check := Check{ID: "sandbox-image", Label: "Agent runtime", Status: "missing", Message: "The installed runtime could not be identified.", Action: "Check the Docker engine and run Check again."}
	var identity runtimeReadinessIdentity
	if len(output) > 4096 || json.Unmarshal([]byte(output), &identity) != nil {
		return check
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(identity.ID, "sha256:"))
	if err != nil || len(digest) != 32 || len(identity.ID) != 71 || !strings.HasPrefix(identity.ID, "sha256:") {
		return check
	}
	if identity.OS != "linux" || identity.Architecture != "amd64" || identity.Result != "v1" || identity.Schema != backend.CodexTaskResultSchemaDigest || identity.NormalZero != "v1" || identity.Harnesses != backend.HarnessTaskResultVersion {
		check.Message = "An agent image is installed, but it does not support the current Luci task workflows."
		check.Action = "Build the current runtime from the source included with Luci. Existing sessions are not restarted."
		check.PreparationMode = "advanced-build"
		return check
	}
	check.Status = "ready"
	check.Message = "Installed runtime advertises the current Codex, Claude Code and OpenCode task contracts."
	check.Action = ""
	return check
}
