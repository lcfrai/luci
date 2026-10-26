import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  productApi,
  type SetupStatus,
  type Workspace,
} from "../../api/product";
import { SetupPanel } from "./SetupPanel";
import { ProjectEntry, type ProjectEntryMode } from "../projects/ProjectEntry";
import { useCockpit } from "../../store/store";
import { errorText } from "../../api/product";
import { useTeamWorkspace } from "../teams/TeamWorkspaceProvider";
import { TeamSettings } from "../teams/TeamSettings";
import { useAuth } from "../auth/AuthProvider";
import { navigationScope } from "../../preferences/projectNavigation";
import { completeOnboardingVisit, onboardingKey, onboardingVisited } from "../../preferences/onboarding";

type SetupContextValue = {
  workspace?: Workspace;
  status: SetupStatus | null;
  statusError: string;
  retryStatus(): void;
  openSettings(target?: "git-identity"): void;
  openAgents(): void;
  openProject(mode?: ProjectEntryMode): void;
  openOnboarding(): void;
  openCloud(): void;
  refreshProjects(): void;
  retryBranches?(problem: import('../../store/slices/dataSlice').CatalogueProblem): void;
};
const SetupContext = createContext<SetupContextValue | null>(null);
export const useSetup = () => useContext(SetupContext);

export function SetupProvider({
  children,
  onProjectsChanged,
  onRetryBranches,
}: {
  children: ReactNode;
  onProjectsChanged(): void;
  onRetryBranches?(problem: import('../../store/slices/dataSlice').CatalogueProblem): void;
}) {
  const auth = useAuth();
  const team = useTeamWorkspace();
  const teamWorkspace = !!team?.teamId;
  const canCreate = !teamWorkspace || !!(team?.selected?.permissions.edit && team.selected.permissions.admin);
  const projects = useCockpit(state => state.projects);
  const projectsLoaded = useCockpit(state => state.projectsLoaded);
  const projectsLoading = useCockpit(state => state.loading.projects);
  const evaluatedVisit = useRef("");
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [open, setOpen] = useState(false);
  const [gitIdentityRequest, setGitIdentityRequest] = useState<number>();
  const revealSequence = useRef(0);
  const [portal, setPortal] = useState(false);
  const [entry, setEntry] = useState<ProjectEntryMode | null>(null);
  const [statusError, setStatusError] = useState("");
  const [statusLoading, setStatusLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);
  const [section, setSection] = useState<
    "local" | "project" | "cloud" | "agents"
  >("local");
  useEffect(() => {
    let cancelled = false;
    setStatusError("");
    setStatusLoading(true);
    productApi
      .status()
      .then((value) => {
        if (!cancelled) {
          setStatus(value);
        }
      })
      .catch((cause) => {
        if (!cancelled) setStatusError(errorText(cause));
      })
      .finally(() => {
        if (!cancelled) setStatusLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [attempt]);
  const currentNavigationScope = useCockpit(state => state.projectNavigationScope);
  const navigationError = useCockpit(state => state.projectNavigationError);
  const scope = navigationScope(productApi.baseUrl, auth?.profile.user.id ?? "", team?.teamId ?? "",
    status?.workspace, status?.config.projectsDir ?? "", status?.access?.user.username ?? "");
  useLayoutEffect(() => { if (scope) useCockpit.getState().bindProjectNavigation(scope); }, [scope]);
  const visitKey = status ? onboardingKey(status, auth?.profile.user.id) : "";
  useEffect(() => {
    if (!visitKey || !projectsLoaded || evaluatedVisit.current === visitKey) return;
    evaluatedVisit.current = visitKey;
    // Existing work takes people back to their retained project; an assigned
    // empty folder still needs the first-visit workspace choice.
    if (projects.length === 0 && !onboardingVisited(visitKey) && !teamWorkspace) setPortal(true);
  }, [visitKey, projectsLoaded, projects.length, teamWorkspace]);
  const finishVisit = useCallback(() => {
    if (visitKey) completeOnboardingVisit(visitKey);
    setPortal(false);
  }, [visitKey]);
  const openSettings = useCallback((target?: "git-identity") => {
    setGitIdentityRequest(target === "git-identity" ? ++revealSequence.current : undefined);
    setSection("local");
    setOpen(true);
  }, []);
  const openProject = useCallback((mode: ProjectEntryMode = "new") => {
    if (!canCreate) { useCockpit.getState().pushToast("Ask your team administrator to create or share a project."); return; }
    setGitIdentityRequest(undefined);
    setOpen(false);
    finishVisit();
    setEntry(mode);
  }, [finishVisit, canCreate]);
  const openCloud = useCallback(() => {
    setGitIdentityRequest(undefined);
    setSection("cloud");
    setOpen(true);
  }, []);
  return (
    <SetupContext.Provider
      value={{
        workspace: status?.workspace,
        status,
        statusError,
        retryStatus: () => setAttempt((value) => value + 1),
        openSettings,
        openAgents: () => {
          setGitIdentityRequest(undefined);
          setSection("agents");
          setOpen(true);
        },
        openProject,
        openOnboarding: () => {
          setGitIdentityRequest(undefined);
          setSection("local");
          setOpen(false);
          setPortal(true);
        },
        openCloud,
        refreshProjects: onProjectsChanged,
        retryBranches: onRetryBranches,
      }}
    >
      {teamWorkspace && (open || portal) && <TeamSettings status={status} onStatus={setStatus} onClose={() => {setOpen(false);finishVisit();}} onSessionStarted={() => {setOpen(false);finishVisit();}}/>}
      {statusLoading || (!!scope && scope !== currentNavigationScope) || (!projectsLoaded && projectsLoading !== false) ? (
        <main className="workspace-loading">
          <strong>Luci</strong>
          <h1>Opening your workspace</h1>
          <p role="status">Checking your workspace connection…</p>
        </main>
      ) : portal && !teamWorkspace ? (
        <SetupPanel
          presentation="portal"
          initialStatus={status}
          initialSection={section}
          onStatus={setStatus}
          onClose={finishVisit}
          onOpenProject={() => openProject()}
          onSessionStarted={() => {
            setPortal(false);
            setOpen(false);
            setGitIdentityRequest(undefined);
          }}
          onProjectsChanged={onProjectsChanged}
        />
      ) : (
        <>{navigationError && <p role="status" className="journey-error">Your last stage could not be saved in this browser. Your saved project work is unaffected.</p>}{children}</>
      )}
      {open && !portal && !teamWorkspace && (
        <SetupPanel
          initialStatus={status}
          initialSection={section}
          gitIdentityRequest={gitIdentityRequest}
          onStatus={setStatus}
          onClose={() => { setOpen(false); setGitIdentityRequest(undefined); }}
          onOpenProject={() => openProject()}
          onSessionStarted={() => { setOpen(false); setGitIdentityRequest(undefined); }}
          onProjectsChanged={onProjectsChanged}
        />
      )}
      {entry && canCreate && (
        <ProjectEntry
          mode={entry}
          onClose={() => setEntry(null)}
          onCreated={(project) => {
            const state = useCockpit.getState();
            state.setProjects([...state.projects.filter(item => item.id !== project.id), project]);
            state.selectProject(project.id, "discover");
            onProjectsChanged();
            setEntry(null);
          }}
        />
      )}
    </SetupContext.Provider>
  );
}
