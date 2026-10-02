import { useEffect, useRef, useState } from "react";
import { productApi, errorText } from "../../api/product";
import { useDismissable } from "../../hooks/useDismissable";
import { useAuth, type ControlPolicy, type ControlUser } from "./AuthProvider";

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
  const permissions = auth.profile.permissions;
  const permitted =
    permissions.manageUsers ||
    permissions.manageBacking ||
    permissions.managePolicy ||
    permissions.inspectAudit;
  const admin = mode === "admin" && permitted;
  const [users, setUsers] = useState<ControlUser[]>([]),
    [events, setEvents] = useState<AuditEvent[]>([]),
    [runtimes, setRuntimes] = useState<Record<string, UserRuntime>>({});
  const [policy, setPolicy] = useState<ControlPolicy>(auth.profile.policy);
  const [harnesses, setHarnesses] = useState(
    auth.profile.policy.allowedHarnesses.join(", "),
  );
  const [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState("");
  const [username, setUsername] = useState(""),
    [displayName, setDisplayName] = useState(""),
    [password, setPassword] = useState(""),
    [role, setRole] = useState<ControlUser["role"]>("member");
  const [currentPassword, setCurrentPassword] = useState(""),
    [newPassword, setNewPassword] = useState("");
  const alive = useRef(true),
    working = useRef(false);
  const ref = useDismissable<HTMLDivElement>(true, () => {
    if (!working.current) onClose();
  });
  useEffect(() => {
    ref.current?.focus();
  }, [ref]);
  function trapFocus(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "Tab") return;
    const items = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>(
        "button:not(:disabled), input:not(:disabled), select:not(:disabled), a[href]",
      ),
    ).filter((item) => !item.closest("details:not([open])"));
    const first = items[0],
      last = items[items.length - 1];
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
  async function reload() {
    const results = await Promise.all([
      permissions.manageUsers
        ? productApi.request<{ users: ControlUser[] }>("/v1/admin/users")
        : Promise.resolve({ users: [] }),
      permissions.inspectAudit
        ? productApi.request<{ events: AuditEvent[] }>("/v1/admin/audit")
        : Promise.resolve({ events: [] }),
    ]);
    if (alive.current) {
      setUsers(results[0].users);
      setEvents(results[1].events);
    }
  }
  useEffect(() => {
    alive.current = true;
    if (admin)
      void reload().catch((cause) => {
        if (alive.current) setError(errorText(cause));
      });
    return () => {
      alive.current = false;
    };
  }, [admin]);
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
          `/v1/admin/users/${encodeURIComponent(user.id)}/runtime${stop ? "/stop" : ""}`,
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
            <p>
              {auth.profile.policy.instanceName} ·{" "}
              {auth.profile.user.displayName || auth.profile.user.username} ·{" "}
              {auth.profile.user.role}
            </p>
          </div>
          <button className="luci-button" disabled={!!busy} onClick={onClose}>
            Close
          </button>
        </header>
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
        {busy && <p role="status">{busy}…</p>}
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
                  autoComplete="current-password"
                  value={currentPassword}
                  onChange={(event) => setCurrentPassword(event.target.value)}
                />
              </label>
              <label className="luci-field">
                New password
                <input
                  type="password"
                  required
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                />
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
                  await productApi.request("/v1/admin/users", {
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
                  New username
                  <input
                    required
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    autoComplete="off"
                  />
                </label>
                <label className="luci-field">
                  New display name
                  <input
                    required
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                  />
                </label>
                <label className="luci-field">
                  Initial password
                  <input
                    required
                    type="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                  />
                </label>
                <label className="luci-field">
                  New user role
                  <select
                    value={role}
                    onChange={(event) =>
                      setRole(event.target.value as ControlUser["role"])
                    }
                  >
                    <option value="member">Member</option>
                    {auth.profile.user.role === "super-admin" && (
                      <option value="admin">Admin</option>
                    )}
                    {auth.profile.user.role === "super-admin" && (
                      <option value="super-admin">Super Admin</option>
                    )}
                  </select>
                </label>
              </div>
              <button className="luci-button" disabled={!!busy}>
                Create user
              </button>
            </form>
            <table className="admin-table">
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
                        canSuper={auth.profile.user.role === "super-admin"}
                        currentId={auth.profile.user.id}
                        onSave={(patch) =>
                          void run("Updating user access", async () => {
                            await productApi.request(
                              `/v1/admin/users/${encodeURIComponent(user.id)}`,
                              patch,
                              "PATCH",
                            );
                            await reload();
                            await auth.refresh();
                            setNotice("User access updated.");
                          })
                        }
                      />
                    </td>
                    <td>
                      {runtimes[user.id] && (
                        <p>
                          {runtimes[user.id].status} ·{" "}
                          {runtimes[user.id].message}
                        </p>
                      )}
                      <div className="journey-actions">
                        <button
                          className="luci-button"
                          disabled={!!busy}
                          onClick={() => runtime(user)}
                        >
                          Inspect runtime
                        </button>
                        <button
                          className="luci-button"
                          disabled={!!busy}
                          onClick={() => runtime(user, true)}
                        >
                          Stop runtime
                        </button>
                      </div>
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
                    "/v1/admin/policy",
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
                  await auth.refresh();
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
                    disabled={!permissions.managePolicy}
                    onChange={(event) =>
                      setPolicy({ ...policy, instanceName: event.target.value })
                    }
                  />
                </label>
                <label className="luci-field">
                  Execution backing
                  <select
                    value={policy.backingMode}
                    disabled={!permissions.managePolicy}
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
                    disabled={!permissions.managePolicy}
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
                    disabled={!permissions.managePolicy}
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
                    disabled={!permissions.managePolicy}
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
                    disabled={!permissions.managePolicy}
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
                        !permissions.managePolicy || policy[key] === undefined
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
                  disabled={!permissions.managePolicy}
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
                  disabled={!permissions.managePolicy}
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
                  disabled={!permissions.managePolicy}
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
            <table className="admin-table">
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
                    <td>{event.actorId}</td>
                    <td>{event.action}</td>
                    <td>{event.target}</td>
                    <td>{event.outcome}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!events.length && (
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
  const locked = busy || (user.role !== "member" && !canSuper);
  return (
    <details>
      <summary>
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
            value={password}
            autoComplete="new-password"
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        <button className="luci-button" disabled={locked}>
          Save access
        </button>
      </form>
    </details>
  );
}
