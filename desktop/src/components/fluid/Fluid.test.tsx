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

function narrowTabGeometry() {
  let width = 420;
  let scale = 1;
  let nextFrame = 0;
  const frames = new Map<number, FrameRequestCallback>();
  const scrollPositions = new WeakMap<HTMLElement, number>();
  const positions = [[4, 82], [86, 117], [203, 128]];
  const reads: { role: string | null; property: string; index: string | undefined }[] = [];
  const record = (element: HTMLElement, property: string) => {
    const role = element.getAttribute("role");
    if (role === "tab" || role === "tablist") reads.push({ role, property, index: element.dataset.fluidHoverIndex });
  };
  const item = (element: HTMLElement) => positions[Number(element.dataset.fluidHoverIndex)];
  const scrollWidth = () => Math.max(width, ...positions.map(([left, itemWidth]) => left + itemWidth + 4));
  const observers: TabResizeObserver[] = [];
  class TabResizeObserver implements ResizeObserver {
    readonly targets = new Set<Element>();
    constructor(readonly callback: ResizeObserverCallback) { observers.push(this); }
    observe(target: Element) { this.targets.add(target); }
    unobserve(target: Element) { this.targets.delete(target); }
    disconnect() { this.targets.clear(); }
  }
  vi.stubGlobal("ResizeObserver", TabResizeObserver);
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { const id = ++nextFrame; frames.set(id, callback); return id; });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => { frames.delete(id); });
  const clientWidth = vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(function (this: HTMLElement) { record(this, "clientWidth"); return this.getAttribute("role") === "tablist" ? width : 0; });
  const offsetWidth = vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockImplementation(function (this: HTMLElement) { record(this, "offsetWidth"); return this.getAttribute("role") === "tablist" ? width : item(this)?.[1] ?? 0; });
  const offsetHeight = vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockImplementation(function (this: HTMLElement) { record(this, "offsetHeight"); return this.getAttribute("role") === "tablist" ? 44 : this.getAttribute("role") === "tab" ? 36 : 0; });
  const offsetLeft = vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockImplementation(function (this: HTMLElement) { record(this, "offsetLeft"); return item(this)?.[0] ?? 0; });
  const offsetTop = vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockImplementation(function (this: HTMLElement) { record(this, "offsetTop"); return this.getAttribute("role") === "tab" ? 4 : 0; });
  const offsetParent = vi.spyOn(HTMLElement.prototype, "offsetParent", "get").mockImplementation(function (this: HTMLElement) { record(this, "offsetParent"); return this.getAttribute("role") === "tab" ? this.closest('[role="tablist"]') : null; });
  const contentWidth = vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockImplementation(function (this: HTMLElement) { record(this, "scrollWidth"); return this.getAttribute("role") === "tablist" ? scrollWidth() : 0; });
  const originalScroll = Object.getOwnPropertyDescriptor(Element.prototype, "scrollLeft")!;
  const scrollGet = vi.spyOn(HTMLElement.prototype, "scrollLeft", "get").mockImplementation(function (this: HTMLElement) { return this.getAttribute("role") === "tablist" ? scrollPositions.get(this) ?? 0 : originalScroll.get!.call(this) as number; });
  const scrollSet = vi.spyOn(HTMLElement.prototype, "scrollLeft", "set").mockImplementation(function (this: HTMLElement, value: number) {
    if (this.getAttribute("role") === "tablist") scrollPositions.set(this, Math.max(0, Math.min(value, scrollWidth() - width)));
    else originalScroll.set!.call(this, value);
  });
  const originalRect = HTMLElement.prototype.getBoundingClientRect;
  const rect = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    record(this, "getBoundingClientRect");
    if (this.getAttribute("role") === "tablist") return DOMRect.fromRect({ x: 100, y: 200, width: width * scale, height: 44 * scale });
    if (this.getAttribute("role") === "tab") {
      const [left, itemWidth] = positions[Number(this.dataset.fluidHoverIndex)];
      const list = this.closest('[role="tablist"]') as HTMLElement;
      return DOMRect.fromRect({ x: 100 + (left - list.scrollLeft) * scale, y: 200 + 4 * scale, width: itemWidth * scale, height: 36 * scale });
    }
    return originalRect.call(this);
  });
  const scrollIntoView = vi.spyOn(Element.prototype, "scrollIntoView").mockClear();
  const pageScroll = vi.spyOn(window, "scrollTo").mockImplementation(() => {});
  const notify = (role: string) => act(() => { for (const observer of observers) if ([...observer.targets].some(element => element.isConnected && element.getAttribute("role") === role)) observer.callback([], observer); });
  return {
    reads, observers, scrollIntoView, pageScroll,
    setWidth(next: number) { width = next; },
    setScale(next: number) { scale = next; },
    resize(next: number) { width = next; notify("tablist"); },
    resizeItem(index: number, left: number, itemWidth: number) { positions[index] = [left, itemWidth]; notify("tab"); },
    async frame() {
      const pending = [...frames.values()]; frames.clear();
      await act(async () => { pending.forEach(callback => callback(performance.now())); });
    },
    restore() {
      for (const spy of [clientWidth, offsetWidth, offsetHeight, offsetLeft, offsetTop, offsetParent, contentWidth, scrollGet, scrollSet, rect, scrollIntoView, pageScroll].reverse()) spy.mockRestore();
      vi.unstubAllGlobals();
    },
  };
}
const teamTabs = <><TabsSubtleItem label="Members" index={0} /><TabsSubtleItem label="Project sharing" index={1} /><TabsSubtleItem label="Service and data" index={2} /></>;

