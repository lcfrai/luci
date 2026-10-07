import { useEffect, useRef, useState } from "react";
import { errorText, productApi } from "../../api/product";
import { teamInvitationsApi, type CreatedTeamInvitation, type TeamInvitation } from "../../api/teamInvitations";
import { emptyGrants, type Team, type TeamGrants, type TeamMember, type TeamPermissions } from "../../api/teams";
import { Button } from "../fluid/components/button";
import "./team-invitations.css";

interface Props { team: Team; membership: TeamMember; permissions: TeamPermissions; onChange(): Promise<void> }
const labels: [keyof TeamGrants, string][] = [["execute", "Run agents and checks"], ["secrets", "Use and manage credentials"], ["deploy", "Deploy applications"], ["admin", "Manage team access"]];
const statusLabel = { pending: "Ready to use", accepted: "Accepted", revoked: "Revoked", expired: "Expired", unavailable: "Issuer access changed" };

export function TeamInvitations(props: Props) {
  if (!props.permissions.admin) return null;
  return <ScopedTeamInvitations key={`${productApi.baseUrl}:${props.team.id}:${props.membership.userId}`} {...props} />;
}
function ScopedTeamInvitations(props: Props) {
  const [open, setOpen] = useState(false);
  return <details className="team-invitations" onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>Invite someone to {props.team.name}</summary>
    {open && <InvitationEditor {...props} />}
  </details>;
}
function InvitationEditor({ team, membership, onChange }: Props) {
  const [invitations, setInvitations] = useState<TeamInvitation[]>([]);
  const [created, setCreated] = useState<CreatedTeamInvitation | null>(null);
  const [role, setRole] = useState<TeamMember["role"]>("viewer");
  const [grants, setGrants] = useState<TeamGrants>({ ...emptyGrants });
  const [revision, setRevision] = useState(team.revision);
  const [visible, setVisible] = useState(12);
  const [busy, setBusy] = useState(false), [loading, setLoading] = useState(true);
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const alive = useRef(true), working = useRef(false);
  useEffect(() => {
    let current = true; alive.current = true;
    void teamInvitationsApi.list(team.id).then(value => {
      if (current) { setInvitations(value.invitations); setRevision(value.team.revision); }
    }).catch(cause => { if (current) setError(errorText(cause)); }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; alive.current = false; };
  }, [team.id]);
  useEffect(() => { if (membership.role !== "owner" && role === "owner") { setRole("viewer"); setGrants({ ...emptyGrants }); } }, [membership.role, role]);
  async function run(action: () => Promise<void>) {
    if (working.current) return;
    working.current = true; setBusy(true); setError(""); setNotice("");
    try { await action(); } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  async function refresh() {
    const value = await teamInvitationsApi.list(team.id);
    if (!alive.current) return;
    setInvitations(value.invitations); setRevision(value.team.revision);
    if (created && value.invitations.find(item => item.id === created.invitation.id)?.status !== "pending") setCreated(null);
    await onChange();
  }
  const disabled = busy || loading;
  return <section aria-label="Team invitations">
    <p>Create a private, single-use code for a new or existing account on this Luci installation. It expires after 48 hours. The person receives only the role and capabilities selected here.</p>
    <p className="journey-muted">A new account is a standard installation member. People without team administration also need access to each project assigned after they join.</p>
    {loading && <p role="status">Loading team invitations…</p>}
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    <form className="journey-form" onSubmit={event => {
      event.preventDefault(); void run(async () => {
        const value = await teamInvitationsApi.create(team.id, Math.max(revision, team.revision), role, role === "owner" ? { ...grants, admin: true } : grants);
        if (!alive.current) return;
        setCreated(value); setRevision(value.team.revision); setInvitations(rows => [value.invitation, ...rows]);
        setNotice("Invitation created. Share the code privately with its intended recipient.");
        await onChange();
      });
    }}>
      <label>Invited person's team role<select aria-label="Invited person's team role" value={role} disabled={disabled} onChange={event => setRole(event.target.value as TeamMember["role"])}>
        <option value="viewer">Viewer</option><option value="editor">Editor</option>{membership.role === "owner" && <option value="owner">Owner</option>}
      </select></label>
      <fieldset className="team-grants"><legend>Additional capabilities</legend>{labels.map(([key, label]) => <label key={key}>
        <input type="checkbox" checked={key === "admin" && role === "owner" || grants[key]} disabled={disabled || key === "admin" && role === "owner"} onChange={event => setGrants(value => ({ ...value, [key]: event.target.checked }))} />{label}
      </label>)}</fieldset>
      <Button type="submit" disabled={disabled}>Create team invitation</Button>
    </form>
    {created && <div className="team-invitation-code">
      <p>This code is shown only now. Anyone with it can claim the selected team access.</p>
      <output aria-label="New team invitation code">{created.inviteCode}</output>
      <p>Expires {new Date(created.invitation.expiresAt).toLocaleString()}. New accounts use Create account at sign-in; existing accounts use Join with invitation in Manage teams or on the managed workspace access screen.</p>
      <Button variant="secondary" disabled={disabled} onClick={() => { setCreated(null); setNotice("Code hidden. Revoke and replace the invitation if you did not save it."); }}>Hide team invitation code</Button>
    </div>}
    <Button variant="secondary" disabled={disabled} onClick={() => void run(refresh)}>Refresh team invitations</Button>
    {!loading && invitations.length === 0 && !error && <p className="journey-muted">No team invitations have been created.</p>}
    <ul className="team-invitation-list">{invitations.slice(0, visible).map(item => <li key={item.id}>
      <div><strong>{item.team.role}</strong> · {statusLabel[item.status]}<p className="journey-muted">Expires {new Date(item.expiresAt).toLocaleString()}</p><p>{labels.filter(([key]) => item.team.grants[key]).map(([, label]) => label).join(" · ") || "No additional capabilities"}</p></div>
      <Button variant="secondary" size="compact" disabled={disabled || item.status !== "pending" && item.status !== "unavailable" || item.team.role === "owner" && membership.role !== "owner"} aria-label={`Revoke team invitation ${item.id}`} onClick={() => void run(async () => {
        const value = await teamInvitationsApi.revoke(team.id, Math.max(revision, team.revision), item.id);
        if (!alive.current) return;
        setRevision(value.team.revision); setInvitations(rows => rows.map(row => row.id === item.id ? value.invitation : row));
        if (created?.invitation.id === item.id) setCreated(null);
        setNotice("Invitation revoked. Its code can no longer grant access."); await onChange();
      })}>Revoke</Button>
    </li>)}</ul>
    {invitations.length > visible && <Button variant="secondary" size="compact" disabled={disabled} onClick={() => setVisible(count => count + 12)}>Show older invitations</Button>}
  </section>;
}

interface JoinTeamInvitationProps { onJoined(team: Team): Promise<void>; expectedTeamId?: string }
export function JoinTeamInvitation({ onJoined, expectedTeamId }: JoinTeamInvitationProps) {
  return <ScopedJoinTeamInvitation key={JSON.stringify([productApi.baseUrl,expectedTeamId])} onJoined={onJoined} expectedTeamId={expectedTeamId} />;
}
function ScopedJoinTeamInvitation({ onJoined, expectedTeamId }: JoinTeamInvitationProps) {
  const [code, setCode] = useState(""), [busy, setBusy] = useState(false), [error, setError] = useState("");
  const [joined, setJoined] = useState<Team | null>(null);
  const working = useRef(false), alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  return <form className="team-invitations journey-form" onSubmit={event => { event.preventDefault(); if (working.current || joined) return; working.current = true; setBusy(true); setError(""); void (async () => {
    try {
      const value = await teamInvitationsApi.accept(code.trim());
      if (!alive.current) return;
      if (expectedTeamId && value.team.id !== expectedTeamId) throw new Error("This invitation belongs to another team. Ask an owner of this managed workspace for its invitation.");
      setCode(""); setJoined(value.team); await onJoined(value.team);
    }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  })(); }}>
    <h3>Join with invitation</h3>
    <p>Use a private team invitation from someone on this installation.</p>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {joined ? <p role="status">You joined {joined.name}. Refresh your workspaces if it has not appeared yet.</p> : <>
      <label>Team invitation code<input type="password" autoComplete="off" value={code} disabled={busy} onChange={event => setCode(event.target.value)} /></label>
      <Button type="submit" disabled={busy || !code.trim()}>Join team</Button>
    </>}
  </form>;
}
