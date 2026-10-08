import type { Harness, PermissionMode } from '../store/types';
import type { ProjectRun } from './projectRuns';

export const permissionLabel = (mode: PermissionMode) => ({ ask: 'Ask', bypass: 'Bypass', plan: 'Plan' })[mode];

/** The server advertises task support separately from interactive agent modes. */
export function swarmTaskCapability(run: ProjectRun, harnesses: Harness[]) {
  const route = run.input.route, harness = harnesses.find(value => value.id === route.harness);
  const modes = Array.isArray(harness?.taskPermissionModes) ? harness.taskPermissionModes.filter(mode => ['ask', 'bypass', 'plan'].includes(mode)) : [];
  const supported = harness?.autonomousTask === true && modes.length > 0;
  const allowed = supported && modes.includes(route.permissionMode);
  const reason = !supported
    ? 'Autonomous task capability is unavailable for this build’s saved agent. Refresh its capabilities or use an interactive agent.'
    : !allowed ? `This build saves ${permissionLabel(route.permissionMode)} permission. ${harness!.name} requires an explicitly saved ${modes.map(permissionLabel).join(' or ')} permission mode for autonomous tasks. Review the plan and create a separate build to change it.` : '';
  const notice = typeof harness?.taskPermissionNotice === 'string' ? harness.taskPermissionNotice : '';
  return { allowed, reason, notice, mode: permissionLabel(route.permissionMode),
    signature: JSON.stringify([route, harness?.autonomousTask ?? null, modes, notice]) };
}
