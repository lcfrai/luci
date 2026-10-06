import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { productApi, errorText, type ProductClient } from "../../api/product";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";

export function ProjectBackup({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  return <ScopedProjectBackup key={JSON.stringify([productApi.baseUrl, scope, projectId])} projectId={projectId} />;
}
function ScopedProjectBackup({ projectId }: { projectId: string }) {
  const access = useProjectAccess(projectId);
  const { canEdit } = access;
  const grantedEdit = access.canRead && access.permissions.edit;
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""), [notice, setNotice] = useState("");
  const alive = useRef(true), allowed = useRef(canEdit), granted = useRef(grantedEdit), generation = useRef(0), working = useRef<number | null>(null);
  if (granted.current && !grantedEdit) generation.current++;
  granted.current = grantedEdit; allowed.current = canEdit;
  useLayoutEffect(() => {
    alive.current = true;
    return () => { alive.current = false; generation.current++; };
  }, []);
  useEffect(() => {
    if (!grantedEdit) {
      if (working.current !== null) setNotice("Project access changed before this backup finished. No backup was downloaded. Review your access, then download the backup again.");
      working.current = null; setBusy(false); setError("");
    }
  }, [grantedEdit]);
  return (
    <section className="project-plan">
      <h2>Keep a project backup</h2>
      <p className="journey-muted">
        Download your project files, Git history and Luci project context. Keep
        the archive private: Git history can contain previously committed
        secrets. Application databases and runtime volumes need separate
        backups.
      </p>
      {access.message && <p role="status" className="journey-muted">{access.message}</p>}
      {notice && <p role="status" className="journey-muted">{notice}</p>}
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      <button
        className="luci-button"
        disabled={!canEdit || busy}
        onClick={() => {
          if (!allowed.current || working.current !== null) return;
          const request = ++generation.current;
          working.current = request;
          const current = () => {
            if (!alive.current || !granted.current || generation.current !== request || working.current !== request) return false;
            if (!allowed.current) {
              setNotice("Editing access could not be confirmed when this backup finished. No backup was downloaded. Review your access, then download the backup again once editing is available.");
              return false;
            }
            return true;
          };
          setBusy(true);
          setError(""); setNotice("");
          void productApi
            .backupProject(projectId)
            .then((blob) => {
              if (!current()) return;
              const url = URL.createObjectURL(blob),
                link = document.createElement("a");
              link.href = url;
              link.download = `luci-${projectId}-backup.zip`;
              document.body.appendChild(link);
              link.click();
              link.remove();
              setTimeout(() => URL.revokeObjectURL(url), 30_000);
            })
            .catch((cause) => { if (current()) setError(errorText(cause)); })
            .finally(() => {
              if (working.current === request) { working.current = null; if (alive.current) setBusy(false); }
            });
        }}
      >
        {busy ? "Preparing backup…" : "Download project backup"}
      </button>
    </section>
  );
}

type ProjectRestoreProps = {
  client: ProductClient;
  onRestored(project: { id: string; name: string; path: string }): void;
  onBusy?(busy: boolean): void;
};
export function ProjectRestore(props: ProjectRestoreProps) {
  const scope = useProjectDraftScope("project-restore");
  return <ScopedProjectRestore key={JSON.stringify([props.client.baseUrl, scope])} {...props} />;
}
function ScopedProjectRestore({
  client,
  onRestored,
  onBusy,
}: ProjectRestoreProps) {
  const [file, setFile] = useState<File | null>(null),
    [name, setName] = useState(""),
    [trusted, setTrusted] = useState(false);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const alive = useRef(true), currentClient = useRef(client);
  currentClient.current = client;
  const working = useRef<{ finish(): void } | null>(null);
  useLayoutEffect(() => {
    alive.current = true; setBusy(false); setTrusted(false); setError("");
    return () => {
      alive.current = false;
      const request = working.current;
      working.current = null;
      // The restore may still complete on the server. Release the owning
      // parent's busy state once, without navigating from a detached form.
      request?.finish();
    };
  }, [client]);
  return (
    <section className="project-plan">
      <h2>Restore a project backup</h2>
      <p className="journey-muted">
        Restore a Luci ZIP into a new project folder. Existing projects are
        never replaced; runtime services and databases are not restored.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      <form
        className="journey-form"
        onSubmit={(event) => {
          event.preventDefault();
          if (working.current || !file || !trusted) return;
          let finished = false;
          const request = { finish: () => { if (!finished) { finished = true; onBusy?.(false); } } };
          working.current = request;
          const current = () => alive.current && currentClient.current === client && working.current === request;
          setBusy(true);
          onBusy?.(true);
          setError("");
          void client
            .restoreProject(name.trim(), file)
            .then(project => { if (current()) onRestored(project); })
            .catch((cause) => { if (current()) setError(errorText(cause)); })
            .finally(() => {
              if (current()) { working.current = null; setBusy(false); }
              request.finish();
            });
        }}
      >
        <label className="luci-field">
          Luci backup ZIP
          <input
            required
            type="file"
            disabled={busy}
            accept=".zip,application/zip"
            onChange={(event) => {
              setFile(event.target.files?.[0] || null);
              setTrusted(false);
            }}
          />
        </label>
        <label className="luci-field">
          Restored project name
          <input
            required
            value={name}
            disabled={busy}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label className="setup-check">
          <input
            type="checkbox"
            disabled={busy}
            checked={trusted}
            onChange={(event) => setTrusted(event.target.checked)}
          />
          I trust the source of this project archive.
        </label>
        <button
          className="luci-button"
          disabled={busy || !trusted || !file || !name.trim()}
        >
          {busy ? "Restoring project…" : "Restore as new project"}
        </button>
      </form>
    </section>
  );
}
