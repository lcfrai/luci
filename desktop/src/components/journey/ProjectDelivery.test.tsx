import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StrictMode } from "react";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { productApi, type GitDelivery, type GitPublication, type GitState } from "../../api/product";
import { ApiError } from "../../api/client";
import { resetCockpit, useCockpit } from "../../store/store";
import { aProject } from "../../test/fixtures";
import { ProjectDelivery } from "./ProjectDelivery";

const identity = vi.hoisted(() => ({ scope: "account-a:workspace-a", allowPublic: true }));
const grants = vi.hoisted(() => ({
  project: { read: true, edit: true, deploy: true, secrets: true, loading: false, error: "" },
  workspace: { read: true, edit: true, deploy: true, secrets: true, loading: false, error: "" },
}));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: (id: string) => {
  const value = id ? grants.project : grants.workspace;
  return { ...value, canRead: value.read, canEdit: value.read && value.edit && !value.loading && !value.error,
    permissions: { ...value }, message: value.loading ? "Checking project permissions…" : "" };
} }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (id: string) => `${identity.scope}:${id}` }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { policy: { allowPublicPublishing: identity.allowPublic } } }) }));

const head = "a".repeat(40), origin = "https://github.com/owner/existing.git";
const git: GitState = { branch: "main", clean: true, changes: [], diff: "", log: [{ hash: head, subject: "Reviewed work" }] };
const stored: GitDelivery = { branch: "main", head, origin };
const connected: GitPublication = { repository: "https://github.com/owner/new.git", status: "connected", verified: false, updatedAt: "" };
function deferred<T>() {
  let resolve!: (value: T) => void, reject!: (cause: Error) => void;
  return { promise: new Promise<T>((done, fail) => { resolve = done; reject = fail; }), resolve, reject };
}
const remote = () => screen.getByLabelText("Repository origin · HTTPS") as HTMLInputElement;
const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;
const editRemote = (value: string) => fireEvent.change(remote(), { target: { value } });
const submitRemote = () => fireEvent.submit(remote().closest("form")!);
async function ready() { await screen.findByText(origin); }

beforeEach(() => {
  resetCockpit();
  useCockpit.setState({ projects: [aProject({ id: "p-1", name: "journal" }), aProject({ id: "p-2", name: "other" })], selection: { projectId: "p-1", branchId: null, worktreeId: null, sessionId: null }, lens: "merge" });
  identity.scope = "account-a:workspace-a"; identity.allowPublic = true;
  for (const value of Object.values(grants)) Object.assign(value, { read: true, edit: true, deploy: true, secrets: true, loading: false, error: "" });
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue(stored);
  vi.spyOn(productApi, "gitRemote").mockResolvedValue(connected);
  vi.spyOn(productApi, "githubCreate").mockResolvedValue(connected);
  vi.spyOn(productApi, "gitPush").mockResolvedValue({ ...connected, status: "pushed", verified: true, commit: head });
  vi.spyOn(productApi, "githubCheck").mockResolvedValue({ available: true, signedIn: true, login: "owner", message: "Signed in" });
  vi.spyOn(productApi, "githubLogin").mockResolvedValue({ id: "job-1", status: "waiting", message: "Continue in GitHub", userCode: "PRIVATE-CODE", verificationUrl: "https://github.com/login/device" } as Awaited<ReturnType<typeof productApi.githubLogin>>);
  vi.spyOn(productApi, "githubLogins").mockResolvedValue({ jobs: [] });
});
afterEach(() => { cleanup(); vi.useRealTimers(); vi.restoreAllMocks(); });

it("preserves a remote typed while the initial connection read is pending", async () => {
  const pending = deferred<GitDelivery>(); vi.mocked(productApi.gitDelivery).mockReturnValueOnce(pending.promise);
  render(<ProjectDelivery projectId="p-1" git={git} />);
  editRemote("https://github.com/owner/draft.git");
  await act(async () => { pending.resolve(stored); });
  expect(remote().value).toBe("https://github.com/owner/draft.git");
  expect(screen.getByText(origin)).not.toBeNull();
});

