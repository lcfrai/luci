import { useCallback, useEffect, useState } from "react";
import { errorText, productApi, type AgentConnection, type AgentConnectionCatalogue, type ProductClient } from "../api/product";

export const AGENT_CONNECTIONS_CHANGED = "luci:agent-connections-changed";

export function useAgentConnections(client: ProductClient = productApi, enabled = true) {
  const [catalogue, setCatalogue] = useState<AgentConnectionCatalogue | null>(null);
  const [error, setError] = useState("");
  const reload = useCallback(async () => await client.agentConnections(), [client]);
  useEffect(() => {
    if (!enabled) return;
    let stopped = false, generation = 0;
    const update = () => {
      const ticket = ++generation;
      void reload().then(value => { if (!stopped && ticket === generation) { setCatalogue(value); setError(""); } })
        .catch(cause => { if (!stopped && ticket === generation) setError(errorText(cause)); });
    };
    update();
    window.addEventListener(AGENT_CONNECTIONS_CHANGED, update);
    return () => { stopped = true; generation++; window.removeEventListener(AGENT_CONNECTIONS_CHANGED, update); };
  }, [reload, enabled]);
  return { catalogue, setCatalogue, error, setError, reload };
}

export function selectedAgentConnection(catalogue: AgentConnectionCatalogue | null, id?: string) {
  const selectedId = id === undefined ? catalogue?.activeConnectionId : id;
  return catalogue?.connections?.find(item => item.id === selectedId);
}
export function configuredAgentModel(connection: AgentConnection | undefined, harness: string) {
  return harness === "codex" ? connection?.models.codex : harness === "claude-code" ? connection?.models.claudeCode : harness === "opencode" ? connection?.models.opencode : undefined;
}
export function agentConnectionIssue(connection: AgentConnection | undefined, harness: string, id?: string) {
  if (!connection) return id ? "This saved connection is unavailable. Choose another connection in Agent connections." : "";
  if (!connection.enabled || !connection.configured) return connection.blockedReason || "This connection is off or incomplete. Update it in Agent connections.";
  if (connection.kind === "codex-subscription") return harness === "codex" ? "" : "Your Codex subscription supports Codex. Choose a connection for this agent.";
  const endpoint = harness === "codex" ? connection.endpoints.responses : harness === "claude-code" ? connection.endpoints.anthropicMessages : harness === "opencode" ? connection.endpoints.chatCompletions : undefined;
  if (!endpoint || !configuredAgentModel(connection, harness)) return "This connection needs an endpoint and model for the selected agent. Update Agent connections.";
  return "";
}
