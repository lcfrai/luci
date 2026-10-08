import type { CheckRun } from './product';

export interface CheckRecoveryReview {
  projectId: string; checkId: string; revision: number;
  state: 'available' | 'unavailable' | 'pending' | 'complete';
  method: 'remove-exact' | 'fence-launch' | null;
  commandIndex: number | null; recoveryDigest: string | null;
  reasonCode: string; message: string;
}
const statuses = ['queued', 'running', 'passed', 'failed', 'cancelled', 'unknown', 'skipped', 'blocked'];
const record = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, limit: number, empty = true): value is string => typeof value === 'string' && value.length <= limit && (empty || !!value.trim());
const revision = (value: unknown) => Number.isSafeInteger(value) && (value as number) >= 1;
const malformed = () => new Error('This check response could not be verified. Refresh its history before continuing.');

/** New protocol fields must be complete; historical checks never gain recovery authority. */
export function checkRunResult(projectId: string, value: unknown, checkId?: string): CheckRun {
  if (!record(value) || value.projectId !== projectId || !text(value.id, 200, false) || checkId && value.id !== checkId ||
    !Array.isArray(value.commands) || value.commands.length > 8 || !text(value.status, 80, false)) throw malformed();
  const modern = value.revision !== undefined && value.revision !== 0;
  if (modern && (!value.commands.length || !revision(value.revision) || !statuses.includes(value.status) ||
    !(typeof value.resourcesHeld === 'boolean' || value.resourcesHeld === null) ||
    value.stage !== undefined && !text(value.stage, 200) || value.reasonCode !== undefined && !text(value.reasonCode, 200))) throw malformed();
  const commands = value.commands.map((command): Record<string, unknown> => {
    if (!record(command) || !text(command.label, 200, false) || !text(command.executable, 80, false) ||
      !(command.args === null || Array.isArray(command.args) && command.args.length <= 100 && command.args.every(arg => typeof arg === 'string')) ||
      !text(command.status, 80, false) || !text(command.output, 262_144) || typeof command.truncated !== 'boolean') throw malformed();
    if (modern && (!statuses.includes(command.status) || !text(command.allocationId, 200, false) || typeof command.cleanupConfirmed !== 'boolean' ||
      command.stage !== undefined && !text(command.stage, 200) || command.reasonCode !== undefined && !text(command.reasonCode, 200) ||
      command.message !== undefined && !text(command.message, 4000))) throw malformed();
    return {...command, args: command.args ?? []};
  });
  if (modern && value.status === 'passed' && (value.resourcesHeld !== false || commands.some(command => command.cleanupConfirmed !== true || command.status !== 'passed' || command.exitCode !== 0))) throw malformed();
  return {...value, commands} as unknown as CheckRun;
}

export function checkRecoveryResult(projectId: string, checkId: string, value: unknown): CheckRecoveryReview {
  if (!record(value) || value.projectId !== projectId || value.checkId !== checkId || !revision(value.revision) ||
    !['available', 'unavailable', 'pending', 'complete'].includes(String(value.state)) ||
    ![null, 'remove-exact', 'fence-launch'].includes(value.method as string | null) ||
    !(value.commandIndex === null || Number.isInteger(value.commandIndex) && (value.commandIndex as number) >= 0 && (value.commandIndex as number) < 8) ||
    !(value.recoveryDigest === null || typeof value.recoveryDigest === 'string' && /^[a-f0-9]{64}$/.test(value.recoveryDigest)) ||
    !text(value.reasonCode, 200) || !text(value.message, 4000)) throw malformed();
  if (value.state === 'available' && (value.method === null || value.commandIndex === null || value.recoveryDigest === null)) throw malformed();
  return value as unknown as CheckRecoveryReview;
}

export function checkCanRecover(value: CheckRecoveryReview): boolean {
  return ['available', 'pending'].includes(value.state) && value.method !== null && value.commandIndex !== null && !!value.recoveryDigest;
}