it("keeps repository name and origin drafts when a new reviewed head arrives", async () => {
  vi.mocked(productApi.gitDelivery).mockResolvedValue({ branch: "main", head });
  const view = render(<ProjectDelivery projectId="p-1" git={git} />);
  await act(async () => {});
  fireEvent.click(screen.getByText("Create a GitHub repository"));
  fireEvent.change(screen.getByLabelText("Repository name"), { target: { value: "my-release" } });
  editRemote("https://github.com/owner/draft.git");
  view.rerender(<ProjectDelivery projectId="p-1" git={{ ...git, log: [{ hash: "b".repeat(40), subject: "New work" }] }} />);
  await act(async () => {});
  expect((screen.getByLabelText("Repository name") as HTMLInputElement).value).toBe("my-release");
  expect(remote().value).toBe("https://github.com/owner/draft.git");
});

it("does not let an older head read replace the latest repository connection", async () => {
  const old = deferred<GitDelivery>(), latest = deferred<GitDelivery>();
  vi.mocked(productApi.gitDelivery).mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />);
  view.rerender(<ProjectDelivery projectId="p-1" git={{ ...git, log: [{ hash: "b".repeat(40), subject: "New work" }] }} />);
  await act(async () => { latest.resolve({ ...stored, origin: connected.repository }); });
  await act(async () => { old.resolve(stored); });
  expect(remote().value).toBe(connected.repository);
  expect(screen.queryByText(origin)).toBeNull();
});

it.each(["edit", "deploy", "loading"] as const)("blocks delivery mutations when project %s access is unavailable", async (reason) => {
  if (reason === "loading") grants.project.loading = true; else grants.project[reason] = false;
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  expect(button("Connect existing repository").disabled).toBe(true);
  expect(button("Push reviewed commit").disabled).toBe(true);
  submitRemote(); fireEvent.click(button("Push reviewed commit"));
  expect(productApi.gitRemote).not.toHaveBeenCalled(); expect(productApi.gitPush).not.toHaveBeenCalled();
});

it("uses workspace secret grants for GitHub account controls independently of project deployment", async () => {
  grants.workspace.secrets = false;
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  expect(button("Check GitHub account").disabled).toBe(true);
  expect(button("Sign in to GitHub").disabled).toBe(true);
  expect(button("Connect existing repository").disabled).toBe(false);
  fireEvent.click(button("Check GitHub account")); fireEvent.click(button("Sign in to GitHub"));
  expect(productApi.githubCheck).not.toHaveBeenCalled(); expect(productApi.githubLogin).not.toHaveBeenCalled();
});

it("protects an unsaved repository draft on navigation without storing its URL", async () => {
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  const draft = "https://private-user:private-draft-secret@github.com/owner/draft.git";
  editRemote(draft);
  act(() => useCockpit.getState().selectProject("p-2"));
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "project", projectId: "p-2" });
  expect(useCockpit.getState().selection.projectId).toBe("p-1");
  expect(JSON.stringify(useCockpit.getState().unsavedProjectDrafts)).not.toContain("private-draft-secret");
  expect(JSON.stringify(localStorage)).not.toContain("private-draft-secret");
  act(() => useCockpit.getState().cancelNavigation()); expect(remote().value).toBe(draft);
  act(() => { useCockpit.getState().selectProject("p-2"); useCockpit.getState().discardDraftsAndNavigate(); });
  expect(useCockpit.getState().selection.projectId).toBe("p-2");
});

it("retains confirmed creation when its connection readback fails instead of offering another create", async () => {
  vi.mocked(productApi.gitDelivery).mockResolvedValueOnce({ branch: "main", head }).mockRejectedValueOnce(new Error("Connection readback failed"));
  render(<ProjectDelivery projectId="p-1" git={git} />); await act(async () => {});
  fireEvent.click(screen.getByText("Create a GitHub repository"));
  fireEvent.submit(button("Create repository").closest("form")!);
  await screen.findByRole("alert");
  expect(screen.getByText(/repository connected\. Push your reviewed commit next/)).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Create repository" })).toBeNull();
  expect(button("Push reviewed commit").disabled).toBe(true);
  expect(productApi.githubCreate).toHaveBeenCalledTimes(1);
});

it("preserves a newer remote draft when the submitted connection is confirmed", async () => {
  const saving = deferred<GitPublication>(); vi.mocked(productApi.gitRemote).mockReturnValueOnce(saving.promise);
  vi.mocked(productApi.gitDelivery).mockResolvedValueOnce(stored).mockResolvedValue({ ...stored, origin: connected.repository });
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  editRemote(connected.repository); submitRemote();
  editRemote("https://github.com/owner/next-draft.git");
  await act(async () => { saving.resolve(connected); });
  await waitFor(() => expect(productApi.gitDelivery).toHaveBeenCalledTimes(2));
  expect(remote().value).toBe("https://github.com/owner/next-draft.git");
  expect(screen.getByText(connected.repository)).not.toBeNull();
});

