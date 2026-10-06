import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { StandaloneDatabase } from "./StandaloneDatabase";
import { platformApi, type PlatformProject } from "../../api/platform";

vi.mock("../../api/platform", () => ({ platformApi: {
  database: vi.fn(), downloadBackup: vi.fn(), restoreDatabase: vi.fn(), recoverPreviousDatabase: vi.fn(), importDatabaseBackup: vi.fn(),
} }));
const access = vi.hoisted(() => ({ read: true, edit: true, deploy: true, secrets: true, loading: false, error: null as string | null, scope: "account-workspace" }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({
  loading: access.loading, error: access.error, canRead: access.read,
  canEdit: !access.loading && !access.error && access.read && access.edit,
  permissions: { read: access.read, edit: access.edit, deploy: access.deploy, secrets: access.secrets },
}) }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (projectId: string) => `${access.scope}:${projectId}` }));

const backup = { id: "backup-owned", createdAt: "2026-10-06T00:00:00Z", bytes: 2048, sha256: "a".repeat(64), database: "app" };
const project: PlatformProject = {
  projectId: "owned", deployments: [], backups: [backup],
  database: { status: "stopped", engine: "PostgreSQL 16.15", message: "Data retained." },
};
const operation = {
  id: "restore-owned", action: "restore", backupId: backup.id,
  stage: "checking", status: "running" as const, message: "Checking the restored database.",
  createdAt: "2026-10-06T01:00:00Z", updatedAt: "2026-10-06T01:00:01Z",
};
// The real platform runner catches task failures and renders their safe message.
const run = vi.fn(async (_label: string, task: () => Promise<unknown>) => { try { await task(); } catch { /* owned by parent UI */ } });
const onDeployments = vi.fn();
const props = { projectId: "owned", project, dockerReady: true, disabled: false, run, onDeployments };
beforeEach(() => {
  vi.clearAllMocks();
  Object.assign(access, { read: true, edit: true, deploy: true, secrets: true, loading: false, error: null, scope: "account-workspace" });
  vi.mocked(platformApi.restoreDatabase).mockResolvedValue(operation);
  vi.mocked(platformApi.recoverPreviousDatabase).mockResolvedValue({ ...operation, action: "rollback-restore" });
  vi.mocked(platformApi.importDatabaseBackup).mockResolvedValue({ ...backup, imported: true });
});
function chooseAndConfirm() {
  fireEvent.change(screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }), { target: { value: backup.id } });
  fireEvent.click(screen.getByRole("checkbox", { name: /Switch to this backup/ }));
}

it("requires a selected saved backup and deliberate confirmation, then submits it once", () => {
  render(<StandaloneDatabase {...props} />);
  const restore = screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement;
  expect(restore.disabled).toBe(true);
  fireEvent.change(screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }), { target: { value: backup.id } });
  expect(restore.disabled).toBe(true);
  fireEvent.click(screen.getByRole("checkbox", { name: /Switch to this backup/ }));
  expect(restore.disabled).toBe(false);
  fireEvent.click(restore); fireEvent.click(restore);
  expect(platformApi.restoreDatabase).toHaveBeenCalledExactlyOnceWith("owned", backup.id);
  expect(restore.disabled).toBe(true);
});

it("explains and blocks active apps or PostgreSQL, with a working deployment destination", () => {
  const deployment = { id: "app", projectId: "owned", provider: "docker" as const, commit: "a".repeat(40), configurationRevision: 0, status: "ready", message: "Healthy.", createdAt: "now", updatedAt: "now" };
  const ready = { ...project, database: { ...project.database, status: "ready" }, deployments: [deployment] };
  const view = render(<StandaloneDatabase {...props} project={ready} />);
  chooseAndConfirm();
  expect(screen.getByText("Use Stop PostgreSQL above, then restore your backup.")).not.toBeNull();
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Open deployments" }));
  expect(onDeployments).toHaveBeenCalledOnce();
  view.rerender(<StandaloneDatabase {...props} project={{ ...ready, database: project.database }} />);
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
  view.rerender(<StandaloneDatabase {...props} project={{ ...project, deployments: [{ ...deployment, status: "stopped" }] }} />);
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(false);
  expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
});

