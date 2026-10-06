import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { designPlanApi, type DesignPlan } from "../../api/designPlan";
import { definitionApi } from "../../api/definition";
import { productApi, type Release } from "../../api/product";
import * as browserAccess from "../../api/browserAccess";
import { platformApi, type PlatformProject } from "../../api/platform";
import DesignWorkspace from "./DesignWorkspace";
import { resetCockpit, useCockpit } from "../../store/store";
import { DEFAULT_BASE_URL } from "../../api/client";

vi.mock("../journey/DesignEditor", () => ({ default: ({ onDirtyChange, onBusyChange }: { onDirtyChange?(dirty: boolean): void; onBusyChange?(busy: boolean): void }) => <div>Drawing tools<button onClick={() => onDirtyChange?.(true)}>Edit sketch fixture</button><button onClick={() => onDirtyChange?.(false)}>Save sketch fixture</button><button onClick={() => onBusyChange?.(true)}>Begin sketch save fixture</button><button onClick={() => onBusyChange?.(false)}>Complete sketch save fixture</button></div> }));
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
beforeEach(() => resetCockpit());
const fixture = (): DesignPlan => ({ projectId:"p-1", revision:2, briefRevision:1, specificationRevision:2, sketchRevision:0, screens:[{id:"screen-1",name:"Home",route:"/",purpose:"Save work",states:["Empty","Saved"],widths:[390,1440],requirementIds:["AC-001"],accessibility:"Keyboard"}],tokens:[],assets:[],implementationNotes:"Saved notes",prototypeReleaseId:"",prototypeCommit:"",status:"draft",digest:"",updatedAt:"" });
it("retains newly typed handoff notes when the sketch refresh completes late", async () => {
  let finish!: (value: DesignPlan) => void;
  vi.spyOn(designPlanApi,"get").mockResolvedValueOnce(fixture()).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  vi.spyOn(definitionApi,"get").mockResolvedValue({projectId:"p-1",version:1,revision:2,briefRevision:1,startingPoint:"new",purpose:"Save work",audience:"People",constraints:"",outOfScope:[],requirements:[{id:"AC-001",title:"Save work",priority:"must",acceptance:"Reopened work returns"}],journeys:[],roles:[],entities:[],integrations:[],glossary:[],decisions:[],questions:[],status:"approved",digest:"a".repeat(64),updatedAt:""});
  vi.spyOn(productApi,"prototypeReleases").mockResolvedValue({releases:[]});
  render(<DesignWorkspace projectId="p-1"/>);
  await screen.findByRole("textbox",{name:"Screen name"});
  await userEvent.click(screen.getByRole("tab",{name:"Sketch"}));
  await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("tab",{name:"Implementation handoff"}));
  const notes = screen.getByRole("textbox",{name:"Implementation notes"});
  await userEvent.clear(notes); await userEvent.type(notes,"Keep my unsaved keyboard behavior");
  await act(async () => { finish({...fixture(),status:"outdated"}); });
  expect((notes as HTMLTextAreaElement).value).toBe("Keep my unsaved keyboard behavior");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
});

function ready(plan = fixture()) {
  vi.spyOn(designPlanApi, "get").mockResolvedValue(plan);
  vi.spyOn(definitionApi, "get").mockResolvedValue({ projectId:"p-1", version:1, revision:2, briefRevision:1, startingPoint:"new", purpose:"Save work", audience:"People", constraints:"", outOfScope:[], requirements:[{id:"AC-001", title:"Save work", priority:"must", acceptance:"Reopened work returns"}], journeys:[], roles:[], entities:[], integrations:[], glossary:[], decisions:[], questions:[], status:"approved", digest:"a".repeat(64), updatedAt:"" });
  vi.spyOn(productApi, "prototypeReleases").mockResolvedValue({ releases: [] });
}

