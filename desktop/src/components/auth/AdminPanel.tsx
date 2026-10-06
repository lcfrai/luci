import { useEffect, useRef, useState } from "react";
import { productApi, errorText, type Workspace } from "../../api/product";
import { useDismissable } from "../../hooks/useDismissable";
import { AdminInvitations } from "./AdminInvitations";
import { AdminTeamServices } from "./AdminTeamServices";
import {
  useAuth,
  type AuthProfile,
  type ControlPolicy,
  type ControlUser,
} from "./AuthProvider";

const resources = [
  {
    key: "sessionMemoryMiB",
    label: "Agent memory · MiB",
    min: 128,
    max: 32768,
    fallback: 1024,
  },
  {
    key: "sessionCPUMillis",
    label: "Agent CPU · millicores",
    min: 100,
    max: 16000,
    fallback: 1000,
  },
  {
    key: "checkMemoryMiB",
    label: "Check memory · MiB",
    min: 128,
    max: 32768,
    fallback: 1024,
  },
  {
    key: "checkCPUMillis",
    label: "Check CPU · millicores",
    min: 100,
    max: 16000,
    fallback: 1000,
  },
] as const;

interface AuditEvent {
  id: string;
  at: string;
  actorId: string;
  action: string;
  target: string;
  outcome: string;
}
interface UserRuntime {
  status: string;
  startedAt?: string;
  message: string;
}
export function AdminPanel({
  mode,
  onClose,
}: {
  mode: "account" | "admin";
  onClose(): void;
}) {
  const auth = useAuth()!;
  const localPermissions = auth.profile.permissions;
  const admin =
    mode === "admin" &&
    (localPermissions.manageUsers ||
      localPermissions.manageBacking ||
      localPermissions.managePolicy ||
      localPermissions.inspectAudit);
  const [selection, setSelection] = useState<
    "loading" | "choose" | "local" | "cloud"
  >(admin ? "loading" : "local");
  const [workspace, setWorkspace] = useState<Workspace>();
  const [remoteProfile, setRemoteProfile] = useState<AuthProfile>();
  const [contextError, setContextError] = useState("");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (!admin) return;
    let cancelled = false;
    void productApi
      .status()
      .then((status) => {
        if (cancelled) return;
        setWorkspace(status.workspace);
        setSelection(status.workspace?.mode === "cloud" ? "cloud" : "local");
      })
      .catch((cause) => {
        if (cancelled) return;
        setContextError(errorText(cause));
        setSelection("choose");
      });
    return () => {
      cancelled = true;
    };
  }, [admin]);
  useEffect(() => {
    if (selection !== "cloud") return;
    let cancelled = false;
    setRemoteProfile(undefined);
    setContextError("");
    void productApi
      .request<AuthProfile>("/v1/setup/workspace/admin/me")
      .then((profile) => {
        if (!cancelled) setRemoteProfile(profile);
      })
      .catch((cause) => {
        if (!cancelled) setContextError(errorText(cause));
      });
    return () => {
      cancelled = true;
    };
  }, [selection, attempt]);
  const profile = selection === "local" ? auth.profile : remoteProfile;
  const refreshProfile = async () => {
    if (selection === "cloud") {
      try {
        const updated = await productApi.request<AuthProfile>(
          "/v1/setup/workspace/admin/me",
        );
        setRemoteProfile(updated);
        return updated;
      } catch (cause) {
        setRemoteProfile(undefined);
        setContextError(errorText(cause));
        throw cause;
      }
    }
    await auth.refresh();
  };
  return (
    <AdminPanelContent
      key={`${selection}:${profile?.user.id ?? "pending"}`}
      admin={admin}
      profile={profile ?? auth.profile}
      contextReady={!!profile}
      base={selection === "cloud" ? "/v1/setup/workspace/admin" : "/v1/admin"}
      refreshProfile={refreshProfile}
      onClose={onClose}
      controls={(busy) =>
        admin && (
          <section className="admin-section">
            <label className="luci-field">
              Control plane
              <select
                value={selection}
                disabled={busy || selection === "loading"}
                onChange={(event) => {
                  setRemoteProfile(undefined);
                  setContextError("");
                  setSelection(event.target.value as "local" | "cloud");
                }}
              >
                {selection === "loading" && (
                  <option value="loading">Checking selected backing…</option>
                )}
                {selection === "choose" && (
                  <option value="choose" disabled>
                    Choose a control plane
                  </option>
                )}
                <option value="local">This machine</option>
                <option value="cloud" disabled={workspace?.mode !== "cloud"}>
                  Selected cloud backing
                </option>
              </select>
            </label>
            {selection === "cloud" && (
              <p className="journey-muted">
                Managing the selected cloud backing
                {workspace?.host ? ` at ${workspace.host}` : ""}. Changes apply
                to this backing’s accounts, policy and runtimes.
              </p>
            )}
            {selection === "local" && (
              <p className="journey-muted">
                Managing this machine’s accounts, policy and runtimes.
              </p>
            )}
            {contextError ? (
              <>
                <p className="journey-error" role="alert">
                  {contextError}
                </p>
                {selection === "cloud" && (
                  <>
                    <p className="journey-muted">
                      Check the cloud connection and its sign-in in Settings,
                      then retry.
                    </p>
                    <button
                      className="luci-button"
                      disabled={busy}
                      onClick={() => setAttempt((value) => value + 1)}
                    >
                      Retry backing connection
                    </button>
                  </>
                )}
              </>
            ) : (
              !profile && <p role="status">Loading control-plane account…</p>
            )}
            {selection === "cloud" &&
              profile &&
              !Object.entries(profile.permissions).some(
                ([name, value]) => name !== "runHostCommands" && value,
              ) && (
                <p className="journey-muted">
                  This backing account has no administrative permissions. Sign
                  in to the backing with an Admin or Super Admin account in
                  Settings.
                </p>
              )}
          </section>
        )
      }
    />
  );
}

