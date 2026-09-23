import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CommandPalette } from "./CommandPalette";
import { __resetOverlays } from "../../hooks/useDismissable";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";

const TITLES = ["orbit-one", "orbit-two", "orbit-three"];

beforeEach(() => {
  resetCockpit();
  __resetOverlays();
  useCockpit.setState({
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false })] },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "wt-evidence" })],
    },
    sessions: Object.fromEntries(
      TITLES.map((title, index) => [
        `s-${index + 1}`,
        aSession({
          id: `s-${index + 1}`,
          title,
          projectId: "p-1",
          branchId: "b-1",
          worktreeId: "w-1",
          status: "running",
        }),
      ]),
    ),
    sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    lens: "develop",
    layout: "columns",
  });
});

function open(): void {
  act(() => {
    useCockpit.getState().openPalette();
  });
}

function selectedRowName(): string {
  const on = screen.getAllByRole("option").find((row) => row.getAttribute("aria-selected") === "true");
  return on?.textContent ?? "<none>";
}

describe("CommandPalette", () => {
  it("renders nothing while the palette is closed", () => {
    render(<CommandPalette />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("Enter on the second visible row runs the second VISIBLE command, not commands[1]", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    // Unfiltered, commands[0] is "Switch to Canvas" and commands[1] is "Hide the rail" — the
    // row an index-held cursor would fire. The query removes both.
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    expect(useCockpit.getState().railCollapsed).toBe(false);
    expect(useCockpit.getState().layout).toBe("columns");
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("keeps the selected row selected while a narrower query still matches it", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-two");
    await user.type(screen.getByRole("combobox"), "-t");
    expect(screen.getAllByRole("option")).toHaveLength(2);
    expect(selectedRowName()).toContain("orbit-two");
  });

  it("falls back to the first visible row when the selected id is filtered out", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-two");
    await user.type(screen.getByRole("combobox"), "-three");
    expect(screen.getAllByRole("option")).toHaveLength(1);
    expect(selectedRowName()).toContain("orbit-three");
  });

  it("wraps the cursor at both ends of the visible list", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowUp}");
    expect(selectedRowName()).toContain("orbit-three");
    await user.keyboard("{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-one");
  });

  it("a cursor that wrapped to the top holds its row when the query is cleared", async () => {
    // Wrapping must land on the first row's ID. Landing on "no selection" looks identical while
    // the query stands, because the fallback shows the first row anyway — but clearing the
    // query puts the verbs back above it, and a null cursor would jump to "Switch to Canvas".
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowUp}{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-one");
    await user.clear(screen.getByRole("combobox"));
    expect(screen.getAllByRole("option").length).toBeGreaterThan(3);
    expect(selectedRowName()).toContain("orbit-one");
  });

  it("runs the row a click lands on", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit-three");
    await user.click(screen.getByRole("option", { name: /orbit-three/ }));
    expect(useCockpit.getState().selection.sessionId).toBe("s-3");
  });

  it("keeps the order it opened with when a poll reorders sessions mid-query", async () => {
    // The list is built once, at open. A poll that lands while the user is typing must not
    // re-rank the rows under the cursor on the next keystroke (§6.7, stable ranking).
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    act(() => {
      useCockpit.setState({ sessionIdsByWorktree: { "w-1": ["s-3", "s-1", "s-2"] } });
    });
    await user.type(screen.getByRole("combobox"), "orbit");
    expect(screen.getAllByRole("option").map((row) => row.textContent)).toEqual([
      expect.stringContaining("orbit-one"),
      expect.stringContaining("orbit-two"),
      expect.stringContaining("orbit-three"),
    ]);
  });

  it("names the query instead of rendering an empty box", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "zzzzz");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByText(/Nothing matches/)).toBeDefined();
  });

  it("returns DOM focus to the element that had it when the palette closes", () => {
    render(
      <>
        <button type="button">prior</button>
        <CommandPalette />
      </>,
    );
    const prior = screen.getByRole("button", { name: "prior" });
    prior.focus();
    open();
    expect(document.activeElement).toBe(screen.getByRole("combobox"));
    act(() => {
      useCockpit.getState().closePalette();
    });
    expect(document.activeElement).toBe(prior);
  });
});
