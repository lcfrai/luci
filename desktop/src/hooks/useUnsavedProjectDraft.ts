import { useEffect, useId, useLayoutEffect } from "react";
import { useCockpit } from "../store/store";
import type { Lens } from "../store/types";

/** Protect edits held by a mounted editor before changing its source or navigation. */
export function useUnsavedProjectDraft(projectId: string, dirty: boolean, label: string, lens: Lens): void {
  const id = useId();
  const register = useCockpit(state => state.setUnsavedProjectDraft);
  useLayoutEffect(() => {
    register(id, dirty ? { projectId, label, lens } : null);
    return () => register(id, null);
  }, [id, projectId, dirty, label, lens, register]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
}
