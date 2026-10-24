import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CloudDeployForm } from "./CloudDeployForm";
import { DeploymentWorkspace } from "../lens/DeployLens";
import { productApi } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";

const identity = vi.hoisted(() => ({ actor: "alice", workspace: "local", root: "/projects/alice", edit: true }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: identity.actor }, permissions: { manageBacking: true } } }) }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ workspace: { mode: identity.workspace === "local" ? "local" : "cloud", connectionId: identity.workspace, host: identity.workspace }, status: { config: { projectsDir: identity.root }, access: { user: { username: identity.actor } } } }) }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: false, error: null, message: "", canRead: true, canEdit: identity.edit, canExecute: identity.edit, permissions: { read: true, edit: identity.edit, execute: identity.edit, deploy: identity.edit, secrets: identity.edit } }) }));
vi.mock("../platform/PlatformWorkspace", () => ({ PlatformWorkspace: () => <p>Application controls</p> }));
vi.mock("./DeploymentOverview", () => ({ DeploymentOverview: () => null }));
vi.mock("./ProjectConfiguration", () => ({ ProjectConfiguration: () => null }));

const target = { id: "target-one", name: "My server", baseUrl: "https://server.example.com", serverUuid: "server-one" };
const callbacks = () => ({ onDeploy: vi.fn(), onSetup: vi.fn() });
function draftRecords() { return Object.keys(localStorage).filter(key => key.startsWith("luci.project-draft.v1:")).map(key => localStorage.getItem(key)!); }
function edit(label: string, value: string) { fireEvent.change(screen.getByLabelText(label), { target: { value } }); }
async function readyForm() { await waitFor(() => expect((screen.getByLabelText("Application domain") as HTMLInputElement).disabled).toBe(false)); }

beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit();
  Object.assign(identity, { actor: "alice", workspace: "local", root: "/projects/alice", edit: true });
  useCockpit.setState(state => ({ lens: "deploy", selection: { ...state.selection, projectId: "p-1" } }));
  vi.spyOn(productApi, "targets").mockResolvedValue({ targets: [target] });
  vi.spyOn(productApi, "git").mockResolvedValue({ branch: "main", clean: true, changes: [], diff: "", log: [{ hash: "a".repeat(40), subject: "Saved source" }] });
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ branch: "main", head: "a".repeat(40), origin: "https://github.com/example/project" });
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  vi.spyOn(productApi, "cloudRelease");
  vi.spyOn(productApi, "prepareDeployKey");
});
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); });

it("recovers cloud settings across both Deploy tabs and navigation without restoring keys or deploying", async () => {
  const view = render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(screen.getByRole("tab", { name: "Previews and publishing" }));
  await userEvent.click(screen.getByRole("tab", { name: "Your cloud" }));
  await readyForm();
  await userEvent.selectOptions(screen.getByLabelText("Deployment server"), target.id);
  await userEvent.selectOptions(screen.getByLabelText("Repository access"), "private");
  expect(screen.getByLabelText("Repository access")).toHaveProperty("value", "private");
  edit("Application domain", "launch.example.com");
  await userEvent.click(screen.getByText("Build and health options", { selector: "summary" }));
  edit("Cloud Dockerfile", "deploy/Dockerfile"); edit("Cloud app port", "8080"); edit("Health check path", "/ready");
  fireEvent.change(screen.getByRole("textbox", { name: /^Deployment server key ID/ }), { target: { value: "ephemeral-server-key" } });
  expect(screen.getByRole("textbox", { name: /^Deployment server key ID/ })).toHaveProperty("value", "ephemeral-server-key");
  expect(draftRecords()).toHaveLength(1);
  expect(draftRecords()[0]).not.toContain("ephemeral-server-key");
  expect(JSON.parse(draftRecords()[0]).value).toMatchObject({ targetId: target.id, domain: "launch.example.com", port: 8080, healthPath: "/ready", dockerfile: "deploy/Dockerfile" });
  for (const next of ["GitHub Pages", "Application platform"]) {
    await userEvent.click(screen.getByRole("tab", { name: next }));
    if (next === "Application platform") await userEvent.click(screen.getByRole("tab", { name: "Previews and publishing" }));
    await userEvent.click(screen.getByRole("tab", { name: "Your cloud" }));
    await screen.findByRole("region", { name: "Cloud deployment draft recovery" });
    expect((screen.getByLabelText("Application domain") as HTMLInputElement).disabled).toBe(true);
    await waitFor(() => expect((screen.getByRole("button", { name: "Restore cloud deployment draft" }) as HTMLButtonElement).disabled).toBe(false));
    await userEvent.click(screen.getByRole("button", { name: "Restore cloud deployment draft" }));
    expect(screen.getByLabelText("Application domain")).toHaveProperty("value", "launch.example.com");
    expect(screen.getByLabelText("Cloud app port")).toHaveProperty("value", "8080");
    expect(screen.getByLabelText("Repository access")).toHaveProperty("value", "private");
    expect(screen.getByRole("textbox", { name: /^Deployment server key ID/ })).toHaveProperty("value", "");
  }
  act(() => useCockpit.getState().setLens("design"));
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "lens", lens: "design" });
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).find(row => row.label === "Cloud deployment settings")?.retained).toBe(true);
  act(() => useCockpit.getState().discardDraftsAndNavigate(true));
  view.unmount();
  render(<CloudDeployForm projectId="p-1" busy="" {...callbacks()} />);
  await screen.findByRole("button", { name: "Restore cloud deployment draft" });
  expect(productApi.cloudRelease).not.toHaveBeenCalled(); expect(productApi.prepareDeployKey).not.toHaveBeenCalled();
});

