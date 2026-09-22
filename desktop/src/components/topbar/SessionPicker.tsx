// Contract §6.4. Every control is gated on the HARNESS DESCRIPTOR, never on a harness id.
// The draft lives here and dies with the popover (§3.3: there is no drafts map in the store).
import { useEffect, useState } from "react";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectResolvedDefaults, selectWorktrees } from "../../store/selectors";
import { resolveDefaults } from "../../store/defaults";
import { useCreateSession } from "../../hooks/useCreateSession";
import { useDismissable } from "../../hooks/useDismissable";
import type {
  Effort,
  PermissionMode,
  SessionDefaultsOverride,
  WorktreeId,
} from "../../store/types";
// BOTH stylesheets, because this renders className="pop pick-panel" and the two prefixes have
// different owners: `.pick-*` is topbar.css's, but `.pop` — the surface, the elevation and
// `position: fixed` — is common.css's. It resolved anyway only because Popover, StatusDot and
// Toasts happen to pull common.css into the same bundle; rendered in isolation, or under any
// future code-split, the picker would lose its surface and land unpositioned. Importing it
// here rather than routing through <Popover/> because §6.4 gives SessionPicker no anchor prop,
// so it has no rect to position from — see the `.pick-panel` rule.
import "../common/common.css";
import "./topbar.css";

const EFFORT_LABEL: Record<Effort, string> = {
  "": "harness default",
  low: "low",
  medium: "medium",
  high: "high",
  xhigh: "xhigh",
};

export function SessionPicker(props: {
  open: boolean;
  worktreeId: WorktreeId | null;
  onClose: () => void;
}): JSX.Element | null {
  const { open, worktreeId, onClose } = props;
  const harnesses = useCockpit(useShallow((s) => s.harnesses));
  const inherited = useCockpit(useShallow(selectResolvedDefaults));
  const branchId = useCockpit((s) => s.selection.branchId);
  const worktrees = useCockpit(useShallow(selectWorktrees(branchId ?? "")));
  const setBranchDefaults = useCockpit((s) => s.setBranchDefaults);
  const createSession = useCreateSession();
  const ref = useDismissable<HTMLDivElement>(open, onClose);

  const [draft, setDraft] = useState<SessionDefaultsOverride>({});
  const [prompt, setPrompt] = useState("");
  const [target, setTarget] = useState<WorktreeId | null>(worktreeId);

  useEffect(() => {
    if (!open) return;
    setDraft({});
    setPrompt("");
    setTarget(worktreeId);
  }, [open, worktreeId]);

  // Re-resolve rather than merge: the clamp in §3.6 is what keeps a stale branch default from
  // offering a model the chosen harness does not have.
  const effective = resolveDefaults(inherited, draft, undefined, harnesses);
  const harness = harnesses.find((h) => h.id === effective.harness);

  if (!open || harness === undefined) return null;

  return (
    <div ref={ref} role="dialog" aria-label="New session" tabIndex={-1} className="pop pick-panel">
      <div className="pick">
        <label className="pick-field">
          Harness
          <select
            value={effective.harness}
            // Replacing the draft rather than spreading is deliberate: a new harness
            // invalidates model, effort and permission mode, and resolveDefaults re-derives
            // all three from the descriptor on the next render.
            onChange={(event) => setDraft({ harness: event.target.value })}
          >
            {harnesses.map((entry) => (
              <option key={entry.id} value={entry.id}>
                {entry.available ? entry.name : `${entry.name} — ${entry.note}`}
              </option>
            ))}
          </select>
        </label>

        {harness.models.length === 0 ? null : (
          <label className="pick-field">
            Model
            <select
              value={effective.model}
              onChange={(event) => setDraft({ ...draft, model: event.target.value })}
            >
              {harness.models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
          </label>
        )}

        {harness.efforts.length === 0 ? null : (
          <label className="pick-field">
            Effort
            <select
              value={effective.effort}
              onChange={(event) =>
                setDraft({ ...draft, effort: event.target.value as Effort })
              }
            >
              {harness.efforts.map((effort) => (
                <option key={effort} value={effort}>
                  {EFFORT_LABEL[effort]}
                </option>
              ))}
            </select>
          </label>
        )}

        {harness.permissionModes.length === 0 ? null : (
          <label className="pick-field">
            Permission mode
            <select
              value={effective.permissionMode}
              onChange={(event) =>
                setDraft({ ...draft, permissionMode: event.target.value as PermissionMode })
              }
            >
              {harness.permissionModes.map((mode) => (
                <option key={mode} value={mode}>
                  {mode}
                </option>
              ))}
            </select>
          </label>
        )}

        <label className="pick-field">
          Worktree
          <select value={target ?? ""} onChange={(event) => setTarget(event.target.value)}>
            {worktrees.map((worktree) => (
              <option key={worktree.id} value={worktree.id}>
                {worktree.name}
              </option>
            ))}
          </select>
        </label>

        {harness.supportsOpeningPrompt ? (
          <label className="pick-field">
            Opening prompt
            <textarea
              rows={3}
              value={prompt}
              placeholder="Blank just opens the session."
              onChange={(event) => setPrompt(event.target.value)}
            />
          </label>
        ) : null}

        {harness.note === "" ? null : <p className="pick-note">{harness.note}</p>}

        <div className="pick-actions">
          <button
            type="button"
            className="top-btn"
            disabled={branchId === null}
            title={branchId === null ? "Select a branch first." : undefined}
            onClick={() => {
              if (branchId !== null) setBranchDefaults(branchId, draft);
              onClose();
            }}
          >
            Save as branch default
          </button>
          <button
            type="button"
            className="top-split-main"
            disabled={target === null || !harness.available}
            title={harness.available ? undefined : harness.note}
            onClick={() => {
              if (target !== null) {
                void createSession(target, { overrides: draft, openingPrompt: prompt });
              }
              onClose();
            }}
          >
            Create session
          </button>
        </div>
      </div>
    </div>
  );
}
