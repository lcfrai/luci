/** Local notifications carry identity only, never drafts or private context. */
export const PROJECT_CONTEXT_CHANGED = "luci:project-context-changed";
export const PROJECT_CONTEXT_EDIT = "luci:project-context-edit";
export type ProjectContextIdentity = { scope: string; projectId: string };
export type ContextEditor = "definition" | "design";

export function notifyProjectContextChanged(scope: string | null, projectId: string) {
  if (scope) window.dispatchEvent(new CustomEvent<ProjectContextIdentity>(PROJECT_CONTEXT_CHANGED, { detail: { scope, projectId } }));
}

// A single request survives a lazy phase mount. Its owning overview clears it
// on account/workspace/project changes; an unrelated editor cannot consume it.
let pendingEdit: (ProjectContextIdentity & { editor: ContextEditor; token: symbol }) | null = null;
export function requestProjectContextEdit(scope: string, projectId: string, editor: ContextEditor) {
  const token = Symbol();
  pendingEdit = { scope, projectId, editor, token };
  window.dispatchEvent(new Event(PROJECT_CONTEXT_EDIT));
  return () => { if (pendingEdit?.token === token) pendingEdit = null; };
}
export function takeProjectContextEdit(scope: string | null, projectId: string, editor: ContextEditor) {
  if (!scope || pendingEdit?.scope !== scope || pendingEdit.projectId !== projectId || pendingEdit.editor !== editor) return false;
  pendingEdit = null;
  return true;
}
