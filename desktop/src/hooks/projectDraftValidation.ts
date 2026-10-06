import type { ProjectSpecification } from "../api/definition";
import type { DesignPlan } from "../api/designPlan";
import type { DesignScene, ImplementationTask } from "../api/product";
import { draftMatches } from "./useProjectDraft";
export const definitionDraftValid = (value: unknown): value is ProjectSpecification => draftMatches(value, {
  projectId: "", revision: 0, briefRevision: 0, version: 1, startingPoint: "", purpose: "", audience: "", constraints: "", outOfScope: [""],
  requirements: [{ id: "", title: "", priority: "", acceptance: "" }], journeys: [{ id: "", name: "", actor: "", steps: "", requirementIds: [""] }],
  roles: [{ id: "", name: "", canDo: "", cannotDo: "" }], entities: [{ id: "", name: "", fields: "", privacy: "" }], integrations: [{ id: "", name: "", purpose: "", constraint: "" }],
  glossary: [{ id: "", term: "", meaning: "" }], decisions: [{ id: "", title: "", decision: "", reason: "", alternatives: "" }], questions: [{ id: "", question: "", answer: "" }], status: "", digest: "", updatedAt: "",
});
export const designPlanDraftValid = (value: unknown): value is DesignPlan => draftMatches(value, {
  projectId: "", revision: 0, briefRevision: 0, specificationRevision: 0, sketchRevision: 0,
  screens: [{ id: "", name: "", route: "", purpose: "", states: [""], widths: [0], requirementIds: [""], accessibility: "" }],
  tokens: [{ name: "", value: "", purpose: "" }], assets: [{ id: "", path: "", purpose: "", licence: "" }],
  implementationNotes: "", prototypeReleaseId: "", prototypeCommit: "", status: "", digest: "", updatedAt: "",
});
export interface PlanDraft { tasks: ImplementationTask[]; title: string }
export const planDraftValid = (value: unknown): value is PlanDraft => draftMatches(value, { title: "", tasks: [{ id: "", title: "", status: "" }] }) &&
  (value as PlanDraft).tasks.every(task => ["todo", "doing", "done"].includes(task.status) && (!task.requirementIds || task.requirementIds.every(id => typeof id === "string")));
export const sketchDraftValid = (value: unknown): value is DesignScene => draftMatches(value, { elements: [], appState: {}, files: {} });
