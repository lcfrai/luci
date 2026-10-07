import { expect, it, vi } from "vitest";
import { dataEmailApi } from "./dataEmail";
import { productApi } from "./product";
vi.mock("./product", () => ({ productApi: { request: vi.fn().mockResolvedValue({}) } }));
it("uses the selected project and the product client's structured POST contract", async () => {
  await dataEmailApi.status("project/owned");
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fowned/data/email");
  const input = { action: "check" as const, expectedRevision: 2 };
  await dataEmailApi.action("project/owned", input);
  expect(productApi.request).toHaveBeenLastCalledWith("/v1/platform/projects/project%2Fowned/data/email", input);
});
