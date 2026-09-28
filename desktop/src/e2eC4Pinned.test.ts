// Task 25 fix round, review round 5. `c4.pinned` needs a real browser and the harness runs `main()`
// on import, so its shape is asserted against its SOURCE, the way e2eDefect6.test.ts asserts
// `d.reload`'s wiring. Every property below is a defect this round fixed, and each one of these
// tests fails against the commit under review:
//   I1  the screen question decided after WIRE-only waits (`stdoutOf` says nothing about what xterm
//       has parsed), which let the PARSE satisfy step 0 and handed the descent a stale bottom;
//   I2  the endless TICK loop started OUTSIDE the try, so one timeout returned from the check with
//       the loop still running in the shared shell — F5's own cascade;
//   M1  `pinnedSamples: 30`, a literal that is true whenever the line is reached;
//   M2  three descents, one error string.
// Comments are stripped before every assertion: a rule that a comment can satisfy is not a rule.

import { beforeAll, describe, expect, it } from "vitest";

const HARNESS = import.meta.glob<string>("../e2e/phase2.e2e.mjs", { query: "?raw", import: "default" });

const stripped = (s: string) => s.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

let source = "";
let body = "";
let helper = "";

beforeAll(async () => {
  const load = Object.values(HARNESS)[0];
  if (load === undefined) throw new Error("desktop/e2e/phase2.e2e.mjs was not found");
  source = (await load()).replace(/\r\n/g, "\n");
  const start = source.indexOf('check("c4.pinned",');
  const end = source.indexOf('\ncheck("', start + 1);
  if (start < 0 || end < 0) throw new Error("c4.pinned's check was not found in the harness");
  body = stripped(source.slice(start, end));
  const hStart = source.indexOf("async function untilAtBottom");
  const hEnd = source.indexOf("\nconst CHECKS", hStart);
  if (hStart < 0 || hEnd < 0) throw new Error("untilAtBottom was not found in the harness");
  helper = stripped(source.slice(hStart, hEnd));
});

describe("c4.pinned decides its screen questions on the screen", () => {
  it("waits for the burst to reach the SCREEN before it samples the bottom step 0 compares against", () => {
    const at = body.indexOf("const settledBottom");
    expect(at).toBeGreaterThan(0);
    const before = body.slice(0, at);
    // The wire waits are not enough on their own: `stdoutOf` is the raw transcript.
    expect(before).toMatch(/screen\(h\.page, id\)/);
    const wait = before.match(/await until\(\s*"([^"]*screen[^"]*)",\s*async \(\) => \{([\s\S]{0,900}?)\}, *([\d_]+)\)/);
    if (wait === null) throw new Error("no screen-level wait stands between the burst and settledBottom");
    expect(wait[2]).toMatch(/screen\(h\.page, id\)/);
    expect(wait[2]).toMatch(/PAD3000/);
  });

  it("samples that screen twice before trusting it, so a mid-parse screen cannot settle", () => {
    const at = body.indexOf("const settledBottom");
    const before = body.slice(0, at);
    // Two identical samples: the same "nothing is moving any more" test untilAtBottom makes.
    expect(before).toMatch(/key === \w+/);
  });
});

describe("c4.pinned cannot leak its endless loop into the shared shell", () => {
  it("starts the TICK loop and waits for it INSIDE the try", () => {
    const tryAt = body.indexOf("try {");
    const loopAt = body.indexOf("while :;");
    const ticksAt = body.indexOf('until("ticks arriving"');
    expect(tryAt).toBeGreaterThan(0);
    expect(loopAt).toBeGreaterThan(tryAt);
    expect(ticksAt).toBeGreaterThan(tryAt);
  });

  it("still presses Control+C from the cleanup, after the catch that captures the failure", () => {
    const caught = body.indexOf("} catch (error) {");
    expect(caught).toBeGreaterThan(0);
    expect(body.lastIndexOf("Control+C")).toBeGreaterThan(caught);
    expect(body.indexOf("failure = error")).toBeGreaterThan(0);
  });
});

describe("c4.pinned exports only what the run measured", () => {
  it("has no literal in `observed`", () => {
    const start = body.indexOf("observed = {");
    const end = body.indexOf("};", start);
    expect(start).toBeGreaterThan(0);
    expect(end).toBeGreaterThan(start);
    expect(body.slice(start, end)).not.toMatch(/\b\w+:\s*(?:true|false|\d)/);
  });

  it("counts the pinned samples instead of restating the loop bound", () => {
    expect(body).toMatch(/pinnedSamples \+= 1/);
    expect(body).toMatch(/observed = \{[^}]*\bpinnedSamples\b/);
  });
});

describe("every wheel descent says which descent it was", () => {
  it("names itself in both of untilAtBottom's reds", () => {
    expect(helper).toMatch(/what/);
    const throws = helper.match(/throw new Error\(`[^`]*`/g) ?? [];
    expect(throws.length).toBeGreaterThanOrEqual(2);
    for (const thrown of throws) expect(thrown).toContain("${what}");
  });

  it("is given a name at every call site", () => {
    const calls = stripped(source).match(/await untilAtBottom\([\s\S]*?\);/g) ?? [];
    expect(calls.length).toBeGreaterThanOrEqual(3);
    for (const call of calls) expect(call).toMatch(/what:/);
  });
});
