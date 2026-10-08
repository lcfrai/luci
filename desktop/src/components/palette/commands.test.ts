import { beforeEach, describe, expect, it } from "vitest";
import { SEND_CHORDS_LABEL, buildCommands, filterCommands } from "./commands";
import type { Command } from "./commands";
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

  // OD-9. One row, always present, whose label never changes and whose state is `checked`.
  it("OD-9: lists 'Send shortcut keys to the terminal' as the last verb, in every state, with its state as checked", () => {
    const row = (): Command | undefined =>
      buildCommands(useCockpit.getState()).find((c) => c.label === SEND_CHORDS_LABEL);
    const lastVerb = (): string | undefined => {
      const verbs = buildCommands(useCockpit.getState()).filter((c) => c.id.startsWith("verb:"));
      return verbs[verbs.length - 1]?.id;
    };
    expect(SEND_CHORDS_LABEL).toBe("Send shortcut keys to the terminal");
    expect([row()?.id, row()?.checked, row()?.meta, row()?.hint, lastVerb()])
      .toEqual(["verb:send-chords-to-terminal", false, undefined, undefined, "verb:send-chords-to-terminal"]);
    useCockpit.getState().setSendChordsToTerminal(true);
    expect([row()?.label, row()?.checked]).toEqual([SEND_CHORDS_LABEL, true]);
    // Still there with nothing selected, where the layout and new-session verbs drop out.
    useCockpit.setState({ selection: { projectId: null, branchId: null, worktreeId: null, sessionId: null } });
    expect(row()?.checked).toBe(true);
    // The only row that sets `checked`.
    expect(buildCommands(useCockpit.getState()).filter((c) => c.checked !== undefined).map((c) => c.id))
      .toEqual(["verb:send-chords-to-terminal"]);
  });

  it("OD-9: running the row sets the setting, says so in a toast, and the toast's Undo restores it", () => {
    const run = () => buildCommands(useCockpit.getState(), "mac").find((c) => c.label === SEND_CHORDS_LABEL)?.run();
    const lastToast = () => {
      const { toasts } = useCockpit.getState();
      return toasts[toasts.length - 1];
    };
    run();
    expect(useCockpit.getState().sendChordsToTerminal).toBe(true);
    // EDITED BY OD-9's FIX ROUND 1. It used to say "click outside the terminal", and a click on
    // a pane's own header or footer, or on another session's rail row, puts focus back into a
    // terminal, where ⌘K is the agent's. The top bar's empty part moves no focus into a pane:
    // AppShell.test.tsx walks this exact route, and the pane-header route, which fails.
    expect(lastToast()?.message).toBe(
      "Shortcut keys now go to the terminal, ⌘K too. To open this palette again, click an empty part of the top bar, then press ⌘K.",
    );
    lastToast()?.undo?.();
    expect(useCockpit.getState().sendChordsToTerminal).toBe(false);

    useCockpit.getState().setSendChordsToTerminal(true);
    run();
    expect(useCockpit.getState().sendChordsToTerminal).toBe(false);
    expect(lastToast()?.message).toBe("Shortcut keys go to the cockpit again, from a terminal too.");
    lastToast()?.undo?.();
    expect(useCockpit.getState().sendChordsToTerminal).toBe(true);
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
    const rows = buildCommands(useCockpit.getState(), "mac");
    expect(rows.find((c) => c.id === "session:s-1")?.hint).toBe("⌘1");
    expect(rows.find((c) => c.id === "session:s-9")?.hint).toBe("⌘9");
    // The row must EXIST before its missing hint means anything: `undefined?.hint` is also
    // undefined, so without this line a tenth session dropped from the list entirely would
    // pass the assertion below and the ⌘1–9 rule would be untested at its only boundary.
    expect(rows.find((c) => c.id === "session:s-10")).toBeDefined();
    expect(rows.find((c) => c.id === "session:s-10")?.hint).toBeUndefined();
  });

  it.each([["mac", "⌘B", "⌘⇧M", "⌘1", "⌘K"], ["other", "Ctrl+B", "Ctrl+Shift+M", "Ctrl+1", "Ctrl+K"]] as const)("uses client %s shortcuts in the palette and terminal handoff notice", (platform, rail, machines, session, palette) => {
    const rows = buildCommands(useCockpit.getState(), platform);
    expect(rows.find(row => row.id === "verb:rail")?.hint).toBe(rail);
    expect(rows.find(row => row.id === "verb:machines")?.hint).toBe(machines);
    expect(rows.find(row => row.id === "session:s-1")?.hint).toBe(session);
    rows.find(row => row.id === "verb:send-chords-to-terminal")?.run();
    const toasts = useCockpit.getState().toasts;
    expect(toasts[toasts.length - 1]?.message).toBe(`Shortcut keys now go to the terminal, ${palette} too. To open this palette again, click an empty part of the top bar, then press ${palette}.`);
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

  it("offers no layout verb outside the Develop lens", () => {
    // Columns and Canvas are Develop's layouts; from Merge the verb would change nothing on screen.
    useCockpit.setState({ lens: "merge" });
    const ids = buildCommands(useCockpit.getState()).map((c) => c.id);
    expect(ids).not.toContain("verb:layout");
    expect(ids).toContain("verb:new-session");
  });

  it("orders every block by id, not by the order the poller happened to insert keys", () => {
    // Object.keys is insertion order, and a poll that replaces an entry moves its key to the
    // end. Each Record below is written in REVERSE id order to stand in for that.
    const P2 = aProject({ id: "p-2", name: "records-api", path: "C:\\work\\records-api" });
    useCockpit.setState({
      projects: [PROJECT, P2],
      branchesByProject: {
        "p-2": [aBranch({ id: "b-9", projectId: "p-2", name: "ingest" })],
        "p-1": [BRANCH, aBranch({ id: "b-2", projectId: "p-1", name: "audit-trail" })],
      },
      worktreesByBranch: {
        "b-2": [
          aWorktree({ id: "w-3", projectId: "p-1", branchId: "b-2", name: "wt-three" }),
          aWorktree({ id: "w-2", projectId: "p-1", branchId: "b-2", name: "wt-two" }),
        ],
        "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "wt-evidence" })],
      },
      sessions: {
        ...useCockpit.getState().sessions,
        "s-8": aSession({ id: "s-8", projectId: "p-1", branchId: "b-2", worktreeId: "w-2" }),
        "s-9": aSession({ id: "s-9", projectId: "p-1", branchId: "b-2", worktreeId: "w-3" }),
      },
      sessionIdsByWorktree: { "w-3": ["s-9"], "w-2": ["s-8"], "w-1": ["s-1", "s-2", "s-3"] },
    });
    const ids = buildCommands(useCockpit.getState()).map((c) => c.id);
    const of = (kind: string): string[] => ids.filter((id) => id.startsWith(`${kind}:`));
    // The selected branch's panes first (selectPaneSessionIds), then the rest by worktree id.
    expect(of("session")).toEqual(["session:s-1", "session:s-2", "session:s-3", "session:s-8", "session:s-9"]);
    // Within a branch the server's order stands (w-3 before w-2); across branches, branch id.
    expect(of("worktree")).toEqual(["worktree:w-1", "worktree:w-3", "worktree:w-2"]);
    expect(of("branch")).toEqual(["branch:b-1", "branch:b-2", "branch:b-9"]);
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
