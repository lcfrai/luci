import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { api } from "../../api";
import { useCockpit, resetCockpit } from "../../store/store";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";
import { NewSessionButton } from "../topbar/NewSessionButton";
import { SessionMenu } from "./SessionMenu";
import { SessionPane } from "./SessionPane";
import { EmptyDevelop } from "./EmptyDevelop";
import { ContextMenu } from "../menu/ContextMenu";
import { useCreateSession } from "../../hooks/useCreateSession";
import { useKeyboardMap } from "../../hooks/useKeyboardMap";
import { __resetSessionVerbs } from "./sessionVerbs";
import { __resetOverlays } from "../../hooks/useDismissable";

const access = vi.hoisted(() => ({ canRead: true, canEdit: false, canExecute: false, canAssist: false, permissions: { admin: false }, message: "Read-only project access." }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ ...access }) }));
vi.mock("../Terminal", () => ({ Terminal: ({ readOnly }: { readOnly: boolean }) => <div data-testid="terminal">{readOnly ? "Retained transcript" : "Interactive terminal"}</div> }));

beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); __resetSessionVerbs(); __resetOverlays();
  access.canEdit = false; access.canExecute = false; access.canAssist = false; access.permissions.admin = false; access.message = "Read-only project access.";
  useCockpit.setState({
    projects: [aProject({ id: "p-1" })], branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", branchId: "b-1", projectId: "p-1" })] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: "s-1" },
    harnesses: [aHarness({ id: "claude-code", name: "Claude Code", supportsResume: true }), aHarness({ id: "shell", name: "Terminal" })],
  });
  useCockpit.getState().upsertSession(aSession({ id: "s-1", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", title: "Existing research", status: "exited" }));
  vi.spyOn(api, "createSession").mockResolvedValue(aSession());
  vi.spyOn(api, "restartSession").mockResolvedValue(aSession());
  vi.spyOn(api, "killSession").mockResolvedValue(aSession());
  vi.spyOn(api, "renameSession").mockResolvedValue(aSession({ id: "s-1", title: "Updated title" }));
});
afterEach(() => { __resetSessionVerbs(); __resetOverlays(); });

it("keeps existing transcript and session metadata visible while disabling restart and start controls", async () => {
  render(<><NewSessionButton worktreeId="w-1" /><SessionPane sessionId="s-1" variant="column" /><EmptyDevelop branch={aBranch({ id: "b-1", projectId: "p-1" })} /></>);
  for (const name of [/New session/i, "Session options", "Restart", "＋ Claude Code", "Open a shell"]) {
    expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(true);
    await userEvent.click(screen.getByRole("button", { name }));
  }
  expect(screen.getByText("Retained transcript")).not.toBeNull();
  expect(api.createSession).not.toHaveBeenCalled(); expect(api.restartSession).not.toHaveBeenCalled();
});

it("lets an editor rename metadata without execution rights and blocks subsequent revocation", async () => {
  access.canEdit = true; access.message = "Execution access is required.";
  const view = render(<SessionMenu sessionId="s-1" anchor={new DOMRect(0, 0, 20, 20)} onClose={() => {}} />);
  expect((screen.getByRole("menuitem", { name: "Restart" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("menuitem", { name: "Copy session id" }) as HTMLButtonElement).disabled).toBe(false);
  await userEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
  await userEvent.clear(screen.getByLabelText("Session title"));
  await userEvent.type(screen.getByLabelText("Session title"), "Updated title{enter}");
  await waitFor(() => expect(api.renameSession).toHaveBeenCalledExactlyOnceWith("s-1", "Updated title"));
  await userEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
  access.canEdit = false;
  view.rerender(<SessionMenu sessionId="s-1" anchor={new DOMRect(0, 0, 20, 20)} onClose={() => {}} />);
  expect((screen.getByLabelText("Session title") as HTMLInputElement).disabled).toBe(true);
  fireEvent.submit(screen.getByLabelText("Session title").closest("form")!);
  expect(api.renameSession).toHaveBeenCalledTimes(1);
});

it("right-click actions apply target project access while retaining copy and file reads", () => {
  useCockpit.getState().openContextMenu({ kind: "worktree", id: "w-1", x: 10, y: 10 });
  render(<ContextMenu />);
  for (const name of ["New session here", "Open a shell here", "Mark ready"]) expect((screen.getByRole("menuitem", { name }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("menuitem", { name: "Copy path" }) as HTMLButtonElement).disabled).toBe(false);
});

it("keyboard creation and kill shortcuts respect denial without changing historical sessions", () => {
  renderHook(() => useKeyboardMap());
  const before = useCockpit.getState().sessions["s-1"];
  fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
  fireEvent.keyDown(window, { key: "w", ctrlKey: true });
  expect(api.createSession).not.toHaveBeenCalled(); expect(api.killSession).not.toHaveBeenCalled();
  expect(useCockpit.getState().sessions["s-1"]).toEqual(before);
});

it("the shared session creator rechecks live permissions even when called through an old callback", async () => {
  access.canEdit = true; access.canExecute = true; access.message = "";
  const hook = renderHook(() => useCreateSession("p-1"));
  const oldCreate = hook.result.current;
  access.canExecute = false; access.message = "Execution access was removed.";
  hook.rerender();
  let result: unknown;
  await act(async () => { result = await oldCreate("w-1"); });
  expect(result).toEqual({ ok: false, message: "Execution access was removed." });
  expect(api.createSession).not.toHaveBeenCalled();
});

it("execution without credential access allows shells but never starts a coding agent", async () => {
  access.canEdit = true; access.canExecute = true; access.message = "";
  const hook = renderHook(() => useCreateSession("p-1"));
  await act(async () => { await hook.result.current("w-1"); });
  expect(api.createSession).not.toHaveBeenCalled();
  await act(async () => { await hook.result.current("w-1", { harness: "shell" }); });
  expect(api.createSession).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ harness: "shell" }));
});
