import { useCallback, useEffect, useId, useRef, useState } from "react";
import { notifyProjectContextChanged, PROJECT_CONTEXT_EDIT, takeProjectContextEdit } from "../../api/projectContextEvents";
import { definitionApi, type ProjectSpecification, type Requirement } from "../../api/definition";
import { errorText } from "../../api/product";
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
  { field: "acceptance", label: "How will we know it works?", help: "Describe each outcome and how someone can check it. These outcomes stay linked to your design, tasks and checks as you edit or reorder them.", placeholder: "For example: a returning user can resume their saved work after reopening the app." },
] as const;

function RecordGroup<T extends { id: string }>({ title, help, items, fields, create, onChange, focusTarget, onFocused }: {
  title: string; help: string; items: T[]; fields: { key: keyof T; label: string }[]; create(): T; onChange(items: T[]): void;
  focusTarget?: { id: string; field: keyof T }; onFocused?(found: boolean): void;
}) {
  const group = useRef<HTMLDetailsElement>(null);
  const inputs = useRef(new Map<string, HTMLTextAreaElement>());
  useEffect(() => {
    if (!focusTarget || !group.current) return;
    group.current.open = true;
    // Finish navigation layout and the draft dialog's focus restoration first.
    let secondFrame = 0;
    const firstFrame = requestAnimationFrame(() => { secondFrame = requestAnimationFrame(() => {
      const input = inputs.current.get(`${focusTarget.id}:${String(focusTarget.field)}`);
      if (input) input.focus();
      else group.current?.querySelector("summary")?.focus();
      onFocused?.(!!input);
    }); });
    return () => { cancelAnimationFrame(firstFrame); cancelAnimationFrame(secondFrame); };
  }, [focusTarget?.id, focusTarget?.field, onFocused]);
  return <details ref={group} className="definition-group">
    <summary><span>{title}</span><span className="journey-muted">{items.length}</span></summary>
    <div className="definition-group-body"><p className="journey-muted">{help}</p>
      {items.map((item, index) => <fieldset className="definition-record" key={item.id}>
        <legend>{title.replace(/s$/, "")} {index + 1}</legend>
        {fields.map(({ key, label }) => <label className="luci-field" key={String(key)}>{label}
          <textarea ref={element => { const id = `${item.id}:${String(key)}`; if (element) inputs.current.set(id, element); else inputs.current.delete(id); }} rows={2} value={String(item[key] ?? "")} onChange={(e) => onChange(items.map((old) => old.id === item.id ? { ...old, [key]: e.target.value } : old))} />
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
  const { canRead, canEdit } = access;
  const [spec, setSpec] = useState<ProjectSpecification | null>(null);
  const savedSpec = useRef<ProjectSpecification | null>(null);
  const [base, setBase] = useState<{ revision: number; briefRevision: number } | null>(null);
  const [tab, setTab] = useState<Tab>("interview");
  const [step, setStep] = useState(0);
  const [questionTarget, setQuestionTarget] = useState<string | null>(null);
  const [questionNotice, setQuestionNotice] = useState("");
  useEffect(() => {
    if (!spec || !canRead) return;
    const edit = () => {
      const target = takeProjectContextEdit(scope, projectId, "definition");
      if (target) { setTab("specification"); setQuestionTarget(target.questionId ?? null); setQuestionNotice(""); }
    };
    edit(); window.addEventListener(PROJECT_CONTEXT_EDIT, edit);
    return () => window.removeEventListener(PROJECT_CONTEXT_EDIT, edit);
  }, [scope, projectId, !!spec, canRead]);
  const [dirty, setDirty] = useState(false);
  const draftBase = base ? JSON.stringify(base) : null;
  const draft = useProjectDraft({ scope, kind: "definition", base: draftBase, baseValue: savedSpec.current, value: spec, dirty, valid: definitionDraftValid });
  useUnsavedProjectDraft(projectId, dirty, "Definition", "discover", scope ? { retained: draft.retained, discard: () => { draft.clear(); setSpec(savedSpec.current); setDirty(false); } } : undefined);
  const [busy, setBusy] = useState(false);
  const [savedRead, setSavedRead] = useState<"loading" | "ready" | "failed">("loading");
  const [pending, setPending] = useState<"" | "save" | "approve">("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const questionFocused = useCallback((found: boolean) => {
    setQuestionTarget(null);
    setQuestionNotice(found ? "" : "This question is no longer in the current definition draft. Review the remaining questions or restore the intended draft.");
  }, []);
  useEffect(() => {
    if (questionTarget && !canEdit) {
      setQuestionTarget(null);
      setQuestionNotice("Editing access is required to answer this question. Your saved definition is unchanged.");
    }
  }, [questionTarget, canEdit]);
  const [history, setHistory] = useState<ProjectSpecification[]>([]);
  const alive = useRef(true);
  const submitting = useRef(false);
  const reading = useRef({ generation: 0, pending: false, ready: false });
  const locked = busy || savedRead !== "ready";
  const tabsId = useId(), selectedIndex = views.findIndex(([key]) => key === tab);
  const setLens = useCockpit((s) => s.setLens);
  const missingJourneyLinks = spec?.journeys.some(journey => journey.requirementIds.some(id => !spec.requirements.some(requirement => requirement.id === id))) ?? false;
  const load = async () => {
    if (!canRead || reading.current.pending || submitting.current) return;
    const generation = ++reading.current.generation;
    reading.current.pending = true; reading.current.ready = false; setSavedRead("loading");
    try {
      const value = await definitionApi.get(projectId);
      if (!alive.current || reading.current.generation !== generation) return;
      savedSpec.current = value; setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision });
      reading.current.ready = true; setSavedRead("ready"); setDirty(false); setError("");
      if (value.purpose || value.revision) setTab("specification");
    } catch (cause) {
      if (alive.current && reading.current.generation === generation) { setSavedRead("failed"); setError(errorText(cause)); }
    } finally {
      if (reading.current.generation === generation) reading.current.pending = false;
    }
  };
  useEffect(() => {
    alive.current = true;
    if (!canRead) return;
    void load();
    return () => { alive.current = false; reading.current.generation++; reading.current.pending = false; reading.current.ready = false; };
  }, [projectId, canRead || !!spec]);
  const reload = () => {
    if (!canRead || reading.current.pending || submitting.current) return;
    draft.offerRecovery(); void load();
  };
  const patch = (value: Partial<ProjectSpecification>) => {
    if (!canEdit || !reading.current.ready || submitting.current || draft.pending) return;
    if (spec) setSpec({ ...spec, ...value, status: "draft", digest: "" });
    setDirty(true); setNotice("");
  };
  async function save() {
    if (!canEdit || !spec || !base || !reading.current.ready || submitting.current || draft.pending || missingJourneyLinks) return;
    submitting.current = true; setBusy(true); setPending("save"); setError(""); setNotice("");
    try {
      const saved = await definitionApi.save(projectId, spec, base.revision, base.briefRevision);
      if (alive.current) { draft.clear(); savedSpec.current = saved; setSpec(saved); setBase({ revision: saved.revision, briefRevision: saved.briefRevision }); setDirty(false); setNotice(`Definition ${saved.revision} saved. The full specification is available to your agent.`); notifyProjectContextChanged(scope, projectId); }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current) { setBusy(false); setPending(""); } }
  }
  async function approve() {
    if (!canEdit || !spec || !reading.current.ready || dirty || draft.pending || submitting.current) return;
    submitting.current = true; setBusy(true); setPending("approve"); setError("");
    try {
      const value = await definitionApi.approve(projectId, spec.revision, spec.briefRevision);
      if (alive.current) { savedSpec.current = value; setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision }); setNotice(`Definition ${value.revision} approved for design and implementation.`); notifyProjectContextChanged(scope, projectId); }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current) { setBusy(false); setPending(""); } }
  }
  const selectTab = (next: Tab) => {
    setQuestionTarget(null);
    setTab(next);
    if (next === "history") void definitionApi.history(projectId).then((r) => { if (alive.current) setHistory(r.revisions); }).catch((cause) => { if (alive.current) setError(errorText(cause)); });
  };
  const requirementChange = (id: string, value: Partial<Requirement>) => spec && patch({ requirements: spec.requirements.map((r) => r.id === id ? { ...r, ...value } : r) });
  const addRequirement = () => spec && patch({ requirements: [...spec.requirements, { id: recordId("AC"), title: "", priority: "must", acceptance: "" }] });
  const removeRequirement = (id: string) => spec && patch({ requirements: spec.requirements.filter((r) => r.id !== id), journeys: spec.journeys.map((j) => ({ ...j, requirementIds: j.requirementIds.filter((r) => r !== id) })) });
  const moveRequirement = (index: number, offset: -1 | 1) => {
    if (!spec || index + offset < 0 || index + offset >= spec.requirements.length) return;
    const requirements = [...spec.requirements];
    [requirements[index], requirements[index + offset]] = [requirements[index + offset], requirements[index]];
    patch({ requirements });
  };
  const interviewValue = !spec ? "" : step === 3 ? spec.outOfScope.join("\n") : step === 4 ? "" : spec[questions[step].field as "purpose" | "audience" | "constraints"];
  const interviewChange = (text: string) => {
    if (!spec) return;
    if (step === 3) patch({ outOfScope: text.split("\n") });
    else if (step !== 4) patch({ [questions[step].field]: text });
  };
  const openQuestionCount = spec?.questions.filter((question) => !question.answer.trim()).length ?? 0;
  return <section className="journey-page definition-workspace">
    <JourneyHeader title="Define your project" subtitle="Turn an idea or existing application into a clear, testable product definition.">
      {spec && <span className="journey-revision" role="status">{dirty ? "Unsaved changes" : `${spec.status === "approved" ? "Approved" : spec.status === "outdated" ? "Needs reconciliation" : "Draft"} · Revision ${spec.revision}`}</span>}
    </JourneyHeader>
    <div className="definition-conversation-link">
      <p>Use this definition to agree what will be built. Luci uses it to explain the next step and the effect of a change.</p>
      <Button type="button" variant="secondary" disabled={!canRead || locked || dirty || !!draft.pending} onClick={() => window.dispatchEvent(new CustomEvent("luci:orchestrator-open", { detail: { projectId } }))}>Discuss or request a change</Button>
      {(dirty || !!draft.pending) && <p className="journey-muted">Save or resolve this draft first so Luci sees the context you intend to use.</p>}
    </div>
    {access.message && <p role="status" className="journey-muted">{access.message}</p>}
    {error && <div className="journey-error" role="alert">{error} <Button type="button" variant="secondary" loading={savedRead === "loading"} disabled={busy || savedRead === "loading" || !canRead} onClick={reload}>Reload saved definition</Button>{dirty && <p>Your edited draft is still here. Reload to compare it with the saved definition and choose what to recover.</p>}</div>}
    {spec && savedRead === "loading" && <p role="status">Loading saved definition… Your draft is kept while the current revision is checked.</p>}
    {spec && savedRead === "failed" && <p role="status">Reload the saved definition before editing or recovering your draft. Your browser draft is unchanged.</p>}
    <DraftRecovery merge draft={draft} label="Definition" base={draftBase} saved={savedSpec.current} busy={locked} readOnly={!canEdit} onDiscard={() => { setSpec(savedSpec.current); setDirty(false); }} onRestore={value => { if (spec) { setSpec({ ...value, projectId, revision: spec.revision, briefRevision: spec.briefRevision, status: "draft", digest: "" }); setDirty(true); setTab("specification"); } }} />
    {missingJourneyLinks && <p role="alert" className="journey-warning">A user journey still refers to an outcome that was removed. Review its related requirements below: choose any replacement outcome and explicitly remove the unavailable link before saving.</p>}
    {!spec && !error && <p role="status">Loading your project definition…</p>}
    {spec && canRead && <>
      <TabsSubtle className="definition-tabs" aria-label="Definition views" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => selectTab(views[index][0])}>
        {views.map(([, label], index) => <TabsSubtleItem key={label} index={index} label={label} />)}
      </TabsSubtle>
      {spec.status === "outdated" && <div className="journey-error" role="status">The brief changed after this definition was saved. Reconcile the current purpose and requirements before approving again.</div>}
      {views.map(([key], index) => <TabsSubtlePanel key={key} idPrefix={tabsId} index={index} selectedIndex={selectedIndex}>
      <div className="definition-layout" style={tab === "repository" ? { gridTemplateColumns: "minmax(0, 1fr)" } : undefined}>
        <div className="definition-content">
          <fieldset disabled={!canEdit || locked || !!draft.pending} className="definition-fields">
            {tab === "interview" && <div className="definition-interview">
              <label className="luci-field">Starting point<select value={spec.startingPoint} onChange={(e) => patch({ startingPoint: e.target.value as "new" | "existing" })}><option value="new">A new idea</option><option value="existing">An existing application</option></select></label>
              {spec.startingPoint === "existing" && <div><p className="journey-muted">Analyse actual source with your chosen provider, review the observed behaviour and gaps, then apply a draft. Save any edited answers before starting.</p><Button type="button" variant="secondary" onClick={() => setTab("repository")}>Analyse this repository</Button></div>}
              <p className="journey-muted">Question {step + 1} of {questions.length}</p>
              <h2>{questions[step].label}</h2><p>{questions[step].help}</p>
              {step === 4 ? <div className="definition-interview-outcomes">
                {!spec.requirements.length && <p>Add an outcome, such as “Customers can book an appointment,” and describe how you will check it.</p>}
                {spec.requirements.map((requirement, index) => <fieldset className="definition-record" key={requirement.id}>
                  <legend>Outcome {index + 1}</legend>
                  <label className="luci-field">Outcome {index + 1}<input value={requirement.title} onChange={event => requirementChange(requirement.id, { title: event.target.value })} placeholder="What the person should be able to do" /></label>
                  <label className="luci-field">How to check outcome {index + 1}<textarea rows={3} value={requirement.acceptance} onChange={event => requirementChange(requirement.id, { acceptance: event.target.value })} placeholder={questions[step].placeholder} /></label>
                  <div className="journey-actions">
                    <Button type="button" variant="secondary" disabled={index === 0} onClick={() => moveRequirement(index, -1)}>Move outcome {index + 1} up</Button>
                    <Button type="button" variant="secondary" disabled={index === spec.requirements.length - 1} onClick={() => moveRequirement(index, 1)}>Move outcome {index + 1} down</Button>
                    <Button type="button" variant="secondary" onClick={() => removeRequirement(requirement.id)}>Remove outcome {index + 1}</Button>
                  </div>
                </fieldset>)}
                <Button type="button" variant="secondary" onClick={addRequirement}>Add an outcome</Button>
              </div> : <label className="luci-field"><span className="sr-only">{questions[step].label}</span><textarea rows={7} value={interviewValue} placeholder={questions[step].placeholder} onChange={(e) => interviewChange(e.target.value)} /></label>}
              <div className="journey-actions"><Button type="button" variant="secondary" disabled={step === 0} onClick={() => setStep(step - 1)}>Back</Button>
                <Button type="button" variant="secondary" onClick={() => step < questions.length - 1 ? setStep(step + 1) : setTab("specification")}>{step < questions.length - 1 ? "Next question" : "Review specification"}</Button></div>
              <p className="journey-muted">You can switch to the specification or save at any time. These answers become the shared context for your conversation with Luci, the design and the implementation.</p>
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
              <details className="definition-group" open={missingJourneyLinks || undefined}><summary><span>User journeys</span><span className="journey-muted">{spec.journeys.length}</span></summary><div className="definition-group-body">
                {spec.journeys.map((j, index) => <fieldset className="definition-record" key={j.id}><legend>Journey {index + 1}</legend>
                  <label className="luci-field">Journey name<input value={j.name} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, name: e.target.value } : old) })} /></label>
                  <label className="luci-field">Person or role<input value={j.actor} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, actor: e.target.value } : old) })} /></label>
                  <label className="luci-field">Steps and recovery<textarea value={j.steps} rows={3} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, steps: e.target.value } : old) })} /></label>
                  <fieldset className="definition-links"><legend>Related requirements</legend>{spec.requirements.map((r) => <label key={r.id}><input type="checkbox" checked={j.requirementIds.includes(r.id)} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, requirementIds: e.target.checked ? [...old.requirementIds, r.id] : old.requirementIds.filter((id) => id !== r.id) } : old) })} />{r.id} · {r.title}</label>)}
                    {j.requirementIds.filter(id => !spec.requirements.some(requirement => requirement.id === id)).map(id => <label key={id}><input type="checkbox" checked aria-label={`Remove unavailable outcome ${id} from journey ${index + 1}`} onChange={() => patch({ journeys: spec.journeys.map(old => old.id === j.id ? { ...old, requirementIds: old.requirementIds.filter(value => value !== id) } : old) })} />{id} · removed outcome. Uncheck this unavailable link; the journey and its steps will be kept.</label>)}
                  </fieldset>
                  <Button type="button" variant="secondary" onClick={() => patch({ journeys: spec.journeys.filter((old) => old.id !== j.id) })}>Remove journey {index + 1}</Button>
                </fieldset>)}<Button type="button" variant="secondary" onClick={() => patch({ journeys: [...spec.journeys, { id: recordId("journey"), name: "", actor: "", steps: "", requirementIds: [] }] })}>Add user journey</Button>
              </div></details>
              <RecordGroup title="Access roles" help="Describe what each person can do and what they must not access." items={spec.roles} fields={[{ key: "name", label: "Role name" }, { key: "canDo", label: "Can do" }, { key: "cannotDo", label: "Cannot do or access" }]} create={() => ({ id: recordId("role"), name: "", canDo: "", cannotDo: "" })} onChange={(roles) => patch({ roles })} />
              <RecordGroup title="Data entities" help="Name the data, fields, privacy and retention expectations." items={spec.entities} fields={[{ key: "name", label: "Entity name" }, { key: "fields", label: "Fields and relationships" }, { key: "privacy", label: "Privacy and retention" }]} create={() => ({ id: recordId("entity"), name: "", fields: "", privacy: "" })} onChange={(entities) => patch({ entities })} />
              <RecordGroup title="Integrations" help="Record external systems and their cost, credential or data boundaries." items={spec.integrations} fields={[{ key: "name", label: "Integration name" }, { key: "purpose", label: "Purpose" }, { key: "constraint", label: "Boundaries and requirements" }]} create={() => ({ id: recordId("integration"), name: "", purpose: "", constraint: "" })} onChange={(integrations) => patch({ integrations })} />
              <RecordGroup title="Glossary terms" help="Agree on the words people and agents should use." items={spec.glossary} fields={[{ key: "term", label: "Term" }, { key: "meaning", label: "Meaning in this project" }]} create={() => ({ id: recordId("term"), term: "", meaning: "" })} onChange={(glossary) => patch({ glossary })} />
              <RecordGroup title="Decisions" help="Record consequential choices and the alternatives considered." items={spec.decisions} fields={[{ key: "title", label: "Decision title" }, { key: "decision", label: "Decision" }, { key: "reason", label: "Why" }, { key: "alternatives", label: "Alternatives considered" }]} create={() => ({ id: recordId("decision"), title: "", decision: "", reason: "", alternatives: "" })} onChange={(decisions) => patch({ decisions })} />
              {questionNotice && <p role="status" className="journey-warning">{questionNotice}</p>}
              <RecordGroup title="Open questions" help="Capture the gaps. Answer them before approving the definition." items={spec.questions} fields={[{ key: "question", label: "Question" }, { key: "answer", label: "Answer" }]} create={() => ({ id: recordId("question"), question: "", answer: "" })} onChange={(questions) => patch({ questions })}
                focusTarget={questionTarget && canEdit && !locked && !draft.pending ? { id: questionTarget, field: "answer" } : undefined} onFocused={questionFocused} />
            </div>}
          </fieldset>
          {tab === "repository" && <RepositoryAnalysisPanel projectId={projectId} specification={spec} dirty={dirty || locked || !!draft.pending} onApplied={value => {
            savedSpec.current = value; setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision }); setDirty(false); setError(""); setTab("specification");
            setNotice(`Repository analysis saved as definition ${value.revision}. Review the draft and resolve definition questions before approving.`); notifyProjectContextChanged(scope, projectId);
          }} />}
          {tab === "history" && <section><h2>Saved definition snapshots</h2>{!history.length && <p>No structured revisions yet. Existing brief text is preserved when you save your first definition.</p>}<ol className="definition-history">{history.slice().reverse().map((r) => <li key={r.revision}><strong>Revision {r.revision} · {r.status}</strong><p>{r.purpose}</p><small>Brief {r.briefRevision} · {r.requirements.length} {r.requirements.length === 1 ? "requirement" : "requirements"} · {new Date(r.updatedAt).toLocaleString()}</small><details><summary>Read this snapshot</summary><pre>{JSON.stringify(r, null, 2)}</pre></details></li>)}</ol></section>}
        </div>

      </div>
      </TabsSubtlePanel>)}
      <footer className="definition-footer"><div className="journey-actions">
        {dirty || spec.status === "outdated" || !spec.revision
          ? <Button type="button" loading={pending === "save"} disabled={!canEdit || locked || !!draft.pending || missingJourneyLinks || (!dirty && spec.status !== "outdated")} onClick={() => void save()}>Save definition</Button>
          : spec.status === "approved"
            ? <Button type="button" disabled={locked} onClick={() => setLens("design")}>Continue to Design</Button>
            : <Button type="button" loading={pending === "approve"} disabled={!canEdit || locked || !!draft.pending} onClick={() => void approve()}>Approve definition</Button>}
      </div><p role="status" className="journey-muted">{notice || `${spec.requirements.length} ${spec.requirements.length === 1 ? "requirement" : "requirements"} · ${openQuestionCount} definition ${openQuestionCount === 1 ? "question" : "questions"}`}</p></footer>
    </>}
    <ContextStorage projectId={projectId} />
  </section>;
}
