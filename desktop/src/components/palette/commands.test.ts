import { beforeEach, describe, expect, it } from "vitest";
import { buildCommands, filterCommands } from "./commands";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";

// Shared-shape §3: the builders are task 15's and every override is an object passed to one
// of them. No literal stands in for a fixture here — a literal is a near-duplicate builder
// that goes stale the day contract §2 gains a field.
const PROJECT = aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" });
const BRANCH = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false });

function seed(sessionIds: string[]): void {
  resetCockpit();
  const sessions = Object.fromEntries(
    sessionIds.map((id, index) => [
      id,
      aSession({
        id,
        title: `orbit-${index + 1}`,
        projectId: "p-1",
        branchId: "b-1",
        worktreeId: "w-1",
        status: "running",
      }),
    ]),
  );
  useCockpit.setState({
    projects: [PROJECT],
    branchesByProject: { "p-1": [BRANCH] },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "wt-evidence" })],
    },
    sessions,
    sessionIdsByWorktree: { "w-1": sessionIds },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    lens: "develop",
    layout: "columns",
  });
}

describe("buildCommands", () => {
  beforeEach(() => seed(["s-1", "s-2", "s-3"]));

  it("returns the fixed block order: verbs, sessions, worktrees, branches, projects", () => {
    const kinds = buildCommands(useCockpit.getState()).map((c) => c.id.split(":")[0]);
    const firstOf = (kind: string): number => kinds.indexOf(kind);
    expect(firstOf("verb")).toBe(0);
    expect(firstOf("verb")).toBeLessThan(firstOf("session"));
    expect(firstOf("session")).toBeLessThan(firstOf("worktree"));
    expect(firstOf("worktree")).toBeLessThan(firstOf("branch"));
    expect(firstOf("branch")).toBeLessThan(firstOf("project"));
  });

  it("names the layout it switches TO, not the one it is on", () => {
    const columns = buildCommands(useCockpit.getState()).find((c) => c.id === "verb:layout");
    expect(columns?.label).toBe("Switch to Canvas");
    useCockpit.setState({ layout: "canvas" });
    const canvas = buildCommands(useCockpit.getState()).find((c) => c.id === "verb:layout");
    expect(canvas?.label).toBe("Switch to Columns");
  });

  it("gives a session its project and worktree as meta", () => {
    const row = buildCommands(useCockpit.getState()).find((c) => c.id === "session:s-2");
    expect(row?.label).toBe("orbit-2");
    expect(row?.meta).toBe("engagement-portal / wt-evidence");
  });

  it("hints ⌘1–⌘9 for the first nine pane sessions and nothing for the tenth", () => {
    seed(Array.from({ length: 10 }, (_, i) => `s-${i + 1}`));
    const rows = buildCommands(useCockpit.getState());
    expect(rows.find((c) => c.id === "session:s-1")?.hint).toBe("⌘1");
    expect(rows.find((c) => c.id === "session:s-9")?.hint).toBe("⌘9");
    // The row must EXIST before its missing hint means anything: `undefined?.hint` is also
    // undefined, so without this line a tenth session dropped from the list entirely would
    // pass the assertion below and the ⌘1–9 rule would be untested at its only boundary.
    expect(rows.find((c) => c.id === "session:s-10")).toBeDefined();
    expect(rows.find((c) => c.id === "session:s-10")?.hint).toBeUndefined();
  });

  it("running a session command selects its branch, then its worktree, then focuses it", () => {
    buildCommands(useCockpit.getState()).find((c) => c.id === "session:s-3")?.run();
    const after = useCockpit.getState().selection;
    expect(after.branchId).toBe("b-1");
    expect(after.worktreeId).toBe("w-1");
    expect(after.sessionId).toBe("s-3");
  });

  it("offers no layout verb when no branch is selected", () => {
    useCockpit.setState({ selection: { projectId: null, branchId: null, worktreeId: null, sessionId: null } });
    const ids = buildCommands(useCockpit.getState()).map((c) => c.id);
    expect(ids).not.toContain("verb:layout");
    expect(ids).not.toContain("verb:new-session");
    expect(ids).toContain("verb:machines");
  });
});

describe("filterCommands", () => {
  beforeEach(() => seed(["s-1", "s-2", "s-3"]));

  it("only removes rows; the survivors keep their original relative order", () => {
    const all = buildCommands(useCockpit.getState());
    const kept = filterCommands(all, "orbit").map((c) => c.id);
    const expected = all.map((c) => c.id).filter((id) => kept.includes(id));
    expect(kept).toEqual(expected);
    expect(kept).toEqual(["session:s-1", "session:s-2", "session:s-3"]);
  });

  it("matches the label case-insensitively and ignores meta", () => {
    const all = buildCommands(useCockpit.getState());
    expect(filterCommands(all, "ORBIT-2").map((c) => c.id)).toEqual(["session:s-2"]);
    // "engagement-portal" is the project NAME, so it matches the project row by label and no
    // session row by meta. §6.7 ties matching to `label`; widening it to `meta` would make one
    // project name match every session under it, which reads as a rank change to the user.
    expect(filterCommands(all, "engagement-portal").map((c) => c.id)).toEqual(["project:p-1"]);
  });

  it("returns every command for an empty or whitespace query", () => {
    const all = buildCommands(useCockpit.getState());
    expect(filterCommands(all, "")).toHaveLength(all.length);
    expect(filterCommands(all, "   ")).toHaveLength(all.length);
  });
});