it("reveals a programmatically selected final tab and its focus gutter after narrowing, without scrolling its parent", async () => {
  const geometry = narrowTabGeometry();
  const tabs = (selected: number) => <div data-testid="dialog-scroll"><TabsSubtle aria-label="Team settings" selectedIndex={selected} onSelect={() => {}}>{teamTabs}</TabsSubtle></div>;
  const view = render(tabs(2));
  try {
    const list = screen.getByRole("tablist"), parent = screen.getByTestId("dialog-scroll"); parent.scrollTop = 80;
    await geometry.frame();
    expect(list.scrollLeft).toBe(0);
    geometry.resize(316);
    await geometry.frame();
    const selected = screen.getByRole("tab", { name: "Service and data" });
    expect(selected.getBoundingClientRect().right + 4).toBeLessThanOrEqual(list.getBoundingClientRect().right);
    expect(list.scrollLeft).toBeGreaterThan(0);
    expect(parent.scrollTop).toBe(80);
    expect(geometry.scrollIntoView).not.toHaveBeenCalled();
    expect(geometry.pageScroll).not.toHaveBeenCalled();
    view.rerender(tabs(0));
    expect(list.scrollLeft).toBe(0);
    view.rerender(tabs(2));
    expect(selected.getBoundingClientRect().right + 4).toBeLessThanOrEqual(list.getBoundingClientRect().right);
  } finally { view.unmount(); geometry.restore(); }
});

it("reveals keyboard focus before manual activation and returns to the first tab with Home", async () => {
  const geometry = narrowTabGeometry();
  function Example() {
    const [selected, setSelected] = useState(0);
    return <TabsSubtle aria-label="Team settings" selectedIndex={selected} onSelect={setSelected}>{teamTabs}</TabsSubtle>;
  }
  const view = render(<Example />);
  try {
    geometry.resize(316);
    await geometry.frame();
    const user = userEvent.setup(), list = screen.getByRole("tablist");
    await user.tab(); await user.keyboard("{End}");
    const last = screen.getByRole("tab", { name: "Service and data" });
    expect(document.activeElement).toBe(last);
    expect(last.getAttribute("aria-selected")).toBe("false");
    expect(last.getBoundingClientRect().right + 4).toBeLessThanOrEqual(list.getBoundingClientRect().right);
    await user.keyboard("{Enter}"); expect(last.getAttribute("aria-selected")).toBe("true");
    await user.keyboard("{Home}");
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Members" }));
    expect(last.getAttribute("aria-selected")).toBe("true");
    expect(list.scrollLeft).toBe(0);
    expect(geometry.scrollIntoView).not.toHaveBeenCalled();
  } finally { view.unmount(); geometry.restore(); }
});

