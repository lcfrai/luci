import { runtimeInstallActive, runtimeInstallRetained } from "../../api/runtimeInstall";
import { Button } from "../fluid/components/button";
import { useEffect, useRef, useState } from "react";
import { AgentConnections } from "./AgentConnections";
import { RuntimeAdmission } from "./RuntimeAdmission";
import { useAuth } from "../auth/AuthProvider";
import {
  errorText,
  type ProductClient,
  type RuntimeJob,
} from "../../api/product";

export function LocalTools({
  client,
  cloud = false,
  manageBacking: workspaceManageBacking,
  onSessionStarted,
  identityOnly = false,
  runtimeBlockedReason = '',
}: {
  client: ProductClient;
  cloud?: boolean;
  manageBacking?: boolean;
  onSessionStarted?(): void;
  identityOnly?: boolean;
  runtimeBlockedReason?: string;
}) {
  const auth = useAuth();
  const manageBacking =
    workspaceManageBacking ?? (!auth || auth.profile.permissions.manageBacking);
  const [open, setOpen] = useState(identityOnly),
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
  const generation = useRef(0);
  const mutation = useRef(0);
  useEffect(() => {
    generation.current += 1;
    working.current = false;
    setLoaded(false);
    setRuntime(null);
    setBusy("");
    return () => { generation.current += 1; };
  }, [client]);
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
    if (!runtimeInstallActive(runtime)) return;
    let alive = true;
    let polling = false;
    const controller = new AbortController();
    const timer = setInterval(() => {
      if (polling) return;
      polling = true;
      const observedMutation = mutation.current;
      void client.runtime(controller.signal).then((next) => {
        if (alive && observedMutation === mutation.current) setRuntime(next);
      }).catch((cause) => {
        if (alive && observedMutation === mutation.current) setError(errorText(cause));
      }).finally(() => { polling = false; });
    }, 3000);
    return () => { alive = false; controller.abort(); clearInterval(timer); };
  }, [client, runtime?.id, runtime?.status]);
  const assemblyAvailable = runtime?.inputProof?.identity.purpose === 'binary-bindings'
    && runtime.inputProof.phase === 'facts-recorded' && runtime.inputProof.cleanupConfirmed
    && !runtime.inputProof.resourcesHeld && !runtime.inputProof.publicationUnknown;
  const runtimeAction = (action: "install" | "documents" | "assemble" | "cancel" | "reconcile") => {
    const starts = action === 'install' || action === 'documents' || action === 'assemble';
    if (!manageBacking || working.current || (!starts && !runtime?.id)) return;
    if (starts && (runtimeBlockedReason || runtimeInstallActive(runtime) || runtimeInstallRetained(runtime))) return;
    if (action === 'assemble' && !assemblyAvailable) return;
    const currentGeneration = generation.current;
    const selectedMutation = ++mutation.current;
    void run(starts ? action === 'assemble' ? 'Assembling document image artifact' : action === 'documents' ? 'Preparing contained document extraction' : "Starting runtime installation" : action === "cancel" ? "Requesting cancellation" : "Checking interrupted installation", async () => {
      const next = action === 'assemble' ? await client.installRuntime('document-assemble') : action === 'documents' ? await client.installRuntime('document-packaged') : action === "install" ? await client.installRuntime() : action === "cancel" ? await client.cancelRuntime(runtime!.id) : await client.reconcileRuntime(runtime!.id);
      if (generation.current === currentGeneration && mutation.current === selectedMutation) setRuntime(next);
    });
  };
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    const currentGeneration = generation.current;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (e) {
      if (generation.current === currentGeneration) setError(errorText(e));
    } finally {
      if (generation.current === currentGeneration) {
        working.current = false;
        setBusy("");
      }
    }
  }
  return (
    <details
      className="local-tools"
      open={open}
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary>
        {identityOnly
          ? "Your Git commit identity"
          : "Git identity, agent connection and runtime"}
      </summary>
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
              <Button
                variant="secondary"
                type="button"
                onClick={() => setLoadAttempt((attempt) => attempt + 1)}
              >
                Retry loading Git identity and runtime
              </Button>
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
                  `Git identity saved for ${cloud ? "this cloud workspace" : "this machine"}. New commits will use your name and email.`,
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
            <Button type="submit" variant="secondary" disabled={!loaded || !!busy}>
              Save Git identity
            </Button>
          </form>
          {!identityOnly &&
            (manageBacking ? (
              <section className="setup-plan">
                <h2>Isolated agent runtime</h2>
                <p className="journey-muted">
                  Build Luci’s sandbox image using Docker on{" "}
                  {cloud ? "the cloud workspace" : "this machine"}. This
                  downloads its open-source dependencies and can take several
                  minutes.
                </p>
                {runtime && (
                  <p role="status">
                    {runtime.status} · {runtime.message}
                  </p>
                )}
                <RuntimeAdmission observation={runtime?.execution?.admission} />
                {runtimeBlockedReason && <p className="preparation-action">{runtimeBlockedReason} Review Docker storage above; you can keep planning.</p>}
                <Button type="button"
                  variant="secondary"
                  disabled={
                    !loaded || !!busy || !!runtimeBlockedReason || runtimeInstallActive(runtime) || runtimeInstallRetained(runtime)
                  }
                  onClick={() => runtimeAction("install")}
                >
                  {runtimeInstallActive(runtime)
                    ? "Runtime preparation in progress…"
                    : "Install agent runtime"}
                </Button>
                <p className="journey-muted">Prepare PDF and image extraction using the document tools included with Luci. This requires a complete installation package.</p>
                <Button type="button" variant="secondary" disabled={!loaded || !!busy || !!runtimeBlockedReason || runtimeInstallActive(runtime) || runtimeInstallRetained(runtime)} onClick={() => runtimeAction('documents')}>
                  Prepare document extraction
                </Button>
                <details>
                  <summary>Maintainer document image assembly</summary>
                  <p className="journey-muted">Assemble a document image artifact from this installation’s completed input proof. This separate operation does not prepare extraction: finalize and package the artifact before installing it.</p>
                  {!assemblyAvailable && <p className="journey-muted">Complete the configured document input proof before assembling its image.</p>}
                  <Button type="button" variant="secondary" disabled={!loaded || !!busy || !!runtimeBlockedReason || !assemblyAvailable || runtimeInstallActive(runtime) || runtimeInstallRetained(runtime)} onClick={() => runtimeAction('assemble')}>
                    Assemble document image artifact
                  </Button>
                </details>
                {runtime?.id && runtimeInstallActive(runtime) && runtime.status !== "reconciling" && (
                  <Button type="button" variant="secondary" disabled={!!busy || !!runtime.cancelRequested || runtime.status === "cancelling"} onClick={() => runtimeAction("cancel")}>
                    {runtime.cancelRequested || runtime.status === "cancelling" ? "Cancellation requested" : "Cancel runtime preparation"}
                  </Button>
                )}
                {runtime?.id && !runtimeInstallActive(runtime) && runtimeInstallRetained(runtime) && (
                  <Button type="button" variant="secondary" disabled={!!busy} onClick={() => runtimeAction("reconcile")}>
                    Check interrupted installation
                  </Button>
                )}
              </section>
            ) : (
              <p className="journey-muted">
                Your administrator manages the Docker runtime and cloud backing.
                Your Git identity and model connection below belong to your
                account.
              </p>
            ))}
          {!identityOnly && (
            <AgentConnections
              client={client}
              cloud={cloud}
              onSessionStarted={onSessionStarted}
            />
          )}
        </div>
      )}
    </details>
  );
}
