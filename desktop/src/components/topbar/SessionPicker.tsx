// Contract §6.4. Every control is gated on the HARNESS DESCRIPTOR, never on a harness id.
// The draft lives here and dies with the popover (§3.3: there is no drafts map in the store).
import { useEffect, useId, useRef, useState } from "react";
import type { JSX, ReactNode } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectResolvedDefaults, selectWorktrees } from "../../store/selectors";
import { resolveDefaults, toCreateRequest } from "../../store/defaults";
import type { CreateSessionRequest } from "../../api/client";
import { useCreateSession } from "../../hooks/useCreateSession";
import { useDismissable } from "../../hooks/useDismissable";
import { productApi } from "../../api/product";
import { useAgentConnections, selectedAgentConnection, configuredAgentModel, agentConnectionIssue } from "../../hooks/useAgentConnections";
import { AgentConnectionSelect } from "../setup/AgentConnectionSelect";
import { useSetup } from "../setup/SetupProvider";
import type {
  Effort,
  ImplementationContextRequest,
  PermissionMode,
  SessionId,
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
  initialPrompt?: string;
  implementationContext?: ImplementationContextRequest;
  onCreated?: (sessionId?: SessionId) => void;
  /** Alternative explicit action; if it starts work, provide truthful labels and consent. */
  onSubmit?: (request: CreateSessionRequest) => Promise<{ ok: true; sessionId?: SessionId } | { ok: false; message: string }>;
  submitLabel?: string;
  /** Describe an alternative action truthfully if it can start reviewed work. */
  progressLabel?: string;
  failureGuidance?: string;
  /** Resolved draft identity only, never availability or execution authority. */
  onResolvedHarnessChange?: (harness: string | null) => void;
  children?: ReactNode;
}): JSX.Element | null {
  const { open, worktreeId, onClose, initialPrompt = "" } = props;
  const harnesses = useCockpit(useShallow((s) => s.harnesses));
  const inherited = useCockpit(useShallow(selectResolvedDefaults));
  const branchId = useCockpit((s) => s.selection.branchId);
  const projectId = useCockpit((s) => s.selection.projectId);
  const worktrees = useCockpit(useShallow(selectWorktrees(branchId ?? "")));
  const setBranchDefaults = useCockpit((s) => s.setBranchDefaults);
  const createSession = useCreateSession();
  const setup = useSetup();
  const working = useRef(false);
  const generation = useRef(0);
  const modelsId = useId();
  const modelHelpId = useId();
  const ref = useDismissable<HTMLDivElement>(open, () => {
    if (!working.current) onClose();
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const [draft, setDraft] = useState<SessionDefaultsOverride>({});
  const [prompt, setPrompt] = useState("");
  const [target, setTarget] = useState<WorktreeId | null>(worktreeId);
  const { catalogue, error: connectionError } = useAgentConnections(productApi, open);
  const [localModel, setLocalModel] = useState("");
  const [connectionChanged, setConnectionChanged] = useState(false);

  useEffect(() => {
    generation.current++;
    if (!open) return;
    setDraft({});
    setConnectionChanged(false);
    setPrompt(initialPrompt);
    setTarget(worktreeId);
    setError("");
    setBusy(false); working.current = false;
    return () => { generation.current++; };
  }, [open, worktreeId, initialPrompt, props.implementationContext]);
  useEffect(() => {
    if (!open) return;
    let stopped = false;
    void productApi.agents().then(value => {
      const local = value.providers.find(item => item.id === "local" && item.configured);
      if (!stopped) setLocalModel(local?.model ? `luci-local/${local.model}` : "");
    }).catch(() => { if (!stopped) setLocalModel(""); });
    return () => { stopped = true; };
  }, [open]);

  // Re-resolve rather than merge: the clamp in §3.6 is what keeps a stale branch default from
  // offering a model the chosen harness does not have.
  const effective = resolveDefaults(inherited, draft, undefined, harnesses);
  if (connectionChanged) effective.agentConnectionId = draft.agentConnectionId;
  const isShell = effective.harness === "shell";
  const harness = harnesses.find((h) => h.id === effective.harness);
  useEffect(() => {
    props.onResolvedHarnessChange?.(open && harness ? effective.harness : null);
  }, [open, harness?.id, effective.harness, props.onResolvedHarnessChange]);
  const connection = isShell ? undefined : selectedAgentConnection(catalogue, effective.agentConnectionId);
  const savedModel = configuredAgentModel(connection, effective.harness);
  const connectionIssue = isShell ? "" : agentConnectionIssue(connection, effective.harness, effective.agentConnectionId ?? catalogue?.activeConnectionId);
  const targetWorktree = worktrees.find(worktree => worktree.id === target);
  const contextIssue = !props.implementationContext ? "" : isShell || !harness?.supportsOpeningPrompt
    ? "Choose a coding agent that accepts an opening prompt for this implementation handoff."
    : projectId !== props.implementationContext.projectId || targetWorktree?.projectId !== props.implementationContext.projectId
      ? "This implementation handoff belongs to another project. Return to that project and choose its worktree."
      : !targetWorktree?.isPrimary ? "Implementation handoffs currently need the main checkout. Choose its branch and worktree." : "";
  const modelReadOnly = !!connection && connection.kind !== "codex-subscription" || !!savedModel || effective.harness === "opencode" && !!localModel && !connection;
  if (isShell) effective.model = "";
  else if (savedModel) effective.model = savedModel;
  else if (connection && connection.kind !== "codex-subscription") effective.model = "";
  else if (effective.harness === "opencode" && localModel) effective.model = localModel;

  if (!open || harness === undefined) return null;

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label={props.submitLabel || "New session"}
      tabIndex={-1}
      className="pop pick-panel"
    >
      <div className="pick" aria-busy={busy}>
        {props.implementationContext && <p className="pick-note">Saved handoff · Brief {props.implementationContext.briefRevision} · Plan {props.implementationContext.planRevision} · Definition {props.implementationContext.specificationRevision} · Design {props.implementationContext.designPlanRevision} · {props.implementationContext.taskIds.length} selected tasks</p>}
        {contextIssue && <p className="journey-error" role="alert">{contextIssue}</p>}
        {error && (
          <p className="journey-error" role="alert">
            {error} {props.failureGuidance || "Your session settings and prompt have been kept. Resolve the problem and try again."}
          </p>
        )}
        {busy && (
          <p role="status">
            {props.onSubmit ? props.progressLabel || "Saving reviewed run inputs… No coding session starts yet." : "Starting the isolated session… A cold runtime can take a little longer."}
          </p>
        )}
        <fieldset className="pick-inputs" disabled={busy}>
          {props.children}
          <label className="pick-field">
            Harness
            <select
              value={effective.harness}
              // A new harness resets its options, while retaining the chosen coding route
              // for switching back from Shell. Shell creation ignores that route.
              onChange={(event) => {
                const next = event.target.value;
                setDraft(current => ({ harness: next, ...(Object.prototype.hasOwnProperty.call(current, "agentConnectionId") ? { agentConnectionId: current.agentConnectionId } : {}) }));
              }}
            >
              {harnesses.map((entry) => (
                <option key={entry.id} value={entry.id} disabled={!entry.available}>
                  {entry.available
                    ? entry.name
                    : `${entry.name} — ${entry.note}`}
                </option>
              ))}
            </select>
          </label>

          {harness.id !== "shell" && <AgentConnectionSelect catalogue={catalogue} value={effective.agentConnectionId} onChange={id => { setDraft({ ...draft, agentConnectionId: id }); setConnectionChanged(true); }} className="pick-field" />}
          {harness.id !== "shell" && (connectionIssue || connectionError) && <p className="journey-error" role="alert">{connectionIssue || connectionError}</p>}

          {harness.supportsCustomModel ? <>
            <label className="pick-field">Model<input value={effective.model} readOnly={modelReadOnly} aria-describedby={modelReadOnly ? modelHelpId : undefined} onChange={event => setDraft({ ...draft, model: event.target.value })} placeholder="Exact available model ID" list={modelsId} /></label>
            <datalist id={modelsId}>{harness.models.map(model => <option key={model.id} value={model.id}>{model.name}</option>)}</datalist>
            {modelReadOnly && <p className="pick-note" id={modelHelpId}>
              {connection ? "The selected agent connection" : "Your saved local model endpoint"} supplies this model. To change it, update the saved model in{" "}
              <button type="button" className="top-btn" onClick={() => { onClose(); setup?.openAgents(); }}>Agent connections</button>, then reopen this session setup.
            </p>}
          </> : harness.models.length === 0 ? null : (
            <label className="pick-field">
              Model
              <select
                value={effective.model}
                onChange={(event) =>
                  setDraft({ ...draft, model: event.target.value })
                }
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
                  setDraft({
                    ...draft,
                    permissionMode: event.target.value as PermissionMode,
                  })
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
            <select
              value={target ?? ""}
              onChange={(event) => setTarget(event.target.value)}
            >
              {worktrees.map((worktree) => (
                <option key={worktree.id} value={worktree.id}>
                  {worktree.name}
                </option>
              ))}
            </select>
          </label>

          {harness.supportsOpeningPrompt && !props.onSubmit ? (
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

          {harness.note === "" ? null : (
            <p className="pick-note">{harness.note}</p>
          )}
          {isShell && <p className="pick-note">Tools installed in this terminal’s private home survive Restart. Forget removes that home and its tools.</p>}
          {harness.dockerAskNotice && <p className="pick-note">{harness.dockerAskNotice}</p>}
          {initialPrompt && !harness.supportsOpeningPrompt && (
            <p className="pick-note">
              Choose a coding agent that accepts an opening prompt to use this
              implementation handoff.
            </p>
          )}
        </fieldset>
        <div className="pick-actions">
          {!props.onSubmit && <button
            type="button"
            className="top-btn"
            disabled={busy || branchId === null}
            title={branchId === null ? "Select a branch first." : undefined}
            onClick={() => {
              if (branchId !== null) setBranchDefaults(branchId, { ...draft, model: effective.model });
              onClose();
            }}
          >
            Save as branch default
          </button>}
          <button
            type="button"
            className="top-split-main"
            disabled={
              target === null ||
              busy ||
              !harness.available ||
              !!connectionIssue ||
              !!contextIssue ||
              (!!initialPrompt && !harness.supportsOpeningPrompt)
            }
            title={harness.available ? undefined : harness.note}
            onClick={async () => {
              if (target !== null && !working.current) {
                const requestGeneration = generation.current;
                working.current = true;
                setBusy(true);
                setError("");
                const options = {
                  overrides: { ...draft, model: effective.model },
                  openingPrompt: prompt,
                  agentConnectionId: isShell ? undefined : effective.agentConnectionId,
                  implementationContext: props.implementationContext,
                };
                const result = props.onSubmit
                  ? await props.onSubmit(toCreateRequest(target, effective, harness, options))
                  : await createSession(target, options);
                if (requestGeneration !== generation.current) return;
                working.current = false;
                setBusy(false);
                if (result.ok) { props.onCreated?.(result.sessionId); onClose(); }
                else setError(result.message);
              }
            }}
          >
            {busy ? (props.onSubmit ? "Saving run…" : "Starting session…") : props.submitLabel || "Create session"}
          </button>
        </div>
      </div>
    </div>
  );
}
