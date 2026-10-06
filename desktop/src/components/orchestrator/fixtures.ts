import type { OrchestratorContext, OrchestratorJob, OrchestratorTurn, OrchestratorView } from "../../api/orchestrator";

export const context = (projectId = "p-1"): OrchestratorContext => ({
  projectId, digest: "a".repeat(64), brief: { projectId, revision: 1, purpose: "A booking service", audience: "Visitors", constraints: "", criteria: [], updatedAt: "" },
  specification: { projectId, version: 1, revision: 2, briefRevision: 1, startingPoint: "new", purpose: "A booking service", audience: "Visitors", constraints: "", outOfScope: [], requirements: [{ id: "REQ-1", title: "Book", priority: "must", acceptance: "A visitor can book" }], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: "approved", digest: "spec", updatedAt: "" },
  designPlan: { projectId, revision: 1, briefRevision: 1, specificationRevision: 2, sketchRevision: 0, screens: [], tokens: [], assets: [], implementationNotes: "", prototypeReleaseId: "", prototypeCommit: "", status: "draft", digest: "design", updatedAt: "" },
  plan: { projectId, revision: 1, briefRevision: 1, tasks: [{ id: "T-1", title: "Build booking", status: "todo" }], updatedAt: "" },
  sketchRevision: 0, sourceCommit: "a1b2c3", sourceDirty: false, sourceKnown: true, checksSummary: "No checks recorded", releasesSummary: "No release recorded",
});
export const turn = (patch: Partial<OrchestratorTurn> = {}): OrchestratorTurn => ({
  id: "turn-1", projectId: "p-1", sequence: 1, actor: { id: "alice", displayName: "Alice" }, message: "Support guest checkout", jobId: "job-1", contextDigest: context().digest, context: context(), status: "succeeded", createdAt: "2026-10-06T14:00:00Z", updatedAt: "2026-10-06T14:01:00Z",
  answer: { summary: "We need to decide how guests recover their booking.", question: "Can guests book without an account?", options: ["Allow guest booking", "Require an account"], rationale: "This changes recovery and personal data handling.", blocking: true, impact: ["Definition and data access", "Booking screens and verification"], proposal: null, nextAction: "Resolve the booking decision." }, ...patch,
});
export const proposalTurn = (): OrchestratorTurn => ({ ...turn(), answer: {
  summary: "Guest checkout can be included in the definition.", question: "", options: [], rationale: "Visitors should be able to book with less friction.", blocking: false, impact: ["Definition approval will need review"], nextAction: "Review the proposed requirement.", proposal: { target: "specification", summary: "Add guest booking and email recovery", specification: { ...context().specification, purpose: "Booking with guest checkout", requirements: [{ id: "REQ-1", title: "Guest booking", priority: "must", acceptance: "A guest receives a private recovery link" }] }, designPlan: null, plan: null },
} });
export const view = (patch: Partial<OrchestratorView> = {}): OrchestratorView => ({
  projectId: "p-1", revision: 3, context: context(), turns: [], nextAction: { id: "design", label: "Review design", reason: "The saved definition is approved; review the design handoff.", lens: "design" }, blockingQuestions: [], hasEarlier: false, beforeSequence: 0, storageDigest: "private-storage-digest", archivePresent: false, ...patch,
});
export const job = (patch: Partial<OrchestratorJob> = {}): OrchestratorJob => ({ id: "job-2", projectId: "p-1", kind: "orchestrator", status: "queued", message: "Waiting", orchestratorTurnId: "turn-2", orchestratorContext: context(), createdAt: "2026-10-06T14:00:00Z", updatedAt: "2026-10-06T14:00:00Z", ...patch });
