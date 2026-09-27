// Task 25 fix round, review Important 1: `d.reload` closes known defect 6, so it must not pass
// without having measured it. The rule lives in desktop/e2e/defect6.mjs (pure); the harness's
// wiring to it is read from disk below, the way seams.test.ts reads its anchors, because the
// harness itself runs `main()` on import and needs a real browser.

import { beforeAll, describe, expect, it } from "vitest";
import { DEFECT6_UNCHECKED, TILE_TOLERANCE_PX, tileKeptSize } from "../e2e/defect6.mjs";

const HARNESS = import.meta.glob<string>("../e2e/phase2.e2e.mjs", { query: "?raw", import: "default" });

const tile = { id: "s-1", width: 700, height: 480 };
const measured = (width: number, height: number) => async () => ({ width, height });

describe("tileKeptSize (known defect 6 in a real window)", () => {
  it("fails, naming defect 6 as not checked, when c5.resize left no tile", async () => {
    let measuredAnything = false;
    const measure = async () => { measuredAnything = true; return { width: 1, height: 1 }; };
    await expect(tileKeptSize(undefined, measure, "3 re-attached")).rejects.toThrow(DEFECT6_UNCHECKED);
    await expect(tileKeptSize(undefined, measure, "3 re-attached")).rejects.toThrow(/^3 re-attached; but /);
    expect(measuredAnything).toBe(false);
  });

  it("passes, returning what it measured, when the tile came back at its size", async () => {
    const back = { width: tile.width + TILE_TOLERANCE_PX, height: tile.height - TILE_TOLERANCE_PX };
    await expect(tileKeptSize(tile, measured(back.width, back.height), "x")).resolves.toEqual({ id: "s-1", ...back });
  });

  it("fails when either dimension came back more than the tolerance off", async () => {
    const over = TILE_TOLERANCE_PX + 1;
    await expect(tileKeptSize(tile, measured(tile.width - over, tile.height), "x")).rejects.toThrow(/defect 6: the resized tile came back/);
    await expect(tileKeptSize(tile, measured(tile.width, tile.height + over), "x")).rejects.toThrow(/defect 6: the resized tile came back/);
  });

  it("measures the tile c5.resize resized, not another", async () => {
    const asked: string[] = [];
    await tileKeptSize(tile, async (id) => { asked.push(id); return tile; }, "x");
    expect(asked).toEqual(["s-1"]);
  });
});

describe("d.reload is wired to it", () => {
  let body = "";
  beforeAll(async () => {
    const load = Object.values(HARNESS)[0];
    if (load === undefined) throw new Error("desktop/e2e/phase2.e2e.mjs was not found");
    const source = (await load()).replace(/\r\n/g, "\n");
    const start = source.indexOf('check("d.reload",');
    const end = source.indexOf('\ncheck("', start + 1);
    if (start < 0 || end < 0) throw new Error("d.reload's check was not found in the harness");
    body = source.slice(start, end).replace(/\/\/[^\n]*/g, "");
  });

  it("hands c5.resize's tile to tileKeptSize unconditionally", () => {
    expect(body).toMatch(/await tileKeptSize\(ctx\.resizedTile,/);
    expect(body).not.toMatch(/ctx\.resizedTile\s*[!=]==/);
    expect(body).not.toMatch(/\bif\s*\(\s*!?\s*ctx\.resizedTile|ctx\.resizedTile\s*(\?\?|&&|\|\|)/);
  });

  it("does not swallow its rejection", () => {
    expect(body).not.toMatch(/\bcatch\b|\btry\b/);
  });
});
