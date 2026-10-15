import type { ProjectSpecification } from "../api/definition";
import { discoverySourceLinksValid } from "../api/discoveryDocuments";
import type { DesignPlan } from "../api/designPlan";
import { editableDesignSceneValid, designRasterPinValid } from '../api/designScene';
import type { CheckCommand, DesignScene, ImplementationTask } from "../api/product";
import { draftMatches } from "./useProjectDraft";
export const definitionDraftValid = (value: unknown): value is ProjectSpecification => draftMatches(value, {
  projectId: "", revision: 0, briefRevision: 0, version: 1, startingPoint: "", purpose: "", audience: "", constraints: "", outOfScope: [""],
  requirements: [{ id: "", title: "", priority: "", acceptance: "" }], journeys: [{ id: "", name: "", actor: "", steps: "", requirementIds: [""] }],
  roles: [{ id: "", name: "", canDo: "", cannotDo: "" }], entities: [{ id: "", name: "", fields: "", privacy: "" }], integrations: [{ id: "", name: "", purpose: "", constraint: "" }],
  glossary: [{ id: "", term: "", meaning: "" }], decisions: [{ id: "", title: "", decision: "", reason: "", alternatives: "" }], questions: [{ id: "", question: "", answer: "" }], status: "", digest: "", updatedAt: "",
}) && ((value as ProjectSpecification).evidence === undefined || discoverySourceLinksValid((value as ProjectSpecification).evidence));
export const designPlanDraftValid = (value: unknown): value is DesignPlan => draftMatches(value, {
  projectId: "", revision: 0, briefRevision: 0, specificationRevision: 0, sketchRevision: 0,
  screens: [{ id: "", name: "", route: "", purpose: "", states: [""], widths: [0], requirementIds: [""], accessibility: "" }],
  tokens: [{ name: "", value: "", purpose: "" }], assets: [{ id: "", path: "", purpose: "", licence: "" }],
  implementationNotes: "", prototypeReleaseId: "", prototypeCommit: "", status: "", digest: "", updatedAt: "",
}) && ((value as DesignPlan).scene === undefined || editableDesignSceneValid((value as DesignPlan).scene)) &&
  (value as DesignPlan).assets.every(asset => asset.content === undefined || designRasterPinValid(asset.content));
export interface PlanDraft { tasks: ImplementationTask[]; title: string }
const optionalTextList = (value: unknown): boolean => value === undefined || (Array.isArray(value) && value.every(item => typeof item === "string"));
export const planDraftValid = (value: unknown): value is PlanDraft => draftMatches(value, { title: "", tasks: [{ id: "", title: "", status: "" }] }) &&
  (value as PlanDraft).tasks.every(task => ["todo", "doing", "done"].includes(task.status) && optionalTextList(task.requirementIds) && optionalTextList(task.dependsOn) && optionalTextList(task.writePaths) && (task.acceptance === undefined || typeof task.acceptance === "string"));
export const sketchDraftValid = (value: unknown): value is DesignScene => draftMatches(value, { elements: [], appState: {}, files: {} });

/** Browser authoring intent only. Execution consent and runtime settings are never recovered. */
export interface CheckSetupDraft { commands: CheckCommand[]; executable: string; args: string; requirementIds: string[]; buildId: string }
export function checkSetupDraftValid(value: unknown): value is CheckSetupDraft {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const draft = value as CheckSetupDraft;
  const text = (item: unknown, limit = 65536): item is string => typeof item === "string" && item.length <= limit;
  return Object.keys(draft).sort().join(",") === "args,buildId,commands,executable,requirementIds" &&
    text(draft.executable, 4096) && text(draft.args) && text(draft.buildId, 256) &&
    Array.isArray(draft.requirementIds) && draft.requirementIds.length <= 1000 && draft.requirementIds.every(id => text(id, 256)) &&
    Array.isArray(draft.commands) && draft.commands.length <= 100 && draft.commands.every(command =>
      !!command && typeof command === "object" && Object.keys(command).sort().join(",") === "args,executable,label" &&
      text(command.label, 4096) && text(command.executable, 4096) && Array.isArray(command.args) && command.args.length <= 1000 && command.args.every(arg => text(arg)));
}
