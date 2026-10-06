import { useEffect, useRef, useState } from "react";
import { errorText, type AgentConnection, type AgentConnectionKind, type AgentConnectionMutation, type ProductClient } from "../../api/product";
import { AGENT_CONNECTIONS_CHANGED, useAgentConnections } from "../../hooks/useAgentConnections";
import { Button } from "../fluid/components/button";
import { emptyGatewaySupport, gatewayAgents, gatewayBaseAddress, savedGatewayAddress } from "./gatewayPreset";

const emptyDraft = () => ({ id: "", name: "", kind: "litellm" as AgentConnectionKind, enabled: true, endpoints: {} as AgentConnection["endpoints"], models: {} as AgentConnection["models"] });
export function AgentConnectionManager({ client }: { client: ProductClient }) {
  const { catalogue, setCatalogue, error, setError, reload } = useAgentConnections(client);
  const [draft, setDraft] = useState(emptyDraft), [apiKey, setApiKey] = useState(""), [clearKey, setClearKey] = useState(false);
  const [busy, setBusy] = useState(false), [notice, setNotice] = useState("");
  const [gatewayAddress, setGatewayAddress] = useState("");
  const [gatewaySupport, setGatewaySupport] = useState(emptyGatewaySupport);
  const [presetError, setPresetError] = useState("");
  const working = useRef(false);
  const alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const entries = catalogue?.connections ?? [];
  const saved = entries.find(item => item.id === draft.id);
  const active = entries.find(item => item.id === catalogue?.activeConnectionId);
  const gatewayOn = active?.kind === "litellm" || active?.kind === "compatible-api";
  const subscription = draft.kind === "codex-subscription";
  const chooseDraft = (id: string) => {
    const value = entries.find(item => item.id === id);
    setDraft(value ? { id: value.id, name: value.name, kind: value.kind, enabled: value.enabled, endpoints: { ...value.endpoints }, models: { ...value.models } } : emptyDraft());
    setApiKey(""); setClearKey(false); setNotice("");
    setGatewayAddress(value ? savedGatewayAddress(value.endpoints) : "");
    setGatewaySupport(value ? Object.fromEntries(gatewayAgents.map(agent => [agent.model, !!value.endpoints[agent.endpoint] || !!value.models[agent.model]])) as typeof gatewaySupport : emptyGatewaySupport());
    setPresetError("");
  };
  function applyGatewayAddress() {
    try {
      const base = gatewayBaseAddress(gatewayAddress);
      const selected = gatewayAgents.filter(agent => gatewaySupport[agent.model]);
      if (!selected.length) throw new Error("Choose at least one agent your gateway supports.");
      if (selected.some(agent => !draft.models[agent.model]?.trim() || /\s/.test(draft.models[agent.model]!.trim()))) throw new Error("Enter the exact available model alias for each selected agent, without spaces.");
      const endpoints: AgentConnection["endpoints"] = {}, models: AgentConnection["models"] = {};
      for (const agent of selected) { endpoints[agent.endpoint] = agent.model === "claudeCode" ? base : `${base}/v1`; models[agent.model] = draft.models[agent.model]!.trim(); }
      setDraft({ ...draft, endpoints, models }); setGatewayAddress(base); setPresetError("");
      setNotice("Gateway addresses applied to this draft. Protocol and model access have not been verified.");
    } catch (cause) { setPresetError(errorText(cause)); }
  }
  async function mutate(input: AgentConnectionMutation, success: string) {
    if (working.current) return;
    working.current = true; setBusy(true); setError(""); setNotice(""); setApiKey("");
    try {
      const value = await client.updateAgentConnection(input);
      window.dispatchEvent(new Event(AGENT_CONNECTIONS_CHANGED));
      if (!alive.current) return;
      setCatalogue(value); setClearKey(false); setNotice(success);
      if (input.action === "save" && !draft.id) {
        const added = value.connections.find(item => !entries.some(previous => previous.id === item.id));
        if (added) setDraft({ id: added.id, name: added.name, kind: added.kind, enabled: added.enabled, endpoints: { ...added.endpoints }, models: { ...added.models } });
      }
      if (input.action === "delete") chooseDraft("");
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  const select = (id: string) => catalogue && mutate({ action: "select", expectedRevision: catalogue.revision, id }, id ? "Active connection updated for new sessions." : "Gateway is off. New sessions use your own agent settings.");
  return <section className="setup-plan agent-connection-manager" aria-label="Saved agent connections" aria-busy={busy}>
    <h2>Agent connections</h2>
    <p>Use a LiteLLM gateway, your own compatible API, or your Codex subscription. Model usage follows the account connected here.</p>
    {!catalogue && !error && <p role="status">Loading saved connections…</p>}
    <fieldset disabled={busy || !catalogue} className="definition-fields">
      <label className="luci-field">Active connection<select value={catalogue?.activeConnectionId ?? ""} onChange={event => void select(event.target.value)}>
        <option value="">Gateway off · own agent settings</option>
        {entries.map(item => <option key={item.id} value={item.id} disabled={!item.enabled || !item.configured}>{item.name}{!item.enabled ? " · off" : !item.configured ? " · needs setup" : ""}</option>)}
      </select></label>
      <label className="data-check"><input type="checkbox" role="switch" checked={!!gatewayOn} onChange={event => {
        if (!event.target.checked) { void select(""); return; }
        const gateway = entries.find(item => (item.kind === "litellm" || item.kind === "compatible-api") && item.enabled && item.configured);
        if (gateway) void select(gateway.id); else setError("Save an enabled gateway connection below, then turn it on.");
      }} />Use gateway</label>
      <p className="journey-muted">{active ? `New sessions use ${active.name}.` : "New sessions use the direct accounts configured below."} Existing sessions keep the connection they started with.</p>
    </fieldset>
    {error && <p className="journey-error" role="alert">{error}</p>}
    {notice && <p className="journey-success" role="status">{notice}</p>}
    <Button type="button" variant="ghost" disabled={busy} onClick={() => void reload().then(value => { setCatalogue(value); setError(""); }).catch(cause => setError(errorText(cause)))}>Refresh saved connections</Button>
    <details><summary>Add or edit a connection</summary><form onSubmit={event => {
      event.preventDefault(); if (!catalogue) return;
      const endpoints = subscription ? {} : Object.fromEntries(Object.entries(draft.endpoints).filter(([, value]) => value?.trim()).map(([key, value]) => [key, value!.trim()]));
      const models = Object.fromEntries(Object.entries(draft.models).filter(([key, value]) => value?.trim() && (!subscription || key === "codex")).map(([key, value]) => [key, value!.trim()]));
      void mutate({ action: "save", expectedRevision: catalogue.revision, ...(draft.id ? { id: draft.id } : {}), name: draft.name.trim(), kind: draft.kind, enabled: draft.enabled, endpoints, models, ...(subscription ? { apiKey: "" } : apiKey || clearKey ? { apiKey: clearKey ? "" : apiKey } : {}) }, "Connection saved. Choose it as the active connection when you are ready. Saving does not verify model access.");
    }}>
      <fieldset disabled={busy || !catalogue} className="definition-fields">
        <label className="luci-field">Edit saved connection<select value={draft.id} onChange={event => chooseDraft(event.target.value)}><option value="">Add a connection</option>{entries.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
        <div className="setup-field-grid">
          <label className="luci-field">Connection name<input required value={draft.name} onChange={event => setDraft({ ...draft, name: event.target.value })} placeholder="A name you recognise" /></label>
          <label className="luci-field">Connection type<select value={draft.kind} onChange={event => { setDraft({ ...draft, kind: event.target.value as AgentConnectionKind }); setApiKey(""); setClearKey(false); setPresetError(""); }}><option value="litellm">LiteLLM gateway</option><option value="compatible-api">Compatible API</option><option value="codex-subscription">My Codex subscription</option><option value="direct-api">Direct provider API</option></select></label>
        </div>
        <label className="data-check"><input type="checkbox" checked={draft.enabled} onChange={event => setDraft({ ...draft, enabled: event.target.checked })} />Enable this connection</label>
        {subscription ? <><p className="journey-muted">Uses your saved official Codex sign-in. Complete the Codex account sign-in below once; no API key or endpoint is needed here.</p>{saved?.keyPresent && <p className="journey-muted">Saving this type removes this connection’s saved API key.</p>}</> : <>
          {draft.kind === "litellm" ? <>
            <p className="journey-muted">Use the gateway address and key supplied for your connection, including an lcfr.ai gateway.</p>
            <label className="luci-field">Gateway server address<input inputMode="url" autoComplete="off" value={gatewayAddress} aria-invalid={!!presetError} onChange={event => { setGatewayAddress(event.target.value); setPresetError(""); }} placeholder="Your gateway server address" /></label>
            <fieldset><legend>Agents to use</legend><p className="journey-muted">Select only agents and exact model aliases available on your gateway. Claude Code needs a Claude-backed model with Messages support.</p>
              {gatewayAgents.map(agent => <div key={agent.model}><label className="data-check"><input type="checkbox" checked={gatewaySupport[agent.model]} onChange={event => {
                setGatewaySupport({ ...gatewaySupport, [agent.model]: event.target.checked }); setPresetError("");
                if (!event.target.checked) { const endpoints = { ...draft.endpoints }, models = { ...draft.models }; delete endpoints[agent.endpoint]; delete models[agent.model]; setDraft({ ...draft, endpoints, models }); }
              }} />Use {agent.name}</label>{gatewaySupport[agent.model] && <label className="luci-field">{agent.modelLabel}<input autoComplete="off" value={draft.models[agent.model] ?? ""} onChange={event => { setDraft({ ...draft, models: { ...draft.models, [agent.model]: event.target.value } }); setPresetError(""); }} placeholder="Exact model alias supplied by your gateway" /></label>}</div>)}
            </fieldset>
            <Button type="button" variant="secondary" onClick={applyGatewayAddress}>Apply gateway address</Button>
            <p className="journey-muted">Apply uses the server address for Claude Code and adds /v1 for Codex and OpenCode. It replaces the protocol addresses only when you choose Apply.</p>
            {saved && !savedGatewayAddress(saved.endpoints) && Object.values(saved.endpoints).some(Boolean) && <p className="journey-muted">This connection has custom protocol addresses. Advanced keeps them until you edit them or apply a gateway address.</p>}
            {presetError && <p className="journey-error" role="alert">{presetError}</p>}
          </> : <p className="journey-muted">Enter the base URL and exact model ID for each supported protocol in Advanced. Leave unsupported protocols empty.</p>}
          <details open={draft.kind !== "litellm"}><summary>Advanced · protocol addresses</summary><div className="setup-field-grid">{gatewayAgents.map(agent => <div key={agent.endpoint}><label className="luci-field">{agent.endpointLabel}<input type="url" value={draft.endpoints[agent.endpoint] ?? ""} onChange={event => setDraft({ ...draft, endpoints: { ...draft.endpoints, [agent.endpoint]: event.target.value } })} /></label>{draft.kind !== "litellm" && <label className="luci-field">{agent.modelLabel}<input autoComplete="off" value={draft.models[agent.model] ?? ""} onChange={event => setDraft({ ...draft, models: { ...draft.models, [agent.model]: event.target.value } })} /></label>}</div>)}</div></details>
          <label className="luci-field">Connection API key<input aria-label="Connection API key" type="password" autoComplete="off" value={apiKey} disabled={clearKey} onChange={event => setApiKey(event.target.value)} /><small>{saved?.keyPresent ? "A key is saved. Leave this blank to keep it, or enter a replacement." : "The key stays in this workspace’s private settings."}</small></label>
          {saved?.keyPresent && <label className="data-check"><input type="checkbox" checked={clearKey} onChange={event => { setClearKey(event.target.checked); setApiKey(""); }} />Clear the saved key when saving</label>}
        </>}
        {subscription && <label className="luci-field">Codex model · optional<input value={draft.models.codex ?? ""} onChange={event => setDraft({ ...draft, models: { codex: event.target.value } })} /></label>}
        {saved?.blockedReason && <p role="status">{saved.blockedReason}</p>}
        <div className="setup-actions"><Button type="submit" disabled={!draft.name.trim()} loading={busy}>Save connection</Button>{saved && <Button type="button" variant="secondary" onClick={() => void mutate({ action: "delete", expectedRevision: catalogue!.revision, id: saved.id }, "Connection removed from new session choices.")}>Delete connection</Button>}</div>
      </fieldset>
    </form></details>
  </section>;
}
