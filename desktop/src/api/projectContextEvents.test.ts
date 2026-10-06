import { expect, it } from "vitest";
import { notifyProjectContextChanged, PROJECT_CONTEXT_CHANGED, requestProjectContextEdit, takeProjectContextEdit } from "./projectContextEvents";

it("publishes only the scoped identity of a saved change and refuses anonymous notifications", () => {
  const events: unknown[] = [];
  const receive = (event: Event) => events.push((event as CustomEvent).detail);
  window.addEventListener(PROJECT_CONTEXT_CHANGED, receive);
  try {
    notifyProjectContextChanged(null, "p");
    notifyProjectContextChanged("actor-host-team-root", "p");
    expect(events).toEqual([{ scope: "actor-host-team-root", projectId: "p" }]);
  } finally { window.removeEventListener(PROJECT_CONTEXT_CHANGED, receive); }
});
it("retains the exact manual target through a delayed mount and refuses other projects, workspaces or editors", () => {
  const cancel = requestProjectContextEdit("scope", "p", "design");
  expect(takeProjectContextEdit("other", "p", "design")).toBe(false);
  expect(takeProjectContextEdit("scope", "other", "design")).toBe(false);
  expect(takeProjectContextEdit("scope", "p", "definition")).toBe(false);
  expect(takeProjectContextEdit("scope", "p", "design")).toBe(true);
  expect(takeProjectContextEdit("scope", "p", "design")).toBe(false);
  cancel();
});
it("clears abandoned navigation and lets a newer request survive the previous owner's cleanup", () => {
  const old = requestProjectContextEdit("scope", "p", "definition");
  const current = requestProjectContextEdit("scope", "p", "design");
  old();
  expect(takeProjectContextEdit("scope", "p", "design")).toBe(true);
  current();
  const abandoned = requestProjectContextEdit("scope", "p", "definition"); abandoned();
  expect(takeProjectContextEdit("scope", "p", "definition")).toBe(false);
});
