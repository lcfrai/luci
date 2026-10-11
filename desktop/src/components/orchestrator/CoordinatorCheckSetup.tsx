import { useEffect, useMemo, useRef, useState } from 'react';
import { coordinatorCheckCommandsValid, type CoordinatorCheckPlan } from '../../api/coordinatorContinuations';
import type { CheckCommand } from '../../api/product';
import { checkSetupDraftValid, type CheckSetupDraft } from '../../hooks/projectDraftValidation';
import { useProjectDraft } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { CheckCommandEditor } from '../journey/CheckCommandEditor';
import { DraftRecovery } from '../common/DraftRecovery';
import { Button } from '../fluid/components/button';

export interface CoordinatorCheckSelection { plan: CoordinatorCheckPlan | null; dirty: boolean; problem: string; draftDigest: string }
/** Authoring only. Nothing executes, and browser recovery never restores consent. */
export function CoordinatorCheckSetup({ projectId, scope, base, requirements, disabled, recordedDraft, onChange }: {
  projectId: string; scope: string; base: string;
  requirements: CoordinatorCheckPlan['requirementSnapshots']; disabled: boolean; recordedDraft?: string;
  onChange(value: CoordinatorCheckSelection): void;
}) {
  const [opened, setOpened] = useState(false);
  const [commands, setCommands] = useState<CheckCommand[]>([]), [executable, setExecutable] = useState(''), [args, setArgs] = useState('[]');
  const [requirementIds, setRequirementIds] = useState<string[]>([]), [trusted, setTrusted] = useState(false), [changed, setChanged] = useState(false), [error, setError] = useState('');
  const value = useMemo<CheckSetupDraft>(() => ({ commands, executable, args, requirementIds, buildId: '' }), [commands, executable, args, requirementIds]);
  const original = useMemo<CheckSetupDraft>(() => ({ commands: [], executable: '', args: '[]', requirementIds: [], buildId: '' }), []);
  const digest = JSON.stringify(value), requirementsKey = JSON.stringify(requirements);
  const draft = useProjectDraft({ scope: JSON.stringify([scope, 'coordinator-checks']), kind: 'verification-setup', base, baseValue: original, value, dirty: changed, valid: checkSetupDraftValid });
  const priorBase = useRef(base);
  function edit() { setChanged(true); setTrusted(false); }
  function clear() { draft.clear(); setChanged(false); setCommands([]); setExecutable(''); setArgs('[]'); setRequirementIds([]); setTrusted(false); setError(''); }
  useUnsavedProjectDraft(projectId, changed || !!draft.pending, 'Continued-work checks', 'develop', { retained: draft.retained || !!draft.pending, discard: clear }, 'coordinator-setup');
  useEffect(() => { if (priorBase.current !== base) { priorBase.current = base; setTrusted(false); setRequirementIds([]); } }, [base, requirementsKey]);
  useEffect(() => { if (disabled) setTrusted(false); }, [disabled]);
  useEffect(() => { if (recordedDraft && recordedDraft === digest) clear(); }, [recordedDraft]);
  const complete = requirements.length > 0 && requirements.length === requirementIds.length && requirements.every(row => requirementIds.includes(row.id));
  const problem = draft.pending ? 'Restore or discard the retained check setup first.' : draft.error ? draft.error : !changed ? '' : !commands.length ? 'Add the commands these checks will execute.' : !coordinatorCheckCommandsValid(commands) ? 'Use at most eight valid check commands, with bounded arguments.' : executable.trim() || args !== '[]' ? 'Add or clear the unfinished command before delegating checks.' : !complete ? 'Select every reviewed requirement exercised by these commands. Partial coverage cannot continue this build automatically.' : !trusted ? 'Confirm the exact commands and requirement coverage before delegating checks.' : '';
  const plan = !disabled && changed && !problem ? { commands, requirementIds: requirements.map(row => row.id), requirementSnapshots: requirements, acknowledgeProjectCommands: true as const } : null;
  const projection = JSON.stringify({ plan, dirty: changed || !!draft.pending, problem, draftDigest: digest });
  useEffect(() => { onChange(JSON.parse(projection) as CoordinatorCheckSelection); }, [projection, onChange]);
  return <details onToggle={event => { if (event.currentTarget.open) setOpened(true); }}><summary>Independent checks for continued work{draft.pending ? " · saved draft" : ""}</summary>{opened && <>
    <p>Choose commands once and identify the reviewed criteria they exercise. Luci can run this exact plan after task integration under the current policy. Passing commands do not prove semantic acceptance or authorize deployment.</p>
    <DraftRecovery draft={draft} label="Continued-work checks" localSetup base={base} saved={original} busy={disabled} readOnly={disabled} onDiscard={clear} onRestore={restored => { setCommands(restored.commands); setExecutable(restored.executable); setArgs(restored.args); setRequirementIds([]); setTrusted(false); setChanged(true); setError(''); }} />
    {error && <p role="alert">{error}</p>}
    <CheckCommandEditor commands={commands} executable={executable} args={args} controlsDisabled={disabled || !!draft.pending} fieldsDisabled={disabled || !!draft.pending} openAdvanced={!commands.length}
      onRemove={index => { if (disabled || draft.pending) return; edit(); setCommands(commands.filter((_, i) => i !== index)); }}
      onExecutableChange={text => { edit(); setExecutable(text); }} onArgsChange={text => { edit(); setArgs(text); }}
      onAdd={() => { if (disabled || draft.pending) return; try { const parsed: unknown = JSON.parse(args); if (!Array.isArray(parsed) || !parsed.every(item => typeof item === 'string')) throw new Error('Arguments must be a JSON array of strings, for example ["test"].'); const next = [...commands, { label: executable.trim(), executable: executable.trim(), args: parsed }]; if (!coordinatorCheckCommandsValid(next)) throw new Error('Use at most eight commands with a simple executable name, at most 100 arguments and at most 32 KiB of arguments per command.'); edit(); setCommands(next); setExecutable(''); setArgs('[]'); setError(''); } catch (cause) { setError(cause instanceof Error ? cause.message : 'The command could not be read.'); } }} />
    {!requirements.length && <p>Link the selected saved tasks to approved requirements before delegating independent checks.</p>}
    {requirements.map(row => <label className="requirement-choice" key={row.id}><input type="checkbox" disabled={disabled || !!draft.pending} checked={requirementIds.includes(row.id)} onChange={event => { edit(); setRequirementIds(ids => event.target.checked ? [...ids, row.id] : ids.filter(id => id !== row.id)); }} /><span>{row.id} · {row.title}<small>{row.acceptance}</small></span></label>)}
    <label className="setup-check"><input type="checkbox" checked={trusted} disabled={disabled || !!draft.pending || !complete || !coordinatorCheckCommandsValid(commands)} onChange={event => setTrusted(event.target.checked)} />I delegate these exact commands and their selected requirement coverage. They may write project files and use the network.</label>
    {problem && <p role="status">{problem}</p>}
    {changed && <Button variant="tertiary" disabled={disabled} onClick={clear}>Discard continued-work check setup</Button>}
  </>}</details>;
}
