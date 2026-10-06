import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { productApi, errorText, type GitState } from "../../api/product";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { ProjectRequired, JourneyHeader } from "../journey/shared";
import { useSetup } from "../setup/SetupProvider";
import { ProjectDelivery } from "../journey/ProjectDelivery";
import { ProjectBackup } from "../journey/ProjectBackup";

export function MergeLens() {
  return (
    <ProjectRequired>
      {(id) => <ProjectGit key={id} projectId={id} />}
    </ProjectRequired>
  );
}
export function ProjectGit({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  return <ProjectGitReview key={JSON.stringify([productApi.baseUrl, scope, projectId])} projectId={projectId} />;
}

function ProjectGitReview({ projectId }: { projectId: string }) {
  const setup = useSetup();
  const access = useProjectAccess(projectId);
  const grantedEdit = access.canRead && access.permissions.edit;
  const [state, setState] = useState<GitState | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState<{ message: string; success: boolean } | null>(null);
  const [busy, setBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [reviewRequired, setReviewRequired] = useState(false);
  const readSequence = useRef(0);
  const saving = useRef<number | null>(null);
  const alive = useRef(true);
  const permission = useRef({ read: access.canRead, edit: access.canEdit, grantedEdit, generation: 0 });
  // A restored grant cannot revive a request admitted before its revocation.
  // Preserve the typed message through a downgrade, but require a new review.
  if (permission.current.read && !access.canRead) ++readSequence.current;
  if (permission.current.grantedEdit && !grantedEdit) ++permission.current.generation;
  permission.current.read = access.canRead;
  permission.current.edit = access.canEdit;
  permission.current.grantedEdit = grantedEdit;
  async function refresh(clearReviewRequired = true) {
    if (!permission.current.read) return;
    const sequence = ++readSequence.current;
    const current = () => alive.current && permission.current.read && sequence === readSequence.current;
    setRefreshing(true);
    try {
      const value = await productApi.git(projectId);
      if (current()) {
        setState(value);
        setError("");
        if (clearReviewRequired) setReviewRequired(false);
      }
    } catch (cause) {
      if (current()) {
        setError(errorText(cause));
        setReviewRequired(true);
      }
    } finally {
      if (current()) setRefreshing(false);
    }
  }
  useLayoutEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      ++readSequence.current;
      ++permission.current.generation;
    };
  }, []);
  useEffect(() => {
    if (!grantedEdit && saving.current !== null) {
      saving.current = null;
      setBusy(false);
      setError("");
      setNotice({ message: "Project access changed while committing. Refresh the review and check Recent commits before trying again. Your message is kept.", success: false });
      setReviewRequired(true);
    }
    if (!access.canRead) { setState(null); setError(""); setRefreshing(false); }
  }, [access.canRead, grantedEdit]);
  useEffect(() => {
    // A read grant restores visibility; only an explicit refresh resolves an
    // uncertain mutation. It must not silently authorize another commit.
    if (access.canRead) void refresh(false);
    return () => { ++readSequence.current; };
  }, [projectId, access.canRead]);
  const commitDisabled = !access.canEdit || busy || refreshing || reviewRequired || !state?.reviewDigest || state.clean || !message.trim();
  return (
    <section className="journey-page">
      <JourneyHeader
        title="Review the work."
        subtitle={
          !access.canRead ? "Project access is unavailable."
            : state
            ? `${state.branch || "No branch yet"} · ${state.clean ? "Everything is committed" : "Uncommitted changes"}`
            : refreshing ? "Reading your Git repository…" : "Refresh the review to read your Git repository."
        }
      >
        <button
          className="luci-button"
          disabled={!access.canRead || busy || refreshing}
          onClick={() => void refresh()}
        >
          {refreshing ? "Refreshing…" : "Refresh review"}
        </button>
      </JourneyHeader>
      {access.message && <p className="journey-muted" role="status">{access.message}</p>}
      {access.canRead && error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {access.canRead && notice && (
        <p className={notice.success ? "journey-success" : "journey-warning"} role="status">
          {notice.message}
        </p>
      )}
      {access.canRead && state && (
        <div className="journey-git-layout">
          <aside>
            <h2>Changed files</h2>
            <ul>
              {state.changes.map((change) => (
                <li key={change.path}>
                  <code>{change.status}</code> {change.path}
                </li>
              ))}
              {!state.changes.length && (
                <li className="journey-muted">No changes to commit.</li>
              )}
            </ul>
            {reviewRequired && <p className="journey-muted" role="status">Refresh the review and inspect the latest changes before committing.{message && " Your message is kept."}</p>}
            {!state.reviewDigest && <p className="journey-muted" role="status">This review has no source identity. Refresh with a current Luci service before committing.</p>}
            <form
              onSubmit={(event) => {
                event.preventDefault();
                if (saving.current !== null || !permission.current.edit || commitDisabled || !state.reviewDigest) return;
                const generation = ++permission.current.generation;
                saving.current = generation;
                const current = () => {
                  if (!alive.current || !permission.current.read || !permission.current.grantedEdit || permission.current.generation !== generation || saving.current !== generation) return false;
                  if (!permission.current.edit) {
                    setNotice({ message: "Editing access could not be confirmed when this commit finished. Refresh the review and check Recent commits before trying again. Your message is kept.", success: false });
                    setReviewRequired(true);
                    return false;
                  }
                  return true;
                };
                setBusy(true);
                setError("");
                setNotice(null);
                void productApi
                  .commit(projectId, message.trim(), state.reviewDigest)
                  .then(async (result) => {
                    if (!current()) return;
                    setMessage("");
                    setReviewRequired(true);
                    setNotice({ message: result.sourceChanged
                      ? "Reviewed changes committed. Newer edits remain uncommitted; review them separately."
                      : "Reviewed changes committed.", success: true });
                    setup?.refreshProjects();
                    await refresh();
                  })
                  .catch((cause) => {
                    if (current()) {
                      setError(errorText(cause));
                      setReviewRequired(true);
                    }
                  })
                  .finally(() => {
                    if (saving.current === generation) {
                      saving.current = null;
                      if (alive.current) setBusy(false);
                    }
                  });
              }}
            >
              <label className="luci-field">
                Commit message
                <textarea
                  required
                  disabled={!access.canEdit || busy}
                  rows={3}
                  value={message}
                  onChange={(event) => setMessage(event.target.value)}
                  placeholder="Describe what changed"
                />
              </label>
              <button
                className="luci-button luci-primary"
                disabled={commitDisabled}
              >
                {busy ? "Committing…" : "Commit changes"}
              </button>
              <p className="journey-muted">
                Commits the source and project context shown in this review locally,
                using your Git identity. Newer changes require a fresh review.
                Nothing is pushed.
              </p>
            </form>
            <h2>Recent commits</h2>
            <ul>
              {state.log.map((commit) => (
                <li key={commit.hash}>
                  <code>{commit.hash.slice(0, 7)}</code> {commit.subject}
                </li>
              ))}
            </ul>
          </aside>
          <pre className="journey-code">
            {state.diff ||
              "No tracked-file diff. New files are listed under Changed files."}
          </pre>
        </div>
      )}
      {access.canRead && state && <ProjectDelivery projectId={projectId} git={state} />}
      {access.canRead && <ProjectBackup projectId={projectId} />}
    </section>
  );
}
