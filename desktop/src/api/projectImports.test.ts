import { beforeEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import {
  clearPendingImport, createPendingImport, loadPendingImport, projectImportsApi,
  savePendingImport, validImportRepository, validPendingImport, verifiedImportAttempt,
  type PendingProjectImport, type ProjectImportAttempt,
} from "./projectImports";

const id = "imp-" + "a".repeat(24), otherId = "imp-" + "b".repeat(24);
const pending = (patch: Partial<PendingProjectImport> = {}): PendingProjectImport => ({ id, name: "garden-notes", repository: "https://github.com/example/garden-notes.git", ...patch });
const attempt = (patch: Partial<ProjectImportAttempt> = {}): ProjectImportAttempt => ({ id, name: "garden-notes", status: "running", message: "Import is still running.", ...patch });
const key = (scope: string) => "luci.project-import.v1:" + scope;
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); });

it("accepts ordinary HTTPS repository addresses without rewriting their identity", () => {
  for (const repository of ["https://github.com/example/repo", "https://git.example.test:8443/team/repo.git", "https://example.test/team/a%20b.git"]) {
    expect(validImportRepository(repository)).toBe(true);
    const value = createPendingImport("garden-notes_2", repository);
    expect(value.id).toMatch(/^imp-[a-f0-9]{24}$/);
    expect(value.name).toBe("garden-notes_2"); expect(value.repository).toBe(repository);
  }
});

it.each([
  "http://github.com/example/repo", "git@github.com:example/repo.git", "ssh://git@github.com/example/repo",
  "file:///tmp/private", "https://user@github.com/example/repo", "https://user:secret@github.com/example/repo",
  "https://github.com/example/repo?token=private", "https://github.com/example/repo?", "https://github.com/example/repo#private", "https://github.com/example/repo#",
  " https://github.com/example/repo", "https://github.com/example/repo ", "https://github.com/exam\nple/repo", "https://github.com\\@example.test/repo",
  "https://", "https://example.test/" + "a".repeat(2048),
])("rejects unsafe or credential-bearing repository recovery addresses: %s", repository => {
  expect(validImportRepository(repository)).toBe(false);
  expect(() => createPendingImport("garden-notes", repository)).toThrow("HTTPS repository URL without credentials");
  expect(validPendingImport(pending({ repository }))).toBe(false);
});

it("validates pending identity and project name before writing browser recovery", () => {
  for (const value of [null, {}, pending({ id: "imp-short" }), pending({ id: "imp-" + "A".repeat(24) }), pending({ name: "../garden" }), pending({ name: "-garden" }), pending({ name: "a".repeat(65) }), pending({ repository: "https://token:secret@example.test/repo" })]) {
    expect(validPendingImport(value)).toBe(false);
    expect(() => savePendingImport("scope", value as PendingProjectImport)).toThrow();
  }
  expect(localStorage.length).toBe(0);
});

it("isolates recovery by the full supplied account, workspace and project-root scope", () => {
  const scope = JSON.stringify(["https://luci.example", "alice", "team-one", "/one/root", "project-import"]);
  savePendingImport(scope, pending());
  expect(loadPendingImport(scope)).toEqual(pending());
  for (const other of [
    ["https://other.example", "alice", "team-one", "/one/root", "project-import"],
    ["https://luci.example", "bob", "team-one", "/one/root", "project-import"],
    ["https://luci.example", "alice", "team-two", "/one/root", "project-import"],
    ["https://luci.example", "alice", "team-one", "/other/root", "project-import"],
  ]) expect(loadPendingImport(JSON.stringify(other))).toBeNull();
  expect(loadPendingImport(null)).toBeNull();
  expect(() => savePendingImport(null, pending())).toThrow("could not be saved");
  expect(clearPendingImport(null, id)).toBe(false);
  expect(loadPendingImport(scope)).toEqual(pending());
});

it("refuses to overwrite another saved attempt and only replaces the explicitly expected identity", () => {
  savePendingImport("scope", pending());
  expect(() => savePendingImport("scope", pending({ id: otherId }))).toThrow("Another import");
  expect(() => savePendingImport("scope", pending({ id: otherId }), otherId)).toThrow("Another import");
  expect(loadPendingImport("scope")).toEqual(pending());
  savePendingImport("scope", pending({ id: otherId, name: "new-project" }), id);
  expect(loadPendingImport("scope")).toEqual(pending({ id: otherId, name: "new-project" }));
});

it("clears only the exact current attempt in its own scope", () => {
  savePendingImport("alice", pending()); savePendingImport("bob", pending({ id: otherId }));
  expect(clearPendingImport("alice", otherId)).toBe(false);
  expect(loadPendingImport("alice")).toEqual(pending());
  expect(clearPendingImport("alice", id)).toBe(true);
  expect(loadPendingImport("alice")).toBeNull();
  expect(loadPendingImport("bob")).toEqual(pending({ id: otherId }));
});

it.each(["{broken", JSON.stringify({ ...pending(), repository: "https://secret@example.test/repo" }), JSON.stringify({ ...pending(), unexpectedPadding: "x".repeat(4097) })])("does not replace corrupt or over-budget saved recovery", raw => {
  localStorage.setItem(key("scope"), raw);
  expect(loadPendingImport("scope")).toBeNull();
  expect(() => savePendingImport("scope", pending())).toThrow("Another import");
  expect(clearPendingImport("scope", id)).toBe(false);
  expect(localStorage.getItem(key("scope"))).toBe(raw);
});

