import { useCallback, useEffect, useState, type ReactNode } from "react";
import {
  productApi,
  errorText,
  type Brief,
  type Design,
  type GitState,
} from "../../api/product";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import { ProjectPlan } from "./ProjectPlan";
import { ProjectChecks } from "./ProjectChecks";
import "./journey.css";

/** Saved project facts are a handoff, not a claim that its acceptance criteria passed. */
export function ProjectWork({
  projectId,
  children,
}: {
  projectId: string;
  children?: ReactNode;
}) {
  const [facts, setFacts] = useState<{
    brief: Brief;
    design: Design;
    git: GitState;
  } | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const setLens = useCockpit((s) => s.setLens);
  const setup = useSetup();
  const load = useCallback(
    async (isCurrent: () => boolean = () => true) => {
      setLoading(true);
      setError("");
      try {
        const [brief, design, git] = await Promise.all([
          productApi.brief(projectId),
          productApi.design(projectId),
          productApi.git(projectId),
        ]);
        if (isCurrent()) setFacts({ brief, design, git });
      } catch (cause) {
        if (isCurrent()) setError(errorText(cause));
      } finally {
        if (isCurrent()) setLoading(false);
      }
    },
    [projectId],
  );
  useEffect(() => {
    let current = true;
    void load(() => current);
    return () => {
      current = false;
    };
  }, [load]);
  return (
    <details className="project-work" open>
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
            {error}
          </p>
        )}
        <div className="journey-actions">
          <button
            className="luci-button"
            disabled={loading}
            onClick={() => void load()}
          >
            Refresh project facts
          </button>
          <button className="luci-button" onClick={() => setup?.openSettings()}>
            Agent and execution settings
          </button>
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
                <button
                  className="luci-button"
                  onClick={() => setLens("discover")}
                >
                  Edit brief
                </button>
              </section>
              <section>
                <h2>Implementation context</h2>
                <p>
                  {facts.design.revision
                    ? `Design revision ${facts.design.revision}, based on brief ${facts.design.briefRevision}.`
                    : "No saved design yet. Add one when a visual reference will help."}
                </p>
                {facts.design.revision > 0 &&
                  facts.design.briefRevision !== facts.brief.revision && (
                    <p className="journey-muted">
                      The brief has changed since this design was saved. Review
                      the design before implementation.
                    </p>
                  )}
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
                  Start an agent session below. The saved LUCI.md carries this
                  brief into the project; review its changes and run checks
                  before deployment.
                </p>
                <div className="journey-actions">
                  <button
                    className="luci-button"
                    onClick={() => setLens("design")}
                  >
                    Review design
                  </button>
                  <button
                    className="luci-button"
                    onClick={() => setLens("files")}
                  >
                    Browse files
                  </button>
                  <button
                    className="luci-button"
                    onClick={() => setLens("merge")}
                  >
                    Review and publish
                  </button>
                  <button
                    className="luci-button"
                    onClick={() => setLens("deploy")}
                  >
                    Preview and deploy
                  </button>
                </div>
              </section>
            </div>
            <ProjectPlan projectId={projectId} brief={facts.brief} />
            <ProjectChecks projectId={projectId} />
            {children}
          </>
        )}
      </div>
    </details>
  );
}
