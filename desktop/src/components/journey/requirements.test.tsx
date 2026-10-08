import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { productApi, type Brief, type CheckRun, type RequirementCoverage } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";
import { ProjectPlan } from "./ProjectPlan";
import { ProjectChecks } from "./ProjectChecks";
import { RequirementsEvidence } from "./RequirementsEvidence";
vi.mock("../../api", () => ({ api: { createSession: vi.fn() } }));

const brief: Brief = { projectId:"p-1", revision:2, purpose:"Offline journal", audience:"Writers", constraints:"Private", criteria:["Keep notes offline"], updatedAt:"" };
const plan = { projectId:"p-1",revision:3,briefRevision:2,tasks:[{id:"t1",title:"Build journal",status:"todo" as const}],updatedAt:"" };
const coverage: RequirementCoverage = {
  projectId:"p-1", specification:{revision:6,status:"approved",digest:"spec-6"}, design:{revision:8,status:"approved",digest:"design-8"},
  source:{commit:"a".repeat(40),sourceDigest:"source-a",dirty:false,available:true},
  rows:[
    {requirementId:"R-OFFLINE",title:"Keep notes offline",acceptance:"A saved note remains after restarting without a network.",taskIds:["t1"],doneTaskIds:[],evidenceRunIds:[],evidenceStatus:"unverified"},
    {requirementId:"R-DELETE",title:"Delete a note",acceptance:"A deleted note is absent after restarting.",taskIds:[],doneTaskIds:[],evidenceRunIds:[],evidenceStatus:"unverified"},
  ],
};
const commands = [{label:"Tests",executable:"npm",args:["test"]}];
const run: CheckRun = { id:"check-1",projectId:"p-1",commit:"a".repeat(40),dirty:false,execution:"docker",sourceDigest:"source-a",status:"passed",commands:[{...commands[0],status:"passed",exitCode:0,output:"One test passed",truncated:false}],createdAt:"2026-10-06T01:00:00Z",updatedAt:"2026-10-06T01:00:01Z",message:"Commands passed" };
beforeEach(() => { vi.restoreAllMocks();resetCockpit();vi.spyOn(productApi,"checks").mockResolvedValue({runs:[],suggested:commands,coverage});vi.spyOn(productApi,"plan").mockResolvedValue(plan); });
const taskLinks = async () => userEvent.click(await screen.findByText("Requirements for task 1 · 0 linked"));
const selectCriteria = async () => { await userEvent.click(await screen.findByText("Link checks to approved requirements · 0 selected"));await userEvent.click(screen.getByRole("checkbox",{name:"Check R-OFFLINE · Keep notes offline"})); };
const trust = () => userEvent.click(screen.getByRole("checkbox",{name:"I trust this project's code and these Docker commands to write project files and use the network."}));

it("saves a task link against the shown approved revision and retains it with the shared dirty guard after a conflict",async()=>{
  const save=vi.spyOn(productApi,"savePlan").mockRejectedValue(new Error("Requirements changed. Refresh and review the links."));
  render(<ProjectPlan projectId="p-1" brief={brief}/>);
  await taskLinks();await userEvent.click(await screen.findByRole("checkbox",{name:"Link R-OFFLINE to task 1"}));
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([{projectId:"p-1",label:"Implementation plan",lens:"develop"}]);
  await userEvent.click(screen.getByRole("button",{name:"Save implementation plan"}));
  expect(save).toHaveBeenCalledWith("p-1",3,[{...plan.tasks[0],requirementIds:["R-OFFLINE"]}],6);
  await screen.findByRole("alert");
  expect((screen.getByRole("checkbox",{name:"Link R-OFFLINE to task 1"}) as HTMLInputElement).checked).toBe(true);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
  expect(screen.queryByText(/Plan saved as revision/)).toBeNull();
});

it("adds approved requirements as explicitly linked tasks without making reported Done a verification result",async()=>{
  const save=vi.spyOn(productApi,"savePlan").mockResolvedValue({...plan,revision:4,specificationRevision:6});
  render(<ProjectPlan projectId="p-1" brief={brief}/>);
  const add=await screen.findByRole("button",{name:"Add approved requirements as tasks"});
  await waitFor(()=>expect((add as HTMLButtonElement).disabled).toBe(false));await userEvent.click(add);
  expect((screen.getByLabelText("Task 2") as HTMLInputElement).value).toBe("Keep notes offline");
  await userEvent.selectOptions(screen.getByLabelText("Progress for task 2"),"done");
  await userEvent.click(screen.getByRole("button",{name:"Save implementation plan"}));
  await waitFor(()=>expect(save).toHaveBeenCalled());
  expect(save.mock.calls[0][2][1]).toMatchObject({title:"Keep notes offline",status:"done",requirementIds:["R-OFFLINE"]});
  expect(save.mock.calls[0][3]).toBe(6);
  expect(save.mock.calls[0][2][2]).toMatchObject({title:"Delete a note",requirementIds:["R-DELETE"]});
});