it("retains only the allowed recovery fields and persists the longest supported repository/name", () => {
  const withExtra = { ...pending(), accessToken: "must-not-persist", rawError: "must-not-persist" };
  savePendingImport("scope", withExtra);
  expect(JSON.parse(localStorage.getItem(key("scope"))!)).toEqual(pending());
  localStorage.setItem(key("scope"), JSON.stringify(withExtra));
  expect(loadPendingImport("scope")).toEqual(pending());
  const prefix = "https://example.test/", repository = prefix + "a".repeat(2048 - prefix.length), name = "a".repeat(64);
  expect(repository).toHaveLength(2048); expect(name).toHaveLength(64);
  expect(validImportRepository(repository)).toBe(true);
  const maximum = pending({ repository, name });
  savePendingImport("maximum", maximum);
  expect(loadPendingImport("maximum")).toEqual(maximum);
  expect(localStorage.getItem(key("maximum"))!.length * 2).toBeLessThanOrEqual(8192);
});

it("blocks import admission when recovery cannot be saved and leaves failed clearing truthful", () => {
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Sensitive quota diagnostic"); });
  expect(() => savePendingImport("scope", pending())).toThrow("Enable browser storage");
  expect(() => savePendingImport("scope", pending())).not.toThrow("Sensitive quota diagnostic");
  vi.restoreAllMocks(); savePendingImport("scope", pending());
  vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => { throw new Error("Denied"); });
  expect(clearPendingImport("scope", id)).toBe(false);
  expect(loadPendingImport("scope")).toEqual(pending());
});

it("observes status with a read-only request and never retries the import", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(attempt({ status: "unconfirmed" }));
  expect(await projectImportsApi.status(id)).toEqual(attempt({ status: "unconfirmed" }));
  expect(request.mock.calls).toEqual([["/v1/product/project-imports/" + id]]);
  request.mockRejectedValue(new Error("Status unavailable"));
  await expect(projectImportsApi.status(id)).rejects.toThrow("Status unavailable");
  expect(request).toHaveBeenCalledTimes(2);
  await expect(projectImportsApi.status("../foreign")).rejects.toThrow("identity is invalid");
  expect(request).toHaveBeenCalledTimes(2);
});

it("rejects foreign identities, unknown statuses and incomplete successful receipts", async () => {
  const request = vi.spyOn(productApi, "request");
  for (const value of [null, {}, attempt({ id: otherId }), attempt({ name: "../other" }), { ...attempt(), status: "unknown-success" }, { ...attempt(), message: 42 }, { ...attempt(), retainedPath: 42 }, attempt({ status: "complete" }), attempt({ project: { id: "p-1", name: "foreign", path: "/projects/garden" } }), attempt({ status: "complete", project: { id: "", name: "garden-notes", path: "/projects/garden" } })]) {
    request.mockResolvedValue(value);
    await expect(projectImportsApi.status(id)).rejects.toThrow("could not be verified");
  }
  expect(() => verifiedImportAttempt(attempt(), id, "different-project")).toThrow("could not be verified");
  const complete = attempt({ status: "complete", project: { id: "p-1", name: "garden-notes", path: "/projects/garden-notes" } });
  expect(verifiedImportAttempt(complete, id, "garden-notes")).toEqual(complete);
});

it("bounds saved valid JSON at 8192 UTF-16 bytes without overwriting oversized recovery", () => {
  const base = { ...pending(), unexpectedPadding: "" };
  const exact = JSON.stringify({ ...base, unexpectedPadding: "x".repeat(4096 - JSON.stringify(base).length) });
  expect(exact.length * 2).toBe(8192);
  localStorage.setItem(key("exact"), exact);
  expect(loadPendingImport("exact")).toEqual(pending());
  const oversized = JSON.stringify({ ...base, unexpectedPadding: "x".repeat(4097 - JSON.stringify(base).length) });
  expect(oversized.length * 2).toBe(8194);
  expect(JSON.parse(oversized).id).toBe(id);
  localStorage.setItem(key("oversized"), oversized);
  expect(loadPendingImport("oversized")).toBeNull();
  expect(() => savePendingImport("oversized", pending(), id)).toThrow("Another import");
  expect(clearPendingImport("oversized", id)).toBe(false);
  expect(localStorage.getItem(key("oversized"))).toBe(oversized);
});

it("accepts bounded receipts including extended Windows paths at their supported limits", () => {
  const prefix = "\\\\?\\C:\\", path = prefix + "a".repeat(32768 - prefix.length);
  const maximum = attempt({ status: "complete", message: "m".repeat(4096), retainedPath: path, project: { id: "p-" + "a".repeat(254), name: "garden-notes", path } });
  expect(maximum.project!.id).toHaveLength(256); expect(path).toHaveLength(32768);
  expect(verifiedImportAttempt(maximum, id, "garden-notes")).toEqual(maximum);
});

it.each([
  { label: "array status", patch: { status: ["running"] } },
  { label: "boxed status", patch: { status: new String("running") } },
  { label: "null status", patch: { status: null } },
  { label: "overlong message", patch: { message: "m".repeat(4097) } },
  { label: "overlong retained path", patch: { retainedPath: "a".repeat(32769) } },
  { label: "overlong project path", patch: { project: { id: "p-1", name: "garden-notes", path: "a".repeat(32769) } } },
  { label: "overlong project identity", patch: { project: { id: "p-" + "a".repeat(255), name: "garden-notes", path: "/projects/garden-notes" } } },
])("rejects $label before the receipt can drive navigation or recovery", ({ patch }) => {
  expect(() => verifiedImportAttempt({ ...attempt(), ...patch }, id)).toThrow("could not be verified");
});
