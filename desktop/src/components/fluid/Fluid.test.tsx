import { expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { Button } from "./components/button";
import { TabsSubtle, TabsSubtleItem, TabsSubtlePanel } from "./components/tabs-subtle";
import { SizeProvider } from "./lib/size-context";

it("does not accidentally submit a surrounding form and keeps loading labels accessible", () => {
  const submit = vi.fn();
  const view = render(<form onSubmit={submit}><Button>Check service</Button></form>);
  fireEvent.click(screen.getByRole("button", { name: "Check service" }));
  expect(submit).not.toHaveBeenCalled();
  view.rerender(<Button loading>Check service</Button>);
  const button = screen.getByRole("button", { name: "Check service" }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
  expect(button.getAttribute("aria-busy")).toBe("true");
});
it("uses the shared density scale", () => {
  const view = render(<SizeProvider size="compact"><Button>Save</Button></SizeProvider>);
  expect(screen.getByRole("button", { name: "Save" }).className).toContain("h-7");
  view.rerender(<SizeProvider size="default"><Button>Save</Button></SizeProvider>);
  expect(screen.getByRole("button", { name: "Save" }).className).toContain("h-9");
});
it("supports keyboard selection and links only the active content to its tab", async () => {
  function Example() {
    const [selected, setSelected] = useState(0);
    return <><TabsSubtle idPrefix="test-tools" aria-label="Tools" selectedIndex={selected} onSelect={setSelected}><TabsSubtleItem label="Code" index={0} /><TabsSubtleItem label="Test" index={1} /></TabsSubtle><TabsSubtlePanel idPrefix="test-tools" index={0} selectedIndex={selected}>Saved source</TabsSubtlePanel><TabsSubtlePanel idPrefix="test-tools" index={1} selectedIndex={selected}>Actual response</TabsSubtlePanel></>;
  }
  render(<Example />);
  const user = userEvent.setup();
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Code" }));
  await user.keyboard("{ArrowRight}");
  expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Test" }));
  expect(screen.getByRole("tab", { name: "Code" }).getAttribute("aria-selected")).toBe("true");
  await user.keyboard("{Enter}");
  expect(screen.getByRole("tabpanel").textContent).toBe("Actual response");
  expect(screen.getByRole("tabpanel").getAttribute("aria-labelledby")).toBe("test-tools-tab-1");
});
