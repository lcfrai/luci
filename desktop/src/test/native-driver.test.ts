import { afterEach, beforeEach, expect, it, vi } from "vitest";
import driverSource from "../../../scripts/native-installed-eval.mjs?raw";

// Evaluate only the real read-only DOM lookup in a fixture. The installed app
// driver and its Windows/disposable-runner admission are never executed here.
const start = driverSource.indexOf("async function lookupElement(");
const end = driverSource.indexOf("async function element(", start);
if (start < 0 || end <= start) throw new Error("Native lookup helper was not found.");
type Command = (path: string, method: string, input: { script: string; args: unknown[] }) => Promise<unknown>;
const lookup = new Function("command", `return (${driverSource.slice(start, end)});`)((async (path, method, input) => {
  expect(path).toBe("/execute/sync"); expect(method).toBe("POST");
  expect(input.script).not.toMatch(/\.click\s*\(|fetch\s*\(|XMLHttpRequest/);
  return new Function(input.script).apply(null, input.args);
}) satisfies Command) as (kind: string, selector: string) => Promise<Element | null>;

beforeEach(() => {
  vi.spyOn(Element.prototype, "getClientRects").mockImplementation(function (this: Element) {
    return (this.hasAttribute("hidden") ? [] : [{}]) as unknown as DOMRectList;
  });
});
afterEach(() => { document.body.innerHTML = ""; vi.restoreAllMocks(); });

it("finds the current visible project-entry dialog root after opening New project", async () => {
  document.body.innerHTML = '<div class="setup-window project-entry" role="dialog" aria-modal="true"><input aria-label="Project name"><button>Create project</button></div>';
  expect(await lookup("css", '.project-entry[role="dialog"][aria-modal="true"]')).toBe(document.querySelector(".project-entry"));
});

it("keeps descendant lookup inside the selected dialog", async () => {
  document.body.innerHTML = '<button id="outside">Create project</button><div class="setup-window" role="dialog" aria-modal="true"><input id="inside" aria-label="Project name"><button id="create">Create project</button></div>';
  expect(await lookup("css", "input")).toBe(document.getElementById("inside"));
  expect(await lookup("css", "#outside")).toBeNull();
  expect(await lookup("button", "Create project")).toBe(document.getElementById("create"));
});

it("uses the latest visible supported modal and ignores hidden roots", async () => {
  document.body.innerHTML = '<div class="setup-window" role="dialog" aria-modal="true"><input id="previous"></div><div class="setup-window project-entry" role="dialog" aria-modal="true"><input id="current"></div><div hidden class="setup-window" role="dialog" aria-modal="true"><input id="hidden"></div>';
  expect(await lookup("css", "input")).toBe(document.getElementById("current"));
  expect(await lookup("css", "#previous")).toBeNull();
});

it("retains visibility and disabled admission for ordinary native clicks", async () => {
  document.body.innerHTML = '<div class="setup-window" role="dialog" aria-modal="true"><button disabled id="disabled">Continue</button><button id="ready">Continue</button><input disabled id="disabled-input"></div>';
  expect(await lookup("button", "Continue")).toBe(document.getElementById("ready"));
  expect(await lookup("css", "#disabled-input")).toBeNull();
});

it("searches the document normally when no supported modal is open", async () => {
  document.body.innerHTML = '<section class="projects-home"><button id="new">New project</button></section>';
  expect(await lookup("css", ".projects-home")).toBe(document.querySelector(".projects-home"));
  expect(await lookup("button", "New project")).toBe(document.getElementById("new"));
  expect(await lookup("css", ".project-entry")).toBeNull();
});

it("does not admit a control disabled by its enclosing fieldset", async () => {
  document.body.innerHTML = '<fieldset disabled><button>Add requirement</button><label>Requirement 1<input></label></fieldset>';
  expect(await lookup("button", "Add requirement")).toBeNull();
  expect(await lookup("label", "Requirement 1")).toBeNull();
});

// Execute the actual driver helpers, keeping only the W3C transport at the
// test boundary. No runner admission, installed app or provider is launched.
const actionStart = driverSource.indexOf("async function until(");
const actionEnd = driverSource.indexOf("async function preparationSettled(", actionStart);
const fillStart = driverSource.indexOf("async function fill(");
const fillEnd = driverSource.indexOf("const bodyHas =", fillStart);
if (actionStart < 0 || actionEnd <= actionStart || fillStart < 0 || fillEnd <= fillStart) throw new Error("Native action helpers were not found.");
const actionSource = driverSource.slice(actionStart, actionEnd) + driverSource.slice(fillStart, fillEnd);
const key = "element-6066-11e4-a52e-4f735466cecf";
type Action = { call: string; kind: string; label: string; phase: string };
type Wire = { [key]: string };
type NativeCall = { path: string; method: string; deadline?: number };
type TransportInput = { script?: string; args?: unknown[]; text?: string };
type TransportHook = (call: NativeCall, input: TransportInput) => void;
const webdriverError = (code: string) => Object.assign(new Error("Controlled WebDriver rejection"), { webdriverError: code });

function actions(hook?: TransportHook) {
  const result: { actionStages?: Action[] } = {};
  const calls: NativeCall[] = [], elements = new Map<string, Element>(), ids = new WeakMap<Element, string>();
  const toWire = (e: Element): Wire => {
    let id = ids.get(e);
    if (!id) { id = "native-fixture-" + idsCount++; ids.set(e, id); elements.set(id, e); }
    return { [key]: id };
  };
  let idsCount = 0;
  const command = async (path: string, method = "POST", input: TransportInput = {}, deadline?: number) => {
    const call = { path, method, deadline }; calls.push(call); hook?.(call, input);
    if (deadline !== undefined && deadline <= Date.now()) throw new Error("Owned action deadline exceeded");
    if (path === "/execute/sync") {
      expect(method).toBe("POST");
      expect(input.script).not.toMatch(/\.click\s*\(|dispatchEvent\s*\(|fetch\s*\(|XMLHttpRequest/);
      const args = (input.args ?? []).map(value => value && typeof value === "object" && key in value ? elements.get((value as Wire)[key]) : value);
      const value = new Function(input.script!).apply(null, args);
      return value instanceof Element ? toWire(value) : value;
    }
    const match = /^\/element\/([^/]+)\/(click|clear|value)$/.exec(path);
    if (!match) throw new Error("Unexpected native transport call");
    const element = elements.get(match[1]);
    if (!element?.isConnected) throw webdriverError("stale element reference");
    if (element.matches(":disabled")) throw webdriverError("element not interactable");
    expect(method).toBe("POST");
    if (match[2] === "clear") (element as HTMLInputElement).value = "";
    if (match[2] === "value") (element as HTMLInputElement).value = input.text!;
    return null;
  };
  const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));
  const assert = (condition: unknown, message: string) => { if (!condition) throw new Error(message); };
  const helpers = new Function("command", "sleep", "assert", "result", "elementKey", `${actionSource}\nreturn {click,fill,centerElement};`)(command, sleep, assert, result, key) as {
    click: (name: string, kind?: string) => Promise<void>;
    fill: (name: string, value: string) => Promise<void>;
  };
  return { ...helpers, calls, result };
}

let originalScroll: PropertyDescriptor | undefined;
beforeEach(() => {
  originalScroll = Object.getOwnPropertyDescriptor(Element.prototype, "scrollIntoView");
  Object.defineProperty(Element.prototype, "scrollIntoView", { configurable: true, writable: true, value: vi.fn() });
});
afterEach(() => {
  vi.useRealTimers();
  if (originalScroll) Object.defineProperty(Element.prototype, "scrollIntoView", originalScroll);
  else Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

it("centers the current control and then sends exactly one real W3C click under the original deadline", async () => {
  document.body.innerHTML = '<button>Add requirement</button><input id="focused">';
  const focus = document.getElementById("focused") as HTMLInputElement; focus.focus();
  const target = document.querySelector("button")!;
  const h = actions(); await h.click("Add requirement");
  expect(target.scrollIntoView).toHaveBeenCalledOnce();
  expect(target.scrollIntoView).toHaveBeenCalledWith({ block: "center", inline: "nearest", behavior: "instant" });
  expect(document.activeElement).toBe(focus);
  expect(h.calls.map(c => c.path)).toEqual(["/execute/sync", "/execute/sync", "/execute/sync", "/element/native-fixture-0/click"]);
  expect(new Set(h.calls.map(c => c.deadline)).size).toBe(1);
  expect(h.result.actionStages).toEqual([{ call: "click", kind: "button", label: "Add requirement", phase: "completed" }]);
});

it("re-resolves a replaced target after scrolling and never clicks its obsolete identity", async () => {
  vi.useFakeTimers();
  document.body.innerHTML = '<button id="old">Add requirement</button>';
  const old = document.getElementById("old")!;
  old.scrollIntoView = () => { old.outerHTML = '<button id="current">Add requirement</button>'; };
  const h = actions(); const done = h.click("Add requirement");
  await vi.advanceTimersByTimeAsync(200); await done;
  expect(h.calls.filter(c => c.path.endsWith("/click")).map(c => c.path)).toEqual(["/element/native-fixture-1/click"]);
});

it("does not click a page control after scrolling opens a supported modal", async () => {
  vi.useFakeTimers(); document.body.innerHTML = '<button>Add requirement</button>';
  document.querySelector("button")!.scrollIntoView = () => { document.body.insertAdjacentHTML("beforeend", '<div class="setup-window" role="dialog" aria-modal="true"><button>Continue</button></div>'); };
  const h = actions(); const rejected = expect(h.click("Add requirement")).rejects.toThrow("timed out");
  await vi.advanceTimersByTimeAsync(30_000); await rejected;
  expect(h.calls.filter(c => c.path.endsWith("/click"))).toHaveLength(0);
  const stages = h.result.actionStages;
  expect(stages?.[stages.length - 1]?.phase).toBe("lookup");
});

it("keeps a control disabled during scrolling from receiving a native click", async () => {
  vi.useFakeTimers(); document.body.innerHTML = '<button>Add requirement</button>';
  const target = document.querySelector("button")!; target.scrollIntoView = () => { target.disabled = true; };
  const h = actions(); const rejected = expect(h.click("Add requirement")).rejects.toThrow("timed out");
  await vi.advanceTimersByTimeAsync(30_000); await rejected;
  expect(h.calls.filter(c => c.path.endsWith("/click"))).toHaveLength(0);
});

it.each(["stale element reference", "element click intercepted", "element not interactable"])("retries only the genuine pre-dispatch %s rejection", async code => {
  vi.useFakeTimers(); document.body.innerHTML = '<button>Add requirement</button>'; let count = 0;
  const h = actions(call => { if (call.path.endsWith("/click") && count++ === 0) throw webdriverError(code); });
  const done = h.click("Add requirement"); await vi.advanceTimersByTimeAsync(200); await done;
  expect(count).toBe(2);
  expect(new Set(h.calls.map(c => c.deadline)).size).toBe(1);
});

it.each([undefined, "unknown error", "invalid session id"])("does not replay an ambiguous or fatal WebDriver failure %s", async code => {
  document.body.innerHTML = '<button>Add requirement</button>'; let count = 0;
  const h = actions(call => { if (call.path.endsWith("/click")) { count++; throw code ? webdriverError(code) : new Error("Ambiguous transport failure"); } });
  await expect(h.click("Add requirement")).rejects.toThrow();
  expect(count).toBe(1);
  const stages = h.result.actionStages;
  expect(stages?.[stages.length - 1]).toEqual({ call: "click", kind: "button", label: "Add requirement", phase: "native-click" });
});

it("keeps one 30-second budget while repeated native pre-dispatch errors occur", async () => {
  vi.useFakeTimers(); vi.setSystemTime(0); document.body.innerHTML = '<button>Add requirement</button>';
  const h = actions(call => { if (call.path.endsWith("/click")) throw webdriverError("element click intercepted"); });
  const rejected = expect(h.click("Add requirement")).rejects.toThrow("timed out");
  await vi.advanceTimersByTimeAsync(30_000); await rejected;
  expect(new Set(h.calls.map(c => c.deadline))).toEqual(new Set([30_000]));
  expect(h.calls.filter(c => c.path.endsWith("/click"))).toHaveLength(150);
});

it("records only the fill control and native phase while preserving real clear and value calls", async () => {
  document.body.innerHTML = '<label>Password<input type="password"></label>';
  const h = actions(); const privateValue = "private-test-only-value"; await h.fill("Password", privateValue);
  expect(h.calls.map(c => c.path)).toEqual(["/execute/sync", "/element/native-fixture-0/clear", "/element/native-fixture-0/value"]);
  expect(document.querySelector("input")!.value).toBe(privateValue);
  expect(h.result.actionStages).toEqual([{ call: "fill", kind: "label", label: "Password", phase: "completed" }]);
  expect(JSON.stringify(h.result)).not.toContain(privateValue);
});

it("retains the precise failed fill phase without recording its private input", async () => {
  document.body.innerHTML = '<label>Password<input type="password"></label>';
  const h = actions(call => { if (call.path.endsWith("/value")) throw new Error("Ambiguous native value transport failure"); });
  const privateValue = "private-test-only-value";
  await expect(h.fill("Password", privateValue)).rejects.toThrow("Ambiguous native value transport failure");
  expect(h.calls.filter(c => c.path.endsWith("/value"))).toHaveLength(1);
  expect(h.result.actionStages).toEqual([{ call: "fill", kind: "label", label: "Password", phase: "native-value" }]);
  expect(JSON.stringify(h.result)).not.toContain(privateValue);
});
