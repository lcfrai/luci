import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { designPlanApi, type DesignPlan } from "../../api/designPlan";
import { definitionApi } from "../../api/definition";
import { productApi } from "../../api/product";
import DesignWorkspace from "./DesignWorkspace";

vi.mock("../journey/DesignEditor", () => ({ default: () => <div>Drawing tools</div> }));
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
const fixture = (): DesignPlan => ({ projectId:"p-1", revision:2, briefRevision:1, specificationRevision:2, sketchRevision:0, screens:[{id:"screen-1",name:"Home",route:"/",purpose:"Save work",states:["Empty","Saved"],widths:[390,1440],requirementIds:["AC-001"],accessibility:"Keyboard"}],tokens:[],assets:[],implementationNotes:"Saved notes",prototypeReleaseId:"",prototypeCommit:"",status:"draft",digest:"",updatedAt:"" });
it("retains newly typed handoff notes when the sketch refresh completes late", async () => {
  let finish!: (value: DesignPlan) => void;
  vi.spyOn(designPlanApi,"get").mockResolvedValueOnce(fixture()).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  vi.spyOn(definitionApi,"get").mockResolvedValue({projectId:"p-1",version:1,revision:2,briefRevision:1,startingPoint:"new",purpose:"Save work",audience:"People",constraints:"",outOfScope:[],requirements:[{id:"AC-001",title:"Save work",priority:"must",acceptance:"Reopened work returns"}],journeys:[],roles:[],entities:[],integrations:[],glossary:[],decisions:[],questions:[],status:"approved",digest:"a".repeat(64),updatedAt:""});
  vi.spyOn(productApi,"releases").mockResolvedValue({releases:[]});
  render(<DesignWorkspace projectId="p-1"/>);
  await screen.findByRole("textbox",{name:"Screen name"});
  await userEvent.click(screen.getByRole("button",{name:"Sketch"}));
  await screen.findByText("Drawing tools");
  await userEvent.click(screen.getByRole("button",{name:"Implementation handoff"}));
  const notes = screen.getByRole("textbox",{name:"Implementation notes"});
  await userEvent.clear(notes); await userEvent.type(notes,"Keep my unsaved keyboard behavior");
  await act(async () => { finish({...fixture(),status:"outdated"}); });
  expect((notes as HTMLTextAreaElement).value).toBe("Keep my unsaved keyboard behavior");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
});
