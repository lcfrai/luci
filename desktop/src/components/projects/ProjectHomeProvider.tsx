import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { projectHomeApi, type ProjectHome } from '../../api/projectHome';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useCockpit } from '../../store/store';
import type { Lens } from '../../store/types';

interface HomeContextValue {
  home: ProjectHome | null; error: string; busy: boolean;
  refresh(): void; pin(projectId: string, pinned: boolean): Promise<void>;
  open(projectId: string, lens?: Lens): void;
}
const ProjectHomeContext = createContext<HomeContextValue | null>(null);
export const useProjectHome = () => useContext(ProjectHomeContext);

export function ProjectHomeProvider({ children }: {children: ReactNode}) {
  const scope = useProjectDraftScope('home');
  const catalogueEpoch = useCockpit(state => state.projectCatalogueEpoch);
  const selectedProjectId = useCockpit(state => state.selection.projectId);
  const pendingNavigation = useCockpit(state => state.pendingNavigation);
  const [snapshot, setSnapshot] = useState<{scope: string; epoch: number; value: ProjectHome} | null>(null);
  const home = snapshot?.scope === scope && snapshot.epoch === catalogueEpoch ? snapshot.value : null;
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  const latest = useRef<ProjectHome | null>(null);
  latest.current = home;
  const pendingOpen = useRef<{projectId: string; stopIds: string[]} | null>(null);
  const refresh = useCallback(() => setRefreshKey(value => value + 1), []);
  useEffect(() => {
    setSnapshot(null); setError(''); pendingOpen.current = null;
  }, [scope, catalogueEpoch]);
  useEffect(() => {
    if (!scope) return;
    const controller = new AbortController();
    let active = false, timer: ReturnType<typeof setTimeout> | undefined;
    const load = async () => {
      if (active || controller.signal.aborted || document.visibilityState === 'hidden') return;
      active = true; setBusy(true);
      try {
        const value = await projectHomeApi.getAll(controller.signal);
        if (!controller.signal.aborted) { setSnapshot({scope, epoch: catalogueEpoch, value}); setError(''); }
      } catch (cause) {
        if (!controller.signal.aborted) {
          setSnapshot(null);
          setError(cause instanceof Error ? cause.message : 'Project activity is unavailable.');
        }
      } finally {
        active = false;
        if (!controller.signal.aborted) { setBusy(false); timer = setTimeout(load, 15_000); }
      }
    };
    const visible = () => { if (document.visibilityState === 'visible') { clearTimeout(timer); void load(); } };
    void load(); document.addEventListener('visibilitychange', visible);
    return () => { controller.abort(); clearTimeout(timer); document.removeEventListener('visibilitychange', visible); };
  }, [scope, catalogueEpoch, refreshKey]);
  useEffect(() => {
    const intent = pendingOpen.current;
    if (!intent || pendingNavigation) return;
    pendingOpen.current = null;
    if (selectedProjectId !== intent.projectId || intent.stopIds.length === 0 || !latest.current) return;
    void projectHomeApi.acknowledge(intent.projectId, intent.stopIds, latest.current.revision)
      .then(() => refresh()).catch(() => { setError('Stopped activity could not be acknowledged. Refresh Home before trying again.'); refresh(); });
  }, [selectedProjectId, pendingNavigation, refreshKey, refresh]);
  const open = useCallback((projectId: string, lens?: Lens) => {
    const row = latest.current?.projects.find(project => project.id === projectId);
    pendingOpen.current = { projectId, stopIds: row?.stops.map(stop => stop.id) ?? [] };
    useCockpit.getState().selectProject(projectId, lens);
    const state = useCockpit.getState();
    if (!state.pendingNavigation && state.selection.projectId === projectId) {
      const intent = pendingOpen.current; pendingOpen.current = null;
      if (intent?.stopIds.length && latest.current) {
        void projectHomeApi.acknowledge(projectId, intent.stopIds, latest.current.revision)
          .then(() => refresh()).catch(() => { setError('Stopped activity could not be acknowledged. Refresh Home before trying again.'); refresh(); });
      }
    }
  }, [refresh]);
  const pin = useCallback(async (projectId: string, pinned: boolean) => {
    const current = latest.current;
    if (!current || busy) return;
    setBusy(true);
    try { await projectHomeApi.pin(projectId, pinned, current.revision); setError(''); refresh(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Your pin could not be saved.'); refresh(); }
    finally { setBusy(false); }
  }, [busy, refresh]);
  return <ProjectHomeContext.Provider value={{home, error, busy, refresh, pin, open}}>{children}</ProjectHomeContext.Provider>;
}
