import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { CheckCommandEditor, type CheckCommandEditorProps } from "./CheckCommandEditor";

function props(overrides: Partial<CheckCommandEditorProps> = {}): CheckCommandEditorProps {
  return {
    commands: [
      { label: "Acceptance tests", executable: "npm", args: ["test", "path with spaces"] },
      { label: "Build", executable: "npm", args: ["run", "build"] },
    ],
    executable: "npm", args: '["test"]', controlsDisabled: false, fieldsDisabled: false,
    openAdvanced: true,
    onRemove: vi.fn(), onExecutableChange: vi.fn(), onArgsChange: vi.fn(), onAdd: vi.fn(),
    ...overrides,
  };
}

it("shows exact command arguments and removes only the selected index without submitting", async () => {
  const callbacks = props();
  render(<CheckCommandEditor {...callbacks} />);
  expect(screen.getByText('npm "test" "path with spaces"')).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: "Remove Build" }));
  expect(callbacks.onRemove).toHaveBeenCalledExactlyOnceWith(1);
  expect(callbacks.onAdd).not.toHaveBeenCalled();
});

it("passes raw edits and submission to the parent without parsing or modifying commands", () => {
  const callbacks = props({ args: "not JSON" });
  render(<CheckCommandEditor {...callbacks} />);
  fireEvent.change(screen.getByRole("textbox", { name: "Executable" }), { target: { value: " go " } });
  fireEvent.change(screen.getByRole("textbox", { name: "Arguments · JSON array" }), { target: { value: '["test", 3]' } });
  expect(callbacks.onExecutableChange).toHaveBeenCalledExactlyOnceWith(" go ");
  expect(callbacks.onArgsChange).toHaveBeenCalledExactlyOnceWith('["test", 3]');
  const form = screen.getByRole("button", { name: "Add check command" }).closest("form")!;
  expect(fireEvent.submit(form)).toBe(false);
  expect(callbacks.onAdd).toHaveBeenCalledOnce();
  expect(callbacks.onRemove).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Remove Acceptance tests" })).toBeTruthy();
});

it("keeps draft fields editable when command actions alone are disabled", async () => {
  const callbacks = props({ controlsDisabled: true });
  render(<CheckCommandEditor {...callbacks} />);
  const field = screen.getByRole("textbox", { name: "Executable" });
  expect(field).toHaveProperty("disabled", false);
  expect(screen.getByRole("textbox", { name: "Arguments · JSON array" })).toHaveProperty("disabled", false);
  expect(screen.getByRole("button", { name: "Add check command" })).toHaveProperty("disabled", true);
  expect(screen.getByRole("button", { name: "Remove Build" })).toHaveProperty("disabled", true);
  await userEvent.click(screen.getByRole("button", { name: "Add check command" }));
  await userEvent.click(screen.getByRole("button", { name: "Remove Build" }));
  fireEvent.change(field, { target: { value: "go" } });
  expect(callbacks.onExecutableChange).toHaveBeenCalledExactlyOnceWith("go");
  expect(callbacks.onAdd).not.toHaveBeenCalled();
  expect(callbacks.onRemove).not.toHaveBeenCalled();
});

it("disables both fields for the parent's authority or pending-draft barrier", async () => {
  const callbacks = props({ controlsDisabled: true, fieldsDisabled: true });
  render(<CheckCommandEditor {...callbacks} />);
  const executable = screen.getByRole("textbox", { name: "Executable" });
  const args = screen.getByRole("textbox", { name: "Arguments · JSON array" });
  expect(executable).toHaveProperty("disabled", true);
  expect(args).toHaveProperty("disabled", true);
  await userEvent.type(executable, "extra");
  await userEvent.type(args, "extra");
  expect(callbacks.onExecutableChange).not.toHaveBeenCalled();
  expect(callbacks.onArgsChange).not.toHaveBeenCalled();
});

it("preserves the scoped disclosure and blank executable guard", async () => {
  const callbacks = props({ openAdvanced: false, executable: "   " });
  render(<CheckCommandEditor {...callbacks} />);
  const summary = screen.getByText("Advanced check commands");
  expect(summary.closest("details")).toHaveProperty("open", false);
  await userEvent.click(summary);
  expect(summary.closest("details")).toHaveProperty("open", true);
  expect(screen.getByRole("button", { name: "Add check command" })).toHaveProperty("disabled", true);
  expect(callbacks.onAdd).not.toHaveBeenCalled();
});