it("locks all database actions during actual restore and retains a useful failed retry", () => {
  const view = render(<StandaloneDatabase {...props} project={{ ...project, databaseOperation: operation }} />);
  expect((screen.getByRole("button", { name: "Start PostgreSQL" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }) as HTMLSelectElement).disabled).toBe(true);
  expect(screen.getByText("Checking the restored database.")).not.toBeNull();
  view.rerender(<StandaloneDatabase {...props} project={{ ...project, databaseOperation: { ...operation, status: "failed", stage: "failed", message: "The restored candidate failed its health check. Existing data is retained." } }} />);
  expect(screen.getByRole("alert").textContent).toContain("Existing data is retained.");
  chooseAndConfirm();
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(false);
});

it("connects verified recovery to application deployments and retains distinct failed and successful history", () => {
  const succeeded = { ...operation, status: "succeeded" as const, stage: "complete", message: "Restored database passed its health check." };
  render(<StandaloneDatabase {...props} project={{ ...project, database: { ...project.database, status: "ready" }, databaseOperation: succeeded,
    databaseRestoreHistory: [succeeded, { ...operation, id: "previous-failed", status: "failed", stage: "failed", message: "An earlier candidate failed; its data remains retained." }] }} />);
  fireEvent.click(screen.getByRole("button", { name: "Open application deployments" }));
  expect(onDeployments).toHaveBeenCalledOnce();
  expect(screen.getAllByText(succeeded.message).length).toBeGreaterThan(0);
  expect(screen.getByRole("list", { name: "PostgreSQL restore history" }).textContent).toContain("succeeded");
  expect(screen.getByRole("list", { name: "PostgreSQL restore history" }).textContent).toContain("failed");
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
});

it.each(["restore", "rollback-restore"])("keeps the %s success destination read-only and pauses it while access is uncertain", action => {
  access.edit = false;
  const succeeded = { ...operation, action, status: "succeeded" as const, stage: "complete", message: "The selected database is verified. Redeploy applications to use it." };
  const recovery = { ...project, databaseOperation: succeeded };
  const view = render(<StandaloneDatabase {...props} project={recovery} />);
  const destination = screen.getByRole("button", { name: "Open application deployments" }) as HTMLButtonElement;
  expect(destination.disabled).toBe(false);
  expect(screen.getByText(succeeded.message)).not.toBeNull();
  fireEvent.click(destination); expect(onDeployments).toHaveBeenCalledOnce();
  expect(platformApi.database).not.toHaveBeenCalled(); expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
  expect(platformApi.recoverPreviousDatabase).not.toHaveBeenCalled();
  access.loading = true; access.deploy = false;
  view.rerender(<StandaloneDatabase {...props} project={recovery} />);
  expect(screen.getByRole("button", { name: "Open application deployments" })).toBe(destination);
  expect(destination.disabled).toBe(true); fireEvent.click(destination); expect(onDeployments).toHaveBeenCalledOnce();
  access.loading = false;
  view.rerender(<StandaloneDatabase {...props} project={recovery} />);
  expect(screen.queryByRole("button", { name: "Open application deployments" })).toBeNull();
});

it("clears consent when the selected backup changes and refuses disappeared metadata", () => {
  const view = render(<StandaloneDatabase {...props} />);
  chooseAndConfirm();
  view.rerender(<StandaloneDatabase {...props} project={{ ...project, backups: [{ ...backup, sha256: "b".repeat(64) }] }} />);
  expect((screen.getByRole("checkbox", { name: /Switch to this backup/ }) as HTMLInputElement).checked).toBe(false);
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
  view.rerender(<StandaloneDatabase {...props} project={{ ...project, backups: [] }} />);
  expect(screen.queryByRole("form", { name: "Restore PostgreSQL backup" })).toBeNull();
  expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
});

it("keeps recovery unavailable when Docker is offline without losing the saved backup", () => {
  render(<StandaloneDatabase {...props} dockerReady={false} />);
  chooseAndConfirm();
  expect(screen.getByText("Start Docker to restore this database.")).not.toBeNull();
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole("list", { name: "PostgreSQL backups" })).not.toBeNull();
});

it("offers an explicit stop when failed cleanup cannot verify the database state", () => {
  render(<StandaloneDatabase {...props} project={{ ...project, database: { ...project.database, status: "unknown" },
    databaseOperation: { ...operation, status: "failed", stage: "failed", message: "PostgreSQL stop could not be confirmed. Existing data is retained." } }} />);
  expect((screen.getByRole("button", { name: "Stop PostgreSQL" }) as HTMLButtonElement).disabled).toBe(false);
  expect(screen.getByText("Use Stop PostgreSQL above to verify it is stopped before retrying recovery.")).not.toBeNull();
  chooseAndConfirm();
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Stop PostgreSQL" }));
  expect(platformApi.database).toHaveBeenCalledExactlyOnceWith("owned", "stop");
});

