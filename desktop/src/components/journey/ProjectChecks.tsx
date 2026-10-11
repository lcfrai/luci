import { useEffect, useMemo, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type CheckCommand,
  type CheckRun,
  type RequirementCoverage,
} from "../../api/product";
import { projectRunsApi, type ProjectRun } from "../../api/projectRuns";
import { notifyProjectContextChanged } from "../../api/projectContextEvents";
import { useProjectDraft, useProjectDraftScope } from "../../hooks/useProjectDraft";
import { checkSetupDraftValid, type CheckSetupDraft } from "../../hooks/projectDraftValidation";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";
import { DraftRecovery } from "../common/DraftRecovery";
import { useCockpit } from "../../store/store";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { ApiError } from "../../api/client";
import { CheckRecoveryDialog } from "./CheckRecoveryDialog";
import { CheckCommandEditor } from "./CheckCommandEditor";
import { Button } from "../fluid/components/button";
import { RequirementsEvidence, commandStatus, evidenceLabel, watchRequirementEvidence } from "./RequirementsEvidence";

// Older settled receipts bind these interim messages into their result digest.
// Correct only their display after exact terminal cleanup is confirmed; preserve
// the raw receipt and every other diagnosis for inspection and recovery.
const legacyPendingCleanupMessages = new Set([
  "The command exit and output were recorded; exact container cleanup is pending.",
  "The exact command exit and output are retained; cleanup is pending.",
  "The command result is retained, but exact Docker cleanup is unconfirmed. Its capacity remains held.",
  "The exact Docker object has not been confirmed absent. Its capacity remains held.",
]);
function checkCommandMessage(command: CheckRun["commands"][number]) {
  const terminalExit = typeof command.exitCode === "number" && Number.isInteger(command.exitCode) &&
    (command.status === "passed" && command.exitCode === 0 || command.status === "failed" && command.exitCode !== 0);
  if (command.cleanupConfirmed === true && command.stage === "cleanup" && terminalExit &&
      (!command.reasonCode || command.reasonCode === "check_cleanup_unconfirmed") &&
      legacyPendingCleanupMessages.has(command.message ?? ""))
    return "The command exit and output were recorded; exact container cleanup is confirmed.";
  return command.message;
}

function checkRunMessage(run: CheckRun) {
  if (run.resourcesHeld === false && (run.status === "passed" || run.status === "failed") &&
      run.stage === "cleanup" && (!run.reasonCode || run.reasonCode === "check_cleanup_unconfirmed")) {
    const confirmed = run.commands.find(command => command.message === run.message && checkCommandMessage(command) !== command.message);
    if (confirmed) return checkCommandMessage(confirmed);
  }
  return run.message;
}

const active = (run: CheckRun) =>
  run.status === "queued" || run.status === "running";
const observes = (run: CheckRun) => active(run) || run.status === "unknown" && run.resourcesHeld === true && ["cleanup", "output", "observe"].includes(run.stage ?? "");
const unsettled = (run: CheckRun) => !active(run) && run.resourcesHeld !== false && (run.revision ?? 0) > 0;
const newer = (previous: CheckRun, next: CheckRun) => (previous.revision ?? 0) > 0 && (next.revision ?? 0) > 0 ? previous.revision! > next.revision! : previous.updatedAt > next.updatedAt;
const contextKey = (value: RequirementCoverage) => JSON.stringify([value.specification, value.design, value.source,
  value.rows.map(row => [row.requirementId, row.title, row.acceptance])]);
const setupBase = (coverage: RequirementCoverage | null, buildId: string) => JSON.stringify([coverage ? contextKey(coverage) : null, buildId]);
export function ProjectChecks({ projectId }: { projectId: string }) {
  const navigationScope = useProjectDraftScope(projectId);
  return <ChecksWorkspace key={JSON.stringify([productApi.baseUrl, navigationScope, projectId])} projectId={projectId} navigationScope={navigationScope} />;
}

function buildContextProblem(build: ProjectRun, coverage: RequirementCoverage | null) {
  if (!coverage) return "Refresh requirement evidence before verifying this build.";
  const reviewed = build.input.approvals;
  if (coverage.specification.status !== "approved" || coverage.design.status !== "approved" ||
      coverage.specification.revision !== reviewed.specificationRevision || coverage.specification.digest !== reviewed.specificationDigest ||
      coverage.design.revision !== reviewed.designRevision || coverage.design.digest !== reviewed.designDigest ||
      build.input.requirements.some(item => !coverage.rows.some(row => row.requirementId === item.id && row.title === item.title && row.acceptance === item.acceptance)))
    return "The current requirements or design no longer match this build's reviewed inputs. Review the changed context and prepare a new build.";
  if (!coverage.source.available) return "Project source is unavailable. Refresh evidence after restoring access.";
  if (coverage.source.dirty || !coverage.source.commit) return "Review and commit your changes before verifying this build. Its results must describe the exact committed version.";
  if (!build.input.requirements.length) return "This build has no saved requirement criteria. Prepare a build with linked requirements before verifying its outcome.";
  return "";
}