it.each(["account", "workspace", "root", "project"])("isolates retained cloud settings from another %s", async kind => {
  const props = callbacks();
  const first = render(<CloudDeployForm projectId="p-1" busy="" {...props} />);
  await readyForm(); edit("Application domain", "private-draft.example.com");
  first.unmount();
  const original = { ...identity };
  if (kind === "account") identity.actor = "bob";
  if (kind === "workspace") identity.workspace = "other-connection";
  if (kind === "root") identity.root = "/projects/other";
  const other = render(<CloudDeployForm projectId={kind === "project" ? "p-2" : "p-1"} busy="" {...props} />);
  await readyForm();
  expect(screen.queryByRole("region", { name: "Cloud deployment draft recovery" })).toBeNull();
  expect(screen.getByLabelText("Application domain")).toHaveProperty("value", "");
  other.unmount(); Object.assign(identity, original);
  render(<CloudDeployForm projectId="p-1" busy="" {...props} />);
  await waitFor(() => expect((screen.getByRole("button", { name: "Restore cloud deployment draft" }) as HTMLButtonElement).disabled).toBe(false));
  expect(screen.getByLabelText("Application domain")).toHaveProperty("disabled", true);
  expect(props.onDeploy).not.toHaveBeenCalled();
});

it("keeps unretained settings on the current tabs and never stores credential-bearing URLs", async () => {
  vi.mocked(productApi.gitDelivery).mockResolvedValue({ branch: "main", head: "a".repeat(40), origin: "https://owner:baseline-secret@example.com/repo" });
  render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(screen.getByRole("tab", { name: "Previews and publishing" }));
  await userEvent.click(screen.getByRole("tab", { name: "Your cloud" })); await readyForm();
  edit("Application domain", "app.example.com");
  expect(draftRecords()).toHaveLength(0);
  edit("Repository URL · HTTPS", "https://github.com/example/project");
  await waitFor(() => expect(draftRecords()).toHaveLength(1));
  expect(draftRecords()[0]).not.toContain("baseline-secret");
  edit("Repository URL · HTTPS", "https://owner:typed-secret@example.com/repo?token=private");
  await screen.findByText(/settings could not be retained safely/);
  expect(draftRecords().join("")).not.toContain("typed-secret");
  await userEvent.click(screen.getByRole("tab", { name: "GitHub Pages" }));
  expect(screen.getByRole("tab", { name: "Your cloud" }).getAttribute("aria-selected")).toBe("true");
  await userEvent.click(screen.getByRole("tab", { name: "Application platform" }));
  expect(screen.getByRole("tab", { name: "Previews and publishing" }).getAttribute("aria-selected")).toBe("true");
  await userEvent.click(screen.getByRole("button", { name: "Discard cloud deployment draft" }));
  await userEvent.click(screen.getByRole("tab", { name: "Application platform" }));
  expect(screen.getByText("Application controls")).toBeTruthy();
  expect(productApi.cloudRelease).not.toHaveBeenCalled();
});

it("keeps the form available when browser recovery fails and permits an explicit discard", async () => {
  render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(screen.getByRole("tab", { name: "Previews and publishing" }));
  await userEvent.click(screen.getByRole("tab", { name: "Your cloud" })); await readyForm();
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new DOMException("full", "QuotaExceededError"); });
  edit("Application domain", "still-here.example.com");
  await screen.findByText(/settings could not be retained safely/);
  expect(screen.getByLabelText("Application domain")).toHaveProperty("value", "still-here.example.com");
  await userEvent.click(screen.getByRole("tab", { name: "GitHub Pages" }));
  expect(screen.getByRole("tab", { name: "Your cloud" }).getAttribute("aria-selected")).toBe("true");
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).find(row => row.label === "Cloud deployment settings")?.retained).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Discard cloud deployment draft" }));
  await userEvent.click(screen.getByRole("tab", { name: "GitHub Pages" }));
  expect(screen.getByRole("tab", { name: "GitHub Pages" }).getAttribute("aria-selected")).toBe("true");
});

it("retains the browser draft through a deployment grant revocation without exposing active controls", async () => {
  const first = render(<CloudDeployForm projectId="p-1" busy="" {...callbacks()} />);
  await readyForm(); edit("Application domain", "retained.example.com"); first.unmount();
  identity.edit = false;
  const view = render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(screen.getByRole("tab", { name: "Previews and publishing" }));
  await userEvent.click(screen.getByRole("tab", { name: "Your cloud" }));
  expect(screen.queryByLabelText("Application domain")).toBeNull();
  expect(screen.queryByRole("button", { name: "Restore cloud deployment draft" })).toBeNull();
  expect(draftRecords()).toHaveLength(1);
  identity.edit = true; view.rerender(<DeploymentWorkspace projectId="p-1" />);
  await waitFor(() => expect((screen.getByRole("button", { name: "Restore cloud deployment draft" }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(screen.getByRole("button", { name: "Restore cloud deployment draft" }));
  expect(screen.getByLabelText("Application domain")).toHaveProperty("value", "retained.example.com");
  expect(productApi.cloudRelease).not.toHaveBeenCalled(); expect(productApi.prepareDeployKey).not.toHaveBeenCalled();
});
