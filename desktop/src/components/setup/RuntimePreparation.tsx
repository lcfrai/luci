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
  onStatus,
}: {
  client: ProductClient;
  manageBacking: boolean;
  automatic?: boolean;
  refreshToken?: number;
  onStatus(status: SetupStatus): void;
}) {
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [job, setJob] = useState<RuntimeJob | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const callback = useRef(onStatus);
  callback.current = onStatus;
  const automaticStarted = useRef(false);
  useEffect(() => {
    let alive = true;
    setChecking(true);
    setError("");
    void Promise.all([client.check(), client.runtime()])
      .then(async ([next, runtime]) => {
        if (!alive) return;
        setStatus(next);
        callback.current(next);
        setJob(runtime);
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
          !image &&
          runtime.status === "idle" &&
          !automaticStarted.current
        ) {
          automaticStarted.current = true;
          const installed = await client.installRuntime();
          if (alive) setJob(installed);
        }
      })
      .catch((cause) => {
        if (alive) setError(errorText(cause));
      })
      .finally(() => {
        if (alive) setChecking(false);
      });
    return () => {
      alive = false;
    };
  }, [client, automatic, manageBacking, attempt, refreshToken]);
  useEffect(() => {
    if (job?.status !== "installing") return;
    let alive = true;
    const timer = setInterval(() => {
      void client
        .runtime()
        .then(async (next) => {
          if (!alive) return;
          setJob(next);
          setError("");
          if (next.status === "ready") {
            const current = await client.check();
            if (alive) {
              setStatus(current);
              callback.current(current);
            }
          }
        })
        .catch((cause) => {
          if (alive) setError(errorText(cause));
        });
    }, 3000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [client, job?.status]);
  const dockerReady =
    status?.checks.find((check) => check.id === "docker")?.status === "ready";
  const imageReady =
    status?.checks.find((check) => check.id === "sandbox-image")?.status ===
    "ready";
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
        {status?.checks
          .filter((check) =>
            ["git", "docker", "sandbox-image"].includes(check.id),
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
      {job?.status === "installing" && (
        <p role="status">
          Installing runtime… {job.message} You can keep planning while this
          runs.
        </p>
      )}
      {job && ["failed", "unknown"].includes(job.status) && (
        <p className="journey-error" role="alert">
          {job.message ||
            "Runtime preparation did not finish. Check dependencies and retry."}
        </p>
      )}
      {status && !dockerReady && (
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
        <button
          className="luci-button"
          disabled={checking}
          onClick={() => setAttempt((value) => value + 1)}
        >
          Check again
        </button>
        {manageBacking &&
          dockerReady &&
          !imageReady &&
          job?.status !== "installing" && (
            <button
              className="luci-button"
              disabled={checking}
              onClick={() => {
                setChecking(true);
                setError("");
                void client
                  .installRuntime()
                  .then(setJob)
                  .catch((cause) => setError(errorText(cause)))
                  .finally(() => setChecking(false));
              }}
            >
              {job?.status === "failed"
                ? "Retry runtime preparation"
                : "Prepare agent runtime"}
            </button>
          )}
      </div>
    </section>
  );
}
