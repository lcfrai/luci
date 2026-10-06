import { useEffect, useRef, useState } from 'react';
import { consumeBillingReturn, readBillingReturn, type BillingReturnKind } from '../../api/billingReturn';
import { teamsApi } from '../../api/teams';
import { errorText } from '../../api/product';
import { useTeamWorkspace } from './TeamWorkspaceProvider';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { Button } from '../fluid/components/button';

/** The server-validated workspace remains authoritative; URLs can only invite
 * navigation. In particular, this component never contacts Stripe. */
export function BillingReturnNavigation({ onOpenBilling }: { onOpenBilling(kind: BillingReturnKind): void }) {
  const workspace = useTeamWorkspace();
  const [hint, setHint] = useState(readBillingReturn);
  const [paused, setPaused] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const live = useRef(true), working = useRef(false);
  const identity = JSON.stringify([workspace?.selected?.membership.userId, workspace?.teamId, workspace?.managedTeamId]);
  const currentIdentity = useRef(identity); currentIdentity.current = identity;
  useEffect(() => { live.current = true; return () => { live.current = false; }; }, []);
  // Legacy checkout links identify no team. Only the authenticated managed
  // service binding can resolve those; never guess from a remembered selection.
  const teamId = hint?.valid ? hint.teamId ?? workspace?.managedTeamId ?? '' : '';
  const target = workspace?.teams.find(row => row.team.id === teamId);
  const bindingMatches = !workspace?.managedTeamId || workspace.managedTeamId === teamId;
  const selected = !!target && bindingMatches && workspace?.selected?.team.id === teamId;
  useEffect(() => {
    if (!hint?.valid || !selected) return;
    consumeBillingReturn(); setHint(null); onOpenBilling(hint.kind);
  }, [hint, selected, onOpenBilling]);
  const dismiss = () => { consumeBillingReturn(); setHint(null); };
  const chooseTeam = async () => {
    if (!workspace || !target || !bindingMatches || working.current) return;
    const expected = currentIdentity.current;
    working.current = true; setBusy(true); setError('');
    try {
      const detail = await teamsApi.detail(teamId);
      if (!live.current || currentIdentity.current !== expected) return;
      if (detail.team.id !== teamId || detail.membership.userId !== target.membership.userId) throw new Error('The team could not be confirmed for this account. Reload your workspace access before continuing.');
      // Keep the hint until the selected scope actually renders. The existing
      // workspace switch owns the unsaved-draft confirmation and cancellation.
      setPaused(true); workspace.select(teamId);
    } catch (cause) { if (live.current && currentIdentity.current === expected) setError(errorText(cause)); }
    finally { working.current = false; if (live.current) setBusy(false); }
  };
  if (!hint || !workspace || selected) return null;
  if (paused) return <button className="rail-settings" onClick={() => setPaused(false)}>Continue billing return</button>;
  return <WorkflowDialog title="Billing return" description="A return link does not confirm a payment or change subscription access." busy={busy} onClose={dismiss}>
    {target && bindingMatches ? <>
      <p>This return is for <strong>{target.team.name}</strong>. Open its Service and data settings to review the saved subscription status. Your current workspace stays selected until you choose to switch.</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button disabled={busy} onClick={() => void chooseTeam()}>Open {target.team.name} billing</Button><Button variant="secondary" disabled={busy} onClick={dismiss}>Stay in this workspace</Button></div>
    </> : <>
      <p>This return link could not be matched to a team available in this workspace. Sign in to the account that manages the subscription, or choose an available team and open Team access → Service and data. No subscription was verified or changed by this link.</p>
      <Button variant="secondary" onClick={dismiss}>Back to my workspace</Button>
    </>}
  </WorkflowDialog>;
}
