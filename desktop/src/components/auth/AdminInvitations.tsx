import { useEffect, useRef, useState } from "react";
import { errorText, productApi } from "../../api/product";
import type { ControlUser } from "./AuthProvider";

export interface Invitation {
  id: string;
  role: ControlUser["role"];
  createdBy: string;
  createdAt: string;
  expiresAt: string;
  revoked: boolean;
  acceptedAt?: string;
  acceptedBy?: string;
}

export function AdminInvitations({
  base,
  canSuper,
  disabled,
  onBusy,
}: {
  base: string;
  canSuper: boolean;
  disabled: boolean;
  onBusy(value: boolean): void;
}) {
  const [open, setOpen] = useState(false);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [role, setRole] = useState<ControlUser["role"]>("member");
  const [created, setCreated] = useState<{
    invitation: Invitation;
    inviteCode: string;
  }>();
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const alive = useRef(true);
  const working = useRef(false);
  useEffect(() => {
    if (!canSuper) setRole("member");
  }, [canSuper]);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  async function load() {
    setLoading(true);
    setError("");
    try {
      const result = await productApi.request<{ invitations: Invitation[] }>(
        `${base}/invitations`,
      );
      if (alive.current) setInvitations(result.invitations);
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      if (alive.current) setLoading(false);
    }
  }
  async function run(action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    onBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      working.current = false;
      if (alive.current) {
        setBusy(false);
        onBusy(false);
      }
    }
  }
  return (
    <section className="admin-section">
      <h2>Invite people</h2>
      <p className="journey-muted">
        Let someone choose their own username and password. Invitations expire
        after 48 hours and can be used once.
      </p>
      {!open ? (
        <button
          className="luci-button"
          disabled={disabled}
          onClick={() => {
            setOpen(true);
            void load();
          }}
        >
          Manage invitations
        </button>
      ) : (
        <>
          {error && (
            <p className="journey-error" role="alert">
              {error}
            </p>
          )}
          {notice && <p role="status">{notice}</p>}
          {loading && <p role="status">Loading invitations…</p>}
          <form
            className="journey-form"
            aria-busy={busy}
            onSubmit={(event) => {
              event.preventDefault();
              void run(async () => {
                const result = await productApi.request<{
                  invitation: Invitation;
                  inviteCode: string;
                }>(`${base}/invitations`, { role });
                if (!alive.current) return;
                setCreated(result);
                setInvitations((current) => [result.invitation, ...current]);
                setNotice(
                  "Invitation created. Share its code privately with the intended person.",
                );
              });
            }}
          >
            <label className="luci-field">
              Invitation role
              <select
                disabled={disabled || busy}
                value={role}
                onChange={(event) =>
                  setRole(event.target.value as ControlUser["role"])
                }
              >
                <option value="member">Member</option>
                {canSuper && <option value="admin">Admin</option>}
                {canSuper && <option value="super-admin">Super Admin</option>}
              </select>
            </label>
            <button className="luci-button" disabled={disabled || busy}>
              {busy ? "Updating invitation…" : "Create invitation"}
            </button>
          </form>
          {created && (
            <div className="auth-help">
              <p>
                Give this code and the correct workspace address to the
                recipient. They choose “Create account” on that workspace’s
                sign-in screen. For an SSH workspace, give them the verified
                connection details as well.
              </p>
              <p>
                <strong>Copy this code now.</strong> It is shown only here and
                cannot be retrieved later.
              </p>
              <pre
                className="auth-invitation-code"
                aria-label="New invitation code"
              >
                {created.inviteCode}
              </pre>
              <p className="journey-muted">
                Expires{" "}
                {new Date(created.invitation.expiresAt).toLocaleString()}.
              </p>
              <button
                className="luci-button"
                disabled={disabled || busy}
                onClick={() => {
                  setCreated(undefined);
                  setNotice(
                    "Invitation code hidden. Revoke and replace the invitation if you did not save its code.",
                  );
                }}
              >
                Hide invitation code
              </button>
            </div>
          )}
          {error && (
            <button
              className="luci-button"
              disabled={disabled || busy || loading}
              onClick={() => void load()}
            >
              Reload invitations
            </button>
          )}
          {!loading && invitations.length === 0 && (
            <p className="journey-muted">No invitations have been created.</p>
          )}
          {invitations.length > 0 && (
            <table className="admin-table">
              <caption>Workspace invitations</caption>
              <thead>
                <tr>
                  <th scope="col">Access</th>
                  <th scope="col">Expires</th>
                  <th scope="col">Status</th>
                  <th scope="col">Action</th>
                </tr>
              </thead>
              <tbody>
                {invitations.map((invitation) => {
                  const state = invitation.acceptedAt
                    ? "Accepted"
                    : invitation.revoked
                      ? "Revoked"
                      : Date.parse(invitation.expiresAt) <= Date.now()
                        ? "Expired"
                        : "Available";
                  return (
                    <tr key={invitation.id}>
                      <td>{invitation.role}</td>
                      <td>{new Date(invitation.expiresAt).toLocaleString()}</td>
                      <td>{state}</td>
                      <td>
                        <button
                          className="luci-button"
                          aria-label={`Revoke invitation ${invitation.id}`}
                          disabled={
                            disabled ||
                            busy ||
                            state !== "Available" ||
                            (!canSuper && invitation.role !== "member")
                          }
                          onClick={() =>
                            void run(async () => {
                              await productApi.request(
                                `${base}/invitations/${encodeURIComponent(invitation.id)}`,
                                undefined,
                                "DELETE",
                              );
                              if (!alive.current) return;
                              setInvitations((current) =>
                                current.map((item) =>
                                  item.id === invitation.id
                                    ? { ...item, revoked: true }
                                    : item,
                                ),
                              );
                              if (created?.invitation.id === invitation.id)
                                setCreated(undefined);
                              setNotice(
                                "Invitation revoked. Its code can no longer create an account.",
                              );
                            })
                          }
                        >
                          Revoke
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </>
      )}
    </section>
  );
}