it("keeps saved design usable after optional prototype history fails and retries without blocking approval", async () => {
  ready();
  vi.mocked(productApi.prototypeReleases).mockRejectedValue(new Error("History unavailable"));
  const save = vi.spyOn(designPlanApi, "save").mockResolvedValue({ ...fixture(), revision: 3, implementationNotes: "Keep saved work" });
  const approve = vi.spyOn(designPlanApi, "approve").mockResolvedValue({ ...fixture(), revision: 4, implementationNotes: "Keep saved work", status: "approved" });
  render(<DesignWorkspace projectId="p-1" />);
  await screen.findByLabelText("Screen name");
  expect(screen.queryByRole("alert")).toBeNull();
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  await userEvent.clear(screen.getByLabelText("Implementation notes")); await userEvent.type(screen.getByLabelText("Implementation notes"), "Keep saved work");
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  await screen.findByText("Design plan 3 saved. Implementation context updated.");
  expect(save).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("tab", { name: "Prototype" }));
  expect((await screen.findByRole("alert")).textContent).toContain("History unavailable");
  expect(screen.queryByText("No verified prototype yet")).toBeNull();
  let finish!: (value: { releases: Release[] }) => void;
  vi.mocked(productApi.prototypeReleases).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Retry prototype history" }));
  await screen.findByText(/Refreshing prototype history/);
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  expect((screen.getByLabelText("Implementation notes") as HTMLTextAreaElement).value).toBe("Keep saved work");
  expect((screen.getByRole("button", { name: "Approve design handoff" }) as HTMLButtonElement).disabled).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  await act(async () => finish({ releases: [] }));
  expect(screen.getByRole("button", { name: "Continue to Develop" })).not.toBeNull();
  expect(designPlanApi.get).toHaveBeenCalledOnce();
});

it("links design tabs to manually activated panels without opening sketch on keyboard focus", async () => {
  ready();
  render(<DesignWorkspace projectId="p-1" />);
  const selected = await screen.findByRole("tab", { name: "Screens and states" });
  const user = userEvent.setup();
  selected.focus();
  await user.keyboard("{ArrowRight}");
  expect(screen.queryByText("Drawing tools")).toBeNull();
  expect(selected.getAttribute("aria-selected")).toBe("true");
  await user.keyboard("{Enter}");
  await screen.findByText("Drawing tools");
  const sketch = screen.getByRole("tab", { name: "Sketch" });
  const panel = screen.getByRole("tabpanel", { name: "Sketch" });
  expect(panel.id).toBe(sketch.getAttribute("aria-controls"));
  expect(panel.getAttribute("aria-labelledby")).toBe(sketch.id);
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
});

it("retains the mounted sketch on attempted navigation and only discards it explicitly", async () => {
  ready();
  const save = vi.spyOn(productApi, "saveDesign");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Sketch" }));
  const canvas = await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button", { name: "Edit sketch fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  expect((await screen.findByRole("alert")).textContent).toContain("unsaved changes");
  expect(screen.getByRole("tab", { name: "Sketch" }).getAttribute("aria-selected")).toBe("true");
  expect(screen.getByText("Drawing tools")).toBe(canvas);
  expect(designPlanApi.get).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getByText("Drawing tools")).toBe(canvas);
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  await userEvent.click(screen.getByRole("button", { name: "Discard sketch and continue" }));
  expect(screen.queryByText("Drawing tools")).toBeNull();
  expect(screen.getByRole("textbox", { name: "Implementation notes" })).not.toBeNull();
  await waitFor(() => expect(designPlanApi.get).toHaveBeenCalledTimes(2));
  expect(save).not.toHaveBeenCalled();
});

