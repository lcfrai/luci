// Contract §6.7. The command set as DATA: pure over a store snapshot, no React, no DOM, no
// import from a component — which is what lets commands.test.ts run without rendering.
//
// Ranking is fixed HERE and filtering only removes rows. docs/DESIGN-SYSTEM.md: "Stable
// ranking; rows never reflow while filtering."
import { selectPaneSessionIds, selectTargetWorktreeId } from "../../store/selectors";
import type { CockpitState } from "../../store/store";
import type { SessionId } from "../../store/types";
import { clientKeyboardPlatform, shortcutLabel, type KeyboardPlatform } from "../../lib/keyboardPlatform";

export interface Command {
  id: string;
  /** Rendered verbatim; matched case-insensitively against the query. */
  label: string;
  /** Right-aligned context, e.g. "engagement-portal / evidence-upload". */
  meta?: string;
  glyph: string;
  /** Shown inline so the palette teaches its own shortcuts. */
  hint?: string;
  /** Set only on a toggle row, and it is the toggle's CURRENT state. CommandPalette renders it
   *  twice from this one field: as the row's visible "On" or "Off", in the meta slot, and as
   *  aria-checked. A toggle row therefore sets no `meta` of its own. Added for OD-9. */
  checked?: boolean;
  run(): void;
}

/** OD-9's setting, named in the owner's words. Exported for the palette's tests, which find
 *  the row by what the user reads rather than by id. */
export const SEND_CHORDS_LABEL = "Send shortcut keys to the terminal";

/** Kind markers, not status markers. Status is colour PLUS shape and belongs to StatusDot
 *  (§7.5); a palette row that drew a status glyph would be a second vocabulary for the same
 *  fact. Three of the five — ▸ U+25B8, ▥ U+25A5, ▣ U+25A3 — are Geometric Shapes, the block
 *  SubRail's LENSES glyphs are drawn from. The other two are not, and are here anyway for
 *  named reasons: › marks an action without implying a platform-specific key, and
 *  ⑂ U+2442 (Optical Character Recognition) is the fork mark SubRail
 *  already uses for Merge, so a second branch glyph would be a second vocabulary. None is a
 *  webfont glyph — all five resolve from the system stack — and the two outside Geometric
 *  Shapes are both already on screen elsewhere in the shell, so neither is a new font risk. */
const GLYPH = {
  verb: "›",
  session: "▸",
  worktree: "▥",
  branch: "⑂", // the glyph SubRail already uses for Merge
  project: "▣",
} as const;

/** Every known session id, deterministically. selectPaneSessionIds first — §3.7 calls it "the
 *  one true ordering" and it is what ⌘1–9 indexes — then every other worktree's sessions, by
 *  worktree id. Object.keys(sessions) is NOT used: it is insertion order, and replaceSessions
 *  rewrites it on a poll, so the palette's order would change under the user's cursor. */
function allSessionIdsInStableOrder(state: CockpitState): SessionId[] {
  const ordered = [...selectPaneSessionIds(state)];
  const seen = new Set(ordered);
  for (const worktreeId of Object.keys(state.sessionIdsByWorktree).sort()) {
    for (const id of state.sessionIdsByWorktree[worktreeId] ?? []) {
      if (!seen.has(id)) {
        seen.add(id);
        ordered.push(id);
      }
    }
  }
  return ordered;
}

/** The command set for one snapshot, in §6.7's fixed block order: verbs, then sessions, then
 *  worktrees, then branches, then projects.
 *
 *  Pure over the snapshot, including its ACTIONS — zustand actions are created once and keep
 *  their identity for the life of the store, so a `run` closure taken here is still correct
 *  after any number of later writes. That is what lets CommandPalette build this list once,
 *  when it opens, instead of rebuilding it on every 2 s poll. */
