import { expect, it } from "vitest";
import { ApiError } from "./client";
import { codexConnectionError } from "./codexConnectionErrors";

it("keeps a failed transport separate from device grant and save outcome", () => {
  for (const code of ["network_unavailable", "timeout"] as const) {
    const cause = new ApiError(code, "generic transport", 0);
    expect(codexConnectionError(cause, "save")).toContain("Saving this sign-in is unconfirmed");
    expect(codexConnectionError(cause, "save")).toContain("Keep the session and refresh");
    expect(codexConnectionError(cause, "start")).toContain("Refresh your sessions before requesting another device code");
    expect(codexConnectionError(cause, "status")).toContain("does not show whether device sign-in succeeded");
  }
});

it("preserves service rejection guidance and ordinary nontransport errors", () => {
  const rejection = new ApiError("conflict", "No complete device grant recorded.", 409);
  expect(codexConnectionError(rejection, "save")).toBe(rejection.message);
  expect(codexConnectionError(new Error("Stop is uncertain."), "save")).toBe("Stop is uncertain.");
});
