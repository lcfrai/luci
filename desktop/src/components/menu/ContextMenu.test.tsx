// The one right-click menu, for all four row kinds. This is the file TASK 22 edits directly,
// so a defect here is a defect task 22 inherits and builds on. It also has the most branching
// in the task — four item sets, the disabled-with-reason row, the act() close-then-run wrapper
// and the branch-with-no-worktree case — and a hook-ordering constraint that breaks silently:
// the four useCockpit reads sit ABOVE the `if (target === null) return null` early return,
// because a hook called after a conditional return throws "Rendered fewer hooks than expected"
// the first time the menu closes.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContextMenu } from "./ContextMenu";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";
import type { ContextMenuTarget, Worktree } from "../../store/types";
import { ApiError } from "../../api/client";
import { __resetSessionVerbs } from "../develop/sessionVerbs";

const { createSession, readyMock, revealMock, setupMock } = vi.hoisted(() => ({
  createSession: vi.fn(), readyMock: vi.fn(), revealMock: vi.fn(), setupMock: vi.fn(),
}));
vi.mock("../../api", () => ({ api: { createSession, setWorktreeReady: readyMock } }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => setupMock() }));
vi.mock("@tauri-apps/plugin-opener", () => ({ revealItemInDir: revealMock }));

const nativeLocalSetup = {
  workspace: { mode: "local", connectionId: "", host: "", ready: true },
  status: { platform: "windows", access: { projectsDirManaged: false } },
};

const project = aProject({ id: "p-1", name: "portal", path: "C:\\work\\portal" });
const branch = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload" });
const worktree = aWorktree({
  id: "w-1",
  projectId: "p-1",
  branchId: "b-1",
  name: "wt-evidence",
  path: "C:\\work\\wt\\evidence",
});

function seed(target: ContextMenuTarget, withWorktree = true): void {
  resetCockpit();
  useCockpit.setState({
    projects: [project],
    branchesByProject: { "p-1": [branch] },
    worktreesByBranch: withWorktree ? { "b-1": [worktree] } : {},
    // branchId matches the seeded branch: a session naming a branch the store does not hold is
    // a fixture the control plane cannot produce, and task 22's Fork reads through it.
    sessions: { "s-1": aSession({ id: "s-1", worktreeId: "w-1", branchId: "b-1" }) },
    sessionIdsByWorktree: { "w-1": ["s-1"] },
    contextMenu: target,
  });
}

const at = { x: 10, y: 10 };
const labels = (): string[] =>
  screen.getAllByRole("menuitem").map((el) => el.textContent ?? "");
const lastToast = () => {
  const toasts = useCockpit.getState().toasts;
  return toasts[toasts.length - 1];
};

