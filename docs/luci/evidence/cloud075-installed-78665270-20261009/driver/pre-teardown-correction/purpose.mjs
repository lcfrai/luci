// CLOUD-064: semantic purpose field within the currently selected fixture owner.
// This is a locator only: no DOM/application state mutation or forced actions.
import assert from 'node:assert/strict';
export function purposeControl(page,teamId,projectId){
 assert(/^[A-Za-z0-9_-]{1,80}$/.test(teamId),'Exact fixture team required');
 assert(/^[A-Za-z0-9_-]{1,80}$/.test(projectId),'Exact fixture project required');
 const scope=`.team-app-host:not([hidden]):has(#team-workspace option:checked[value="${teamId}"]):has([role="treeitem"][data-kind="project"][data-id="${projectId}"][aria-selected="true"]) .definition-workspace`;
 return page.locator(scope).getByRole('textbox',{name:'The idea',exact:true});
}
