import { useEffect, useState } from 'react';
import { useDesignWorkStatus } from '../../store/designWorkStatus';
export function DesignWorkIndicator({scope, active = true}: {scope: string | null; active?: boolean}) {
  const status = useDesignWorkStatus(scope), [visible, setVisible] = useState(document.visibilityState !== 'hidden');
  const [clock, setClock] = useState(Date.now()), [afterHidden, setAfterHidden] = useState(0);
  useEffect(() => { const changed = () => { const now = Date.now(); if (document.visibilityState === 'hidden') setAfterHidden(now); setClock(now); setVisible(document.visibilityState !== 'hidden'); }; document.addEventListener('visibilitychange', changed); return () => document.removeEventListener('visibilitychange', changed); }, []);
  useEffect(() => {
    setClock(Date.now());
    if (status?.phase !== 'working' || !active || !visible) return;
    // One presentation expiry, not a job poller. Only a new observed response
    // can make a stale running state animate again.
    const timer = window.setTimeout(() => setClock(Date.now()), Math.max(0, status.observedAt + 10000 - Date.now()));
    return () => window.clearTimeout(timer);
  }, [status, active, visible]);
  if (!status) return <p className="journey-muted">No current Luci job observation. Editing the canvas starts no agent.</p>;
  const labels = {working: 'Luci is working in this project', waiting: 'Luci job is waiting', unknown: 'Luci job outcome is unconfirmed', 'response-ready': 'Luci response ready for review', failed: 'Luci job failed; inspect the conversation', stopped: 'Luci job stopped'};
  const fresh = status.observedAt > afterHidden && clock - status.observedAt < 10000;
  return <p role="status" className="design-work-indicator" data-active={status.phase === 'working' && visible && active && fresh || undefined}>
    <span className="design-work-dots" aria-hidden="true"><span /><span /><span /></span>{status.phase === 'working' && !fresh ? 'Last job observation was running; current work is unconfirmed' : labels[status.phase]} · {status.jobId}. Responses do not change the scene until their amendment is accepted.
  </p>;
}
