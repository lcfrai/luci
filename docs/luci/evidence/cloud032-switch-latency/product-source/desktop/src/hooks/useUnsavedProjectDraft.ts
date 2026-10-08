import { useEffect, useId, useLayoutEffect, useRef, useCallback } from "react";
import { useCockpit } from "../store/store";
import type { Lens } from "../store/types";
import type { UnsavedProjectDraft } from "../store/slices/uiSlice";

/** Protect edits held by a mounted editor before changing its source or navigation. */
export function useUnsavedProjectDraft(projectId: string, dirty: boolean, label: string, lens: Lens, recovery?: { retained: boolean; discard(): void }, purpose?: UnsavedProjectDraft["purpose"]): void {
  const id = useId();
  const register = useCockpit(state => state.setUnsavedProjectDraft);
  const callback = useRef(recovery?.discard); callback.current = recovery?.discard;
  const discard = useCallback(() => callback.current?.(), []);
  const retained = recovery?.retained;
  useLayoutEffect(() => {
    register(id, dirty ? { projectId, label, lens, ...(purpose ? { purpose } : {}), ...(recovery ? { retained, discard } : {}) } : null);
    return () => register(id, null);
  }, [id, projectId, dirty, label, lens, register, retained, discard, purpose]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
}