describe("ContextMenu", () => {
  beforeEach(() => {
    createSession.mockReset();
    readyMock.mockReset();
    revealMock.mockReset();
    revealMock.mockResolvedValue(undefined);
    setupMock.mockReset();
    setupMock.mockReturnValue(null);
    Reflect.deleteProperty(window, "__TAURI_INTERNALS__");
    __resetSessionVerbs();
  });

  it("renders nothing at all when no row has been right-clicked", () => {
    resetCockpit();
    render(<ContextMenu />);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("gives each row kind its own verb set", () => {
    seed({ kind: "project", id: "p-1", ...at });
    const { unmount } = render(<ContextMenu />);
    expect(labels()).toEqual(["New branch", "Copy path", "Open project files"]);
    unmount();

    seed({ kind: "branch", id: "b-1", ...at });
    const second = render(<ContextMenu />);
    expect(labels()).toEqual([
      "Switch to this branch",
      "New session here",
      "Copy branch name",
      "Set branch defaults…",
    ]);
    second.unmount();

    // "Mark ready" is UNCONDITIONAL — §10.2 landed, so `ready` is always on the wire and there
    // is no world in which the row is absent (contract §6.2, §8 defect 9 as amended).
    seed({ kind: "worktree", id: "w-1", ...at });
    const third = render(<ContextMenu />);
    expect(labels()).toEqual([
      "New session here",
      "Open a shell here",
      "Copy path",
      "Mark ready",
    ]);
    third.unmount();

    // Task 22's lifecycle verbs, from ../develop/sessionVerbs. Resume is disabled because no
    // harness is seeded here, and Fork because "b-1" has a single worktree — both render with
    // a reason rather than vanishing, which is §6.4.
    seed({ kind: "session", id: "s-1", ...at });
    render(<ContextMenu />);
    expect(labels()).toEqual([
      "Resume",
      "Fork into…",
      "Restart",
      "Rename…",
      "Open a shell here",
      "Kill",
      "Forget",
      "Copy session id",
    ]);
  });

  // I5's third survivor, `rail-rename-never-opens-the-dots-menu`. The rail row has no text
  // field, so Rename… from HERE is a two-step: it closes this menu and opens the ⋯ menu, which
  // owns the only rename surface in the app. Nothing asserted either half, so a `beginRename`
  // that did nothing at all would have looked identical.
  it("the rail's Rename… closes this menu and opens the ⋯ menu for the same session", async () => {
    seed({ kind: "session", id: "s-1", ...at });
    render(<ContextMenu />);

    await userEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));

    expect(useCockpit.getState().contextMenu).toBeNull();
    expect(useCockpit.getState().sessionMenuFor).toBe("s-1");
  });

  // The §6.7 amendment this task argues for: <SessionMenu/> is mounted HERE, not in AppShell,
  // because shared-shape §1 gives AppShell's edit row to task 23. Unproven anywhere else, and
  // the ⋯ menu must be able to open while no right-click menu is up — the normal case.
  it("mounts the ⋯ menu from sessionMenuFor, with no right-click menu open", () => {
    seed({ kind: "session", id: "s-1", ...at });
    useCockpit.getState().closeContextMenu();
    useCockpit.getState().openSessionMenu("s-1");
    render(<ContextMenu />);

    expect(useCockpit.getState().contextMenu).toBeNull();
    expect(screen.getByRole("menuitem", { name: "Kill" })).not.toBeNull();
  });

  it("opens the real branch creation flow from the project menu", async () => {
    seed({ kind: "project", id: "p-1", ...at });
    render(<ContextMenu />);
    const item = screen.getByRole("menuitem", { name: "New branch" });
    expect(item.hasAttribute("disabled")).toBe(false);
    expect(
      screen.getByRole("menuitem", { name: "Copy path" }).hasAttribute("disabled"),
    ).toBe(false);
    await userEvent.click(item);
    expect(useCockpit.getState().branchManager).toEqual({ projectId: "p-1", action: "create" });
    expect(useCockpit.getState().contextMenu).toBeNull();
  });

  it("opens the chosen browser project's Files with one guarded navigation", async () => {
    seed({ kind: "project", id: "p-1", ...at });
    useCockpit.setState({ projects: [project, aProject({ id: "p-other" })],
      selection: { projectId: "p-other", branchId: null, worktreeId: null, sessionId: null }, lens: "design" });
    setupMock.mockReturnValue(nativeLocalSetup);
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Open project files" }));
    expect(useCockpit.getState().selection.projectId).toBe("p-1");
    expect(useCockpit.getState().lens).toBe("files");
    expect(useCockpit.getState().contextMenu).toBeNull();
    expect(revealMock).not.toHaveBeenCalled();
  });

  it.each(["cancel", "continue"])("keeps the exact project/Files destination through a draft guard: %s", async choice => {
    seed({ kind: "project", id: "p-1", ...at });
    useCockpit.setState({ projects: [project, aProject({ id: "p-other" })],
      selection: { projectId: "p-other", branchId: null, worktreeId: null, sessionId: null }, lens: "design" });
    useCockpit.getState().setUnsavedProjectDraft("editor", { projectId: "p-other", label: "Design", lens: "design" });
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Open project files" }));
    expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "project", projectId: "p-1", lens: "files" });
    expect(useCockpit.getState().selection.projectId).toBe("p-other");
    expect(useCockpit.getState().lens).toBe("design");
    act(() => choice === "cancel" ? useCockpit.getState().cancelNavigation() : useCockpit.getState().discardDraftsAndNavigate());
    expect(useCockpit.getState().pendingNavigation).toBeNull();
    expect(useCockpit.getState().selection.projectId).toBe(choice === "cancel" ? "p-other" : "p-1");
    expect(useCockpit.getState().lens).toBe(choice === "cancel" ? "design" : "files");
    expect(!!useCockpit.getState().unsavedProjectDrafts.editor).toBe(choice === "cancel");
  });

  it("keeps the draft when the destination project disappears before confirmation", async () => {
    seed({ kind: "project", id: "p-1", ...at });
    const other = aProject({ id: "p-other" });
    useCockpit.setState({ projects: [project, other],
      selection: { projectId: other.id, branchId: null, worktreeId: null, sessionId: null }, lens: "design" });
    useCockpit.getState().setUnsavedProjectDraft("editor", { projectId: other.id, label: "Design", lens: "design" });
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Open project files" }));
    act(() => {
      useCockpit.getState().setProjects([other]);
      useCockpit.getState().discardDraftsAndNavigate();
    });
    expect(useCockpit.getState().selection.projectId).toBe(other.id);
    expect(useCockpit.getState().lens).toBe("design");
    expect(useCockpit.getState().unsavedProjectDrafts.editor).toBeDefined();
    expect(lastToast()?.message).toContain("Your edited draft is still here");
  });

  it("does not navigate from a stale menu to a missing project", () => {
    seed({ kind: "project", id: "p-1", ...at });
    render(<ContextMenu />);
    const item = screen.getByRole("menuitem", { name: "Open project files" });
    act(() => {
      useCockpit.getState().setProjects([]);
      item.click();
    });
    expect(useCockpit.getState().selection.projectId).toBeNull();
    expect(useCockpit.getState().lens).toBe("develop");
    expect(useCockpit.getState().pendingNavigation).toBeNull();
    expect(revealMock).not.toHaveBeenCalled();
    expect(lastToast()?.message).toContain("no longer available");
  });

  it.each(["remote", "container", "missing bridge"])("does not reveal non-local native controller paths: %s", async kind => {
    seed({ kind: "project", id: "p-1", ...at });
    Object.defineProperty(window, "__TAURI_INTERNALS__", { value: kind === "missing bridge" ? {} : { invoke: vi.fn() }, configurable: true });
    setupMock.mockReturnValue(kind === "remote" ? { ...nativeLocalSetup,
      workspace: { mode: "cloud", connectionId: "ssh-owned", host: "owned.example", ready: true } } : kind === "container" ? {
        ...nativeLocalSetup, status: { platform: "linux", access: { projectsDirManaged: true } },
      } : nativeLocalSetup);
    render(<ContextMenu />);
    expect(screen.queryByRole("menuitem", { name: "Reveal in folder" })).toBeNull();
    await userEvent.click(screen.getByRole("menuitem", { name: "Open project files" }));
    expect(useCockpit.getState().selection.projectId).toBe("p-1");
    expect(useCockpit.getState().lens).toBe("files");
    expect(revealMock).not.toHaveBeenCalled();
  });

  it("reveals only the actual local native project folder", async () => {
    seed({ kind: "project", id: "p-1", ...at });
    Object.defineProperty(window, "__TAURI_INTERNALS__", { value: { invoke: vi.fn() }, configurable: true });
    setupMock.mockReturnValue(nativeLocalSetup);
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Reveal in folder" }));
    await waitFor(() => expect(revealMock).toHaveBeenCalledWith(project.path));
    expect(revealMock).toHaveBeenCalledOnce();
    expect(useCockpit.getState().contextMenu).toBeNull();
    expect(useCockpit.getState().lens).toBe("develop");
  });

  it("reports a native folder failure without leaking the opener error", async () => {
    seed({ kind: "project", id: "p-1", ...at });
    Object.defineProperty(window, "__TAURI_INTERNALS__", { value: { invoke: vi.fn() }, configurable: true });
    setupMock.mockReturnValue(nativeLocalSetup);
    revealMock.mockRejectedValue(new Error("private native diagnostic"));
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Reveal in folder" }));
    await waitFor(() => expect(lastToast()?.message).toContain("couldn't reveal this folder"));
    expect(JSON.stringify(useCockpit.getState().toasts)).not.toContain("private native diagnostic");
    expect(useCockpit.getState().selection.projectId).toBeNull();
  });

  it("the real readiness menu recovers after an admission refusal without hiding it", async () => {
    seed({ kind: "worktree", id: "w-1", ...at });
    const ready = { ...worktree, ready: true };
    useCockpit.getState().setWorktrees("b-1", [ready]);
    let refuse!: (cause: unknown) => void;
    readyMock.mockReturnValue(new Promise<Worktree>((_resolve, reject) => { refuse = reject; }));
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Mark not ready" }));
    expect(readyMock).toHaveBeenCalledWith("p-1", "w-1", false);
    act(() => useCockpit.getState().openContextMenu({ kind: "worktree", id: "w-1", ...at }));
    expect(screen.getByRole("menuitem", { name: "Mark not ready" }).hasAttribute("disabled")).toBe(true);
    await act(async () => { refuse(new ApiError("conflict", "Project source is changing.", 409)); });
    expect(screen.getByRole("menuitem", { name: "Mark not ready" }).hasAttribute("disabled")).toBe(false);
    expect(useCockpit.getState().worktreesByBranch["b-1"]).toEqual([ready]);
    expect(lastToast()?.message).toBe("Project source is changing.");
  });

  it("does not reveal a changed project's stale native folder path", async () => {
    seed({ kind: "project", id: "p-1", ...at });
    Object.defineProperty(window, "__TAURI_INTERNALS__", { value: { invoke: vi.fn() }, configurable: true });
    setupMock.mockReturnValue(nativeLocalSetup);
    render(<ContextMenu />);
    const item = screen.getByRole("menuitem", { name: "Reveal in folder" });
    await act(async () => {
      useCockpit.getState().setProjects([{ ...project, path: "C:\\other\\portal" }]);
      item.click();
    });
    expect(revealMock).not.toHaveBeenCalled();
    expect(lastToast()?.message).toContain("project changed");
  });

  it("disables New session here for a branch with no worktrees, and says so", () => {
    seed({ kind: "branch", id: "b-1", ...at }, false);
    render(<ContextMenu />);
    const item = screen.getByRole("menuitem", { name: "New session here" });
    expect(item.hasAttribute("disabled")).toBe(true);
    expect(item.getAttribute("title")).toBe("This branch has no worktrees yet.");
  });

  // The act() wrapper closes the menu BEFORE running the verb. A menu that stayed open behind
  // the thing it launched would be a second surface the user has to dismiss.
  it("closes the menu when a verb runs", async () => {
    seed({ kind: "worktree", id: "w-1", ...at });
    render(<ContextMenu />);
    expect(useCockpit.getState().contextMenu).not.toBeNull();
    await userEvent.click(screen.getByRole("menuitem", { name: "Copy path" }));
    expect(useCockpit.getState().contextMenu).toBeNull();
  });

  // §6.2's "Set branch defaults…" has no surface of its own; it selects the branch and opens
  // the picker, which is the §6.4 amendment this task argued for.
  it("routes Set branch defaults to the picker, with the branch selected", async () => {
    seed({ kind: "branch", id: "b-1", ...at });
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Set branch defaults…" }));
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().pickerOpen).toBe(true);
  });

  // The hook-ordering constraint, exercised rather than asserted: closing the menu re-renders
  // the component with target === null. If the four useCockpit reads were below the early
  // return, React would throw "Rendered fewer hooks than expected" on exactly this transition.
  it("survives being closed, which is where a misplaced hook would throw", () => {
    seed({ kind: "project", id: "p-1", ...at });
    const { rerender } = render(<ContextMenu />);
    expect(screen.queryByRole("menu")).not.toBeNull();
    useCockpit.getState().closeContextMenu();
    rerender(<ContextMenu />);
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
