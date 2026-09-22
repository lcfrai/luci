// Contract §6.1. Owns the grid, the grain layer, the one keyboard-map mount (task 23) and the
// overlays that belong to no surface. Owns NO focus and NO scroll — §6.8.
import type { JSX } from "react";
import { useCockpit } from "../store/store";
import { Rail } from "./rail/Rail";
import { SubRail } from "./subrail/SubRail";
import { TopBar } from "./topbar/TopBar";
import { LensPlaceholder } from "./lens/LensPlaceholder";
import { ContextMenu } from "./menu/ContextMenu";
import { Toasts } from "./common/Toasts";
import type { Lens } from "../store/types";
import "./shell.css";

function LensRouter(props: { lens: Lens }): JSX.Element {
  switch (props.lens) {
    case "develop":
      // ── TASK 20 INSERTION POINT ────────────────────────────────────────────────────
      // Task 20 replaces the next line with `return <DevelopLens />;` and adds the import.
      // This makes AppShell.tsx a MODIFIED file for task 20, which §9's task-20 row does
      // not list; §9 gives DevelopLens no mounter at all, so the addition is unavoidable.
      // Per shared-shape rule 2, the step that adds the line is the step that defines
      // DevelopLens. Anchored guard for task 20's commit step:
      //   grep -cE '^[[:space:]]+return <DevelopLens' src/components/AppShell.tsx
      // ───────────────────────────────────────────────────────────────────────────────
      return (
        <LensPlaceholder
          lens="develop"
          phase={2}
          what="Sessions on this branch, as columns or a canvas."
        />
      );
    case "discover":
      return (
        <LensPlaceholder
          lens="discover"
          phase={4}
          what="Turn a conversation into a spec this branch can be measured against."
        />
      );
    case "design":
      return (
        <LensPlaceholder
          lens="design"
          phase={5}
          what="A canvas of real HTML frames an agent can edit directly."
        />
      );
    case "files":
      return (
        <LensPlaceholder
          lens="files"
          phase={3}
          what="The worktree's files, with what each session changed."
        />
      );
    case "merge":
      return (
        <LensPlaceholder
          lens="merge"
          phase={3}
          what="Review a worktree's diff and land it on the branch."
        />
      );
    case "deploy":
      return (
        <LensPlaceholder
          lens="deploy"
          phase={5}
          what="Ship the branch, with the spec attached to the release."
        />
      );
    default:
      return <></>;
  }
}

export function AppShell(): JSX.Element {
  const railCollapsed = useCockpit((s) => s.railCollapsed);
  const branchId = useCockpit((s) => s.selection.branchId);
  const lens = useCockpit((s) => s.lens);

  // ── TASK 23 INSERTION POINT ────────────────────────────────────────────────────────
  // Task 23 adds `useKeyboardMap();` here — ONE window keydown listener for the whole app
  // (§6.8), and it must delegate Escape to closeTopOverlay() from hooks/useDismissable
  // rather than tracking overlay state itself. Anchored guard for task 23:
  //   grep -cE '^[[:space:]]+useKeyboardMap\(\);' src/components/AppShell.tsx
  // ───────────────────────────────────────────────────────────────────────────────────

  const className = ["shell", "grain"]
    .concat(branchId === null ? [] : ["shell-deep"])
    .concat(railCollapsed ? ["shell-rail-collapsed"] : [])
    .join(" ");

  return (
    <div className={className}>
      <Rail />
      {branchId === null ? <div aria-hidden="true" /> : <SubRail />}
      <main className="shell-main">
        <TopBar />
        <div className="shell-lens">
          <LensRouter lens={lens} />
        </div>
      </main>
      <ContextMenu />
      <Toasts />
      {/* ── TASK 23 INSERTION POINT ──────────────────────────────────────────────────
          Task 23 mounts <CommandPalette /> here. Anchored guard:
            grep -cE '^[[:space:]]+<CommandPalette' src/components/AppShell.tsx
          ───────────────────────────────────────────────────────────────────────── */}
    </div>
  );
}
