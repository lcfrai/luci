import { useEffect, useId, useRef, useState } from "react";
import { errorText, type AgentConnection, type ProductClient } from "../../api/product";
import type { LocalModelSave, LocalModelSnapshot } from "../../api/localModel";
import { selectedTeamId, teamScopeGeneration, TEAM_WORKSPACE_CHANGED } from "../../api/teamScope";
import { AGENT_CONNECTIONS_CHANGED } from "../../hooks/useAgentConnections";
import { useAuth } from "../auth/AuthProvider";
import { Button } from "../fluid/components/button";

type Draft = Omit<LocalModelSave, "expectedRevision">;
const emptyDraft = (): Draft => ({ enabled: true, startWithLuci: false, executablePath: "", modelStore: "", port: 11435, model: "" });
const draftFrom = (snapshot: LocalModelSnapshot): Draft => snapshot.config ? {
  enabled: snapshot.config.enabled,
  startWithLuci: snapshot.config.startWithLuci,
  executablePath: snapshot.config.executablePath ?? "",
  modelStore: snapshot.config.modelStore ?? "",
  port: snapshot.config.port,
  model: snapshot.config.model,
} : emptyDraft();
const runningStates = new Set(["starting", "available", "loaded", "stopping"]);
const stateLabels: Record<LocalModelSnapshot["status"]["state"], string> = {
  disabled: "Not enabled", stopped: "Stopped", starting: "Starting", available: "Server available",
  loaded: "Model loaded", stopping: "Stopping", blocked: "Needs attention", unknown: "Status unconfirmed",
};
const failureMessages: Record<string, string> = {
  port_unavailable: "This port is unavailable. Check other local services or save a different port.",
  model_unavailable: "The saved model is not installed in this model folder. Choose an installed model.",
  model_profile_mismatch: "Choose an installed model configured for this service’s 16k context limit.",
  loaded_model_profile_mismatch: "The loaded model does not match the saved model and its required profile.",
  host_capacity_unavailable: "Luci cannot confirm enough free memory and CPU capacity for this model. Free some capacity, then refresh.",
  gpu_capacity_unavailable: "Luci cannot confirm enough free GPU memory for this model.",
  gpu_observation_unavailable: "Luci could not check GPU capacity. The local model cannot run until that check succeeds.",
  gpu_binary_untrusted: "Luci could not verify the installed NVIDIA tool’s signature. Model startup is paused.",
  gpu_metadata_unavailable: "Luci could not verify the installed NVIDIA tool’s version information. Model startup is paused.",
  gpu_process_unavailable: "Luci could not start or observe its GPU capacity check. No model was started.",
  gpu_process_exited: "The GPU capacity check exited before it produced a usable observation. No model was started.",
  gpu_pipe_unavailable: "Luci could not read the GPU capacity check’s output. No model was started.",
  gpu_sample_invalid: "The GPU capacity check returned invalid or changing device information. No model was started.",
  gpu_sample_missing: "Luci did not confirm GPU capacity within five seconds. No model was started.",
  gpu_sample_stale: "The GPU capacity check did not produce a current observation. No model was started.",
  gpu_capacity_low: "The GPU needs at least 6 GiB of free memory to start this model. Close other GPU work before trying again.",
  binary_unavailable: "Luci cannot find the saved Ollama application. Check its location.",
  binary_untrusted: "Luci could not verify the saved Ollama application’s identity and publisher.",
  model_store_unavailable: "Luci cannot access the saved model folder. Check its location and permissions.",
  listener_ownership_unavailable: "Luci cannot confirm that its model process owns the configured port.",
  startup_exit: "The model server exited before it became available. Check the saved application and model folder.",
  startup_timeout: "The model server did not become available within its startup time limit.",
  model_memory_unavailable: "The model could not allocate the memory it needs. Free some memory before trying again.",
  observation_unavailable: "Luci could not confirm the model’s current status.",
  status_unavailable: "The model’s current status is unavailable. Refresh before starting new work.",
  cleanup_unknown: "The previous model process has not been confirmed closed. A replacement will not be started.",
  authority_changed: "Your permission to manage this model changed. Refresh to see the actions available to you.",
  previous_work_unsettled: "End or reconcile the previous managed-model session in Luci before starting a replacement.",
  configuration_unavailable: "Luci could not read the saved model configuration. Refresh before changing it.",
};

