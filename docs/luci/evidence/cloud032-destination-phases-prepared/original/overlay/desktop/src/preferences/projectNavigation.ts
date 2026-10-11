import { homePhase, homeCount } from "../diagnostics/homePhaseTiming";
import type { Lens, Project } from "../store/types";

// Navigation is a preference, never execution consent or evidence. Its separate
// identity-scoped bucket survives workspace resets without leaking a last stage
// into another account, host, team or projects directory.
export const PROJECT_NAVIGATION_PREFIX = "luci.project-navigation.v1:";
export const MAX_NAVIGATION_PROJECTS = 64;
const MAX_SCOPES = 8;
const MAX_RECORD_BYTES = 256 * 1024;
const MAX_AGE = 90 * 24 * 60 * 60 * 1000;
const lenses = new Set<Lens>(["discover", "design", "develop", "files", "merge", "deploy"]);
export interface ProjectPosition { lens: Lens; path: string; updatedAt: number }
export interface ProjectNavigation { projects: Record<string, ProjectPosition>; selected: string | null }
const empty = (): ProjectNavigation => ({ projects: {}, selected: null });

export function navigationScope(baseUrl: string, account: string, team: string, workspace: {
  mode?: string; connectionId?: string; host?: string;
} | undefined, projectsDir: string, workspaceUser: string): string | null {
  if (!account || !projectsDir) return null;
  return JSON.stringify([baseUrl, account, team, workspace?.mode ?? "local", workspace?.connectionId ?? "", workspace?.host ?? "", workspaceUser, projectsDir]);
}

export function pruneProjectPositions(positions: Record<string, ProjectPosition>, projects: Project[]): Record<string, ProjectPosition> {
  const paths = new Map(projects.map(project => [project.id, project.path]));
  const retained = Object.entries(positions).filter(([id, record]) => paths.get(id) === record.path)
    .sort((a, b) => b[1].updatedAt - a[1].updatedAt).slice(0, MAX_NAVIGATION_PROJECTS);
  return retained.length === Object.keys(positions).length ? positions : Object.fromEntries(retained);
}

export function readProjectNavigation(scope: string): ProjectNavigation {
  try {
    const raw = localStorage.getItem(PROJECT_NAVIGATION_PREFIX + scope);
    if (!raw || raw.length * 2 > MAX_RECORD_BYTES) return empty();
    const data = JSON.parse(raw);
    const now = Date.now();
    if (data.version !== 1 || typeof data.projects !== "object" || !data.projects || Array.isArray(data.projects)) return empty();
    const records: [string, ProjectPosition][] = [];
    for (const [id, value] of Object.entries(data.projects)) {
      if (!id || ["__proto__", "constructor", "prototype"].includes(id) || id.length > 512 || !value || typeof value !== "object") continue;
      const record = value as ProjectPosition;
      if (!lenses.has(record.lens) || typeof record.path !== "string" || record.path.length > 4096 ||
          !Number.isFinite(record.updatedAt) || record.updatedAt > now + 60000 || now - record.updatedAt > MAX_AGE) continue;
      records.push([id, { lens: record.lens, path: record.path, updatedAt: record.updatedAt }]);
    }
    const projects = Object.fromEntries(records.sort((a, b) => b[1].updatedAt - a[1].updatedAt).slice(0, MAX_NAVIGATION_PROJECTS));
    return { projects, selected: typeof data.selected === "string" && Object.prototype.hasOwnProperty.call(projects, data.selected) ? data.selected : null };
  } catch { return empty(); }
}

export function writeProjectNavigation(scope: string, navigation: ProjectNavigation): boolean {
  return homePhase(4, () => {
  try {
    const key = PROJECT_NAVIGATION_PREFIX + scope;
    const raw = homePhase(5, () => {
      const projects = Object.fromEntries(Object.entries(navigation.projects)
        .sort((a, b) => b[1].updatedAt - a[1].updatedAt).slice(0, MAX_NAVIGATION_PROJECTS));
      return JSON.stringify({ version: 1, ...navigation, projects, updatedAt: Date.now() });
    });
    if (raw.length * 2 > MAX_RECORD_BYTES) return false;
    homePhase(6, () => {
    homeCount(10, localStorage.length);
    // Bound only our small preferences; never evict project drafts or application data.
    const others: { key: string; updatedAt: number }[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const stored = localStorage.key(i);
      if (!stored?.startsWith(PROJECT_NAVIGATION_PREFIX) || stored === key) continue;
      let updatedAt = 0;
      try { updatedAt = Number(JSON.parse(localStorage.getItem(stored) ?? "{}").updatedAt) || 0; } catch { /* Invalid preference is the oldest. */ }
      others.push({ key: stored, updatedAt });
    }
    for (const record of others.sort((a, b) => b.updatedAt - a.updatedAt).slice(MAX_SCOPES - 1)) localStorage.removeItem(record.key);
    homeCount(11, others.length);
    });
    homePhase(7, () => localStorage.setItem(key, raw));
    return true;
  } catch { return false; }
  });
}
