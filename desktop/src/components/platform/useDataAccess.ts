import { useCallback, useEffect, useRef } from "react";
import { productApi } from "../../api/product";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";

/** Private data reads require deployment and secrets grants; mutations also require editing. */
export function useDataAccess(projectId: string) {
  const access = useProjectAccess(projectId);
  const draftScope = useProjectDraftScope(projectId);
  const scope = draftScope ?? JSON.stringify([productApi.baseUrl, projectId]);
  const ready = !access.loading && !access.error;
  const canRead = ready && access.canRead && access.permissions.deploy && access.permissions.secrets;
  const canWrite = canRead && access.canEdit;
  const canExecute = canWrite && access.canExecute;
  const alive = useRef(true);
  const settled = useRef({ scope, read: false, write: false, execute: false, epoch: 0 });
  const previousError = useRef("");
  if (settled.current.scope !== scope) settled.current = { scope, read: false, write: false, execute: false, epoch: settled.current.epoch + 1 };
  // The permission provider keeps read=true only for a same-project transient
  // refresh failure. Retain the form in that case, never permission to act.
  // An error still invalidates callbacks started under the earlier grant.
  if (access.error && previousError.current !== access.error) settled.current.epoch++;
  previousError.current = access.error || "";
  if (ready || (!access.loading && !access.canRead)) {
    if ((settled.current.read && !canRead) || (settled.current.write && !canWrite) || (settled.current.execute && !canExecute)) settled.current.epoch++;
    settled.current.read = canRead; settled.current.write = canWrite; settled.current.execute = canExecute;
  }
  // A routine permission refresh disables new actions without throwing away the
  // same account's private form. Confirmed denial immediately hides it.
  const showPrivate = canRead || ((access.loading || (!!access.error && access.canRead)) && settled.current.read);
  const live = useRef({ scope, epoch: settled.current.epoch, showPrivate });
  live.current = { scope, epoch: settled.current.epoch, showPrivate };
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const capture = useCallback(() => {
    const original = live.current;
    return () => alive.current && live.current.scope === original.scope && live.current.epoch === original.epoch && live.current.showPrivate;
  }, []);
  const message = access.loading ? "Checking data permissions…"
    : access.error ? `Data access unavailable: ${access.error}`
      : !canRead ? "Ask a team owner for project read, deployment and secrets access to inspect application data and backups."
        : !canWrite ? "Read-only data access. Ask a team owner for editing access to change data services, application accounts, records and backups." : "";
  return { scope, canRead, canWrite, showPrivate, loading: access.loading, message, capture, canExecute };
}
