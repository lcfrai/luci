import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button } from "../fluid/components/button";
import type { ControlUser } from "./AuthProvider";

export interface ControlStatus {
  configured: boolean;
  instanceName: string;
  backing: string;
  bootstrapRequired: boolean;
  bootstrapTokenPath?: string;
}
export interface LocalAccess {
  available: boolean;
  owner?: ControlUser | null;
  transport?: "native" | "portable";
}
export type EntryMode = "sign-in" | "create" | "recover";
export interface EntryDraft {
  username: string;
  displayName: string;
  password: string;
  inviteCode: string;
}

export function AuthEntry({
  status,
  local,
  loading,
  busy,
  error,
  connectionError,
  localError,
  portableLocal,
  mode,
  bootstrapToken,
  onBootstrapToken,
  onMode,
  onRetry,
  onSubmit,
}: {
  status: ControlStatus | null;
  local: LocalAccess;
  loading: boolean;
  busy: boolean;
  error: string;
  connectionError: boolean;
  localError: string;
  portableLocal: boolean;
  mode: EntryMode;
  bootstrapToken: string;
  onBootstrapToken(value: string): void;
  onMode(value: EntryMode): void;
  onRetry(): void;
  onSubmit(value: EntryDraft): Promise<void>;
}) {
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [inviteCode, setInviteCode] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [validation, setValidation] = useState("");
  const [bootstrapEdited, setBootstrapEdited] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const fresh = status?.bootstrapRequired === true;
  const create = fresh || mode === "create";
  const recover = !fresh && mode === "recover";
  const newPassword = create || recover;
  const recoverable = local.available && !!local.owner;
  const formAvailable =
    !!status &&
    !loading &&
    (!recover || recoverable) &&
    !(localError && create && !fresh && !local.available);
  useEffect(() => {
    setPassword("");
    setConfirm("");
    setInviteCode("");
    setShowPassword(false);
    setValidation("");
  }, [mode]);
  useEffect(() => {
    if (!loading && (recover || !status)) heading.current?.focus();
  }, [mode, loading]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    if (newPassword && password !== confirm) {
      setValidation(
        "The passwords do not match. Re-enter the same password in both fields.",
      );
      return;
    }
    setValidation("");
    await onSubmit({ username, displayName, password, inviteCode });
  }
  return (
    <main className="auth-screen">
      <section className="auth-card" aria-labelledby="auth-title">
        <h1 id="auth-title" ref={heading} tabIndex={-1}>
          {!status
            ? "Connect to Luci."
            : fresh
              ? "Create your Luci account."
              : recover
                ? "Recover your sign-in."
                : create
                  ? "Create an account."
                  : "Sign in to Luci."}
        </h1>
        <p className="auth-workspace journey-muted">
          {status?.instanceName || "Luci"}
          {local.available ? " · This computer" : ""}
        </p>
        {status && (
          <p className="journey-muted">
            {fresh
              ? "Your first account owns this workspace. You will then choose this computer, your own cloud, or an existing team workspace. No lcfr.ai account is needed."
              : recover
                ? recoverable
                  ? `Reset the password for ${local.owner!.displayName || local.owner!.username} (${local.owner!.username}). This keeps the original account, projects and settings. Other sign-ins for this account will end.`
                  : "Ask this workspace’s administrator to confirm your username or reset your password. Your account belongs to this workspace; there is no central lcfr.ai password reset."
                : create
                  ? local.available
                    ? "Create a separate account on this computer, with its own projects and administrator access. Existing accounts and their projects stay intact. To return to existing projects, recover the original account instead."
                    : "Use an invitation from this workspace’s administrator and choose your own sign-in. An invitation is needed because this workspace already has an owner."
                  : "Use the account created for this workspace. If this is a new installation, Luci may have found a workspace saved by an earlier installation."}
          </p>
        )}
      {status && !fresh && !local.available && portableLocal && mode !== "sign-in" && (
        <p className="journey-muted">
          If this is your portable copy, reopen Start-Luci.ps1 (Windows) or start-luci.sh (macOS/Linux) on this computer. Its private local access link lets you create an account or recover the original owner without an invitation.
        </p>
      )}
        {status &&
          !fresh &&
          mode === "sign-in" &&
          local.available &&
          local.owner && (
            <p className="journey-muted">
              Saved owner:{" "}
              <strong>{local.owner.displayName || local.owner.username}</strong>{" "}
              · {local.owner.username}. Sign in to this account to return to its
              projects.
            </p>
          )}
        {loading && <p role="status">Connecting to your workspace…</p>}
        {(validation || error) && (
          <p className="journey-error" role="alert">
            {validation || error}
          </p>
        )}
        {localError && (
          <div className="auth-help">
            <p className="journey-warning">
              Local account access is unavailable. {localError}
            </p>
            <Button type="button"
              variant="secondary"
              disabled={busy || loading}
              onClick={onRetry}
            >
              Retry local account access
            </Button>
          </div>
        )}
        {formAvailable && (
          <form
            className="journey-form"
            aria-busy={busy}
            onSubmit={(event) => void submit(event)}
          >
            {fresh && (
              <>
                <details
                  className="auth-bootstrap"
                  open={!bootstrapToken || bootstrapEdited || undefined}
                >
                  <summary>
                    {bootstrapToken
                      ? "This workspace is ready for first setup"
                      : "Get the one-time setup code"}
                  </summary>
                  <label className="luci-field">
                    <span id="auth-bootstrap-label">Bootstrap token</span>
                    <input
                      type="password"
                      autoComplete="off"
                      required
                      disabled={busy}
                      autoFocus={!bootstrapToken}
                      aria-labelledby="auth-bootstrap-label"
                      aria-describedby="auth-bootstrap-help"
                      value={bootstrapToken}
                      onChange={(event) => {
                        setBootstrapEdited(true);
                        onBootstrapToken(event.target.value);
                      }}
                    />
                    <small id="auth-bootstrap-help">
                      {bootstrapToken
                        ? "Loaded from your local launcher, or entered by you. It is used only to create the first account."
                        : "For a portable copy, open Start-Luci.ps1 (Windows) or start-luci.sh (macOS/Linux) from the extracted package. Its browser link fills this code for you. If you manage this service, you can also copy its one-time code below; otherwise ask its administrator."}
                    </small>
                  </label>
                  {!bootstrapToken && status?.bootstrapTokenPath && (
                    <p className="auth-path">
                      <code>{status.bootstrapTokenPath}</code>
                    </p>
                  )}
                  {!bootstrapToken && (
                    <Button
                      variant="secondary"
                      type="button"
                      disabled={busy}
                      onClick={onRetry}
                    >
                      Check setup again
                    </Button>
                  )}
                </details>
              </>
            )}
            {create && !fresh && !local.available && (
              <label className="luci-field">
                <span id="auth-invite-label">Invitation code</span>
                <input
                  type="password"
                  autoComplete="off"
                  required
                  disabled={busy}
                  autoFocus
                  aria-labelledby="auth-invite-label"
                  value={inviteCode}
                  onChange={(event) => setInviteCode(event.target.value)}
                />
                <small>
                  Ask an Admin for a new code if yours has expired or was
                  already used. Codes work only for the workspace that issued
                  them.
                </small>
              </label>
            )}
            {create && (
              <label className="luci-field">
                Display name
                <input
                  required
                  maxLength={120}
                  disabled={busy}
                  autoFocus={!!bootstrapToken || local.available}
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                  autoComplete="name"
                />
              </label>
            )}
            {!recover && (
              <label className="luci-field">
                <span id="auth-username-label">Username</span>
                <input
                  required
                  disabled={busy}
                  autoFocus={!create}
                  autoCapitalize="none"
                  spellCheck={false}
                  maxLength={64}
                  minLength={create ? 3 : undefined}
                  pattern={
                    create ? "[A-Za-z0-9][A-Za-z0-9_.\\-]{2,63}" : undefined
                  }
                  aria-labelledby="auth-username-label"
                  aria-describedby={create ? "auth-username-help" : undefined}
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  autoComplete="username"
                />
                {create && (
                  <small id="auth-username-help">
                    3–64 characters. Start with a letter or number; dots,
                    underscores and hyphens are allowed.
                  </small>
                )}
              </label>
            )}
            <label className="luci-field">
              <span id="auth-password-label">
                {recover ? "New password" : "Password"}
              </span>
              <input
                required
                disabled={busy}
                type={showPassword ? "text" : "password"}
                minLength={newPassword ? 12 : undefined}
                maxLength={1024}
                aria-labelledby="auth-password-label"
                aria-describedby={
                  newPassword ? "auth-password-help" : undefined
                }
                autoComplete={newPassword ? "new-password" : "current-password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
              {newPassword && (
                <small id="auth-password-help">
                  Use at least 12 characters.
                </small>
              )}
            </label>
            {newPassword && (
              <label className="luci-field">
                Confirm password
                <input
                  required
                  disabled={busy}
                  type={showPassword ? "text" : "password"}
                  autoComplete="new-password"
                  maxLength={1024}
                  value={confirm}
                  onChange={(event) => setConfirm(event.target.value)}
                />
              </label>
            )}
            <label className="setup-check auth-password-toggle">
              <input
                type="checkbox"
                disabled={busy}
                checked={showPassword}
                onChange={(event) => setShowPassword(event.target.checked)}
              />
              {newPassword ? "Show passwords" : "Show password"}
            </label>
            <Button type="submit" variant="primary" loading={busy} disabled={busy}>
              {busy
                ? recover
                  ? "Recovering account…"
                  : create
                    ? "Creating account…"
                    : "Signing in…"
                : fresh
                  ? "Create account"
                  : recover
                    ? "Recover owner account"
                    : create
                      ? "Create account"
                      : "Sign in"}
            </Button>
          </form>
        )}
        {busy && (
          <p role="status">
            {recover
              ? "Recovering the original owner account…"
              : create
                ? "Creating your workspace account…"
                : "Signing in to your workspace…"}
          </p>
        )}
        {!loading && (connectionError || !status) && (
          <Button type="button" variant="secondary" disabled={busy} onClick={onRetry}>
            Retry connection
          </Button>
        )}
        {!!status && !loading && !fresh && (
          <nav className="auth-options" aria-label="Account options">
            {mode !== "sign-in" && (
              <Button type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => onMode("sign-in")}
              >
                Back to sign in
              </Button>
            )}
            {mode !== "create" && (
              <Button type="button"
                variant="secondary"
                disabled={busy}
                onClick={() => onMode("create")}
              >
                Create account
              </Button>
            )}
            {mode !== "recover" && (
              <Button type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => onMode("recover")}
              >
                Forgot sign-in?
              </Button>
            )}
          </nav>
        )}
        {!!status && !loading && (
          <details className="auth-help">
            <summary>Using an existing team workspace?</summary>
            <p className="journey-muted">
              If you are in the installed Luci app, create or sign in to your
              account on this computer first. Then choose “Connect to an
              existing workspace” in Settings, verify its server and sign in
              with your separate team account. Ask its administrator for
              connection details and an account or invitation.
            </p>
            <p className="journey-muted">
              If your administrator sent you a workspace address, open that
              address and sign in there. Creating an account here does not
              create one on a different server.
            </p>
          </details>
        )}
      </section>
    </main>
  );
}
