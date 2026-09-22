// The one right-click menu, for all four row kinds. Contract §6.2 and §6.7.
//
// TASK 22 INSERTION POINTS: sessionItems() and the "Mark ready" entry in worktreeItems().
// Both are marked below. Task 22 must add its call IN THE SAME STEP that defines what the
// call reaches (shared-shape rule 2), and its pre-commit guard must be ANCHORED (rule 3) —
// an unanchored grep for "sessionVerbs" matches this comment.
import { Fragment, useMemo } from "react";
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import {
  selectBranchById,
  selectProjectById,
  selectWorktreeById,
  selectWorktrees,
} from "../../store/selectors";
import { useCreateSession } from "../../hooks/useCreateSession";
import { Popover } from "../common/Popover";
import "./menu.css";

/** One row of the menu. `run: undefined` means disabled, and `disabledReason` is then
 *  mandatory — a control the user cannot operate must say why (contract §6.4, §7.7). */
export interface MenuItem {
  id: string;
  label: string;
  run?: () => void;
  disabledReason?: string;
  danger?: boolean;
  separatorBefore?: boolean;
}

function copyText(text: string): void {
  void navigator.clipboard?.writeText(text);
}

/** Dynamic import so a jsdom test that renders the shell never loads the Tauri IPC bridge. */
function revealInExplorer(path: string): void {
  void import("@tauri-apps/plugin-opener").then((mod) => mod.revealItemInDir(path));
}

export function ContextMenu(): JSX.Element | null {
  const target = useCockpit((s) => s.contextMenu);
  const closeContextMenu = useCockpit((s) => s.closeContextMenu);
  const selectBranch = useCockpit((s) => s.selectBranch);
  const openPicker = useCockpit((s) => s.openPicker);
  // Shared-shape rule 4: the derived reads live in selectors.ts, never inline here. Each hook
  // call is UNCONDITIONAL — the `kind` test is inside the selector argument, so the hook order
  // is identical on every render — and each returns the store's own object, so a poll that
  // changed nothing re-renders nothing.
  const project = useCockpit((s) =>
    target !== null && target.kind === "project" ? selectProjectById(target.id)(s) : undefined,
  );
  const branch = useCockpit((s) =>
    target !== null && target.kind === "branch" ? selectBranchById(target.id)(s) : undefined,
  );
  const worktree = useCockpit((s) =>
    target !== null && target.kind === "worktree" ? selectWorktreeById(target.id)(s) : undefined,
  );
  // The branch verbs create into the branch's FIRST worktree; §3.7's selectWorktrees already
  // returns them in server order, so "first" needs no second ordering rule.
  const branchWorktree = useCockpit((s) =>
    branch === undefined ? undefined : selectWorktrees(branch.id)(s)[0],
  );
  const createSession = useCreateSession();

  // A context menu anchors to a POINT, so the rect is zero-sized. Popover's clamp then does
  // the right thing at a screen edge without a second positioning implementation.
  const anchor = useMemo(
    () => (target === null ? null : new DOMRect(target.x, target.y, 0, 0)),
    [target],
  );

  if (target === null) return null;

  const act = (fn: () => void) => (): void => {
    closeContextMenu();
    fn();
  };

  const projectItems = (): MenuItem[] => {
    if (project === undefined) return [];
    return [
      {
        id: "new-branch",
        label: "New branch",
        disabledReason:
          "Phase 2 has no create-branch endpoint (contract §10.3). Create it in git and it appears here.",
      },
      { id: "copy-path", label: "Copy path", run: act(() => copyText(project.path)) },
      { id: "reveal", label: "Reveal in Explorer", run: act(() => revealInExplorer(project.path)) },
    ];
  };

  const branchItems = (): MenuItem[] => {
    if (branch === undefined) return [];
    const first = branchWorktree;
    return [
      {
        id: "new-session",
        label: "New session here",
        run: first === undefined ? undefined : act(() => void createSession(first.id)),
        disabledReason: first === undefined ? "This branch has no worktrees yet." : undefined,
      },
      { id: "copy-branch", label: "Copy branch name", run: act(() => copyText(branch.name)) },
      {
        id: "branch-defaults",
        label: "Set branch defaults…",
        run: act(() => {
          selectBranch(branch.projectId, branch.id);
          openPicker();
        }),
      },
    ];
  };

  const worktreeItems = (): MenuItem[] => {
    if (worktree === undefined) return [];
    return [
      {
        id: "new-session",
        label: "New session here",
        run: act(() => void createSession(worktree.id)),
      },
      {
        id: "new-shell",
        label: "Open a shell here",
        run: act(() => void createSession(worktree.id, { harness: "shell" })),
      },
      { id: "copy-path", label: "Copy path", run: act(() => copyText(worktree.path)) },
      // ── TASK 22 INSERTION POINT ──────────────────────────────────────────────────────
      // Task 22 appends "Mark ready" here, UNCONDITIONALLY. §10.2 was requested and has
      // SHIPPED: store/types.ts records that catalog.go's worktreeJSON carries `Ready bool`
      // with no `omitempty`, so the flag is always on the wire as true or false — never
      // absent, never null. Task 22's contingency is resolved and there is no world to
      // check for: render the verb always, and PATCH …/worktrees/{id} with {"ready": bool}.
      // `Worktree.ready` stays OPTIONAL in the type only because that safely accepts an
      // always-present boolean; it is NOT a signal the field may be absent, and a guard on
      // `worktree.ready !== undefined` would be dead code that is always true against the
      // real server. Do not seed `ready: undefined` in a test to pin "the verb is hidden" —
      // that pins a state the control plane cannot produce.
      // The flag must survive the next list. Anchored guard for task 22's commit step:
      //   grep -cE '^[[:space:]]+id: "mark-ready"' src/components/menu/ContextMenu.tsx
      // ─────────────────────────────────────────────────────────────────────────────────
    ];
  };

  const sessionItems = (id: string): MenuItem[] => {
    // ── TASK 22 INSERTION POINT ──────────────────────────────────────────────────────
    // Task 22 replaces this body with the six lifecycle verbs — Resume, Fork into…,
    // Restart, Rename…, Kill, Forget — built by components/develop/sessionVerbs.ts with
    // optimistic mutation and an Undo toast. Per shared-shape rule 2, the step that adds
    // the call here is the step that defines sessionVerbs.ts; do not add the import first
    // and the module later. Anchored guard for task 22's commit step:
    //   grep -cE '^[[:space:]]+return sessionVerbs\(' src/components/menu/ContextMenu.tsx
    // Until then the only session verb is the one that mutates nothing.
    // ─────────────────────────────────────────────────────────────────────────────────
    return [{ id: "copy-session-id", label: "Copy session id", run: act(() => copyText(id)) }];
  };

  const items =
    target.kind === "project"
      ? projectItems()
      : target.kind === "branch"
        ? branchItems()
        : target.kind === "worktree"
          ? worktreeItems()
          : sessionItems(target.id);

  return (
    <Popover anchor={anchor} onClose={closeContextMenu}>
      <div className="menu" role="menu">
        {items.map((item) => (
          <Fragment key={item.id}>
            {item.separatorBefore === true ? <div className="menu-sep" /> : null}
            <button
              type="button"
              role="menuitem"
              className={item.danger === true ? "menu-item menu-item-danger" : "menu-item"}
              disabled={item.run === undefined}
              title={item.disabledReason}
              onClick={item.run}
            >
              {item.label}
            </button>
          </Fragment>
        ))}
      </div>
    </Popover>
  );
}
