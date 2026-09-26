// Contract §6.1. Owns the grid, the grain layer, the one keyboard-map mount (task 23) and the
// overlays that belong to no surface. Owns NO focus and NO scroll — §6.8.
import type { JSX } from "react";
import { useCockpit } from "../store/store";
import { Rail } from "./rail/Rail";
import { SubRail } from "./subrail/SubRail";
import { TopBar } from "./topbar/TopBar";
import { DiscoverLens } from "./lens/DiscoverLens";
import { DesignLens } from "./lens/DesignLens";
import { FilesLens } from "./lens/FilesLens";
import { MergeLens } from "./lens/MergeLens";
import { DeployLens } from "./lens/DeployLens";
import { DevelopLens } from "./develop/DevelopLens";
import { ContextMenu } from "./menu/ContextMenu";
import { Toasts } from "./common/Toasts";
import { CommandPalette } from "./palette/CommandPalette";
import { useKeyboardMap } from "../hooks/useKeyboardMap";
import type { Lens } from "../store/types";
import "./shell.css";

// CLOSED FOR THE PROGRAMME — task 26, seam SX.1 in docs/SEAMS.md. Every lens is one file
// under ./lens/, and a later phase fills its own file rather than this switch: five phases
// editing one import block is five merge conflicts (dependency-graph-and-order.md:143-145).
function LensRouter(props: { lens: Lens }): JSX.Element {
  switch (props.lens) {
    case "develop":
      // ── TASK 20 FILLED THIS INSERTION POINT ───────────────────────────────────────
      // Develop is the one lens Phase 2 builds, so it is the one arm that is not a
      // placeholder. DevelopLens chooses Columns vs Canvas vs the zero state and owns
      // nothing else (§6.5); the other five arms stay honest about the phase that makes
      // them real. Guard, still true:
      //   grep -cE '^[[:space:]]+return <DevelopLens' src/components/AppShell.tsx  # 1
      // ───────────────────────────────────────────────────────────────────────────────
      return <DevelopLens />;
    case "discover":
      return <DiscoverLens />;
    case "design":
      return <DesignLens />;
    case "files":
      return <FilesLens />;
    case "merge":
      return <MergeLens />;
    case "deploy":
      return <DeployLens />;
    default:
      return <></>;
  }
}

export function AppShell(): JSX.Element {
  const railCollapsed = useCockpit((s) => s.railCollapsed);
  const branchId = useCockpit((s) => s.selection.branchId);
  const lens = useCockpit((s) => s.lens);

  // §6.8: the ONE window keydown listener in the app, mounted once, here. Escape is NOT in
  // it — hooks/useDismissable owns the overlay stack and therefore owns Escape (§6.9).
  useKeyboardMap();

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
      <CommandPalette />
    </div>
  );
}