function AdminPanelContent({
  admin,
  profile,
  contextReady,
  base,
  refreshProfile,
  controls,
  onClose,
}: {
  admin: boolean;
  profile: AuthProfile;
  contextReady: boolean;
  base: string;
  refreshProfile(): Promise<AuthProfile | void>;
  controls(busy: boolean): React.ReactNode;
  onClose(): void;
}) {
  const auth = useAuth()!;
  const permissions = contextReady
    ? profile.permissions
    : {
        manageUsers: false,
        manageBacking: false,
        managePolicy: false,
        inspectAudit: false,
        runHostCommands: false,
      };
  const [users, setUsers] = useState<ControlUser[]>([]),
    [events, setEvents] = useState<AuditEvent[]>([]),
    [runtimes, setRuntimes] = useState<Record<string, UserRuntime>>({});
  const [policy, setPolicy] = useState<ControlPolicy>(profile.policy);
  const [harnesses, setHarnesses] = useState(
    profile.policy.allowedHarnesses.join(", "),
  );
  const [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState("");
  const [reading, setReading] = useState(admin && contextReady);
  const [username, setUsername] = useState(""),
    [displayName, setDisplayName] = useState(""),
    [password, setPassword] = useState(""),
    [role, setRole] = useState<ControlUser["role"]>("member");
  const [currentPassword, setCurrentPassword] = useState(""),
    [newPassword, setNewPassword] = useState("");
  const alive = useRef(true),
    working = useRef(false);
  const readGeneration = useRef(0);
  const ref = useDismissable<HTMLDivElement>(true, () => {
    if (!working.current) onClose();
  });
  useEffect(() => {
    ref.current?.focus();
  }, [ref]);
  useEffect(() => {
    if (busy && !ref.current?.contains(document.activeElement))
      ref.current?.focus();
  }, [busy, ref]);
  function trapFocus(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "Tab") return;
    const items = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>(
        "button:not(:disabled), input:not(:disabled), select:not(:disabled), a[href], summary",
      ),
    ).filter((item) => {
      const closed = item.closest("details:not([open])");
      return !closed || item === closed.querySelector(":scope > summary");
    });
    const first = items[0],
      last = items[items.length - 1];
    if (!first) {
      event.preventDefault();
      event.currentTarget.focus();
      return;
    }
    if (
      event.shiftKey &&
      (document.activeElement === first ||
        document.activeElement === event.currentTarget)
    ) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  }
  async function reload(access = profile) {
    const generation = ++readGeneration.current;
    setReading(true);
    try {
      const results = await Promise.all([
        access.permissions.manageUsers
          ? productApi.request<{ users: ControlUser[] }>(`${base}/users`)
          : Promise.resolve({ users: [] }),
        access.permissions.inspectAudit
          ? productApi.request<{ events: AuditEvent[] }>(`${base}/audit`)
          : Promise.resolve({ events: [] }),
      ]);
      if (alive.current && generation === readGeneration.current) {
        setUsers(results[0].users);
        setEvents(results[1].events);
      }
    } finally {
      if (alive.current && generation === readGeneration.current)
        setReading(false);
    }
  }
  useEffect(() => {
    alive.current = true;
    if (admin && contextReady)
      void reload().catch((cause) => {
        if (alive.current) setError(errorText(cause));
      });
    return () => {
      alive.current = false;
      readGeneration.current++;
    };
  }, [admin, contextReady, permissions.manageUsers, permissions.inspectAudit]);
  useEffect(() => {
    setPolicy(profile.policy);
    setHarnesses(profile.policy.allowedHarnesses.join(", "));
  }, [profile.policy]);
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) setBusy("");
    }
  }
  const runtime = (user: ControlUser, stop = false) =>
    void run(
      stop ? "Stopping user runtime" : "Reading user runtime",
      async () => {
        const value = await productApi.request<UserRuntime>(
          `${base}/users/${encodeURIComponent(user.id)}/runtime${stop ? "/stop" : ""}`,
          stop ? {} : undefined,
        );
        if (alive.current)
          setRuntimes((previous) => ({ ...previous, [user.id]: value }));
      },
    );
  return (
    <div className="setup-backdrop">
      <div
        className="admin-dialog"
        ref={ref}
        onKeyDown={trapFocus}
        role="dialog"
        aria-modal="true"
        aria-label={admin ? "Workspace administration" : "Your account"}
        tabIndex={-1}
      >
        <header className="journey-header">
          <div>
            <h1>{admin ? "Workspace administration" : "Your account"}</h1>
            {contextReady && (
              <p>
                {profile.policy.instanceName} ·{" "}
                {profile.user.displayName || profile.user.username} ·{" "}
                {profile.user.role}
              </p>
            )}
          </div>
          <button className="luci-button" disabled={!!busy} onClick={onClose}>
            Close
          </button>
        </header>
        {controls(!!busy)}
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
        {(busy || reading) && (
          <p role="status">
            {busy ? `${busy}…` : "Loading people and activity…"}
          </p>
        )}
        {!admin && (
          <section className="admin-section">
            <h2>Change your password</h2>
            <form
              className="journey-form"
              onSubmit={(event) => {
                event.preventDefault();
                void run("Changing password", async () => {
                  await productApi.request("/v1/auth/password", {
                    currentPassword,
                    newPassword,
                  });
                  setCurrentPassword("");
                  setNewPassword("");
                  setNotice(
                    "Password changed. Follow any sign-in request from the control plane.",
                  );
                });
              }}
            >
              <label className="luci-field">
                Current password
                <input
                  type="password"
                  required
                  disabled={!!busy}
                  autoComplete="current-password"
                  value={currentPassword}
                  onChange={(event) => setCurrentPassword(event.target.value)}
                />
              </label>
              <label className="luci-field">
                <span id="account-password-label">New password</span>
                <input
                  type="password"
                  required
                  disabled={!!busy}
                  minLength={12}
                  aria-labelledby="account-password-label"
                  aria-describedby="account-password-help"
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                />
                <small id="account-password-help">
                  Use at least 12 characters.
                </small>
              </label>
              <button className="luci-button" disabled={!!busy}>
                Change password
              </button>
            </form>
            <button
              className="luci-button"
              disabled={!!busy}
              onClick={() => void run("Signing out", auth.logout)}
            >
              Sign out
            </button>
          </section>
        )}
        {admin && contextReady && profile.user.role === "super-admin" && base === "/v1/admin" && (
          <AdminTeamServices disabled={!!busy} onBusy={(value) => {
            working.current = value;
            setBusy(value ? "Updating team service access" : "");
          }} />
        )}
        {admin && permissions.manageUsers && (
          <AdminInvitations
            base={base}
            canSuper={profile.user.role === "super-admin"}
            disabled={!!busy}
            onBusy={(value) => {
              working.current = value;
              setBusy(value ? "Updating invitations" : "");
            }}
          />
        )}
        {admin && permissions.manageUsers && (
          <section className="admin-section">
            <h2>People and access</h2>
            <p className="journey-muted">
              Each person has a separate project root and runtime. Role and
              account changes are enforced by the control plane.
            </p>
            <form
              className="journey-form"
              onSubmit={(event) => {
                event.preventDefault();
                void run("Creating user", async () => {
                  await productApi.request(`${base}/users`, {
                    username,
                    displayName,
                    password,
                    role,
                  });
                  setUsername("");
                  setDisplayName("");
                  setPassword("");
                  setRole("member");
                  await reload();
                  setNotice(
                    "User created. Share their initial credentials through your chosen private channel.",
                  );
                });
              }}
            >
              <div className="setup-field-grid">
                <label className="luci-field">
                  <span id="admin-username-label">New username</span>
                  <input
                    required
                    disabled={!!busy}
                    minLength={3}
                    maxLength={64}
                    pattern="[A-Za-z0-9][A-Za-z0-9_.\-]{2,63}"
                    autoCapitalize="none"
                    spellCheck={false}
                    aria-labelledby="admin-username-label"
                    aria-describedby="admin-username-help"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    autoComplete="off"
                  />
                  <small id="admin-username-help">
                    3–64 characters. Start with a letter or number; dots,
                    underscores and hyphens are allowed.
                  </small>
                </label>
                <label className="luci-field">
                  New display name
                  <input
                    required
                    disabled={!!busy}
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                  />
                </label>
                <label className="luci-field">
                  <span id="admin-password-label">Initial password</span>
                  <input
                    required
                    disabled={!!busy}
                    type="password"
                    minLength={12}
                    aria-labelledby="admin-password-label"
                    aria-describedby="admin-password-help"
                    autoComplete="new-password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                  />
                  <small id="admin-password-help">
                    Use at least 12 characters.
                  </small>
                </label>
                <label className="luci-field">
                  New user role
                  <select
                    disabled={!!busy}
                    value={role}
                    onChange={(event) =>
                      setRole(event.target.value as ControlUser["role"])
                    }
                  >
                    <option value="member">Member</option>
                    {profile.user.role === "super-admin" && (
                      <option value="admin">Admin</option>
                    )}
                    {profile.user.role === "super-admin" && (
                      <option value="super-admin">Super Admin</option>
                    )}
                  </select>
                </label>
              </div>
              <button className="luci-button" disabled={!!busy}>
                Create user
              </button>
            </form>
            <table className="admin-table" aria-busy={reading}>
              <caption>Workspace accounts</caption>
              <thead>
                <tr>
                  <th scope="col">Person</th>
                  <th scope="col">Access</th>
                  <th scope="col">Runtime</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.id}>
                    <td>
                      <strong>{user.displayName || user.username}</strong>
                      <br />
                      <small>{user.username}</small>
                    </td>
                    <td>
                      <UserAccess
                        user={user}
                        busy={!!busy}
                        canSuper={profile.user.role === "super-admin"}
                        currentId={profile.user.id}
                        onSave={(patch) =>
                          void run("Updating user access", async () => {
                            await productApi.request(
                              `${base}/users/${encodeURIComponent(user.id)}`,
                              patch,
                              "PATCH",
                            );
                            const updatedProfile = await refreshProfile();
                            await reload(updatedProfile ?? profile);
                            setNotice("User access updated.");
                          })
                        }
                      />
                    </td>
                    <td>
                      {runtimes[user.id] && (
                        <p role="status">
                          {runtimes[user.id].status} ·{" "}
                          {runtimes[user.id].message}
                        </p>
                      )}
                      <div className="journey-actions">
                        <button
                          className="luci-button"
                          disabled={!!busy}
                          aria-label={`Inspect runtime for ${user.username}`}
                          onClick={() => runtime(user)}
                        >
                          Inspect runtime
                        </button>
                        <button
                          className="luci-button"
                          disabled={
                            !!busy ||
                            (profile.user.role !== "super-admin" &&
                              user.role !== "member")
                          }
                          aria-label={`Stop runtime for ${user.username}`}
                          onClick={() => runtime(user, true)}
                        >
                          Stop runtime
                        </button>
                      </div>
                      {profile.user.role !== "super-admin" &&
                        user.role !== "member" && (
                          <small>
                            Only a Super Admin can stop administrator runtimes.
                          </small>
                        )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        )}
        {admin && (permissions.managePolicy || permissions.manageBacking) && (
          <section className="admin-section">
            <h2>Backing and operating policy</h2>
            <p className="journey-muted">
              Super Admins control execution and resource limits for this
              workspace. A monthly budget is a planning limit; it does not cap
              cloud-provider or model billing.
            </p>
            <form
              className="journey-form"
              onSubmit={(event) => {
                event.preventDefault();
                if (!permissions.managePolicy) return;
                void run("Saving workspace policy", async () => {
                  const updated = await productApi.request<ControlPolicy>(
                    `${base}/policy`,
                    {
                      ...(permissions.manageBacking
                        ? {
                            instanceName: policy.instanceName,
                            backingMode: policy.backingMode,
                          }
                        : {}),
                      ...(permissions.managePolicy
                        ? {
                            ...Object.fromEntries(
                              resources
                                .filter(({ key }) => policy[key] !== undefined)
                                .map(({ key }) => [key, policy[key]]),
                            ),
                            allowHostCommands: policy.allowHostCommands,
                            allowCloudProvisioning:
                              policy.allowCloudProvisioning,
                            allowPublicPublishing: policy.allowPublicPublishing,
                            maxSessionsPerUser: policy.maxSessionsPerUser,
                            maxConcurrentChecksPerUser:
                              policy.maxConcurrentChecksPerUser,
                            monthlyBudgetUsd: policy.monthlyBudgetUsd,
                            allowedHarnesses: harnesses
                              .split(",")
                              .map((item) => item.trim())
                              .filter(Boolean),
                          }
                        : {}),
                    },
                    "PATCH",
                  );
                  setPolicy(updated);
                  setHarnesses(updated.allowedHarnesses.join(", "));
                  await refreshProfile();
                  setNotice("Workspace policy saved.");
                });
              }}
            >
              <div className="setup-field-grid">
                <label className="luci-field">
                  Workspace name
                  <input
                    required
                    value={policy.instanceName}
                    disabled={!!busy || !permissions.managePolicy}
                    onChange={(event) =>
                      setPolicy({ ...policy, instanceName: event.target.value })
                    }
                  />
                </label>
                <label className="luci-field">
                  Execution backing
                  <select
                    value={policy.backingMode}
                    disabled={!!busy || !permissions.managePolicy}
                    onChange={(event) =>
                      setPolicy({
                        ...policy,
                        backingMode: event.target
                          .value as ControlPolicy["backingMode"],
                      })
                    }
                  >
                    <option value="docker">Docker</option>
                    <option value="aws">AWS</option>
                    <option value="azure">Azure</option>
                  </select>
                </label>
                <label className="luci-field">
                  Sessions per user
                  <input
                    disabled={!!busy || !permissions.managePolicy}
                    type="number"
                    required
                    min="1"
                    value={policy.maxSessionsPerUser}
                    onChange={(event) =>
                      setPolicy({
                        ...policy,
                        maxSessionsPerUser: Number(event.target.value),
                      })
                    }
                  />
                </label>
                <label className="luci-field">
                  Concurrent checks per user
                  <input
                    disabled={!!busy || !permissions.managePolicy}
                    type="number"
                    required
                    min="1"
                    value={policy.maxConcurrentChecksPerUser}
                    onChange={(event) =>
                      setPolicy({
                        ...policy,
                        maxConcurrentChecksPerUser: Number(event.target.value),
                      })
                    }
                  />
                </label>
                <label className="luci-field">
                  Monthly planning budget · USD
                  <input
                    disabled={!!busy || !permissions.managePolicy}
                    type="number"
                    required
                    min="0"
                    value={policy.monthlyBudgetUsd}
                    onChange={(event) =>
                      setPolicy({
                        ...policy,
                        monthlyBudgetUsd: Number(event.target.value),
                      })
                    }
                  />
                </label>
                <label className="luci-field">
                  Allowed coding harnesses
                  <input
                    disabled={!!busy || !permissions.managePolicy}
                    value={harnesses}
                    onChange={(event) => setHarnesses(event.target.value)}
                  />
                  <small>
                    Comma-separated IDs, for example claude-code, codex,
                    opencode, shell.
                  </small>
                </label>
              </div>
              <div className="setup-field-grid">
                {resources.map(({ key, label, min, max, fallback }) => (
                  <label className="luci-field" key={key}>
                    {label}
                    <input
                      disabled={
                        !!busy ||
                        !permissions.managePolicy ||
                        policy[key] === undefined
                      }
                      type="number"
                      required
                      min={min}
                      max={max}
                      value={policy[key] ?? fallback}
                      onChange={(event) =>
                        setPolicy({
                          ...policy,
                          [key]: Number(event.target.value),
                        })
                      }
                    />
                  </label>
                ))}
              </div>
              <p className="journey-muted">
                Start with 1 GiB and one CPU per agent or check. Raise these
                limits for workloads that need more memory or processing time.
                Changes apply to new work. An older control plane may need
                updating before these controls become available.
              </p>
              <label className="setup-check">
                <input
                  disabled={!!busy || !permissions.managePolicy}
                  type="checkbox"
                  checked={policy.allowCloudProvisioning}
                  onChange={(event) =>
                    setPolicy({
                      ...policy,
                      allowCloudProvisioning: event.target.checked,
                    })
                  }
                />
                Allow authorized users to provision cloud backing
              </label>
              <label className="setup-check">
                <input
                  disabled={!!busy || !permissions.managePolicy}
                  type="checkbox"
                  checked={policy.allowPublicPublishing}
                  onChange={(event) =>
                    setPolicy({
                      ...policy,
                      allowPublicPublishing: event.target.checked,
                    })
                  }
                />
                Allow explicit public repository publishing
              </label>
              <label className="setup-check">
                <input
                  disabled={!!busy || !permissions.managePolicy}
                  type="checkbox"
                  checked={policy.allowHostCommands}
                  onChange={(event) =>
                    setPolicy({
                      ...policy,
                      allowHostCommands: event.target.checked,
                    })
                  }
                />
                Allow permitted users to run trusted host diagnostics
              </label>
              <button
                className="luci-button luci-primary"
                disabled={!!busy || !permissions.managePolicy}
              >
                Save workspace policy
              </button>
            </form>
          </section>
        )}
        {admin && permissions.inspectAudit && (
          <section className="admin-section">
            <div className="journey-header">
              <h2>Operational audit</h2>
              <button
                className="luci-button"
                disabled={!!busy}
                onClick={() => void run("Refreshing administration", reload)}
              >
                Refresh administration
              </button>
            </div>
            <table className="admin-table" aria-busy={reading}>
              <caption>Recent workspace activity</caption>
              <thead>
                <tr>
                  <th scope="col">Time</th>
                  <th scope="col">Actor</th>
                  <th scope="col">Action</th>
                  <th scope="col">Target</th>
                  <th scope="col">Outcome</th>
                </tr>
              </thead>
              <tbody>
                {events.map((event) => (
                  <tr key={event.id}>
                    <td>{new Date(event.at).toLocaleString()}</td>
                    <td><span title={event.actorId}>{users.find(user => user.id === event.actorId)?.displayName || event.actorId}</span></td>
                    <td>{event.action}</td>
                    <td>{event.target}</td>
                    <td>{event.outcome}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!reading && !error && !events.length && (
              <p className="journey-muted">No audit events returned.</p>
            )}
          </section>
        )}
      </div>
    </div>
  );
}

function UserAccess({
  user,
  busy,
  canSuper,
  currentId,
  onSave,
}: {
  user: ControlUser;
  busy: boolean;
  canSuper: boolean;
  currentId: string;
  onSave(patch: {
    role: ControlUser["role"];
    disabled: boolean;
    password?: string;
  }): void;
}) {
  const [role, setRole] = useState(user.role),
    [disabled, setDisabled] = useState(user.disabled),
    [password, setPassword] = useState("");
  useEffect(() => {
    setRole(user.role);
    setDisabled(user.disabled);
  }, [user.role, user.disabled]);
  const locked = busy || (user.role !== "member" && !canSuper);
  return (
    <details>
      <summary
        aria-label={`Access for ${user.username}: ${user.role}, ${user.disabled ? "disabled" : "active"}`}
      >
        {user.role} · {user.disabled ? "disabled" : "active"}
      </summary>
      <form
        className="journey-form"
        onSubmit={(event) => {
          event.preventDefault();
          onSave({ role, disabled, ...(password ? { password } : {}) });
          setPassword("");
        }}
      >
        <label className="luci-field">
          Role for {user.username}
          <select
            value={role}
            disabled={locked}
            onChange={(event) =>
              setRole(event.target.value as ControlUser["role"])
            }
          >
            <option value="member">Member</option>
            {(canSuper || user.role === "admin") && (
              <option value="admin">Admin</option>
            )}
            {(canSuper || user.role === "super-admin") && (
              <option value="super-admin">Super Admin</option>
            )}
          </select>
        </label>
        <label className="setup-check">
          <input
            type="checkbox"
            aria-label={`Disable ${user.username}`}
            checked={disabled}
            disabled={locked || user.id === currentId}
            onChange={(event) => setDisabled(event.target.checked)}
          />
          Disabled
        </label>
        <label className="luci-field">
          Reset password · optional
          <input
            disabled={locked}
            type="password"
            minLength={12}
            aria-label={`Reset password for ${user.username}`}
            aria-describedby={`password-help-${user.id}`}
            value={password}
            autoComplete="new-password"
            onChange={(event) => setPassword(event.target.value)}
          />
          <small id={`password-help-${user.id}`}>
            Leave blank to keep the current password, or enter at least 12
            characters.
          </small>
        </label>
        <button className="luci-button" disabled={locked}>
          Save access
        </button>
        {!canSuper && user.role !== "member" && (
          <small>Only a Super Admin can change administrator access.</small>
        )}
      </form>
    </details>
  );
}
