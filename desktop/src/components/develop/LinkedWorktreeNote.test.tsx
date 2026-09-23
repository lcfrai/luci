// The sandbox git fix, part 3: a linked worktree says plainly that git is unavailable in it.
// A guard with two outcomes has a negative case, so every surface is pinned both ways: the
// notice renders for `isPrimary: false` and does NOT render for `isPrimary: true`.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { LINKED_WORKTREE_GIT_NOTE, LinkedWorktreeNote } from "./LinkedWorktreeNote";
import { EmptyDevelop } from "./EmptyDevelop";
import { SessionPane } from "./SessionPane";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aHarness, aSession, aWorktree } from "../../test/fixtures";

vi.mock("../Terminal", () => ({
  Terminal: ({ sessionId }: { sessionId: string }) => (
    <div data-testid="terminal" data-session={sessionId} />
  ),
}));

const { createSessionMock } = vi.hoisted(() => ({ createSessionMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession: createSessionMock } }));

const CLAUDE = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: [""],
  permissionModes: ["ask"],
});

const BRANCH = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload" });

/** One worktree on one branch, primary or linked, with one running session on it. */
function seed(isPrimary: boolean): void {
  const worktree = aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "evidence-upload", isPrimary });
  const session = aSession({ id: "s-1", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "running" });
  useCockpit.setState({
    harnesses: [CLAUDE],
    worktreesByBranch: { "b-1": [worktree] },
    sessions: { [session.id]: session },
    sessionIdsByWorktree: { "w-1": [session.id] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null },
  });
}

function notesIn(container: HTMLElement): Element[] {
  return Array.from(container.querySelectorAll("[data-linked-worktree-note]"));
}

describe("LinkedWorktreeNote", () => {
  beforeEach(() => {
    resetCockpit();
  });

  it("is one sentence naming what is unavailable, the fact behind it, and Phase 3", () => {
    // One sentence: a single terminal full stop, at the end.
    expect(LINKED_WORKTREE_GIT_NOTE.match(/\.(\s|$)/g)?.length).toBe(1);
    expect(LINKED_WORKTREE_GIT_NOTE.endsWith(".")).toBe(true);
    for (const part of [/^Git is unavailable/, /linked worktree/, /\.git/, /sandbox cannot see/, /Phase 3/]) {
      expect(part.test(LINKED_WORKTREE_GIT_NOTE)).toBe(true);
    }
  });

  it("renders the sentence for a linked worktree", () => {
    const { container } = render(<LinkedWorktreeNote worktree={aWorktree({ isPrimary: false })} />);
    const notes = notesIn(container);
    expect(notes).toHaveLength(1);
    expect(notes[0].textContent).toBe(LINKED_WORKTREE_GIT_NOTE);
    expect(notes[0].getAttribute("role")).toBe("note");
  });

  it("renders nothing for the primary worktree, whose git works", () => {
    const { container } = render(<LinkedWorktreeNote worktree={aWorktree({ isPrimary: true })} />);
    expect(container.childNodes.length).toBe(0);
  });

  it("renders nothing for a worktree the store has not loaded, which proves nothing", () => {
    const { container } = render(<LinkedWorktreeNote worktree={undefined} />);
    expect(container.childNodes.length).toBe(0);
  });
});

describe("where a session is READ: SessionPane", () => {
  beforeEach(() => {
    resetCockpit();
  });

  it("shows the notice in a pane on a linked worktree", () => {
    seed(false);
    const { container } = render(<SessionPane sessionId="s-1" variant="column" />);
    const notes = notesIn(container);
    expect(notes).toHaveLength(1);
    expect(notes[0].textContent).toBe(LINKED_WORKTREE_GIT_NOTE);
    // Inside the pane, after the terminal's body: `.pane`'s grid rows are positional.
    const pane = container.querySelector("section.pane");
    expect(notes[0].parentElement).toBe(pane);
    expect(notes[0].previousElementSibling?.className).toBe("pane-body");
  });

  it("shows nothing in a pane on the primary worktree", () => {
    seed(true);
    const { container } = render(<SessionPane sessionId="s-1" variant="column" />);
    expect(container.querySelector("section.pane")).not.toBeNull();
    expect(notesIn(container)).toHaveLength(0);
  });
});

describe("where a session is STARTED: EmptyDevelop", () => {
  beforeEach(() => {
    resetCockpit();
  });

  it("shows the notice before the start buttons on a linked worktree", () => {
    seed(false);
    useCockpit.setState({ sessionIdsByWorktree: { "w-1": [] }, sessions: {} });
    const { container } = render(<EmptyDevelop branch={BRANCH} />);
    const notes = notesIn(container);
    expect(notes).toHaveLength(1);
    expect(notes[0].nextElementSibling?.className).toBe("pane-empty-actions");
  });

  it("shows nothing on the primary worktree", () => {
    seed(true);
    useCockpit.setState({ sessionIdsByWorktree: { "w-1": [] }, sessions: {} });
    const { container } = render(<EmptyDevelop branch={BRANCH} />);
    expect(container.querySelector(".pane-empty-actions")).not.toBeNull();
    expect(notesIn(container)).toHaveLength(0);
  });
});
