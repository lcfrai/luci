// The one right-click menu, for all four row kinds. Contract §6.2 and §6.7.
//
// The session verbs and the worktree ready mark come from ../develop/sessionVerbs (task 22).
// MenuItem is declared here and imported back by that module as a TYPE ONLY, so the cycle is
// erased at compile time and the value edge runs one way: ContextMenu -> SessionMenu ->
// sessionVerbs.
//
// This file also MOUNTS <SessionMenu/>, which amends contract §6.7. §6.7 lists the overlays by
// file and does not say who mounts which; the natural home was AppShell.tsx, but shared-shape
// §1 gives that file's edit row to task 23, and this is the only file task 22 may edit that
// AppShell already mounts exactly once. The two menus are the same concern.
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
import { SessionMenu, anchorRectFor } from "../develop/SessionMenu";
import { sessionVerbs, worktreeReadyItems } from "../develop/sessionVerbs";
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

export function ContextMenu(): JSX.Element {
  const target = useCockpit((s) => s.contextMenu);
  const closeContextMenu = useCockpit((s) => s.closeContextMenu);
  const selectBranch = useCockpit((s) => s.selectBranch);
  const openPicker = useCockpit((s) => s.openPicker);
  const sessionMenuFor = useCockpit((s) => s.sessionMenuFor);
  const closeSessionMenu = useCockpit((s) => s.closeSessionMenu);
  const openSessionMenu = useCockpit((s) => s.openSessionMenu);
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

  // useMemo is NOT an optimisation here, it is a correctness fix. Popover's positioning effect
  // depends on [anchor, ref] and ends with el.focus(); a fresh DOMRect on every render re-fires
  // it and pulls focus out of the rename input mid-keystroke — §6.8's "overlays capture focus
  // on open" turned into "overlays capture focus forever". Reading the DOM in an effect instead
  // would render the popover at the fallback position for one frame and then jump, which is the
  // layout shift docs/AGENTIC-UX.md §10 rule 7 bans.
  const sessionAnchor = useMemo(
    () => (sessionMenuFor === null ? null : anchorRectFor(sessionMenuFor)),
    [sessionMenuFor],
  );

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
      // Unconditional: §10.2 landed, so `ready` is always on the wire. The row's object literal
      // lives beside its runner in sessionVerbs.ts rather than here, because contract §8 defect
      // 9's regression test sits in sessionVerbs.test.ts and task 22 writes no ContextMenu test
      // — a gate expressed as JSX here could not be reached from there.
      ...worktreeReadyItems(worktree, closeContextMenu),
    ];
  };

  const sessionItems = (id: string): MenuItem[] => {
    return sessionVerbs(id, {
      createSession,
      close: closeContextMenu,
      // TWO STEPS, AND THAT IS INTENDED. From the rail, Rename… closes this menu and opens the
      // ⋯ menu; the user then clicks Rename… again there to get the input. The rail row has no
      // room for a text field and this task creates no second rename surface, so the ⋯ menu
      // owns the only one in the app — one surface, two ways in.
      //
      // The cheap alternative was considered and not taken: a module-level "rename armed for
      // id" that `SessionMenu` reads on mount would open the input directly. It is a third piece
      // of cross-menu module state whose lifetime nothing owns — it has to be cleared on mount,
      // on close, and on a session that disappears between the two — to save one click on the
      // less-used of the two entry points. If a reviewer wants the one click, that is the shape,
      // and the clearing rules are the work.
      //
      // The close is HERE rather than in `sessionVerbs`' `act` wrapper: `act` would also close
      // SessionMenu's own menu, which is the surface the rename input has to appear in. That was
      // this task's correction 2 and it is what made Rename work at all.
      beginRename: (sessionId) => {
        closeContextMenu();
        openSessionMenu(sessionId);
      },
    });
  };

  const items =
    target === null
      ? []
      : target.kind === "project"
        ? projectItems()
        : target.kind === "branch"
          ? branchItems()
          : target.kind === "worktree"
            ? worktreeItems()
            : sessionItems(target.id);

  // A fragment rather than an early return: the ⋯ menu must be able to open while no
  // right-click menu is up, which is the normal case. The return type narrows from
  // `JSX.Element | null` to `JSX.Element`, a widening of what it can render and not of its
  // contract — AppShell mounts it as a bare element.
  return (
    <>
      {sessionMenuFor === null ? null : (
        <SessionMenu
          sessionId={sessionMenuFor}
          anchor={sessionAnchor}
          onClose={closeSessionMenu}
        />
      )}
      {target === null ? null : (
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
      )}
    </>
  );
}
