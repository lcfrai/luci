import { useEffect, useRef, useState } from 'react';
import { orchestratorApi, type OrchestratorTurn } from '../../api/orchestrator';
import { blueprintDraftIntent, blueprintIntentMatchesContext } from '../../api/orchestratorBlueprint';
import { errorText } from '../../api/product';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import './blueprint-proposal.css';

export function BlueprintProposalReview({ turn, scope, canEdit, blocked }: { turn: OrchestratorTurn; scope: string; canEdit: boolean; blocked: boolean }) {
 const [busy, setBusy] = useState(false), [error, setError] = useState('');
 const owner = useRef({ scope, canEdit, blocked, turnId: turn.id }); owner.current = { scope, canEdit, blocked, turnId: turn.id };
 const pending = useRef<AbortController | null>(null);
 useEffect(() => () => { pending.current?.abort(); }, []);
 useEffect(() => { pending.current?.abort(); pending.current = null; setBusy(false); }, [scope, canEdit, blocked, turn.id]);
 async function open() {
  if (!canEdit || blocked || pending.current) return;
  const controller = new AbortController(); pending.current = controller; setBusy(true); setError('');
  const current = () => !controller.signal.aborted && pending.current === controller && owner.current.scope === scope && owner.current.turnId === turn.id && owner.current.canEdit && !owner.current.blocked;
  try {
   const intent = blueprintDraftIntent(turn, scope);
   const view = await orchestratorApi.get(turn.projectId, undefined, controller.signal);
   if (!current()) return;
   const state = useCockpit.getState();
   if (!blueprintIntentMatchesContext(intent, view.context) || state.selection.projectId !== turn.projectId || Object.values(state.unsavedProjectDrafts).some(draft => draft.projectId === turn.projectId)) throw new Error('The saved blueprint, source, context or editor changed. Keep your edits and ask Luci for a fresh topology proposal.');
   useCockpit.setState({ projectBlueprintDraft: intent });
   state.setLens('deploy');
   const after = useCockpit.getState();
   if (after.lens !== 'deploy' || after.pendingNavigation) {
    useCockpit.setState(value => value.projectBlueprintDraft === intent ? { projectBlueprintDraft: null } : {});
    throw new Error('Resolve the current navigation draft before reviewing this proposal in Deploy.');
   }
  } catch (cause) { if (current()) setError(errorText(cause)); }
  finally { if (pending.current === controller) { pending.current = null; if (!controller.signal.aborted) setBusy(false); } }
 }
 return <section className="blueprint-proposal-review" aria-label="Review service blueprint proposal">
  <p>Review this proposed topology in Deploy, then save it explicitly. Saving a blueprint does not authorize deployment.</p>
  <details><summary>Inspect the complete proposed topology</summary><pre>{JSON.stringify(turn.answer?.proposal?.deploymentBlueprint, null, 2)}</pre></details>
  <Button type="button" variant="secondary" disabled={!canEdit || blocked || busy} onClick={() => void open()}>Review in Deploy</Button>
  {busy && <p role="status">Checking the saved blueprint and current source…</p>}{error && <p role="alert">{error}</p>}
 </section>;
}
