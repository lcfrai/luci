import { useId, useState } from "react";
import type { OrchestratorRunSummary, OrchestratorView } from "../../api/orchestrator";
import type { ContextEditor } from "../../api/projectContextEvents";
import { Button } from "../fluid/components/button";
import "./project-understanding.css";

type Stage = OrchestratorView["nextAction"]["lens"];
const stages: { lens: Stage; label: string; purpose: string }[] = [
  { lens: "discover", label: "Define", purpose: "Agree the idea and outcomes" },
  { lens: "design", label: "Design", purpose: "Decide how it should work" },
  { lens: "develop", label: "Develop", purpose: "Build and verify the agreed work" },
  { lens: "deploy", label: "Deploy", purpose: "Release and look after the application" },
];
const runLabels: Record<string, string> = {
  ready: "Build prepared", coding: "Building", "awaiting-checks": "Ready for independent checks",
  "awaiting-release": "Ready for release", blocked: "Needs attention", cancelled: "Further work cancelled", complete: "Run accepted",
};
const activityLabels = { running: "Running", unknown: "Outcome unconfirmed", pending: "Reserved, not started", none: "No pending coding operation" };
const operationLabels = { running: "Running", unknown: "Outcome unconfirmed", dispatching: "Dispatch began; outcome unconfirmed", reserved: "Reserved, not started" };

