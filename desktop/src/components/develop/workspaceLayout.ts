export interface WorkspaceLayout { mode: 'tiles' | 'focus'; columnWidth: number }
export const DEFAULT_WORKSPACE_LAYOUT: WorkspaceLayout = { mode: 'tiles', columnWidth: 520 };
const PREFIX = 'luci.develop-layout.v1:';

export function workspaceLayoutKey(scope: string | null, branchId: string | null): string | null {
  return scope && branchId ? PREFIX + JSON.stringify([scope, branchId]) : null;
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
