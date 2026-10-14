import { useCallback, useEffect, useRef, useState } from "react";
import { errorText, productApi, type AgentConnection, type AgentConnectionCatalogue, type ProductClient } from "../api/product";

export const AGENT_CONNECTIONS_CHANGED = "luci:agent-connections-changed";
// Own-provider saves use a separate signal: the saved OpenCode model is read
// from /setup/agents, not from the saved connection catalogue above.
export const OWN_AGENT_PROVIDER_CHANGED = "luci:own-agent-provider-changed";

export function useAgentConnections(client: ProductClient = productApi, enabled = true) {
  const [catalogue, storeCatalogue] = useState<AgentConnectionCatalogue | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(enabled);
  const active = useRef(false), request = useRef(0);
  // A confirmed mutation supersedes every earlier read, including explicit
  // refreshes. Otherwise a delayed catalogue can misstate the billing account
  // selected for the next session.
  const setCatalogue = useCallback((value: AgentConnectionCatalogue) => {
    ++request.current;
    if (active.current) { storeCatalogue(value); setLoading(false); }
  }, []);
  const reload = useCallback(async () => {
    const ticket = ++request.current;
    if (active.current) setLoading(true);
    try {
      const value = await client.agentConnections();
      if (active.current && ticket === request.current) { storeCatalogue(value); setError(""); }
      return value;
    } catch (cause) {
      if (active.current && ticket === request.current) setError(errorText(cause));
      throw cause;
    } finally {
      if (active.current && ticket === request.current) setLoading(false);
    }
  }, [client]);
  useEffect(() => {
    active.current = enabled;
    if (!enabled) { setLoading(false); return; }
    const update = () => { void reload().catch(() => {}); };
    update();
    window.addEventListener(AGENT_CONNECTIONS_CHANGED, update);
    return () => { active.current = false; ++request.current; window.removeEventListener(AGENT_CONNECTIONS_CHANGED, update); };
  }, [reload, enabled]);
  return { catalogue, setCatalogue, error, setError, reload, loading };
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
