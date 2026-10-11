import type { CheckCommand } from "../../api/product";
import { Button } from "../fluid/components/button";

export interface CheckCommandEditorProps {
  commands: readonly CheckCommand[];
  executable: string;
  args: string;
  controlsDisabled: boolean;
  fieldsDisabled: boolean;
  openAdvanced?: boolean;
  onRemove: (index: number) => void;
  onExecutableChange: (value: string) => void;
  onArgsChange: (value: string) => void;
  onAdd: () => void;
}

// Callers retain command parsing, draft ownership, consent and submission guards.
export function CheckCommandEditor({
  commands, executable, args, controlsDisabled, fieldsDisabled, openAdvanced,
  onRemove, onExecutableChange, onArgsChange, onAdd,
}: CheckCommandEditorProps) {
  return <>
    <ul className="check-commands">
      {commands.map((command, index) => (
        <li key={index}>
          <div>
            <strong>{command.label}</strong>
            <code>
              {command.executable}{" "}
              {command.args.map((arg) => JSON.stringify(arg)).join(" ")}
            </code>
          </div>
          <Button type="button" variant="tertiary"
            disabled={controlsDisabled}
            aria-label={`Remove ${command.label}`}
            onClick={() => onRemove(index)}
          >
            Remove
          </Button>
        </li>
      ))}
    </ul>
    <details open={openAdvanced || undefined}><summary>Advanced check commands</summary>
      <form className="check-command-form" onSubmit={(event) => { event.preventDefault(); onAdd(); }}>
        <label className="luci-field">
          Executable
          <input required disabled={fieldsDisabled} value={executable}
            onChange={(event) => onExecutableChange(event.target.value)} placeholder="npm" />
        </label>
        <label className="luci-field">
          Arguments · JSON array
          <input required disabled={fieldsDisabled} value={args}
            onChange={(event) => onArgsChange(event.target.value)} placeholder={'["test"]'} />
        </label>
        <Button type="submit" variant="secondary" disabled={controlsDisabled || !executable.trim()}>
          Add check command
        </Button>
      </form>
    </details>
  </>;
}
