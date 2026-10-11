import { homeRenderBegin, homeCommit } from "../diagnostics/homePhaseTiming";
// Owns responsive navigation, the single keyboard-map mount and shared overlays.
import { useEffect, useLayoutEffect, useRef, useState, type JSX } from "react";
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
import { ProjectsHome } from "./projects/ProjectsHome";
import { ColumnsLayout } from "./develop/ColumnsLayout";
import { useSetup } from "./setup/SetupProvider";
import { DraftNavigationDialog } from "./common/DraftNavigationDialog";
import { BranchManager } from "./journey/BranchManager";
import { OrchestratorPanel } from "./orchestrator/OrchestratorPanel";

// Each phase owns its workspace. Source checkout revisions remount them together
// so a saved definition, design or file view cannot linger from the old branch.
function LensRouter(props: { lens: Lens }): JSX.Element {
  switch (props.lens) {
    case "develop":
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
  const homeRender = homeRenderBegin();
  const setup = useSetup();
  const railCollapsed = useCockpit((s) => s.railCollapsed);
  const branchId = useCockpit((s) => s.selection.branchId);
  const projectId = useCockpit((s) => s.selection.projectId);
  const sourceRevision = useCockpit(s => projectId ? s.projectSourceRevisions[projectId] ?? 0 : 0);
  const lens = useCockpit((s) => s.lens);
  const sessionId = useCockpit((s) => s.selection.sessionId);
  const accountSession = useCockpit((s) =>
    s.selection.sessionId ? s.sessions[s.selection.sessionId] : undefined,
  );
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

  useLayoutEffect(() => {
    if (projectId === null && branchId === null) homeCommit(homeRender);
  }, [projectId, branchId, homeRender]);

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
        <div className="shell-workspace">
        <div className="shell-lens">
          {projectId === null && accountSession?.authOnly ? (
            <section className="account-session-workspace">
              <header>
                <div>
                  <h1>Connect your Codex account</h1>
                  <p>
                    Complete the official sign-in below. When it finishes,
                    return to save this connection for new projects.
                  </p>
                </div>
                <button
                  className="luci-button luci-primary"
                  onClick={() => setup?.openAgents()}
                >
                  Return to agent connection
                </button>
              </header>
              <div className="account-session-terminal">
                <ColumnsLayout sessionIds={[accountSession.id]} />
              </div>
            </section>
          ) : projectId === null && branchId === null ? (
            <ProjectsHome />
          ) : (
            <LensRouter key={`${projectId}:${sourceRevision}`} lens={lens} />
          )}
        </div>
        {projectId && <OrchestratorPanel projectId={projectId} />}
        </div>
      </main>
      {drawerOpen && <div className="shell-scrim" aria-hidden="true" />}
      <ContextMenu />
      <Toasts />
      <CommandPalette />
      <BranchManager />
      <DraftNavigationDialog />
    </div>
  );
}
