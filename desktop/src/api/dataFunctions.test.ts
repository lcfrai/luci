import { expect, it, vi } from "vitest";
import { dataFunctionsApi } from "./dataFunctions";
import { productApi } from "./product";
vi.mock("./product", () => ({ productApi: { request: vi.fn().mockResolvedValue({}) } }));

it("keeps revision, source concurrency and application session credentials in the structured request", async () => {
  await dataFunctionsApi.catalog("project/one");
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fone/data/functions");
  const draft = { expectedDigest: "saved-digest", authMode: "authenticated" as const, files: [{ path: "index.ts", content: "source" }] };
  await dataFunctionsApi.save("project/one", "welcome", draft);
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fone/data/functions/welcome/source", draft);
  await dataFunctionsApi.action("project/one", "welcome", { action: "publish", expectedRevision: 7, commit: "head", acknowledgeExecute: true });
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fone/data/functions/welcome/action", { action: "publish", expectedRevision: 7, commit: "head", acknowledgeExecute: true });
  await dataFunctionsApi.invoke("project/one", "welcome", { name: "A" }, "app-session");
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fone/data/functions/welcome/invoke", { input: { name: "A" }, applicationAccessToken: "app-session" });
  await dataFunctionsApi.invoke("project/one", "hello", {});
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fone/data/functions/hello/invoke", { input: {} });
});