it("recovers only the history-bound previous database after a separate explicit confirmation", () => {
  const restored = { ...project, database: { ...project.database, database: "luci_restored_owned" },
    databaseRestoreHistory: [{ ...operation, status: "succeeded" as const, stage: "complete", previousDatabase: "app", candidateDatabase: "luci_restored_owned" }] };
  const view = render(<StandaloneDatabase {...props} project={restored} />);
  const button = screen.getByRole("button", { name: "Use previous database" }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
  fireEvent.click(screen.getByRole("checkbox", { name: /Switch to the verified previous database/ }));
  expect(button.disabled).toBe(false);
  fireEvent.click(button); fireEvent.click(button);
  expect(platformApi.recoverPreviousDatabase).toHaveBeenCalledExactlyOnceWith("owned", operation.id);
  expect(button.disabled).toBe(true);
  view.rerender(<StandaloneDatabase {...props} project={{ ...restored, database: { ...restored.database, database: "different-current" } }} />);
  expect(screen.queryByRole("button", { name: "Use previous database" })).toBeNull();
});

it("does not offer recovery from failed, unbound or same-database history", () => {
  render(<StandaloneDatabase {...props} project={{ ...project, database: { ...project.database, database: "app" },
    databaseRestoreHistory: [
      { ...operation, status: "failed", previousDatabase: "previous", candidateDatabase: "app" },
      { ...operation, id: "unbound", status: "succeeded", previousDatabase: "previous", candidateDatabase: "other" },
      { ...operation, id: "same", status: "succeeded", previousDatabase: "app", candidateDatabase: "app" },
    ] }} />);
  expect(screen.queryByRole("button", { name: "Use previous database" })).toBeNull();
  expect(platformApi.recoverPreviousDatabase).not.toHaveBeenCalled();
});

it("imports only an explicitly trusted file without starting or restoring PostgreSQL", async () => {
  render(<StandaloneDatabase {...props} dockerReady={false} />);
  const button = screen.getByRole("button", { name: "Import PostgreSQL backup" }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
  const file = new File(["PGDMP-fixture"], "saved.dump");
  fireEvent.change(screen.getByLabelText("PostgreSQL dump file"), { target: { files: [file] } });
  expect(button.disabled).toBe(true);
  fireEvent.click(screen.getByRole("checkbox", { name: "I trust this dump and the SQL it can execute during restore." }));
  fireEvent.click(button); fireEvent.click(button);
  await waitFor(() => expect(platformApi.importDatabaseBackup).toHaveBeenCalledExactlyOnceWith("owned", file, true, expect.any(AbortSignal), expect.any(Function)));
  expect(platformApi.database).not.toHaveBeenCalled();
  expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
  expect((screen.getByRole("checkbox", { name: /Switch to this backup/ }) as HTMLInputElement).checked).toBe(false);
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/A dump can contain SQL supplied by its author/)).not.toBeNull();
});