it("requires re-saving the handoff against the changed sketch before approving the exact new plan revision", async () => {
  ready();
  let finishRead!: (value: DesignPlan) => void;
  vi.mocked(designPlanApi.get).mockResolvedValueOnce(fixture()).mockImplementationOnce(() => new Promise(resolve => { finishRead = resolve; }));
  const rebound: DesignPlan = { ...fixture(), revision: 3, sketchRevision: 7 };
  const save = vi.spyOn(designPlanApi, "save").mockResolvedValue(rebound);
  const approve = vi.spyOn(designPlanApi, "approve").mockResolvedValue({ ...rebound, revision: 4, status: "approved" });
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Sketch" }));
  await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button", { name: "Edit sketch fixture" }));
  await userEvent.click(screen.getByRole("button", { name: "Save sketch fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  const staleApprove = screen.getByRole("button", { name: "Approve design handoff" });
  expect((staleApprove as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(staleApprove);
  expect(approve).not.toHaveBeenCalled();
  await act(async () => finishRead({ ...fixture(), status: "outdated" }));
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  expect(save).toHaveBeenCalledExactlyOnceWith("p-1", { ...fixture(), status: "outdated" }, 2, 1, 2);
  await userEvent.click(screen.getByRole("tab", { name: "Implementation handoff" }));
  expect(screen.getByText("Sketch", { selector: "dt" }).nextElementSibling?.textContent).toBe("7");
  await userEvent.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  expect(screen.getByRole("button", { name: "Continue to Develop" })).not.toBeNull();
});

it("does not unmount the sketch during its explicit save operation", async () => {
  ready();
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Sketch" }));
  const canvas = await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button", { name: "Begin sketch save fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  expect(screen.getByText("Drawing tools")).toBe(canvas);
  expect(screen.queryByRole("alert")).toBeNull();
  expect(designPlanApi.get).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByRole("button", { name: "Complete sketch save fixture" }));
  await userEvent.click(screen.getByRole("tab", { name: "Screens and states" }));
  await screen.findByRole("textbox", { name: "Screen name" });
});

it("requires saved design notes and explicit approval before continuing to Develop", async () => {
  ready();
  let finish!: (value: DesignPlan) => void;
  const save = vi.spyOn(designPlanApi, "save").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const approve = vi.spyOn(designPlanApi, "approve").mockResolvedValue({ ...fixture(), revision: 3, status: "approved", digest: "b".repeat(64) });
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Implementation handoff" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Implementation notes" }), " and real recovery");
  expect(screen.queryByRole("button", { name: "Approve design handoff" })).toBeNull();
  const action = screen.getByRole("button", { name: "Save design plan" });
  await userEvent.click(action);
  expect(action.getAttribute("aria-busy")).toBe("true");
  expect((action as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(action);
  expect(save).toHaveBeenCalledOnce();
  expect(approve).not.toHaveBeenCalled();
  await act(async () => finish({ ...save.mock.calls[0][1], revision: 3 }));
  await userEvent.click(screen.getByRole("button", { name: "Approve design handoff" }));
  expect(approve).toHaveBeenCalledExactlyOnceWith("p-1", 3);
  await userEvent.click(await screen.findByRole("button", { name: "Continue to Develop" }));
  expect(useCockpit.getState().lens).toBe("develop");
});

it("retains the design draft and save action after a revision refusal", async () => {
  ready();
  vi.spyOn(designPlanApi, "save").mockRejectedValue(new Error("The sketch changed. Review the current saved design before retrying."));
  const approve = vi.spyOn(designPlanApi, "approve");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Implementation handoff" }));
  const notes = screen.getByRole("textbox", { name: "Implementation notes" });
  await userEvent.type(notes, " with unsaved recovery steps");
  await userEvent.click(screen.getByRole("button", { name: "Save design plan" }));
  expect((await screen.findByRole("alert")).textContent).toContain("The sketch changed");
  expect((notes as HTMLTextAreaElement).value).toBe("Saved notes with unsaved recovery steps");
  expect((screen.getByRole("button", { name: "Save design plan" }) as HTMLButtonElement).disabled).toBe(false);
  expect(screen.queryByRole("button", { name: "Continue to Develop" })).toBeNull();
  expect(approve).not.toHaveBeenCalled();
});

it("preserves the local isolated prototype gateway and rejects control-origin or credential URLs", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const release = { id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url: `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`, message: "Verified", createdAt: "" };
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(release.url);
  expect(frame.getAttribute("sandbox")).toBe("allow-scripts allow-forms allow-same-origin");
  view.unmount();
  for (const url of [window.location.origin + "/", DEFAULT_BASE_URL + "/", "http://account:password@localhost:8731/", `http://${"a".repeat(64)}.localhost:8731/`]) {
    vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ ...release, url }] });
    const attempt = render(<DesignWorkspace projectId="p-1" />);
    await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Open prototype" })).not.toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
    expect((await screen.findByRole("alert")).textContent).toContain("verified separate application origin");
    expect(screen.queryByTitle("Design prototype")).toBeNull();
    attempt.unmount();
  }
});

const applicationRelease = (): Release => ({
  id: "deploy-owned", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40),
  configurationRevision: 4,
  target: "application", status: "ready", url: "", browserConnectionRequired: true,
  message: "Verified application", createdAt: "",
});

const nativeProject = (cloud = false): { project: PlatformProject; busy: boolean } => ({
  project: { projectId: "p-1", deployments: [{
    id: "deploy-owned", projectId: "p-1", provider: "docker", commit: "a".repeat(40),
    configurationRevision: 4, status: "ready", message: "Owned application", createdAt: "", updatedAt: "",
    browserConnectionRequired: cloud,
    url: cloud ? undefined : `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`,
  }], database: { status: "not-created", engine: "postgres", message: "" }, backups: [] }, busy: false,
});

