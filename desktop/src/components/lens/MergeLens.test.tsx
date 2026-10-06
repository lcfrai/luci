import { StrictMode } from "react";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { productApi, type GitState } from "../../api/product";
import { ProjectGit } from "./MergeLens";

const identity = vi.hoisted(() => ({ actor: "alice", workspace: "local" }));
const access = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: false, secrets: false, deploy: false, admin: false } }));
const refreshProjects = vi.hoisted(() => vi.fn());
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: identity.actor } } }) }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ refreshProjects, workspace: { mode: "local", connectionId: identity.workspace, host: "" }, status: { config: { projectsDir: "/projects" }, access: { user: { username: identity.actor } } } }) }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => access, useTeamWorkspace: () => null }));
vi.mock("../journey/ProjectDelivery", () => ({ ProjectDelivery: () => <section aria-label="Project delivery" /> }));
vi.mock("../journey/ProjectBackup", () => ({ ProjectBackup: () => <section aria-label="Project backup" /> }));

const review: GitState = {
  reviewDigest: "a".repeat(64), branch: "main", clean: false,
  changes: [{ path: "src/app.ts", status: " M" }],
  log: [{ hash: "a".repeat(40), subject: "Earlier application" }], diff: "- Earlier behavior\n+ Reviewed behavior",
};
const updated: GitState = { ...review, reviewDigest: "b".repeat(64), diff: "+ Updated behavior" };
const committed = { commit: "c".repeat(40), message: "Improve application", sourceChanged: false };
function deferred<T>() {
  let resolve!: (value: T) => void, reject!: (cause: Error) => void;
  return { promise: new Promise<T>((done, fail) => { resolve = done; reject = fail; }), resolve, reject };
}
const message = () => screen.getByLabelText("Commit message") as HTMLTextAreaElement;
const commitButton = () => screen.getByRole("button", { name: "Commit changes" }) as HTMLButtonElement;
const typeMessage = (value = "Improve application") => fireEvent.change(message(), { target: { value } });
const refresh = () => userEvent.click(screen.getByRole("button", { name: "Refresh review" }));
async function ready() { await screen.findByLabelText("Commit message"); await waitFor(() => expect((screen.getByRole("button", { name: "Refresh review" }) as HTMLButtonElement).disabled).toBe(false)); }

