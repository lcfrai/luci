import type { SetupMode } from '../../api/product';

export function SetupModeChoices({ selected, busy, manageBacking, allowProvision, onChoose, onExisting }: {
  selected?: SetupMode | '';
  busy: boolean;
  manageBacking: boolean;
  allowProvision: boolean;
  onChoose(mode: SetupMode): void;
  onExisting(): void;
}) {
  const choices: { mode: SetupMode; title: string; description: string; note: string; disabled: boolean }[] = [
    { mode: 'local', title: 'Local', description: 'Keep projects on this computer. Luci runs coding agents and previews in isolated Docker.', note: 'Recommended · no cloud hosting bill. Connect your agent account or a local model separately.', disabled: false },
    { mode: 'bring-your-own-cloud', title: 'Bring your own cloud', description: 'Use infrastructure you control. Prepare an Azure workspace with guided setup commands, then connect securely.', note: !manageBacking ? 'An administrator manages cloud setup for this workspace.' : !allowProvision ? 'New cloud resources are disabled by workspace policy. Existing workspace connections remain available below.' : 'Azure first · cloud and model charges are separate. Review resources and costs before running setup.', disabled: !manageBacking || !allowProvision },
    { mode: 'luci-cloud', title: 'Luci Cloud', description: 'The future Luci-hosted service. Hosted accounts, billing and runtime are still being developed before launch.', note: 'Not available yet · choose Local or Bring your own cloud to work now.', disabled: false },
  ];
  return <>
    <div className="setup-route-choices" role="group" aria-label="Workspace setup options">
      {choices.map(choice => <button key={choice.mode} type="button" className="setup-route-choice" disabled={busy || choice.disabled} aria-pressed={selected === choice.mode} aria-labelledby={`setup-mode-${choice.mode}`} aria-describedby={`setup-mode-description-${choice.mode} setup-mode-note-${choice.mode}`} onClick={() => onChoose(choice.mode)}>
        <span className="setup-route-title" id={`setup-mode-${choice.mode}`}>{choice.title}</span>
        <span className="setup-route-description" id={`setup-mode-description-${choice.mode}`}>{choice.description}</span>
        <span className={`setup-route-note${choice.mode === 'local' ? ' setup-route-recommended' : ''}`} id={`setup-mode-note-${choice.mode}`}>{choice.note}</span>
      </button>)}
    </div>
    <p className="journey-muted">Choosing a mode saves a preference. Your existing projects and active connection stay where they are; setup and connection changes require their own review.</p>
    <button className="setup-existing-link" type="button" disabled={busy || !manageBacking} onClick={onExisting}>Connect to an existing workspace</button>
  </>;
}
