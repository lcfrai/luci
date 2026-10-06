import { useEffect, useId, useRef, useState } from "react";
import { definitionApi, type ProjectSpecification, type DefinitionAssistance, type Requirement, type DefinitionAgentOptions } from "../../api/definition";
import { errorText, productApi } from "../../api/product";
import { useCockpit } from "../../store/store";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraft, useProjectDraftScope } from "../../hooks/useProjectDraft";
import { definitionDraftValid } from "../../hooks/projectDraftValidation";
import { DraftRecovery } from "../common/DraftRecovery";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";
import { ContextStorage } from "../journey/ContextStorage";
import { JourneyHeader } from "../journey/shared";
import { RepositoryAnalysisPanel } from "./RepositoryAnalysisPanel";
import { Button } from "../fluid/components/button";
import { AgentConnectionSelect } from "../setup/AgentConnectionSelect";
import { useAgentConnections, selectedAgentConnection, configuredAgentModel, agentConnectionIssue } from "../../hooks/useAgentConnections";
import { TabsSubtle, TabsSubtleItem, TabsSubtlePanel } from "../fluid/components/tabs-subtle";
import "./definition.css";

const recordId = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 12)}`;
type Tab = "interview" | "specification" | "history" | "repository";
const views: [Tab, string][] = [["interview", "Guided interview"], ["specification", "Specification"], ["repository", "Repository analysis"], ["history", "Revision history"]];
const questions = [
  { field: "purpose", label: "What should this solution do?", help: "Describe the problem, the result you want and why it matters.", placeholder: "A useful outcome, in your own words…" },
  { field: "audience", label: "Who will use it?", help: "Think about the person, their situation and what they need to achieve.", placeholder: "The people and their main tasks…" },
  { field: "constraints", label: "What does it need to respect?", help: "Include privacy, accessibility, platforms, budget or existing systems. You can leave this open for now.", placeholder: "The important boundaries…" },
  { field: "outOfScope", label: "What should we leave out?", help: "Keep the first complete version focused. Enter one exclusion per line.", placeholder: "Features or changes outside this project…" },
  { field: "acceptance", label: "How will we know it works?", help: "Name one observable outcome per line. We will give each a stable ID for Design, tasks and checks.", placeholder: "For example: a returning user can resume their saved work after reopening the app." },
] as const;

function RecordGroup<T extends { id: string }>({ title, help, items, fields, create, onChange }: {
  title: string; help: string; items: T[]; fields: { key: keyof T; label: string }[]; create(): T; onChange(items: T[]): void;
}) {
  return <details className="definition-group">
    <summary><span>{title}</span><span className="journey-muted">{items.length}</span></summary>
    <div className="definition-group-body"><p className="journey-muted">{help}</p>
      {items.map((item, index) => <fieldset className="definition-record" key={item.id}>
        <legend>{title.replace(/s$/, "")} {index + 1}</legend>
        {fields.map(({ key, label }) => <label className="luci-field" key={String(key)}>{label}
          <textarea rows={2} value={String(item[key] ?? "")} onChange={(e) => onChange(items.map((old) => old.id === item.id ? { ...old, [key]: e.target.value } : old))} />
        </label>)}
        <Button type="button" variant="secondary" onClick={() => onChange(items.filter((old) => old.id !== item.id))}>Remove {title.toLowerCase().replace(/s$/, "")} {index + 1}</Button>
      </fieldset>)}
      <Button type="button" variant="secondary" onClick={() => onChange([...items, create()])}>Add {title.toLowerCase().replace(/s$/, "")}</Button>
    </div>
  </details>;
}

export function DefinitionWorkspace({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  return <ProjectDefinition key={`${scope}:${projectId}`} projectId={projectId} scope={scope} />;
}
function ProjectDefinition({ projectId, scope }: { projectId: string; scope: string | null }) {
  const access = useProjectAccess(projectId);
  const { canRead, canEdit, canExecute, canAssist } = access;
  const [spec, setSpec] = useState<ProjectSpecification | null>(null);
  const savedSpec = useRef<ProjectSpecification | null>(null);
  const [base, setBase] = useState<{ revision: number; briefRevision: number } | null>(null);
  const [tab, setTab] = useState<Tab>("interview");
  const [step, setStep] = useState(0);
  const [dirty, setDirty] = useState(false);
  const draftBase = base ? JSON.stringify(base) : null;
  const draft = useProjectDraft({ scope, kind: "definition", base: draftBase, value: spec, dirty, valid: definitionDraftValid });
  useUnsavedProjectDraft(projectId, dirty, "Definition", "discover", scope ? { retained: draft.retained, discard: () => { draft.clear(); setSpec(savedSpec.current); setDirty(false); } } : undefined);
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<"" | "save" | "approve" | "ask">("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [history, setHistory] = useState<ProjectSpecification[]>([]);
  const [message, setMessage] = useState("");
  const [job, setJob] = useState<DefinitionAssistance | null>(null);
  const [assistError, setAssistError] = useState("");
  const [assistant, setAssistant] = useState<DefinitionAgentOptions>(() => {
    const state = useCockpit.getState(), defaults = { ...state.defaults, ...state.projectDefaults[projectId] };
    return { harness: ["codex", "claude-code", "opencode"].includes(defaults.harness) ? defaults.harness as DefinitionAgentOptions["harness"] : "codex", model: defaults.model === "default" ? "" : defaults.model, ...(defaults.agentConnectionId === undefined ? {} : { agentConnectionId: defaults.agentConnectionId }) };
  });
  const { catalogue, error: connectionError } = useAgentConnections();
  const [localModel, setLocalModel] = useState("");
  const connection = selectedAgentConnection(catalogue, assistant.agentConnectionId);
  const assistantModel = configuredAgentModel(connection, assistant.harness || "codex") || (connection && connection.kind !== "codex-subscription" ? "" : assistant.harness === "opencode" && localModel ? localModel : assistant.model);
  const assistantModelReadOnly = !!configuredAgentModel(connection, assistant.harness || "codex") || !!connection && connection.kind !== "codex-subscription" || assistant.harness === "opencode" && !!localModel;
  const connectionIssue = agentConnectionIssue(connection, assistant.harness || "codex", assistant.agentConnectionId ?? catalogue?.activeConnectionId);
  useEffect(() => {
    if (assistant.harness !== "opencode" || connection) return;
    let stopped = false;
    void productApi.agents().then(value => {
      const local = value.providers.find(item => item.id === "local" && item.configured);
      if (!stopped) setLocalModel(local?.model ? `luci-local/${local.model}` : "");
    }).catch(() => { if (!stopped) setLocalModel(""); });
    return () => { stopped = true; };
  }, [assistant.harness, connection]);
  const alive = useRef(true);
  const submitting = useRef(false);
  const assistGeneration = useRef(0);
  const tabsId = useId(), selectedIndex = views.findIndex(([key]) => key === tab);
  const setLens = useCockpit((s) => s.setLens);
  const load = async () => {
    if (!canRead) return;
    const value = await definitionApi.get(projectId);
    if (!alive.current) return;
    savedSpec.current = value; setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision });
    setDirty(false); setError("");
    if (value.purpose || value.revision) setTab("specification");
  };
  useEffect(() => {
    alive.current = true;
    if (!canRead) return;
    const ticket = ++assistGeneration.current;
    void load().catch((cause) => { if (alive.current) setError(errorText(cause)); });
    void definitionApi.listAssistance(projectId).then(async ({ jobs }) => {
      const latest = jobs.filter(item => item.projectId === projectId && (!item.kind || item.kind === "interview"))
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0];
      if (!latest || !alive.current || ticket !== assistGeneration.current) return;
      const value = await definitionApi.assistance(projectId, latest.id);
      if (alive.current && ticket === assistGeneration.current && value.projectId === projectId && (!value.kind || value.kind === "interview")) setJob(value);
    }).catch(cause => { if (alive.current && ticket === assistGeneration.current) setAssistError(errorText(cause)); });
    return () => { alive.current = false; assistGeneration.current++; };
  }, [projectId, canRead || !!spec]);
  useEffect(() => {
    if (!canRead || !job || (job.status !== "queued" && job.status !== "running")) return;
    let stopped = false;
    const timer = setInterval(() => {
      void definitionApi.assistance(projectId, job.id).then((value) => { if (!stopped) setJob(value); })
        .catch((cause) => { if (!stopped) setAssistError(errorText(cause)); });
    }, 2500);
    return () => { stopped = true; clearInterval(timer); };
  }, [projectId, job?.id, job?.status, canRead]);
  const patch = (value: Partial<ProjectSpecification>) => {
    if (spec) setSpec({ ...spec, ...value, status: "draft", digest: "" });
    setDirty(true); setNotice("");
  };
  async function save() {
    if (!canEdit || !spec || !base || submitting.current || draft.pending) return;
    submitting.current = true; setBusy(true); setPending("save"); setError(""); setNotice("");
    try {
      const saved = await definitionApi.save(projectId, spec, base.revision, base.briefRevision);
      if (alive.current) { draft.clear(); savedSpec.current = saved; setSpec(saved); setBase({ revision: saved.revision, briefRevision: saved.briefRevision }); setDirty(false); setNotice(`Definition ${saved.revision} saved. The full specification is available to your agent.`); }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current) { setBusy(false); setPending(""); } }
  }
  async function approve() {
    if (!canEdit || !spec || dirty || draft.pending || submitting.current) return;
    submitting.current = true; setBusy(true); setPending("approve"); setError("");
    try {
      const value = await definitionApi.approve(projectId, spec.revision, spec.briefRevision);
      if (alive.current) { savedSpec.current = value; setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision }); setNotice(`Definition ${value.revision} approved for design and implementation.`); }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current) { setBusy(false); setPending(""); } }
  }
  async function ask() {
    if (!canAssist || !spec || !message.trim() || dirty || draft.pending || busy || connectionIssue) return;
    const ticket = ++assistGeneration.current;
    setBusy(true); setPending("ask"); setAssistError("");
    try { const value = await definitionApi.assist(projectId, spec.revision, message.trim(), { ...assistant, ...(assistantModel ? { model: assistantModel } : { model: undefined }) }); if (alive.current && ticket === assistGeneration.current) { setJob(value); setMessage(""); } }
    catch (cause) { if (alive.current && ticket === assistGeneration.current) setAssistError(errorText(cause)); }
    finally { if (alive.current && ticket === assistGeneration.current) { setBusy(false); setPending(""); } }
  }
  const selectTab = (next: Tab) => {
    setTab(next);
    if (next === "history") void definitionApi.history(projectId).then((r) => { if (alive.current) setHistory(r.revisions); }).catch((cause) => { if (alive.current) setError(errorText(cause)); });
  };
  const requirementChange = (id: string, value: Partial<Requirement>) => spec && patch({ requirements: spec.requirements.map((r) => r.id === id ? { ...r, ...value } : r) });
  const addRequirement = () => spec && patch({ requirements: [...spec.requirements, { id: recordId("AC"), title: "", priority: "must", acceptance: "" }] });
  const removeRequirement = (id: string) => spec && patch({ requirements: spec.requirements.filter((r) => r.id !== id), journeys: spec.journeys.map((j) => ({ ...j, requirementIds: j.requirementIds.filter((r) => r !== id) })) });
  const interviewValue = !spec ? "" : step === 3 ? spec.outOfScope.join("\n") : step === 4 ? spec.requirements.map((r) => r.acceptance || r.title).join("\n") : spec[questions[step].field as "purpose" | "audience" | "constraints"];
  const interviewChange = (text: string) => {
    if (!spec) return;
    if (step === 3) patch({ outOfScope: text.split("\n") });
    else if (step === 4) {
      const lines = text.split("\n");
      // Existing records keep their identities while their words are edited.
      patch({ requirements: lines.map((line, index) => ({ id: spec.requirements[index]?.id || recordId("AC"), title: line, acceptance: line, priority: spec.requirements[index]?.priority || "must" })) });
    } else patch({ [questions[step].field]: text });
  };
  return <section className="journey-page definition-workspace">
    <JourneyHeader title="Define your project" subtitle="Turn an idea or existing application into a clear, testable product definition.">
      {spec && <span className="journey-revision" role="status">{dirty ? "Unsaved changes" : `${spec.status === "approved" ? "Approved" : spec.status === "outdated" ? "Needs reconciliation" : "Draft"} · Revision ${spec.revision}`}</span>}
    </JourneyHeader>
    {access.message && <p role="status" className="journey-muted">{access.message}</p>}
    {error && <div className="journey-error" role="alert">{error} <Button type="button" variant="secondary" disabled={busy} onClick={() => { draft.offerRecovery(); void load().catch((e) => setError(errorText(e))); }}>Reload saved definition</Button>{dirty && <p>Your edited draft is still here. Reload to compare it with the saved definition and choose what to recover.</p>}</div>}
    <DraftRecovery draft={draft} label="Definition" base={draftBase} saved={savedSpec.current} busy={busy} readOnly={!canEdit} onDiscard={() => { setSpec(savedSpec.current); setDirty(false); }} onRestore={value => { if (spec) { setSpec({ ...value, projectId, revision: spec.revision, briefRevision: spec.briefRevision, status: "draft", digest: "" }); setDirty(true); setTab("specification"); } }} />
    {!spec && !error && <p role="status">Loading your project definition…</p>}
    {spec && canRead && <>
      <TabsSubtle className="definition-tabs" aria-label="Definition views" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => selectTab(views[index][0])}>
        {views.map(([, label], index) => <TabsSubtleItem key={label} index={index} label={label} />)}
      </TabsSubtle>
      {spec.status === "outdated" && <div className="journey-error" role="status">The brief changed after this definition was saved. Reconcile the current purpose and requirements before approving again.</div>}
      {views.map(([key], index) => <TabsSubtlePanel key={key} idPrefix={tabsId} index={index} selectedIndex={selectedIndex}>
      <div className="definition-layout" style={tab === "repository" ? { gridTemplateColumns: "minmax(0, 1fr)" } : undefined}>
        <div className="definition-content">
          <fieldset disabled={!canEdit || busy || !!draft.pending} className="definition-fields">
            {tab === "interview" && <div className="definition-interview">
              <label className="luci-field">Starting point<select value={spec.startingPoint} onChange={(e) => patch({ startingPoint: e.target.value as "new" | "existing" })}><option value="new">A new idea</option><option value="existing">An existing application</option></select></label>
              {spec.startingPoint === "existing" && <div><p className="journey-muted">Analyse actual source with your chosen provider, review the observed behaviour and gaps, then apply a draft. Save any edited answers before starting.</p><Button type="button" variant="secondary" onClick={() => setTab("repository")}>Analyse this repository</Button></div>}
              <p className="journey-muted">Question {step + 1} of {questions.length}</p>
              <h2>{questions[step].label}</h2><p>{questions[step].help}</p>
              <label className="luci-field"><span className="sr-only">{questions[step].label}</span><textarea rows={7} value={interviewValue} placeholder={questions[step].placeholder} onChange={(e) => interviewChange(e.target.value)} /></label>
              <div className="journey-actions"><Button type="button" variant="secondary" disabled={step === 0} onClick={() => setStep(step - 1)}>Back</Button>
                <Button type="button" variant="secondary" onClick={() => step < questions.length - 1 ? setStep(step + 1) : setTab("specification")}>{step < questions.length - 1 ? "Next question" : "Review specification"}</Button></div>
              <p className="journey-muted">You can switch to the specification or save at any time. This guided outline uses your answers; the connected assistant can propose deeper questions.</p>
            </div>}
            {tab === "specification" && <div className="definition-specification">
              <h2>Product outline</h2>
              <label className="luci-field">The idea<textarea rows={4} value={spec.purpose} onChange={(e) => patch({ purpose: e.target.value })} /></label>
              <label className="luci-field">Who it is for<textarea rows={2} value={spec.audience} onChange={(e) => patch({ audience: e.target.value })} /></label>
              <label className="luci-field">Boundaries and constraints<textarea rows={3} value={spec.constraints} onChange={(e) => patch({ constraints: e.target.value })} /></label>
              <label className="luci-field">Out of scope<textarea rows={2} value={spec.outOfScope.join("\n")} onChange={(e) => patch({ outOfScope: e.target.value.split("\n") })} /><small>One exclusion per line.</small></label>
              <section className="definition-requirements"><h2>Requirements and acceptance</h2><p className="journey-muted">Keep each ID through design, implementation and verification. Completing a task does not prove a requirement.</p>
                {!spec.requirements.length && <p>Add the observable outcomes that define a complete solution.</p>}
                {spec.requirements.map((r, index) => <fieldset key={r.id} className="definition-record"><legend>{r.id}</legend>
                  <label className="luci-field">Requirement {index + 1}<input value={r.title} onChange={(e) => requirementChange(r.id, { title: e.target.value })} /></label>
                  <label className="luci-field">Priority<select value={r.priority} onChange={(e) => requirementChange(r.id, { priority: e.target.value as Requirement["priority"] })}><option value="must">Must have</option><option value="should">Should have</option><option value="could">Could have</option></select></label>
                  <label className="luci-field">Acceptance scenario {index + 1}<textarea rows={3} placeholder="Given a starting state, when the person takes an action, then an observable outcome happens." value={r.acceptance} onChange={(e) => requirementChange(r.id, { acceptance: e.target.value })} /></label>
                  <Button type="button" variant="secondary" onClick={() => removeRequirement(r.id)}>Remove requirement {index + 1}</Button>
                </fieldset>)}
                <Button type="button" variant="secondary" onClick={addRequirement}>Add requirement</Button>
              </section>
              <details className="definition-group"><summary><span>User journeys</span><span className="journey-muted">{spec.journeys.length}</span></summary><div className="definition-group-body">
                {spec.journeys.map((j, index) => <fieldset className="definition-record" key={j.id}><legend>Journey {index + 1}</legend>
                  <label className="luci-field">Journey name<input value={j.name} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, name: e.target.value } : old) })} /></label>
                  <label className="luci-field">Person or role<input value={j.actor} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, actor: e.target.value } : old) })} /></label>
                  <label className="luci-field">Steps and recovery<textarea value={j.steps} rows={3} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, steps: e.target.value } : old) })} /></label>
                  <fieldset className="definition-links"><legend>Related requirements</legend>{spec.requirements.map((r) => <label key={r.id}><input type="checkbox" checked={j.requirementIds.includes(r.id)} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, requirementIds: e.target.checked ? [...old.requirementIds, r.id] : old.requirementIds.filter((id) => id !== r.id) } : old) })} />{r.id} · {r.title}</label>)}</fieldset>
                  <Button type="button" variant="secondary" onClick={() => patch({ journeys: spec.journeys.filter((old) => old.id !== j.id) })}>Remove journey {index + 1}</Button>
                </fieldset>)}<Button type="button" variant="secondary" onClick={() => patch({ journeys: [...spec.journeys, { id: recordId("journey"), name: "", actor: "", steps: "", requirementIds: [] }] })}>Add user journey</Button>
              </div></details>
              <RecordGroup title="Access roles" help="Describe what each person can do and what they must not access." items={spec.roles} fields={[{ key: "name", label: "Role name" }, { key: "canDo", label: "Can do" }, { key: "cannotDo", label: "Cannot do or access" }]} create={() => ({ id: recordId("role"), name: "", canDo: "", cannotDo: "" })} onChange={(roles) => patch({ roles })} />
              <RecordGroup title="Data entities" help="Name the data, fields, privacy and retention expectations." items={spec.entities} fields={[{ key: "name", label: "Entity name" }, { key: "fields", label: "Fields and relationships" }, { key: "privacy", label: "Privacy and retention" }]} create={() => ({ id: recordId("entity"), name: "", fields: "", privacy: "" })} onChange={(entities) => patch({ entities })} />
              <RecordGroup title="Integrations" help="Record external systems and their cost, credential or data boundaries." items={spec.integrations} fields={[{ key: "name", label: "Integration name" }, { key: "purpose", label: "Purpose" }, { key: "constraint", label: "Boundaries and requirements" }]} create={() => ({ id: recordId("integration"), name: "", purpose: "", constraint: "" })} onChange={(integrations) => patch({ integrations })} />
              <RecordGroup title="Glossary terms" help="Agree on the words people and agents should use." items={spec.glossary} fields={[{ key: "term", label: "Term" }, { key: "meaning", label: "Meaning in this project" }]} create={() => ({ id: recordId("term"), term: "", meaning: "" })} onChange={(glossary) => patch({ glossary })} />
              <RecordGroup title="Decisions" help="Record consequential choices and the alternatives considered." items={spec.decisions} fields={[{ key: "title", label: "Decision title" }, { key: "decision", label: "Decision" }, { key: "reason", label: "Why" }, { key: "alternatives", label: "Alternatives considered" }]} create={() => ({ id: recordId("decision"), title: "", decision: "", reason: "", alternatives: "" })} onChange={(decisions) => patch({ decisions })} />
              <RecordGroup title="Open questions" help="Capture the gaps. Answer them before approving the definition." items={spec.questions} fields={[{ key: "question", label: "Question" }, { key: "answer", label: "Answer" }]} create={() => ({ id: recordId("question"), question: "", answer: "" })} onChange={(questions) => patch({ questions })} />
            </div>}
          </fieldset>
          {tab === "repository" && <RepositoryAnalysisPanel projectId={projectId} specification={spec} dirty={dirty || busy || !!draft.pending} onApplied={value => {
            savedSpec.current = value; setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision }); setDirty(false); setError(""); setTab("specification");
            setNotice(`Repository analysis saved as definition ${value.revision}. Review the draft and resolve open questions before approving.`);
          }} />}
          {tab === "history" && <section><h2>Saved definition snapshots</h2>{!history.length && <p>No structured revisions yet. Existing brief text is preserved when you save your first definition.</p>}<ol className="definition-history">{history.slice().reverse().map((r) => <li key={r.revision}><strong>Revision {r.revision} · {r.status}</strong><p>{r.purpose}</p><small>Brief {r.briefRevision} · {r.requirements.length} requirements · {new Date(r.updatedAt).toLocaleString()}</small><details><summary>Read this snapshot</summary><pre>{JSON.stringify(r, null, 2)}</pre></details></li>)}</ol></section>}
        </div>
        {tab !== "repository" && <aside className="definition-assistant" aria-label="Definition assistant">
          <h2>Work through it with Luci</h2><p>Ask your connected agent to find gaps, explore an idea or propose a fuller definition. Suggestions remain drafts until you accept them.</p>
          <fieldset disabled={!canAssist || busy || job?.status === "running" || job?.status === "queued"} className="definition-fields">
            <label className="luci-field">Assistant provider<select value={assistant.harness} onChange={event => setAssistant({ ...assistant, harness: event.target.value as DefinitionAgentOptions["harness"], model: "" })}><option value="codex">Codex</option><option value="claude-code">Claude Code</option><option value="opencode">OpenCode</option></select></label>
            <AgentConnectionSelect catalogue={catalogue} value={assistant.agentConnectionId} onChange={id => setAssistant({ ...assistant, agentConnectionId: id })} label="Assistant connection" />
            <label className="luci-field">Assistant model<input value={assistantModel ?? ""} readOnly={assistantModelReadOnly} onChange={event => setAssistant({ ...assistant, model: event.target.value })} placeholder="Agent default, or an exact available model ID" /></label>
          </fieldset>
          {(connectionIssue || connectionError) && <p className="journey-error" role="alert">{connectionIssue || connectionError}</p>}
          <label className="luci-field">What would you like to explore?<textarea rows={4} value={message} disabled={!canAssist || busy || job?.status === "running" || job?.status === "queued"} onChange={(e) => setMessage(e.target.value)} placeholder="Help me make the first version complete. Ask the next useful question." /></label>
          <Button type="button" variant="secondary" loading={pending === "ask"} onClick={() => void ask()} disabled={!canAssist || dirty || busy || !!connectionIssue || !message.trim() || job?.status === "running" || job?.status === "queued"}>Ask Luci</Button>
          {dirty && <p className="journey-muted">Save your draft first so the assistant works from the same definition.</p>}
          {assistError && <p className="journey-error" role="alert">{assistError}</p>}
          {job && <div className="definition-answer"><p role="status">{job.message || job.status}</p>
            {(job.status === "queued" || job.status === "running") && <Button type="button" variant="secondary" disabled={!canExecute} onClick={() => { if (!canExecute) return; void definitionApi.cancelAssistance(projectId, job.id).then(setJob).catch((e) => setAssistError(errorText(e))); }}>Cancel assistance</Button>}
            {job.result && <><h3>{job.result.question}</h3><p>{job.result.rationale}</p>{job.result.options.map((option) => <Button type="button" variant="secondary" className="definition-option" key={option} onClick={() => setMessage(option)}>{option}</Button>)}
              {job.result.proposal && <details><summary>Review proposed definition</summary><p>{job.result.proposal.purpose}</p><ul>{job.result.proposal.requirements.map((r) => <li key={r.id}>{r.title}<br /><small>{r.acceptance}</small></li>)}</ul><p>This replaces the edited definition with a proposed draft. It does not approve or save it.</p><Button type="button" variant="secondary" disabled={!canEdit || !!draft.pending} onClick={() => { const proposal = job.result?.proposal; if (proposal) { setSpec({ ...proposal, projectId, version: 1, revision: spec.revision, briefRevision: spec.briefRevision, status: "draft", digest: "" }); setDirty(true); setTab("specification"); setNotice("Proposal accepted as an editable draft. Review it and save when ready."); } }}>Use this proposal as my draft</Button></details>}
            </>}
          </div>}
          {canEdit && !canAssist && <p className="journey-muted">Assistance requires execution and secret access for the selected provider.</p>}
          <p className="journey-muted">The assistant uses your selected connection and a ready Docker runtime. Your definition is sent to that provider.</p>
        </aside>}
      </div>
      </TabsSubtlePanel>)}
      <footer className="definition-footer"><div className="journey-actions">
        {dirty || spec.status === "outdated" || !spec.revision
          ? <Button type="button" loading={pending === "save"} disabled={!canEdit || busy || !!draft.pending || (!dirty && spec.status !== "outdated")} onClick={() => void save()}>Save definition</Button>
          : spec.status === "approved"
            ? <Button type="button" disabled={busy} onClick={() => setLens("design")}>Continue to Design</Button>
            : <Button type="button" loading={pending === "approve"} disabled={!canEdit || busy || !!draft.pending} onClick={() => void approve()}>Approve definition</Button>}
      </div><p role="status" className="journey-muted">{notice || `${spec.requirements.length} requirements · ${spec.questions.filter((q) => !q.answer.trim()).length} open questions`}</p></footer>
    </>}
    <ContextStorage projectId={projectId} />
  </section>;
}
