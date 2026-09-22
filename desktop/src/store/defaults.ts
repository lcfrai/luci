// desktop/src/store/defaults.ts
//
// Contract §3.6. Pure: no store import, no state, no clock. The one import is `import type`,
// erased at build, so this module has no runtime edge to the API client.

import type { CreateSessionRequest } from "../api/client";
import type {
  Effort,
  Harness,
  HarnessId,
  ModelId,
  PermissionMode,
  SessionDefaults,
  SessionDefaultsOverride,
  WorktreeId,
} from "./types";

/** Preferred when the recorded harness is gone or unavailable. */
const PREFERRED_HARNESS: HarnessId = "claude-code";
/** The one harness that is always there: the sandbox image has bash. */
const LAST_RESORT_HARNESS: HarnessId = "shell";

/** Later wins, per field. An ABSENT key inherits and never resets, and a key present with an
 *  `undefined` value counts as absent — see decision (a). `""` is a value, not "unset", so it
 *  overrides: for `effort`, `""` means "send no flag". */
function applyOverride(
  base: SessionDefaults,
  patch: SessionDefaultsOverride | undefined,
): SessionDefaults {
  if (patch === undefined) return base;
  return {
    harness: patch.harness ?? base.harness,
    model: patch.model ?? base.model,
    effort: patch.effort ?? base.effort,
    permissionMode: patch.permissionMode ?? base.permissionMode,
  };
}

function clampHarness(id: HarnessId, harnesses: readonly Harness[]): HarnessId {
  const chosen = harnesses.find((h) => h.id === id);
  if (chosen !== undefined && chosen.available) return chosen.id;
  const preferred = harnesses.find((h) => h.id === PREFERRED_HARNESS && h.available);
  if (preferred !== undefined) return preferred.id;
  const firstAvailable = harnesses.find((h) => h.available);
  if (firstAvailable !== undefined) return firstAvailable.id;
  return LAST_RESORT_HARNESS;
}

/** `models` is `[]` and never `null` — task 8's fix round made the Go side allocate
 *  unconditionally — so `.length` is always safe and no `?? []` belongs here. */
function clampModel(model: ModelId, h: Harness): ModelId {
  if (h.models.length === 0) return "";
  if (h.models.some((m) => m.id === model)) return model;
  return h.defaultModel;
}

function clampEffort(effort: Effort, h: Harness): Effort {
  if (h.efforts.length === 0) return "";
  if (h.efforts.includes(effort)) return effort;
  return "";
}

function clampPermissionMode(mode: PermissionMode, h: Harness): PermissionMode {
  if (h.permissionModes.length === 0) return "ask";
  if (h.permissionModes.includes(mode)) return mode;
  return h.permissionModes[0];
}

/** Clamps a merged set against the live catalogue, in §3.6's order: harness, then model,
 *  then effort, then permissionMode. The order matters — every field after the first is
 *  checked against the harness the first step chose, not the one that was asked for. */
export function clampToCatalogue(
  d: SessionDefaults,
  harnesses: readonly Harness[],
): SessionDefaults {
  const harness = clampHarness(d.harness, harnesses);
  const h = harnesses.find((x) => x.id === harness);
  if (h === undefined) return { harness, model: "", effort: "", permissionMode: "ask" };
  return {
    harness,
    model: clampModel(d.model, h),
    effort: clampEffort(d.effort, h),
    permissionMode: clampPermissionMode(d.permissionMode, h),
  };
}

/** §3.6. project → branch → session, then clamped. Pure, and it mutates none of its inputs. */
export function resolveDefaults(
  app: SessionDefaults,
  project: SessionDefaultsOverride | undefined,
  branch: SessionDefaultsOverride | undefined,
  harnesses: readonly Harness[],
): SessionDefaults {
  return clampToCatalogue(applyOverride(applyOverride(app, project), branch), harnesses);
}

/** §3.6's inclusion table, exhaustively. `harness` is taken from the DESCRIPTOR rather than
 *  from `d`, so a caller that passes a descriptor for a different harness cannot send a body
 *  whose flags and harness disagree — the one mismatch that would produce
 *  400 unsupported_option with a body that looks correct. `machineId` is never sent: the Go
 *  contract defaults it to "local" and Phase 2 has one machine. */
export function toCreateRequest(
  worktreeId: WorktreeId,
  d: SessionDefaults,
  harness: Harness,
  opts: {
    openingPrompt?: string;
    title?: string;
    resume?: string;
    contextRef?: string;
  } = {},
): CreateSessionRequest {
  const req: CreateSessionRequest = { worktreeId, harness: harness.id };
  if (harness.models.length > 0 && d.model !== "") req.model = d.model;
  if (harness.efforts.length > 0 && d.effort !== "") req.effort = d.effort;
  if (harness.permissionModes.length > 0) req.permissionMode = d.permissionMode;

  const prompt = (opts.openingPrompt ?? "").trim();
  if (harness.supportsOpeningPrompt && prompt !== "") req.openingPrompt = prompt;

  const title = (opts.title ?? "").trim();
  if (title !== "") req.title = title;

  if (harness.supportsResume && opts.resume !== undefined && opts.resume !== "") {
    req.resume = opts.resume;
  }
  if (opts.contextRef !== undefined && opts.contextRef !== "") req.contextRef = opts.contextRef;
  return req;
}
