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
