import { useEffect, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type CheckCommand,
  type CheckRun,
} from "../../api/product";
import { useAuth } from "../auth/AuthProvider";

const active = (run: CheckRun) =>
  run.status === "queued" || run.status === "running";
export function ProjectChecks({ projectId }: { projectId: string }) {
  const [runs, setRuns] = useState<CheckRun[]>([]);
  const [commands, setCommands] = useState<CheckCommand[]>([]);
  const [trusted, setTrusted] = useState(false);
  const [execution, setExecution] = useState<"docker" | "host">("docker");
  const auth = useAuth();
  const allowHost =
    auth?.profile.permissions.runHostCommands &&
    auth.profile.policy.allowHostCommands;
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const working = useRef(false);
  const alive = useRef(true);
  const [executable, setExecutable] = useState("");
  const [args, setArgs] = useState("[]");
  const updateRun = (run: CheckRun) =>
    setRuns((current) => [
      run,
      ...current.filter((item) => item.id !== run.id),
    ]);
  useEffect(() => {
    alive.current = true;
    void productApi
      .checks(projectId)
      .then((value) => {
        if (alive.current) {
          setRuns(value.runs);
          setCommands(value.suggested || []);
        }
      })
      .catch((cause) => {
        if (alive.current) setError(errorText(cause));
      });
    return () => {
      alive.current = false;
    };
  }, [projectId]);
  const activeIds = runs
    .filter(active)
    .map((run) => run.id)
    .join(",");
  useEffect(() => {
    if (!activeIds) return;
    let cancelled = false;
    const timer = setInterval(() => {
      void Promise.all(
        activeIds.split(",").map((id) => productApi.checkRun(projectId, id)),
      )
        .then((values) => {
          if (!cancelled) values.forEach(updateRun);
        })
        .catch((cause) => {
          if (!cancelled) setError(errorText(cause));
        });
    }, 1500);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [projectId, activeIds]);
  async function operate(action: () => Promise<CheckRun>) {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError("");
    try {
      const run = await action();
      if (alive.current) updateRun(run);
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="project-checks">
      <h2>Verify the implementation</h2>
      <p className="journey-muted">
        Run reviewed commands in this project's Docker backing. Results record
        the commit and any uncommitted changes; passing commands do not prove
        every acceptance criterion.
      </p>
      {allowHost && (
        <label className="luci-field">
          Check execution
          <select
            value={execution}
            onChange={(event) => {
              setExecution(event.target.value as typeof execution);
              setTrusted(false);
            }}
          >
            <option value="docker">Docker · isolated project checks</option>
            <option value="host">Host diagnostics · trusted commands</option>
          </select>
        </label>
      )}
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      <ul className="check-commands">
        {commands.map((command, index) => (
          <li key={index}>
            <div>
              <strong>{command.label}</strong>
              <code>
                {command.executable}{" "}
                {command.args.map((arg) => JSON.stringify(arg)).join(" ")}
              </code>
            </div>
            <button
              className="luci-button"
              disabled={busy || !!activeIds}
              aria-label={`Remove ${command.label}`}
              onClick={() => {
                setCommands(commands.filter((_, i) => index !== i));
                setTrusted(false);
              }}
            >
              Remove
            </button>
          </li>
        ))}
      </ul>
      <form
        className="check-command-form"
        onSubmit={(event) => {
          event.preventDefault();
          try {
            const parsed: unknown = JSON.parse(args);
            if (
              !Array.isArray(parsed) ||
              !parsed.every((value) => typeof value === "string")
            )
              throw new Error(
                'Arguments must be a JSON array of strings, for example ["test"].',
              );
            setCommands([
              ...commands,
              {
                label: executable.trim(),
                executable: executable.trim(),
                args: parsed,
              },
            ]);
            setExecutable("");
            setArgs("[]");
            setTrusted(false);
            setError("");
          } catch (cause) {
            setError(errorText(cause));
          }
        }}
      >
        <label className="luci-field">
          Executable
          <input
            required
            value={executable}
            onChange={(event) => setExecutable(event.target.value)}
            placeholder="npm"
          />
        </label>
        <label className="luci-field">
          Arguments · JSON array
          <input
            required
            value={args}
            onChange={(event) => setArgs(event.target.value)}
            placeholder={'["test"]'}
          />
        </label>
        <button
          className="luci-button"
          disabled={busy || !!activeIds || !executable.trim()}
        >
          Add check command
        </button>
      </form>
      <label className="setup-check">
        <input
          type="checkbox"
          checked={trusted}
          onChange={(event) => setTrusted(event.target.checked)}
        />
        {execution === "host" && allowHost
          ? "I trust these commands to run directly with my workspace host account's access."
          : "I trust this project's code and these Docker commands to write project files and use the network."}
      </label>
      <button
        className="luci-button luci-primary"
        disabled={busy || !!activeIds || !trusted || !commands.length}
        onClick={() =>
          void operate(() =>
            productApi.runChecks(
              projectId,
              commands,
              trusted,
              allowHost ? execution : "docker",
            ),
          )
        }
      >
        {busy
          ? "Starting checks…"
          : activeIds
            ? "Checks running…"
            : "Run project checks"}
      </button>
      {!runs.length && (
        <p className="journey-muted">No verification runs recorded yet.</p>
      )}
      <ul className="check-runs">
        {runs.map((run) => (
          <li key={run.id}>
            <div className="journey-header">
              <div>
                <strong>{run.status}</strong>
                <p>{run.message}</p>
                {run.sourceChanged && (
                  <p className="journey-warning">
                    Project files changed during this run. Run checks again to
                    verify the final state.
                  </p>
                )}
                <small>
                  {run.execution || "docker"} ·{" "}
                  {run.commit ? run.commit.slice(0, 8) : "No commit"}
                  {run.dirty
                    ? " · includes uncommitted changes"
                    : " · clean working tree"}{" "}
                  · {new Date(run.createdAt).toLocaleString()}
                </small>
              </div>
              {active(run) && (
                <button
                  className="luci-button"
                  disabled={busy}
                  onClick={() =>
                    void operate(() =>
                      productApi.cancelChecks(projectId, run.id),
                    )
                  }
                >
                  Cancel checks
                </button>
              )}
            </div>
            {run.commands.map((command, index) => (
              <details key={index}>
                <summary>
                  {command.label} · {command.status}
                  {command.exitCode !== undefined
                    ? ` · exit ${command.exitCode}`
                    : ""}
                </summary>
                <pre className="journey-code">
                  {command.output || "No output recorded."}
                  {command.truncated ? "\n[Output truncated]" : ""}
                </pre>
              </details>
            ))}
          </li>
        ))}
      </ul>
    </section>
  );
}
