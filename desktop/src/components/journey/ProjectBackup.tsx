import { useRef, useState } from "react";
import { productApi, errorText, type ProductClient } from "../../api/product";

export function ProjectBackup({ projectId }: { projectId: string }) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const working = useRef(false);
  return (
    <section className="project-plan">
      <h2>Keep a project backup</h2>
      <p className="journey-muted">
        Download your project files, Git history and Luci project context. Keep
        the archive private: Git history can contain previously committed
        secrets. Application databases and runtime volumes need separate
        backups.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      <button
        className="luci-button"
        disabled={busy}
        onClick={() => {
          if (working.current) return;
          working.current = true;
          setBusy(true);
          setError("");
          void productApi
            .backupProject(projectId)
            .then((blob) => {
              const url = URL.createObjectURL(blob),
                link = document.createElement("a");
              link.href = url;
              link.download = `luci-${projectId}-backup.zip`;
              document.body.appendChild(link);
              link.click();
              link.remove();
              setTimeout(() => URL.revokeObjectURL(url), 30_000);
            })
            .catch((cause) => setError(errorText(cause)))
            .finally(() => {
              working.current = false;
              setBusy(false);
            });
        }}
      >
        {busy ? "Preparing backup…" : "Download project backup"}
      </button>
    </section>
  );
}

export function ProjectRestore({
  client,
  onRestored,
}: {
  client: ProductClient;
  onRestored(project: { id: string; name: string; path: string }): void;
}) {
  const [file, setFile] = useState<File | null>(null),
    [name, setName] = useState(""),
    [trusted, setTrusted] = useState(false);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const working = useRef(false);
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
          working.current = true;
          setBusy(true);
          setError("");
          void client
            .restoreProject(name.trim(), file)
            .then(onRestored)
            .catch((cause) => setError(errorText(cause)))
            .finally(() => {
              working.current = false;
              setBusy(false);
            });
        }}
      >
        <label className="luci-field">
          Luci backup ZIP
          <input
            required
            type="file"
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
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label className="setup-check">
          <input
            type="checkbox"
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