beforeEach(() => {
  identity.actor = "alice"; identity.workspace = "local"; access.loading = false; access.error = "";
  Object.assign(access.permissions, { read: true, edit: true, execute: false, secrets: false, deploy: false, admin: false });
  vi.spyOn(productApi, "git").mockResolvedValue(review);
  vi.spyOn(productApi, "commit").mockResolvedValue(committed);
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("commits only the displayed review with editing access and preserves the local-only boundary", async () => {
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage("  Improve application  ");
  await userEvent.click(commitButton());
  await screen.findByText("Reviewed changes committed.");
  expect(productApi.commit).toHaveBeenCalledExactlyOnceWith("p-1", "Improve application", review.reviewDigest);
  expect(message().value).toBe(""); expect(refreshProjects).toHaveBeenCalledOnce();
  expect(screen.getByText(/Nothing is pushed\./)).not.toBeNull();
});

it("requires a source identity from the service before admitting a commit", async () => {
  vi.mocked(productApi.git).mockResolvedValue({ ...review, reviewDigest: undefined });
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage();
  expect(screen.getByText(/This review has no source identity/)).not.toBeNull();
  expect(commitButton().disabled).toBe(true);
  fireEvent.submit(message().closest("form")!);
  expect(productApi.commit).not.toHaveBeenCalled();
});

it.each(["clean", "blank"])("refuses a %s commit", async invalid => {
  if (invalid === "clean") vi.mocked(productApi.git).mockResolvedValue({ ...review, clean: true, changes: [] });
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage(invalid === "blank" ? " \n " : "Improve application");
  expect(commitButton().disabled).toBe(true); fireEvent.submit(message().closest("form")!);
  expect(productApi.commit).not.toHaveBeenCalled();
});

it("preserves a conflicted message and requires an explicit refreshed review before retrying", async () => {
  vi.mocked(productApi.commit).mockRejectedValueOnce(new Error("The source changed after your review."));
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage("Keep this explanation");
  await userEvent.click(commitButton());
  await screen.findByRole("alert");
  expect(message().value).toBe("Keep this explanation"); expect(commitButton().disabled).toBe(true);
  expect(productApi.git).toHaveBeenCalledTimes(1);
  fireEvent.submit(message().closest("form")!); expect(productApi.commit).toHaveBeenCalledTimes(1);
  vi.mocked(productApi.git).mockResolvedValue(updated); await refresh();
  await screen.findByText("+ Updated behavior"); expect(message().value).toBe("Keep this explanation");
  expect(commitButton().disabled).toBe(false); await userEvent.click(commitButton());
  expect(productApi.commit).toHaveBeenLastCalledWith("p-1", "Keep this explanation", updated.reviewDigest);
});

it("keeps a failed refresh from authorizing the old displayed source", async () => {
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage();
  vi.mocked(productApi.git).mockRejectedValueOnce(new Error("Repository connection interrupted"));
  await refresh(); await screen.findByRole("alert");
  expect(screen.getByText(/Reviewed behavior/)).not.toBeNull();
  expect(message().value).toBe("Improve application"); expect(commitButton().disabled).toBe(true);
  vi.mocked(productApi.git).mockResolvedValue(updated); await refresh();
  await screen.findByText("+ Updated behavior"); expect(commitButton().disabled).toBe(false);
});

it("does not invite a duplicate commit when a successful commit's refresh fails", async () => {
  vi.mocked(productApi.git).mockResolvedValueOnce(review).mockRejectedValueOnce(new Error("Could not refresh after commit"));
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage(); await userEvent.click(commitButton());
  await screen.findByText("Could not refresh after commit");
  expect(screen.getByText("Reviewed changes committed.")).not.toBeNull();
  expect(message().value).toBe(""); expect(commitButton().disabled).toBe(true);
  typeMessage("New work"); fireEvent.submit(message().closest("form")!);
  expect(productApi.commit).toHaveBeenCalledTimes(1); expect(commitButton().disabled).toBe(true);
  vi.mocked(productApi.git).mockResolvedValue(updated); await refresh();
  await screen.findByText("+ Updated behavior"); expect(commitButton().disabled).toBe(false);
});

it("identifies newer uncommitted edits separately from the successful reviewed commit", async () => {
  vi.mocked(productApi.commit).mockResolvedValue({ ...committed, sourceChanged: true });
  vi.mocked(productApi.git).mockResolvedValueOnce(review).mockResolvedValue(updated);
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage(); await userEvent.click(commitButton());
  await screen.findByText("Reviewed changes committed. Newer edits remain uncommitted; review them separately.");
  await screen.findByText("+ Updated behavior"); expect(message().value).toBe("");
});

it("does not send another commit while the admitted commit is pending", async () => {
  const pending = deferred<typeof committed>(); vi.mocked(productApi.commit).mockReturnValue(pending.promise);
  render(<ProjectGit projectId="p-1" />); await ready(); typeMessage();
  fireEvent.submit(message().closest("form")!); fireEvent.submit(message().closest("form")!);
  expect(productApi.commit).toHaveBeenCalledTimes(1); expect(message().disabled).toBe(true);
  await act(async () => pending.resolve(committed));
});

it("lets viewers inspect source but does not admit edits or require runtime grants", async () => {
  access.permissions.edit = false;
  render(<ProjectGit projectId="p-1" />); await ready();
  expect(screen.getByText(/Reviewed behavior/)).not.toBeNull(); expect(message().disabled).toBe(true);
  expect(commitButton().disabled).toBe(true); typeMessage(); fireEvent.submit(message().closest("form")!);
  expect(productApi.commit).not.toHaveBeenCalled();
});

it("makes no repository request and reveals no source when reading is denied", () => {
  access.permissions.read = false; render(<ProjectGit projectId="p-1" />);
  expect(productApi.git).not.toHaveBeenCalled(); expect(screen.queryByLabelText("Commit message")).toBeNull();
  expect(screen.queryByLabelText("Project delivery")).toBeNull(); expect(screen.queryByLabelText("Project backup")).toBeNull();
  expect((screen.getByRole("button", { name: "Refresh review" }) as HTMLButtonElement).disabled).toBe(true);
});

it("preserves a typed message through an editing downgrade", async () => {
  const view = render(<ProjectGit projectId="p-1" />); await ready(); typeMessage("Keep my local words");
  access.permissions.edit = false; view.rerender(<ProjectGit projectId="p-1" />);
  expect(message().value).toBe("Keep my local words"); expect(message().disabled).toBe(true);
  access.permissions.edit = true; view.rerender(<ProjectGit projectId="p-1" />);
  expect(message().value).toBe("Keep my local words"); expect(message().disabled).toBe(false);
});

it.each(["edit", "read"] as const)("does not revive a commit after %s access is revoked and restored", async permission => {
  const pending = deferred<typeof committed>(); vi.mocked(productApi.commit).mockReturnValueOnce(pending.promise);
  const view = render(<ProjectGit projectId="p-1" />); await ready(); typeMessage("Keep my uncertain message"); await userEvent.click(commitButton());
  access.permissions[permission] = false; view.rerender(<ProjectGit projectId="p-1" />);
  if (permission === "read") {
    expect(screen.queryByText(/Reviewed behavior/)).toBeNull(); expect(screen.queryByLabelText("Commit message")).toBeNull();
    expect(screen.queryByLabelText("Project delivery")).toBeNull(); expect(screen.queryByLabelText("Project backup")).toBeNull();
  }
  access.permissions[permission] = true; view.rerender(<ProjectGit projectId="p-1" />); await ready();
  await act(async () => pending.resolve(committed));
  expect(message().value).toBe("Keep my uncertain message"); expect(commitButton().disabled).toBe(true);
  expect(screen.queryByText("Reviewed changes committed.")).toBeNull(); expect(refreshProjects).not.toHaveBeenCalled();
  expect(screen.getByText(/Project access changed while committing/)).not.toBeNull();
  await refresh(); expect(commitButton().disabled).toBe(false);
});

it.each(["loading", "error"] as const)("requires a new review if editing access is still %s when a commit finishes", async uncertainty => {
  const pending = deferred<typeof committed>(); vi.mocked(productApi.commit).mockReturnValueOnce(pending.promise);
  const view = render(<ProjectGit projectId="p-1" />); await ready(); typeMessage(); await userEvent.click(commitButton());
  if (uncertainty === "loading") access.loading = true; else access.error = "Permission read failed";
  view.rerender(<ProjectGit projectId="p-1" />); await act(async () => pending.resolve(committed));
  expect(message().value).toBe("Improve application"); expect(screen.getByText(/Editing access could not be confirmed/)).not.toBeNull();
  access.loading = false; access.error = ""; view.rerender(<ProjectGit projectId="p-1" />);
  expect(commitButton().disabled).toBe(true); expect(refreshProjects).not.toHaveBeenCalled();
  await refresh(); expect(commitButton().disabled).toBe(false);
});

it("does not let a stale failure clear or interrupt a newer authorized commit", async () => {
  const older = deferred<typeof committed>(), newer = deferred<typeof committed>();
  vi.mocked(productApi.commit).mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise);
  const view = render(<ProjectGit projectId="p-1" />); await ready(); typeMessage(); await userEvent.click(commitButton());
  access.permissions.edit = false; view.rerender(<ProjectGit projectId="p-1" />);
  access.permissions.edit = true; view.rerender(<ProjectGit projectId="p-1" />); await refresh(); await userEvent.click(commitButton());
  await act(async () => older.reject(new Error("Old private failure")));
  expect(screen.queryByRole("alert")).toBeNull(); expect(message().disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Committing…" }) as HTMLButtonElement).disabled).toBe(true);
  await act(async () => newer.resolve(committed)); expect(message().value).toBe("");
});

it.each(["success", "failure"])("ignores a StrictMode replay's late %s without overwriting the latest review", async outcome => {
  const older = deferred<GitState>(), newer = deferred<GitState>();
  vi.mocked(productApi.git).mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise);
  render(<StrictMode><ProjectGit projectId="p-1" /></StrictMode>);
  expect(productApi.git).toHaveBeenCalledTimes(2);
  await act(async () => newer.resolve(updated)); await ready(); typeMessage("Use only the latest source");
  await act(async () => { if (outcome === "success") older.resolve(review); else older.reject(new Error("Old read failed")); });
  expect(screen.getByText("+ Updated behavior")).not.toBeNull(); expect(screen.queryByRole("alert")).toBeNull();
  await userEvent.click(commitButton());
  expect(productApi.commit).toHaveBeenCalledExactlyOnceWith("p-1", "Use only the latest source", updated.reviewDigest);
});

it("drops an earlier private read across revocation even when read access returns before it resolves", async () => {
  const older = deferred<GitState>(); vi.mocked(productApi.git).mockReturnValueOnce(older.promise).mockResolvedValue(updated);
  const view = render(<ProjectGit projectId="p-1" />);
  access.permissions.read = false; view.rerender(<ProjectGit projectId="p-1" />);
  access.permissions.read = true; view.rerender(<ProjectGit projectId="p-1" />); await ready();
  await act(async () => older.resolve(review));
  expect(screen.getByText("+ Updated behavior")).not.toBeNull(); expect(screen.queryByText(/Reviewed behavior/)).toBeNull();
});

it.each(["account", "workspace", "project"])("clears private review and message, and ignores a late commit after a changed %s", async changed => {
  const pending = deferred<typeof committed>(); vi.mocked(productApi.commit).mockReturnValueOnce(pending.promise);
  const view = render(<ProjectGit projectId="p-1" />); await ready(); typeMessage("Private old message"); await userEvent.click(commitButton());
  if (changed === "account") identity.actor = "bob";
  if (changed === "workspace") identity.workspace = "team-b";
  vi.mocked(productApi.git).mockResolvedValue(updated);
  view.rerender(<ProjectGit projectId={changed === "project" ? "p-2" : "p-1"} />); await ready(); typeMessage("New workspace message");
  await act(async () => pending.resolve(committed));
  expect(message().value).toBe("New workspace message"); expect(screen.getByText("+ Updated behavior")).not.toBeNull();
  expect(screen.queryByText("Reviewed changes committed.")).toBeNull(); expect(refreshProjects).not.toHaveBeenCalled();
});

it.each(["account", "workspace", "project"])("ignores a late private source read after a changed %s", async changed => {
  const older = deferred<GitState>(); vi.mocked(productApi.git).mockReturnValueOnce(older.promise).mockResolvedValue(updated);
  const view = render(<ProjectGit projectId="p-1" />);
  if (changed === "account") identity.actor = "bob";
  if (changed === "workspace") identity.workspace = "team-b";
  view.rerender(<ProjectGit projectId={changed === "project" ? "p-2" : "p-1"} />); await ready();
  await act(async () => older.resolve(review));
  expect(screen.getByText("+ Updated behavior")).not.toBeNull(); expect(screen.queryByText(/Reviewed behavior/)).toBeNull();
});
