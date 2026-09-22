// The one create-a-session path. Contract §3.6 (inheritance and clamping), §4.4 (createSession
// is 201, not 200), §6.4 (one click for the common case, no dialog).
import { useCallback } from "react";
import { api } from "../api";
import type { ApiError } from "../api/client";
import { resolveDefaults, toCreateRequest } from "../store/defaults";
import { selectResolvedDefaults } from "../store/selectors";
import { useCockpit } from "../store/store";
import type { HarnessId, SessionDefaultsOverride, WorktreeId } from "../store/types";

export interface CreateSessionOptions {
  /** Forces a harness — "Open a shell here" passes "shell". Re-clamped like any override. */
  harness?: HarnessId;
  /** Everything the picker collected. Absent keys inherit; they never reset (§3.6). */
  overrides?: SessionDefaultsOverride;
  openingPrompt?: string;
  title?: string;
}

/** network_unavailable and backend_unavailable are different sentences and must not be
 *  collapsed (§4.2): the first means our control plane is not up, the second means Docker is
 *  not. Telling a user to start Docker when the control plane is dead costs them ten minutes.
 *  Every other code renders the server's own message verbatim — including the sixteenth,
 *  non_loopback_publish, whose sandbox is RUNNING and must never be described as stopped.
 *
 *  The `switch` this replaces would need a `default` for the same reason this has a trailing
 *  return: task 16's client passes an UNKNOWN code through intact rather than validating it,
 *  so ProblemCode is closed by convention and never by the type system. */
function sentenceFor(error: ApiError): string {
  if (error.code === "network_unavailable") {
    return "The Panacea control plane is not running, so the session was not created.";
  }
  if (error.code === "backend_unavailable") {
    return "Docker is not running on this machine, so the sandbox could not be started.";
  }
  return error.message;
}

export function useCreateSession(): (
  worktreeId: WorktreeId,
  options?: CreateSessionOptions,
) => Promise<void> {
  return useCallback(async (worktreeId: WorktreeId, options?: CreateSessionOptions) => {
    const state = useCockpit.getState();
    const wanted: SessionDefaultsOverride = {
      ...options?.overrides,
      ...(options?.harness === undefined ? {} : { harness: options.harness }),
    };
    // Re-resolve rather than spread: switching harness invalidates model, effort and
    // permission mode, and resolveDefaults is the only place that clamp lives (§3.6).
    const resolved = resolveDefaults(selectResolvedDefaults(state), wanted, undefined, state.harnesses);
    const harness = state.harnesses.find((h) => h.id === resolved.harness);
    if (harness === undefined) {
      state.pushToast("No harness is available on this machine, so no session can be started.");
      return;
    }

    state.setLoading("sessions", true);
    try {
      const session = await api.createSession(
        toCreateRequest(worktreeId, resolved, harness, {
          openingPrompt: options?.openingPrompt,
          title: options?.title,
        }),
      );
      state.upsertSession(session);
      state.setError(null);
      state.selectWorktree(worktreeId);
      // Focus is what makes defect 4's scroll-into-view fire in task 20's ColumnsLayout.
      state.focusSession(session.id);
    } catch (caught) {
      const error = caught as ApiError;
      state.setError({ code: error.code, message: error.message });
      state.pushToast(sentenceFor(error));
    } finally {
      useCockpit.getState().setLoading("sessions", false);
    }
  }, []);
}
