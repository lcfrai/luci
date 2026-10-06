import { useCockpit } from "../../store/store";

export interface OrchestratorAgent {
  harness: "codex" | "claude-code" | "opencode";
  model: string;
  agentConnectionId?: string;
}
const prefix = "luci.orchestrator-agent.v1:";
const harnesses = ["codex", "claude-code", "opencode"];

/** Only routing preferences are retained here, never credentials or runtime forms. */
export function readAgentPreference(scope: string, projectId: string): OrchestratorAgent {
  try {
    const raw = localStorage.getItem(prefix + scope);
    if (raw && raw.length < 4096) {
      const value = JSON.parse(raw) as OrchestratorAgent;
      if (harnesses.includes(value.harness) && typeof value.model === "string" && value.model.length <= 512 &&
          (value.agentConnectionId === undefined || typeof value.agentConnectionId === "string" && value.agentConnectionId.length <= 256)) return {
        harness: value.harness, model: value.model,
        ...(value.agentConnectionId === undefined ? {} : { agentConnectionId: value.agentConnectionId }),
      };
    }
  } catch { /* A blocked browser store does not stop an explicit conversation. */ }
  const state = useCockpit.getState(), defaults = { ...state.defaults, ...state.projectDefaults[projectId] };
  return {
    harness: harnesses.includes(defaults.harness) ? defaults.harness as OrchestratorAgent["harness"] : "codex",
    model: defaults.model === "default" ? "" : defaults.model,
    ...(defaults.agentConnectionId === undefined ? {} : { agentConnectionId: defaults.agentConnectionId }),
  };
}

export function writeAgentPreference(scope: string, value: OrchestratorAgent): void {
  try { localStorage.setItem(prefix + scope, JSON.stringify(value)); }
  catch { /* Keep the current selection in memory when storage is unavailable. */ }
}
