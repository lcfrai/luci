import { useCallback, useEffect, useRef, useState } from "react";
import { useShallow } from "zustand/react/shallow";
import { definitionApi, type DefinitionAssistance, type ProjectSpecification, type RepositoryAnalysisRequest } from "../../api/definition";
import { errorText, productApi } from "../../api/product";
import { useCockpit } from "../../store/store";

type Props = { projectId: string; specification: ProjectSpecification; dirty: boolean; onApplied(value: ProjectSpecification): void };
const running = (job: DefinitionAssistance) => job.status === "queued" || job.status === "running";
const providers = { codex: "Codex", "claude-code": "Claude Code", opencode: "OpenCode" };
const supportedProvider = (value: string) => Object.prototype.hasOwnProperty.call(providers, value);
export function RepositoryAnalysisPanel(props: Props) { return <ProjectRepositoryAnalysis key={props.projectId} {...props} />; }
function ProjectRepositoryAnalysis({ projectId, specification, dirty, onApplied }: Props) {
  const defaults = useCockpit(useShallow(state => ({
    harness: state.projectDefaults[projectId]?.harness ?? state.defaults.harness,
    model: state.projectDefaults[projectId]?.model ?? state.defaults.model,
  })));
  const [harness, setHarness] = useState<RepositoryAnalysisRequest["harness"] | "">(() => supportedProvider(defaults.harness) ? defaults.harness as RepositoryAnalysisRequest["harness"] : "");
  const [model, setModel] = useState(() => supportedProvider(defaults.harness) && defaults.model !== "default" ? defaults.model : "");
  const [localModel, setLocalModel] = useState("");
  const [connectionError, setConnectionError] = useState("");
  const [authMode, setAuthMode] = useState<"chatgpt" | "api">("chatgpt");
  const [sourceMode, setSourceMode] = useState<"worktree" | "commit">("worktree");
  const [message, setMessage] = useState("");
  const [acknowledged, setAcknowledged] = useState(false), [reviewed, setReviewed] = useState(false);
  const [jobs, setJobs] = useState<DefinitionAssistance[]>([]), [job, setJob] = useState<DefinitionAssistance | null>(null);
  const [pending, setPending] = useState(""), [error, setError] = useState("");
  const alive = useRef(true), submitting = useRef(false), generation = useRef(0);
  const accept = useCallback((value: DefinitionAssistance) => {
    if (value.projectId !== projectId || value.kind !== "repository") throw new Error("This analysis does not belong to the selected project.");
    setJob(value);
    setJobs(current => [value, ...current.filter(item => item.id !== value.id)].sort((a, b) => b.createdAt.localeCompare(a.createdAt)));
  }, [projectId]);
  const load = useCallback(async (selectLatest = false) => {
    const ticket = generation.current;
    const result = await definitionApi.listAssistance(projectId);
    if (!alive.current || ticket !== generation.current) return;
    if (result.jobs.some(item => item.projectId !== projectId)) throw new Error("Luci could not verify this project's saved analysis history.");
    setJobs(result.jobs);
    if (selectLatest) {
      const latest = result.jobs.filter(item => item.kind === "repository").sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0];
      if (latest) {
        const value = await definitionApi.assistance(projectId, latest.id);
        if (alive.current && ticket === generation.current) accept(value);
      }
    }
  }, [projectId, accept]);
  useEffect(() => {
    alive.current = true; generation.current++;
    void load(true).catch(cause => { if (alive.current) setError(errorText(cause)); });
    void productApi.agents().then(value => {
      if (!alive.current) return;
      const configured = value.providers.find(item => item.id === "local" && item.configured && item.model?.trim());
      setLocalModel(configured ? `luci-local/${configured.model!.trim()}` : "");
    }).catch(cause => { if (alive.current) setConnectionError(errorText(cause)); });
    return () => { alive.current = false; generation.current++; };
  }, [load]);
  const activeJob = jobs.find(running), activeAnalysis = job && running(job);
  useEffect(() => {
    if (!activeJob && !activeAnalysis) return;
    let stopped = false, inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      void (activeAnalysis ? definitionApi.assistance(projectId, job.id).then(value => { if (!stopped) accept(value); }) : load())
        .catch(cause => { if (!stopped) setError(errorText(cause)); }).finally(() => { inFlight = false; });
    }, 2500);
    return () => { stopped = true; window.clearInterval(timer); };
  }, [activeJob?.id, activeJob?.status, activeAnalysis, projectId, job?.id, accept, load]);
  const locked = !!pending || !!activeJob || !!activeAnalysis || dirty;
  const selectedModel = harness === "opencode" ? localModel : model.trim();
  const validModel = !!selectedModel && selectedModel !== "default";
  async function act(action: "start" | "apply" | "cancel" | "select" | "reload", id = "") {
    if (submitting.current) return;
    if (action === "start" && (locked || !harness || !validModel || !acknowledged)) return;
    if (action === "apply" && (locked || !reviewed || job?.status !== "succeeded" || !job.result?.proposal || job.appliedRevision)) return;
    submitting.current = true; setPending(action); setError("");
    const ticket = ++generation.current;
    try {
      if (action === "start" && harness) {
        const value = await definitionApi.analyseRepository(projectId, {
          expectedRevision: specification.revision, expectedBriefRevision: specification.briefRevision,
          sourceMode, harness, model: selectedModel, authMode: harness === "codex" ? authMode : harness === "claude-code" ? "api" : "configured",
          acknowledgeSourceRead: true, ...(message.trim() ? { message: message.trim() } : {}),
        });
        if (alive.current && generation.current === ticket) { accept(value); setAcknowledged(false); setReviewed(false); }
      } else if (action === "apply" && job) {
        const value = await definitionApi.applyAnalysis(projectId, job.id, {
          expectedRevision: specification.revision, expectedBriefRevision: specification.briefRevision, acknowledgePartial: true,
        });
        if (alive.current && generation.current === ticket) {
          if (value.projectId !== projectId || value.status !== "draft" || value.revision <= specification.revision) throw new Error("The saved analysis draft could not be verified. Reload the definition.");
          setJob({ ...job, appliedRevision: value.revision }); setReviewed(false); onApplied(value);
        }
      } else if (action === "cancel" && job) {
        const value = await definitionApi.cancelAssistance(projectId, job.id);
        if (alive.current && generation.current === ticket) accept(value);
      } else if (action === "select") {
        const value = await definitionApi.assistance(projectId, id);
        if (alive.current && generation.current === ticket) { accept(value); setReviewed(false); }
      } else if (action === "reload") await load(!job);
    } catch (cause) { if (alive.current && generation.current === ticket) setError(errorText(cause)); }
    finally { submitting.current = false; if (alive.current && generation.current === ticket) setPending(""); }
  }
  const source = job?.source, proposal = job?.result?.proposal;
  function changed() { setAcknowledged(false); }
  return <section className="repository-analysis" aria-label="Repository analysis">
    <h2>Understand the existing application</h2>
    <p>Analyse a bounded selection of this project's source with your connected provider. Luci records the files it used and separates observed behaviour, inferences and gaps. Review the proposed definition before saving it as a draft.</p>
    {error && <p className="journey-error" role="alert">{error}</p>}
    <div className="journey-actions"><button type="button" className="luci-button" disabled={!!pending} onClick={() => void act("reload")}>Refresh saved analyses</button></div>
    {jobs.some(item => item.kind === "repository") && <label className="luci-field">Saved analysis<select value={job?.id || ""} disabled={!!pending} onChange={event => void act("select", event.target.value)}><option value="" disabled>Choose an analysis</option>{jobs.filter(item => item.kind === "repository").map(item => <option key={item.id} value={item.id}>{new Date(item.createdAt).toLocaleString()} · {item.status}</option>)}</select></label>}
    <form onSubmit={event => { event.preventDefault(); void act("start"); }}>
      <fieldset disabled={locked} className="definition-fields">
        <div className="setup-field-grid">
          <label className="luci-field">Source to analyse<select value={sourceMode} onChange={event => { setSourceMode(event.target.value as typeof sourceMode); changed(); }}><option value="worktree">Current working files</option><option value="commit">Latest committed source</option></select></label>
          <label className="luci-field">Analysis provider<select required value={harness} onChange={event => { setHarness(event.target.value as typeof harness); setModel(""); changed(); }}><option value="" disabled>Choose your connected provider</option>{Object.entries(providers).map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>
          <label className="luci-field">Analysis model<input required value={harness === "opencode" ? localModel : model} readOnly={harness === "opencode"} autoComplete="off" onChange={event => { setModel(event.target.value); changed(); }} placeholder={harness === "opencode" ? "Connect a local model in Agent connection" : "Enter a model available to your account"} /></label>
          {harness === "codex" && <label className="luci-field">Codex connection<select value={authMode} onChange={event => { setAuthMode(event.target.value as typeof authMode); changed(); }}><option value="chatgpt">Codex subscription</option><option value="api">Connected API account</option></select></label>}
        </div>
        {harness === "opencode" && <p className="journey-muted">OpenCode uses the model saved in Agent connection. {localModel ? "This analysis will use that exact connection." : "Save a local provider and model there before starting."}</p>}
        {harness === "opencode" && connectionError && <p role="alert">{connectionError}</p>}
        <label className="luci-field">What should the analysis focus on?<textarea rows={3} value={message} onChange={event => { setMessage(event.target.value); changed(); }} placeholder="Optional: the intended changes, questions or parts you want understood." /></label>
        <p className="journey-muted">Uses the selected provider and its configured account. Source sent for analysis excludes ignored, private, generated and binary files. The selection is partial and can miss behaviours elsewhere in the repository. Model usage follows your account's charges or subscription limits.</p>
        <label className="data-check"><input type="checkbox" checked={acknowledged} onChange={event => setAcknowledged(event.target.checked)} />Send the selected source and definition to this provider for analysis.</label>
        <button className="luci-button luci-primary" disabled={locked || !harness || !validModel || !acknowledged}>{pending === "start" ? "Starting analysis…" : "Analyse repository"}</button>
      </fieldset>
    </form>
    {dirty && <p>Save your definition changes before analysing or applying a proposal.</p>}
    {activeJob && !activeAnalysis && <p role="status">Another definition request is running. Its saved progress will remain available.</p>}
    {job && <section className="definition-answer" aria-label="Analysis result">
      <p role="status">{job.message || job.status}</p>
      {activeAnalysis && <><p>You can leave this page and return to the saved run.</p><button type="button" className="luci-button" disabled={!!pending} onClick={() => void act("cancel")}>Cancel repository analysis</button></>}
      {source && <><h3>Source used</h3><p>{source.fileCount} files · {Math.ceil(source.includedBytes / 1024)} KiB · {source.skippedFiles} files omitted · {source.mode === "commit" ? `commit ${source.commit?.slice(0, 12) || "unavailable"}` : "working files"}</p><p className="journey-muted">{source.limitation}</p><details><summary>Included files and source fingerprint</summary><code>{source.digest}</code><ul>{source.files.map(file => <li key={file.path}><code>{file.path}</code> · {file.bytes} bytes</li>)}</ul></details></>}
      {job.result && <><h3>Findings</h3><p>{job.result.rationale}</p><ul>{job.result.findings?.map((finding, index) => <li key={index}><strong>{finding.kind === "observed" ? "Observed in source" : finding.kind === "inference" ? "Inference" : "Gap"}</strong><p>{finding.summary}</p>{!!finding.paths.length && <small>{finding.paths.join(", ")}</small>}</li>)}</ul></>}
      {proposal && <><h3>Proposed definition</h3><p>{proposal.purpose}</p><ul>{proposal.requirements.map(requirement => <li key={requirement.id}><strong>{requirement.id} · {requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul><details><summary>Read the complete proposed definition</summary><pre>{JSON.stringify(proposal, null, 2)}</pre></details>
        <p>Applying replaces the saved definition with this draft. Approval stays a separate step. Luci checks that both the source and saved definition still match this analysis.</p>
        {job.appliedRevision ? <p role="status">Applied as draft revision {job.appliedRevision}.</p> : <><label className="data-check"><input type="checkbox" checked={reviewed} disabled={locked || job.status !== "succeeded"} onChange={event => setReviewed(event.target.checked)} />I reviewed the proposed definition and its partial source coverage.</label><button type="button" className="luci-button" disabled={locked || !reviewed || job.status !== "succeeded"} onClick={() => void act("apply")}>{pending === "apply" ? "Saving reviewed draft…" : "Apply as definition draft"}</button></>}
      </>}
    </section>}
  </section>;
}
