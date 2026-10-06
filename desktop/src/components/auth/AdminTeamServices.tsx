import { useEffect, useRef, useState } from "react";
import { errorText } from "../../api/product";
import { teamServiceApi, type TeamServiceRow } from "../../api/teamService";

export function AdminTeamServices({ disabled, onBusy }: { disabled: boolean; onBusy(value: boolean): void }) {
  const [open, setOpen] = useState(false);
  const [rows, setRows] = useState<TeamServiceRow[]>([]);
  const [selected, setSelected] = useState("");
  const [choice, setChoice] = useState("self-hosted:active");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const alive = useRef(true), working = useRef(false);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const row = rows.find(value => value.teamId === selected);
  function choose(value?: TeamServiceRow) {
    setSelected(value?.teamId ?? "");
    setChoice(value ? `${value.service.mode}:${value.service.status}` : "self-hosted:active");
    setReason(""); setNotice("");
  }
  async function run(action: () => Promise<void>) {
    if (working.current || disabled) return;
    working.current = true; setBusy(true); onBusy(true); setError(""); setNotice("");
    try { await action(); } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) { setBusy(false); onBusy(false); } }
  }
  const reload = () => run(async () => {
    const data = await teamServiceApi.list();
    if (!alive.current) return;
    setRows(data.teams); setLoaded(true);
    choose(data.teams.find(value => value.teamId === selected) ?? data.teams[0]);
  });
  return <section className="admin-section">
    <details open={open} onToggle={event => {
      const next = event.currentTarget.open; setOpen(next);
      if (next && !loaded) void reload();
    }}>
      <summary>Team service access on this machine</summary>
      <p className="journey-muted">Self-hosted teams need no subscription. These operator controls enable or suspend managed execution; they do not collect payment or verify a dedicated host.</p>
      <p className="journey-muted">Suspension closes team terminals and browser connections. Existing workloads and provider charges may continue: stop owned sessions and deployments explicitly. Project data and access grants are retained.</p>
      {error && <p className="journey-error" role="alert">{error} Reload service access before retrying a changed revision.</p>}
      {notice && <p className="journey-success" role="status">{notice}</p>}
      {busy && <p role="status">Updating team service access…</p>}
      {loaded && rows.length === 0 && <p>No teams have been created on this machine.</p>}
      {rows.length > 0 && <form className="journey-form" onSubmit={event => {
        event.preventDefault();
        if (!row || !reason.trim()) return;
        const target = row;
        void run(async () => {
          const [mode, status] = choice.split(":") as ["self-hosted" | "managed", "active" | "suspended"];
          const service = await teamServiceApi.save(target.teamId, { expectedRevision: target.service.revision, mode, status, reason: reason.trim() });
          if (!alive.current) return;
          setRows(previous => previous.map(value => value.teamId === target.teamId ? { ...value, service } : value));
          setReason(""); setNotice(`Service access saved for ${target.name}.`);
        });
      }}>
        <label className="luci-field">Team to operate<select value={selected} disabled={busy || disabled} onChange={event => choose(rows.find(value => value.teamId === event.target.value))}>{rows.map(value => <option key={value.teamId} value={value.teamId}>{value.name}</option>)}</select></label>
        {row && <p>Current access: {row.service.mode === "self-hosted" ? "Self-hosted" : "Managed"} · {row.service.status} · revision {row.service.revision}</p>}
        <label className="luci-field">Service access<select value={choice} disabled={busy || disabled} onChange={event => setChoice(event.target.value)}>
          <option value="self-hosted:active">Self-hosted · active</option>
          <option value="managed:active">Managed · execution active</option>
          <option value="managed:suspended">Managed · execution suspended</option>
        </select></label>
        <label className="luci-field">Reason for service change<input value={reason} required maxLength={240} disabled={busy || disabled} onChange={event => setReason(event.target.value)} /></label>
        <button className="luci-button" disabled={busy || disabled || !row || !reason.trim()}>Save service access</button>
      </form>}
      <button className="luci-button" disabled={busy || disabled} onClick={() => void reload()}>Reload service access</button>
    </details>
  </section>;
}
