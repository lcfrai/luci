import { productApi, type ProductClient } from "./product";

export interface PendingProjectImport {
  id: string;
  name: string;
  repository: string;
}
export interface ProjectImportAttempt {
  id: string;
  name: string;
  status: "running" | "failed" | "complete" | "interrupted" | "unconfirmed";
  message: string;
  retainedPath?: string;
  project?: { id: string; name: string; path: string };
}
const PREFIX = "luci.project-import.v1:";
const ID = /^imp-[a-f0-9]{24}$/;
const NAME = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/;
const MAX_BYTES = 8192;
const MAX_MESSAGE = 4096;
// Windows extended-length paths can exceed the usual POSIX path limit.
const MAX_PATH = 32768;
const storageError = "Import recovery could not be saved in this browser. Enable browser storage and try again before importing.";

/** Repository recovery must never persist URL credentials or token parameters. */
export function validImportRepository(value: string): boolean {
  if (value.length > 2048 || value.trim() !== value || !value.startsWith("https://") || /[\s\\]/.test(value)) return false;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && !!url.hostname && !url.username && !url.password && !url.search && !url.hash && !value.includes("?") && !value.includes("#");
  } catch { return false; }
}
export function validPendingImport(value: unknown): value is PendingProjectImport {
  if (!value || typeof value !== "object") return false;
  const item = value as PendingProjectImport;
  return typeof item.id === "string" && ID.test(item.id) && typeof item.name === "string" && NAME.test(item.name) &&
    typeof item.repository === "string" && validImportRepository(item.repository);
}
export function createPendingImport(name: string, repository: string): PendingProjectImport {
  const bytes = crypto.getRandomValues(new Uint8Array(12));
  const pending = { id: `imp-${Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("")}`, name, repository };
  if (!validPendingImport(pending)) throw new Error("Use a project name with letters, numbers, hyphens or underscores and an HTTPS repository URL without credentials, query parameters or a fragment.");
  return pending;
}
function parsePending(raw: string | null): PendingProjectImport | null {
  if (!raw || raw.length * 2 > MAX_BYTES) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!validPendingImport(value)) return null;
    // Do not preserve arbitrary fields from browser storage.
    return { id: value.id, name: value.name, repository: value.repository };
  } catch { return null; }
}
export function loadPendingImport(scope: string | null): PendingProjectImport | null {
  if (!scope) return null;
  try { return parsePending(localStorage.getItem(PREFIX + scope)); } catch { return null; }
}
/** Save before sending; never replace a different tab's pending request. */
export function savePendingImport(scope: string | null, pending: PendingProjectImport, expectedId?: string): void {
  if (!scope || !validPendingImport(pending)) throw new Error(storageError);
  try {
    const raw = localStorage.getItem(PREFIX + scope);
    const previous = parsePending(raw);
    if ((raw !== null && !previous) || (previous?.id ?? null) !== (expectedId ?? null)) {
      throw new Error("Another import is saved for this workspace. Reopen Import repository to check its status before starting another.");
    }
    const value = JSON.stringify({ id: pending.id, name: pending.name, repository: pending.repository });
    if (value.length * 2 > MAX_BYTES) throw new Error(storageError);
    localStorage.setItem(PREFIX + scope, value);
  } catch (cause) {
    if (cause instanceof Error && cause.message.startsWith("Another import")) throw cause;
    throw new Error(storageError);
  }
}
export function clearPendingImport(scope: string | null, id: string): boolean {
  if (!scope) return false;
  try {
    const key = PREFIX + scope;
    if (parsePending(localStorage.getItem(key))?.id !== id) return false;
    localStorage.removeItem(key);
    return true;
  } catch { return false; }
}
export function verifiedImportAttempt(value: unknown, id: string, name?: string): ProjectImportAttempt {
  const item = value as ProjectImportAttempt | undefined;
  if (!item || !ID.test(id) || item.id !== id || typeof item.name !== "string" || !NAME.test(item.name) || (name !== undefined && item.name !== name) ||
      typeof item.status !== "string" || !["running", "failed", "complete", "interrupted", "unconfirmed"].includes(item.status) ||
      typeof item.message !== "string" || item.message.length > MAX_MESSAGE ||
      (item.retainedPath !== undefined && (typeof item.retainedPath !== "string" || item.retainedPath.length > MAX_PATH)) ||
      (item.project !== undefined && (!item.project || typeof item.project.id !== "string" || !item.project.id || item.project.id.length > 256 || item.project.name !== item.name ||
        typeof item.project.path !== "string" || !item.project.path || item.project.path.length > MAX_PATH)) ||
      (item.status === "complete" && !item.project)) {
    throw new Error("The import response could not be verified. Keep its saved request and check its status again.");
  }
  return item;
}
export const projectImportsApi = {
  async status(id: string, client: ProductClient = productApi): Promise<ProjectImportAttempt> {
    if (!ID.test(id)) throw new Error("The saved import identity is invalid.");
    return verifiedImportAttempt(await client.request<unknown>(`/v1/product/project-imports/${encodeURIComponent(id)}`), id);
  },
};
