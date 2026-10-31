package backend

import "context"

const CodexTaskResultCapability = "ai.lcfr.codex-task-result"
const CodexTaskResultSchemaLabel = "ai.lcfr.codex-task-result.schema-sha256"
const CodexTaskResultSchemaDigest = "202ad6cfee380ee028962f42c93a23c730f47b7d8396f8ad58d03508b10c1e8c"

// This observes an installed image's advertised capability, not proof of file
// contents or successful execution. No pull/build/probe/fallback is permitted.
type CodexTaskResultImageObserver interface {
	ObserveCodexTaskResultImage(context.Context, string) (string, error)
}
