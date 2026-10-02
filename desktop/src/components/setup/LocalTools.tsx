import { useEffect, useRef, useState } from "react";
import { AgentConnections } from "./AgentConnections";
import { useAuth } from "../auth/AuthProvider";
import {
  errorText,
  type ProductClient,
  type RuntimeJob,
} from "../../api/product";

export function LocalTools({
  client,
  cloud = false,
  onSessionStarted,
}: {
  client: ProductClient;
  cloud?: boolean;
  onSessionStarted?(): void;
}) {
  const auth = useAuth();
  const manageBacking = !auth || auth.profile.permissions.manageBacking;
  const [open, setOpen] = useState(false),
    [loaded, setLoaded] = useState(false),
    [loadAttempt, setLoadAttempt] = useState(0),
    [loadError, setLoadError] = useState(""),
    [name, setName] = useState(""),
    [email, setEmail] = useState("");
  const [runtime, setRuntime] = useState<RuntimeJob | null>(null),
    [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const working = useRef(false);
  useEffect(() => {
    if (!open || loaded) return;
    let cancelled = false;
    setLoadError("");
    Promise.all([client.gitIdentity(), client.runtime()])
      .then(([identity, job]) => {
        if (!cancelled) {
          setName(identity.name);
          setEmail(identity.email);
          setRuntime(job);
          setLoaded(true);
        }
      })
      .catch((e) => {
        if (!cancelled) setLoadError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [client, open, loaded, loadAttempt]);
  useEffect(() => {
    if (runtime?.status !== "installing") return;
    const timer = setInterval(() => {
      void client
        .runtime()
        .then(setRuntime)
        .catch((e) => setError(errorText(e)));
    }, 3000);
    return () => clearInterval(timer);
  }, [client, runtime?.status]);
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (e) {
      setError(errorText(e));
    } finally {
      working.current = false;
      setBusy("");
    }
  }
  return (
    <details
      className="local-tools"
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary>Git identity, agent connection and runtime</summary>
      {open && (
        <div>
          {!loaded && !loadError && (
            <p className="journey-muted" role="status">
              Loading Git identity and runtime…
            </p>
          )}
          {loadError && (
            <div>
              <p className="journey-error" role="alert">
                {loadError}
              </p>
              <button
                className="luci-button"
                type="button"
                onClick={() => setLoadAttempt((attempt) => attempt + 1)}
              >
                Retry loading Git identity and runtime
              </button>
            </div>
          )}
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
          <form
            aria-busy={!loaded && !loadError}
            onSubmit={(e) => {
              e.preventDefault();
              if (!loaded || busy) return;
              void run("Saving identity", async () => {
                await client.saveGitIdentity(name, email);
                setNotice(
                  "Git identity saved for this machine. New commits will use your name and email.",
                );
              });
            }}
          >
            <h2>Your Git identity</h2>
            <p className="journey-muted">
              Choose the name and email attached to your commits. Saving updates
              your global Git configuration on{" "}
              {cloud ? "the cloud workspace" : "this machine"}.
            </p>
            <div className="setup-field-grid">
              <label className="luci-field">
                Commit author name
                <input
                  required
                  disabled={!loaded || !!busy}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </label>
              <label className="luci-field">
                Commit author email
                <input
                  required
                  type="email"
                  disabled={!loaded || !!busy}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </label>
            </div>
            <button className="luci-button" disabled={!loaded || !!busy}>
              Save Git identity
            </button>
          </form>
          {manageBacking ? (
            <section className="setup-plan">
              <h2>Isolated agent runtime</h2>
              <p className="journey-muted">
                Build Luci’s sandbox image using Docker on{" "}
                {cloud ? "the cloud workspace" : "this machine"}. This downloads
                its open-source dependencies and can take several minutes.
              </p>
              {runtime && (
                <p role="status">
                  {runtime.status} · {runtime.message}
                </p>
              )}
              <button
                className="luci-button"
                disabled={!loaded || !!busy || runtime?.status === "installing"}
                onClick={() =>
                  void run("Starting runtime installation", async () =>
                    setRuntime(await client.installRuntime()),
                  )
                }
              >
                {runtime?.status === "installing"
                  ? "Installing runtime…"
                  : "Install agent runtime"}
              </button>
            </section>
          ) : (
            <p className="journey-muted">
              Your administrator manages the Docker runtime and cloud backing.
              Your Git identity and model connection below belong to your
              account.
            </p>
          )}
          <AgentConnections
            client={client}
            cloud={cloud}
            onSessionStarted={onSessionStarted}
          />
        </div>
      )}
    </details>
  );
}
