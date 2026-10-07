import { Button } from "../fluid/components/button";
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
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [bootstrapToken, setBootstrapToken] = useState("");
  const [inviteCode, setInviteCode] = useState("");
  const [mode, setMode] = useState<"sign-in" | "create" | "help">("sign-in");
  const [showPassword, setShowPassword] = useState(false);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const alive = useRef(true),
    working = useRef(false);
  const create = status?.bootstrapRequired || mode === "create";
  const received = (value: WorkspaceAuth) => {
    if (alive.current) {
      setStatus(value);
      if (value.authenticated || value.bootstrapRequired) setMode("sign-in");
      onAuthenticated(value.authenticated);
    }
  };
  useEffect(() => {
    alive.current = true;
    let cancelled = false;
    setLoading(true);
    void client
      .workspaceAuth(connectionId)
      .then((value) => {
        if (!cancelled) received(value);
      })
      .catch((cause) => {
        if (!cancelled) setError(errorText(cause));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      alive.current = false;
      cancelled = true;
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
        setConfirm("");
        setBootstrapToken("");
        setInviteCode("");
        setShowPassword(false);
      }
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy(false);
    }
  }
  function changeMode(next: typeof mode) {
    setMode(next);
    setError("");
    setPassword("");
    setConfirm("");
    setInviteCode("");
    setShowPassword(false);
  }
  return (
    <section className="workspace-signin">
      <h3>
        {!status
          ? "Connect to your workspace account"
          : status.bootstrapRequired || mode === "create"
            ? "Create your workspace account"
            : mode === "help"
              ? "Recover your workspace sign-in"
              : "Sign in to your workspace"}
      </h3>
      <p className="journey-muted">
        Your credentials travel through the verified SSH connection. This is
        your separate account on the connected workspace, not your account on
        this computer.
      </p>
      {loading && <p role="status">Checking the workspace account…</p>}
      {busy && <p role="status">Updating workspace sign-in…</p>}
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
      {status && !status.authenticated && mode === "help" && (
        <p className="journey-muted">
          Ask this workspace’s administrator to confirm your username or reset
          your password. To join as a new person, ask for an invitation. Your
          local owner recovery cannot change a remote account.
        </p>
      )}
      {status && !status.authenticated && mode !== "help" && (
        <form
          className="journey-form"
          aria-busy={busy}
          onSubmit={(event) => {
            event.preventDefault();
            if (create && password !== confirm) {
              setError(
                "The passwords do not match. Enter the same password in both fields.",
              );
              return;
            }
            void run(() =>
              client.signInWorkspace({
                connectionId,
                username,
                password,
                ...(create ? { displayName } : {}),
                ...(status.bootstrapRequired
                  ? bootstrapToken
                    ? { bootstrapToken }
                    : {}
                  : mode === "create"
                    ? { inviteCode }
                    : {}),
              }),
            );
          }}
        >
          {create && (
            <>
              <p className="journey-muted">
                {status.bootstrapRequired
                  ? "Create the first account for this workspace. It will manage the workspace and its people."
                  : "Use the invitation code from this workspace’s administrator. Choose your own username and password."}
              </p>
              {status.bootstrapRequired && !status.bootstrapTokenAvailable && (
                <label className="luci-field">
                  <span id={`remote-bootstrap-${connectionId}`}>
                    Remote bootstrap token
                  </span>
                  <input
                    aria-labelledby={`remote-bootstrap-${connectionId}`}
                    aria-describedby={`remote-bootstrap-help-${connectionId}`}
                    required
                    disabled={busy}
                    type="password"
                    autoComplete="off"
                    value={bootstrapToken}
                    onChange={(event) => setBootstrapToken(event.target.value)}
                  />
                  <small id={`remote-bootstrap-help-${connectionId}`}>
                    Ask the server administrator for its one-time setup code.
                    Luci could not read it through this SSH account.
                  </small>
                </label>
              )}
              {!status.bootstrapRequired && (
                <label className="luci-field">
                  <span id={`remote-invite-${connectionId}`}>
                    Remote invitation code
                  </span>
                  <input
                    aria-labelledby={`remote-invite-${connectionId}`}
                    aria-describedby={`remote-invite-help-${connectionId}`}
                    required
                    disabled={busy}
                    type="password"
                    autoComplete="off"
                    value={inviteCode}
                    onChange={(event) => setInviteCode(event.target.value)}
                  />
                  <small id={`remote-invite-help-${connectionId}`}>
                    Codes expire after 48 hours and work once. Ask the
                    administrator for a replacement if needed.
                  </small>
                </label>
              )}
              <label className="luci-field">
                Remote display name
                <input
                  required
                  disabled={busy}
                  maxLength={120}
                  autoComplete="name"
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                />
              </label>
            </>
          )}
          <div className="setup-field-grid">
            <label className="luci-field">
              <span id={`remote-username-${connectionId}`}>
                Remote username
              </span>
              <input
                required
                disabled={busy}
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                minLength={create ? 3 : undefined}
                maxLength={64}
                pattern={
                  create ? "[A-Za-z0-9][A-Za-z0-9_.\\-]{2,63}" : undefined
                }
                aria-labelledby={`remote-username-${connectionId}`}
                value={username}
                onChange={(event) => setUsername(event.target.value)}
              />
              {create && (
                <small>
                  3–64 characters. Letters, numbers, dots, underscores and
                  hyphens.
                </small>
              )}
            </label>
            <label className="luci-field">
              <span id={`remote-password-${connectionId}`}>
                Remote password
              </span>
              <input
                required
                disabled={busy}
                type={showPassword ? "text" : "password"}
                minLength={create ? 12 : undefined}
                maxLength={1024}
                aria-labelledby={`remote-password-${connectionId}`}
                autoComplete={create ? "new-password" : "current-password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
              {create && <small>Use at least 12 characters.</small>}
            </label>
            {create && (
              <label className="luci-field">
                Confirm remote password
                <input
                  required
                  disabled={busy}
                  type={showPassword ? "text" : "password"}
                  autoComplete="new-password"
                  value={confirm}
                  onChange={(event) => setConfirm(event.target.value)}
                />
              </label>
            )}
          </div>
          <label className="setup-check">
            <input
              type="checkbox"
              disabled={busy}
              checked={showPassword}
              onChange={(event) => setShowPassword(event.target.checked)}
            />
            Show remote password{create ? "s" : ""}
          </label>
          <Button type="submit" variant="secondary" disabled={busy}>
            {busy
              ? "Connecting…"
              : create
                ? "Create workspace account"
                : "Sign in to workspace"}
          </Button>
        </form>
      )}
      <div className="journey-actions">
        {status && !status.authenticated && !status.bootstrapRequired && (
          <>
            {mode !== "sign-in" && (
              <Button type="button"
                variant="secondary"
                disabled={busy}
                onClick={() => changeMode("sign-in")}
              >
                Back to workspace sign-in
              </Button>
            )}
            {mode !== "create" && (
              <Button type="button"
                variant="secondary"
                disabled={busy}
                onClick={() => changeMode("create")}
              >
                Join with an invitation
              </Button>
            )}
            {mode !== "help" && (
              <Button type="button"
                variant="secondary"
                disabled={busy}
                onClick={() => changeMode("help")}
              >
                Forgot workspace sign-in?
              </Button>
            )}
          </>
        )}
        <Button type="button"
          variant="secondary"
          disabled={busy || loading}
          onClick={() => void run(() => client.workspaceAuth(connectionId))}
        >
          Check workspace sign-in
        </Button>
        {status?.authenticated && (
          <Button type="button"
            variant="secondary"
            disabled={busy}
            onClick={() =>
              void run(() => client.disconnectWorkspaceAuth(connectionId))
            }
          >
            Disconnect remote sign-in
          </Button>
        )}
      </div>
    </section>
  );
}
