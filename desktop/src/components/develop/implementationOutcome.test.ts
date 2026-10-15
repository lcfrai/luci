import { describe, expect, it } from "vitest";
import { aSession } from "../../test/fixtures";
import { implementationOutcomeMessage } from "./implementationOutcome";

describe("native implementation turn guidance", () => {
  it("does not erase independent project-run checks when a read-only native turn finishes", () => {
    const session = aSession({ harness: "opencode", status: "idle", implementationContext: { projectId: "p", worktreeId: "w", taskIds: ["task"], briefRevision: 1, planRevision: 1, specificationRevision: 1, designPlanRevision: 1, briefDigest: "b", planDigest: "p", specificationDigest: "s", designPlanDigest: "d", sourceCommit: "", sourceDigest: "source", sourceDirty: false, taskTitles: ["Review"], requirementIds: [] }, implementationOutcome: { state: "quiet", evidence: "project-run", nativeTurnId: "msg_turn1", nativeToolClass: "reads-only" } });
    const message = implementationOutcomeMessage(session);
    expect(message).toContain("Review its retained work and run checks");
    expect(message).not.toContain("no implementation or checks are linked");
  });
});
