import { Button } from "../fluid/components/button";
import { useEffect, useRef, useState } from "react";
import {
  errorText,
  type AgentProvider,
  type CodexAccountConnection,
  type ProductClient,
} from "../../api/product";
import { useCockpit } from "../../store/store";
import { OWN_AGENT_PROVIDER_CHANGED } from "../../hooks/useAgentConnections";
import { AgentConnectionManager } from "./AgentConnectionManager";
import { codexConnectionError } from "../../api/codexConnectionErrors";

export function AgentConnections({
  client,
  cloud,
  onSessionStarted,
  initialProvider = "anthropic",
}: {
  client: ProductClient;
  cloud: boolean;
  onSessionStarted?(): void;
  initialProvider?: string;
}) {
  const codex = useCockpit((state) =>
    state.harnesses.find((harness) => harness.id === "codex"),
  );
  const sessions = useCockpit((state) => state.sessions);
  const signInSessions = Object.values(sessions)
    .filter(
      (session) =>
        session.harness === "codex" &&
        session.authOnly &&
        session.status === "exited",
    )
    .sort((left, right) => right.createdAt.localeCompare(left.createdAt));
  const [providers, setProviders] = useState<AgentProvider[]>([]);
  const [providerId, setProviderId] = useState(initialProvider);
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("http://localhost:11434/v1");
  const [model, setModel] = useState("");
  const [account, setAccount] = useState<CodexAccountConnection | null>(null);
  const [accountUnavailable, setAccountUnavailable] = useState(false);
  const [signInSessionId, setSignInSessionId] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const working = useRef(false),
    alive = useRef(true);
  const provider = providers.find((entry) => entry.id === providerId);
  const keyLabel =
    providerId === "anthropic"
      ? "Anthropic API key"
      : providerId === "openai"
        ? "OpenAI API key"
        : "Endpoint API key · optional";
  useEffect(() => {
    alive.current = true;
    // Activity can reconnect after an existing request settled while hidden.
    // Reflect that operation owner; do not start or replay a mutation.
    setBusy(working.current);
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
  useEffect(() => {
    if (providerId !== "openai") return;
    let cancelled = false;
    setAccount(null);
    setAccountUnavailable(false);
    void client
      .codexAccount()
      .then((value) => {
        if (!cancelled) { setAccount(value); setAccountUnavailable(false); }
      })
      .catch((cause) => {
        if (!cancelled) { setAccountUnavailable(true); setError(codexConnectionError(cause, "status")); }
      });
    return () => {
      cancelled = true;
    };
  }, [client, providerId]);
  const selectedSignIn =
    signInSessions.find((session) => session.id === signInSessionId) ??
    signInSessions[0];
  const pendingSignIn = Object.values(sessions).find(
    (session) =>
      session.harness === "codex" &&
      session.authOnly &&
      session.status !== "exited",
  );
  const openSignIn = (sessionId: string) => {
    const state = useCockpit.getState();
    state.selectProject(null);
    state.setLens("develop");
    state.focusSession(sessionId);
    onSessionStarted?.();
  };
  async function updateAccount(action: "connect" | "disconnect" | "refresh") {
    if (working.current || (action === "connect" && !selectedSignIn)) return;
    working.current = true;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const value =
        action === "connect"
          ? await client.connectCodexAccount(selectedSignIn!.id)
          : action === "disconnect"
            ? await client.disconnectCodexAccount()
            : await client.codexAccount();
      if (action === "connect" && !value.configured)
        throw new Error("Saving this sign-in was not confirmed. Keep the session and refresh Codex connection before trying Save again.");
      if (alive.current) {
        setAccount(value);
        setAccountUnavailable(false);
        if (action !== "refresh")
          setNotice(
            action === "connect"
              ? "Codex sign-in is saved for new sessions in this workspace. A completed model request is still needed to verify access."
              : "Codex sign-in is disconnected for new sessions. Existing sessions keep their private sign-in. Any saved API key remains available separately.",
          );
      }
    } catch (cause) {
      if (alive.current) {
        setAccountUnavailable(true);
        setError(codexConnectionError(cause, action === "connect" ? "save" : "status"));
      }
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  async function save(clear = false) {
    if (
      working.current ||
      (providerId === "openai" && account?.configured && !clear)
    )
      return;
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
      if (providerId === "local") window.dispatchEvent(new Event(OWN_AGENT_PROVIDER_CHANGED));
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
    if (working.current || pendingSignIn) return;
    working.current = true;
    setBusy(true);
    setError("");
    try {
      const session = await client.startCodexSignIn();
      const state = useCockpit.getState();
      state.upsertSession(session);
      openSignIn(session.id);
    } catch (cause) {
      if (alive.current) setError(codexConnectionError(cause, "start"));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <>
      <AgentConnectionManager client={client} />
    <section className="setup-plan">
      <h2>Your own agent accounts</h2>
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
            ? "Connect your ChatGPT subscription through Codex’s official sign-in, or use separately billed OpenAI API credits. Model usage belongs to your account."
            : "Connect an OpenAI-compatible endpoint you control. Choose a model already available there. Luci will not switch to a paid provider if it is unavailable. Localhost is resolved from the active execution workspace."}
      </p>
      {providerId === "openai" && (
        <div className="setup-plan">
          <h3>ChatGPT subscription</h3>
          <p role="status">
            {accountUnavailable
              ? "Codex connection status is unavailable."
              : account?.configured
              ? cloud
                ? "Codex sign-in saved in your cloud workspace."
                : "Codex sign-in saved on this machine."
              : account
                ? "No Codex subscription sign-in saved."
                : "Checking Codex sign-in…"}
          </p>
          <Button type="button"
            variant="secondary"
            disabled={
              busy ||
              !!account?.activeSessionId ||
              !!pendingSignIn ||
              !codex?.available
            }
            onClick={() => void signIn()}
          >
            Open Codex account sign-in
          </Button>
          <p className="journey-muted">
            {codex && !codex.available
              ? `${codex.note || "Codex is unavailable in this runtime."} `
              : ""}
            The sign-in panel provides Copy code and Open OpenAI sign-in outside
            the terminal. Complete OpenAI’s page, then explicitly Save Codex
            sign-in. If interrupted, choose Start new device sign-in there. New implementation
            sessions will use that subscription, even if an API key is saved.
            Credentials stay outside project Git and are private to your Luci
            account.
          </p>
          {pendingSignIn && (
            <Button
              variant="secondary"
              type="button"
              disabled={busy}
              onClick={() => openSignIn(pendingSignIn.id)}
            >
              Resume Codex sign-in
            </Button>
          )}
          {signInSessions.length > 0 && (
            <>
              <label className="luci-field">
                Ended sign-in session
                <select
                  value={selectedSignIn?.id ?? ""}
                  disabled={busy}
                  onChange={(event) => setSignInSessionId(event.target.value)}
                >
                  {signInSessions.map((session) => (
                    <option key={session.id} value={session.id}>
                      {session.title || "Codex sign-in"} · Started {session.createdAt}
                    </option>
                  ))}
                </select>
              </label>
              <p className="journey-muted">
                An ended session does not confirm that authorization completed.
                Save validates and retains the sign-in for this workspace.
              </p>
              <Button type="button"
                variant="secondary"
                disabled={busy || !!account?.activeSessionId}
                onClick={() => void updateAccount("connect")}
              >
                Save Codex sign-in
              </Button>
            </>
          )}
          {account?.activeSessionId && (
            <p className="journey-muted">
              A Codex subscription session is active. Stop it before connecting
              another sign-in or starting another subscription session.
            </p>
          )}
          <div className="setup-actions">
            <Button type="button"
              variant="secondary"
              disabled={busy}
              onClick={() => void updateAccount("refresh")}
            >
              Refresh Codex connection
            </Button>
            {account?.configured && (
              <Button type="button"
                variant="secondary"
                disabled={busy}
                onClick={() => void updateAccount("disconnect")}
              >
                Disconnect Codex sign-in
              </Button>
            )}
          </div>
        </div>
      )}
      {providerId === "openai" && <h3>OpenAI API credits</h3>}
      {providerId === "openai" && account?.configured && !accountUnavailable && (
        <p className="journey-muted">
          Your subscription is selected for new Codex sessions. Disconnect it
          first to use API credits. Removing a saved API key does not disconnect
          your subscription.
        </p>
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
          {keyLabel}
          <input
            aria-label={keyLabel}
            type="password"
            autoComplete="off"
            required={providerId !== "local"}
            disabled={
              busy || (providerId === "openai" && !!account?.configured)
            }
            value={apiKey}
            onChange={(event) => setApiKey(event.target.value)}
          />
          <small>
            The key stays in the selected workspace service and is supplied only
            to the selected coding agent.
          </small>
        </label>
        <div className="setup-actions">
          <Button type="submit"
            variant="secondary"
            disabled={
              busy ||
              (providerId === "openai" && !!account?.configured) ||
              (providerId === "local"
                ? !baseUrl.trim() || !model.trim()
                : !apiKey)
            }
          >
            {busy ? "Saving connection…" : "Save agent connection"}
          </Button>
          {provider?.configured && (
            <Button
              type="button"
              variant="secondary"
              disabled={busy}
              onClick={() => void save(true)}
            >
              {providerId === "local" ? "Remove endpoint" : "Remove saved key"}
            </Button>
          )}
        </div>
      </form>
    </section>
    </>
  );
}