it.each(["push", "create"] as const)("does not treat an existing remote draft as submitted by %s", async (kind) => {
  if (kind === "create") vi.mocked(productApi.gitDelivery).mockResolvedValueOnce({ branch: "main", head }).mockResolvedValue({ ...stored, origin: connected.repository });
  else vi.mocked(productApi.gitPush).mockResolvedValue({ ...connected, repository: origin, status: "pushed", verified: true, commit: head });
  render(<ProjectDelivery projectId="p-1" git={git} />); await act(async () => {});
  editRemote("https://github.com/owner/not-submitted.git");
  if (kind === "create") {
    fireEvent.click(screen.getByText("Create a GitHub repository"));
    fireEvent.submit(button("Create repository").closest("form")!);
  } else fireEvent.click(button("Push reviewed commit"));
  await waitFor(() => expect(productApi.gitDelivery).toHaveBeenCalledTimes(2));
  expect(remote().value).toBe("https://github.com/owner/not-submitted.git");
  expect(productApi.gitRemote).not.toHaveBeenCalled();
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
});

it("fences the first StrictMode read when its replacement finishes sooner", async () => {
  const abandoned = deferred<GitDelivery>(), latest = deferred<GitDelivery>();
  vi.mocked(productApi.gitDelivery).mockReturnValueOnce(abandoned.promise).mockReturnValueOnce(latest.promise);
  render(<StrictMode><ProjectDelivery projectId="p-1" git={git} /></StrictMode>);
  expect(productApi.gitDelivery).toHaveBeenCalledTimes(2);
  await act(async () => { latest.resolve(stored); });
  await act(async () => { abandoned.resolve({ ...stored, origin: "https://github.com/abandoned/repository.git" }); });
  expect(remote().value).toBe(origin);
  expect(screen.queryByText(/abandoned\/repository/)).toBeNull();
});

it("requires an explicit successful read after an uncertain mutation, preserving its draft through a failed read", async () => {
  vi.mocked(productApi.gitRemote).mockRejectedValueOnce(new Error("The save response was interrupted"));
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  editRemote(connected.repository); submitRemote();
  await screen.findByText("The save response was interrupted");
  expect(button("Connect existing repository").disabled).toBe(true);
  submitRemote(); expect(productApi.gitRemote).toHaveBeenCalledTimes(1);
  vi.mocked(productApi.gitDelivery).mockRejectedValueOnce(new Error("Refresh unavailable"));
  fireEvent.click(button("Refresh repository connection")); await screen.findByText("Refresh unavailable");
  expect(remote().value).toBe(connected.repository);
  expect(button("Connect existing repository").disabled).toBe(true);
  fireEvent.click(button("Refresh repository connection"));
  await waitFor(() => expect(button("Connect existing repository").disabled).toBe(false));
  expect(remote().value).toBe(connected.repository);
});

it("keeps a confirmed push visible when readback fails and prevents immediate replay", async () => {
  vi.mocked(productApi.gitPush).mockResolvedValue({ ...connected, repository: origin, status: "pushed", commit: head, verified: true });
  vi.mocked(productApi.gitDelivery).mockResolvedValueOnce(stored).mockRejectedValueOnce(new ApiError("network_unavailable", "Luci’s local service is not available. Start the service and retry.", 0));
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  fireEvent.click(button("Push reviewed commit")); await screen.findByText("The latest repository status could not be read. Refresh the repository connection to try again.");
  expect(screen.queryByText(/Start the service and retry/)).toBeNull();
  expect(screen.getByText("Remote commit verified: aaaaaaaa.")).not.toBeNull();
  expect(button("Push reviewed commit").disabled).toBe(true);
  fireEvent.click(button("Push reviewed commit")); expect(productApi.gitPush).toHaveBeenCalledTimes(1);
});