it("opens a local Docker prototype through its existing owned gateway without requiring a cloud account", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [applicationRelease()] });
  const local = nativeProject();
  const read = vi.spyOn(platformApi, "project").mockResolvedValue(local);
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(local.project.deployments[0].url);
  expect(read).toHaveBeenCalledExactlyOnceWith("p-1");
  expect(prepare).not.toHaveBeenCalled();
});

it.each([
  "project", "deployment-project", "missing", "duplicate", "commit", "configuration", "busy", "stopped", "gateway",
])("rejects changed local prototype metadata: %s", async (changed) => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [applicationRelease()] });
  const local = nativeProject();
  const deployment = local.project.deployments[0];
  if (changed === "project") local.project.projectId = "another-project";
  if (changed === "deployment-project") deployment.projectId = "another-project";
  if (changed === "missing") local.project.deployments = [];
  if (changed === "duplicate") local.project.deployments.push({ ...deployment });
  if (changed === "commit") deployment.commit = "b".repeat(40);
  if (changed === "configuration") deployment.configurationRevision++;
  if (changed === "busy") local.busy = true;
  if (changed === "stopped") deployment.status = "stopped";
  if (changed === "gateway") deployment.url = DEFAULT_BASE_URL;
  vi.spyOn(platformApi, "project").mockResolvedValue(local);
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  await screen.findByRole("alert");
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect(prepare).not.toHaveBeenCalled();
});

it("opens a cloud Docker prototype through its owned isolated browser connection", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  const release = applicationRelease();
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  vi.spyOn(platformApi, "project").mockResolvedValue(nativeProject(true));
  const connection = { projectId: "p-1", kind: "application" as const, resourceId: release.id,
    url: `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/` };
  const prepare = vi.spyOn(platformApi, "prepareBrowser").mockResolvedValue(connection);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  expect(screen.getByRole("option", { name: /Application · aaaaaaaa/ })).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  expect(prepare).toHaveBeenCalledExactlyOnceWith("p-1", "application", release.id);
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(connection.url);
  expect(frame.getAttribute("sandbox")).toBe("allow-scripts allow-forms allow-same-origin");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "Preview width" }), "390");
  expect((frame as HTMLIFrameElement).style.width).toBe("390px");
});

it.each([
  { projectId: "another-project", kind: "application" as const, resourceId: "deploy-owned", url: "http://127.0.0.1:8731/" },
  { projectId: "p-1", kind: "data" as const, resourceId: "deploy-owned", url: "http://127.0.0.1:8731/" },
  { projectId: "p-1", kind: "application" as const, resourceId: "another-release", url: "http://127.0.0.1:8731/" },
  { projectId: "p-1", kind: "application" as const, resourceId: "deploy-owned", url: DEFAULT_BASE_URL },
])("refuses a changed prototype/browser binding: %j", async (connection) => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [applicationRelease()] });
  vi.spyOn(platformApi, "project").mockResolvedValue(nativeProject(true));
  vi.spyOn(platformApi, "prepareBrowser").mockResolvedValue(connection);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  expect((await screen.findByRole("alert")).textContent).toContain("verify this prototype's browser connection");
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect((screen.getByRole("button", { name: "Open prototype" }) as HTMLButtonElement).disabled).toBe(false);
});

it("makes a stopped saved prototype recoverable and never offers a neighbouring project's release", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [
    { ...applicationRelease(), status: "stopped" },
    { ...applicationRelease(), id: "foreign-release", projectId: "other-project" },
  ] });
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  expect(screen.getByText(/This prototype has stopped or is no longer available/)).not.toBeNull();
  expect(screen.getByRole("option", { name: "Saved prototype unavailable" })).not.toBeNull();
  expect(screen.queryByRole("option", { name: /foreign-release/ })).toBeNull();
  const open = screen.getByRole("button", { name: "Open prototype" });
  expect((open as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(open);
  expect(prepare).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Go to Deploy" }));
  expect(useCockpit.getState().lens).toBe("deploy");
});

