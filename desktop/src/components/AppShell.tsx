// Owns responsive navigation, the single keyboard-map mount and shared overlays.
import { useEffect, useRef, useState, type JSX } from "react";
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
import { useDismissable } from "../hooks/useDismissable";
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
  const projectId = useCockpit((s) => s.selection.projectId);
  const lens = useCockpit((s) => s.lens);
  const sessionId = useCockpit((s) => s.selection.sessionId);
  const toggleRail = useCockpit((s) => s.toggleRail);
  const [compact, setCompact] = useState(
    () => window.matchMedia("(max-width: 760px)").matches,
  );
  const [projectsOpen, setProjectsOpen] = useState(false);
  const previousRailCollapsed = useRef(railCollapsed);
  const previousDrawerOpen = useRef(false);
  const drawerOpen = compact && projectsOpen;
  const railRef = useDismissable<HTMLElement>(drawerOpen, () =>
    setProjectsOpen(false),
  );
  useEffect(() => {
    const query = window.matchMedia("(max-width: 760px)");
    const update = (): void => {
      setCompact(query.matches);
      setProjectsOpen(false);
    };
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    setProjectsOpen(false);
  }, [projectId, branchId, sessionId, lens]);
  useEffect(() => {
    if (compact && previousRailCollapsed.current !== railCollapsed)
      setProjectsOpen((open) => !open);
    previousRailCollapsed.current = railCollapsed;
  }, [compact, railCollapsed]);
  useEffect(() => {
    if (drawerOpen)
      railRef.current?.querySelector<HTMLButtonElement>(".rail-close")?.focus();
    else if (previousDrawerOpen.current)
      document.querySelector<HTMLButtonElement>(".top-projects")?.focus();
    previousDrawerOpen.current = drawerOpen;
  }, [drawerOpen, railRef]);

  // §6.8: the ONE window keydown listener in the app, mounted once, here. Escape is NOT in
  // it — hooks/useDismissable owns the overlay stack and therefore owns Escape (§6.9).
  useKeyboardMap();

  const className = ["shell"]
    .concat(projectId === null && branchId === null ? [] : ["shell-deep"])
    .concat(railCollapsed ? ["shell-rail-collapsed"] : [])
    .concat(drawerOpen ? ["shell-projects-open"] : [])
    .join(" ");

  return (
    <div className={className}>
      <Rail
        panelRef={railRef}
        hidden={compact ? !drawerOpen : railCollapsed}
        drawerOpen={drawerOpen}
        onClose={() => setProjectsOpen(false)}
      />
      {projectId === null && branchId === null ? (
        <div className="shell-sub-empty" aria-hidden="true" />
      ) : (
        <SubRail inert={drawerOpen} />
      )}
      <main className="shell-main" inert={drawerOpen}>
        <TopBar
          projectsExpanded={compact ? drawerOpen : !railCollapsed}
          onToggleProjects={() =>
            compact ? setProjectsOpen((open) => !open) : toggleRail()
          }
        />
        <div className="shell-lens">
          <LensRouter lens={lens} />
        </div>
      </main>
      {drawerOpen && <div className="shell-scrim" aria-hidden="true" />}
      <ContextMenu />
      <Toasts />
      <CommandPalette />
    </div>
  );
}
