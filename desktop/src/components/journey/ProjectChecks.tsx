import { useEffect, useRef, useState } from "react";
import {
  errorText,
  productApi,
  type CheckCommand,
  type CheckRun,
  type RequirementCoverage,
} from "../../api/product";
import { useAuth } from "../auth/AuthProvider";
import { Button } from "../fluid/components/button";
import { RequirementsEvidence, commandStatus, evidenceLabel, watchRequirementEvidence } from "./RequirementsEvidence";

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
  const working = useRef<number | null>(null);
  const alive = useRef(true);
  const [executable, setExecutable] = useState("");
  const [args, setArgs] = useState("[]");
  const [coverage, setCoverage] = useState<RequirementCoverage | null>(null);
  const [requirementIds, setRequirementIds] = useState<string[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [notice, setNotice] = useState("");
  const generation = useRef(0), readGeneration = useRef(0);
  const selectedContext = useRef<string | null>(null);
  const contextKey = (value: RequirementCoverage) => JSON.stringify([value.specification.revision, value.specification.digest, value.specification.status, value.design.revision, value.design.digest, value.design.status]);
  const canLink = coverage?.specification.status === "approved" && coverage.design.status === "approved" && coverage.source.available;
  async function load(replaceCommands = false) {
    const context = generation.current, request = ++readGeneration.current;
    setRefreshing(true);
    try {
      const value = await productApi.checks(projectId);
      if (!alive.current || context !== generation.current || request !== readGeneration.current) return;
      if (value.coverage && value.coverage.projectId !== projectId) throw new Error("Requirement evidence belongs to another project. Refresh the current project.");
      setRuns(current => [
        ...value.runs.map(row => {
          const previous = current.find(run => run.id === row.id);
          return previous && previous.updatedAt > row.updatedAt ? previous : row;
        }),
        ...current.filter(run => !value.runs.some(row => row.id === run.id)),
      ].sort((a, b) => b.createdAt.localeCompare(a.createdAt)));
      if (replaceCommands) setCommands(value.suggested || []);
      if (selectedContext.current && (!value.coverage || contextKey(value.coverage) !== selectedContext.current)) {
        setRequirementIds([]); selectedContext.current = null; setTrusted(false);
        setNotice("Requirements or design changed. Review and select the criteria again.");
      }
      setCoverage(value.coverage ?? null);
    } catch (cause) {
      if (alive.current && context === generation.current && request === readGeneration.current) { setCoverage(null); setError(errorText(cause)); }
    } finally {
      if (alive.current && context === generation.current && request === readGeneration.current) setRefreshing(false);
    }
  }
  const updateRun = (run: CheckRun) =>
    setRuns((current) => [
      run,
      ...current.filter((item) => item.id !== run.id),
    ]);
  useEffect(() => {
    generation.current++;
    alive.current = true;
    setRuns([]); setCommands([]); setCoverage(null); setRequirementIds([]); setTrusted(false); setNotice(""); setError(""); setBusy(false); setExecutable(""); setArgs("[]"); selectedContext.current = null;
    void load(true);
    const unwatch = watchRequirementEvidence(projectId, () => void load());
    return () => {
      alive.current = false;
      generation.current++;
      unwatch();
    };
  }, [projectId]);
  const activeIds = runs
    .filter(active)
    .map((run) => run.id)
    .join(",");
  useEffect(() => {
    if (!activeIds) return;
    let cancelled = false;
    let polling = false;
    const timer = setInterval(() => {
      if (polling) return;
      polling = true;
      void Promise.all(
        activeIds.split(",").map((id) => productApi.checkRun(projectId, id)),
      )
        .then((values) => {
          if (!cancelled) {
            values.forEach(updateRun);
            if (values.some(run => !active(run))) void load();
          }
        })
        .catch((cause) => {
          if (!cancelled) setError(errorText(cause));
        }).finally(() => { polling = false; });
    }, 1500);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [projectId, activeIds]);
  async function operate(action: () => Promise<CheckRun>) {
    const context = generation.current;
    if (working.current === context) return;
    working.current = context;
    setBusy(true);
    setError("");
    try {
      const run = await action();
      if (alive.current && context === generation.current) {
        updateRun(run);
        if (!active(run)) void load();
      }
    } catch (cause) {
      if (alive.current && context === generation.current) setError(errorText(cause));
    } finally {
      if (working.current === context) working.current = null;
      if (alive.current && context === generation.current) setBusy(false);
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
      {notice && <p className="journey-muted" role="status">{notice}</p>}
      <RequirementsEvidence coverage={coverage} runs={runs} refreshing={refreshing} onRefresh={() => void load()} />
      <details className="check-requirements">
        <summary>Link checks to approved requirements · {requirementIds.length} selected</summary>
        <p className="journey-muted">Select the criteria these commands actually exercise. Unselected commands are recorded as generic checks.</p>
        {!canLink && <p className="journey-muted">Approve the current requirements and design, and refresh readable project source, before linking evidence.</p>}
        {(coverage?.rows ?? []).map(row => <label className="requirement-choice" key={row.requirementId}>
          <input type="checkbox" aria-label={`Check ${row.requirementId} · ${row.title}`} checked={requirementIds.includes(row.requirementId)} disabled={busy || !!activeIds || !canLink} onChange={event => {
            setRequirementIds(current => event.target.checked ? [...current, row.requirementId] : current.filter(id => id !== row.requirementId));
            selectedContext.current = coverage ? contextKey(coverage) : null;
            setNotice(""); setTrusted(false);
          }} />
          <span>{row.requirementId} · {row.title}<small>{row.acceptance}</small></span>
        </label>)}
        {!!requirementIds.length && <Button type="button" variant="tertiary" disabled={busy || !!activeIds} onClick={() => { setRequirementIds([]); selectedContext.current = null; setTrusted(false); }}>Clear requirement selection</Button>}
      </details>
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
            <Button type="button" variant="tertiary"
              disabled={busy || !!activeIds}
              aria-label={`Remove ${command.label}`}
              onClick={() => {
                setCommands(commands.filter((_, i) => index !== i));
                setTrusted(false);
              }}
            >
              Remove
            </Button>
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
        <Button type="submit" variant="secondary"
          disabled={busy || !!activeIds || !executable.trim()}
        >
          Add check command
        </Button>
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
      <Button type="button" variant="primary" loading={busy}
        disabled={busy || !!activeIds || !trusted || !commands.length || (!!requirementIds.length && !canLink)}
        onClick={() =>
          void operate(() =>
            requirementIds.length && coverage ? productApi.runChecks(
              projectId,
              commands,
              trusted,
              allowHost ? execution : "docker",
              { requirementIds, expectedSpecificationRevision: coverage.specification.revision, expectedDesignPlanRevision: coverage.design.revision },
            ) : productApi.runChecks(projectId, commands, trusted, allowHost ? execution : "docker"),
          )
        }
      >
        {busy
          ? "Starting checks…"
          : activeIds
            ? "Checks running…"
            : requirementIds.length ? "Run selected requirement checks" : "Run project checks"}
      </Button>
      {!runs.length && (
        <p className="journey-muted">No verification runs recorded yet.</p>
      )}
      <ul className="check-runs">
        {runs.map((run) => (
          <li key={run.id}>
            <div className="journey-header">
              <div>
                <strong>{commandStatus(run.status)}</strong>
                <p>{run.message}</p>
                <p>{evidenceLabel(run.evidenceStatus)}{run.requirementIds?.length ? ` · ${run.requirementIds.join(", ")}` : " · generic commands without requirement links"}</p>
                {run.requirementIds?.length ? <small>Requirements revision {run.specificationRevision ?? 0} · design revision {run.designPlanRevision ?? 0}</small> : null}
                {!!run.staleReasons?.length && <ul>{run.staleReasons.map((reason, index) => <li key={index}>{reason}</li>)}</ul>}
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
                <Button type="button" variant="secondary"
                  disabled={busy}
                  onClick={() =>
                    void operate(() =>
                      productApi.cancelChecks(projectId, run.id),
                    )
                  }
                >
                  Cancel checks
                </Button>
              )}
            </div>
            {!!run.requirementSnapshots?.length && <details><summary>Requirement text recorded with this run</summary><ul>{run.requirementSnapshots.map(requirement => <li key={requirement.id}><strong>{requirement.id} · {requirement.title}</strong><p>{requirement.acceptance}</p></li>)}</ul></details>}
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