it("does not show a previous project's late application connection after navigation", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(designPlanApi.get).mockImplementation(async (id) => ({ ...fixture(), projectId: id, prototypeReleaseId: id === "p-1" ? "deploy-owned" : "" }));
  vi.mocked(productApi.prototypeReleases).mockImplementation(async (id) => ({ releases: id === "p-1" ? [applicationRelease()] : [] }));
  vi.spyOn(platformApi, "project").mockResolvedValue(nativeProject(true));
  let finish!: (connection: { projectId: string; kind: "application"; resourceId: string; url: string }) => void;
  vi.spyOn(platformApi, "prepareBrowser").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  view.rerender(<DesignWorkspace projectId="p-2" />);
  await screen.findByText("No verified prototype yet");
  await act(async () => { finish({ projectId: "p-1", kind: "application", resourceId: "deploy-owned", url: "http://127.0.0.1:8731/" }); });
  expect(screen.queryByTitle("Design prototype")).toBeNull();
  expect((screen.getByRole("button", { name: "Open prototype" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("alert")).toBeNull();
});

it("does not prepare an old cloud browser after its deployment read finishes in another project", async () => {
  ready({ ...fixture(), prototypeReleaseId: "deploy-owned" });
  vi.mocked(designPlanApi.get).mockImplementation(async (id) => ({ ...fixture(), projectId: id, prototypeReleaseId: id === "p-1" ? "deploy-owned" : "" }));
  vi.mocked(productApi.prototypeReleases).mockImplementation(async (id) => ({ releases: id === "p-1" ? [applicationRelease()] : [] }));
  let finish!: (value: ReturnType<typeof nativeProject>) => void;
  vi.spyOn(platformApi, "project").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const prepare = vi.spyOn(platformApi, "prepareBrowser");
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  view.rerender(<DesignWorkspace projectId="p-2" />);
  await screen.findByText("No verified prototype yet");
  await act(async () => { finish(nativeProject(true)); });
  expect(prepare).not.toHaveBeenCalled();
  expect(screen.queryByTitle("Design prototype")).toBeNull();
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it("keeps saved design readable for viewers without offering screen edits or approval", async () => {
  ready(); teamAccess.permissions.edit = false; teamAccess.permissions.execute = false;
  const save = vi.spyOn(designPlanApi, "save"), approve = vi.spyOn(designPlanApi, "approve");
  render(<DesignWorkspace projectId="p-1" />);
  const name = await screen.findByLabelText("Screen name");
  expect((name.closest("fieldset.definition-fields") as HTMLFieldSetElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Approve design handoff" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.type(name, " forbidden");
  expect((name as HTMLInputElement).value).toBe("Home"); expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
});


it("prepares a fresh one-use handoff for a separate tab and keeps the embedded ticket unchanged", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/app#review`;
  const release = { id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" };
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [release] });
  const prepare = vi.spyOn(browserAccess, "prepareBrowserAccess").mockResolvedValueOnce("https://isolated.example/.well-known/luci-team-access?ticket=embedded").mockResolvedValueOnce("https://isolated.example/.well-known/luci-team-access?ticket=separate");
  const tab = { opener: window, location: { href: "about:blank" }, close: vi.fn() };
  vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
  render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  await userEvent.click(screen.getByRole("button", { name: "Open prototype in a separate tab" }));
  await waitFor(() => expect(tab.location.href).toContain("ticket=separate"));
  expect(prepare.mock.calls).toEqual([["p-1", url], ["p-1", url]]);
  expect(frame.getAttribute("src")).toContain("ticket=embedded"); expect(tab.opener).toBeNull(); expect(tab.close).not.toHaveBeenCalled();
});

it("closes a pending separate tab when execution access is revoked and does not reuse the embedded ticket", async () => {
  ready({ ...fixture(), prototypeReleaseId: "verified" });
  const url = `http://${"a".repeat(32)}.${"b".repeat(32)}.localhost:8731/`;
  vi.mocked(productApi.prototypeReleases).mockResolvedValue({ releases: [{ id: "verified", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "ready", url, message: "Verified", createdAt: "" }] });
  let finish!: (url: string) => void;
  vi.spyOn(browserAccess, "prepareBrowserAccess").mockResolvedValueOnce(url).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const tab = { opener: window, location: { href: "about:blank" }, close: vi.fn() };
  vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" })); await screen.findByTitle("Design prototype");
  await userEvent.click(screen.getByRole("button", { name: "Open prototype in a separate tab" }));
  teamAccess.permissions.execute = false; view.rerender(<DesignWorkspace projectId="p-1" />);
  await act(async () => finish("https://isolated.example/.well-known/luci-team-access?ticket=stale"));
  expect(tab.close).toHaveBeenCalledOnce(); expect(tab.location.href).toBe("about:blank"); expect(screen.queryByTitle("Design prototype")).toBeNull();
  teamAccess.permissions.execute = true; view.rerender(<DesignWorkspace projectId="p-1" />);
  expect(screen.queryByTitle("Design prototype")).toBeNull();
});
