import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { projectContextApi, type ProjectContext, type ContextArchivePage } from "../../api/projectContext";
import { productApi } from "../../api/product";
import { ContextStorage } from "./ContextStorage";

const identity = vi.hoisted(() => ({ actor: "alice", workspace: "local" }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: identity.actor } } }) }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ workspace: { mode: "local", connectionId: identity.workspace, host: "" }, status: { config: { projectsDir: "/projects" }, access: { user: { username: "alice" } } } }) }));
vi.mock("./ContextHistory", () => ({ ContextHistory: () => <div>Context activity fixture</div> }));
const head = "a".repeat(64), older = "b".repeat(64);
const fixture = (patch: Partial<ProjectContext> = {}): ProjectContext => ({ projectId: "p-1", contextId: "portable-context", imported: false, expectedDigest: "saved-digest", exports: [{ path: "LUCI.md", status: "current" }, { path: ".luci/specification.md", status: "changed" }], archiveHead: head, archiveChunks: 2, archiveBytes: 16000, archiveRevisions: 8, archiveBytesLimit: 268435456, archiveChunksLimit: 256, recentRevisions: 32, unusedArchiveChunks: 0, unusedArchiveBytes: 0, ...patch });
const archived = (digest = head, nextCursor = older): ContextArchivePage => ({ digest, nextCursor, chunk: { version: 1, contextId: "portable-context", projectId: "original-imported-project", previous: nextCursor, briefRevisions: [{ projectId: "original-imported-project", revision: 1, purpose: "Archived original purpose", audience: "People", constraints: "", criteria: [], updatedAt: "2026-10-05T00:00:00Z" }] } });
async function open() { await userEvent.click(screen.getByText("Context storage and history")); await screen.findByText("Files shared with agents"); }
let download: ReturnType<typeof vi.spyOn>;
beforeEach(() => {
  identity.actor = "alice"; identity.workspace = "local";
  vi.spyOn(projectContextApi, "get").mockResolvedValue(fixture());
  vi.spyOn(projectContextApi, "repair").mockResolvedValue(fixture({ exports: [{ path: "LUCI.md", status: "current" }, { path: ".luci/specification.md", status: "current" }] }));
  vi.spyOn(projectContextApi, "archive").mockImplementation(async (_id, cursor) => archived(cursor, cursor === head ? older : ""));
  vi.spyOn(projectContextApi, "prune").mockResolvedValue(fixture({ expectedDigest: "pruned-digest", archiveHead: "", archiveChunks: 0, archiveBytes: 0, archiveRevisions: 0 }));
  vi.spyOn(productApi, "backupProject").mockResolvedValue(new Blob(["zip-fixture"], { type: "application/zip" }));
  vi.stubGlobal("URL", class extends URL { static createObjectURL = vi.fn(() => "blob:fixture"); static revokeObjectURL = vi.fn(); });
  download = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("reads only after opening and repairs generated exports only after reviewing the replacement action", async () => {
  render(<ContextStorage projectId="p-1" />);
  expect(projectContextApi.get).not.toHaveBeenCalled(); await open();
  expect(projectContextApi.repair).not.toHaveBeenCalled(); expect(screen.getByText("Changed outside the editor")).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Review export repair" }));
  expect(screen.getByRole("dialog", { name: "Repair shared context files?" }).textContent).toContain("will be replaced");
  expect(projectContextApi.repair).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Repair context exports" }));
  await screen.findByText("Shared context files now match the saved project.");
  expect(projectContextApi.repair).toHaveBeenCalledExactlyOnceWith("p-1", "saved-digest");
});

it("browses immutable imported history with its stable context identity and exact cursor", async () => {
  vi.mocked(projectContextApi.get).mockResolvedValue(fixture({ imported: true }));
  render(<ContextStorage projectId="p-1" />); await open();
  expect(screen.getByText(/historical review records do not grant current approvals/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Browse archived revisions" }));
  await screen.findByText(/Archived original purpose/);
  expect(projectContextApi.archive).toHaveBeenLastCalledWith("p-1", head);
  await userEvent.click(screen.getByRole("button", { name: "Older archive page" }));
  await waitFor(() => expect(projectContextApi.archive).toHaveBeenLastCalledWith("p-1", older));
  await userEvent.click(screen.getByRole("button", { name: "Newer archive page" }));
  await waitFor(() => expect(projectContextApi.archive).toHaveBeenLastCalledWith("p-1", head));
  expect(projectContextApi.prune).not.toHaveBeenCalled();
});

it("refuses archive pages from another context even when they reuse the requested digest", async () => {
  vi.mocked(projectContextApi.archive).mockResolvedValue({ ...archived(), chunk: { ...archived().chunk!, contextId: "foreign-context" } });
  render(<ContextStorage projectId="p-1" />); await open();
  await userEvent.click(screen.getByRole("button", { name: "Browse archived revisions" }));
  expect((await screen.findByRole("alert")).textContent).toContain("does not match the selected project context");
  expect(screen.queryByText(/Archived original purpose/)).toBeNull();
});

it("requires an actual backup download and an explicit saved-file acknowledgement before removing archives", async () => {
  render(<ContextStorage projectId="p-1" />); await open();
  await userEvent.click(screen.getByText("Free archive space"));
  expect((screen.getByRole("button", { name: "Remove archived revisions" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  await screen.findByText(/Backup download started/);
  expect(download).toHaveBeenCalledOnce(); expect(projectContextApi.prune).not.toHaveBeenCalled();
  expect((screen.getByRole("button", { name: "Remove archived revisions" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("checkbox", { name: /I saved the backup/ }));
  await userEvent.click(screen.getByRole("button", { name: "Remove archived revisions" }));
  await screen.findByText("Archived revisions removed. Current documents and recent revisions are retained.");
  expect(projectContextApi.prune).toHaveBeenCalledExactlyOnceWith("p-1", "saved-digest");
});

it("cannot enable archive removal after a failed backup or a newer context observed during download", async () => {
  vi.mocked(productApi.backupProject).mockRejectedValueOnce(new Error("Backup failed"));
  render(<ContextStorage projectId="p-1" />); await open(); await userEvent.click(screen.getByText("Free archive space"));
  await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  await screen.findByText("Backup failed");
  expect(download).not.toHaveBeenCalled(); expect((screen.getByRole("checkbox") as HTMLInputElement).disabled).toBe(true);
  vi.mocked(projectContextApi.get).mockResolvedValue(fixture({ expectedDigest: "newer-context" }));
  await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  await screen.findByText(/saved context changed while preparing the backup/);
  expect(download).toHaveBeenCalledOnce(); expect((screen.getByRole("checkbox") as HTMLInputElement).disabled).toBe(true);
  expect(projectContextApi.prune).not.toHaveBeenCalled();
});

it("retains a stale repair refusal without automatically retrying against a newer version", async () => {
  vi.mocked(projectContextApi.repair).mockRejectedValue(new Error("Context changed. Reload before repairing exports."));
  render(<ContextStorage projectId="p-1" />); await open();
  await userEvent.click(screen.getByRole("button", { name: "Review export repair" }));
  await userEvent.click(screen.getByRole("button", { name: "Repair context exports" }));
  await screen.findByText("Context changed. Reload before repairing exports.");
  expect(projectContextApi.repair).toHaveBeenCalledTimes(1); expect(projectContextApi.get).toHaveBeenCalledTimes(1);
  expect(screen.queryByText("Shared context files now match the saved project.")).toBeNull();
});

it("drops a late backup result when the account or selected workspace changes", async () => {
  let finish!: (value: Blob) => void;
  vi.mocked(productApi.backupProject).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ContextStorage projectId="p-1" />); await open(); await userEvent.click(screen.getByText("Free archive space"));
  await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  identity.actor = "bob"; identity.workspace = "other"; view.rerender(<ContextStorage projectId="p-1" />); await open();
  await act(async () => finish(new Blob(["private old account backup"])));
  expect(download).not.toHaveBeenCalled(); expect(screen.queryByText(/Backup download started/)).toBeNull();
});

it("shows interrupted unused archive files and permits their explicit backup cleanup while blocking corrupt archives", async () => {
  vi.mocked(projectContextApi.get).mockResolvedValue(fixture({ archiveHead: "", archiveChunks: 0, archiveBytes: 0, archiveRevisions: 0, unusedArchiveChunks: 1, unusedArchiveBytes: 4000 }));
  const view = render(<ContextStorage projectId="p-1" />); await open();
  expect(screen.getByText(/unused archive files from an interrupted save/)).not.toBeNull(); expect(screen.getByText("Free archive space")).not.toBeNull();
  view.unmount(); vi.mocked(projectContextApi.get).mockResolvedValue(fixture({ archiveError: "Archive file digest mismatch" }));
  render(<ContextStorage projectId="p-1" />); await open();
  expect((await screen.findByRole("alert")).textContent).toContain("digest mismatch");
  expect(screen.queryByText("Free archive space")).toBeNull();
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it("lets a viewer inspect archive history while disabling export repair, private backup export and archive deletion", async () => {
  teamAccess.permissions.edit = false;
  render(<ContextStorage projectId="p-1" />); await open();
  expect((screen.getByRole("button", { name: "Review export repair" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Browse archived revisions" })); await screen.findByText(/Archived original purpose/);
  await userEvent.click(screen.getByText("Free archive space"));
  expect((screen.getByRole("button", { name: "Download backup before removal" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Remove archived revisions" }) as HTMLButtonElement).disabled).toBe(true);
  expect(projectContextApi.repair).not.toHaveBeenCalled(); expect(productApi.backupProject).not.toHaveBeenCalled(); expect(projectContextApi.prune).not.toHaveBeenCalled();
});

it.each(["backup", "context readback"])("drops a private backup after editing is revoked during %s, even if editing returns", async phase => {
  let finish!: (value: Blob | ProjectContext) => void;
  const pending = new Promise<Blob | ProjectContext>(resolve => { finish = resolve; });
  const view = render(<ContextStorage projectId="p-1" />); await open();
  if (phase === "backup") vi.mocked(productApi.backupProject).mockImplementationOnce(() => pending as Promise<Blob>);
  else vi.mocked(projectContextApi.get).mockImplementationOnce(() => pending as Promise<ProjectContext>);
  await userEvent.click(screen.getByText("Free archive space"));
  await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  if (phase === "context readback") await waitFor(() => expect(projectContextApi.get).toHaveBeenCalledTimes(2));
  teamAccess.permissions.edit = false; view.rerender(<ContextStorage projectId="p-1" />);
  teamAccess.permissions.edit = true; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(phase === "backup" ? new Blob(["private source"]) : fixture()));
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled();
  expect(screen.queryByText(/Backup download started/)).toBeNull();
  expect((screen.getByRole("checkbox", { name: /I saved the backup/ }) as HTMLInputElement).disabled).toBe(true);
  expect(projectContextApi.get).toHaveBeenCalledTimes(phase === "backup" ? 1 : 2);
});

it("keeps archive reading available through an edit downgrade and permission refresh, but fences actual read revocation", async () => {
  let finish!: (value: ContextArchivePage) => void;
  vi.mocked(projectContextApi.archive).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ContextStorage projectId="p-1" />); await open();
  await userEvent.click(screen.getByRole("button", { name: "Browse archived revisions" }));
  teamAccess.permissions.edit = false; teamAccess.loading = true; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(archived()));
  expect(screen.getByText(/Archived original purpose/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Download these revisions" })); expect(download).toHaveBeenCalledOnce();
  download.mockClear(); vi.mocked(URL.createObjectURL).mockClear();
  vi.mocked(projectContextApi.archive).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Older archive page" }));
  teamAccess.permissions.read = false; view.rerender(<ContextStorage projectId="p-1" />);
  teamAccess.permissions.read = true; teamAccess.loading = false; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(archived(older, "")));
  expect(screen.queryByText(/Archived original purpose/)).toBeNull(); expect(download).not.toHaveBeenCalled();
});

it("does not publish a late context read after access was lost and a fresh read completed", async () => {
  let finish!: (value: ProjectContext) => void;
  vi.mocked(projectContextApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ContextStorage projectId="p-1" />);
  await userEvent.click(screen.getByText("Context storage and history"));
  await waitFor(() => expect(projectContextApi.get).toHaveBeenCalledOnce());
  teamAccess.permissions.read = false; view.rerender(<ContextStorage projectId="p-1" />);
  vi.mocked(projectContextApi.get).mockResolvedValue(fixture({ archiveError: "Current archive needs repair" }));
  teamAccess.permissions.read = true; view.rerender(<ContextStorage projectId="p-1" />);
  await screen.findByText("Current archive needs repair");
  await act(async () => finish(fixture({ archiveError: "Private obsolete error" })));
  expect(screen.queryByText("Private obsolete error")).toBeNull(); expect(screen.getByText("Current archive needs repair")).not.toBeNull();
});

it("drops a backup readback when the disclosure unmounts", async () => {
  let finish!: (value: ProjectContext) => void;
  render(<ContextStorage projectId="p-1" />); await open();
  vi.mocked(projectContextApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByText("Free archive space")); await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  await waitFor(() => expect(projectContextApi.get).toHaveBeenCalledTimes(2));
  await userEvent.click(screen.getByText("Context storage and history"));
  await waitFor(() => expect(screen.queryByRole("region", { name: "Project context storage" })).toBeNull());
  await act(async () => finish(fixture()));
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled();
});

it.each(["backup", "context readback"])("cannot download or show success after read access disappears during %s", async phase => {
  let finish!: (value: Blob | ProjectContext) => void;
  const pending = new Promise<Blob | ProjectContext>(resolve => { finish = resolve; });
  const view = render(<ContextStorage projectId="p-1" />); await open();
  if (phase === "backup") vi.mocked(productApi.backupProject).mockImplementationOnce(() => pending as Promise<Blob>);
  else vi.mocked(projectContextApi.get).mockImplementationOnce(() => pending as Promise<ProjectContext>);
  await userEvent.click(screen.getByText("Free archive space")); await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  if (phase === "context readback") await waitFor(() => expect(projectContextApi.get).toHaveBeenCalledTimes(2));
  teamAccess.permissions.read = false; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(phase === "backup" ? new Blob(["private source"]) : fixture()));
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled(); expect(screen.queryByText(/Backup download started/)).toBeNull();
  expect(screen.queryByText("Files shared with agents")).toBeNull();
  expect(projectContextApi.get).toHaveBeenCalledTimes(phase === "backup" ? 1 : 2);
});

it("does not publish a late repair outcome after edit revocation", async () => {
  let finish!: (value: ProjectContext) => void;
  vi.mocked(projectContextApi.repair).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ContextStorage projectId="p-1" />); await open();
  await userEvent.click(screen.getByRole("button", { name: "Review export repair" })); await userEvent.click(screen.getByRole("button", { name: "Repair context exports" }));
  teamAccess.permissions.edit = false; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(fixture({ exports: [{ path: "LUCI.md", status: "current" }] })));
  expect(screen.queryByText("Shared context files now match the saved project.")).toBeNull();
  expect(screen.getByText("Changed outside the editor")).not.toBeNull();
  expect(screen.queryByRole("dialog")).toBeNull();
});

it.each(["backup", "context readback"])("retains an admitted %s through a refresh that confirms the same editing grants", async phase => {
  let finish!: (value: Blob | ProjectContext) => void;
  const pending = new Promise<Blob | ProjectContext>(resolve => { finish = resolve; });
  const view = render(<ContextStorage projectId="p-1" />); await open();
  if (phase === "backup") vi.mocked(productApi.backupProject).mockImplementationOnce(() => pending as Promise<Blob>);
  else vi.mocked(projectContextApi.get).mockImplementationOnce(() => pending as Promise<ProjectContext>);
  await userEvent.click(screen.getByText("Free archive space")); await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  if (phase === "context readback") await waitFor(() => expect(projectContextApi.get).toHaveBeenCalledTimes(2));
  teamAccess.loading = true; view.rerender(<ContextStorage projectId="p-1" />);
  expect(screen.getByText("Preparing context backup…")).not.toBeNull();
  teamAccess.loading = false; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(phase === "backup" ? new Blob(["private source"]) : fixture()));
  await screen.findByText(/Backup download started/);
  expect(download).toHaveBeenCalledOnce(); expect(productApi.backupProject).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("checkbox", { name: /I saved the backup/ }));
  teamAccess.loading = true; view.rerender(<ContextStorage projectId="p-1" />);
  expect((screen.getByRole("button", { name: "Remove archived revisions" }) as HTMLButtonElement).disabled).toBe(true);
  teamAccess.loading = false; view.rerender(<ContextStorage projectId="p-1" />);
  expect((screen.getByRole("checkbox", { name: /I saved the backup/ }) as HTMLInputElement).checked).toBe(true);
  expect((screen.getByRole("button", { name: "Remove archived revisions" }) as HTMLButtonElement).disabled).toBe(false);
});

it.each(["backup", "context readback"])("drops %s completion during permission refresh with an explicit retry and no stuck busy state", async phase => {
  let finish!: (value: Blob | ProjectContext) => void;
  const pending = new Promise<Blob | ProjectContext>(resolve => { finish = resolve; });
  const view = render(<ContextStorage projectId="p-1" />); await open();
  if (phase === "backup") vi.mocked(productApi.backupProject).mockImplementationOnce(() => pending as Promise<Blob>);
  else vi.mocked(projectContextApi.get).mockImplementationOnce(() => pending as Promise<ProjectContext>);
  await userEvent.click(screen.getByText("Free archive space")); await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  if (phase === "context readback") await waitFor(() => expect(projectContextApi.get).toHaveBeenCalledTimes(2));
  teamAccess.loading = true; view.rerender(<ContextStorage projectId="p-1" />);
  await act(async () => finish(phase === "backup" ? new Blob(["private source"]) : fixture()));
  expect(screen.getByText(/Editing access could not be confirmed when this action finished/)).not.toBeNull();
  expect(screen.queryByText("Preparing context backup…")).toBeNull();
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled();
  expect(projectContextApi.get).toHaveBeenCalledTimes(phase === "backup" ? 1 : 2);
  teamAccess.loading = false; view.rerender(<ContextStorage projectId="p-1" />);
  expect(download).not.toHaveBeenCalled(); expect((screen.getByRole("checkbox", { name: /I saved the backup/ }) as HTMLInputElement).checked).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Download backup before removal" }));
  await screen.findByText(/Backup download started/); expect(download).toHaveBeenCalledOnce();
  expect(productApi.backupProject).toHaveBeenCalledTimes(2);
});
