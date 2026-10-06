import { useEffect, useState } from "react";
import { teamAuditApi, type TeamAuditResponse } from "../../api/teamAudit";
import { productApi, errorText } from "../../api/product";
import { Button } from "../fluid/components/button";
import "./team-invitations.css";

const descriptions: Record<string, string> = {
  "team.data.deletion.request": "Requested operator review of data deletion", "team.data.deletion.cancel": "Cancelled a data deletion request",
  "team.create": "Created the team", "team.rename": "Renamed the team", "team.member.save": "Changed team membership",
  "team.member.remove": "Removed team membership", "team.project.access": "Changed project access",
  "team.invitation.create": "Created a team invitation", "team.invitation.revoke": "Revoked a team invitation",
  "team.invitation.accept": "Joined through a team invitation", "team.service.active": "Activated service access", "team.service.suspended": "Suspended service access",
};
export function TeamAudit({ teamId, actorId, allowed }: { teamId: string; actorId: string; allowed: boolean }) {
  if (!allowed) return null;
  return <ScopedTeamAudit key={`${productApi.baseUrl}:${teamId}:${actorId}`} teamId={teamId} />;
}
function ScopedTeamAudit({ teamId }: { teamId: string }) {
  const [open, setOpen] = useState(false);
  return <details className="team-invitations" onToggle={event => setOpen(event.currentTarget.open)}><summary>Team access and service history</summary>{open && <AuditEvents teamId={teamId} />}</details>;
}
function AuditEvents({ teamId }: { teamId: string }) {
  const [value, setValue] = useState<TeamAuditResponse | null>(null), [error, setError] = useState("");
  const [refresh, setRefresh] = useState(0), [loading, setLoading] = useState(true), [visible, setVisible] = useState(12);
  useEffect(() => {
    let current = true; setLoading(true); setError(""); setValue(null); setVisible(12);
    void teamAuditApi.get(teamId).then(result => { if (current) setValue(result); }).catch(cause => { if (current) setError(errorText(cause)); }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [teamId, refresh]);
  return <section aria-label="Team management activity">
    <p>Who changed this team's access and service settings. Project edits and reviews appear in each project's context history.</p>
    <p className="journey-muted">Shows up to 200 recent events for this team from the installation's 1,000 retained management events. Older activity may no longer be available.</p>
    {loading && <p role="status">Loading team activity…</p>}
    {error && <p role="alert" className="journey-error">{error}</p>}
    <Button variant="secondary" size="compact" disabled={loading} onClick={() => setRefresh(value => value + 1)}>Refresh team activity</Button>
    {value && (value.events.length === 0 ? <p>No retained management activity is available for this team.</p> : <>
      <ol className="team-invitation-list">{value.events.slice(0, visible).map(event => <li key={event.id}><div>
        <p><strong>{event.actor.displayName}</strong> · {descriptions[event.action] ?? "Changed team settings"}</p>
        <p className="journey-muted"><time dateTime={event.at}>{new Date(event.at).toLocaleString()}</time> · {event.outcome === "success" || event.outcome === "succeeded" ? "Saved" : event.outcome}</p>
      </div></li>)}</ol>
      {value.events.length > visible && <Button variant="secondary" size="compact" onClick={() => setVisible(count => count + 12)}>Show older team activity</Button>}
    </>)}
  </section>;
}
