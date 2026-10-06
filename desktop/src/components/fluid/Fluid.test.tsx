import { expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
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

function observeLabelMeasurements() {
  const reads: HTMLElement[] = [];
  const observers: RecordingResizeObserver[] = [];
  let width = 64;
  class RecordingResizeObserver implements ResizeObserver {
    readonly targets = new Set<Element>();
    readonly observed: Element[] = [];
    readonly disconnect = vi.fn(() => this.targets.clear());
    constructor(readonly callback: ResizeObserverCallback) { observers.push(this); }
    observe(target: Element) { this.targets.add(target); this.observed.push(target); }
    unobserve(target: Element) { this.targets.delete(target); }
  }
  const original = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "offsetWidth")!.get!;
  const measure = vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockImplementation(function (this: HTMLElement) {
    if (this.tagName === "SPAN" && this.classList.contains("inline-grid")) { reads.push(this); return width; }
    return original.call(this) as number;
  });
  vi.stubGlobal("ResizeObserver", RecordingResizeObserver);
  return {
    reads, observers,
    setWidth(value: number) { width = value; },
    labelObservers: () => observers.filter(observer => observer.observed.some(target => target.tagName === "SPAN" && target.classList.contains("inline-grid"))),
    restore() { measure.mockRestore(); vi.unstubAllGlobals(); },
  };
}

const TabIcon = () => <svg aria-hidden="true" />;

it.each([
  { activeLabel: false, withIcon: true },
  { activeLabel: true, withIcon: false },
])("does not measure ordinary labels with activeLabel=$activeLabel and icon=$withIcon", ({ activeLabel, withIcon }) => {
  const measurements = observeLabelMeasurements();
  const view = render(<TabsSubtle aria-label="Tools" selectedIndex={0} onSelect={() => {}} activeLabel={activeLabel}>
    <TabsSubtleItem label="Code" index={0} icon={withIcon ? TabIcon : undefined} />
    <TabsSubtleItem label="Test" index={1} icon={withIcon ? TabIcon : undefined} />
  </TabsSubtle>);
  try {
    expect(screen.getAllByRole("tab")).toHaveLength(2);
    // The shared tab geometry still has observers; only unused label
    // measurement is absent, rather than disabling observation altogether.
    expect(measurements.observers.some(observer => observer.observed.some(target => target.getAttribute("role") === "tab"))).toBe(true);
    expect(measurements.reads).toEqual([]);
    expect(measurements.labelObservers()).toEqual([]);
  } finally { view.unmount(); measurements.restore(); }
});

it("measures collapsing labels and disconnects their observers when mode changes or tabs unmount", () => {
  const measurements = observeLabelMeasurements();
  const tabs = (activeLabel: boolean) => <TabsSubtle aria-label="Tools" selectedIndex={0} onSelect={() => {}} activeLabel={activeLabel}>
    <TabsSubtleItem label="Code" index={0} icon={TabIcon} />
    <TabsSubtleItem label="Test" index={1} icon={TabIcon} />
  </TabsSubtle>;
  const view = render(tabs(false));
  try {
    expect(measurements.reads).toEqual([]);
    view.rerender(tabs(true));
    const initial = measurements.labelObservers();
    expect(initial).toHaveLength(1); // Only the selected icon's label is shown.
    const observed = [...initial[0].targets][0];
    expect(observed.textContent).toContain("Code");
    expect(measurements.reads).toContain(observed);
    const readsBeforeResize = measurements.reads.length;
    measurements.setWidth(96);
    act(() => initial[0].callback([], initial[0]));
    expect(measurements.reads.length).toBeGreaterThan(readsBeforeResize);

    view.rerender(tabs(false));
    expect(initial[0].disconnect).toHaveBeenCalled();
    expect(initial[0].targets.size).toBe(0);
    const ordinaryReads = measurements.reads.length;
    view.rerender(tabs(false));
    expect(measurements.reads).toHaveLength(ordinaryReads);
    expect(measurements.labelObservers()).toHaveLength(1);

    view.rerender(tabs(true));
    const reactivated = measurements.labelObservers();
    expect(reactivated).toHaveLength(2);
    expect(reactivated[1].targets.size).toBe(1);
    view.unmount();
    expect(reactivated[1].disconnect).toHaveBeenCalled();
    expect(reactivated[1].targets.size).toBe(0);
  } finally { view.unmount(); measurements.restore(); }
});
