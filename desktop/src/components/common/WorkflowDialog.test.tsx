import { expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { WorkflowDialog } from "./WorkflowDialog";

function DraftExample() {
  const [open, setOpen] = useState(false);
  return <>
    <label>Draft<textarea defaultValue="Keep this unsaved idea" /></label>
    <button onClick={() => setOpen(true)}>Design</button>
    {open && <WorkflowDialog title="Keep your changes?" description="Save your draft before continuing." onClose={() => setOpen(false)}>
      <button onClick={() => setOpen(false)}>Keep editing</button>
    </WorkflowDialog>}
  </>;
}

it.each(["Escape", "Keep editing"])("returns focus to the retained navigation control after %s closes a trigger-less Fluid dialog", async close => {
  render(<DraftExample />);
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Design" });
  await user.click(trigger);
  const dialog = await screen.findByRole("dialog", { name: "Keep your changes?" });
  expect(dialog.getAttribute("aria-modal")).toBe("true");
  await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));
  if (close === "Escape") await user.keyboard("{Escape}");
  else await user.click(screen.getByRole("button", { name: "Keep editing" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  // Radix's close autofocus runs after its focus trap is removed. Waiting here
  // also detects a premature hook cleanup that leaves focus on document.body.
  await waitFor(() => expect(document.activeElement).toBe(trigger));
  expect((screen.getByRole("textbox", { name: "Draft" }) as HTMLTextAreaElement).value).toBe("Keep this unsaved idea");
});

it.each([true, false])("resolves an explicit close target after the focus trap is removed, with current admission %s", async permitted => {
  const admission = { allowed: true };
  const target = vi.fn(() => {
    expect(screen.queryByRole("dialog")).toBeNull();
    return admission.allowed ? screen.getByRole("textbox", { name: "Draft" }) : null;
  });
  function Example() {
    const [open, setOpen] = useState(false);
    return <>
      <label>Draft<textarea defaultValue="Protected message" /></label>
      <button onClick={() => setOpen(true)}>Suggested answer</button>
      {open && <WorkflowDialog title="Choose answer" description="Review the draft first." onClose={() => setOpen(false)} getCloseFocusTarget={target}>
        <button onClick={() => setOpen(false)}>Use answer</button>
      </WorkflowDialog>}
    </>;
  }
  render(<Example />);
  await userEvent.click(screen.getByRole("button", { name: "Suggested answer" }));
  await screen.findByRole("dialog");
  fireEvent.click(screen.getByRole("button", { name: "Use answer" }));
  // The decision is evaluated by Radix's actual close event, not by the click
  // handler or a guessed number of animation frames before trap removal.
  expect(target).not.toHaveBeenCalled();
  admission.allowed = permitted;
  await waitFor(() => expect(target).toHaveBeenCalledOnce());
  const draft = screen.getByRole("textbox", { name: "Draft" });
  if (permitted) expect(document.activeElement).toBe(draft);
  else expect(document.activeElement).not.toBe(draft);
});