it("coalesces registration and repeated resize notifications into one geometry snapshot", async () => {
  const geometry = narrowTabGeometry();
  const view = render(<TabsSubtle aria-label="Team settings" selectedIndex={2} onSelect={() => {}}>{teamTabs}</TabsSubtle>);
  try {
    expect(geometry.reads).toEqual([]);
    await geometry.frame();
    expect(geometry.reads.filter(read => read.role === "tab" && read.property === "offsetWidth").map(read => read.index)).toEqual(["0", "1", "2"]);
    expect(geometry.observers.filter(observer => [...observer.targets].some(target => target.getAttribute("role") === "tablist"))).toHaveLength(1);
    geometry.reads.length = 0;
    geometry.resize(320); geometry.resize(316); geometry.resizeItem(2, 203, 148);
    expect(geometry.reads).toEqual([]);
    await geometry.frame();
    expect(geometry.reads.filter(read => read.role === "tab" && read.property === "offsetWidth").map(read => read.index)).toEqual(["0", "1", "2"]);
    expect(geometry.reads.filter(read => read.role === "tablist" && read.property === "clientWidth")).toHaveLength(1);
    expect(screen.getByRole("tablist").scrollLeft).toBe(39);
  } finally { view.unmount(); geometry.restore(); }
});

it("reuses measured geometry across programmatic selection and unchanged parent renders", async () => {
  const geometry = narrowTabGeometry(); geometry.setWidth(180);
  const tabs = (selected: number) => <TabsSubtle aria-label="Team settings" selectedIndex={selected} onSelect={() => {}}>
    <TabsSubtleItem label="Members" index={0} /><TabsSubtleItem label="Project sharing" index={1} /><TabsSubtleItem label="Service and data" index={2} />
  </TabsSubtle>;
  const view = render(tabs(0));
  try {
    await geometry.frame(); geometry.reads.length = 0;
    view.rerender(tabs(2)); expect(screen.getByRole("tablist").scrollLeft).toBe(155);
    view.rerender(tabs(0)); expect(screen.getByRole("tablist").scrollLeft).toBe(0);
    view.rerender(tabs(1)); expect(screen.getByRole("tablist").scrollLeft).toBe(27);
    view.rerender(tabs(1));
    await geometry.frame();
    expect(geometry.reads).toEqual([]);
  } finally { view.unmount(); geometry.restore(); }
});

it("uses one layout snapshot for the selected pill and transformed viewport reveal", async () => {
  const geometry = narrowTabGeometry(); geometry.setWidth(316); geometry.setScale(1.5);
  const view = render(<div data-testid="dialog-scroll"><TabsSubtle aria-label="Team settings" selectedIndex={2} onSelect={() => {}}>{teamTabs}</TabsSubtle></div>);
  try {
    const parent = screen.getByTestId("dialog-scroll"); parent.scrollTop = 80;
    await geometry.frame();
    const list = screen.getByRole("tablist"), last = screen.getByRole("tab", { name: "Service and data" });
    expect(list.scrollLeft).toBe(19);
    const pill = list.querySelector<HTMLElement>(":scope > .bg-active")!;
    expect(pill).not.toBeNull();
    expect(pill.style.left).toBe("203px"); expect(pill.style.width).toBe("128px");
    expect(pill.style.top).toBe("4px"); expect(pill.style.height).toBe("36px");
    expect(last.getBoundingClientRect().right + 4 * 1.5).toBe(list.getBoundingClientRect().right);
    expect(parent.scrollTop).toBe(80); expect(geometry.scrollIntoView).not.toHaveBeenCalled(); expect(geometry.pageScroll).not.toHaveBeenCalled();
  } finally { view.unmount(); geometry.restore(); }
});

it("updates cached position after manual scrolling so End and Home reveal the correct focus gutter", async () => {
  const geometry = narrowTabGeometry(); geometry.setWidth(180);
  const view = render(<TabsSubtle aria-label="Team settings" selectedIndex={0} onSelect={() => {}}>{teamTabs}</TabsSubtle>);
  try {
    await geometry.frame(); const user = userEvent.setup(), list = screen.getByRole("tablist");
    await user.tab(); list.scrollLeft = 90; fireEvent.scroll(list); geometry.reads.length = 0;
    await user.keyboard("{End}");
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Service and data" })); expect(list.scrollLeft).toBe(155);
    list.scrollLeft = 20; fireEvent.scroll(list); await user.keyboard("{Home}");
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Members" })); expect(list.scrollLeft).toBe(0);
    expect(geometry.reads).toEqual([]); expect(geometry.scrollIntoView).not.toHaveBeenCalled();
  } finally { view.unmount(); geometry.restore(); }
});

