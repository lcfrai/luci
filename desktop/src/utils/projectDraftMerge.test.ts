import { describe, expect, it } from "vitest";
import { mergeProjectDraft } from "./projectDraftMerge";

const key = (...path: string[]) => JSON.stringify(path);
const requirement = (id: string, title = id, acceptance = "Works") => ({ id, title, acceptance });

describe("mergeProjectDraft", () => {
  it("keeps Alice's constraints and Bob's independent requirement edit", () => {
    const baseline = { constraints: "Local only", requirements: [requirement("r1")] };
    const draft = { ...baseline, constraints: "Local only; keyboard accessible" };
    const saved = { ...baseline, requirements: [requirement("r1", "Invite a teammate")] };
    const result = mergeProjectDraft(baseline, draft, saved);
    expect(result).toEqual({ value: { ...saved, constraints: draft.constraints }, conflicts: [], unresolved: 0 });
  });

  it("combines edits to different fields of the same identified record", () => {
    const baseline = { requirements: [requirement("r1")] };
    const draft = { requirements: [requirement("r1", "Invite a teammate")] };
    const saved = { requirements: [requirement("r1", "r1", "Expired invitations are refused")] };
    expect(mergeProjectDraft(baseline, draft, saved)).toEqual({
      value: { requirements: [requirement("r1", "Invite a teammate", "Expired invitations are refused")] },
      conflicts: [], unresolved: 0,
    });
  });

  it("requires an explicit same-field choice and keeps stable JSON path keys", () => {
    const id = 'requirement/with["punctuation"]';
    const baseline = { requirements: [requirement(id, "Before")] };
    const draft = { requirements: [requirement(id, "Alice")] };
    const saved = { requirements: [requirement(id, "Bob")] };
    const path = ["requirements", `id:${id}`, "title"];
    const pending = mergeProjectDraft(baseline, draft, saved);
    expect(pending.unresolved).toBe(1);
    expect(pending.value).toEqual(saved);
    expect(pending.conflicts).toEqual([{ key: JSON.stringify(path), path, baseline: "Before", draft: "Alice", saved: "Bob" }]);
    for (const choice of ["draft", "saved"] as const) {
      const resolved = mergeProjectDraft(baseline, draft, saved, { [JSON.stringify(path)]: choice });
      expect(resolved.unresolved).toBe(0);
      expect(resolved.conflicts).toEqual(pending.conflicts);
      expect(resolved.value).toEqual(choice === "draft" ? draft : saved);
    }
  });

  it("preserves disjoint additions, their IDs and placement between shared anchors", () => {
    const baseline = { tasks: [requirement("first"), requirement("last")] };
    const draft = { tasks: [requirement("first"), requirement("alice1"), requirement("alice2"), requirement("last")] };
    const saved = { tasks: [requirement("first"), requirement("bob"), requirement("last")] };
    const result = mergeProjectDraft(baseline, draft, saved);
    expect(result.unresolved).toBe(0);
    expect(result.value.tasks.map(task => task.id)).toEqual(["first", "bob", "alice1", "alice2", "last"]);
  });

  it("keeps an unchanged-side removal while merging another record's edit", () => {
    const baseline = { screens: [requirement("removed"), requirement("kept")] };
    const draft = { screens: [requirement("kept")] };
    const saved = { screens: [requirement("removed"), requirement("kept", "Edited")] };
    expect(mergeProjectDraft(baseline, draft, saved)).toEqual({ value: { screens: [requirement("kept", "Edited")] }, conflicts: [], unresolved: 0 });
  });

  it.each(["draft", "saved"] as const)("requires explicit %s choice for deletion versus concurrent edit", choice => {
    const baseline = { requirements: [requirement("r1")] };
    const draft = { requirements: [] };
    const saved = { requirements: [requirement("r1", "New title")] };
    const conflictKey = key("requirements", "id:r1");
    const pending = mergeProjectDraft(baseline, draft, saved);
    expect(pending.unresolved).toBe(1);
    expect(pending.conflicts[0]).toEqual({ key: conflictKey, path: ["requirements", "id:r1"], baseline: baseline.requirements[0], draft: undefined, saved: saved.requirements[0] });
    const resolved = mergeProjectDraft(baseline, draft, saved, { [conflictKey]: choice });
    expect(resolved.unresolved).toBe(0);
    expect(resolved.value).toEqual(choice === "draft" ? draft : saved);
    const reversed = mergeProjectDraft(baseline, saved, draft, { [conflictKey]: choice });
    expect(reversed.unresolved).toBe(0);
    expect(reversed.value).toEqual(choice === "draft" ? saved : draft);
  });

  it("accepts a unilateral reorder and retains additions from both sides", () => {
    const baseline = { tasks: [requirement("a"), requirement("b"), requirement("c")] };
    const draft = { tasks: [requirement("b"), requirement("a"), requirement("alice"), requirement("c")] };
    const saved = { tasks: [requirement("a"), requirement("b"), requirement("bob"), requirement("c")] };
    const result = mergeProjectDraft(baseline, draft, saved);
    expect(result.conflicts).toEqual([]);
    expect(result.value.tasks.map(task => task.id)).toEqual(["b", "a", "alice", "bob", "c"]);
  });

  it("requires a competing reorder choice without discarding independent added records", () => {
    const baseline = { tasks: [requirement("a"), requirement("b"), requirement("c")] };
    const draft = { tasks: [requirement("b"), requirement("a"), requirement("alice"), requirement("c")] };
    const saved = { tasks: [requirement("a"), requirement("c"), requirement("bob"), requirement("b")] };
    const conflictKey = key("tasks", "$order");
    expect(mergeProjectDraft(baseline, draft, saved).conflicts).toEqual([{
      key: conflictKey, path: ["tasks", "$order"], baseline: ["a", "b", "c"],
      draft: ["b", "a", "alice", "c"], saved: ["a", "c", "bob", "b"],
    }]);
    for (const choice of ["draft", "saved"] as const) {
      const result = mergeProjectDraft(baseline, draft, saved, { [conflictKey]: choice });
      expect(result.unresolved).toBe(0);
      expect(result.value.tasks.map(task => task.id).sort()).toEqual(["a", "alice", "b", "bob", "c"]);
      expect(result.value.tasks.filter(task => ["a", "b", "c"].includes(task.id)).map(task => task.id)).toEqual(choice === "draft" ? ["b", "a", "c"] : ["a", "c", "b"]);
    }
  });

  it("does not need an order choice for the same concurrent reorder", () => {
    const baseline = { tasks: [requirement("a"), requirement("b")] };
    const draft = { tasks: [requirement("b", "Changed title"), requirement("a")] };
    const saved = { tasks: [requirement("b", "b", "Changed acceptance"), requirement("a")] };
    const result = mergeProjectDraft(baseline, draft, saved);
    expect(result.conflicts).toEqual([]);
    expect(result.value.tasks).toEqual([requirement("b", "Changed title", "Changed acceptance"), requirement("a")]);
  });

  it("uses an order choice for conflicting placement of shared newly added IDs", () => {
    const baseline = { tasks: [] as ReturnType<typeof requirement>[] };
    const draft = { tasks: [requirement("a"), requirement("b")] };
    const saved = { tasks: [requirement("b"), requirement("a")] };
    expect(mergeProjectDraft(baseline, draft, saved).conflicts.map(conflict => conflict.key)).toEqual([key("tasks", "$order")]);
  });

  it("treats plain lists and duplicate or missing IDs atomically", () => {
    const cases = [
      { baseline: ["a"], draft: ["a", "draft"], saved: ["a", "saved"] },
      { baseline: [requirement("a")], draft: [requirement("a"), requirement("a", "duplicate")], saved: [requirement("a", "edited")] },
      { baseline: [{ name: "a" }], draft: [{ name: "draft" }], saved: [{ name: "saved" }] },
    ];
    for (const { baseline, draft, saved } of cases) {
      const result = mergeProjectDraft<unknown>({ values: baseline }, { values: draft }, { values: saved });
      expect(result.unresolved).toBe(1);
      expect(result.conflicts[0].key).toBe(key("values"));
    }
  });

  it("always retains saved root metadata, including its absence, without suppressing nested task status edits", () => {
    const metadata = { projectId: "p", version: 1, revision: 2, briefRevision: 3, specificationRevision: 4, specificationDigest: "spec", sketchRevision: 5, status: "approved", digest: "digest", updatedAt: "today" };
    const baseline = { ...metadata, purpose: "Before", tasks: [{ id: "t", status: "todo" }] };
    const draft = { ...Object.fromEntries(Object.keys(metadata).map(name => [name, "stale"])), purpose: "After", tasks: [{ id: "t", status: "done" }], sketchRevision: 9 };
    const saved = { ...baseline } as Record<string, unknown>;
    delete saved.sketchRevision;
    const result = mergeProjectDraft<Record<string, unknown>>(baseline, draft, saved);
    expect(result.unresolved).toBe(0);
    expect(result.value).toEqual({ ...saved, purpose: "After", tasks: [{ id: "t", status: "done" }] });
    expect(result.value).not.toHaveProperty("sketchRevision");
  });

  it("never infers a legacy draft field was unchanged, including absent records", () => {
    const draft = { constraints: "Old constraint", requirements: [requirement("r1", "Old"), requirement("draft-only")], purpose: "Same" };
    const saved = { constraints: "New constraint", requirements: [requirement("r1", "Current"), requirement("saved-only")], purpose: "Same" };
    const result = mergeProjectDraft(undefined, draft, saved);
    expect(result.value).toEqual(saved);
    expect(result.unresolved).toBe(4);
    expect(result.conflicts.map(conflict => conflict.key)).toEqual([
      key("constraints"), key("requirements", "id:draft-only"), key("requirements", "id:r1", "title"), key("requirements", "id:saved-only"),
    ]);
    expect(result.conflicts.every(conflict => conflict.baseline === undefined)).toBe(true);
  });

  it("requires legacy order choices and keeps equal legacy values without conflicts", () => {
    const draft = { tasks: [requirement("a"), requirement("b")] };
    const saved = { tasks: [requirement("b"), requirement("a")] };
    expect(mergeProjectDraft(undefined, draft, saved).conflicts.map(conflict => conflict.key)).toEqual([key("tasks", "$order")]);
    expect(mergeProjectDraft(undefined, saved, saved)).toEqual({ value: saved, conflicts: [], unresolved: 0 });
  });

  it("preserves property deletion and merges independently edited nested objects", () => {
    const baseline: Record<string, unknown> = { obsolete: "remove", details: { title: "Old", description: "Old" } };
    const draft = { details: { title: "New", description: "Old" } };
    const saved = { ...baseline, details: { title: "Old", description: "Current" } };
    expect(mergeProjectDraft(baseline, draft, saved)).toEqual({ value: { details: { title: "New", description: "Current" } }, conflicts: [], unresolved: 0 });
  });

  it("treats JSON prototype-named fields as own data rather than inherited values", () => {
    const baseline = JSON.parse('{"details": {}}') as Record<string, unknown>;
    const draft = JSON.parse('{"details": {"__proto__": {"title":"Alice"}}}') as Record<string, unknown>;
    const saved = JSON.parse('{"details": {"constructor":"Bob"}}') as Record<string, unknown>;
    const result = mergeProjectDraft(baseline, draft, saved);
    expect(result.unresolved).toBe(0);
    expect(result.value).toEqual(JSON.parse('{"details":{"__proto__":{"title":"Alice"},"constructor":"Bob"}}'));
    expect(Object.getPrototypeOf(result.value.details)).toBe(Object.prototype);
    expect(Object.prototype).not.toHaveProperty("title");
  });

  it("does not alias whole-record conflict evidence with either inputs or its provisional value", () => {
    const baseline = { tasks: [requirement("r1", "Before")] };
    const draft = { tasks: [] };
    const saved = { tasks: [requirement("r1", "Saved")] };
    const result = mergeProjectDraft(baseline, draft, saved);
    result.value.tasks[0].title = "Changed provisional result";
    expect(result.conflicts[0].saved).toEqual(requirement("r1", "Saved"));
    (result.conflicts[0].saved as ReturnType<typeof requirement>).acceptance = "Changed evidence";
    expect(saved.tasks[0]).toEqual(requirement("r1", "Saved"));
  });

  it("is pure: neither inputs nor conflicts share mutable records with the merged result", () => {
    const baseline = { requirements: [requirement("r1", "Before")] };
    const draft = { requirements: [requirement("r1", "Draft")] };
    const saved = { requirements: [requirement("r1", "Saved")] };
    const original = JSON.stringify({ baseline, draft, saved });
    const result = mergeProjectDraft(baseline, draft, saved);
    result.value.requirements[0].acceptance = "Mutated";
    expect(JSON.stringify({ baseline, draft, saved })).toBe(original);
    expect(result.conflicts[0].saved).toBe("Saved");
  });
});
