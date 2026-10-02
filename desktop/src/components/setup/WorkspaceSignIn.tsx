import { useEffect, useRef, useState } from "react";
import {
  errorText,
  type ProductClient,
  type WorkspaceAuth,
} from "../../api/product";

export function WorkspaceSignIn({
  client,
  connectionId,
  onAuthenticated,
}: {
  client: ProductClient;
  connectionId: string;
  onAuthenticated(value: boolean): void;
}) {
  const [status, setStatus] = useState<WorkspaceAuth | null>(null);
  const [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [displayName, setDisplayName] = useState(""),
    [bootstrapToken, setBootstrapToken] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const alive = useRef(true),
    working = useRef(false);
  const received = (value: WorkspaceAuth) => {
    if (alive.current) {
      setStatus(value);
      onAuthenticated(value.authenticated);
    }
  };
  async function check() {
    received(await client.workspaceAuth(connectionId));
  }
  useEffect(() => {
    alive.current = true;
    void check().catch((cause) => {
      if (alive.current) setError(errorText(cause));
    });
    return () => {
      alive.current = false;
    };
  }, [client, connectionId]);
  async function run(action: () => Promise<WorkspaceAuth>) {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError("");
    try {
      received(await action());
      if (alive.current) {
        setPassword("");
        setBootstrapToken("");
      }
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="workspace-signin">
      <h3>Sign in to the cloud control plane</h3>
      <p className="journey-muted">
        Credentials travel through the verified SSH connection. This sign-in is
        separate from your local Luci account.
      </p>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {status && (
        <p role="status">
          {status.message}
          {status.authenticated && status.user
            ? ` · ${status.user.displayName || status.user.username} (${status.user.role})`
            : ""}
        </p>
      )}
      {status && !status.authenticated && (
        <form
          className="journey-form"
          onSubmit={(event) => {
            event.preventDefault();
            void run(() =>
              client.signInWorkspace({
                connectionId,
                username,
                password,
                ...(status.bootstrapRequired
                  ? {
                      displayName,
                      ...(bootstrapToken ? { bootstrapToken } : {}),
                    }
                  : {}),
              }),
            );
          }}
        >
          {status.bootstrapRequired && (
            <>
              <label className="luci-field">
                Remote display name
                <input
                  required
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                />
              </label>
              {!status.bootstrapTokenAvailable && (
                <label className="luci-field">
                  Remote bootstrap token
                  <input
                    required
                    type="password"
                    autoComplete="off"
                    value={bootstrapToken}
                    onChange={(event) => setBootstrapToken(event.target.value)}
                  />
                </label>
              )}
            </>
          )}
          <div className="setup-field-grid">
            <label className="luci-field">
              Remote username
              <input
                required
                autoComplete="off"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
              />
            </label>
            <label className="luci-field">
              Remote password
              <input
                required
                type="password"
                autoComplete={
                  status.bootstrapRequired ? "new-password" : "current-password"
                }
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>
          </div>
          <button className="luci-button" disabled={busy}>
            {busy
              ? "Connecting…"
              : status.bootstrapRequired
                ? "Create remote Super Admin"
                : "Sign in to workspace"}
          </button>
        </form>
      )}
      <div className="journey-actions">
        <button
          className="luci-button"
          disabled={busy}
          onClick={() => void run(() => client.workspaceAuth(connectionId))}
        >
          Check workspace sign-in
        </button>
        {status?.authenticated && (
          <button
            className="luci-button"
            disabled={busy}
            onClick={() =>
              void run(() => client.disconnectWorkspaceAuth(connectionId))
            }
          >
            Disconnect remote sign-in
          </button>
        )}
      </div>
    </section>
  );
}
