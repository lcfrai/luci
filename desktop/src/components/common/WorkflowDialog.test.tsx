import { expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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
