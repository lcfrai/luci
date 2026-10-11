import type { RuntimeJob } from "./product";

export function runtimeInstallActive(job: RuntimeJob | null | undefined): boolean {
  return !!job && ["installing", "cancelling", "reconciling"].includes(job.status);
}

export function runtimeInstallRetained(job: RuntimeJob | null | undefined): boolean {
  return job?.execution?.resourcesHeld === true || job?.status === "unknown";
}
