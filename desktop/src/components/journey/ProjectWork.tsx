import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  productApi,
  errorText,
  type Brief,
  type Design,
  type GitState,
} from "../../api/product";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { definitionApi, type ProjectSpecification } from "../../api/definition";
import { designPlanApi, type DesignPlan } from "../../api/designPlan";
import { PROJECT_CONTEXT_CHANGED, PROJECT_CONTEXT_EDIT, takeProjectContextEdit, type ProjectContextIdentity } from "../../api/projectContextEvents";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import { ProjectPlan } from "./ProjectPlan";
import { ProjectChecks } from "./ProjectChecks";
import { ProjectRuns, type ProjectRunDraft } from "./ProjectRuns";
import { Button } from "../fluid/components/button";
import "./journey.css";

/** Saved project facts are a handoff, not a claim that its acceptance criteria passed. */
type Props = {
  projectId: string;
  children?: ReactNode;
  expanded?: boolean;
};
export function ProjectWork(props: Props) {
  const navigationScope = useProjectDraftScope(props.projectId);
  return <ProjectWorkView key={JSON.stringify([productApi.baseUrl, navigationScope, props.projectId])} {...props} navigationScope={navigationScope} />;
}
function ProjectWorkView({ projectId, children, expanded = true, navigationScope }: Props & { navigationScope: string | null }) {
  const access = useProjectAccess(projectId);
  const canRead = useRef(access.canRead); canRead.current = access.canRead;
  const alive = useRef(true), read = useRef(0);
  const [facts, setFacts] = useState<{
    brief: Brief;
    sketch: Design;
    definition: ProjectSpecification;
    handoff: DesignPlan;
    git: GitState;
  } | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const setLens = useCockpit((s) => s.setLens);
  const setup = useSetup();
  const [runDraft, setRunDraft] = useState<ProjectRunDraft>();
  const details = useRef<HTMLDetailsElement>(null);
  const planRegion = useRef<HTMLElement>(null);
  const [planFocus, setPlanFocus] = useState<{ origin: Element | null; fromDialog: boolean } | null>(null);
  const verification = useCockpit(state => state.projectRunVerification);
  useEffect(() => {
    if (navigationScope && verification?.scope === navigationScope && verification.projectId === projectId && details.current) details.current.open = true;
  }, [verification, navigationScope, projectId]);
  const load = useCallback(
    async () => {
      if (!canRead.current) return;
      const ticket = ++read.current;
      const isCurrent = () => alive.current && canRead.current && ticket === read.current;
      setLoading(true);
      setError("");
      try {
        const [brief, sketch, definition, handoff, git] = await Promise.all([
          productApi.brief(projectId),
          productApi.design(projectId),
          definitionApi.get(projectId),
          designPlanApi.get(projectId),
          productApi.git(projectId),
        ]);
        if (!isCurrent()) return;
        if ([brief, sketch, definition, handoff].some(value => value.projectId !== projectId)) throw new Error("The saved context belongs to another project. Refresh the current project before continuing.");
        setFacts({ brief, sketch, definition, handoff, git });
      } catch (cause) {
        if (isCurrent()) setError(errorText(cause));
      } finally {
        if (isCurrent()) setLoading(false);
      }
    },
    [projectId],
  );
  useEffect(() => {
    alive.current = true;
    if (!access.canRead) { read.current++; setFacts(null); setPlanFocus(null); setLoading(false); return; }
    void load();
    const changed = (event: Event) => {
      const identity = (event as CustomEvent<ProjectContextIdentity>).detail;
      if (navigationScope && identity?.scope === navigationScope && identity.projectId === projectId) void load();
    };
    window.addEventListener(PROJECT_CONTEXT_CHANGED, changed);
    return () => {
      alive.current = false; read.current++;
      window.removeEventListener(PROJECT_CONTEXT_CHANGED, changed);
    };
  }, [load, navigationScope, projectId, access.canRead]);
  useEffect(() => {
    if (!access.canRead) return;
    const reveal = () => {
      if (!takeProjectContextEdit(navigationScope, projectId, "plan")) return;
      if (details.current) details.current.open = true;
      setPlanFocus({ origin: document.activeElement, fromDialog: !!document.activeElement?.closest('[role="dialog"]') });
    };
    reveal(); window.addEventListener(PROJECT_CONTEXT_EDIT, reveal);
    return () => window.removeEventListener(PROJECT_CONTEXT_EDIT, reveal);
  }, [navigationScope, projectId, access.canRead]);
  useEffect(() => {
    if (!planFocus) return;
    const cancel = () => setPlanFocus(null);
    document.addEventListener("pointerdown", cancel, true); document.addEventListener("keydown", cancel, true);
    return () => { document.removeEventListener("pointerdown", cancel, true); document.removeEventListener("keydown", cancel, true); };
  }, [planFocus]);
  useEffect(() => {
    if (!planFocus || !facts || !access.canRead) return;
    let second = 0;
    // A guarded navigation dialog restores its trigger first. Reveal the
    // requested plan after that restoration rather than losing final focus.
    const frame = requestAnimationFrame(() => { second = requestAnimationFrame(() => {
      if (!alive.current || !canRead.current) return;
      const node = planRegion.current;
      // A delayed read must not take focus from newer work elsewhere.
      if (node && (document.activeElement === planFocus.origin || document.activeElement === document.body || (planFocus.fromDialog && !planFocus.origin?.isConnected))) {
        node.focus({ preventScroll: true }); node.scrollIntoView?.({ block: "start" });
      }
      setPlanFocus(null);
    }); });
    return () => { cancelAnimationFrame(frame); cancelAnimationFrame(second); };
  }, [planFocus, facts, access.canRead]);
  const definitionNeedsReview = !!facts && (facts.definition.status === "outdated" || facts.definition.briefRevision !== facts.brief.revision);
  const handoffNeedsReview = !!facts && (facts.handoff.status === "outdated" || facts.handoff.briefRevision !== facts.brief.revision || facts.handoff.specificationRevision !== facts.definition.revision || facts.handoff.sketchRevision !== facts.sketch.revision || definitionNeedsReview || (facts.handoff.status === "approved" && facts.definition.status !== "approved"));
  const reviewStatus = (status: string, stale: boolean) => stale ? "Needs review" : status === "approved" ? "Approved" : "Draft";
  const stateCount = facts?.handoff.screens.reduce((count, screen) => count + screen.states.length, 0) ?? 0;
  if (!access.canRead) return <section className="project-work"><p role="status">{access.message}</p></section>;
  return (
    <>
    <ProjectRuns projectId={projectId} draft={runDraft} onDraftClose={() => setRunDraft(undefined)} onReviewPlan={() => { if (details.current) { details.current.open = true; details.current.scrollIntoView({ block: 'nearest' }); } }} onOpenCodingSession={target => {
      const current = useCockpit.getState();
      if (target.scope === navigationScope && target.projectId === projectId && current.selection.projectId === projectId && current.selection.sessionId === target.sessionId && !current.pendingNavigation && details.current) details.current.open = false;
    }} />
    <details className="project-work" open={expanded} ref={details}>
      <summary>
        <strong role="heading" aria-level={1}>Project plan and checks</strong>
        <span>
          {facts
            ? `Brief ${facts.brief.revision} · ${facts.git.clean ? "Committed" : "Changes to review"}`
            : loading
              ? "Reading project…"
              : "Project handoff"}
        </span>
      </summary>
      <div className="project-work-body">
        {error && (
          <p className="journey-error" role="alert">
            {error}{facts && " The facts below are from the last successful read; refresh to check their current status. Your draft controls remain unchanged."}
          </p>
        )}
        {loading && facts && <p className="journey-muted" role="status">Refreshing saved project facts… Your draft controls remain available.</p>}
        <div className="journey-actions">
          <Button
            variant="secondary"
            disabled={loading}
            onClick={() => void load()}
          >
            Refresh project facts
          </Button>
          <Button variant="secondary" onClick={() => setup?.openSettings()}>
            Agent and execution settings
          </Button>
        </div>
        {facts && (
          <>
            <div className="project-work-facts">
              <section>
                <h2>The brief</h2>
                <p>
                  {facts.brief.purpose ||
                    "Save an idea and acceptance criteria in Define to give the work a direction."}
                </p>
                {!!facts.brief.criteria.length && (
                  <>
                    <h3>Acceptance criteria</h3>
                    <ol>
                      {facts.brief.criteria.map((criterion, index) => (
                        <li key={index}>{criterion}</li>
                      ))}
                    </ol>
                    <p className="journey-muted">
                      These are the saved outcomes to verify. Saving them does
                      not mark them complete.
                    </p>
                  </>
                )}
                <Button
                  variant="secondary"
                  onClick={() => setLens("discover")}
                >
                  Edit brief
                </Button>
              </section>
              <section>
                <h2>Implementation context</h2>
                <p>
                  {facts.definition.revision ? `Definition revision ${facts.definition.revision} · ${reviewStatus(facts.definition.status, definitionNeedsReview)}.` : "No structured definition saved yet. Define the outcomes and review them before preparing a build."}
                </p>
                <p>{facts.handoff.revision ? `Design handoff revision ${facts.handoff.revision} · ${reviewStatus(facts.handoff.status, handoffNeedsReview)}.` : "No structured design handoff saved yet. Describe the screens, behavior and recovery in Design before implementation."}</p>
                {facts.handoff.revision > 0 && <p>{facts.handoff.screens.length} planned {facts.handoff.screens.length === 1 ? "screen" : "screens"} · {stateCount} documented {stateCount === 1 ? "state" : "states"} · based on definition {facts.handoff.specificationRevision}.</p>}
                {facts.handoff.revision > 0 && handoffNeedsReview && <p className="journey-warning">The saved handoff needs review against the current brief, definition and sketch. Review and save it in Design, then approve the current context before implementation.</p>}
                {facts.handoff.revision > 0 && !handoffNeedsReview && facts.handoff.status === "draft" && <p className="journey-muted">Review and approve this design handoff before preparing a build.</p>}
                <p className="journey-muted">{facts.sketch.revision ? `Optional sketch revision ${facts.sketch.revision}, based on brief ${facts.sketch.briefRevision}.` : "No optional sketch saved."}</p>
                <p>
                  {facts.git.clean
                    ? "Your current changes are committed."
                    : `${facts.git.changes.length} changed files need review before release.`}{" "}
                  {facts.git.log[0] && (
                    <>
                      Current commit{" "}
                      <code>{facts.git.log[0].hash.slice(0, 8)}</code>.
                    </>
                  )}
                </p>
                <p className="journey-muted">
                  Prepare a build from the saved plan below. Its reviewed
                  definition, design handoff and selected tasks guide the agent.
                  Review its changes and run checks before deployment.
                </p>
                <div className="journey-actions">
                  <Button
                    variant="secondary"
                    onClick={() => setLens("design")}
                  >
                    Review design
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => setLens("files")}
                  >
                    Browse files
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => setLens("merge")}
                  >
                    Review and publish
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => setLens("deploy")}
                  >
                    Preview and deploy
                  </Button>
                </div>
              </section>
            </div>
            <section ref={planRegion} tabIndex={-1} aria-label="Implementation plan"><ProjectPlan projectId={projectId} brief={facts.brief} onPrepareRun={setRunDraft} /></section>
            <ProjectChecks projectId={projectId} />
            {children}
          </>
        )}
      </div>
    </details>
    </>
  );
}
