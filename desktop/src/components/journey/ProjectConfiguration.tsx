import { useEffect, useRef, useState } from "react";
import { ApiError } from "../../api/client";
import { Button } from "../fluid/components/button";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import {
  productApi,
  type ConfigurationUpdate,
  type ProjectConfiguration as Configuration,
} from "../../api/product";

interface Props {
  projectId: string;
  disabled?: boolean;
  onBusyChange?(busy: boolean): void;
}

export function ProjectConfiguration(props: Props) {
  const scope = useProjectDraftScope(props.projectId);
  // Private drafts stay in memory and never cross an account/workspace boundary.
  return <ConfigurationFields key={JSON.stringify([productApi.baseUrl, scope, props.projectId])} {...props} />;
}

function ConfigurationFields({
  projectId,
  disabled = false,
  onBusyChange,
}: Props) {
  const access = useProjectAccess(projectId);
  const canConfigure = access.canEdit && access.permissions.secrets;
  const accessEpoch = useRef(0);
  const permission = useRef({ read: access.canRead, write: canConfigure });
  if (permission.current.read && !access.canRead) accessEpoch.current++;
  permission.current = { read: access.canRead, write: canConfigure };
  const busyChanged = useRef(onBusyChange);
  busyChanged.current = onBusyChange;
  const [configuration, setConfiguration] = useState<Configuration | null>(
    null,
  );
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  const [emptyValue, setEmptyValue] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [conflict, setConflict] = useState(false);
  const hasDraft = !!name.trim() || !!value || emptyValue;
  useUnsavedProjectDraft(projectId, hasDraft, "Runtime configuration", "deploy");
  const privateValue = useRef<HTMLInputElement>(null);
  const alive = useRef(true);
  const saving = useRef(false);
  const readGeneration = useRef(0);
  const blocked = !canConfigure || disabled || !!busy || loading || !configuration || conflict;
  const accessMessage = access.loading || access.error || !access.canRead
    ? access.message
    : !canConfigure
      ? "Editing and secrets access are required to change runtime configuration. Ask a team owner to update your project access."
      : "";

  function ownsResult(epoch: number) {
    return alive.current && permission.current.read && accessEpoch.current === epoch;
  }

  async function refresh() {
    if (!alive.current || !permission.current.read || saving.current) return;
    const epoch = accessEpoch.current, generation = ++readGeneration.current;
    const current = () => ownsResult(epoch) && readGeneration.current === generation;
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const metadata = await productApi.configuration(projectId);
      if (current()) {
        setConfiguration(metadata);
        setConflict(false);
      }
    } catch {
      if (current())
        setError(
          "Could not load runtime configuration. Check the workspace connection and refresh stored names.",
        );
    } finally {
      if (current()) setLoading(false);
    }
  }
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      accessEpoch.current++;
      readGeneration.current++;
      busyChanged.current?.(false);
    };
  }, []);
  useEffect(() => {
    if (access.canRead) {
      void refresh();
    } else {
      readGeneration.current++;
      setConfiguration(null);
      setError("");
      setNotice("");
      setLoading(false);
    }
  }, [access.canRead]);

  async function apply(update: ConfigurationUpdate) {
    if (saving.current || !permission.current.write || blocked || !configuration) return;
    saving.current = true;
    const epoch = accessEpoch.current;
    // Any older names read must not replace the confirmed mutation response.
    readGeneration.current++;
    setBusy(update.remove ? "Removing variable" : "Saving configuration");
    busyChanged.current?.(true);
    setError("");
    setNotice("");
    // Keep the value only in the request, never in a saved field or error message.
    setValue("");
    setEmptyValue(false);
    try {
      const metadata = await productApi.saveConfiguration(
        projectId,
        configuration.revision,
        [update],
      );
      if (ownsResult(epoch)) {
        setConfiguration(metadata);
        setName("");
        setNotice(
          update.remove
            ? "Variable removed. The change applies to the next preview, deployment or rollback."
            : "Runtime configuration saved. It applies to the next preview, deployment or rollback.",
        );
      }
    } catch (cause) {
      if (!ownsResult(epoch)) return;
      if (cause instanceof ApiError && cause.status === 409) {
        setConflict(true);
        setError(
          "Configuration changed elsewhere. Refresh stored names, then enter the value and save again.",
        );
      } else {
        setError(
          "Could not save runtime configuration. Check the workspace connection, refresh stored names and try again. Enter private values again before saving.",
        );
      }
    } finally {
      saving.current = false;
      if (alive.current) {
        setBusy("");
        busyChanged.current?.(false);
        // Access may have returned while this request was still in flight. Its
        // old response is retired; read current names under the new grant.
        if (accessEpoch.current !== epoch && permission.current.read) void refresh();
      }
    }
  }
  function submit(event: React.FormEvent) {
    event.preventDefault();
    if (blocked || !permission.current.write) return;
    const variable = name.trim();
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(variable)) {
      setError(
        "Use an environment variable name containing letters, numbers and underscores, beginning with a letter or underscore.",
      );
      return;
    }
    if (!emptyValue && !value) return;
    if (
      /\u0000|\r|\n/.test(value) ||
      new TextEncoder().encode(value).length > 16 * 1024
    ) {
      setError(
        "The value must be a single line of at most 16 KiB without null characters.",
      );
      return;
    }
    if (
      configuration &&
      configuration.variables.length >= 100 &&
      !configuration.variables.some((item) => item.name === variable)
    ) {
      setError(
        "This project already has 100 variables. Remove a variable before adding another.",
      );
      return;
    }
    void apply({ name: variable, value: emptyValue ? "" : value });
  }
  return (
    <section className="project-plan" aria-label="Runtime configuration">
      <div className="journey-header">
        <div>
          <h2>Runtime configuration</h2>
          <p>
            Runtime configuration, stored privately; excluded from project
            backups. Rebuilding old code uses current configuration.
          </p>
        </div>
        {access.canRead && <Button type="button"
          variant="ghost"
          loading={loading}
          disabled={disabled || !!busy || loading}
          onClick={() => void refresh()}
        >
          Refresh stored names
        </Button>}
      </div>
      {accessMessage && <p className="journey-muted" role="status">{accessMessage}</p>}
      {access.canRead && <>
      <p className="journey-muted">
        Set variables such as DATABASE_URL for your running app. Changes apply
        to the next preview, deployment or rollback. This does not provision a
        database or supply secrets during a build.
      </p>
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
      {loading && <p role="status">Loading stored variable names…</p>}
      {configuration && !configuration.variables.length && (
        <p className="journey-muted">No runtime variables saved.</p>
      )}
      {configuration && configuration.variables.length > 0 && (
        <ul className="check-runs" aria-label="Stored runtime variables">
          {configuration.variables.map((variable) => (
            <li className="journey-actions" key={variable.name}>
              <code>{variable.name}</code>
              <Button type="button"
                variant="secondary"
                disabled={blocked || hasDraft}
                aria-label={`Update ${variable.name}`}
                onClick={() => {
                  setName(variable.name);
                  setValue("");
                  setEmptyValue(false);
                  setError("");
                  setNotice("");
                  privateValue.current?.focus();
                }}
              >
                Update
              </Button>
              <Button type="button"
                variant="secondary"
                disabled={blocked || hasDraft}
                aria-label={`Remove ${variable.name}`}
                onClick={() =>
                  void apply({ name: variable.name, remove: true })
                }
              >
                Remove
              </Button>
            </li>
          ))}
        </ul>
      )}
      <form className="journey-form" onSubmit={submit} autoComplete="off">
        <div className="setup-field-grid">
          <label className="luci-field">
            Variable name
            <input
              required
              disabled={blocked}
              maxLength={128}
              autoComplete="off"
              spellCheck={false}
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="DATABASE_URL"
            />
          </label>
          <label className="luci-field">
            Private value
            <input
              ref={privateValue}
              type="password"
              disabled={blocked || emptyValue}
              autoComplete="new-password"
              spellCheck={false}
              value={value}
              onChange={(event) => setValue(event.target.value)}
              placeholder="Enter a new or replacement value"
            />
            <small>
              Saved values are never displayed. Entering an existing name
              replaces its value.
            </small>
          </label>
        </div>
        <label className="setup-check">
          <input
            type="checkbox"
            disabled={blocked}
            checked={emptyValue}
            onChange={(event) => {
              setEmptyValue(event.target.checked);
              setValue("");
            }}
          />
          Set an empty value
        </label>
        <div className="journey-actions">
          <Button type="submit"
            variant="secondary"
            loading={!!busy}
            disabled={blocked || !name.trim() || (!emptyValue && !value)}
          >
            {busy || "Save runtime variable"}
          </Button>
          {hasDraft && <Button type="button" variant="ghost" disabled={disabled || !!busy || loading} onClick={() => { setName(""); setValue(""); setEmptyValue(false); setError(""); setNotice("Variable draft discarded. Saved configuration is unchanged."); }}>Discard variable draft</Button>}
        </div>
      </form>
      </>}
    </section>
  );
}
