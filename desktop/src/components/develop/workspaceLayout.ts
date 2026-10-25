import { preferenceServiceIdentity } from '../../preferences/serviceIdentity';

export interface WorkspaceLayout { mode: 'tiles' | 'focus'; columnWidth: number }
export const DEFAULT_WORKSPACE_LAYOUT: WorkspaceLayout = { mode: 'focus', columnWidth: 520 };
const PREFIX = 'luci.develop-layout.v1:';

export function workspaceLayoutKey(scope: string | null, branchId: string | null): string | null {
  return scope && branchId ? PREFIX + JSON.stringify([layoutPreferenceScope(scope), branchId]) : null;
}

function layoutPreferenceScope(scope: string): string {
  try {
    // Normalize only this preference's copy of the known project-draft tuple.
    // Document recovery and its authority boundaries keep their original scope.
    const parts: unknown = JSON.parse(scope);
    if (!Array.isArray(parts) || parts.length !== 8 || !parts.every(part => typeof part === 'string')) return scope;
    const base = parts[0] as string;
    const service = base.replace(/\/v1\/teams\/[^/]+\/workspace$/, '');
    const identity = preferenceServiceIdentity(service);
    if (identity === service) return scope; // Remote, external and unbound services stay exact.
    // The service helper strips team routes for its original owner-key use.
    // Layout preferences must retain that workspace route as well as actor,
    // backing, project root, project and branch identity.
    return JSON.stringify([[identity, base.slice(service.length)], ...parts.slice(1)]);
  } catch { return scope; }
}

export function readWorkspaceLayout(key: string | null): WorkspaceLayout {
  if (!key) return { ...DEFAULT_WORKSPACE_LAYOUT };
  const raw = localStorage.getItem(key);
  if (!raw) return { ...DEFAULT_WORKSPACE_LAYOUT };
  if (raw.length > 256) throw new Error('Saved workspace layout is too large.');
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== 'object') throw new Error('Saved workspace layout is invalid.');
  const layout = value as Partial<WorkspaceLayout>;
  if ((layout.mode !== 'tiles' && layout.mode !== 'focus') || !Number.isInteger(layout.columnWidth) || layout.columnWidth! < 320 || layout.columnWidth! > 960) {
    throw new Error('Saved workspace layout is invalid.');
  }
  return { mode: layout.mode, columnWidth: layout.columnWidth! };
}
