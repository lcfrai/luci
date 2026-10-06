import { afterEach, expect, it, vi } from "vitest";
import { definitionApi, type DefinitionAssistance, type RepositoryAnalysisRequest } from "./definition";
import { productApi } from "./product";

const input: RepositoryAnalysisRequest = { expectedRevision: 3, expectedBriefRevision: 4, sourceMode: "worktree", harness: "codex", model: "chosen-model", agentConnectionId: "chosen-connection", expectedAgentConnectionRevision: 7, acknowledgeSourceRead: true, message: "Keep the existing accounts", replyToAnalysisId: "earlier-analysis", idempotencyKey: "analysis-exact-original-key" };
const receipt: DefinitionAssistance = { id: "analysis-two", projectId: "p-1", requestedBy: "actor", ownRequest: true, kind: "repository", status: "reserved", message: "Preparing request", createdAt: "now", updatedAt: "now", idempotencyKey: input.idempotencyKey, replyToAnalysisId: input.replyToAnalysisId };
afterEach(() => vi.restoreAllMocks());

it("submits the exact reply and performs read-only original-key recovery", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(receipt);
  expect(await definitionApi.analyseRepository("p-1", input)).toEqual(receipt);
  expect(request.mock.calls[0]).toEqual(["/v1/product/projects/p-1/assist/repository", input]);
  expect(await definitionApi.repositoryRequest("p-1", input.idempotencyKey!)).toEqual(receipt);
  expect(request.mock.calls[1]).toEqual(["/v1/product/projects/p-1/assist/repository/requests/analysis-exact-original-key"]);
});

it("rejects foreign or mismatched receipts and malformed keys without replay", async () => {
  const request = vi.spyOn(productApi, "request");
  for (const changed of [{ projectId: "other" }, { ownRequest: false }, { ownRequest: undefined }, { idempotencyKey: "another-original-key" }, { kind: "interview" }, { id: "" }, { status: "invented" }]) {
    request.mockResolvedValue({ ...receipt, ...changed });
    await expect(definitionApi.repositoryRequest("p-1", input.idempotencyKey!)).rejects.toThrow("could not be verified");
  }
  const calls = request.mock.calls.length;
  await expect(definitionApi.repositoryRequest("p-1", "../../invalid-key")).rejects.toThrow("identity is invalid");
  expect(request).toHaveBeenCalledTimes(calls);
  request.mockRejectedValue(new Error("Connection unavailable"));
  await expect(definitionApi.analyseRepository("p-1", input)).rejects.toThrow("Connection unavailable");
  expect(request).toHaveBeenCalledTimes(calls + 1);
});
