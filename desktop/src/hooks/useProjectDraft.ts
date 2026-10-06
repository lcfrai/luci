import { useLayoutEffect, useRef, useState } from "react";
import { productApi } from "../api/product";
import { useAuth } from "../components/auth/AuthProvider";
import { useSetup } from "../components/setup/SetupProvider";

const PREFIX = "luci.project-draft.v1:";
const MAX_BYTES = 2 * 1024 * 1024;
const TOTAL_BYTES = 4 * 1024 * 1024;
const MAX_AGE = 30 * 24 * 60 * 60 * 1000;
export interface ProjectDraft<T> { version: 1; base: string; value: T; original?: T; updatedAt: number }
export function useProjectDraftScope(projectId: string): string | null {
  const auth = useAuth(), setup = useSetup();
  const workspace = setup?.workspace, status = setup?.status;
  // Never create an anonymous bucket. A local account can select a remote
  // workspace account; both identities and the actual API route are required.
  if (!auth?.profile.user.id || !status || !status.config?.projectsDir) return null;
  return JSON.stringify([productApi.baseUrl, auth.profile.user.id, workspace?.mode ?? "local",
    workspace?.connectionId ?? "", workspace?.host ?? "", status.access?.user.username ?? "",
    status.config.projectsDir, projectId]);
}
export interface ProjectDraftState<T> {
  pending: ProjectDraft<T> | null;
  retained: boolean;
  restored: boolean;
  error: string;
  take(): ProjectDraft<T> | null;
  clear(): void;
  offerRecovery(): void;
}
/** Only project documents and command drafts enter this hook. Runtime consent and credentials do not. */
export function useProjectDraft<T>({ scope, kind, base, baseValue, value, dirty, valid }: {
  scope: string | null; kind: "definition" | "design-plan" | "sketch" | "implementation-plan" | "verification-setup";
  base: string | null; baseValue?: T | null; value: T | null; dirty: boolean; valid(value: unknown): value is T;
}): ProjectDraftState<T> {
  const key = scope ? PREFIX + JSON.stringify([scope, kind]) : null;
  const expected = useRef<string | null>(null);
  const [error, setError] = useState("");
  const [retained, setRetained] = useState(false);
  const [restored, setRestored] = useState(false);
  const [pending, setPending] = useState<ProjectDraft<T> | null>(() => {
    if (!key) return null;
    try {
      const raw = localStorage.getItem(key); expected.current = raw;
      if (!raw) return null;
      const record = JSON.parse(raw) as ProjectDraft<unknown>;
      if (raw.length * 2 > MAX_BYTES || record.version !== 1 || typeof record.base !== "string" ||
          !Number.isFinite(record.updatedAt) || record.updatedAt > Date.now() + 60000 || !valid(record.value)) return null;
      if (Date.now() - record.updatedAt > MAX_AGE) { localStorage.removeItem(key); expected.current = null; return null; }
      // Older drafts have no original document. Keep their edits recoverable,
      // but never infer a merge baseline from a newer saved revision.
      return { ...record, original: valid(record.original) ? record.original : undefined } as ProjectDraft<T>;
    } catch { return null; }
  });
  const latest = useRef({ value, base, baseValue }); latest.current = { value, base, baseValue };
  function clear() {
    if (key) {
      try {
        // Another tab's draft is never deleted by a late save in this tab.
        if (localStorage.getItem(key) === expected.current) { localStorage.removeItem(key); expected.current = null; }
      } catch { setError("Browser storage is unavailable. This draft could not be removed from this browser."); return; }
    }
    setPending(null); setRetained(false); setRestored(false); setError("");
  }
  useLayoutEffect(() => {
    if (!dirty || !value || !base || pending) return;
    const localSetup = kind === "verification-setup";
    if (!key) { setError(`Draft recovery is unavailable until your account and workspace are known. ${localSetup ? "Keep this page open or copy your command text before leaving." : "Keep this page open until you save."}`); return; }
    try {
      if (localSetup && !valid(value)) throw new Error("This check setup exceeds the browser recovery limits. Keep this page open or copy your command text before leaving.");
      if (localStorage.getItem(key) !== expected.current) throw new Error(`Another tab changed this browser draft. Your edits are still here; ${localSetup ? "copy your command text before leaving this page." : "save them to the project before leaving this page."}`);
      const record: ProjectDraft<T> = { version: 1, base, value, ...(baseValue ? { original: baseValue } : {}), updatedAt: Date.now() };
      const raw = JSON.stringify(record);
      if (raw.length * 2 > MAX_BYTES) throw new Error(`This draft is too large for browser recovery. ${localSetup ? "Keep this page open or copy your command text before leaving." : "Keep this page open until you save it to the project."}`);
      let bytes = raw.length * 2;
      for (let i = localStorage.length - 1; i >= 0; i--) {
        const other = localStorage.key(i);
        if (!other?.startsWith(PREFIX) || other === key) continue;
        const stored = localStorage.getItem(other) ?? "";
        try {
          const candidate = JSON.parse(stored) as ProjectDraft<unknown>;
          if (Number.isFinite(candidate.updatedAt) && Date.now() - candidate.updatedAt > MAX_AGE) { localStorage.removeItem(other); continue; }
        } catch { /* Preserve unknown entries rather than removing another edit. */ }
        bytes += stored.length * 2;
      }
      if (bytes > TOTAL_BYTES) throw new Error("Browser draft storage is full. Save or discard an existing draft before leaving this page.");
      localStorage.setItem(key, raw); expected.current = raw;
      setRetained(true); setError("");
    } catch (cause) {
      setRetained(false);
      setError(cause instanceof Error && !["QuotaExceededError", "SecurityError"].includes(cause.name) ? cause.message : localSetup ? "Browser storage is unavailable or full. Keep this page open or copy your command text before leaving." : "Browser storage is unavailable or full. Keep this page open until you save your changes.");
    }
  }, [key, kind, dirty, base, baseValue, value, pending, valid]);
  return {
    pending, retained, restored, error,
    take: () => { const record = pending; setPending(null); setRestored(!!record); return record; },
    clear,
    offerRecovery: () => {
      const snapshot = latest.current;
      if (snapshot.value && snapshot.base && dirty) setPending({ version: 1, base: snapshot.base, value: snapshot.value, ...(snapshot.baseValue ? { original: snapshot.baseValue } : {}), updatedAt: Date.now() });
    },
  };
}

/** Validate saved drafts against the document shape before rendering them. */
export function draftMatches(value: unknown, example: unknown): boolean {
  if (example === null || example === undefined) return true;
  if (Array.isArray(example)) return Array.isArray(value) && (example.length === 0 || value.every(item => draftMatches(item, example[0])));
  if (typeof example === "object") return !!value && typeof value === "object" && !Array.isArray(value) &&
    Object.entries(example).every(([key, child]) => draftMatches((value as Record<string, unknown>)[key], child));
  return typeof value === typeof example;
}
