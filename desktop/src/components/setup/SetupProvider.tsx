import {
  createContext,
  useCallback,
  useContext,
  useEffect,
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
import { useAuth } from "../auth/AuthProvider";
import { completeOnboardingVisit, onboardingKey, onboardingVisited } from "../../preferences/onboarding";

type SetupContextValue = {
  workspace?: Workspace;
  status: SetupStatus | null;
  statusError: string;
  retryStatus(): void;
  openSettings(): void;
  openAgents(): void;
  openProject(mode?: ProjectEntryMode): void;
  openOnboarding(): void;
  openCloud(): void;
  refreshProjects(): void;
};
const SetupContext = createContext<SetupContextValue | null>(null);
export const useSetup = () => useContext(SetupContext);

export function SetupProvider({
  children,
  onProjectsChanged,
}: {
  children: ReactNode;
  onProjectsChanged(): void;
}) {
  const auth = useAuth();
  const projects = useCockpit(state => state.projects);
  const projectsLoaded = useCockpit(state => state.projectsLoaded);
  const projectsLoading = useCockpit(state => state.loading.projects);
  const evaluatedVisit = useRef("");
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [open, setOpen] = useState(false);
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
  const visitKey = status ? onboardingKey(status, auth?.profile.user.id) : "";
  useEffect(() => {
    if (!visitKey || !projectsLoaded || evaluatedVisit.current === visitKey) return;
    evaluatedVisit.current = visitKey;
    // Existing work takes people back to their retained project; an assigned
    // empty folder still needs the first-visit workspace choice.
    if (projects.length === 0 && !onboardingVisited(visitKey)) setPortal(true);
  }, [visitKey, projectsLoaded, projects.length]);
  const finishVisit = useCallback(() => {
    if (visitKey) completeOnboardingVisit(visitKey);
    setPortal(false);
  }, [visitKey]);
  const openSettings = useCallback(() => {
    setSection("local");
    setOpen(true);
  }, []);
  const openProject = useCallback((mode: ProjectEntryMode = "new") => {
    setOpen(false);
    finishVisit();
    setEntry(mode);
  }, [finishVisit]);
  const openCloud = useCallback(() => {
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
          setSection("agents");
          setOpen(true);
        },
        openProject,
        openOnboarding: () => {
          setSection("local");
          setOpen(false);
          setPortal(true);
        },
        openCloud,
        refreshProjects: onProjectsChanged,
      }}
    >
      {statusLoading || (!projectsLoaded && projectsLoading !== false) ? (
        <main className="workspace-loading">
          <strong>Luci</strong>
          <h1>Opening your workspace</h1>
          <p role="status">Checking your workspace connection…</p>
        </main>
      ) : portal ? (
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
          }}
          onProjectsChanged={onProjectsChanged}
        />
      ) : (
        children
      )}
      {open && !portal && (
        <SetupPanel
          initialStatus={status}
          initialSection={section}
          onStatus={setStatus}
          onClose={() => setOpen(false)}
          onOpenProject={() => openProject()}
          onSessionStarted={() => setOpen(false)}
          onProjectsChanged={onProjectsChanged}
        />
      )}
      {entry && (
        <ProjectEntry
          mode={entry}
          onClose={() => setEntry(null)}
          onCreated={(project) => {
            onProjectsChanged();
            useCockpit.getState().selectProject(project.id);
            useCockpit.getState().setLens("discover");
            setEntry(null);
          }}
        />
      )}
    </SetupContext.Provider>
  );
}
