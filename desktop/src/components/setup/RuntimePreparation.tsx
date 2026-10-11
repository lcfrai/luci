import { runtimeInstallActive, runtimeInstallRetained } from "../../api/runtimeInstall";
import { storagePreparationBlock } from "../../api/executionStorage";
import { RuntimeAdmission } from "./RuntimeAdmission";
import { Button } from "../fluid/components/button";
import { useEffect, useRef, useState } from "react";
import {
  errorText,
  type ProductClient,
  type RuntimeJob,
  type SetupStatus,
} from "../../api/product";

/** Preparation has real backend states; percentages and successful installs are never inferred. */
export function RuntimePreparation({
  client,
  manageBacking,
  automatic = false,
  refreshToken = 0,
  storageRevision,
  statusSnapshot,
  onStatus,
  onPendingChange,
}: {
  client: ProductClient;
  manageBacking: boolean;
  automatic?: boolean;
  refreshToken?: number;
  /** Invalidates older check responses without starting another probe. */
  storageRevision?: number;
  /** Authoritative or explicitly invalidated parent advice; never a storage DTO. */
  statusSnapshot?: SetupStatus | null;
  /** The parent may reject a response invalidated before React has rendered new props. */
  onStatus(status: SetupStatus, checkedStorageRevision?: number): void | boolean;
  /** Initial checks and install acceptance only; the backend build continues independently. */
  onPendingChange?(pending: boolean): void;
}) {
  const [statusObservation, setStatusObservation] = useState<{ client: ProductClient; value: SetupStatus } | null>(null);
  const [jobObservation, setJobObservation] = useState<{ client: ProductClient; value: RuntimeJob } | null>(null);
  // A new backing must never inherit control targets or advice from the old one,
  // even during the render before its first observation effect has completed.
  const status = statusObservation?.client === client ? statusObservation.value : null;
  const job = jobObservation?.client === client ? jobObservation.value : null;
  const setStatus = (value: SetupStatus) => setStatusObservation({ client, value });
  const setJob = (value: RuntimeJob) => setJobObservation({ client, value });
  const [checking, setChecking] = useState(false);
  const [controlBusy, setControlBusy] = useState(false);
  const mutation = useRef(0);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const callback = useRef(onStatus);
  callback.current = onStatus;
  const pendingCallback = useRef(onPendingChange);
  pendingCallback.current = onPendingChange;
  const automaticStarted = useRef(new WeakSet<ProductClient>());
  const installations = useRef(new WeakMap<ProductClient, Promise<RuntimeJob>>());
  const generation = useRef(0);
  const readiness = useRef<AbortController | null>(null);
  const currentStorageRevision = useRef(storageRevision ?? 0);
  currentStorageRevision.current = storageRevision ?? 0;
  const publish = (value: SetupStatus, checkedStorageRevision: number) => {
    if (checkedStorageRevision !== currentStorageRevision.current) return false;
    const accepted = storageRevision === undefined ? callback.current(value) : callback.current(value, checkedStorageRevision);
    if (accepted === false) return false;
    setStatus(value);
    return true;
  };
  const prepare = (selected: ProductClient) => {
    const existing = installations.current.get(selected);
    if (existing) return existing;
    const pending = selected.installRuntime();
    installations.current.set(selected, pending);
    const clear = () => {
      if (installations.current.get(selected) === pending)
        installations.current.delete(selected);
    };
    void pending.then(clear, clear);
    return pending;
  };
  useEffect(() => {
    let alive = true;
    const controller = new AbortController();
    readiness.current = controller;
    generation.current += 1;
    setControlBusy(false);
    setChecking(true);
    pendingCallback.current?.(true);
    setError("");
    const checkedStorageRevision = currentStorageRevision.current;
    const observedMutation = mutation.current;
    void Promise.all([client.check(controller.signal), client.runtime(controller.signal)])
      .then(async ([next, runtime]) => {
        if (!alive || observedMutation !== mutation.current) return;
        setJob(runtime);
        if (!publish(next, checkedStorageRevision)) return;
        const docker =
          next.checks.find((check) => check.id === "docker")?.status ===
          "ready";
        const image =
          next.checks.find((check) => check.id === "sandbox-image")?.status ===
          "ready";
        if (
          automatic &&
          manageBacking &&
          docker &&
          !storagePreparationBlock(next) &&
          !image &&
          runtime.status === "idle" &&
          !automaticStarted.current.has(client)
        ) {
          automaticStarted.current.add(client);
          prepare(client);
        }
        let settled = runtime;
        const pending = installations.current.get(client);
        if (pending) {
          const installed = await pending;
          if (!alive || observedMutation !== mutation.current) return;
          settled = installed;
          setJob(installed);
        }
        // A job can finish after the first image check. Confirm actual image
        // readiness once rather than treating its ready state as proof.
        if (alive && settled.status === "ready" && !image) {
          const checkRevision = currentStorageRevision.current;
          const current = await client.check(controller.signal);
          if (alive) {
            publish(current, checkRevision);
          }
        }
      })
      .catch((cause) => {
        if (alive) setError(errorText(cause));
      })
      .finally(() => {
        if (alive) {
          setChecking(false);
          pendingCallback.current?.(false);
        }
      });
    return () => {
      alive = false;
      controller.abort();
      if (readiness.current === controller) readiness.current = null;
      generation.current += 1;
      pendingCallback.current?.(false);
    };
  }, [client, automatic, manageBacking, attempt, refreshToken]);
  useEffect(() => {
    if (!runtimeInstallActive(job)) return;
    let alive = true;
    const controller = new AbortController();
    let polling = false;
    const timer = setInterval(() => {
      if (polling) return;
      polling = true;
      const observedMutation = mutation.current;
      void client
        .runtime(controller.signal)
        .then(async (next) => {
          if (!alive || observedMutation !== mutation.current) return;
          let current: SetupStatus | undefined;
          const checkRevision = currentStorageRevision.current;
          let readinessError = "";
          if (next.status === "ready") {
            try {
              current = await client.check(controller.signal);
            } catch (cause) {
              readinessError = errorText(cause);
            }
          }
          if (!alive || observedMutation !== mutation.current) return;
          // Changing installing to ready tears down this polling effect. Apply
          // its actual readiness response before finishing that lifecycle.
          if (current) {
            publish(current, checkRevision);
          }
          setJob(next);
          setError(readinessError);
        })
        .catch((cause) => {
          if (alive && observedMutation === mutation.current) setError(errorText(cause));
        })
        .finally(() => {
          polling = false;
        });
    }, 3000);
    return () => {
      alive = false;
      controller.abort();
      clearInterval(timer);
    };
  }, [client, job?.id, job?.status, automatic, manageBacking, attempt, refreshToken]);
  const control = (action: "cancel" | "reconcile") => {
    if (!manageBacking || controlBusy || !job?.id) return;
    const selectedJob = job.id;
    const currentGeneration = generation.current;
    const selectedMutation = ++mutation.current;
    setControlBusy(true);
    setError("");
    const request = action === "cancel" ? client.cancelRuntime(selectedJob) : client.reconcileRuntime(selectedJob);
    void request.then(async (next) => {
      if (generation.current !== currentGeneration || mutation.current !== selectedMutation) return;
      if (next.status === "ready") {
        const checkRevision = currentStorageRevision.current;
        try {
          const current = await client.check(readiness.current?.signal);
          if (generation.current !== currentGeneration || mutation.current !== selectedMutation) return;
          publish(current, checkRevision);
        } catch (cause) {
          if (generation.current !== currentGeneration || mutation.current !== selectedMutation) return;
          setError(errorText(cause));
        }
      }
      setJob(next);
    }).catch((cause) => {
      if (generation.current === currentGeneration && mutation.current === selectedMutation) setError(errorText(cause));
    }).finally(() => {
      if (generation.current === currentGeneration && mutation.current === selectedMutation) setControlBusy(false);
    });
  };
  const shownStatus = statusSnapshot ?? status;
  const dockerReady =
    shownStatus?.checks.find((check) => check.id === "docker")?.status === "ready";
  const imageReady =
    shownStatus?.checks.find((check) => check.id === "sandbox-image")?.status ===
    "ready";
  const storageBlocked = storagePreparationBlock(shownStatus);
  return (
    <section
      className="runtime-preparation"
      aria-labelledby="preparation-title"
    >
      <h2 id="preparation-title">Prepare your workspace</h2>
      <p className="journey-muted">
        Luci checks Git and Docker, then prepares its isolated agent runtime
        when Docker is ready. This downloads dependencies and can take several
        minutes. No model is called.
      </p>
      {checking && (
        <p role="status">Checking dependencies and preparing your workspace…</p>
      )}
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      <ul className="preparation-list">
        {shownStatus?.checks
          .filter((check) =>
            ["git", "docker", "sandbox-image", "docker-backing-storage"].includes(check.id),
          )
          .map((check) => (
            <li key={check.id}>
              <div>
                <strong>{check.label}</strong>
                <span className={`setup-status setup-status-${check.status}`}>
                  {check.status}
                </span>
              </div>
              <p>{check.message}</p>
              {check.action && check.status !== "ready" && (
                <p className="preparation-action">{check.action}</p>
              )}
            </li>
          ))}
      </ul>
      {storageBlocked && <p className="preparation-action">{storageBlocked} Existing workers keep running; you can keep planning while new runtime preparation waits.</p>}
      {runtimeInstallActive(job) && (
        <p role="status">
          {job?.status === "cancelling" ? "Cancelling runtime preparation…" : job?.status === "reconciling" ? "Checking interrupted installation…" : "Installing runtime…"} {job?.message} You can keep planning while this
          runs.
        </p>
      )}
      {job && ["failed", "unknown", "blocked"].includes(job.status) && (
        <p className="journey-error" role="alert">
          {job.message ||
            "Runtime preparation did not finish. Check dependencies and retry."}
        </p>
      )}
      {job?.status === "cancelled" && <p role="status">{job.message}</p>}
      {manageBacking && <RuntimeAdmission observation={job?.execution?.admission} />}
      {shownStatus && !dockerReady && (
        <p className="journey-muted">
          Use the tool setup above to install Docker where supported. Open
          Docker Desktop and wait for its engine to start, then check again.
        </p>
      )}
      {!manageBacking && (
        <p className="journey-muted">
          Your administrator prepares the runtime. You can create and define a
          project now.
        </p>
      )}
      <div className="setup-actions">
        <Button type="button"
          variant="secondary"
          disabled={checking || controlBusy}
          onClick={() => setAttempt((value) => value + 1)}
        >
          Check again
        </Button>
        {manageBacking && job?.id && runtimeInstallActive(job) && job.status !== "reconciling" && (
          <Button type="button" variant="secondary" disabled={controlBusy || !!job.cancelRequested || job.status === "cancelling"} onClick={() => control("cancel")}>
            {job.cancelRequested || job.status === "cancelling" ? "Cancellation requested" : "Cancel runtime preparation"}
          </Button>
        )}
        {manageBacking && job?.id && !runtimeInstallActive(job) && runtimeInstallRetained(job) && (
          <Button type="button" variant="secondary" disabled={checking || controlBusy} onClick={() => control("reconcile")}>
            Check interrupted installation
          </Button>
        )}
        {manageBacking &&
          dockerReady &&
          !imageReady &&
          !runtimeInstallActive(job) && !runtimeInstallRetained(job) && (
            <Button type="button"
              variant="secondary"
              disabled={checking || controlBusy || !!storageBlocked}
              onClick={() => {
                if (storageBlocked) return;
                const currentGeneration = generation.current;
                const signal = readiness.current?.signal;
                setChecking(true);
                pendingCallback.current?.(true);
                setError("");
                void prepare(client)
                  .then(async (next) => {
                    if (generation.current !== currentGeneration) return;
                    setJob(next);
                    if (next.status === "ready" && !imageReady) {
                      const checkRevision = currentStorageRevision.current;
                      const current = await client.check(signal);
                      if (generation.current === currentGeneration) {
                        publish(current, checkRevision);
                      }
                    }
                  })
                  .catch((cause) => {
                    if (generation.current === currentGeneration) setError(errorText(cause));
                  })
                  .finally(() => {
                    if (generation.current === currentGeneration) {
                      setChecking(false);
                      pendingCallback.current?.(false);
                    }
                  });
              }}
            >
              {job?.status === "failed"
                ? "Retry runtime preparation"
                : "Prepare agent runtime"}
            </Button>
          )}
      </div>
    </section>
  );
}
