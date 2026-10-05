import type { SetupStatus } from "../api/product";

const PREFIX = "luci.onboarding.v1:";

/** A visit preference, never a claim that Docker, an agent or a cloud is ready. */
export function onboardingKey(status: SetupStatus, accountId?: string): string {
  return PREFIX + encodeURIComponent(JSON.stringify([
    accountId || status.access?.user.username || "local-owner",
    status.workspace?.mode || "local",
    status.workspace?.connectionId || status.config.projectsDir,
  ]));
}

export function onboardingVisited(key: string): boolean {
  try { return localStorage.getItem(key) === "visited"; }
  catch { return false; }
}

export function completeOnboardingVisit(key: string): void {
  try { localStorage.setItem(key, "visited"); }
  catch { /* The current visit still completes if this browser blocks storage. */ }
}
