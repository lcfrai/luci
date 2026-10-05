import { beforeEach, expect, it } from "vitest";
import type { SetupStatus } from "../api/product";
import { completeOnboardingVisit, onboardingKey, onboardingVisited } from "./onboarding";

const local = { config: { projectsDir: "C:/private/projects" }, workspace: { mode: "local", connectionId: "", host: "", ready: false } } as SetupStatus;
beforeEach(() => localStorage.clear());
it("scopes the visit to an account and workspace without changing readiness", () => {
  const key = onboardingKey(local, "user-1");
  completeOnboardingVisit(key);
  expect(onboardingVisited(key)).toBe(true);
  expect(onboardingVisited(onboardingKey(local, "user-2"))).toBe(false);
  expect(onboardingVisited(onboardingKey({ ...local, workspace: { ...local.workspace!, mode: "cloud", connectionId: "cloud-1" } }, "user-1"))).toBe(false);
  expect(local.workspace?.ready).toBe(false);
});