export function buildCommands(state: CockpitState, platform: KeyboardPlatform = clientKeyboardPlatform()): Command[] {
  const commands: Command[] = [];
  const paneIds = selectPaneSessionIds(state);
  const hintFor = (id: SessionId): string | undefined => {
    const position = paneIds.indexOf(id) + 1;
    return position >= 1 && position <= 9 ? shortcutLabel(String(position), platform) : undefined;
  };
  const projectName = (id: string): string => state.projects.find((p) => p.id === id)?.name ?? id;

  /* ── verbs ──────────────────────────────────────────────────────────────────────────── */
  if (state.lens === "develop" && state.selection.branchId !== null) {
    const next = state.layout === "columns" ? "canvas" : "columns";
    commands.push({
      id: "verb:layout",
      label: next === "canvas" ? "Switch to Canvas" : "Switch to Columns",
      glyph: GLYPH.verb,
      run: () => state.setLayout(next),
    });
  }
  commands.push({
    id: "verb:rail",
    label: state.railCollapsed ? "Show the rail" : "Hide the rail",
    glyph: GLYPH.verb,
    hint: shortcutLabel("B", platform),
    run: () => state.toggleRail(),
  });
  commands.push({
    id: "verb:machines",
    label: "Machines",
    glyph: GLYPH.verb,
    hint: shortcutLabel("M", platform, true),
    run: () => state.openMachines(),
  });
  if (selectTargetWorktreeId(state) !== null) {
    commands.push({
      id: "verb:new-session",
      // The picker, not a create: ⌘⏎ is the no-dialog path (§6.4) and duplicating it here
      // would give the same words two different outcomes.
      label: "New session…",
      glyph: GLYPH.verb,
      run: () => state.openPicker(),
    });
  }
  // OD-9. Always listed, last among the verbs because it is the least used. The label never
  // changes, so typing "shortcut" always finds it, and the state is `checked`, not a second
  // wording. No hint, because no chord toggles it: a chord would be one the setting hands
  // away. There is no settings page yet, and the pane is not the place for it.
  //
  // The palette closes before `run`, so the toast is the only thing that shows the change took,
  // and it says the one thing the new mode hides: with the setting on, ⌘K pressed in a
  // terminal goes to the agent, so the way back to this row is ⌘K from outside one.
  //
  // EDITED BY OD-9's FIX ROUND 1. It names ONE surface, the top bar's empty part, because
  // "outside the terminal" was not enough: a click on a pane's own header or footer, or on
  // another session's rail row, hands focus straight back to a terminal (contract §6.8's two
  // movers). And the way back needs a pointer: with the setting on, no key leaves a terminal
  // (contract §6.9, an open owner question, recorded and not built).
  const sendChords = state.sendChordsToTerminal;
  commands.push({
    id: "verb:send-chords-to-terminal",
    label: SEND_CHORDS_LABEL,
    glyph: GLYPH.verb,
    checked: sendChords,
    run: () => {
      state.setSendChordsToTerminal(!sendChords);
      state.pushToast(
        sendChords
          ? "Shortcut keys go to the cockpit again, from a terminal too."
          : `Shortcut keys now go to the terminal, ${shortcutLabel("K", platform)} too. To open this palette again, click an empty part of the top bar, then press ${shortcutLabel("K", platform)}.`,
        () => state.setSendChordsToTerminal(sendChords),
      );
    },
  });

  /* ── sessions ───────────────────────────────────────────────────────────────────────── */
  for (const id of allSessionIdsInStableOrder(state)) {
    const session = state.sessions[id];
    if (session === undefined) continue;
    const worktree = (state.worktreesByBranch[session.branchId] ?? []).find(
      (w) => w.id === session.worktreeId,
    );
    commands.push({
      id: `session:${session.id}`,
      label: session.title,
      meta: `${projectName(session.projectId)} / ${worktree?.name ?? session.worktreeId}`,
      glyph: GLYPH.session,
      hint: hintFor(session.id),
      // selectBranch CLEARS the worktree and session selection (§3.3), so this order is
      // load-bearing: branch, then worktree, then focus. Any other order discards the focus
      // it just set. Exited sessions are listed and reachable on purpose — a dropped socket
      // destroys nothing and the transcript is the artifact.
      run: () => {
        state.selectBranch(session.projectId, session.branchId);
        state.selectWorktree(session.worktreeId);
        state.focusSession(session.id);
      },
    });
  }

  /* ── worktrees ──────────────────────────────────────────────────────────────────────── */
  for (const branchId of Object.keys(state.worktreesByBranch).sort()) {
    for (const worktree of state.worktreesByBranch[branchId] ?? []) {
      commands.push({
        id: `worktree:${worktree.id}`,
        label: worktree.name,
        meta: projectName(worktree.projectId),
        glyph: GLYPH.worktree,
        run: () => {
          state.selectBranch(worktree.projectId, worktree.branchId);
          state.selectWorktree(worktree.id);
        },
      });
    }
  }

  /* ── branches ───────────────────────────────────────────────────────────────────────── */
  for (const projectId of Object.keys(state.branchesByProject).sort()) {
    for (const branch of state.branchesByProject[projectId] ?? []) {
      commands.push({
        id: `branch:${branch.id}`,
        label: branch.name,
        meta: projectName(branch.projectId),
        glyph: GLYPH.branch,
        run: () => state.selectBranch(branch.projectId, branch.id),
      });
    }
  }

  /* ── projects ───────────────────────────────────────────────────────────────────────── */
  for (const project of state.projects) {
    commands.push({
      id: `project:${project.id}`,
      label: project.name,
      meta: project.path,
      glyph: GLYPH.project,
      run: () => state.selectProject(project.id),
    });
  }

  return commands;
}

/** §6.7: filtering only REMOVES rows; it never re-ranks. Matching is case-insensitive over
 *  `label` alone, which is what §6.7's comment on that field says — widening it to `meta`
 *  would make one project name match every session beneath it, and a result set that grows
 *  by a factor of ten on one keystroke reads as a rank change even though the order is
 *  untouched. Addition to §6.7: the contract fixes the rule and names no function for it. */
export function filterCommands(commands: readonly Command[], query: string): Command[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") return [...commands];
  return commands.filter((command) => command.label.toLowerCase().includes(needle));
}
