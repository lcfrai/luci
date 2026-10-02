import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  productApi,
  type SetupStatus,
  type Workspace,
} from "../../api/product";
import { SetupPanel } from "./SetupPanel";

type SetupContextValue = {
  workspace?: Workspace;
  openSettings(): void;
  openProject(): void;
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
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [open, setOpen] = useState(false);
  const [section, setSection] = useState<"local" | "project" | "cloud">(
    "local",
  );
  useEffect(() => {
    let cancelled = false;
    productApi
      .status()
      .then((value) => {
        if (!cancelled) {
          setStatus(value);
          if (!value.configured) setOpen(true);
        }
      })
      .catch(() => {
        /* Existing cockpit remains available during service recovery. */
      });
    return () => {
      cancelled = true;
    };
  }, []);
  const openSettings = useCallback(() => {
    setSection("local");
    setOpen(true);
  }, []);
  const openProject = useCallback(() => {
    setSection("project");
    setOpen(true);
  }, []);
  const openCloud = useCallback(() => {
    setSection("cloud");
    setOpen(true);
  }, []);
  return (
    <SetupContext.Provider
      value={{
        workspace: status?.workspace,
        openSettings,
        openProject,
        openCloud,
        refreshProjects: onProjectsChanged,
      }}
    >
      {children}
      {open && (
        <SetupPanel
          initialStatus={status}
          initialSection={section}
          onStatus={setStatus}
          onClose={() => setOpen(false)}
          onProjectsChanged={onProjectsChanged}
        />
      )}
    </SetupContext.Provider>
  );
}
