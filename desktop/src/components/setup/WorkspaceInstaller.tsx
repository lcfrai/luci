import { useEffect, useRef, useState } from "react";
import {
  errorText,
  type ProductClient,
  type Tunnel,
  type WorkspaceInstall,
} from "../../api/product";
import { changeWorkspace } from "./workspace";

export function WorkspaceInstaller({
  client,
  connections,
  switchWorkspace = changeWorkspace,
}: {
  client: ProductClient;
  connections: Tunnel[];
  switchWorkspace?: (activate: () => Promise<unknown>) => Promise<void>;
}) {
  const [jobs, setJobs] = useState<WorkspaceInstall[]>([]),
    [accepted, setAccepted] = useState<Record<string, boolean>>({}),
    [busy, setBusy] = useState(""),
    [error, setError] = useState("");
  const alive = useRef(true),
    working = useRef(false);
  const load = async () => {
    const result = await client.workspaceInstalls();
    if (alive.current) setJobs((result.jobs ?? []).sort((a,b)=>Date.parse(b.createdAt)-Date.parse(a.createdAt)));
  };
  useEffect(() => {
    alive.current = true;
    void load().catch((e) => {
      if (alive.current) setError(errorText(e));
    });
    return () => {
      alive.current = false;
    };
  }, [client]);
  useEffect(() => {
    if (
      !jobs.some((j) => ["installing", "running", "queued"].includes(j.status))
    )
      return;
    const timer = setInterval(() => {
      void load().catch((e) => {
        if (alive.current) setError(errorText(e));
      });
    }, 3000);
    return () => clearInterval(timer);
  }, [client, jobs]);
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    setBusy(label);
    setError("");
    try {
      await action();
    } catch (e) {
      if (alive.current) setError(errorText(e));
    } finally {
      working.current = false;
      if (alive.current) setBusy("");
    }
  }
  return (
    <section className="setup-plan">
      <h2>Run all of Luci on your cloud</h2>
      <p className="journey-muted">
        Install Luci’s workspace service on a verified server. Define, Design,
        Develop and Deploy then use that machine through your private SSH
        connection. After switching, install the agent runtime in This workspace
        → Git identity, agent connection and runtime.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {!connections.some((c) => c.status === "connected") && (
        <p>Connect your server above before installing a workspace.</p>
      )}
      {connections
        .filter((c) => c.status === "connected")
        .map((connection) => {
          const job = jobs.find((j) => j.connectionId === connection.id);
          const installing =
            job && ["installing", "running", "queued"].includes(job.status);
          return (
            <article key={connection.id} className="cloud-job">
              <strong>{connection.host}</strong>
              {job && (
                <p role="status">
                  {job.status} · {job.message}
                </p>
              )}
              <label className="luci-check">
                <input
                  type="checkbox"
                  checked={accepted[connection.id] ?? false}
                  onChange={(e) =>
                    setAccepted((v) => ({
                      ...v,
                      [connection.id]: e.target.checked,
                    }))
                  }
                />
                Install Luci and its runtime on this server using my SSH access.
              </label>
              <div className="setup-actions">
                <button
                  className="luci-button"
                  disabled={!!busy || !!installing || !accepted[connection.id]}
                  onClick={() =>
                    void run("Installing Luci", async () => {
                      const result = await client.installWorkspace(
                        connection.id,
                        true,
                      );
                      setJobs((v) => [
                        result,
                        ...v.filter((j) => j.connectionId !== connection.id),
                      ]);
                    })
                  }
                >
                  {installing ? "Installing Luci…" : "Install Luci workspace"}
                </button>
                <button
                  className="luci-button luci-primary"
                  disabled={!!busy || !!installing}
                  onClick={() =>
                    void run("Verifying workspace", () =>
                      switchWorkspace(() =>
                        client.activateWorkspace(connection.id),
                      ),
                    )
                  }
                >
                  {busy === "Verifying workspace"
                    ? "Verifying workspace…"
                    : "Use this cloud workspace"}
                </button>
              </div>
              <p className="journey-muted">
                Switching refreshes the app and detaches current terminal views.
                Existing sessions keep running on their original machine. Save
                your brief and design before switching.
              </p>
            </article>
          );
        })}
      {jobs.length > 0 && (
        <button
          className="luci-button"
          disabled={!!busy}
          onClick={() => void run("Checking installation", load)}
        >
          Check workspace installation
        </button>
      )}
    </section>
  );
}