it("uses the latest requested selection when a coalesced measurement is pending", async () => {
  const geometry = narrowTabGeometry();
  const tabs = (selected: number) => <TabsSubtle aria-label="Team settings" selectedIndex={selected} onSelect={() => {}}>{teamTabs}</TabsSubtle>;
  const view = render(tabs(0));
  try {
    await geometry.frame(); geometry.reads.length = 0;
    geometry.resize(180); view.rerender(tabs(2)); view.rerender(tabs(1));
    expect(geometry.reads).toEqual([]); expect(screen.getByRole("tablist").scrollLeft).toBe(0);
    await geometry.frame();
    expect(screen.getByRole("tablist").scrollLeft).toBe(27);
    expect(screen.getByRole("tab", { name: "Project sharing" }).getAttribute("aria-selected")).toBe("true");
  } finally { view.unmount(); geometry.restore(); }
});

it("keeps the latest keyboard focus target through a pending resize without changing the selected tab", async () => {
  const geometry = narrowTabGeometry(); geometry.setWidth(316);
  const view = render(<TabsSubtle aria-label="Team settings" selectedIndex={2} onSelect={() => {}}>{teamTabs}</TabsSubtle>);
  try {
    await geometry.frame(); const user = userEvent.setup(), list = screen.getByRole("tablist"); await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Service and data" }));
    geometry.reads.length = 0; geometry.resize(180);
    await user.keyboard("{Home}{End}{Home}");
    expect(geometry.reads).toEqual([]); expect(list.scrollLeft).toBe(19);
    await geometry.frame();
    expect(document.activeElement).toBe(screen.getByRole("tab", { name: "Members" })); expect(list.scrollLeft).toBe(0);
    expect(screen.getByRole("tab", { name: "Service and data" }).getAttribute("aria-selected")).toBe("true");
  } finally { view.unmount(); geometry.restore(); }
});

it("tracks native scrolling on a replacement tablist and ignores events from its retired container", async () => {
  const geometry = narrowTabGeometry(); geometry.setWidth(220);
  const tabs = (selected: number, compact = false) => <TabsSubtle aria-label="Team settings" selectedIndex={selected} onSelect={() => {}} size={compact ? "compact" : undefined}>{teamTabs}</TabsSubtle>;
  const view = render(tabs(0));
  try {
    await geometry.frame(); const previous = screen.getByRole("tablist");
    view.rerender(tabs(0, true)); await geometry.frame();
    const current = screen.getByRole("tablist"); expect(current).not.toBe(previous); expect(previous.isConnected).toBe(false);
    current.scrollLeft = 115; fireEvent.scroll(current);
    view.rerender(tabs(1, true));
    expect(current.scrollLeft).toBe(82);
    const selected = screen.getByRole("tab", { name: "Project sharing" });
    expect(selected.getBoundingClientRect().left).toBe(current.getBoundingClientRect().left + 4);
    expect(selected.getBoundingClientRect().right + 4).toBeLessThanOrEqual(current.getBoundingClientRect().right);
    previous.scrollLeft = 0; fireEvent.scroll(previous);
    view.rerender(tabs(0, true));
    expect(current.scrollLeft).toBe(0);
    expect(geometry.scrollIntoView).not.toHaveBeenCalled(); expect(geometry.pageScroll).not.toHaveBeenCalled();
  } finally { view.unmount(); geometry.restore(); }
});

it("remeasures a replacement tablist when its container narrows without changing any tab width", async () => {
  const geometry = narrowTabGeometry();
  const tabs = (compact = false) => <TabsSubtle aria-label="Team settings" selectedIndex={2} onSelect={() => {}} size={compact ? "compact" : undefined}>{teamTabs}</TabsSubtle>;
  const view = render(tabs());
  try {
    await geometry.frame(); const previous = screen.getByRole("tablist");
    view.rerender(tabs(true)); await geometry.frame();
    const current = screen.getByRole("tablist"); expect(current).not.toBe(previous); expect(current.scrollLeft).toBe(0);
    geometry.resize(220); await geometry.frame();
    expect(current.scrollLeft).toBe(115);
    expect(screen.getByRole("tab", { name: "Service and data" }).getBoundingClientRect().right + 4).toBe(current.getBoundingClientRect().right);
    expect(geometry.observers.filter(observer => observer.targets.has(current))).toHaveLength(1);
    expect(geometry.observers.some(observer => observer.targets.has(previous))).toBe(false);
  } finally { view.unmount(); geometry.restore(); }
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
