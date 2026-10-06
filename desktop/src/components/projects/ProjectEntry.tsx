import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { errorText, productApi, ProductApiError, type ProductClient } from "../../api/product";
import {
  clearPendingImport, createPendingImport, loadPendingImport, projectImportsApi,
  savePendingImport, verifiedImportAttempt, type PendingProjectImport, type ProjectImportAttempt,
} from "../../api/projectImports";
import { useDismissable } from "../../hooks/useDismissable";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { ProjectRestore } from "../journey/ProjectBackup";
import "./projects.css";

export type ProjectEntryMode = "new" | "import" | "restore";
interface ProjectEntryProps {
  mode?: ProjectEntryMode;
  onClose(): void;
  onCreated(project: { id: string; name: string; path: string }): void;
  client?: ProductClient;
}
export function ProjectEntry(props: ProjectEntryProps) {
  const scope = useProjectDraftScope("project-import");
  // Account and workspace navigation invalidates every outstanding callback.
  return <ProjectEntryForm key={scope ?? "unavailable"} {...props} scope={scope} />;
}
function ProjectEntryForm({
  mode: initialMode = "new",
  onClose,
  onCreated,
  client = productApi,
  scope,
}: ProjectEntryProps & { scope: string | null }) {
  const [restored] = useState(() => loadPendingImport(scope));
  const [pending, setPending] = useState<PendingProjectImport | null>(restored);
  const [attempt, setAttempt] = useState<ProjectImportAttempt | null>(null);
  const [missing, setMissing] = useState(false);
  const [mode, setMode] = useState(initialMode);
  const [name, setName] = useState(initialMode === "import" ? restored?.name ?? "" : "");
  const [repository, setRepository] = useState(restored?.repository ?? "");
  const [trusted, setTrusted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const working = useRef(false);
  const alive = useRef(true);
  const generation = useRef(0);
  useLayoutEffect(() => {
    alive.current = true;
    return () => { alive.current = false; generation.current++; working.current = false; };
  }, []);
  const ref = useDismissable<HTMLDivElement>(true, () => {
    if (!working.current) onClose();
  });
  const title = useRef<HTMLHeadingElement>(null);
  useLayoutEffect(() => {
    title.current?.focus();
  }, []);
  useEffect(() => {
    // Returning to this screen can observe a receipt, never start a clone.
    // New-project and backup-restore flows remain independent of that import.
    if (mode === "import" && pending) void checkImport(pending);
  }, [mode]);
  const retryable = attempt?.status === "failed" || attempt?.status === "interrupted";
  async function checkImport(request: PendingProjectImport) {
    if (working.current) return;
    working.current = true;
    const current = ++generation.current;
    setBusy(true); setError(""); setMissing(false);
    try {
      const value = verifiedImportAttempt(await projectImportsApi.status(request.id, client), request.id, request.name);
      if (!alive.current || current !== generation.current) return;
      setAttempt(value);
    } catch (cause) {
      if (!alive.current || current !== generation.current) return;
      setAttempt(null);
      if (cause instanceof ProductApiError && cause.status === 404) {
        setMissing(true);
        setError("No import receipt was found. The original request may still be in transit. Retry only that original request to avoid starting a second import.");
      } else {
        setError("The import status could not be confirmed. Your original request is saved; check again when this workspace is available.");
      }
    } finally {
      if (alive.current && current === generation.current) { working.current = false; setBusy(false); }
    }
  }
  async function importRepository(original?: PendingProjectImport) {
    if (working.current || !trusted) return;
    let request: PendingProjectImport;
    try {
      if (!scope) throw new Error("Import recovery is unavailable until your account and workspace are known. Finish loading or setting up this workspace before importing.");
      request = original ?? createPendingImport(pending?.name ?? name.trim(), pending?.repository ?? repository.trim());
      if (!original) savePendingImport(scope, request, pending?.id);
      else if (loadPendingImport(scope)?.id !== request.id) throw new Error("The saved import changed in another tab. Reopen Import repository to check its status.");
    } catch (cause) { setError(errorText(cause)); return; }
    setPending(request); setAttempt(null); setMissing(false); setError("");
    working.current = true; setBusy(true);
    const current = ++generation.current;
    try {
      const project = await client.createProject(request.name, request.repository, request.id);
      if (!alive.current || current !== generation.current) return;
      if (!project || typeof project.id !== "string" || !project.id || project.name !== request.name || typeof project.path !== "string" || !project.path) throw new Error("Unverified import result");
      clearPendingImport(scope, request.id);
      setPending(null);
      onCreated(project);
    } catch (cause) {
      if (!alive.current || current !== generation.current) return;
      const body = cause instanceof ProductApiError ? cause.body as { attempt?: unknown; error?: string } | undefined : undefined;
      if (body?.attempt) {
        try {
          setAttempt(verifiedImportAttempt(body.attempt, request.id, request.name));
          return;
        } catch { /* Retain the exact request when a response cannot be verified. */ }
      }
      const rejected = cause instanceof ProductApiError && !body?.attempt && (
        (cause.status === 400 && (body?.error === "invalid_name" || body?.error === "invalid_remote")) ||
        (cause.status === 409 && (body?.error === "project_exists" || body?.error === "setup_required"))
      );
      if (rejected && clearPendingImport(scope, request.id)) {
        setName(request.name); setRepository(request.repository); setPending(null);
        setError(body?.error === "invalid_name" ? "Use a project name with letters, numbers, hyphens or underscores." :
          body?.error === "invalid_remote" ? "Check the HTTPS repository URL before importing again." :
          body?.error === "project_exists" ? "A project with this name already exists. Choose a different name to import this repository." :
          "Finish workspace setup before importing a repository.");
        return;
      }
      // Raw Git output can contain credentials or server details. Recovery
      // relies on the receipt, not an arbitrary transport/error message.
      setError("The import result could not be confirmed. Your original request is saved. Check its status before trying again.");
    } finally {
      if (alive.current && current === generation.current) { working.current = false; setBusy(false); }
    }
  }
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
                if (id === "import" && pending) { setName(pending.name); setRepository(pending.repository); }
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
              onRestored={(project) => { if (alive.current) onCreated(project); }}
              onBusy={(value) => {
                if (!alive.current) return;
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
                  (mode === "import" && (!trusted || pending))
                )
                  return;
                if (mode === "import") { void importRepository(); return; }
                working.current = true;
                const current = ++generation.current;
                setBusy(true);
                setError("");
                void client
                  .createProject(name.trim())
                  .then((project) => { if (alive.current && current === generation.current) onCreated(project); })
                  .catch((cause) => { if (alive.current && current === generation.current) setError(errorText(cause)); })
                  .finally(() => {
                    if (!alive.current || current !== generation.current) return;
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
                  value={mode === "import" && pending ? pending.name : name}
                  disabled={busy || (mode === "import" && !!pending)}
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
                      value={pending?.repository ?? repository}
                      disabled={busy || !!pending}
                      onChange={(event) => setRepository(event.target.value)}
                      placeholder="https://github.com/you/project.git"
                    />
                    <small>
                      Use an HTTPS URL without passwords, tokens, query parameters or a fragment. Private
                      repositories use the Git credentials configured in this
                      workspace.
                    </small>
                  </label>
                  <label className="project-trust">
                    <input
                      type="checkbox"
                      disabled={busy || (!!pending && !retryable && !missing)}
                      checked={trusted}
                      onChange={(event) => setTrusted(event.target.checked)}
                    />
                    I trust the source of this repository.
                  </label>
                </>
              )}
              {mode === "import" && pending && (
                <section aria-label="Import recovery" aria-live="polite">
                  <h2>Import status</h2>
                  <p>{attempt ? importStatusMessage(attempt.status) : "This import has a saved request. Check its status before starting any further work."}</p>
                  {attempt?.status !== "complete" && <p className="journey-muted">Closing this window keeps the request saved and does not cancel an import. Return to Import repository to check it.</p>}
                  <details>
                    <summary>Import details</summary>
                    {attempt?.message && <p>{attempt.message}</p>}
                    <p>Attempt ID: <code>{pending.id}</code></p>
                    {attempt?.retainedPath && <p>Partial files were kept separately from your projects for review. <code>{attempt.retainedPath}</code></p>}
                  </details>
                  <div className="setup-actions">
                    <button className="luci-button" type="button" disabled={busy} onClick={() => void checkImport(pending)}>Check import status</button>
                    {attempt?.status === "complete" && attempt.project && <button className="luci-button luci-primary" type="button" disabled={busy} onClick={() => {
                      clearPendingImport(scope, pending.id);
                      onCreated(attempt.project!);
                    }}>Open imported project</button>}
                    {retryable && <>
                      <button className="luci-button luci-primary" type="button" disabled={busy || !trusted} onClick={() => void importRepository()}>Retry import</button>
                      <button className="luci-button" type="button" disabled={busy} onClick={() => {
                        if (!clearPendingImport(scope, pending.id)) { setError("The saved import changed or browser storage is unavailable. Reopen Import repository to check its status."); return; }
                        setName(pending.name); setRepository(pending.repository);
                        setPending(null); setAttempt(null); setMissing(false); setTrusted(false); setError("");
                      }}>Edit import details</button>
                    </>}
                    {missing && <button className="luci-button luci-primary" type="button" disabled={busy || !trusted} onClick={() => void importRepository(pending)}>Retry original request</button>}
                  </div>
                </section>
              )}
              {error && (
                <p className="journey-error" role="alert">
                  {error}
                </p>
              )}
              <div className="setup-actions">
                {(mode !== "import" || !pending) && <button
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
                </button>}
                <button
                  className="luci-button"
                  type="button"
                  disabled={busy}
                  onClick={onClose}
                >
                  {mode === "import" && pending ? "Close" : "Cancel"}
                </button>
              </div>
              {busy && (
                <p role="status">
                  {mode === "import"
                    ? "Waiting for this import request. Large repositories can take a few minutes."
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

function importStatusMessage(status: ProjectImportAttempt["status"]): string {
  switch (status) {
    case "running": return "The repository import is still running. Check again shortly; this check will not start another clone.";
    case "failed": return "The repository could not be imported. Check its URL, your Git credentials and connection, then retry. You can reuse the same project name.";
    case "interrupted": return "The import was interrupted before a project was published. You can retry with the same project name.";
    case "complete": return "The repository was imported successfully. Open the project when you are ready.";
    case "unconfirmed": return "The import outcome is not yet confirmed. Keep this saved request and check again; starting another import may duplicate work.";
  }
}