it("invalidates a read begun before mutation confirmation even if it finishes after readback", async () => {
  const saving = deferred<GitPublication>(), beforeConfirmation = deferred<GitDelivery>(), readback = deferred<GitDelivery>();
  vi.mocked(productApi.gitRemote).mockReturnValueOnce(saving.promise);
  vi.mocked(productApi.gitDelivery).mockResolvedValueOnce(stored).mockReturnValueOnce(beforeConfirmation.promise).mockReturnValueOnce(readback.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  editRemote(connected.repository); submitRemote();
  view.rerender(<ProjectDelivery projectId="p-1" git={{ ...git, log: [{ hash: "b".repeat(40), subject: "New head" }] }} />);
  await act(async () => { saving.resolve(connected); });
  await act(async () => { readback.resolve({ ...stored, origin: connected.repository }); });
  await act(async () => { beforeConfirmation.resolve(stored); });
  expect(remote().value).toBe(connected.repository);
  expect(screen.queryByText(origin)).toBeNull();
  expect(button("Connect existing repository").disabled).toBe(false);
});

it("discards an in-flight publication after deploy access is revoked and regranted", async () => {
  const saving = deferred<GitPublication>(); vi.mocked(productApi.gitRemote).mockReturnValueOnce(saving.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  editRemote(connected.repository); submitRemote();
  grants.project.deploy = false; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  expect(remote().disabled).toBe(true); expect(remote().value).toBe(connected.repository);
  grants.project.deploy = true; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  await act(async () => { saving.resolve(connected); });
  expect(productApi.gitDelivery).toHaveBeenCalledTimes(1);
  expect(screen.queryByText(/repository connected\. Push/)).toBeNull();
  expect(button("Connect existing repository").disabled).toBe(true);
  fireEvent.click(button("Refresh repository connection"));
  await waitFor(() => expect(button("Connect existing repository").disabled).toBe(false));
  expect(remote().value).toBe(connected.repository);
});

it("keeps the physical publication lock across revocation until the original write settles", async () => {
  const saving = deferred<GitPublication>(); vi.mocked(productApi.gitRemote).mockReturnValueOnce(saving.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  editRemote(connected.repository); submitRemote();
  grants.project.deploy = false; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  grants.project.deploy = true; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  // A current read can still see the old origin while the original write runs.
  fireEvent.click(button("Refresh repository connection")); await act(async () => {});
  expect(button("Connect existing repository").disabled).toBe(true);
  submitRemote(); expect(productApi.gitRemote).toHaveBeenCalledTimes(1);
  expect(productApi.gitDelivery).toHaveBeenCalledTimes(1);
  await act(async () => { saving.resolve(connected); });
  expect(button("Connect existing repository").disabled).toBe(true);
  fireEvent.click(button("Refresh repository connection"));
  await waitFor(() => expect(button("Connect existing repository").disabled).toBe(false));
  expect(remote().value).toBe(connected.repository);
});

it("hides private GitHub state and stops polling when workspace secrets are revoked", async () => {
  const poll = deferred<Awaited<ReturnType<typeof productApi.githubLogins>>>();
  vi.mocked(productApi.githubLogins).mockReturnValueOnce(poll.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  vi.useFakeTimers();
  await act(async () => { fireEvent.click(button("Sign in to GitHub")); });
  expect(screen.getByText("PRIVATE-CODE")).not.toBeNull();
  await act(async () => { vi.advanceTimersByTime(2000); });
  expect(productApi.githubLogins).toHaveBeenCalledTimes(1);
  grants.workspace.secrets = false; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  expect(screen.queryByText("PRIVATE-CODE")).toBeNull();
  expect(button("Sign in to GitHub").disabled).toBe(true);
  grants.workspace.secrets = true; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  await act(async () => { poll.resolve({ jobs: [{ id: "job-1", status: "waiting", userCode: "LATE-PRIVATE-CODE" } as Awaited<ReturnType<typeof productApi.githubLogin>>] }); vi.advanceTimersByTime(5000); });
  expect(screen.queryByText("LATE-PRIVATE-CODE")).toBeNull();
  expect(productApi.githubLogins).toHaveBeenCalledTimes(1);
  expect(button("Sign in to GitHub").disabled).toBe(false);
});

it("ignores a pending GitHub sign-in after workspace editing is revoked and regranted", async () => {
  const login = deferred<Awaited<ReturnType<typeof productApi.githubLogin>>>();
  vi.mocked(productApi.githubLogin).mockReturnValueOnce(login.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  fireEvent.click(button("Sign in to GitHub"));
  grants.workspace.edit = false; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  grants.workspace.edit = true; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  expect(button("Sign in to GitHub").disabled).toBe(true);
  fireEvent.click(button("Sign in to GitHub")); expect(productApi.githubLogin).toHaveBeenCalledTimes(1);
  await act(async () => { login.resolve({ id: "old-login", status: "waiting", userCode: "STALE-PRIVATE-CODE" } as Awaited<ReturnType<typeof productApi.githubLogin>>); });
  expect(screen.queryByText("STALE-PRIVATE-CODE")).toBeNull();
  expect(button("Sign in to GitHub").disabled).toBe(false);
});

it("hides the repository on read revocation and fences reads across a regrant", async () => {
  const old = deferred<GitDelivery>();
  vi.mocked(productApi.gitDelivery).mockReturnValueOnce(old.promise).mockResolvedValue({ ...stored, origin: connected.repository });
  const view = render(<ProjectDelivery projectId="p-1" git={git} />);
  grants.project.read = false; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  expect(screen.queryByLabelText("Repository origin · HTTPS")).toBeNull();
  grants.project.read = true; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  await screen.findByText(connected.repository);
  await act(async () => { old.resolve(stored); });
  expect(remote().value).toBe(connected.repository); expect(screen.queryByText(origin)).toBeNull();
});

it.each(["project", "account", "workspace"] as const)("isolates drafts and pending publication when the %s changes", async (changed) => {
  const saving = deferred<GitPublication>(); vi.mocked(productApi.gitRemote).mockReturnValueOnce(saving.promise);
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  editRemote("https://github.com/previous/private-draft.git"); submitRemote();
  vi.mocked(productApi.gitDelivery).mockResolvedValue({ ...stored, origin: "https://github.com/current/repository.git" });
  if (changed !== "project") identity.scope = changed === "account" ? "account-b:workspace-a" : "account-a:workspace-b";
  view.rerender(<ProjectDelivery projectId={changed === "project" ? "p-2" : "p-1"} git={git} />);
  await screen.findByText("https://github.com/current/repository.git");
  await act(async () => { saving.resolve(connected); });
  expect(remote().value).toBe("https://github.com/current/repository.git");
  expect(screen.queryByText(/repository connected\. Push/)).toBeNull();
  expect(productApi.gitDelivery).toHaveBeenCalledTimes(2);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(0);
});

it("guards forced create submission and resets public consent when its destination changes", async () => {
  vi.mocked(productApi.gitDelivery).mockResolvedValue({ branch: "main", head });
  const view = render(<ProjectDelivery projectId="p-1" git={git} />); await act(async () => {});
  fireEvent.click(screen.getByText("Create a GitHub repository"));
  fireEvent.change(screen.getByLabelText("Repository visibility"), { target: { value: "public" } });
  fireEvent.submit(button("Create repository").closest("form")!);
  expect(productApi.githubCreate).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("checkbox")); expect(button("Create repository").disabled).toBe(false);
  fireEvent.change(screen.getByLabelText("GitHub owner · optional"), { target: { value: "new-owner" } });
  expect((screen.getByRole("checkbox") as HTMLInputElement).checked).toBe(false);
  grants.project.deploy = false; view.rerender(<ProjectDelivery projectId="p-1" git={git} />);
  fireEvent.change(screen.getByLabelText("Repository visibility"), { target: { value: "private" } });
  fireEvent.submit(button("Create repository").closest("form")!);
  expect(productApi.githubCreate).not.toHaveBeenCalled();
});

it("clears only an accepted remote draft and does not sign in or publish on mount", async () => {
  vi.mocked(productApi.gitDelivery).mockResolvedValueOnce(stored).mockResolvedValue({ ...stored, origin: connected.repository });
  render(<ProjectDelivery projectId="p-1" git={git} />); await ready();
  expect(productApi.githubCheck).not.toHaveBeenCalled(); expect(productApi.githubLogin).not.toHaveBeenCalled();
  expect(productApi.githubCreate).not.toHaveBeenCalled(); expect(productApi.gitPush).not.toHaveBeenCalled();
  editRemote(connected.repository); submitRemote();
  await waitFor(() => expect(button("Connect existing repository").disabled).toBe(false));
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(0);
  act(() => useCockpit.getState().selectProject("p-2"));
  expect(useCockpit.getState().pendingNavigation).toBeNull(); expect(useCockpit.getState().selection.projectId).toBe("p-2");
});
