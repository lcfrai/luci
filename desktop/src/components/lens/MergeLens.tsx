import { useEffect, useRef, useState } from "react";
import { productApi, errorText, type GitState } from "../../api/product";
import { ProjectRequired, JourneyHeader } from "../journey/shared";
import { useSetup } from "../setup/SetupProvider";

export function MergeLens() {
  return (
    <ProjectRequired>
      {(id) => <ProjectGit key={id} projectId={id} />}
    </ProjectRequired>
  );
}
export function ProjectGit({ projectId }: { projectId: string }) {
  const setup = useSetup();
  const [state, setState] = useState<GitState | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const saving = useRef(false);
  const alive = useRef(true);
  async function refresh() {
    const value = await productApi.git(projectId);
    if (alive.current) {
      setState(value);
      setError("");
    }
  }
  useEffect(() => {
    alive.current = true;
    void refresh().catch((cause) => {
      if (alive.current) setError(errorText(cause));
    });
    return () => {
      alive.current = false;
    };
  }, [projectId]);
  return (
    <section className="journey-page">
      <JourneyHeader
        title="Review the work."
        subtitle={
          state
            ? `${state.branch || "No branch yet"} · ${state.clean ? "Everything is committed" : "Uncommitted changes"}`
            : "Reading your Git repository…"
        }
      >
        <button
          className="luci-button"
          disabled={busy}
          onClick={() =>
            void refresh().catch((cause) => setError(errorText(cause)))
          }
        >
          Refresh
        </button>
      </JourneyHeader>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="journey-success" role="status">
          {notice}
        </p>
      )}
      {state && (
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
            <form
              onSubmit={(event) => {
                event.preventDefault();
                if (saving.current) return;
                saving.current = true;
                setBusy(true);
                setError("");
                setNotice("");
                void productApi
                  .commit(projectId, message.trim())
                  .then(async () => {
                    await refresh();
                    setup?.refreshProjects();
                    if (alive.current) {
                      setMessage("");
                      setNotice("Changes committed locally.");
                    }
                  })
                  .catch((cause) => {
                    if (alive.current) setError(errorText(cause));
                  })
                  .finally(() => {
                    saving.current = false;
                    if (alive.current) setBusy(false);
                  });
              }}
            >
              <label className="luci-field">
                Commit message
                <textarea
                  required
                  rows={3}
                  value={message}
                  onChange={(event) => setMessage(event.target.value)}
                  placeholder="Describe what changed"
                />
              </label>
              <button
                className="luci-button luci-primary"
                disabled={busy || state.clean || !message.trim()}
              >
                {busy ? "Committing…" : "Commit changes"}
              </button>
              <p className="journey-muted">
                Commits all current changes locally using your Git identity.
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
    </section>
  );
}