function eligibleForBuild(check: CheckRun, build: ProjectRun, coverage: RequirementCoverage | null) {
  return !buildContextProblem(build, coverage) && check.projectId === build.projectId &&
    check.status === "passed" && (!(check.revision ?? 0) || check.resourcesHeld === false && check.commands.every(command => command.cleanupConfirmed)) && check.evidenceStatus === "linked-test-evidence" && !check.dirty && !check.sourceChanged &&
    !!check.commit && !!check.sourceDigest && check.commit === coverage?.source.commit && check.sourceDigest === coverage?.source.sourceDigest &&
    check.specificationRevision === build.input.approvals.specificationRevision && check.specificationDigest === build.input.approvals.specificationDigest &&
    check.designPlanRevision === build.input.approvals.designRevision && check.designPlanDigest === build.input.approvals.designDigest &&
    (check.execution === "docker" || check.execution === "host") && check.commands.length > 0 && check.commands.every(command => command.status === "passed" && command.exitCode === 0) &&
    build.input.requirements.every(requirement => check.requirementIds?.includes(requirement.id) && check.requirementSnapshots?.some(item => item.id === requirement.id && item.title === requirement.title && item.acceptance === requirement.acceptance));
}

function ChecksWorkspace({ projectId, navigationScope }: { projectId: string; navigationScope: string | null }) {
  const access = useProjectAccess(projectId);
  const { canRead, canEdit, canExecute } = access;
  const mayEdit = useRef(canEdit); mayEdit.current = canEdit;
  const mayRead = useRef(canRead); mayRead.current = canRead;
  const verification = useCockpit(state => state.projectRunVerification);
  const [build, setBuild] = useState<ProjectRun | null>(null);
  const [buildId, setBuildId] = useState("");
  const selectedBuild = useRef(buildId); selectedBuild.current = buildId;
  const [readingBuild, setReadingBuild] = useState(false);
  const [buildRead, setBuildRead] = useState(0);
  const [joining, setJoining] = useState("");
  const joinWorking = useRef(false);
  const region = useRef<HTMLElement>(null);
  const [runs, setRuns] = useState<CheckRun[]>([]);
  const [commands, setCommands] = useState<CheckCommand[]>([]);
  const [suggested, setSuggested] = useState<CheckCommand[]>([]);
  const [changed, setChanged] = useState(false);
  const [recordedSetup, setRecordedSetup] = useState("");
  const setupEdited = useRef(false);
  const [trusted, setTrusted] = useState(false);
  const [startUnconfirmed, setStartUnconfirmed] = useState(false);
  // The mutation belongs to this project scope, not the current display authority.
  // Only a usable receipt or an explicit read begun after settlement can retire it.
  const startAttempt = useRef<{ settled: boolean } | null>(null);
  const [recoveryId, setRecoveryId] = useState<string | null>(null);
  const [readDenied, setReadDenied] = useState(false);
  const readDeniedNow = useRef(false); readDeniedNow.current = readDenied;
  const checkRows = useRef(new Map<string, HTMLLIElement>());
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const working = useRef<number | null>(null);
  const alive = useRef(true);
  const [executable, setExecutable] = useState("");
  const [args, setArgs] = useState("[]");
  const [coverage, setCoverage] = useState<RequirementCoverage | null>(null);
  const [requirementIds, setRequirementIds] = useState<string[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [notice, setNotice] = useState("");
  const generation = useRef(0), readGeneration = useRef(0);
  const authorityKey = `${canRead}:${canEdit}:${canExecute}`, authority = useRef(authorityKey);
  if (authority.current !== authorityKey) { authority.current = authorityKey; generation.current++; }
  const historyRead = useRef<AbortController | null>(null);
  const selectedContext = useRef<string | null>(null);
  const [recoveringSelection, setRecoveringSelection] = useState<{ base: string; ids: string[]; buildId: string } | null>(null);
  const draftValue = useMemo<CheckSetupDraft>(() => ({ commands, executable, args, requirementIds, buildId }), [commands, executable, args, requirementIds, buildId]);
  const draftOriginal = useMemo<CheckSetupDraft>(() => ({ commands: suggested, executable: "", args: "[]", requirementIds: [], buildId }), [suggested, buildId]);
  const draft = useProjectDraft({ scope: navigationScope, kind: "verification-setup", base: setupBase(coverage, buildId), baseValue: draftOriginal, value: draftValue, dirty: changed, valid: checkSetupDraftValid });
  const pendingDraft = useRef(!!draft.pending); pendingDraft.current = !!draft.pending;
  function editSetup() { setupEdited.current = true; setChanged(true); setTrusted(false); }
  function discardSetup() {
    draft.clear(); setupEdited.current = false; setChanged(false); setCommands(suggested); setExecutable(""); setArgs("[]");
    setRequirementIds([]); selectedContext.current = null; setRecoveringSelection(null); setTrusted(false); setNotice("");
  }
  useUnsavedProjectDraft(projectId, changed && JSON.stringify(draftValue) !== recordedSetup, "Verification setup", "develop", navigationScope ? { retained: draft.retained, discard: discardSetup } : undefined, "verification-setup");
  function restoreSetup(value: CheckSetupDraft) {
    const base = draft.pending?.base;
    const target = buildId || value.buildId;
    editSetup(); setCommands(value.commands); setExecutable(value.executable); setArgs(value.args);
    setRequirementIds([]); selectedContext.current = null;
    setBuildId(target);
    if (target) setBuildRead(current => current + 1);
    setRecoveringSelection(base ? { base, ids: value.requirementIds, buildId: value.buildId } : null);
    setNotice("Verification setup recovered. Review the commands and give fresh execution consent before running anything.");
  }
  const scoped = !!buildId;
  const buildProblem = build ? buildContextProblem(build, coverage) : scoped ? "Read the selected build before running its checks." : "";
  const requiredSelected = !!build?.input.requirements.length && build.input.requirements.every(item => requirementIds.includes(item.id));
  const requirementChoices = build ? build.input.requirements.map(item => ({ requirementId: item.id, title: item.title, acceptance: item.acceptance })) : coverage?.rows ?? [];
  const canLink = coverage?.specification.status === "approved" && coverage.design.status === "approved" && coverage.source.available;
  function invalidateRead(context: number) {
    if (!alive.current || !mayRead.current || generation.current !== context) return;
    // A confirmed read refusal invalidates cached private content, without discarding local drafts.
    generation.current++; readGeneration.current++; readDeniedNow.current = true;
    historyRead.current?.abort(); setReadDenied(true); setRuns([]); setCoverage(null); setBuild(null);
    setRecoveryId(null); setTrusted(false); setRefreshing(false); setReadingBuild(false);
  }
  async function load(replaceCommands = false, confirmStart = false, confirmAccess = false) {
    if (!canRead || readDeniedNow.current && !confirmAccess) return;
    const context = generation.current, request = ++readGeneration.current;
    const confirmingAttempt = confirmStart && startAttempt.current?.settled ? startAttempt.current : null;
    historyRead.current?.abort(); const controller = new AbortController(); historyRead.current = controller;
    setRefreshing(true);
    try {
      const value = await productApi.checks(projectId, controller.signal);
      if (!alive.current || !mayRead.current || context !== generation.current || request !== readGeneration.current) return;
      if (value.runs.some(run => run.projectId !== projectId)) throw new Error("Check history belongs to another project. Refresh the current project.");
      if (value.coverage && value.coverage.projectId !== projectId) throw new Error("Requirement evidence belongs to another project. Refresh the current project.");
      setRuns(current => [
        ...value.runs.map(row => {
          const previous = current.find(run => run.id === row.id);
          return previous && newer(previous, row) ? previous : row;
        }),
        ...current.filter(run => !value.runs.some(row => row.id === run.id)),
      ].sort((a, b) => b.createdAt.localeCompare(a.createdAt)));
      if (confirmAccess) { readDeniedNow.current = false; setReadDenied(false); setBuildRead(current => current + 1); }
      if (confirmingAttempt && startAttempt.current === confirmingAttempt) {
        startAttempt.current = null; setStartUnconfirmed(false); setTrusted(false);
      }
      setSuggested(value.suggested || []);
      if (replaceCommands && !setupEdited.current && !pendingDraft.current) setCommands(value.suggested || []);
      if (selectedContext.current && (!value.coverage || contextKey(value.coverage) !== selectedContext.current)) {
        setRequirementIds([]); selectedContext.current = null; setTrusted(false);
        setNotice("Requirements, design or source changed. Review and select the criteria again.");
      }
      setCoverage(value.coverage ?? null);
    } catch (cause) {
      if (alive.current && mayRead.current && !controller.signal.aborted && context === generation.current && request === readGeneration.current) { setCoverage(null); setTrusted(false); if (cause instanceof ApiError && [401, 403].includes(cause.status)) invalidateRead(context); setError(errorText(cause)); }
    } finally {
      if (historyRead.current === controller) historyRead.current = null;
      if (alive.current && context === generation.current && request === readGeneration.current) setRefreshing(false);
    }
  }
  const updateRun = (run: CheckRun) => setRuns(current => {
    if (readDeniedNow.current || run.projectId !== projectId) return current;
    const previous = current.find(item => item.id === run.id);
    if (previous && newer(previous, run)) return current;
    return [run, ...current.filter(item => item.id !== run.id)].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  });
  useEffect(() => {
    generation.current++;
    alive.current = true;
    void load(true);
    const unwatch = watchRequirementEvidence(projectId, () => void load());
    return () => {
      alive.current = false;
      generation.current++;
      historyRead.current?.abort(); unwatch();
    };
  }, [projectId, canRead, canEdit, canExecute]);
  useEffect(() => { if (!canExecute) setTrusted(false); }, [canExecute]);
  useEffect(() => { if (!canRead) { setRuns([]); setCoverage(null); setRecoveryId(null); } }, [canRead]);
  const currentContext = coverage ? contextKey(coverage) : "";
  useEffect(() => { setTrusted(false); }, [currentContext]);
  useEffect(() => {
    if (!verification || !canRead || !navigationScope || verification.scope !== navigationScope || verification.projectId !== projectId) return;
    void load(!setupEdited.current);
    setBuildId(verification.runId); setRequirementIds([]); setRecoveringSelection(null); setTrusted(false); setNotice(""); setError(""); selectedContext.current = null;
    setBuildRead(current => current + 1);
    useCockpit.setState(state => state.projectRunVerification === verification ? { projectRunVerification: null } : {});
    requestAnimationFrame(() => { if (alive.current) { region.current?.scrollIntoView?.({ block: 'nearest' }); region.current?.focus(); } });
  }, [verification, canRead, navigationScope, projectId]);
  useEffect(() => {
    setBuild(null);
    if (!buildId || !canRead || readDenied) { setReadingBuild(false); return; }
    let current = true;
    setReadingBuild(true);
    void projectRunsApi.get(projectId, buildId).then(value => {
      if (!current || !alive.current || !mayRead.current) return;
      if (value.id !== buildId || value.projectId !== projectId || value.input.implementation.projectId !== projectId) throw new Error("The requested build does not belong to this project.");
      setBuild(value);
    }).catch(cause => { if (current && alive.current) setError(errorText(cause)); }).finally(() => {
      if (!current || !alive.current) return;
      setReadingBuild(false);
    });
    return () => { current = false; };
  }, [buildId, buildRead, canRead, readDenied, projectId]);
  useEffect(() => {
    if (!recoveringSelection || !coverage || refreshing || (buildId && (readingBuild || !build))) return;
    const matching = recoveringSelection.base === setupBase(coverage, buildId) && recoveringSelection.buildId === buildId &&
      (!build || !buildContextProblem(build, coverage)) && canLink && recoveringSelection.ids.every(id => requirementChoices.some(row => row.requirementId === id));
    setRequirementIds(matching ? recoveringSelection.ids : []);
    selectedContext.current = matching && recoveringSelection.ids.length ? contextKey(coverage) : null;
    if (!matching && recoveringSelection.ids.length) setNotice("Commands and unfinished arguments recovered. The source, reviewed context or selected build changed; review and select the criteria again.");
    setRecoveringSelection(null);
  }, [recoveringSelection, coverage, refreshing, readingBuild, build, buildId]);
  const activeIds = runs
    .filter(active)
    .map((run) => run.id)
    .join(",");
  const observedIds = runs.filter(observes).map(run => run.id).sort().join(",");
  const needsRecovery = runs.some(unsettled);
  useEffect(() => {
    if (!observedIds || !canRead || readDenied || busy || recoveryId) return;
    let cancelled = false, polling = false, cursor = 0;
    const context = generation.current, ids = observedIds.split(",");
    let currentRead: AbortController | null = null;
    const timer = setInterval(() => {
      if (polling || document.visibilityState === 'hidden') return;
      const selected = Array.from({length: Math.min(4, ids.length)}, (_, index) => ids[(cursor + index) % ids.length]);
      cursor = (cursor + selected.length) % ids.length; polling = true;
      const controller = new AbortController(); currentRead = controller;
      void Promise.all(selected.map(id => productApi.checkRun(projectId, id, controller.signal)))
        .then(values => {
          if (cancelled || controller.signal.aborted || !mayRead.current || context !== generation.current) return;
          values.forEach(updateRun);
          if (values.some(run => !observes(run))) { void load(); notifyProjectContextChanged(navigationScope, projectId); }
        }).catch(cause => {
          if (!cancelled && !controller.signal.aborted && context === generation.current) {
            if (cause instanceof ApiError && [401, 403].includes(cause.status)) { cancelled = true; invalidateRead(context); }
            setError(errorText(cause));
          }
          controller.abort();
        }).finally(() => { polling = false; if (currentRead === controller) currentRead = null; });
    }, 5000);
    const visibility = () => { if (document.visibilityState === "hidden") currentRead?.abort(); };
    document.addEventListener("visibilitychange", visibility);
    return () => { cancelled = true; clearInterval(timer); currentRead?.abort(); document.removeEventListener("visibilitychange", visibility); };
  }, [projectId, observedIds, canRead, canEdit, canExecute, readDenied, busy, recoveryId]);
  async function operate(action: () => Promise<CheckRun>, submittedSetup?: CheckSetupDraft, authority: "execute" | "edit" = "execute") {
    if (!canRead || readDeniedNow.current || (authority === "edit" ? !canEdit : !canEdit || !canExecute) || submittedSetup && startUnconfirmed) return;
    const context = generation.current;
    if (working.current !== null) return;
    working.current = context;
    const attempt = submittedSetup ? { settled: false } : null;
    if (attempt) { startAttempt.current = attempt; setTrusted(false); }
    let confirmedStart = false;
    setBusy(true);
    setError("");
    try {
      const run = await action();
      if (alive.current && mayRead.current && context === generation.current) {
        if (run.projectId !== projectId) throw new Error("The returned checks do not belong to this project. Refresh check history before continuing.");
        updateRun(run);
        confirmedStart = true;
        // Keep a reusable browser template, but do not call an unchanged,
        // recorded submission unsaved work. Unadded text remains a draft.
        if (submittedSetup && !submittedSetup.executable && submittedSetup.args === "[]") setRecordedSetup(JSON.stringify(submittedSetup));
        notifyProjectContextChanged(navigationScope, projectId);
        if (!active(run)) void load();
      }
    } catch (cause) {
      if (alive.current && context === generation.current) {
        setError(errorText(cause));
        confirmedStart = cause instanceof ApiError && cause.status >= 400 && cause.status < 500;
      }
    } finally {
      if (attempt && alive.current && startAttempt.current === attempt) {
        attempt.settled = true;
        if (confirmedStart) startAttempt.current = null;
        setStartUnconfirmed(!confirmedStart); setTrusted(false);
      }
      if (working.current === context) { working.current = null; if (alive.current) setBusy(false); }
    }
  }
  async function useResults(check: CheckRun) {
    if (readDeniedNow.current || !build || !navigationScope || !canEdit || joinWorking.current || !eligibleForBuild(check, build, coverage)) return;
    const selected = build, context = generation.current;
    joinWorking.current = true; setJoining(check.id); setError(""); setNotice("");
    try {
      const current = await projectRunsApi.get(projectId, selected.id);
      if (!alive.current || context !== generation.current || selectedBuild.current !== selected.id) return;
      if (!mayEdit.current) throw new Error("Editing access changed. An authorized project member can use these results.");
      if (current.id !== selected.id || current.projectId !== projectId || JSON.stringify(current.input) !== JSON.stringify(selected.input)) throw new Error("The build's reviewed inputs changed. Open its verification step again.");
      if (!current.allowedActions.includes("join-check")) throw new Error("This build cannot accept check evidence yet. Return to its results and reconcile the recorded work.");
      const saved = await projectRunsApi.joinCheck(projectId, current.id, current.revision, check.id);
      if (!alive.current || context !== generation.current || selectedBuild.current !== selected.id) return;
      setBuild(saved);
      setNotice("Results added to this build. Continue from its next step above.");
      notifyProjectContextChanged(navigationScope, projectId);
      useCockpit.setState({ projectRunInspection: { scope: navigationScope, projectId, runId: saved.id, refresh: true } });
    } catch (cause) { if (alive.current && context === generation.current && selectedBuild.current === selected.id) setError(errorText(cause)); }
    finally { joinWorking.current = false; if (alive.current) setJoining(""); }
  }
  function inspectBuild() {
    if (navigationScope && buildId) useCockpit.setState({ projectRunInspection: { scope: navigationScope, projectId, runId: buildId, refresh: true } });
  }
  if (!canRead || readDenied) return <section className="project-checks" ref={region} tabIndex={-1}><p role="status">{!canRead ? access.message : "Check access could not be confirmed. Refresh access before continuing."}</p>{canRead && <Button variant="secondary" disabled={refreshing} onClick={() => void load(false, false, true)}>{refreshing ? 'Checking access…' : 'Refresh check access'}</Button>}</section>;
  return (
    <section className="project-checks" ref={region} tabIndex={-1} aria-label={scoped ? "Build verification" : "Project checks"}>
      <h2>{scoped ? "Verify this build" : "Verify the implementation"}</h2>
      {scoped && <div className="check-build-scope">
        {readingBuild ? <p role="status">Reading the reviewed build…</p> : build && <>
          <p><strong>{build.input.implementation.taskTitles.join("; ")}</strong></p>
          <p className="journey-muted">{build.input.outcome === "publish" ? "Public deployment" : "Workspace preview"} · requirements {build.input.approvals.specificationRevision} · design {build.input.approvals.designRevision}. These are the criteria saved with this build.</p>
          <p>Select each criterion only when the commands below exercise it. All of this build's criteria need passing results before continuing.</p>
        </>}
        {buildProblem && !readingBuild && <p role="status" className="journey-warning">{buildProblem}</p>}
        {!build && !readingBuild && <Button variant="secondary" onClick={() => setBuildRead(current => current + 1)}>Retry reading build</Button>}
        {coverage?.source.dirty && <Button variant="secondary" onClick={() => useCockpit.getState().setLens("merge")}>Review and commit changes</Button>}
        <Button variant="tertiary" onClick={inspectBuild}>Return to build results</Button>
        <Button variant="tertiary" disabled={busy || !!joining || !!draft.pending} onClick={() => { editSetup(); setBuild(null); setBuildId(""); setRequirementIds([]); setRecoveringSelection(null); selectedContext.current = null; }}>Show all project checks</Button>
      </div>}
      {access.message && <p role="status" className="journey-muted">{access.message}</p>}
      <p className="journey-muted">
        Run these checks in an isolated Docker environment. Each result records
        the project version it tested; passing commands do not prove
        every acceptance criterion.
      </p>
      <p className="journey-muted">New checks run in Docker with shared resource admission. Historical host results remain readable; host execution cannot enforce these limits.</p>
      {startUnconfirmed && <p role="status" className="journey-warning">The check start result is unconfirmed. Refresh check history before authorizing another run; the interrupted request may already have started these commands. <Button variant="secondary" disabled={refreshing || busy} onClick={() => void load(false, true)}>Refresh check history</Button></p>}
      {needsRecovery && <p className="journey-warning">An earlier check still holds capacity or has an unconfirmed allocation. Inspect its recovery below before starting more commands.</p>}
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {notice && <p className="journey-muted" role="status">{notice}</p>}
      <DraftRecovery draft={draft} label="Verification setup" localSetup base={coverage ? setupBase(coverage, buildId || draft.pending?.value.buildId || "") : null} saved={draftOriginal}
        busy={busy || !!joining || refreshing || readingBuild} readOnly={!canExecute} onRestore={restoreSetup} onDiscard={discardSetup} />
      {changed && !draft.pending && <Button type="button" variant="tertiary" disabled={busy || !!joining} onClick={discardSetup}>Discard verification setup</Button>}
      <RequirementsEvidence coverage={coverage} runs={runs} refreshing={refreshing} onRefresh={() => void load(false, true)} />
      <details className="check-requirements" open={scoped || undefined}>
        <summary>Link checks to approved requirements · {requirementIds.length} selected</summary>
        <p className="journey-muted">{scoped ? "Confirm the coverage of these commands. Nothing is selected automatically." : "Select the criteria these commands actually exercise. Unselected commands are recorded as generic checks."}</p>
        {!canLink && <p className="journey-muted">Approve the current requirements and design, and refresh readable project source, before linking evidence.</p>}
        {requirementChoices.map(row => <label className="requirement-choice" key={row.requirementId}>
          <input type="checkbox" aria-label={`Check ${row.requirementId} · ${row.title}`} checked={requirementIds.includes(row.requirementId)} disabled={!canExecute || busy || !!activeIds || !!draft.pending || !!recoveringSelection || refreshing || !canLink || (scoped && !!buildProblem)} onChange={event => {
            editSetup();
            setRequirementIds(current => event.target.checked ? [...current, row.requirementId] : current.filter(id => id !== row.requirementId));
            selectedContext.current = coverage ? contextKey(coverage) : null;
            setNotice(""); setTrusted(false);
          }} />
          <span>{row.requirementId} · {row.title}<small>{row.acceptance}</small></span>
        </label>)}
        {!!requirementIds.length && <Button type="button" variant="tertiary" disabled={!canExecute || busy || !!activeIds || !!draft.pending} onClick={() => { editSetup(); setRequirementIds([]); selectedContext.current = null; }}>Clear requirement selection</Button>}
      </details>
      <CheckCommandEditor
        commands={commands}
        executable={executable}
        args={args}
        controlsDisabled={!canExecute || busy || !!activeIds || !!draft.pending}
        fieldsDisabled={!canExecute || !!draft.pending}
        openAdvanced={!scoped}
        onRemove={(index) => {
          editSetup();
          setCommands(commands.filter((_, i) => index !== i));
          setTrusted(false);
        }}
        onExecutableChange={(value) => { editSetup(); setExecutable(value); }}
        onArgsChange={(value) => { editSetup(); setArgs(value); }}
        onAdd={() => {
          if (!canExecute || draft.pending) return;
          try {
            const parsed: unknown = JSON.parse(args);
            if (
              !Array.isArray(parsed) ||
              !parsed.every((value) => typeof value === "string")
            )
              throw new Error(
                'Arguments must be a JSON array of strings, for example ["test"].',
              );
            editSetup(); setCommands([
              ...commands,
              {
                label: executable.trim(),
                executable: executable.trim(),
                args: parsed,
              },
            ]);
            setExecutable("");
            setArgs("[]");
            setTrusted(false);
            setError("");
          } catch (cause) {
            setError(errorText(cause));
          }
        }}
      />
      <label className="setup-check">
        <input
          type="checkbox"
          checked={trusted}
          disabled={!canExecute || busy || startUnconfirmed || !!draft.pending || !!recoveringSelection || refreshing || readingBuild}
          onChange={(event) => setTrusted(event.target.checked)}
        />
        I trust this project's code and these Docker commands to write project files and use the network.
      </label>
      <Button type="button" variant="primary" loading={busy}
        disabled={!canEdit || !canExecute || busy || !!joining || !!activeIds || needsRecovery || startUnconfirmed || !!draft.pending || !!recoveringSelection || refreshing || readingBuild || !trusted || !commands.length || (!!requirementIds.length && !canLink) || (scoped && (!!buildProblem || !requiredSelected || !build?.allowedActions.includes("join-check")))}
        onClick={() =>
          void operate(() =>
            requirementIds.length && coverage ? productApi.runChecks(
              projectId,
              commands,
              trusted,
              "docker",
              { requirementIds, expectedSpecificationRevision: coverage.specification.revision, expectedDesignPlanRevision: coverage.design.revision },
            ) : productApi.runChecks(projectId, commands, trusted, "docker"), draftValue,
          )
        }
      >
        {busy
          ? "Starting checks…"
          : activeIds
            ? "Checks running…"
            : scoped || requirementIds.length ? "Run selected requirement checks" : "Run project checks"}
      </Button>
      {scoped && !requiredSelected && !buildProblem && <p className="journey-muted">Select the criteria these commands cover before starting. Add or improve the tests if a criterion is not exercised yet.</p>}
      {!runs.length && (
        <p className="journey-muted">No verification runs recorded yet.</p>
      )}
      <ul className="check-runs">
        {runs.map((run) => (
          <li key={run.id} ref={element => { if (element) checkRows.current.set(run.id, element); else checkRows.current.delete(run.id); }}>
            <div className="journey-header">
              <div>
                <strong tabIndex={-1} data-check-heading>{commandStatus(run.status)}</strong>
                <p>{checkRunMessage(run)}</p>
                {run.resourcesHeld === true && <p className="journey-warning">Capacity remains held until the original check allocation is confirmed stopped and cleaned up.</p>}
                {(run.revision ?? 0) > 0 && run.resourcesHeld === null && <p className="journey-warning">Capacity ownership is unavailable; no release is assumed.</p>}
                {run.status === "blocked" && <p className="journey-muted">No blocked command will retry automatically. Earlier command output is retained below. Resolve the reported cause, then separately authorize a new run when capacity is released.</p>}
                {!(run.revision ?? 0) && run.status === "unknown" && <p className="journey-muted">This older check has no recorded recovery protocol. Keep its output and ask a host operator to inspect the original execution; starting it again does not resolve that uncertainty.</p>}
                {run.status === "unknown" && <p className="journey-warning">The check outcome is unconfirmed. A passed command below does not establish that this run finished or released its capacity.</p>}
                <p>{evidenceLabel(run.evidenceStatus)}{run.requirementIds?.length ? ` · ${run.requirementIds.join(", ")}` : " · generic commands without requirement links"}</p>
                {run.requirementIds?.length ? <small>Requirements revision {run.specificationRevision ?? 0} · design revision {run.designPlanRevision ?? 0}</small> : null}
                {!!run.staleReasons?.length && <ul>{run.staleReasons.map((reason, index) => <li key={index}>{reason}</li>)}</ul>}
                {run.sourceChanged && (
                  <p className="journey-warning">
                    Project files changed during this run. Run checks again to
                    verify the final state.
                  </p>
                )}
                <small>
                  {run.execution || "docker"} ·{" "}
                  {run.commit ? run.commit.slice(0, 8) : "No commit"}
                  {run.dirty
                    ? " · includes uncommitted changes"
                    : " · clean working tree"}{" "}
                  · {new Date(run.createdAt).toLocaleString()}
                </small>
              </div>
              {active(run) && (
                <Button type="button" variant="secondary"
                  disabled={!canEdit || busy}
                  onClick={() =>
                    void operate(() =>
                      productApi.cancelChecks(projectId, run.id), undefined, "edit",
                    )
                  }
                >
                  Cancel checks
                </Button>
              )}
            </div>
            {(run.revision ?? 0) > 0 && (run.resourcesHeld !== false || run.status === "unknown") && <Button variant="secondary" disabled={busy} onClick={() => setRecoveryId(run.id)}>Review check recovery</Button>}
            {build && <div className="journey-actions">
              {build.checks.some(item => item.checkId === run.id) ? <p role="status">These results are saved with this build.</p> : eligibleForBuild(run, build, coverage) ? <Button variant="primary" loading={joining === run.id} disabled={!canEdit || busy || !!joining || !build.allowedActions.includes("join-check")} onClick={() => void useResults(run)}>Use these results for this build</Button> : run.status === "passed" ? <p className="journey-muted">These results do not match every saved criterion and the current committed version. Run the selected requirement checks for this build.</p> : run.status === "failed" ? <><p className="journey-muted">Review the failed command output, repair the implementation, commit the changes, and verify again.</p><Button variant="secondary" onClick={inspectBuild}>Return to build to repair</Button></> : null}
            </div>}
            {!!run.requirementSnapshots?.length && <details><summary>Requirement text recorded with this run</summary><ul>{run.requirementSnapshots.map(requirement => <li key={requirement.id}><strong>{requirement.id} · {requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul></details>}
            {run.commands.map((command, index) => (
              <details key={index}>
                <summary>
                  {command.label} · {command.status}
                  {command.exitCode !== undefined
                    ? ` · exit ${command.exitCode}`
                    : ""}
                </summary>
                {command.message && <p>{checkCommandMessage(command)}</p>}
                {command.cleanupConfirmed === false && ["passed", "failed", "unknown", "cancelled"].includes(command.status) && <p className="journey-warning">Cleanup has not been confirmed for this command.</p>}
                <pre className="journey-code">
                  {command.output || "No output recorded."}
                  {command.truncated ? "\n[Output truncated]" : ""}
                </pre>
              </details>
            ))}
          </li>
        ))}
      </ul>
      {recoveryId && runs.find(run => run.id === recoveryId) && <CheckRecoveryDialog readAccess={{...access, canRead: canRead && !readDenied}}
        key={recoveryId} run={runs.find(run => run.id === recoveryId)!} projectName={useCockpit.getState().projects.find(project => project.id === projectId)?.name || projectId}
        canRead={canRead} canEdit={canEdit} onUpdate={updateRun} onClose={() => setRecoveryId(null)} onReadDenied={() => invalidateRead(generation.current)}
        getCloseFocusTarget={openingControl => {
          if (!alive.current || !mayRead.current) return null;
          if (readDeniedNow.current) return region.current?.querySelector<HTMLElement>('button') ?? region.current;
          const row = checkRows.current.get(recoveryId);
          if (!row?.isConnected) return null;
          return openingControl?.isConnected && row.contains(openingControl) ? openingControl : row.querySelector<HTMLElement>('[data-check-heading]');
        }} />}
    </section>
  );
}