it("clears file trust when the selection or project changes and cancels pending import", async () => {
  const view = render(<StandaloneDatabase {...props} />);
  const choose = (name: string) => fireEvent.change(screen.getByLabelText("PostgreSQL dump file"), { target: { files: [new File(["PGDMP-fixture"], name)] } });
  const trust = () => screen.getByRole("checkbox", { name: "I trust this dump and the SQL it can execute during restore." }) as HTMLInputElement;
  choose("first.dump"); fireEvent.click(trust()); choose("second.dump");
  expect(trust().checked).toBe(false);
  let finish!: (value: typeof backup) => void;
  vi.mocked(platformApi.importDatabaseBackup).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  fireEvent.click(trust()); fireEvent.click(screen.getByRole("button", { name: "Import PostgreSQL backup" }));
  const signal = vi.mocked(platformApi.importDatabaseBackup).mock.calls[0][3]!;
  view.rerender(<StandaloneDatabase {...props} projectId="other" project={{ ...project, projectId: "other" }} />);
  expect(signal.aborted).toBe(true);
  expect(trust().checked).toBe(false);
  finish(backup);
  await waitFor(() => expect((screen.getByRole("button", { name: "Import PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(true));
  expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
});

it.each(["read", "deploy", "secrets"] as const)("requires %s access before revealing private backups or offering recovery", permission => {
  access[permission] = false;
  render(<StandaloneDatabase {...props} />);
  expect(screen.queryByText("Data retained.")).toBeNull();
  expect(screen.queryByRole("list", { name: "PostgreSQL backups" })).toBeNull();
  expect(screen.queryByLabelText("PostgreSQL dump file")).toBeNull();
  expect(screen.getByRole("status").textContent).toMatch(/owner|access/i);
  expect(platformApi.database).not.toHaveBeenCalled(); expect(platformApi.downloadBackup).not.toHaveBeenCalled();
});

it("lets authorized readers download a backup while preventing all database mutations", async () => {
  access.edit = false;
  render(<StandaloneDatabase {...props} />);
  expect(screen.getByRole("list", { name: "PostgreSQL backups" })).not.toBeNull();
  expect(screen.getByRole("status").textContent).toMatch(/edit/i);
  const download = screen.getByRole("button", { name: /^Download database backup/ });
  expect((download as HTMLButtonElement).disabled).toBe(false);
  fireEvent.click(download);
  expect(platformApi.downloadBackup).toHaveBeenCalledExactlyOnceWith("owned", backup, expect.any(Function));
  for (const name of ["Start PostgreSQL", "Stop PostgreSQL", "Create database backup", "Import PostgreSQL backup", "Restore PostgreSQL backup"]) {
    const button = screen.getByRole("button", { name }) as HTMLButtonElement;
    expect(button.disabled).toBe(true); fireEvent.click(button);
  }
  fireEvent.submit(screen.getByRole("form", { name: "Import PostgreSQL backup" }));
  fireEvent.submit(screen.getByRole("form", { name: "Restore PostgreSQL backup" }));
  expect(platformApi.database).not.toHaveBeenCalled(); expect(platformApi.importDatabaseBackup).not.toHaveBeenCalled();
  expect(platformApi.restoreDatabase).not.toHaveBeenCalled(); expect(platformApi.recoverPreviousDatabase).not.toHaveBeenCalled();
});

it("preserves restore and upload drafts through a routine permission refresh while disabling actions", () => {
  const view = render(<StandaloneDatabase {...props} />);
  chooseAndConfirm();
  const file = new File(["PGDMP-fixture"], "saved.dump");
  const input = screen.getByLabelText("PostgreSQL dump file") as HTMLInputElement;
  fireEvent.change(input, { target: { files: [file] } });
  const trust = screen.getByRole("checkbox", { name: /I trust this dump/ }) as HTMLInputElement;
  fireEvent.click(trust);
  const form = screen.getByRole("form", { name: "Restore PostgreSQL backup" });
  access.loading = true;
  view.rerender(<StandaloneDatabase {...props} />);
  expect(screen.getByRole("form", { name: "Restore PostgreSQL backup" })).toBe(form);
  expect(screen.getByLabelText("PostgreSQL dump file")).toBe(input);
  expect(input.files?.[0]).toBe(file); expect(trust.checked).toBe(true);
  expect((screen.getByRole("checkbox", { name: /Switch to this backup/ }) as HTMLInputElement).checked).toBe(true);
  for (const name of ["Import PostgreSQL backup", "Restore PostgreSQL backup"]) expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: /^Download database backup/ }) as HTMLButtonElement).disabled).toBe(true);
  access.loading = false; view.rerender(<StandaloneDatabase {...props} />);
  expect(input.files?.[0]).toBe(file); expect(trust.checked).toBe(true);
  expect((screen.getByRole("button", { name: "Import PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(false);
  expect((screen.getByRole("button", { name: "Restore PostgreSQL backup" }) as HTMLButtonElement).disabled).toBe(false);
  expect(platformApi.importDatabaseBackup).not.toHaveBeenCalled(); expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
});

it("clears private selections on confirmed read revocation and does not restore prior consent on regrant", () => {
  const view = render(<StandaloneDatabase {...props} />);
  chooseAndConfirm();
  fireEvent.change(screen.getByLabelText("PostgreSQL dump file"), { target: { files: [new File(["PGDMP"], "private.dump")] } });
  fireEvent.click(screen.getByRole("checkbox", { name: /I trust this dump/ }));
  access.secrets = false; view.rerender(<StandaloneDatabase {...props} />);
  expect(screen.queryByRole("list", { name: "PostgreSQL backups" })).toBeNull();
  expect(screen.queryByRole("form", { name: "Restore PostgreSQL backup" })).toBeNull();
  access.secrets = true; view.rerender(<StandaloneDatabase {...props} />);
  expect((screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }) as HTMLSelectElement).value).toBe("");
  expect((screen.getByLabelText("PostgreSQL dump file") as HTMLInputElement).value).toBe("");
  expect((screen.getByRole("checkbox", { name: /I trust this dump/ }) as HTMLInputElement).checked).toBe(false);
  expect((screen.getByRole("checkbox", { name: /Switch to this backup/ }) as HTMLInputElement).checked).toBe(false);
});

it("refuses a deferred mutation after editing access is revoked, even when later granted again", async () => {
  let task!: () => Promise<unknown>;
  const deferredRun = vi.fn(async (_label: string, deferred: () => Promise<unknown>) => { task = deferred; });
  const view = render(<StandaloneDatabase {...props} run={deferredRun} />);
  fireEvent.click(screen.getByRole("button", { name: "Start PostgreSQL" }));
  access.edit = false; view.rerender(<StandaloneDatabase {...props} run={deferredRun} />);
  access.edit = true; view.rerender(<StandaloneDatabase {...props} run={deferredRun} />);
  await expect(task()).rejects.toThrow("Database access changed");
  expect(platformApi.database).not.toHaveBeenCalled();
});

it("aborts an in-flight upload on edit revocation and ignores its late receipt after regrant", async () => {
  let finish!: (value: typeof backup) => void;
  vi.mocked(platformApi.importDatabaseBackup).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<StandaloneDatabase {...props} />);
  fireEvent.change(screen.getByLabelText("PostgreSQL dump file"), { target: { files: [new File(["PGDMP-fixture"], "private.dump")] } });
  fireEvent.click(screen.getByRole("checkbox", { name: /I trust this dump/ }));
  fireEvent.click(screen.getByRole("button", { name: "Import PostgreSQL backup" }));
  const signal = vi.mocked(platformApi.importDatabaseBackup).mock.calls[0][3]!;
  access.edit = false; view.rerender(<StandaloneDatabase {...props} />);
  expect(signal.aborted).toBe(true);
  expect(screen.getByRole("list", { name: "PostgreSQL backups" })).not.toBeNull();
  access.edit = true; view.rerender(<StandaloneDatabase {...props} />);
  await act(async () => finish(backup));
  expect((screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }) as HTMLSelectElement).value).toBe("");
  expect((screen.getByRole("checkbox", { name: /I trust this dump/ }) as HTMLInputElement).checked).toBe(false);
  expect(platformApi.restoreDatabase).not.toHaveBeenCalled();
});

