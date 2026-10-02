import { useEffect, useRef, useState } from "react";
import {
  errorText,
  type AgentProvider,
  type ProductClient,
} from "../../api/product";
import { api } from "../../api";
import { useCockpit } from "../../store/store";

export function AgentConnections({
  client,
  cloud,
  onSessionStarted,
}: {
  client: ProductClient;
  cloud: boolean;
  onSessionStarted?(): void;
}) {
  const worktreeId = useCockpit((state) => state.selection.worktreeId);
  const codex = useCockpit((state) =>
    state.harnesses.find((harness) => harness.id === "codex"),
  );
  const [providers, setProviders] = useState<AgentProvider[]>([]);
  const [providerId, setProviderId] = useState("anthropic");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("http://localhost:11434/v1");
  const [model, setModel] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const working = useRef(false),
    alive = useRef(true);
  const provider = providers.find((entry) => entry.id === providerId);
  useEffect(() => {
    alive.current = true;
    void client
      .agents()
      .then((result) => {
        if (alive.current) setProviders(result.providers);
      })
      .catch((cause) => {
        if (alive.current) setError(errorText(cause));
      });
    return () => {
      alive.current = false;
    };
  }, [client]);
  async function save(clear = false) {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const value =
        providerId === "anthropic"
          ? await client.saveAnthropicKey(clear ? "" : apiKey)
          : providerId === "openai"
            ? await client.saveOpenAIKey(clear ? "" : apiKey)
            : await client.saveLocalAgent(
                clear ? "" : baseUrl.trim(),
                clear ? "" : model.trim(),
                clear ? undefined : apiKey,
              );
      if (alive.current) {
        setProviders(value.providers);
        setApiKey("");
        setNotice(
          clear
            ? "Agent connection removed from Luci. Existing sessions are unchanged."
            : "Your agent connection is saved for new sessions. The provider has not been validated by saving these settings.",
        );
      }
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  async function signIn() {
    if (working.current || !worktreeId) return;
    working.current = true;
    setBusy(true);
    setError("");
    try {
      const session = await api.createSession({
        worktreeId,
        harness: "codex",
        authOnly: true,
      });
      const state = useCockpit.getState();
      state.upsertSession(session);
      state.selectWorktree(worktreeId);
      state.setLens("develop");
      state.focusSession(session.id);
      onSessionStarted?.();
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="setup-plan">
      <h2>Your agent connection</h2>
      <label className="luci-field">
        Coding agent
        <select
          value={providerId}
          disabled={busy}
          onChange={(event) => {
            const id = event.target.value,
              entry = providers.find((item) => item.id === id);
            setProviderId(id);
            setApiKey("");
            setError("");
            setNotice("");
            setBaseUrl(entry?.baseUrl || "http://localhost:11434/v1");
            setModel(entry?.model || "");
          }}
        >
          <option value="anthropic">Claude Code · Anthropic</option>
          <option value="openai">Codex · OpenAI</option>
          <option value="local">OpenCode · your model endpoint</option>
        </select>
      </label>
      <p className="journey-muted">
        {providerId === "anthropic"
          ? "Use your own Anthropic API key for agent sessions. API usage is billed by Anthropic separately from a Claude subscription. You can also sign in through Claude Code in a session."
          : providerId === "openai"
            ? "Use your own OpenAI API key, or start Codex and use its official sign-in flow. API billing and subscription access are separate; model usage belongs to your account."
            : "Connect an OpenAI-compatible endpoint you control. Choose a model already available there. Luci will not switch to a paid provider if it is unavailable. Localhost is resolved from the active execution workspace."}
      </p>
      {providerId === "openai" && (
        <div className="setup-plan">
          <button
            className="luci-button"
            disabled={busy || !worktreeId || !codex?.available}
            onClick={() => void signIn()}
          >
            Open Codex account sign-in
          </button>
          <p className="journey-muted">
            {!worktreeId ? "Select a project worktree first. " : ""}
            {codex && !codex.available
              ? `${codex.note || "Codex is unavailable in this runtime."} `
              : ""}
            The terminal displays Codex’s official device sign-in. After it
            exits, restart that session to begin work with the saved account.
            Credentials stay outside project Git.
          </p>
        </div>
      )}
      <p role="status">
        {provider?.configured
          ? providerId === "local"
            ? `Endpoint saved for ${provider.model || "your model"}.`
            : cloud
              ? "Personal API key saved in the cloud workspace."
              : "Personal API key saved on this machine."
          : providerId === "local"
            ? "No model endpoint saved."
            : "No personal API key saved."}
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
      <form
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        {providerId === "local" && (
          <div className="setup-field-grid">
            <label className="luci-field">
              Model endpoint URL
              <input
                required
                type="url"
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
              />
            </label>
            <label className="luci-field">
              Installed model ID
              <input
                required
                value={model}
                onChange={(event) => setModel(event.target.value)}
                placeholder="The model ID served by your endpoint"
              />
            </label>
          </div>
        )}
        <label className="luci-field">
          {providerId === "anthropic"
            ? "Anthropic API key"
            : providerId === "openai"
              ? "OpenAI API key"
              : "Endpoint API key · optional"}
          <input
            type="password"
            autoComplete="off"
            required={providerId !== "local"}
            value={apiKey}
            onChange={(event) => setApiKey(event.target.value)}
          />
          <small>
            The key stays in the selected workspace service and is supplied only
            to the selected coding agent.
          </small>
        </label>
        <div className="setup-actions">
          <button
            className="luci-button"
            disabled={
              busy ||
              (providerId === "local"
                ? !baseUrl.trim() || !model.trim()
                : !apiKey)
            }
          >
            {busy ? "Saving connection…" : "Save agent connection"}
          </button>
          {provider?.configured && (
            <button
              type="button"
              className="luci-button"
              disabled={busy}
              onClick={() => void save(true)}
            >
              {providerId === "local" ? "Remove endpoint" : "Remove saved key"}
            </button>
          )}
        </div>
      </form>
    </section>
  );
}
