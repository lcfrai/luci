import type { AgentConnection } from "../../api/product";

export const gatewayAgents = [
  { model: "codex", endpoint: "responses", name: "Codex", endpointLabel: "Codex · Responses base URL", modelLabel: "Codex model" },
  { model: "claudeCode", endpoint: "anthropicMessages", name: "Claude Code", endpointLabel: "Claude Code · Messages base URL", modelLabel: "Claude Code model" },
  { model: "opencode", endpoint: "chatCompletions", name: "OpenCode", endpointLabel: "OpenCode · Chat Completions base URL", modelLabel: "OpenCode model" },
] as const;
export type GatewayAgentSupport = Record<typeof gatewayAgents[number]["model"], boolean>;
export const emptyGatewaySupport = (): GatewayAgentSupport => ({ codex: false, claudeCode: false, opencode: false });

export function gatewayBaseAddress(value: string) {
  const address = value.trim();
  if (address.length > 2048 || !/^https?:\/\//.test(address) || /[\s\\?#]/.test(address)) throw new Error("Enter an HTTP or HTTPS gateway server address without credentials, query parameters or a fragment.");
  let url: URL;
  try { url = new URL(address); } catch { throw new Error("Enter a valid gateway server address."); }
  if (!url.hostname || url.username || url.password) throw new Error("Enter the gateway server address without credentials. Put the key in Connection API key.");
  let path = url.pathname.replace(/\/+$/, "");
  if (/\/(responses|messages|models|chat\/completions)$/.test(path)) throw new Error("Enter the gateway server’s base address.");
  path = path.replace(/\/v1$/, "");
  return url.origin + path;
}

/** Prefill only a common canonical address; editing never writes the saved endpoints. */
export function savedGatewayAddress(endpoints: AgentConnection["endpoints"]) {
  const roots: string[] = [];
  for (const agent of gatewayAgents) {
    const endpoint = endpoints[agent.endpoint]?.replace(/\/+$/, "");
    if (!endpoint) continue;
    if (agent.model === "claudeCode") roots.push(endpoint);
    else if (endpoint.endsWith("/v1")) roots.push(endpoint.slice(0, -3));
    else return "";
  }
  if (!roots.length || roots.some(root => root !== roots[0])) return "";
  try { return gatewayBaseAddress(roots[0]); } catch { return ""; }
}
