import { useEffect, useRef, useState } from "react";
import { platformApi, type PlatformProject } from "../../api/platform";
import { Button } from "../fluid/components/button";

const stageNames: Record<string, string> = {
  queued: "Preparing restore",
  starting: "Starting PostgreSQL",
  restoring: "Restoring the selected backup",
  checking: "Checking the restored database",
  promoting: "Switching the active database",
  complete: "Restore complete",
  failed: "Restore failed",
  interrupted: "Restore interrupted",
};

export function StandaloneDatabase({
  projectId,
  project,
  dockerReady,
  disabled,
  run,
  onDeployments,
}: {
  projectId: string;
  project: PlatformProject | null;
  dockerReady: boolean;
  disabled: boolean;
  run: (label: string, task: () => Promise<unknown>) => Promise<void>;
  onDeployments: () => void;
}) {
  const [backupId, setBackupId] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [previousConfirmed, setPreviousConfirmed] = useState(false);
  const [importFile, setImportFile] = useState<File>();
  const [trustImport, setTrustImport] = useState(false);
  const importController = useRef<AbortController | null>(null);
  const importInput = useRef<HTMLInputElement>(null);
  const operation = project?.databaseOperation;
  const locked = disabled || operation?.status === "running";
  const database = project?.database;
  const backups = project?.backups || [];
  const selected = backups.find((backup) => backup.id === backupId);
  const previous = project?.databaseRestoreHistory?.find((entry) =>
    entry.status === "succeeded" && entry.candidateDatabase === database?.database &&
    !!entry.previousDatabase && entry.previousDatabase !== entry.candidateDatabase,
  );
  const activeApplications = !!project?.deployments.some(
    (deployment) =>
      deployment.provider === "docker" &&
      ["ready", "building", "deploying", "starting", "running"].includes(deployment.status),
  );
  useEffect(() => { setConfirmed(false); }, [selected?.id, selected?.sha256]);
  useEffect(() => { setPreviousConfirmed(false); }, [previous?.id, database?.database]);
  useEffect(() => {
    setImportFile(undefined); setTrustImport(false);
    if (importInput.current) importInput.current.value = "";
    return () => { importController.current?.abort(new Error("Project changed")); importController.current = null; };
  }, [projectId]);
  const canRestore = !locked && dockerReady && database?.status === "stopped" &&
    !activeApplications && !!selected && confirmed;
  const canRecoverPrevious = !locked && dockerReady && database?.status === "stopped" &&
    !activeApplications && !!previous && previousConfirmed;

  return (
    <details className="data-disclosure">
      <summary>Standalone PostgreSQL database</summary>
      <section className="data-section" aria-label="Standalone PostgreSQL">
        <h3>Standalone PostgreSQL</h3>
        <p>{database?.message || "Create a private database on this Docker host."}</p>
        <p>
          Choose this lighter database when your app needs PostgreSQL without the
          full data and authentication stack. Its Docker volume retains your data;
          new application deployments receive DATABASE_URL privately.
        </p>
        <p>Status: <strong>{database?.status || "not-created"}</strong></p>
        <div className="journey-actions">
          <Button variant="secondary" disabled={locked || !dockerReady || database?.status === "ready"}
            onClick={() => void run("Start PostgreSQL", () => platformApi.database(projectId, "start"))}>
            Start PostgreSQL
          </Button>
          <Button variant="secondary" disabled={locked || !["ready", "unknown", "failed", "starting"].includes(database?.status || "")}
            onClick={() => void run("Stop PostgreSQL", () => platformApi.database(projectId, "stop"))}>
            Stop PostgreSQL
          </Button>
          <Button variant="secondary" disabled={locked || database?.status !== "ready"}
            onClick={() => void run("Create database backup", () => platformApi.database(projectId, "backup"))}>
            Create database backup
          </Button>
        </div>
        <p className="data-summary">Backups are private PostgreSQL custom-format dumps, up to 64 MiB.</p>
        {!backups.length && <p className="data-summary">Create a backup while PostgreSQL is running, or import a saved dump to enable recovery.</p>}
        <form className="data-form data-recovery" aria-label="Import PostgreSQL backup" onSubmit={event => {
          event.preventDefault();
          if (locked || !importFile || !trustImport || importController.current) return;
          const selectedFile = importFile;
          const controller = new AbortController();
          importController.current = controller;
          setTrustImport(false);
          void run("Import PostgreSQL backup", async () => {
            const imported = await platformApi.importDatabaseBackup(projectId, selectedFile, true, controller.signal);
            if (!controller.signal.aborted) {
              setBackupId(imported.id); setConfirmed(false); setImportFile(undefined);
              if (importInput.current) importInput.current.value = "";
            }
            return imported;
          }).finally(() => { if (importController.current === controller) importController.current = null; });
        }}>
          <h4>Import a saved PostgreSQL dump</h4>
          <p>Recover a downloaded Luci database backup in this project. Import checks its format and checksum and stores it privately. It does not start PostgreSQL or change your database. Restore it separately after starting and stopping this project's database.</p>
          <p>A dump can contain SQL supplied by its author, which runs during restore. Only import a dump from a source you trust. External database roles and credentials are not included.</p>
          <label>PostgreSQL dump file<input ref={importInput} type="file" accept=".dump,.backup,application/octet-stream" disabled={locked} onChange={event => { setImportFile(event.target.files?.[0]); setTrustImport(false); }} /></label>
          <label className="data-check"><input type="checkbox" disabled={locked || !importFile} checked={trustImport} onChange={event => setTrustImport(event.target.checked)} />I trust this dump and the SQL it can execute during restore.</label>
          <Button type="submit" variant="secondary" disabled={locked || !importFile || !trustImport}>Import PostgreSQL backup</Button>
        </form>
        {!!backups.length && (
          <ul className="data-records" aria-label="PostgreSQL backups">
            {backups.map((backup) => (
              <li key={backup.id}>
                <div>
                  <strong>{backup.imported ? "Imported " : ""}{new Date(backup.createdAt).toLocaleString()}</strong>
                  <small>{Math.ceil(backup.bytes / 1024)} KiB · SHA-256 <code>{backup.sha256.slice(0, 16)}…</code></small>
                </div>
                <Button variant="secondary" disabled={locked}
                  aria-label={`Download database backup ${new Date(backup.createdAt).toLocaleString()}`}
                  onClick={() => void run("Download database backup", () => platformApi.downloadBackup(projectId, backup))}>
                  Download backup
                </Button>
              </li>
            ))}
          </ul>
        )}
        {operation && (
          <div className="data-operation" role={operation.status === "failed" || operation.status === "interrupted" ? "alert" : "status"}>
            <strong>{operation.action === "rollback-restore" && operation.stage === "complete"
              ? "Previous database recovered" : stageNames[operation.stage] || "Database recovery"}</strong>
            <span>{operation.message}</span>
            {operation.status === "succeeded" && <small>Redeploy your application to connect to the restored database.</small>}
            {(operation.status === "failed" || operation.status === "interrupted") && <small>The previous database is retained. Review the message and retry the saved backup below.</small>}
          </div>
        )}
        {!!backups.length && (
          <form className="data-form data-recovery" aria-label="Restore PostgreSQL backup" onSubmit={(event) => {
            event.preventDefault();
            if (!canRestore || !selected) return;
            setConfirmed(false);
            void run("Restore PostgreSQL backup", () => platformApi.restoreDatabase(projectId, selected.id));
          }}>
            <h4>Restore a saved backup</h4>
            <p>
              Luci checks the backup, restores it into a separate database and
              verifies it before switching. The previous database and its volume
              stay available for recovery. Stop application deployments and
              PostgreSQL first so nothing writes during the switch.
            </p>
            {activeApplications && <div>
              <p>Stop your running application deployments before restoring.</p>
              <Button type="button" variant="secondary" disabled={locked} onClick={onDeployments}>Open deployments</Button>
            </div>}
            {database?.status === "ready" && <p>Use Stop PostgreSQL above, then restore your backup.</p>}
            {database?.status === "unknown" && <p>Use Stop PostgreSQL above to verify it is stopped before retrying recovery.</p>}
            {!dockerReady && <p>Start Docker to restore this database.</p>}
            <label>PostgreSQL backup to restore
              <select required disabled={locked} value={backupId} onChange={(event) => {
                setBackupId(event.target.value); setConfirmed(false);
              }}>
                <option value="">Choose a saved backup</option>
                {backups.map((backup) => <option key={backup.id} value={backup.id}>{new Date(backup.createdAt).toLocaleString()} · {Math.ceil(backup.bytes / 1024)} KiB</option>)}
              </select>
            </label>
            <label className="data-check">
              <input type="checkbox" disabled={locked || !selected} checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} />
              Switch to this backup after verification, keeping the previous database.
            </label>
            <Button type="submit" variant="secondary" disabled={!canRestore}>Restore PostgreSQL backup</Button>
          </form>
        )}
        {previous && (
          <details className="data-disclosure">
            <summary>Recover the previous database</summary>
            <form className="data-form" aria-label="Recover previous PostgreSQL database" onSubmit={(event) => {
              event.preventDefault();
              if (!canRecoverPrevious) return;
              setPreviousConfirmed(false);
              void run("Recover previous PostgreSQL database", () => platformApi.recoverPreviousDatabase(projectId, previous.id));
            }}>
              <p>
                Return to the database retained before your last successful switch,
                recorded {new Date(previous.createdAt).toLocaleString()}. Luci checks
                it before changing the active database and keeps the current one.
                Stop your application deployments and PostgreSQL first. After
                recovery, redeploy your application to use it.
              </p>
              <label className="data-check">
                <input type="checkbox" disabled={locked} checked={previousConfirmed} onChange={(event) => setPreviousConfirmed(event.target.checked)} />
                Switch to the verified previous database, keeping the current database.
              </label>
              <Button type="submit" variant="secondary" disabled={!canRecoverPrevious}>Use previous database</Button>
            </form>
          </details>
        )}
        {!!project?.databaseRestoreHistory?.length && (
          <details className="data-disclosure">
            <summary>Restore history</summary>
            <ul className="data-records" aria-label="PostgreSQL restore history">
              {project.databaseRestoreHistory.map((entry) => <li key={entry.id}><div>
                <strong>{new Date(entry.createdAt).toLocaleString()} · {entry.status}</strong>
                <small>{entry.action === "rollback-restore" ? "Previous database recovery" : "Saved backup restore"}</small>
                <small>{entry.message}</small>
              </div></li>)}
            </ul>
          </details>
        )}
      </section>
    </details>
  );
}
