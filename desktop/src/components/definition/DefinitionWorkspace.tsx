import { useEffect, useRef, useState } from "react";
import { definitionApi, type ProjectSpecification, type DefinitionAssistance, type Requirement } from "../../api/definition";
import { errorText } from "../../api/product";
import { useCockpit } from "../../store/store";
import { JourneyHeader } from "../journey/shared";
import "./definition.css";

const recordId = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 12)}`;
type Tab = "interview" | "specification" | "history";
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
        <button className="luci-button" type="button" onClick={() => onChange(items.filter((old) => old.id !== item.id))}>Remove {title.toLowerCase().replace(/s$/, "")} {index + 1}</button>
      </fieldset>)}
      <button className="luci-button" type="button" onClick={() => onChange([...items, create()])}>Add {title.toLowerCase().replace(/s$/, "")}</button>
    </div>
  </details>;
}

export function DefinitionWorkspace({ projectId }: { projectId: string }) {
  const [spec, setSpec] = useState<ProjectSpecification | null>(null);
  const [base, setBase] = useState<{ revision: number; briefRevision: number } | null>(null);
  const [tab, setTab] = useState<Tab>("interview");
  const [step, setStep] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [history, setHistory] = useState<ProjectSpecification[]>([]);
  const [message, setMessage] = useState("");
  const [job, setJob] = useState<DefinitionAssistance | null>(null);
  const [assistError, setAssistError] = useState("");
  const alive = useRef(true);
  const submitting = useRef(false);
  const setLens = useCockpit((s) => s.setLens);
  const load = async () => {
    const value = await definitionApi.get(projectId);
    if (!alive.current) return;
    setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision });
    setDirty(false); setError("");
    if (value.purpose || value.revision) setTab("specification");
  };
  useEffect(() => {
    alive.current = true;
    void load().catch((cause) => { if (alive.current) setError(errorText(cause)); });
    return () => { alive.current = false; };
  }, [projectId]);
  useEffect(() => {
    if (!job || (job.status !== "queued" && job.status !== "running")) return;
    let stopped = false;
    const timer = setInterval(() => {
      void definitionApi.assistance(projectId, job.id).then((value) => { if (!stopped) setJob(value); })
        .catch((cause) => { if (!stopped) setAssistError(errorText(cause)); });
    }, 2500);
    return () => { stopped = true; clearInterval(timer); };
  }, [projectId, job?.id, job?.status]);
  const patch = (value: Partial<ProjectSpecification>) => {
    if (spec) setSpec({ ...spec, ...value, status: "draft", digest: "" });
    setDirty(true); setNotice("");
  };
  async function save() {
    if (!spec || !base || submitting.current) return;
    submitting.current = true; setBusy(true); setError(""); setNotice("");
    try {
      const saved = await definitionApi.save(projectId, spec, base.revision, base.briefRevision);
      if (alive.current) { setSpec(saved); setBase({ revision: saved.revision, briefRevision: saved.briefRevision }); setDirty(false); setNotice(`Definition ${saved.revision} saved. The full specification is available to your agent.`); }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current) setBusy(false); }
  }
  async function approve() {
    if (!spec || dirty || submitting.current) return;
    submitting.current = true; setBusy(true); setError("");
    try {
      const value = await definitionApi.approve(projectId, spec.revision, spec.briefRevision);
      if (alive.current) { setSpec(value); setBase({ revision: value.revision, briefRevision: value.briefRevision }); setNotice(`Definition ${value.revision} approved for design and implementation.`); }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current) setBusy(false); }
  }
  async function ask() {
    if (!spec || !message.trim() || dirty || busy) return;
    setBusy(true); setAssistError("");
    try { const value = await definitionApi.assist(projectId, spec.revision, message.trim()); if (alive.current) { setJob(value); setMessage(""); } }
    catch (cause) { if (alive.current) setAssistError(errorText(cause)); }
    finally { if (alive.current) setBusy(false); }
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
    {error && <div className="journey-error" role="alert">{error} <button className="luci-button" disabled={busy} onClick={() => void load().catch((e) => setError(errorText(e)))}>Reload saved definition</button>{dirty && <p>Your edited draft is still here. Reloading replaces it with the saved definition.</p>}</div>}
    {!spec && !error && <p role="status">Loading your project definition…</p>}
    {spec && <>
      <nav className="definition-tabs" aria-label="Definition views">
        <button aria-current={tab === "interview" ? "page" : undefined} onClick={() => selectTab("interview")}>Guided interview</button>
        <button aria-current={tab === "specification" ? "page" : undefined} onClick={() => selectTab("specification")}>Specification</button>
        <button aria-current={tab === "history" ? "page" : undefined} onClick={() => selectTab("history")}>Revision history</button>
      </nav>
      {spec.status === "outdated" && <div className="journey-error" role="status">The brief changed after this definition was saved. Reconcile the current purpose and requirements before approving again.</div>}
      <div className="definition-layout">
        <div className="definition-content">
          <fieldset disabled={busy} className="definition-fields">
            {tab === "interview" && <div className="definition-interview">
              <label className="luci-field">Starting point<select value={spec.startingPoint} onChange={(e) => patch({ startingPoint: e.target.value as "new" | "existing" })}><option value="new">A new idea</option><option value="existing">An existing application</option></select></label>
              {spec.startingPoint === "existing" && <p className="journey-muted">Use the assistant to inspect the project definition and propose gaps. Repository source analysis runs separately and must be accepted as a draft.</p>}
              <p className="journey-muted">Question {step + 1} of {questions.length}</p>
              <h2>{questions[step].label}</h2><p>{questions[step].help}</p>
              <label className="luci-field"><span className="sr-only">{questions[step].label}</span><textarea rows={7} value={interviewValue} placeholder={questions[step].placeholder} onChange={(e) => interviewChange(e.target.value)} /></label>
              <div className="journey-actions"><button type="button" className="luci-button" disabled={step === 0} onClick={() => setStep(step - 1)}>Back</button>
                <button type="button" className="luci-button luci-primary" onClick={() => step < questions.length - 1 ? setStep(step + 1) : setTab("specification")}>{step < questions.length - 1 ? "Next question" : "Review specification"}</button></div>
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
                  <button type="button" className="luci-button" onClick={() => removeRequirement(r.id)}>Remove requirement {index + 1}</button>
                </fieldset>)}
                <button type="button" className="luci-button" onClick={addRequirement}>Add requirement</button>
              </section>
              <details className="definition-group"><summary><span>User journeys</span><span className="journey-muted">{spec.journeys.length}</span></summary><div className="definition-group-body">
                {spec.journeys.map((j, index) => <fieldset className="definition-record" key={j.id}><legend>Journey {index + 1}</legend>
                  <label className="luci-field">Journey name<input value={j.name} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, name: e.target.value } : old) })} /></label>
                  <label className="luci-field">Person or role<input value={j.actor} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, actor: e.target.value } : old) })} /></label>
                  <label className="luci-field">Steps and recovery<textarea value={j.steps} rows={3} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, steps: e.target.value } : old) })} /></label>
                  <fieldset className="definition-links"><legend>Related requirements</legend>{spec.requirements.map((r) => <label key={r.id}><input type="checkbox" checked={j.requirementIds.includes(r.id)} onChange={(e) => patch({ journeys: spec.journeys.map((old) => old.id === j.id ? { ...old, requirementIds: e.target.checked ? [...old.requirementIds, r.id] : old.requirementIds.filter((id) => id !== r.id) } : old) })} />{r.id} · {r.title}</label>)}</fieldset>
                  <button type="button" className="luci-button" onClick={() => patch({ journeys: spec.journeys.filter((old) => old.id !== j.id) })}>Remove journey {index + 1}</button>
                </fieldset>)}<button type="button" className="luci-button" onClick={() => patch({ journeys: [...spec.journeys, { id: recordId("journey"), name: "", actor: "", steps: "", requirementIds: [] }] })}>Add user journey</button>
              </div></details>
              <RecordGroup title="Access roles" help="Describe what each person can do and what they must not access." items={spec.roles} fields={[{ key: "name", label: "Role name" }, { key: "canDo", label: "Can do" }, { key: "cannotDo", label: "Cannot do or access" }]} create={() => ({ id: recordId("role"), name: "", canDo: "", cannotDo: "" })} onChange={(roles) => patch({ roles })} />
              <RecordGroup title="Data entities" help="Name the data, fields, privacy and retention expectations." items={spec.entities} fields={[{ key: "name", label: "Entity name" }, { key: "fields", label: "Fields and relationships" }, { key: "privacy", label: "Privacy and retention" }]} create={() => ({ id: recordId("entity"), name: "", fields: "", privacy: "" })} onChange={(entities) => patch({ entities })} />
              <RecordGroup title="Integrations" help="Record external systems and their cost, credential or data boundaries." items={spec.integrations} fields={[{ key: "name", label: "Integration name" }, { key: "purpose", label: "Purpose" }, { key: "constraint", label: "Boundaries and requirements" }]} create={() => ({ id: recordId("integration"), name: "", purpose: "", constraint: "" })} onChange={(integrations) => patch({ integrations })} />
              <RecordGroup title="Glossary terms" help="Agree on the words people and agents should use." items={spec.glossary} fields={[{ key: "term", label: "Term" }, { key: "meaning", label: "Meaning in this project" }]} create={() => ({ id: recordId("term"), term: "", meaning: "" })} onChange={(glossary) => patch({ glossary })} />
              <RecordGroup title="Decisions" help="Record consequential choices and the alternatives considered." items={spec.decisions} fields={[{ key: "title", label: "Decision title" }, { key: "decision", label: "Decision" }, { key: "reason", label: "Why" }, { key: "alternatives", label: "Alternatives considered" }]} create={() => ({ id: recordId("decision"), title: "", decision: "", reason: "", alternatives: "" })} onChange={(decisions) => patch({ decisions })} />
              <RecordGroup title="Open questions" help="Capture the gaps. Answer them before approving the definition." items={spec.questions} fields={[{ key: "question", label: "Question" }, { key: "answer", label: "Answer" }]} create={() => ({ id: recordId("question"), question: "", answer: "" })} onChange={(questions) => patch({ questions })} />
            </div>}
          </fieldset>
          {tab === "history" && <section><h2>Saved definition snapshots</h2>{!history.length && <p>No structured revisions yet. Existing brief text is preserved when you save your first definition.</p>}<ol className="definition-history">{history.slice().reverse().map((r) => <li key={r.revision}><strong>Revision {r.revision} · {r.status}</strong><p>{r.purpose}</p><small>Brief {r.briefRevision} · {r.requirements.length} requirements · {new Date(r.updatedAt).toLocaleString()}</small><details><summary>Read this snapshot</summary><pre>{JSON.stringify(r, null, 2)}</pre></details></li>)}</ol></section>}
        </div>
        <aside className="definition-assistant" aria-label="Definition assistant">
          <h2>Work through it with Luci</h2><p>Ask the connected Codex agent to find gaps, explore an idea or propose a fuller definition. Suggestions remain drafts until you accept them.</p>
          <label className="luci-field">What would you like to explore?<textarea rows={4} value={message} disabled={busy || job?.status === "running" || job?.status === "queued"} onChange={(e) => setMessage(e.target.value)} placeholder="Help me make the first version complete. Ask the next useful question." /></label>
          <button className="luci-button luci-primary" onClick={() => void ask()} disabled={dirty || busy || !message.trim() || job?.status === "running" || job?.status === "queued"}>Ask Luci</button>
          {dirty && <p className="journey-muted">Save your draft first so the assistant works from the same definition.</p>}
          {assistError && <p className="journey-error" role="alert">{assistError}</p>}
          {job && <div className="definition-answer"><p role="status">{job.message || job.status}</p>
            {(job.status === "queued" || job.status === "running") && <button className="luci-button" onClick={() => void definitionApi.cancelAssistance(projectId, job.id).then(setJob).catch((e) => setAssistError(errorText(e)))}>Cancel assistance</button>}
            {job.result && <><h3>{job.result.question}</h3><p>{job.result.rationale}</p>{job.result.options.map((option) => <button className="luci-button definition-option" key={option} onClick={() => setMessage(option)}>{option}</button>)}
              {job.result.proposal && <details><summary>Review proposed definition</summary><p>{job.result.proposal.purpose}</p><ul>{job.result.proposal.requirements.map((r) => <li key={r.id}>{r.title}<br /><small>{r.acceptance}</small></li>)}</ul><p>This replaces the edited definition with a proposed draft. It does not approve or save it.</p><button className="luci-button" onClick={() => { const proposal = job.result?.proposal; if (proposal) { setSpec({ ...proposal, projectId, version: 1, revision: spec.revision, briefRevision: spec.briefRevision, status: "draft", digest: "" }); setDirty(true); setTab("specification"); setNotice("Proposal accepted as an editable draft. Review it and save when ready."); } }}>Use this proposal as my draft</button></details>}
            </>}
          </div>}
          <p className="journey-muted">The assistant requires your own connected Codex account and a ready Docker runtime. Your definition is sent to that provider.</p>
        </aside>
      </div>
      <footer className="definition-footer"><div className="journey-actions"><button className="luci-button luci-primary" disabled={busy || (!dirty && spec.status !== "outdated")} onClick={() => void save()}>{busy ? "Working…" : "Save definition"}</button><button className="luci-button" disabled={busy || dirty || !spec.revision || spec.status === "approved"} onClick={() => void approve()}>Approve definition</button>{spec.status === "approved" && !dirty && <button className="luci-button" onClick={() => setLens("design")}>Continue to Design</button>}</div><p role="status" className="journey-muted">{notice || `${spec.requirements.length} requirements · ${spec.questions.filter((q) => !q.answer.trim()).length} open questions`}</p></footer>
    </>}
  </section>;
}
