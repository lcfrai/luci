import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { projectContextApi } from "./projectContext";
afterEach(() => vi.restoreAllMocks());
it("uses encoded project paths and explicit compare-and-swap bodies for context mutations", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue({});
  await projectContextApi.get("project/space");
  await projectContextApi.repair("project/space", "reviewed-digest");
  await projectContextApi.archive("project/space", "page digest");
  await projectContextApi.prune("project/space", "backed-up-digest");
  expect(request.mock.calls).toEqual([
    ["/v1/product/projects/project%2Fspace/context"],
    ["/v1/product/projects/project%2Fspace/context/exports/repair", { expectedDigest: "reviewed-digest" }],
    ["/v1/product/projects/project%2Fspace/context/archive?cursor=page%20digest"],
    ["/v1/product/projects/project%2Fspace/context/archive/prune", { expectedDigest: "backed-up-digest", acknowledgeArchivedHistoryRemoval: true }],
  ]);
});
