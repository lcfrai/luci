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
  expect(takeProjectContextEdit("scope", "p", "design")).toEqual({ editor: "design" });
  expect(takeProjectContextEdit("scope", "p", "design")).toBe(false);
  cancel();
});
it("clears abandoned navigation and lets a newer request survive the previous owner's cleanup", () => {
  const old = requestProjectContextEdit("scope", "p", "definition");
  const current = requestProjectContextEdit("scope", "p", "design");
  old();
  expect(takeProjectContextEdit("scope", "p", "design")).toEqual({ editor: "design" });
  current();
  const abandoned = requestProjectContextEdit("scope", "p", "definition"); abandoned();
  expect(takeProjectContextEdit("scope", "p", "definition")).toBe(false);
});
it("retains only the question identity for its exact scoped definition and never carries it into Design", () => {
  const cancel = requestProjectContextEdit("owner-workspace", "p", "definition", "membership-question");
  expect(takeProjectContextEdit(null, "p", "definition")).toBe(false);
  expect(takeProjectContextEdit("other-owner", "p", "definition")).toBe(false);
  expect(takeProjectContextEdit("owner-workspace", "another-project", "definition")).toBe(false);
  expect(takeProjectContextEdit("owner-workspace", "p", "design")).toBe(false);
  expect(takeProjectContextEdit("owner-workspace", "p", "definition")).toEqual({ editor: "definition", questionId: "membership-question" });
  cancel();
  requestProjectContextEdit("owner-workspace", "p", "design", "membership-question");
  expect(takeProjectContextEdit("owner-workspace", "p", "design")).toEqual({ editor: "design" });
});
