import type { OrchestratorView } from "../../api/orchestrator";
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
  ready: "Ready to build", coding: "Building", "awaiting-checks": "Ready for independent checks",
  "awaiting-release": "Ready for release", blocked: "Needs attention", cancelled: "Stopped", complete: "Run accepted",
};

export function ProjectUnderstanding({ view, onAsk, onReply, onNavigate }: {
  view: OrchestratorView; onAsk(message?: string): void; onReply(turnId: string): void; onNavigate(lens: Stage): void;
}) {
  const { context, nextAction, blockingQuestions } = view;
  const spec = context.specification, run = context.currentRun;
  const stage = stages.find(item => item.lens === nextAction.lens)!;
  const requirements = spec.requirements || [];
  const decisions = (spec.decisions || []).filter(item => item.decision.trim());
  const questions = (spec.questions || []).filter(item => !item.answer.trim());
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
      <Button variant="secondary" onClick={() => onAsk()}>Change the plan</Button>
      <p className="understanding-note">Describe a change or edit a stage directly. Review what it affects before saving; changed context needs fresh review before a new build.</p>
    </section>

    <section className="understanding-next" aria-label="Next step"><h3>Next step</h3><p>{nextAction.reason}</p><Button onClick={() => nextAction.id === "answer-question" ? discussDecision() : onNavigate(nextAction.lens)}>{nextAction.label}</Button></section>

    {run && <section className="understanding-build" aria-labelledby={`understanding-build-${view.projectId}`}>
      <h3 id={`understanding-build-${view.projectId}`}>In this build</h3>
      <p><strong>{runLabels[run.state] || "Build status unavailable"}</strong>{run.outcome ? ` · ${run.outcome === "publish" ? "Publish target" : "Preview target"}` : ""}</p>
      {run.tasks?.length ? <ul>{run.tasks.map(task => <li key={task.id}>{task.title}</li>)}</ul> : <p>The selected tasks are available in the build workspace.</p>}
      {!!run.requirements?.length && <details><summary>Outcomes this build must prove</summary><ul>{run.requirements.map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul></details>}
      {!!run.specificationRevision && <p className="understanding-note">Uses reviewed definition {run.specificationRevision}{run.designRevision ? ` and design ${run.designRevision}` : ""}. Later edits do not silently change work already started.</p>}
      {run.reason && <p>{run.reason}</p>}
      {!run.valid && <p className="understanding-note">This run needs verification against the current context. Its recorded progress does not prove the current application is ready.</p>}
      <Button variant="secondary" onClick={() => onNavigate("develop")}>Inspect build and checks</Button>
    </section>}

    <section aria-labelledby={`understanding-outcomes-${view.projectId}`}>
      <h3 id={`understanding-outcomes-${view.projectId}`}>Agreed outcomes</h3>
      {spec.status !== "approved" && requirements.length > 0 && <p className="understanding-note">These outcomes are proposed until the definition is reviewed.</p>}
      {requirements.length ? <ul className="understanding-outcomes">{requirements.slice(0, 3).map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance || "Add an observable acceptance outcome."}</p></li>)}</ul> : <p>No outcomes have been agreed yet. Add examples of how you will know the solution works.</p>}
      {requirements.length > 3 && <details><summary>See all {requirements.length} outcomes</summary><ul>{requirements.slice(3).map(requirement => <li key={requirement.id}><strong>{requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul></details>}
    </section>

    {(blockingQuestions.length > 0 || questions.length > 0) && <section className="understanding-decisions" aria-label="Decisions to resolve">
      <h3>Decisions to resolve</h3>
      {blockingQuestions.map(question => <div key={question.turnId}><p><strong>{question.question}</strong></p><p>{question.rationale}</p></div>)}
      {questions.length > 0 && <ul>{questions.map(question => <li key={question.id}>{question.question}</li>)}</ul>}
      {blockingQuestions.length > 0 && <p className="understanding-note">Resolve these material questions before new implementation work starts.</p>}
      <Button variant="secondary" onClick={discussDecision}>Work through the decisions</Button>
    </section>}

    {(spec.constraints.trim() || spec.outOfScope.length > 0 || decisions.length > 0) && <details className="understanding-boundaries"><summary>Scope and decisions</summary>
      {spec.constraints.trim() && <><h4>Constraints</h4><p>{spec.constraints}</p></>}
      {spec.outOfScope.length > 0 && <><h4>Outside this version</h4><ul>{spec.outOfScope.map((item, index) => <li key={index}>{item}</li>)}</ul></>}
      {decisions.length > 0 && <><h4>Decisions already made</h4>{decisions.map(decision => <div key={decision.id}><strong>{decision.title}</strong><p>{decision.decision}</p>{decision.reason && <p className="understanding-note">{decision.reason}</p>}</div>)}</>}
    </details>}

  </section>;
}
