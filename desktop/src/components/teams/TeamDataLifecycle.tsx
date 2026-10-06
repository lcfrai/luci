import { useEffect, useRef, useState } from "react";
import { productApi, errorText } from "../../api/product";
import { teamDataLifecycleApi, type TeamDataLifecycleResponse } from "../../api/teamDataLifecycle";
import type { Team, TeamMember } from "../../api/teams";
import { Button } from "../fluid/components/button";
import "./team-invitations.css";

interface Props { team: Team; membership: TeamMember; onChange(): Promise<void> }
export function TeamDataLifecycle(props: Props) {
  if (props.membership.role !== "owner") return null;
  return <ScopedTeamDataLifecycle key={`${productApi.baseUrl}:${props.team.id}:${props.membership.userId}`} {...props} />;
}
function ScopedTeamDataLifecycle({ team, onChange }: Props) {
  const [value, setValue] = useState<TeamDataLifecycleResponse | null>(null);
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [confirmed, setConfirmed] = useState(false);
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const alive = useRef(true), working = useRef(false);
  useEffect(() => {
    let current = true; alive.current = true;
    void teamDataLifecycleApi.get(team.id).then(result => { if (current) setValue(result); }).catch(cause => { if (current) setError(errorText(cause)); }).finally(() => { if (current) setLoading(false); });
    return () => { current = false; alive.current = false; };
  }, [team.id]);
  async function perform(action: () => Promise<TeamDataLifecycleResponse>, message = "") {
    if (working.current) return;
    working.current = true; setBusy(true); setError(""); setNotice("");
    try {
      const result = await action();
      if (!alive.current) return;
      setValue(result); setConfirmed(false); setNotice(message);
      if (message) await onChange();
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  const disabled = loading || busy;
  const lifecycle = value?.dataLifecycle;
  return <section className="team-invitations" aria-label="Team data lifecycle">
    <h3>Team data</h3>
    <p>Export the source and context you want to keep before requesting permanent deletion. This owner-only request asks the service operator to review the team's data; it does not erase data, stop applications or cancel billing.</p>
    <p className="journey-muted">The operator must verify a current backup and handle deployed applications, external services and retained backups separately. There is no automatic deletion schedule.</p>
    {loading && <p role="status">Loading data request…</p>}
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {lifecycle?.status === "requested" && <div>
      <p><strong>Awaiting operator review</strong> · Requested <time dateTime={lifecycle.requestedAt}>{new Date(lifecycle.requestedAt!).toLocaleString()}</time></p>
      <p className="journey-muted">Reference: {lifecycle.requestId}. This is a request receipt, not confirmation that data has been deleted.</p>
      <Button variant="secondary" disabled={disabled} onClick={() => void perform(() => teamDataLifecycleApi.cancel(team.id, Math.max(team.revision, value!.team.revision), lifecycle.requestId!), "Deletion request cancelled. The cancellation leaves the team's data, service and billing unchanged.")}>Cancel deletion request</Button>
    </div>}
    {lifecycle?.status === "cancelled" && <p>Request cancelled <time dateTime={lifecycle.cancelledAt}>{new Date(lifecycle.cancelledAt!).toLocaleString()}</time>. No data was erased by this request workflow.</p>}
    {value && lifecycle?.status !== "requested" && <details>
      <summary>Request permanent data deletion</summary>
      <form className="journey-form" onSubmit={event => {
        event.preventDefault(); if (!confirmed || disabled) return;
        void perform(() => teamDataLifecycleApi.request(team.id, Math.max(team.revision, value.team.revision)), "Deletion request saved for operator review. Your team's data is still available.");
      }}>
        <p>Permanent removal of the managed host's stored data cannot be undone without a retained backup. Other owners can cancel a pending request. Ask your operator about the remaining external data and backup retention.</p>
        <label><input type="checkbox" checked={confirmed} disabled={disabled} onChange={event => setConfirmed(event.target.checked)} />I want an operator to review permanent deletion of this team's data.</label>
        <Button type="submit" disabled={disabled || !confirmed}>Submit deletion request</Button>
      </form>
    </details>}
    <Button variant="secondary" size="compact" disabled={disabled} onClick={() => void perform(() => teamDataLifecycleApi.get(team.id))}>Refresh data request</Button>
  </section>;
}