function usable(snapshot: LocalModelSnapshot | null): snapshot is LocalModelSnapshot {
  if (!snapshot?.available || !snapshot.config?.enabled || !["available", "loaded"].includes(snapshot.status.state)) return false;
  const age = Date.now() - Date.parse(snapshot.status.observedAt);
  return Number.isFinite(age) && age >= -1000 && age <= 5000 && !!snapshot.status.endpoint && snapshot.status.model === snapshot.config.model;
}

/** One host-owned model service. Viewing or saving this panel never starts it. */
export function ManagedLocalModel({ client, cloud }: { client: ProductClient; cloud: boolean }) {
  const helpId = useId();
  const actor = useAuth()?.profile.user.id ?? "";
  const [scope, setScope] = useState(teamScopeGeneration);
  const [snapshot, setSnapshot] = useState<LocalModelSnapshot | null>(null);
  const [draft, setDraft] = useState(emptyDraft);
  const [draftRevision, setDraftRevision] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pollCycle, setPollCycle] = useState(0);
  const [observationError, setObservationError] = useState("");
  const observationFailed = !!observationError;
  const lifetime = useRef(0), readNumber = useRef(0), working = useRef(false), initialized = useRef(false);
  const edits = useRef(0);
  const identity = useRef({ client, actor, scope, cloud });
  identity.current = { client, actor, scope, cloud };
  const inPersonalLocalWorkspace = !cloud && !selectedTeamId();
  const inCurrentWorkspace = () => !cloud && !selectedTeamId() && scope === teamScopeGeneration()
    && identity.current.client === client && identity.current.actor === actor && identity.current.scope === scope && !identity.current.cloud;

  useEffect(() => {
    const changed = () => setScope(teamScopeGeneration());
    window.addEventListener(TEAM_WORKSPACE_CHANGED, changed);
    return () => window.removeEventListener(TEAM_WORKSPACE_CHANGED, changed);
  }, []);

  async function read(signal?: AbortSignal, replaceDraft = false) {
    if (!inCurrentWorkspace()) return;
    const generation = lifetime.current, request = ++readNumber.current, editSequence = edits.current;
    const current = () => generation === lifetime.current && request === readNumber.current && inCurrentWorkspace();
    setLoading(true);
    try {
      const result = await client.localModel(signal);
      if (!current()) return;
      setSnapshot(result); setObservationError("");
      if ((!initialized.current || replaceDraft) && editSequence === edits.current) {
        initialized.current = true;
        setDraft(draftFrom(result)); setDraftRevision(result.revision); setDirty(false);
      }
    } catch (cause) {
      if (!signal?.aborted && current()) setObservationError(errorText(cause));
    } finally {
      if (current()) { setLoading(false); setPollCycle(value => value + 1); }
    }
  }

  useEffect(() => {
    lifetime.current++; readNumber.current++; initialized.current = false; working.current = false; edits.current++;
    setSnapshot(null); setDraft(emptyDraft()); setDraftRevision(0); setDirty(false); setBusy(""); setError(""); setNotice(""); setObservationError("");
    const controller = new AbortController();
    if (inPersonalLocalWorkspace) void read(controller.signal);
    return () => { lifetime.current++; readNumber.current++; controller.abort(); };
  }, [client, actor, scope, inPersonalLocalWorkspace]);

  useEffect(() => {
    if (!snapshot?.available || !runningStates.has(snapshot.status.state) || busy || !inPersonalLocalWorkspace) return;
    const controller = new AbortController();
    const timer = setTimeout(() => { if (!working.current) void read(controller.signal); }, 2000);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [snapshot, pollCycle, busy, client, actor, scope, inPersonalLocalWorkspace]);

  async function change(action: "save" | "start" | "stop") {
    if (working.current || !snapshot?.canManage || !inCurrentWorkspace()) return;
    const generation = lifetime.current;
    const current = () => generation === lifetime.current && inCurrentWorkspace();
    working.current = true; readNumber.current++; setBusy(action); setError(""); setNotice("");
    try {
      const result = action === "save"
        ? await client.saveLocalModel({ ...draft, executablePath: draft.executablePath.trim(), modelStore: draft.modelStore.trim(), model: draft.model.trim(), expectedRevision: draftRevision })
        : action === "start" ? await client.startLocalModel(snapshot.revision)
          : await client.stopLocalModel(snapshot.revision, snapshot.status.generation);
      if (!current()) return;
      setSnapshot(result); setObservationError("");
      if (action === "save") {
        setDraft(draftFrom(result)); setDraftRevision(result.revision); setDirty(false);
        setNotice("Configuration saved. The model has not been started or downloaded.");
      }
    } catch (cause) { if (current()) setError(errorText(cause)); }
    finally { if (current()) { working.current = false; setBusy(""); setLoading(false); } }
  }

  async function useModel() {
    if (working.current || !usable(snapshot) || !inCurrentWorkspace()) return;
    const generation = lifetime.current;
    working.current = true; readNumber.current++; setBusy("use"); setError(""); setNotice("");
    const current = () => generation === lifetime.current && inCurrentWorkspace();
    try {
      const observed = await client.localModel();
      if (!current()) return;
      setSnapshot(observed); setObservationError("");
      if (!usable(observed) || observed.revision !== snapshot.revision) throw new Error("The local model changed. Review its current status before selecting it.");
      let catalogue = await client.agentConnections();
      if (!current()) return;
      // A matching display name never grants ownership of a BYO connection.
      let connection: AgentConnection | undefined = catalogue.connections.find(entry => entry.managedLocalModel?.revision === observed.revision && entry.kind === "compatible-api" && entry.openCodeService === "local-ollama" && entry.openCodeThinking === "off" && entry.endpoints.chatCompletions === observed.status.endpoint && entry.models.opencode === observed.status.model);
      if (!connection?.enabled || !connection.configured) {
        const owned = connection ?? catalogue.connections.find(entry => entry.managedLocalModel);
        const previousIDs = new Set(catalogue.connections.map(entry => entry.id));
        catalogue = await client.updateAgentConnection({
          action: "save", expectedRevision: catalogue.revision, ...(owned ? { id: owned.id } : {}),
          name: owned?.name ?? `Local · ${observed.config!.model}`, kind: "compatible-api", enabled: true,
          endpoints: { chatCompletions: observed.status.endpoint }, models: { opencode: observed.config!.model },
          openCodeService: "local-ollama", openCodeThinking: "off", managedLocalModel: { revision: observed.revision }, apiKey: "",
        });
        if (!current()) return;
        window.dispatchEvent(new Event(AGENT_CONNECTIONS_CHANGED));
        connection = catalogue.connections.find(entry => (owned ? entry.id === owned.id : !previousIDs.has(entry.id)) && entry.managedLocalModel?.revision === observed.revision);
        if (!connection?.enabled || !connection.configured) throw new Error("The saved local connection is not ready to select. Refresh saved connections to inspect it.");
      }
      if (catalogue.activeConnectionId !== connection.id) {
        await client.updateAgentConnection({ action: "select", expectedRevision: catalogue.revision, id: connection.id });
        if (!current()) return;
        window.dispatchEvent(new Event(AGENT_CONNECTIONS_CHANGED));
      }
      setNotice("Local model selected for new OpenCode sessions. Existing sessions keep their current connection.");
    } catch (cause) { if (current()) setError(`${errorText(cause)} Refresh saved connections before trying again.`); }
    finally { if (current()) { working.current = false; setBusy(""); setLoading(false); } }
  }

  if (!inPersonalLocalWorkspace || (snapshot && !snapshot.available)) return null;
  const state = snapshot?.status.state;
  const active = !!state && runningStates.has(state);
  const configurationLocked = active || state === "unknown";
  const staleDraft = snapshot !== null && snapshot.revision !== draftRevision;
  const canStart = !!snapshot?.canManage && !!snapshot.config?.enabled && !configurationLocked && !dirty && !staleDraft && !error && !observationError;
  const canStop = !!snapshot?.canManage && !!snapshot.status.generation && /^(?:[1-9]\d{0,19})$/.test(snapshot.status.generation) && state !== "stopped" && state !== "disabled" && state !== "unknown";
  const failure = snapshot?.status.failure;
  const setField = <K extends keyof Draft>(key: K, value: Draft[K]) => { edits.current++; setDraft(previous => ({ ...previous, [key]: value })); setDirty(true); setNotice(""); };
  return <section className="setup-plan managed-local-model" aria-label="Local model" aria-busy={!!busy}>
    <div className="local-model-heading"><div><h2>Local model</h2><p>Run an installed Ollama model on this PC. Luci manages its process and resource limits.</p></div><span role="status" className="local-model-state">{observationFailed ? "Status unavailable" : snapshot ? stateLabels[snapshot.status.state] ?? "Status unconfirmed" : "Checking availability…"}</span></div>
    {snapshot?.config && <div className="local-model-summary"><div><span>Model</span><strong>{snapshot.config.model}</strong></div><div><span>Context</span><strong>{snapshot.config.profile.contextTokens.toLocaleString()} tokens</strong></div><div><span>Starts with Luci</span><strong>{snapshot.config.startWithLuci ? "On" : "Off"}</strong></div></div>}
    {!observationFailed && state === "available" && <p className="journey-muted">The server is available. The model loads when a session needs it.</p>}
    {!observationFailed && state === "loaded" && <p className="journey-muted">The configured model is currently loaded. A completed task still needs its own checks.</p>}
    {state === "unknown" && <p role="alert" className="journey-error">Luci cannot confirm the previous process has closed. Starting a replacement is disabled.</p>}
    {failure && <p role="alert" className="journey-error">{failureMessages[failure.code] ?? "The local model service needs attention. Check its configuration and refresh its status."}{failure.exitCode !== undefined ? ` Exit code: ${failure.exitCode}.` : ""}</p>}
    {error && <p role="alert" className="journey-error">{error}</p>}
    {observationError && <p role="alert" className="journey-error">{observationError}</p>}
    {notice && <p role="status" className="journey-success">{notice}</p>}
    <div className="setup-actions">
      {snapshot?.canManage && <Button type="button" disabled={!!busy || loading || !canStart} loading={busy === "start"} onClick={() => void change("start")}>Start model</Button>}
      {snapshot?.canManage && <Button type="button" variant="secondary" disabled={!!busy || !canStop || state === "stopping"} loading={busy === "stop"} onClick={() => void change("stop")}>Stop model</Button>}
      <Button type="button" variant="secondary" disabled={!!busy || loading || !!error || !!observationError || dirty || !usable(snapshot)} loading={busy === "use"} onClick={() => void useModel()}>Use for OpenCode</Button>
      <Button type="button" variant="ghost" disabled={!!busy} loading={loading} onClick={() => { setError(""); void read(); }}>Refresh model status</Button>
    </div>
    {snapshot && !snapshot.canManage && <p className="journey-muted">A host administrator manages this service. You can select it when it is available.</p>}
    {snapshot?.canManage && <details open={!snapshot.config}><summary>Configure local model</summary><form onSubmit={event => { event.preventDefault(); void change("save"); }}>
      <fieldset disabled={!!busy || configurationLocked}>
        {active && <p className="journey-muted">Stop the model before changing its configuration.</p>}
        {staleDraft && <p role="alert">Configuration changed elsewhere. Your draft is preserved. Reload the saved configuration before saving.</p>}
        <div className="setup-field-grid">
          <label className="luci-field">Ollama application<input aria-label="Ollama application" aria-describedby={`${helpId}-application`} required autoComplete="off" spellCheck={false} value={draft.executablePath} onChange={event => setField("executablePath", event.target.value)} /><small id={`${helpId}-application`}>The installed Ollama executable on this PC.</small></label>
          <label className="luci-field">Model folder<input aria-label="Model folder" aria-describedby={`${helpId}-folder`} required autoComplete="off" spellCheck={false} value={draft.modelStore} onChange={event => setField("modelStore", event.target.value)} /><small id={`${helpId}-folder`}>The existing folder containing the installed model.</small></label>
          <label className="luci-field">Installed model<input aria-label="Installed model" aria-describedby={`${helpId}-model`} required autoComplete="off" spellCheck={false} value={draft.model} onChange={event => setField("model", event.target.value)} /><small id={`${helpId}-model`}>Exact model name configured for a 16k context.</small></label>
          <label className="luci-field">Local port<input aria-label="Local port" aria-describedby={`${helpId}-port`} required type="number" min={1024} max={65535} step={1} value={draft.port} onChange={event => setField("port", Number(event.target.value))} /><small id={`${helpId}-port`}>Choose an unused port. Existing Ollama servers stay separate.</small></label>
        </div>
        <label className="data-check"><input type="checkbox" checked={draft.enabled} onChange={event => setField("enabled", event.target.checked)} />Enable managed local model</label>
        <label className="data-check"><input type="checkbox" checked={draft.startWithLuci} onChange={event => setField("startWithLuci", event.target.checked)} />Start with Luci when capacity is available</label>
        <p className="journey-muted">Saving does not start the model, download files, or change your active connection.</p>
        <div className="setup-actions"><Button type="submit" loading={busy === "save"} disabled={!dirty || staleDraft || !draft.executablePath.trim() || !draft.modelStore.trim() || !draft.model.trim() || !Number.isInteger(draft.port) || draft.port < 1024 || draft.port > 65535}>Save model configuration</Button><Button type="button" variant="ghost" onClick={() => void read(undefined, true)}>Reload saved configuration</Button></div>
      </fieldset>
    </form></details>}
  </section>;
}