it("preserves an unavailable saved link until the user removes it and refuses silently saving it against a new specification",async()=>{
  vi.spyOn(productApi,"plan").mockResolvedValue({...plan,specificationRevision:3,tasks:[{...plan.tasks[0],requirementIds:["R-OLD"]}]});
  const save=vi.spyOn(productApi,"savePlan").mockResolvedValue({...plan,revision:4});
  render(<ProjectPlan projectId="p-1" brief={brief}/>);
  await screen.findByText(/Task links were saved against requirements revision 3/);
  await userEvent.click(screen.getByText("Requirements for task 1 · 1 linked"));
  await userEvent.type(screen.getByLabelText("Task 1")," revised");
  expect((screen.getByRole("button",{name:"Save implementation plan"}) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("checkbox",{name:"Remove unavailable requirement R-OLD from task 1"}));
  await userEvent.click(screen.getByRole("checkbox",{name:"Link R-OFFLINE to task 1"}));
  await userEvent.click(screen.getByRole("button",{name:"Save implementation plan"}));
  expect(save).toHaveBeenCalledWith("p-1",3,[{...plan.tasks[0],title:"Build journal revised",requirementIds:["R-OFFLINE"]}],6);
});

it("records only the selected criteria with exact approved requirement and design revisions",async()=>{
  const bound={...run,requirementIds:["R-OFFLINE"],specificationRevision:6,designPlanRevision:8,evidenceStatus:"linked-test-evidence" as const,requirementSnapshots:[{id:"R-OFFLINE",title:"Keep notes offline",acceptance:coverage.rows[0].acceptance}]};
  const start=vi.spyOn(productApi,"runChecks").mockResolvedValue(bound);
  const refreshed={...coverage,rows:coverage.rows.map(row=>row.requirementId==="R-OFFLINE"?{...row,evidenceStatus:"linked-test-evidence" as const,evidenceRunIds:[run.id]}:row)};
  vi.spyOn(productApi,"checks").mockResolvedValueOnce({runs:[],suggested:commands,coverage}).mockResolvedValue({runs:[bound],suggested:commands,coverage:refreshed});
  render(<ProjectChecks projectId="p-1"/>);await selectCriteria();await trust();
  await userEvent.click(screen.getByRole("button",{name:"Run selected requirement checks"}));
  expect(start).toHaveBeenCalledWith("p-1",commands,true,"docker",{requirementIds:["R-OFFLINE"],expectedSpecificationRevision:6,expectedDesignPlanRevision:8});
  await screen.findByText(/Requirement evidence · 1 of 2 with current linked tests/);
  expect(screen.getByText(/passing commands and tasks marked Done do not establish requirement satisfaction/)).not.toBeNull();
});

it("refresh clears a selection and trust when approved design changes while retaining the reviewed commands",async()=>{
  const checks=vi.spyOn(productApi,"checks").mockResolvedValueOnce({runs:[],suggested:commands,coverage}).mockResolvedValue({runs:[],suggested:[],coverage:{...coverage,design:{revision:9,status:"approved",digest:"design-9"}}});
  render(<ProjectChecks projectId="p-1"/>);await selectCriteria();await trust();
  await userEvent.click(screen.getByText(/Requirement evidence ·/));
  await userEvent.click(screen.getByRole("button",{name:"Refresh requirement evidence"}));
  await screen.findByText("Requirements, design or source changed. Review and select the criteria again.");
  expect(screen.getByText("Tests")).not.toBeNull();
  expect((screen.getByRole("checkbox",{name:"Check R-OFFLINE · Keep notes offline"}) as HTMLInputElement).checked).toBe(false);
  expect((screen.getByRole("button",{name:"Run project checks"}) as HTMLButtonElement).disabled).toBe(true);
  expect(checks).toHaveBeenCalledTimes(2);
});

it("generic passing commands and reported Done leave requirement coverage unverified",async()=>{
  const reported={...coverage,rows:[{...coverage.rows[0],doneTaskIds:["t1"]}]};
  render(<RequirementsEvidence coverage={reported} runs={[run]} refreshing={false} onRefresh={()=>{}}/>);
  await userEvent.click(screen.getByText("Requirement evidence · 0 of 1 with current linked tests"));
  expect(screen.getByText("Unverified")).not.toBeNull();
  expect(screen.getByText("Linked tasks: t1. 1 marked Done by you.")).not.toBeNull();
  expect(screen.getByText("No linked check run recorded.")).not.toBeNull();
  expect(screen.queryByText(/Current linked test evidence/)).toBeNull();
});

it("keeps a newer failed run visible alongside older passing linked evidence and marks source drift stale",async()=>{
  const older={...run,requirementIds:["R-OFFLINE"],specificationRevision:6,designPlanRevision:8,evidenceStatus:"stale" as const,staleReasons:["Project source differs from the checked snapshot."]};
  const latest={...older,id:"check-2",status:"failed",createdAt:"2026-10-06T02:00:00Z",evidenceStatus:"unverified" as const,staleReasons:[]};
  const changed={...coverage,source:{...coverage.source,sourceDigest:"source-b",dirty:true},rows:[{...coverage.rows[0],evidenceStatus:"stale" as const,evidenceRunIds:[latest.id,older.id]}]};
  vi.spyOn(productApi,"checks").mockResolvedValue({runs:[latest,older],suggested:commands,coverage:changed});
  render(<ProjectChecks projectId="p-1"/>);await userEvent.click(await screen.findByText("Requirement evidence · 0 of 1 with current linked tests"));
  const evidence=screen.getByText(/Requirement evidence ·/).closest("details")!;
  expect(within(evidence).getByText("Stale evidence")).not.toBeNull();
  expect(within(evidence).getByText(/Latest linked run check-2 · Commands failed · Unverified/)).not.toBeNull();
  expect(screen.getByText("Project source differs from the checked snapshot.")).not.toBeNull();
});

it("an unavailable source cannot be selected for linked checks and cancellation remains unverified",async()=>{
  const active={...run,status:"running",requirementIds:["R-OFFLINE"],evidenceStatus:"unverified" as const};
  const canceled={...active,status:"cancelled",message:"Checks canceled"};
  vi.spyOn(productApi,"checks").mockResolvedValueOnce({runs:[active],suggested:commands,coverage:{...coverage,source:{...coverage.source,available:false}}}).mockResolvedValue({runs:[canceled],suggested:commands,coverage});
  const cancel=vi.spyOn(productApi,"cancelChecks").mockResolvedValue(canceled);
  render(<ProjectChecks projectId="p-1"/>);
  await userEvent.click(await screen.findByText("Link checks to approved requirements · 0 selected"));
  expect((screen.getByRole("checkbox",{name:"Check R-OFFLINE · Keep notes offline"}) as HTMLInputElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button",{name:"Cancel checks"}));
  expect(cancel).toHaveBeenCalledWith("p-1",run.id);
  await screen.findByText("Commands canceled");
  expect(screen.getByText("Unverified · R-OFFLINE")).not.toBeNull();
  expect(screen.queryByText("Current linked test evidence")).toBeNull();
});

it("a late previous-project coverage response cannot overwrite the selected project's criteria",async()=>{
  let finish!: (value:Awaited<ReturnType<typeof productApi.checks>>)=>void;
  vi.spyOn(productApi,"checks").mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValue({runs:[],suggested:commands,coverage:{...coverage,projectId:"p-2",rows:[{...coverage.rows[1],requirementId:"R-SECOND"}]}});
  const ui=render(<ProjectChecks projectId="p-1"/>);ui.rerender(<ProjectChecks projectId="p-2"/>);
  await screen.findByText("Link checks to approved requirements · 0 selected");
  await act(async()=>finish({runs:[run],suggested:[],coverage}));
  await userEvent.click(screen.getByText("Link checks to approved requirements · 0 selected"));
  expect(await screen.findByRole("checkbox",{name:"Check R-SECOND · Delete a note"})).not.toBeNull();
  expect(screen.queryByRole("checkbox",{name:"Check R-OFFLINE · Keep notes offline"})).toBeNull();
  expect(screen.queryByText("Commands passed")).toBeNull();
});

it("an old project's save cannot replace a new task draft or release the new project's in-flight save",async()=>{
  let finishOld!: (value: typeof plan) => void, finishNew!: (value: typeof plan) => void;
  const second={...plan,projectId:"p-2",revision:6,tasks:[{id:"t2",title:"Second project",status:"todo" as const}]};
  vi.spyOn(productApi,"plan").mockResolvedValueOnce(plan).mockResolvedValue(second);
  vi.spyOn(productApi,"checks").mockImplementation(projectId=>Promise.resolve({runs:[],suggested:commands,coverage:{...coverage,projectId}}));
  const save=vi.spyOn(productApi,"savePlan").mockImplementationOnce(()=>new Promise(resolve=>{finishOld=resolve;})).mockImplementationOnce(()=>new Promise(resolve=>{finishNew=resolve;}));
  const ui=render(<ProjectPlan projectId="p-1" brief={brief}/>);
  await userEvent.type(await screen.findByLabelText("Task 1")," old edit");await userEvent.click(screen.getByRole("button",{name:"Save implementation plan"}));
  ui.rerender(<ProjectPlan projectId="p-2" brief={{...brief,projectId:"p-2"}}/>);
  await waitFor(()=>expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Second project"));
  await userEvent.type(screen.getByLabelText("Task 1")," new edit");await userEvent.click(screen.getByRole("button",{name:"Save implementation plan"}));
  expect(save).toHaveBeenCalledTimes(2);
  await act(async()=>finishOld({...plan,revision:4}));
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe("Second project new edit");
  expect((screen.getByRole("button",{name:"Saving plan…"}) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByText("Plan saved as revision 4.")).toBeNull();
  await act(async()=>finishNew({...second,revision:7,tasks:[{...second.tasks[0],title:"Second project new edit"}]}));
  expect(await screen.findByText("Plan saved as revision 7.")).not.toBeNull();
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
});

it("an old project's check completion cannot appear in the new project or clear its running-action lock",async()=>{
  let finishOld!: (value: CheckRun) => void, finishNew!: (value: CheckRun) => void;
  vi.spyOn(productApi,"checks").mockImplementation(projectId=>Promise.resolve({runs:[],suggested:commands,coverage:{...coverage,projectId}}));
  const start=vi.spyOn(productApi,"runChecks").mockImplementationOnce(()=>new Promise(resolve=>{finishOld=resolve;})).mockImplementationOnce(()=>new Promise(resolve=>{finishNew=resolve;}));
  const ui=render(<ProjectChecks projectId="p-1"/>);await screen.findByText("Tests");await trust();await userEvent.click(screen.getByRole("button",{name:"Run project checks"}));
  ui.rerender(<ProjectChecks projectId="p-2"/>);await screen.findByText("Tests");await trust();await userEvent.click(screen.getByRole("button",{name:"Run project checks"}));
  expect(start).toHaveBeenCalledTimes(2);
  await act(async()=>finishOld({...run,message:"First project completed"}));
  expect(screen.queryByText("First project completed")).toBeNull();
  expect((screen.getByRole("button",{name:"Starting checks…"}) as HTMLButtonElement).disabled).toBe(true);
  await act(async()=>finishNew({...run,projectId:"p-2",message:"Second project completed"}));
  expect(await screen.findByText("Second project completed")).not.toBeNull();
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it.each([true,false])("keeps verification evidence readable with Execute revoked and permits cancellation only with Edit=%s", async canEdit => {
  teamAccess.permissions.edit = canEdit;
  teamAccess.permissions.execute = false;
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [{ ...run, status: "running" }], suggested: commands, coverage });
  const start = vi.spyOn(productApi, "runChecks"), cancel = vi.spyOn(productApi, "cancelChecks").mockImplementation(async () => {
    const stopped = { ...run, status: "cancelled", message: "Checks cancelled by an authorized editor." };
    vi.mocked(productApi.checks).mockResolvedValue({ runs: [stopped], suggested: commands, coverage });
    return stopped;
  });
  render(<ProjectChecks projectId="p-1" />);
  const stop = await screen.findByRole("button", { name: "Cancel checks" });
  expect((stop as HTMLButtonElement).disabled).toBe(!canEdit);
  expect((screen.getByRole("button", { name: "Checks running…" }) as HTMLButtonElement).disabled).toBe(true);
  const evidence = screen.getByText(/Requirement evidence ·/);
  await userEvent.click(evidence);
  expect(within(evidence.closest('details')!).getByText(coverage.rows[0].acceptance)).not.toBeNull();
  await userEvent.click(stop); expect(start).not.toHaveBeenCalled();
  if (canEdit) {
    expect(cancel).toHaveBeenCalledExactlyOnceWith("p-1", run.id);
    await screen.findByText("Checks cancelled by an authorized editor.");
    expect((screen.getByRole("button", { name: "Run project checks" }) as HTMLButtonElement).disabled).toBe(true);
  } else expect(cancel).not.toHaveBeenCalled();
  expect(screen.getByText(canEdit ? /Execution access is required/ : /Read-only access/)).not.toBeNull();
});