it("passes an artifact guard that survives loading but closes permanently on grant loss", () => {
  const view = render(<StandaloneDatabase {...props} />);
  fireEvent.click(screen.getByRole("button", { name: /^Download database backup/ }));
  const current = vi.mocked(platformApi.downloadBackup).mock.calls[0][2]!;
  expect(current()).toBe(true);
  access.loading = true; view.rerender(<StandaloneDatabase {...props} />);
  expect(current()).toBe(true);
  access.loading = false; access.secrets = false; view.rerender(<StandaloneDatabase {...props} />);
  expect(current()).toBe(false);
  access.secrets = true; view.rerender(<StandaloneDatabase {...props} />);
  expect(current()).toBe(false);
});

it("clears selected backups across account/workspace changes and fences downloads after unmount", () => {
  const view = render(<StandaloneDatabase {...props} />);
  chooseAndConfirm();
  fireEvent.click(screen.getByRole("button", { name: /^Download database backup/ }));
  const previous = vi.mocked(platformApi.downloadBackup).mock.calls[0][2]!;
  access.scope = "different-account-workspace"; view.rerender(<StandaloneDatabase {...props} />);
  expect(previous()).toBe(false);
  expect((screen.getByRole("combobox", { name: "PostgreSQL backup to restore" }) as HTMLSelectElement).value).toBe("");
  fireEvent.click(screen.getByRole("button", { name: /^Download database backup/ }));
  const current = vi.mocked(platformApi.downloadBackup).mock.calls[1][2]!;
  view.unmount();
  expect(current()).toBe(false);
});
