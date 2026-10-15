import { useEffect, useState } from 'react';
import { useDesignWorkStatus } from '../../store/designWorkStatus';
export function DesignWorkIndicator({scope, active = true}: {scope: string | null; active?: boolean}) {
  const status = useDesignWorkStatus(scope), [visible, setVisible] = useState(document.visibilityState !== 'hidden');
  useEffect(() => { const changed = () => setVisible(document.visibilityState !== 'hidden'); document.addEventListener('visibilitychange', changed); return () => document.removeEventListener('visibilitychange', changed); }, []);
  if (!status) return <p className="journey-muted">No current Luci job observation. Editing the canvas starts no agent.</p>;
  const labels = {working: 'Luci is working in this project', waiting: 'Luci job is waiting', unknown: 'Luci job outcome is unconfirmed', 'response-ready': 'Luci response ready for review', failed: 'Luci job failed; inspect the conversation', stopped: 'Luci job stopped'};
  return <p role="status" className="design-work-indicator" data-active={status.phase === 'working' && visible && active || undefined}>
    <span className="design-work-dots" aria-hidden="true"><span /><span /><span /></span>{labels[status.phase]} · {status.jobId}. Responses do not change the scene until their amendment is accepted.
  </p>;
}
