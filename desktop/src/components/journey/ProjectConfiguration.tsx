import { useEffect, useRef, useState } from "react";
import { ApiError } from "../../api/client";
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
  // A project change discards private drafts and pending results from its predecessor.
  return <ConfigurationFields key={props.projectId} {...props} />;
}

function ConfigurationFields({
  projectId,
  disabled = false,
  onBusyChange,
}: Props) {
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
  const privateValue = useRef<HTMLInputElement>(null);
  const alive = useRef(true),
    working = useRef(false);
  const blocked = disabled || !!busy || loading || !configuration || conflict;

  async function refresh() {
    if (working.current) return;
    working.current = true;
    setLoading(true);
    setError("");
    setNotice("");
    setValue("");
    setEmptyValue(false);
    try {
      const metadata = await productApi.configuration(projectId);
      if (alive.current) {
        setConfiguration(metadata);
        setConflict(false);
      }
    } catch {
      if (alive.current)
        setError(
          "Could not load runtime configuration. Check the workspace connection and refresh stored names.",
        );
    } finally {
      working.current = false;
      if (alive.current) setLoading(false);
    }
  }
  useEffect(() => {
    alive.current = true;
    void refresh();
    return () => {
      alive.current = false;
      onBusyChange?.(false);
    };
  }, [projectId, onBusyChange]);

  async function apply(update: ConfigurationUpdate) {
    if (working.current || blocked || !configuration) return;
    working.current = true;
    setBusy(update.remove ? "Removing variable" : "Saving configuration");
    onBusyChange?.(true);
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
      if (alive.current) {
        setConfiguration(metadata);
        setName("");
        setNotice(
          update.remove
            ? "Variable removed. The change applies to the next preview, deployment or rollback."
            : "Runtime configuration saved. It applies to the next preview, deployment or rollback.",
        );
      }
    } catch (cause) {
      if (!alive.current) return;
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
      working.current = false;
      if (alive.current) {
        setBusy("");
        onBusyChange?.(false);
      }
    }
  }
  function submit(event: React.FormEvent) {
    event.preventDefault();
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
        <button
          type="button"
          className="luci-button"
          disabled={disabled || !!busy || loading}
          onClick={() => void refresh()}
        >
          Refresh stored names
        </button>
      </div>
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
              <button
                type="button"
                className="luci-button"
                disabled={blocked}
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
              </button>
              <button
                type="button"
                className="luci-button"
                disabled={blocked}
                aria-label={`Remove ${variable.name}`}
                onClick={() =>
                  void apply({ name: variable.name, remove: true })
                }
              >
                Remove
              </button>
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
          <button
            className="luci-button"
            disabled={blocked || !name.trim() || (!emptyValue && !value)}
          >
            {busy || "Save runtime variable"}
          </button>
        </div>
      </form>
    </section>
  );
}
