import { useLayoutEffect, useRef, useState } from "react";
import { errorText, productApi, type ProductClient } from "../../api/product";
import { useDismissable } from "../../hooks/useDismissable";
import { ProjectRestore } from "../journey/ProjectBackup";
import "./projects.css";

export type ProjectEntryMode = "new" | "import" | "restore";
export function ProjectEntry({
  mode: initialMode = "new",
  onClose,
  onCreated,
  client = productApi,
}: {
  mode?: ProjectEntryMode;
  onClose(): void;
  onCreated(project: { id: string; name: string; path: string }): void;
  client?: ProductClient;
}) {
  const [mode, setMode] = useState(initialMode);
  const [name, setName] = useState("");
  const [repository, setRepository] = useState("");
  const [trusted, setTrusted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const working = useRef(false);
  const ref = useDismissable<HTMLDivElement>(true, () => {
    if (!working.current) onClose();
  });
  const title = useRef<HTMLHeadingElement>(null);
  useLayoutEffect(() => {
    title.current?.focus();
  }, []);
  const heading =
    mode === "new"
      ? "Create a project"
      : mode === "import"
        ? "Import a repository"
        : "Restore a project";
  return (
    <div className="setup-backdrop">
      <div
        ref={ref}
        className="setup-window project-entry"
        role="dialog"
        aria-modal="true"
        aria-labelledby="project-entry-title"
        onKeyDown={(event) => {
          if (event.key !== "Tab") return;
          const controls = Array.from(
            event.currentTarget.querySelectorAll<HTMLElement>(
              "button:not(:disabled),input:not(:disabled),a[href],select:not(:disabled)",
            ),
          );
          const first = controls[0],
            last = controls[controls.length - 1];
          if (!first) {
            event.preventDefault();
            title.current?.focus();
            return;
          }
          if (
            event.shiftKey &&
            (document.activeElement === first ||
              document.activeElement === title.current)
          ) {
            event.preventDefault();
            last?.focus();
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first?.focus();
          }
        }}
      >
        <header className="setup-header">
          <div>
            <h1 id="project-entry-title" ref={title} tabIndex={-1}>
              {heading}
            </h1>
            <p>
              Every project has its own brief, design, code and release history.
            </p>
          </div>
          <button
            className="setup-close"
            aria-label="Close project creation"
            disabled={busy}
            onClick={onClose}
          >
            ×
          </button>
        </header>
        <nav className="setup-tabs" aria-label="Project source">
          {(
            [
              ["new", "New project"],
              ["import", "Import repository"],
              ["restore", "Restore backup"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              disabled={busy}
              aria-current={mode === id ? "page" : undefined}
              onClick={() => {
                setMode(id);
                setError("");
              }}
            >
              {label}
            </button>
          ))}
        </nav>
        <div className="setup-content">
          {mode === "restore" ? (
            <ProjectRestore
              client={client}
              onRestored={onCreated}
              onBusy={(value) => {
                working.current = value;
                setBusy(value);
              }}
            />
          ) : (
            <form
              aria-busy={busy}
              onSubmit={(event) => {
                event.preventDefault();
                if (
                  working.current ||
                  !name.trim() ||
                  (mode === "import" && !trusted)
                )
                  return;
                working.current = true;
                setBusy(true);
                setError("");
                void client
                  .createProject(
                    name.trim(),
                    mode === "import" ? repository.trim() : undefined,
                  )
                  .then(onCreated)
                  .catch((cause) => setError(errorText(cause)))
                  .finally(() => {
                    working.current = false;
                    setBusy(false);
                  });
              }}
            >
              <p className="journey-muted">
                {mode === "new"
                  ? "Start with an idea. Luci creates a Git repository in your assigned projects folder; you can define and design before connecting an agent."
                  : "Bring an existing HTTPS Git repository into your workspace. Its files and history are cloned; importing does not run its code."}
              </p>
              <label className="luci-field">
                Project name
                <input
                  required
                  autoComplete="off"
                  value={name}
                  disabled={busy}
                  onChange={(event) => setName(event.target.value)}
                  placeholder="my-next-app"
                />
              </label>
              {mode === "import" && (
                <>
                  <label className="luci-field">
                    Repository URL
                    <input
                      required
                      type="url"
                      pattern="https://.*"
                      autoComplete="off"
                      value={repository}
                      disabled={busy}
                      onChange={(event) => setRepository(event.target.value)}
                      placeholder="https://github.com/you/project.git"
                    />
                    <small>
                      Use an HTTPS URL without passwords or tokens. Private
                      repositories use the Git credentials configured in this
                      workspace.
                    </small>
                  </label>
                  <label className="project-trust">
                    <input
                      type="checkbox"
                      disabled={busy}
                      checked={trusted}
                      onChange={(event) => setTrusted(event.target.checked)}
                    />
                    I trust the source of this repository.
                  </label>
                </>
              )}
              {error && (
                <p className="journey-error" role="alert">
                  {error}
                </p>
              )}
              <div className="setup-actions">
                <button
                  className="luci-button luci-primary"
                  disabled={
                    busy || !name.trim() || (mode === "import" && !trusted)
                  }
                >
                  {busy
                    ? mode === "new"
                      ? "Creating project…"
                      : "Importing repository…"
                    : mode === "new"
                      ? "Create project"
                      : "Import repository"}
                </button>
                <button
                  className="luci-button"
                  type="button"
                  disabled={busy}
                  onClick={onClose}
                >
                  Cancel
                </button>
              </div>
              {busy && (
                <p role="status">
                  {mode === "import"
                    ? "Cloning the repository. Large repositories can take a few minutes."
                    : "Preparing your project…"}
                </p>
              )}
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