function BuildScope({ run, title, onInspect }: { run: OrchestratorRunSummary; title: string; onInspect(): void }) {
  const pending = !!run.activity && run.activity !== "none";
  const cancelled = run.dispatchCancelled || run.state === "cancelled";
  return <section className="understanding-build" aria-label={title}>
    <h3>{title}</h3>
    <p><strong>{pending ? `Last recorded: ${activityLabels[run.activity!]}` : runLabels[run.state] || "Build status unavailable"}</strong>{run.outcome ? ` · ${run.outcome === "publish" ? "Publish target" : "Preview target"}` : ""}</p>
    {cancelled && pending && <p><strong>Further work cancelled.</strong> This does not stop an agent already running. Open the run to stop it explicitly.</p>}
    {run.scopeOmitted ? <p>{run.taskCount ?? 0} selected tasks / {run.requirementCount ?? 0} outcomes; inspect this build for its full saved scope.</p>
      : run.tasks?.length ? <ul>{run.tasks.map(task => <li key={task.id}>{task.title}</li>)}</ul> : <p>The selected tasks are available in the build workspace.</p>}
    {!!run.requirements?.length && <details><summary>Outcomes this build must prove</summary><ul>{run.requirements.map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul></details>}
    {!!run.specificationRevision && <p className="understanding-note">Uses reviewed definition {run.specificationRevision}{run.designRevision ? ` and design ${run.designRevision}` : ""}. Later edits do not silently change work already started.</p>}
    {!pending && run.state === "complete" && <p>This outcome was accepted. Historical acceptance does not guarantee current release health.</p>}
    {!pending && run.reason && (run.state !== "complete" || !run.valid) && <p>{run.reason}</p>}
    {pending && <p className="understanding-note">Recorded status. Open the run to reconcile its current outcome.</p>}
    {run.operationsOmitted && <p>{run.operationCount} unfinished coding attempts; inspect this build for their full recorded activity.</p>}
    {!pending && !run.valid && <p className="understanding-note">This run needs verification against the current context. Its recorded progress does not prove the current application is ready.</p>}
    {(pending || !!run.operations?.length) && <details><summary>Recorded coding activity</summary>
      {run.reason && <p>{run.reason}</p>}{run.activityReason && <p>{run.activityReason}</p>}
      {!!run.operations?.length && <ul>{run.operations.map(operation => <li key={operation.id}>
      <strong>{operationLabels[operation.status]}</strong><p>Session {operation.childId}</p><time dateTime={operation.updatedAt}>{new Date(operation.updatedAt).toLocaleString()}</time>
    </li>)}</ul>}</details>}
    <Button variant="secondary" onClick={onInspect}>Inspect build and checks</Button>
  </section>;
}

export function ProjectUnderstanding({ view, onAsk, onReply, onNavigate, onInspect, onEdit, canEdit = true }: {
  view: OrchestratorView; onAsk(message?: string): void; onReply(turnId: string): void; onNavigate(lens: Stage): void; onInspect?(runId: string): void; onEdit?(editor: ContextEditor, questionId?: string): void; canEdit?: boolean;
}) {
  const [changing, setChanging] = useState(false);
  const changeId = useId();
  const edit = (editor: ContextEditor, questionId?: string) => { setChanging(false); if (onEdit) onEdit(editor, questionId); else onNavigate(editor === "definition" ? "discover" : "design"); };
  const { context, nextAction, blockingQuestions } = view;
  const spec = context.specification, run = context.currentRun;
  const activeRuns = context.activeRuns ?? [];
  const latestSeparate = run && !activeRuns.some(active => active.id === run.id);
  const inspect = (runId: string) => onInspect ? onInspect(runId) : onNavigate("develop");
  const stage = stages.find(item => item.lens === nextAction.lens)!;
  const requirements = spec.requirements || [];
  const decisions = (spec.decisions || []).filter(item => item.decision.trim());
  const questions = (spec.questions || []).filter(item => !item.answer.trim());
  const answers = (spec.questions || []).filter(item => item.answer.trim());
  const definitionStatus = spec.status === "approved" ? "Reviewed definition" : spec.status === "outdated" ? "Definition needs review" : "Draft definition";
  const discussDecision = () => {
    const question = blockingQuestions[0];
    if (question) onReply(question.turnId);
    else onAsk();
  };
  return <section className="project-understanding" aria-label="Current project understanding">
    <div className="understanding-stage">
      <p className="understanding-label">Project stage</p>
      <h3>{stage.label}</h3><p>{stage.purpose}</p>
      <nav aria-label="Project stages"><ol>{stages.map(item => <li key={item.lens}>
        <button type="button" aria-label={`Open ${item.label} stage`} aria-current={item.lens === stage.lens ? "step" : undefined} onClick={() => onNavigate(item.lens)}>{item.label}</button>
      </li>)}</ol></nav>
      <small>You can revisit any stage. The next step follows the saved context and verified work.</small>
    </div>

    <section aria-labelledby={`understanding-idea-${view.projectId}`}>
      <h3 id={`understanding-idea-${view.projectId}`}>What we’re building</h3>
      <p className="understanding-purpose">{spec.purpose.trim() || context.brief.purpose.trim() || "The idea is still taking shape. Describe the problem and the result you want."}</p>
      <p className="understanding-label">{definitionStatus}{spec.revision ? ` · Revision ${spec.revision}` : ""}</p>
      {spec.audience.trim() && <p><strong>For </strong>{spec.audience}</p>}
      <Button variant="secondary" aria-expanded={changing} aria-controls={changeId} onClick={() => setChanging(value => !value)}>Change the plan</Button>
      {changing && <section id={changeId} aria-label="Change the plan options">
        <p>Choose what to change. You can edit directly without an agent connection.</p>
        <div className="journey-actions">
          <Button variant="secondary" disabled={!canEdit} onClick={() => edit("definition")}>Edit outcomes</Button>
          <Button variant="secondary" disabled={!canEdit} onClick={() => edit("design")}>Edit experience</Button>
          <Button variant="secondary" onClick={() => { setChanging(false); onAsk(); }}>Discuss a change with Luci</Button>
        </div>
        <p className="understanding-note">Direct edits are drafts until you save them. Luci can propose an amendment when you connect an agent; nothing is sent automatically.</p>
      </section>}
      <p className="understanding-note">Describe a change or edit a stage directly. Review what it affects before saving; changed context needs fresh review before a new build.</p>
    </section>

    <section className="understanding-next" aria-label="Next step"><h3>Next step</h3><p>{nextAction.reason}</p><Button onClick={() => nextAction.id === "answer-question" ? discussDecision() : nextAction.runId && nextAction.lens === "develop" ? inspect(nextAction.runId) : onNavigate(nextAction.lens)}>{nextAction.label}</Button></section>

    {activeRuns.length > 0 && <section aria-label="Active and unconfirmed work">
      <h3>Active and unconfirmed work</h3>
      <p className="understanding-note">These builds have unfinished coding activity. Preparing another build does not change what existing agents were asked to build.</p>
      {activeRuns.map((active, index) => <BuildScope key={active.id} run={active} title={activeRuns.length === 1 ? "In this build" : `Build ${index + 1}`} onInspect={() => inspect(active.id)} />)}
    </section>}
    {latestSeparate && <BuildScope run={run} title={activeRuns.length ? run.state === "ready" ? "Latest prepared build" : "Latest recorded build" : "In this build"} onInspect={() => inspect(run.id)} />}

    <section aria-labelledby={`understanding-outcomes-${view.projectId}`}>
      <h3 id={`understanding-outcomes-${view.projectId}`}>Agreed outcomes</h3>
      {spec.status !== "approved" && requirements.length > 0 && <p className="understanding-note">These outcomes are proposed until the definition is reviewed.</p>}
      {requirements.length ? <ul className="understanding-outcomes">{requirements.slice(0, 3).map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance || "Add an observable acceptance outcome."}</p></li>)}</ul> : <p>No outcomes have been agreed yet. Add examples of how you will know the solution works.</p>}
      {requirements.length > 3 && <details><summary>See all {requirements.length} outcomes</summary><ul>{requirements.slice(3).map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul></details>}
    </section>

    {(blockingQuestions.length > 0 || questions.length > 0) && <section className="understanding-decisions" aria-label="Decisions to resolve">
      <h3>Decisions to resolve</h3>
      {blockingQuestions.map(question => <div key={question.turnId}><p><strong>{question.question}</strong></p><p>{question.rationale}</p></div>)}
      {questions.length > 0 && <ul>{questions.map(question => <li key={question.id}><p>{question.question}</p><Button variant="secondary" disabled={!canEdit} onClick={() => edit("definition", question.id)}>Answer in definition</Button></li>)}</ul>}
      {blockingQuestions.length > 0 && <p className="understanding-note">Resolve these material questions before new implementation work starts.</p>}
      {blockingQuestions.length > 0 && <Button variant="secondary" onClick={discussDecision}>Work through the decisions</Button>}
    </section>}

    {answers.length > 0 && <details className="understanding-boundaries"><summary>Saved answers</summary>
      <p className="understanding-note">{spec.status === "approved" ? `Part of reviewed definition ${spec.revision}.` : "These saved answers need review with the current definition."}</p>
      {answers.map(question => <div key={question.id}><h4>{question.question}</h4><p>{question.answer}</p><Button variant="secondary" disabled={!canEdit} onClick={() => edit("definition", question.id)}>Edit answer in definition</Button></div>)}
    </details>}

    {(spec.constraints.trim() || spec.outOfScope.length > 0 || decisions.length > 0) && <details className="understanding-boundaries"><summary>Scope and decisions</summary>
      {spec.constraints.trim() && <><h4>Constraints</h4><p>{spec.constraints}</p></>}
      {spec.outOfScope.length > 0 && <><h4>Outside this version</h4><ul>{spec.outOfScope.map((item, index) => <li key={index}>{item}</li>)}</ul></>}
      {decisions.length > 0 && <><h4>Decisions already made</h4>{decisions.map(decision => <div key={decision.id}><strong>{decision.title}</strong><p>{decision.decision}</p>{decision.reason && <p className="understanding-note">{decision.reason}</p>}</div>)}</>}
    </details>}

  </section>;
}
