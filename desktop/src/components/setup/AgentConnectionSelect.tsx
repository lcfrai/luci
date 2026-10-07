import type { AgentConnectionCatalogue } from "../../api/product";

export function AgentConnectionSelect({ catalogue, value, onChange, label = "Agent connection", className = "luci-field" }: {
  catalogue: AgentConnectionCatalogue | null; value?: string; onChange(value: string | undefined): void; label?: string; className?: string;
}) {
  const entries = catalogue?.connections ?? [];
  return <label className={className}>{label}<select value={value === undefined ? "@active" : value} onChange={event => onChange(event.target.value === "@active" ? undefined : event.target.value)}>
    <option value="@active">{catalogue?.activeConnectionId ? `Active · ${entries.find(item => item.id === catalogue.activeConnectionId)?.name ?? "saved connection"}` : "Use active connection / own agent settings"}</option>
    <option value="">Gateway off · own agent settings</option>
    {value && !entries.some(item => item.id === value) && <option value={value}>Unavailable saved connection</option>}
    {entries.map(item => <option key={item.id} value={item.id}>{item.name}{!item.enabled ? " · off" : !item.configured ? " · needs setup" : ""}</option>)}
  </select></label>;
}
