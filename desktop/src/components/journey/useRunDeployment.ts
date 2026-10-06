import { useEffect, useRef, useState } from "react";
import { productApi, errorText } from "../../api/product";
import { projectRunsApi, type ProjectRun } from "../../api/projectRuns";
import type { ProjectRunDeploymentIntent } from "../../store/slices/uiSlice";

/** Navigation carries identity, never authority. Reload the owned immutable check
 * and current source before offering a build; the release API fences its commit. */
export function useRunDeployment(projectId: string, intent: ProjectRunDeploymentIntent | null) {
  const [run, setRun] = useState<ProjectRun | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(!!intent);
  const active = useRef(true), generation = useRef(0);
  async function validate() {
    if (!intent) return null;
    const ticket = ++generation.current;
    setLoading(true);
    try {
      const [saved, source, git] = await Promise.all([
        projectRunsApi.get(projectId, intent.runId), productApi.gitDelivery(projectId), productApi.git(projectId),
      ]);
      if (!active.current || ticket !== generation.current) throw new Error("The deployment workspace changed. Reopen the intended build results.");
      const check = saved.checks[saved.checks.length - 1];
      if (saved.id !== intent.runId || saved.projectId !== projectId || intent.projectId !== projectId ||
          saved.input.outcome !== intent.outcome || !check || check.projectId !== projectId ||
          check.checkId !== intent.checkId || check.commit !== intent.commit || check.sourceDigest !== intent.sourceDigest ||
          !saved.allowedActions.includes("join-release")) {
        throw new Error("This build's checked evidence or next step changed. Return to its results and review the intended release again.");
      }
      if (!git.clean || source.head !== intent.commit) {
        throw new Error("The project source changed after this build was checked. Return to its results and verify the intended source before deploying.");
      }
      setRun(saved); setError("");
      return saved;
    } catch (cause) {
      if (active.current && ticket === generation.current) { setRun(null); setError(errorText(cause)); }
      throw cause;
    } finally {
      if (active.current && ticket === generation.current) setLoading(false);
    }
  }
  useEffect(() => {
    active.current = true;
    if (intent) void validate().catch(() => {});
    return () => { active.current = false; ++generation.current; };
  }, [projectId, intent]);
  return { run, error, loading, validate };
}
