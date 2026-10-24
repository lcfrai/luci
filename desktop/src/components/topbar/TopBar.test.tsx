import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { resetCockpit, useCockpit } from "../../store/store";
import { TopBar } from "./TopBar";

vi.mock("./SessionPicker", () => ({ SessionPicker: () => null }));
vi.mock("../orchestrator/OrchestratorPanel", () => ({ ORCHESTRATOR_OPEN: "luci:orchestrator-open" }));

beforeEach(() => { localStorage.clear(); resetCockpit(); });
afterEach(() => cleanup());

it.each([
  ["discover", "Discover"], ["design", "Design"], ["deploy", "Deploy"], ["distribute", "Distribute"],
] as const)("labels the active %s workspace rather than falling back to another stage", (lens, label) => {
  useCockpit.setState({
    workspaceSurface: "tools", lens,
    selection: { projectId: "p-1", branchId: null, worktreeId: null, sessionId: null },
  });
  render(<TopBar />);
  expect(screen.getByText(label, { selector: ".top-title" })).toBeTruthy();
  if (lens === "distribute") expect(screen.queryByText("Deploy", { selector: ".top-title" })).toBeNull();
  expect(screen.queryByRole("navigation", { name: "Develop tools" })).toBeNull();
});
