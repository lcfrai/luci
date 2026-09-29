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
// F5 fix round 6 (SDD/f5-fix-review-6.md) added what none of those pinned:
//   I1  the BOUNDS. The pinned window is the check's only product assertion, and cutting it from
//       30 samples to 1 kept every test here green while turning a caught defect into a PASS
//       (mutant MB). The window, step 1's follow wait and the descents' budget are pinned below,
//       each beside the sentence that says why it is that number;
//   M1  the settle's ORDER: `key === filling` was asserted, and a hoisted `filling = key` made it
//       a tautology without breaking the match (mutant MA);
//   M2  `pinnedSamples` went: the loop runs 30 times or throws, so the count was 30 on every path
//       that exported it — the same constant, counted instead of written down.
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
    const wait = before.match(/await until\(\s*"([^"]*screen[^"]*)",\s*async \(\) => \{([\s\S]{0,900}?)\}, *([\d_]+)\)/);
    if (wait === null) throw new Error("no screen-level wait stands between the burst and settledBottom");
    const probe = wait[2];
    // Two identical samples: THIS probe's screen against the PREVIOUS probe's, which lives outside
    // the closure — the same "nothing is moving any more" test untilAtBottom makes. Matching
    // `key === x` is not that on its own: hoisting `x = key` above the comparison keeps the match
    // and makes it a tautology (review 6, mutant MA). So the ORDER is the assertion: the
    // comparison reads the old sample, and only after it is the old sample overwritten.
    const compared = probe.match(/const settled = [^;]*\bkey === (\w+)/);
    if (compared === null) throw new Error("the settle does not compare this sample against an earlier one");
    const previous = compared[1];
    expect(before.slice(0, before.indexOf(wait[0]))).toMatch(new RegExp(`\\blet ${previous} = null;`));
    const readAt = probe.indexOf(compared[0]);
    expect(probe.slice(0, readAt)).not.toMatch(new RegExp(`\\b${previous} = `));
    expect(probe.indexOf(`${previous} = key;`)).toBeGreaterThan(readAt);
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

  it("exports no count the loop bound already fixes", () => {
    // The pinned loop runs 30 times or throws, so a sample count is 30 on every path that reaches
    // `observed`: round 5's deleted literal, counted instead of written down (review 6, Minor 2).
    // The bound is pinned in source below; what varies travels as `pinnedMs` and `pinnedGrowthBytes`.
    const start = body.indexOf("observed = {");
    const fields = body.slice(start, body.indexOf("};", start));
    expect(start).toBeGreaterThan(0);
    expect(fields).toMatch(/\bpinnedMs\b/);
    expect(fields).toMatch(/\bpinnedGrowthBytes\b/);
    expect(body).not.toMatch(/\bpinnedSamples\b/);
  });
});

describe("c4.pinned's bounds are its claim, so each is pinned beside the reason it is that number", () => {
  it("samples the scrolled-up viewport 30 times, 100 ms apart, each against the frozen screen", () => {
    // Why 30 × 100 ms. This loop is c4.pinned's ONLY product assertion, so its length IS the
    // claim: "scrolled up, the viewport does not move while output arrives" holds for exactly as
    // long as it is sampled. 100 ms is the registry's flush interval (FLUSH_INTERVAL_MS,
    // attachRegistry.ts:17, one term.write per flush), so there is a sample per flush that could
    // have moved the pane; 30 of them are the three seconds step 2 names, up to 60 ticks of the
    // loop's 50 ms. A defect that is not instantaneous — a re-arming timer, a debounce, a rule that
    // holds for two seconds and then yanks — is caught only inside that window: review 6's
    // scenario 12 REDs at sample 18, and a one-sample window (mutant MB, `i < 1`) PASSES it. A
    // shorter window is how a slow host buys a false green, so the change has to come through here.
    const from = body.indexOf("const pinnedFrom");
    const to = body.indexOf("const pinnedMs");
    expect(from).toBeGreaterThan(0);
    expect(to).toBeGreaterThan(from);
    expect(body.slice(from, to)).toMatch(new RegExp([
      String.raw`for \(let i = 0; i < 30; i \+= 1\) \{`,
      String.raw`await sleep\(100\);`,
      String.raw`const now = await screen\(h\.page, id\);`,
      String.raw`if \(now\.join\("\\n"\) === frozen\.join\("\\n"\)\) continue;`,
      String.raw`throw new Error\(\x60the pinned rule failed: `,
    ].join(String.raw`\s*`)));
    // A window with no output under it proves nothing either way, and without this red it PASSES.
    expect(body.slice(to)).toMatch(/if \(pinnedGrowthBytes <= 0\) throw new Error\("no output arrived while pinned/);
  });

  it("gives step 1 three seconds to see the bottom viewport follow, before step 2 scrolls up", () => {
    // Why step 1 is pinned at all: it is what makes step 2 falsifiable. A pane that has stopped
    // drawing never moves either, and `pinnedGrowthBytes` counts the WIRE, so without a
    // screen-level proof that the bottom viewport is drawing the ticks, step 2 passes on a dead
    // pane. Why 3_000: xterm follows a bottom viewport by itself (f5-evidence/r3-xterm.json), so
    // three seconds, up to 60 ticks, is headroom for a loaded host, and its red is a precondition
    // (task-25.md's rubric), never a product result. Pinned so that the number is changed here,
    // beside this sentence, and not wherever a slow host makes it red.
    const follow = body.search(/await until\("the viewport to follow new output", async \(\) => \(await lastTick\(\)\) !== atStart, 3_000\);/);
    expect(follow).toBeGreaterThan(0);
    expect(follow).toBeLessThan(body.indexOf("const frozen"));
  });

  it("gives every descent 40 wheel events inside a 10 s deadline, and no call site moves either", () => {
    // Why 40 and 10_000. One wheel event moves five rows at any delta (f5-diagnosis.md:109,
    // measured). The longest walk here is back past the ticks that arrived under a scrolled-up
    // viewport: about 65 rows after a three-second window (f5-diagnosis.md:10), 13 events — so 40
    // is 200 rows of room. And 40 × 100 ms of sleep is 4 s inside a 10 s deadline, so a descent that
    // cannot arrive ends in the COUNT red, the one that carries rows want/got and both tails, not in
    // the bare timeout. Raise the count past the deadline and a stuck descent loses its tails;
    // lower it below the rows a descent walks and a working wheel reads as a red.
    expect(helper).toMatch(/\{ expect: target = null, maxEvents = 40, ms = 10_000, what = /);
    expect(helper).toMatch(/for \(let events = 0; events < maxEvents; events \+= 1\)/);
    const calls = stripped(source).match(/await untilAtBottom\([\s\S]*?\);/g) ?? [];
    expect(calls.length).toBeGreaterThanOrEqual(3);
    for (const call of calls) expect(call).not.toMatch(/\bmaxEvents\b|\bms\s*:/);
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
