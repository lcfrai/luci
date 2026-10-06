import { useCallback, useEffect, useRef, useState } from "react";
import { useShallow } from "zustand/react/shallow";
import { definitionApi, type DefinitionAssistance, type ProjectSpecification, type RepositoryAnalysisRequest } from "../../api/definition";
import { errorText, productApi } from "../../api/product";
import { ApiError } from "../../api/client";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { useCockpit } from "../../store/store";
import { useAuth } from "../auth/AuthProvider";
import { Button } from "../fluid/components/button";
import { AgentConnectionSelect } from "../setup/AgentConnectionSelect";
import { useAgentConnections, selectedAgentConnection, configuredAgentModel, agentConnectionIssue } from "../../hooks/useAgentConnections";
import {
  finishRepositoryAnalysisRequest, isRepositoryAnalysisRecoveryKey, readRepositoryAnalysisRecovery,
  retainRepositoryAnalysisRequest, saveRepositoryAnalysisDraft, type RepositoryAnalysisDraft,
  type PendingRepositoryAnalysisRequest,
} from "./repositoryAnalysisRecovery";

type Props = { projectId: string; specification: ProjectSpecification; dirty: boolean; onApplied(value: ProjectSpecification): void };
const running = (job: DefinitionAssistance) => job.status === "reserved" || job.status === "queued" || job.status === "running";
const providers = { codex: "Codex", "claude-code": "Claude Code", opencode: "OpenCode" };
const supportedProvider = (value: string) => Object.prototype.hasOwnProperty.call(providers, value);
export function RepositoryAnalysisPanel(props: Props) {
  const scope = useProjectDraftScope(props.projectId), auth = useAuth();
  return <ProjectRepositoryAnalysis key={`${scope ?? "unavailable"}:${auth?.profile.user.id ?? ""}:${props.projectId}`} {...props} scope={scope} />;
}
function ProjectRepositoryAnalysis({ projectId, specification, dirty, onApplied, scope }: Props & { scope: string | null }) {
  const access = useProjectAccess(projectId);
  const { canEdit, canExecute, canAssist } = access;
  const canRead = access.canRead && !access.error;
  const permissions = useRef({ canRead, canAssist }); permissions.current = { canRead, canAssist };
  const defaults = useCockpit(useShallow(state => ({
    harness: state.projectDefaults[projectId]?.harness ?? state.defaults.harness,
    model: state.projectDefaults[projectId]?.model ?? state.defaults.model,
    agentConnectionId: state.projectDefaults[projectId]?.agentConnectionId ?? state.defaults.agentConnectionId,
  })));
  const [recovered] = useState(() => scope ? readRepositoryAnalysisRecovery(scope) : { draft: null, pending: null, error: "Account and workspace identity are required to save analysis recovery." });
  const [draft, setDraft] = useState<RepositoryAnalysisDraft>(() => recovered.draft ?? {
    harness: supportedProvider(defaults.harness) ? defaults.harness as RepositoryAnalysisRequest["harness"] : "",
    model: supportedProvider(defaults.harness) && defaults.model !== "default" ? defaults.model : "",
    sourceMode: "worktree", message: "", replyToAnalysisId: "", agentConnectionId: defaults.agentConnectionId,
  });
  const { harness, model, sourceMode, message, agentConnectionId } = draft;
  const draftRef = useRef(draft); draftRef.current = draft;
  const savedDraft = useRef(recovered.draft);
  const draftQueue = useRef(Promise.resolve());
  const storageFailure = useRef(recovered.error);
  const [storageError, setStorageError] = useState(recovered.error);
  const [savedRequest, setSavedRequest] = useState(recovered.pending);
  const [missingRequest, setMissingRequest] = useState(false);
  const [retryConfirmed, setRetryConfirmed] = useState(false);
  const [localModel, setLocalModel] = useState("");
  const [connectionError, setConnectionError] = useState("");
  const { catalogue, error: catalogueError } = useAgentConnections(undefined, canRead);
  const connection = selectedAgentConnection(catalogue, agentConnectionId);
  const selectedConnectionID = agentConnectionId ?? catalogue?.activeConnectionId ?? "";
  const savedModel = configuredAgentModel(connection, harness);
  const savedAPI = connection && connection.kind !== "codex-subscription";
  const connectionIssue = agentConnectionIssue(connection, harness, selectedConnectionID) ||
    (selectedConnectionID && (!Number.isSafeInteger(connection?.revision) || Number(connection?.revision) <= 0)
      ? "This saved connection's revision is unavailable. Refresh Agent connections and review its settings before sending source." : "");
  const [acknowledged, setAcknowledged] = useState(false), [reviewed, setReviewed] = useState(false);
  const [jobs, setJobs] = useState<DefinitionAssistance[]>([]), [job, setJob] = useState<DefinitionAssistance | null>(null);
  const [pending, setPending] = useState(""), [error, setError] = useState("");
  const alive = useRef(true), submitting = useRef(false), generation = useRef(0);
  const answerInput = useRef<HTMLTextAreaElement>(null);
  function editDraft(patch: Partial<RepositoryAnalysisDraft>) {
    const next = { ...draftRef.current, ...patch };
    draftRef.current = next; setDraft(next); setAcknowledged(false);
    draftQueue.current = draftQueue.current.then(async () => {
      if (!scope) throw new Error("Account and workspace identity are required to save this analysis draft.");
      savedDraft.current = await saveRepositoryAnalysisDraft(scope, savedDraft.current, next);
      storageFailure.current = "";
      if (alive.current) setStorageError("");
    }).catch(cause => {
      storageFailure.current = errorText(cause);
      if (alive.current) setStorageError(storageFailure.current);
    });
  }
  function discardDraft() {
    const original = draftRef.current;
    draftQueue.current = draftQueue.current.then(async () => {
      if (!scope) throw new Error("Account and workspace identity are required to clear this saved draft.");
      savedDraft.current = await saveRepositoryAnalysisDraft(scope, savedDraft.current, null);
      storageFailure.current = "";
      if (alive.current) {
        setStorageError("");
        if (draftRef.current === original) {
          const next = { ...original, message: "", replyToAnalysisId: "" };
          draftRef.current = next; setDraft(next); setAcknowledged(false);
        }
      }
    }).catch(cause => {
      storageFailure.current = errorText(cause);
      if (alive.current) setStorageError(storageFailure.current);
    });
  }
  useEffect(() => {
    if (catalogue && draftRef.current.agentConnectionId === undefined) editDraft({ agentConnectionId: catalogue.activeConnectionId || "" });
  }, [catalogue, agentConnectionId]);
  useEffect(() => {
    if (!scope) return;
    const changed = (event: StorageEvent) => {
      if (!isRepositoryAnalysisRecoveryKey(scope, event.key)) return;
      const current = readRepositoryAnalysisRecovery(scope);
      setSavedRequest(current.pending);
      setMissingRequest(false); setRetryConfirmed(false);
      if (current.error || JSON.stringify(current.draft) !== JSON.stringify(savedDraft.current)) {
        storageFailure.current = current.error || "Another tab changed this analysis draft. Your local edits are still here; reopen this workspace before sending.";
        setStorageError(storageFailure.current);
      }
    };
    window.addEventListener("storage", changed);
    return () => window.removeEventListener("storage", changed);
  }, [scope]);
  const accept = useCallback((value: DefinitionAssistance) => {
    if (value.projectId !== projectId || value.kind !== "repository") throw new Error("This analysis does not belong to the selected project.");
    if (!alive.current || !permissions.current.canRead) return;
    setJob(value);
    setJobs(current => [value, ...current.filter(item => item.id !== value.id)].sort((a, b) => b.createdAt.localeCompare(a.createdAt)));
  }, [projectId]);
  const load = useCallback(async (selectLatest = false) => {
    if (!permissions.current.canRead) return;
    const ticket = generation.current;
    const result = await definitionApi.listAssistance(projectId);
    if (!alive.current || ticket !== generation.current || !permissions.current.canRead) return;
    if (result.jobs.some(item => item.projectId !== projectId)) throw new Error("Luci could not verify this project's saved analysis history.");
    setJobs(result.jobs);
    if (selectLatest) {
      const latest = result.jobs.find(item => item.id === draftRef.current.replyToAnalysisId) ?? result.jobs.filter(item => item.kind === "repository").sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0];
      if (latest) {
        const value = await definitionApi.assistance(projectId, latest.id);
        if (alive.current && ticket === generation.current) accept(value);
      }
    }
  }, [projectId, accept]);
  useEffect(() => {
    alive.current = true; generation.current++;
    void load(true).catch(cause => { if (alive.current) setError(errorText(cause)); });
    if (permissions.current.canRead) void productApi.agents().then(value => {
      if (!alive.current) return;
      const configured = value.providers.find(item => item.id === "local" && item.configured && item.model?.trim());
      setLocalModel(configured ? `luci-local/${configured.model!.trim()}` : "");
    }).catch(cause => { if (alive.current) setConnectionError(errorText(cause)); });
    return () => { alive.current = false; generation.current++; submitting.current = false; };
  }, [load, canRead]);
  useEffect(() => {
    if (!canRead) { generation.current++; submitting.current = false; setPending(""); setJob(null); setJobs([]); setReviewed(false); }
  }, [canRead]);
  const activeJob = jobs.find(running), activeAnalysis = job && running(job);
  useEffect(() => {
    if (!canRead || (!activeJob && !activeAnalysis)) return;
    let stopped = false, inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      const ticket = generation.current;
      void (savedRequest ? definitionApi.repositoryRequest(projectId, savedRequest.idempotencyKey).then(value => { if (!stopped) return receipt(value, savedRequest, ticket); })
        : activeAnalysis ? definitionApi.assistance(projectId, job.id).then(value => { if (!stopped) accept(value); }) : load())
        .catch(cause => { if (!stopped) setError(errorText(cause)); }).finally(() => { inFlight = false; });
    }, 2500);
    return () => { stopped = true; window.clearInterval(timer); };
  }, [canRead, activeJob?.id, activeJob?.status, activeAnalysis, projectId, job?.id, savedRequest, accept, load]);
  const locked = !!pending || !!activeJob || !!activeAnalysis || dirty || !!savedRequest;
  const selectedModel = savedModel || (savedAPI ? "" : harness === "opencode" ? localModel : model.trim());
  useEffect(() => { setAcknowledged(false); }, [selectedModel, connection?.id, connection?.revision]);
  const validModel = !!catalogue && !!selectedModel && selectedModel !== "default" && !connectionIssue;
  const replying = !!draft.replyToAnalysisId;
  const ownQuestion = job?.status === "succeeded" && !!job.result?.question.trim() && job.ownRequest === true;
  const validReply = !replying || (ownQuestion && job?.id === draft.replyToAnalysisId && !!message.trim());
  function answerQuestion(option?: string) {
    if (!canAssist || !ownQuestion || !job) return;
    editDraft({ replyToAnalysisId: job.id, ...(option === undefined ? {} : { message: option }) });
    answerInput.current?.focus();
  }
  async function receipt(value: DefinitionAssistance, request: PendingRepositoryAnalysisRequest, ticket: number) {
    if (!alive.current || ticket !== generation.current || !permissions.current.canRead) return;
    if (value.projectId !== projectId || value.kind !== "repository" || typeof value.id !== "string" || !value.id ||
        (!running(value) && !["succeeded", "failed", "cancelled"].includes(value.status)) || value.idempotencyKey !== request.idempotencyKey || value.ownRequest !== true ||
        (value.replyToAnalysisId ?? "") !== (request.replyToAnalysisId ?? "")) throw new Error("The original analysis receipt could not be verified. Its exact request is still saved.");
    if (!scope) throw new Error("The original workspace identity is unavailable.");
    if (!running(value)) {
      await finishRepositoryAnalysisRequest(scope, request);
      if (!alive.current || ticket !== generation.current || !permissions.current.canRead) return;
      setSavedRequest(null);
    }
    setMissingRequest(false); setRetryConfirmed(false);
    accept(value); setReviewed(false); setAcknowledged(false);
  }
  async function recover(action: "check" | "retry") {
    if (!savedRequest || !scope || submitting.current || !canRead) return;
    if (action === "retry" && (!canAssist || dirty || !missingRequest || !retryConfirmed)) return;
    const original = savedRequest;
    submitting.current = true; setPending(action); setError("");
    const ticket = ++generation.current;
    try {
      const value = action === "retry" ? await definitionApi.analyseRepository(projectId, original) : await definitionApi.repositoryRequest(projectId, original.idempotencyKey);
      await receipt(value, original, ticket);
    } catch (cause) {
      if (!alive.current || ticket !== generation.current) return;
      setMissingRequest(action === "check" && cause instanceof ApiError && cause.status === 404);
      setRetryConfirmed(false);
      setError(action === "check" && cause instanceof ApiError && cause.status === 404
        ? "No receipt was found. The original request may still arrive. Only an explicit retry of its exact saved request is available."
        : "The original request is still unconfirmed. Keep it saved and check again before starting another analysis.");
      if (action === "check") void load(true).catch(() => {});
    } finally {
      if (alive.current && ticket === generation.current) { submitting.current = false; setPending(""); }
    }
  }
  useEffect(() => {
    if (recovered.pending && canRead) void recover("check");
  }, [canRead]);
  async function act(action: "start" | "apply" | "cancel" | "select" | "reload", id = "") {
    if (submitting.current || !canRead) return;
    if (action === "cancel" && !canExecute) return;
    if (action === "start" && (!canAssist || locked || !harness || !validModel || !acknowledged || !validReply)) return;
    if (action === "apply" && (!canEdit || locked || !reviewed || job?.status !== "succeeded" || !job.result?.proposal || job.appliedRevision)) return;
    submitting.current = true; setPending(action); setError("");
    const ticket = ++generation.current;
    try {
      if (action === "start" && harness) {
        const input: PendingRepositoryAnalysisRequest = {
          expectedRevision: specification.revision, expectedBriefRevision: specification.briefRevision,
          sourceMode, harness, model: selectedModel, agentConnectionId: selectedConnectionID,
          ...(selectedConnectionID && connection ? { expectedAgentConnectionRevision: connection.revision } : {}),
          acknowledgeSourceRead: true, ...(message.trim() ? { message: message.trim() } : {}),
          ...(draft.replyToAnalysisId ? { replyToAnalysisId: draft.replyToAnalysisId } : {}),
          idempotencyKey: `analysis-${crypto.randomUUID()}`,
        };
        if (new TextEncoder().encode(input.message ?? "").length > 16000) throw new Error("Keep the answer or analysis focus within 16,000 UTF-8 bytes.");
        await draftQueue.current;
        if (storageFailure.current) throw new Error(storageFailure.current);
        if (!scope) throw new Error("Wait for your account and workspace before sending an analysis.");
        await retainRepositoryAnalysisRequest(scope, input);
        if (!alive.current || ticket !== generation.current) return;
        setSavedRequest(input); setAcknowledged(false); setMissingRequest(false);
        if (!permissions.current.canAssist) throw new Error("Analysis access changed before sending. Check the saved request before continuing.");
        const value = await definitionApi.analyseRepository(projectId, input);
        await receipt(value, input, ticket);
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
        if (alive.current && generation.current === ticket) {
          if (savedRequest && savedRequest.idempotencyKey === value.idempotencyKey) await receipt(value, savedRequest, ticket);
          else accept(value);
        }
      } else if (action === "select") {
        const value = await definitionApi.assistance(projectId, id);
        if (alive.current && generation.current === ticket) { accept(value); setReviewed(false); }
      } else if (action === "reload") await load(!job);
    } catch (cause) { if (alive.current && generation.current === ticket) setError(errorText(cause)); }
    finally { if (alive.current && generation.current === ticket) { submitting.current = false; setPending(""); } }
  }
  const source = job?.source, proposal = job?.result?.proposal;
  if (!canRead) return <section className="repository-analysis" aria-label="Repository analysis"><h2>Understand the existing application</h2><p role="status">{access.message || "Project read access is required to inspect repository analysis. Ask a team owner for access."}</p></section>;
  return <section className="repository-analysis" aria-label="Repository analysis">
    <h2>Understand the existing application</h2>
    <p>Understand the source, answer open questions, then review a proposed definition before saving it.</p>
    {error && <p className="journey-error" role="alert">{error}</p>}
    {storageError && <p className="journey-error" role="alert">{storageError} Keep this page open; no new request will be sent until recovery can be saved.</p>}
    {savedRequest && <section className="definition-answer" aria-label="Saved analysis request">
      <h3>Original analysis request</h3>
      <p>This exact request is saved separately from your editable draft. Checking it does not contact a model.</p>
      <details><summary>Review original submitted request</summary>
        <p>{providers[savedRequest.harness]} · {savedRequest.model} · {savedRequest.sourceMode === "commit" ? "Latest committed source" : "Current working files"}</p>
        <p>Connection: {savedRequest.agentConnectionId || "Personal agent settings"}</p>
        {savedRequest.replyToAnalysisId && <p>Reply to analysis: <code>{savedRequest.replyToAnalysisId}</code></p>}
        <p>{savedRequest.message || "No additional analysis focus."}</p><code>{savedRequest.idempotencyKey}</code>
      </details>
      <Button type="button" variant="secondary" disabled={!!pending || !canRead} loading={pending === "check"} onClick={() => void recover("check")}>Check original request</Button>
      {missingRequest && <>
        <label className="data-check"><input type="checkbox" checked={retryConfirmed} disabled={!!pending || !canAssist || dirty} onChange={event => setRetryConfirmed(event.target.checked)} />Retry only this saved request with its original answer, source consent, provider and model.</label>
        <Button type="button" disabled={!!pending || !canAssist || dirty || !retryConfirmed} loading={pending === "retry"} onClick={() => void recover("retry")}>Retry original request</Button>
      </>}
    </section>}
    {job?.result?.question && <section className="repository-question" aria-label="Saved analysis question">
      <h3>A question about this application</h3><p className="repository-question-text">{job.result.question}</p>
      {!!job.result.options.length && <ul className="repository-options">{job.result.options.map((option, index) => <li key={index}><Button type="button" variant="secondary" className="definition-option" disabled={!canAssist || !ownQuestion} onClick={() => answerQuestion(option)}>Use answer: {option}</Button></li>)}</ul>}
      {ownQuestion ? <Button type="button" variant="secondary" disabled={!canAssist} onClick={() => answerQuestion()}>Answer this question</Button>
        : <p>This question belongs to another person or an older unattributed analysis. Start a new analysis with your own account to ask a follow-up.</p>}
      <p className="journey-muted">Choose an option, or select “Answer this question” to write your own response. Nothing is sent until you review and confirm it.</p>
    </section>}
    <form onSubmit={event => { event.preventDefault(); void act("start"); }}>
      <fieldset disabled={!canAssist} className="definition-fields">
        <label className="luci-field">{replying ? "Your answer" : "What should the analysis focus on?"}<textarea ref={answerInput} rows={3} value={message} onChange={event => editDraft({ message: event.target.value })} placeholder={replying ? "Answer the saved question in your own words." : "Optional: the intended changes, questions or parts you want understood."} /></label>
        {replying && <p className="journey-muted">Sending uses the original question and a new snapshot of the source you choose below.</p>}
        {replying && !validReply && <p>Choose your original saved question and write an answer before sending, or start a new analysis.</p>}
        <h3>Review source and provider</h3>
        <div className="setup-field-grid">
          <label className="luci-field">Source to analyse<select value={sourceMode} onChange={event => editDraft({ sourceMode: event.target.value as typeof sourceMode })}><option value="worktree">Current working files</option><option value="commit">Latest committed source</option></select></label>
          <label className="luci-field">Analysis provider<select required value={harness} onChange={event => editDraft({ harness: event.target.value as typeof harness, model: "" })}><option value="" disabled>Choose your connected provider</option>{Object.entries(providers).map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>
          <AgentConnectionSelect catalogue={catalogue} value={agentConnectionId} onChange={id => editDraft({ agentConnectionId: id })} label="Analysis connection" />
          <label className="luci-field">Analysis model<input required value={savedModel || (savedAPI ? "" : harness === "opencode" ? localModel : model)} readOnly={!!savedAPI || !!savedModel || harness === "opencode"} autoComplete="off" onChange={event => editDraft({ model: event.target.value })} placeholder={harness === "opencode" ? "Save a model in Agent connections" : "Enter a model available to your account"} /></label>
        </div>
        {(connectionIssue || catalogueError) && <p role="alert" className="journey-error">{connectionIssue || catalogueError}</p>}
        {harness === "opencode" && <p className="journey-muted">OpenCode uses the model saved in Agent connections. {selectedModel ? "This analysis will use that exact connection." : "Save a provider and model there before starting."}</p>}
        {harness === "opencode" && connectionError && <p role="alert">{connectionError}</p>}
        <p className="journey-muted">Only a partial source selection is sent; ignored, private, generated and binary files are excluded. Your selected account's model charges or subscription limits apply.</p>
        <label className="data-check"><input type="checkbox" checked={acknowledged} onChange={event => setAcknowledged(event.target.checked)} />Send the selected source and definition to this provider for analysis.</label>
        <Button type="submit" variant={proposal && !job?.appliedRevision ? "secondary" : "primary"} loading={pending === "start"} disabled={!canAssist || locked || !harness || !validModel || !acknowledged || !validReply || !!storageError}>{replying ? "Send answer" : "Analyse repository"}</Button>
      </fieldset>
      <details className="repository-draft-actions"><summary>Other draft actions</summary>
        {replying && <Button type="button" variant="ghost" disabled={!canAssist} onClick={() => editDraft({ replyToAnalysisId: "" })}>Start a new analysis instead</Button>}
        <Button type="button" variant="ghost" disabled={!scope || (!savedDraft.current && !message && !replying)} onClick={discardDraft}>Discard saved analysis draft</Button>
        <p className="journey-muted">Your account's draft stays in this browser. Later edits are separate from an original submitted request. Discarding this draft does not cancel or remove that request.</p>
      </details>
    </form>
    {dirty && <p>Save your definition changes before analysing or applying a proposal.</p>}
    {activeJob && !activeAnalysis && <p role="status">Another definition request is running. Its saved progress will remain available.</p>}
    {job && <section className="definition-answer" aria-label="Analysis result">
      <p role="status">{job.message || job.status}</p>
      {job.userMessage && <details><summary>{job.replyToAnalysisId ? "Submitted answer" : "Submitted analysis focus"}</summary><p>{job.userMessage}</p></details>}
      {job.replySourceChanged && <p className="journey-warning">The source changed since the earlier question. This reply used a new source snapshot; the earlier findings describe historical source.</p>}
      {activeAnalysis && <><p>You can leave this page and return to the saved run.</p><Button type="button" variant="secondary" loading={pending === "cancel"} disabled={!canExecute || !!pending} onClick={() => void act("cancel")}>Cancel repository analysis</Button></>}
      {proposal && <><h3>Proposed definition</h3><p>{proposal.purpose}</p><ul>{proposal.requirements.map(requirement => <li key={requirement.id}><strong>{requirement.id} · {requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul><details><summary>Read the complete proposed definition</summary><pre>{JSON.stringify(proposal, null, 2)}</pre></details>
        <p>Applying replaces the saved definition with this draft. Approval stays a separate step. Luci checks that both the source and saved definition still match this analysis.</p>
        {job.appliedRevision ? <p role="status">Applied as draft revision {job.appliedRevision}.</p> : <><label className="data-check"><input type="checkbox" checked={reviewed} disabled={!canEdit || locked || job.status !== "succeeded"} onChange={event => setReviewed(event.target.checked)} />I reviewed the proposed definition and its partial source coverage.</label><Button type="button" loading={pending === "apply"} disabled={!canEdit || locked || !reviewed || job.status !== "succeeded"} onClick={() => void act("apply")}>Apply as definition draft</Button></>}
      </>}
      {(source || job.result) && <details className="repository-evidence"><summary>Review findings and source evidence</summary>
        {job.result && <><h3>Findings</h3><p>{job.result.rationale}</p><ul>{job.result.findings?.map((finding, index) => <li key={index}><strong>{finding.kind === "observed" ? "Observed in source" : finding.kind === "inference" ? "Inference" : "Gap"}</strong><p>{finding.summary}</p>{!!finding.paths.length && <small>{finding.paths.join(", ")}</small>}</li>)}</ul></>}
        {source && <><h3>Source used</h3><p>{source.fileCount} files · {Math.ceil(source.includedBytes / 1024)} KiB · {source.skippedFiles} files omitted · {source.mode === "commit" ? `commit ${source.commit?.slice(0, 12) || "unavailable"}` : "working files"}</p><p className="journey-muted">{source.limitation}</p><details><summary>Included files and source fingerprint</summary><code>{source.digest}</code><ul>{source.files.map(file => <li key={file.path}><code>{file.path}</code> · {file.bytes} bytes</li>)}</ul></details></>}
      </details>}
    </section>}
    <div className="repository-history">
      {jobs.some(item => item.kind === "repository") && <label className="luci-field">Saved analysis<select value={job?.id || ""} disabled={!!pending} onChange={event => void act("select", event.target.value)}><option value="" disabled>Choose an analysis</option>{jobs.filter(item => item.kind === "repository").map(item => <option key={item.id} value={item.id}>{new Date(item.createdAt).toLocaleString()} · {item.status}</option>)}</select></label>}
      <Button type="button" variant="ghost" loading={pending === "reload"} disabled={!!pending} onClick={() => void act("reload")}>Refresh saved analyses</Button>
    </div>
  </section>;
}
