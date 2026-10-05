import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { designPlanApi, type DesignPlan } from "../../api/designPlan";
import { definitionApi } from "../../api/definition";
import { productApi } from "../../api/product";
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
  vi.spyOn(productApi,"releases").mockResolvedValue({releases:[]});
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
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
}

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
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [release] });
  const view = render(<DesignWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
  await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
  const frame = await screen.findByTitle("Design prototype");
  expect(frame.getAttribute("src")).toBe(release.url);
  expect(frame.getAttribute("sandbox")).toBe("allow-scripts allow-forms allow-same-origin");
  view.unmount();
  for (const url of [window.location.origin + "/", DEFAULT_BASE_URL + "/", "http://account:password@localhost:8731/", `http://${"a".repeat(64)}.localhost:8731/`]) {
    vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...release, url }] });
    const attempt = render(<DesignWorkspace projectId="p-1" />);
    await userEvent.click(await screen.findByRole("tab", { name: "Prototype" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Open prototype" })).not.toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "Open prototype" }));
    expect((await screen.findByRole("alert")).textContent).toContain("verified separate application origin");
    expect(screen.queryByTitle("Design prototype")).toBeNull();
    attempt.unmount();
  }
});
