// desktop/e2e/defect6.mjs
// Task 25 fix round (review Important 1). `d.reload` closes two things: every live session
// re-attaches after a reload, and known defect 6 in a real window (a hand-resized Canvas tile
// comes back at the size it was given). The first version checked defect 6 only when
// `c5.resize` had left a tile behind, and passed without it otherwise, printing
// `tileKeptSize: false` — which read as "the tile lost its size" and meant "not looked at".
// Two runs on 2026-09-27 printed exactly that PASS. The rule here: a check that names
// defect 6 cannot pass without having measured it.
//
// Kept apart from phase2.e2e.mjs because that file runs `main()` on import and needs a real
// browser; this one is pure, so desktop/src/e2eDefect6.test.ts can hold the rule.

/** Why the check failed when nothing was resized. Exported so the test names the same text. */
export const DEFECT6_UNCHECKED =
  "known defect 6 not checked: c5.resize did not pass (or did not run), so no tile was resized to compare";

/** A tile's rendered size may differ by sub-pixel rounding after a reload, and no more. */
export const TILE_TOLERANCE_PX = 1;

/**
 * Throws unless the tile `resized` names comes back, as `measure(id)` reports it after the
 * reload, within TILE_TOLERANCE_PX of the size `resized` recorded, in both dimensions.
 * `resized` is what c5.resize left in the shared context (undefined when it did not pass).
 * `already` is what the calling check had proven before this point; it leads every error,
 * so a failure here does not hide the half that held.
 * @param {{ id: string, width: number, height: number } | undefined} resized
 * @param {(id: string) => Promise<{ width: number, height: number }>} measure
 * @param {string} already
 * @returns {Promise<{ id: string, width: number, height: number }>}
 */
export async function tileKeptSize(resized, measure, already) {
  if (resized === undefined) throw new Error(`${already}; but ${DEFECT6_UNCHECKED}`);
  const tile = await measure(resized.id);
  const off = (a, b) => Math.abs(a - b) > TILE_TOLERANCE_PX;
  if (off(tile.width, resized.width) || off(tile.height, resized.height)) {
    throw new Error(
      `${already}; but defect 6: the resized tile came back ${tile.width}x${tile.height}, was ${resized.width}x${resized.height}`,
    );
  }
  return { id: resized.id, width: tile.width, height: tile.height };
}
