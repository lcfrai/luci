import { expect, it } from "vitest";
import { prototypeScreenRoute } from "./PrototypeReview";

it.each(["https://elsewhere.example/", "//elsewhere.example/", "/\\elsewhere.example/", "/notes\n", "/.well-known/luci-team-access?ticket=other", "/notes/:id", "/notes/[id]", "notes"])("does not offer an unbound or unresolved screen route: %s", route => {
  expect(prototypeScreenRoute(route)).toBeNull();
});

it("preserves a concrete application route including its query and anchor", () => {
  expect(prototypeScreenRoute("/notes?filter=saved#detail")).toBe("/notes?filter=saved#detail");
  expect(prototypeScreenRoute("/")).toBe("/");
});
