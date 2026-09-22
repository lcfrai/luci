import { describe, expect, it } from "vitest";

import { __resetIdCounter, makeId } from "./ids";

describe("makeId", () => {
  it("joins the caller's prefix, a base-36 counter and eight hex characters", () => {
    __resetIdCounter();
    expect(makeId("toast")).toMatch(/^toast-1-[0-9a-f]{8}$/);
  });

  it("counts in base 36, so the counter stays one character until 36", () => {
    __resetIdCounter();
    let last = "";
    for (let i = 0; i < 35; i += 1) last = makeId("t");
    expect(last.split("-")[1]).toBe("z");
    expect(makeId("t").split("-")[1]).toBe("10");
  });

  it("is monotonic from a known point, which is the whole of what __resetIdCounter buys", () => {
    __resetIdCounter();
    const counters = [makeId("t"), makeId("t"), makeId("t")].map((id) => id.split("-")[1]);
    expect(counters).toEqual(["1", "2", "3"]);
  });

  it("does not collide on fast successive calls — known defect 2", () => {
    __resetIdCounter();
    const ids = new Set<string>();
    for (let i = 0; i < 2000; i += 1) ids.add(makeId("row"));
    expect(ids.size).toBe(2000);
  });

  it("stays unique across a counter reset, because the suffix is random and the counter is not", () => {
    __resetIdCounter();
    const first = makeId("t");
    __resetIdCounter();
    const second = makeId("t");
    expect(second.split("-")[1]).toBe("1");
    expect(second).not.toBe(first);
  });

  it("leaves a prefix containing a hyphen intact, because nothing ever parses the prefix back out", () => {
    __resetIdCounter();
    expect(makeId("ctx-menu").startsWith("ctx-menu-1-")).toBe(true);
  });
});
