import { describe, expect, it } from "vitest";

import type { SessionId } from "../store/types";
import {
  idAtVisiblePosition,
  moveWithinVisible,
  replaceAt,
  replaceById,
} from "./immutable";

interface Row {
  id: SessionId;
  label: string;
}

const row = (id: string): Row => ({ id, label: id.toUpperCase() });
const ids = (rows: readonly Row[]): string[] => rows.map((r) => r.id);

describe("replaceAt", () => {
  it("replaces one element and keeps every later one — known defect 7", () => {
    const items = [row("a"), row("b"), row("c"), row("d")];
    const out = replaceAt(items, 1, { id: "b", label: "EDITED" });
    expect(ids(out)).toEqual(["a", "b", "c", "d"]);
    expect(out[1].label).toBe("EDITED");
  });

  it("returns a new array and leaves the input untouched", () => {
    const items = [row("a"), row("b")];
    const out = replaceAt(items, 0, { id: "a", label: "EDITED" });
    expect(out).not.toBe(items);
    expect(items[0].label).toBe("A");
  });

  it("makes an out-of-range index a no-op copy, never a throw and never an append", () => {
    const items = [row("a"), row("b")];
    for (const index of [-1, 2, 99, 1.5, NaN]) {
      const out = replaceAt(items, index, { id: "z", label: "Z" });
      expect(ids(out)).toEqual(["a", "b"]);
      expect(out).not.toBe(items);
    }
  });
});

describe("replaceById", () => {
  it("replaces by identity rather than by position", () => {
    const items = [row("a"), row("b"), row("c")];
    const out = replaceById(items, { id: "c", label: "EDITED" });
    expect(ids(out)).toEqual(["a", "b", "c"]);
    expect(out[2].label).toBe("EDITED");
  });

  it("never appends an unmatched element, because upsert is a different operation", () => {
    const items = [row("a"), row("b")];
    const out = replaceById(items, { id: "ghost", label: "GHOST" });
    expect(ids(out)).toEqual(["a", "b"]);
    expect(out).not.toBe(items);
  });
});

describe("moveWithinVisible", () => {
  // "b" is filtered out. What happens to it is the whole of known defect 10.
  const all = [row("a"), row("b"), row("c"), row("d")];
  const visible = ["a", "c", "d"];

  it("moves within the filtered subsequence and anchors the hidden row — known defect 10", () => {
    const out = moveWithinVisible(all, visible, "a", 1);
    expect(ids(out)).toEqual(["c", "b", "a", "d"]);
    // The prototype's bug produced this instead: an index from the filtered list
    // applied to the unfiltered array, swapping "a" with a row the user cannot see.
    expect(ids(out)).not.toEqual(["b", "a", "c", "d"]);
  });

  it("moves upwards through a hidden row without disturbing it", () => {
    const out = moveWithinVisible(all, visible, "d", -2);
    expect(ids(out)).toEqual(["d", "b", "a", "c"]);
  });

  it("keeps two hidden rows anchored at their own absolute positions", () => {
    const wide = [row("h1"), row("a"), row("h2"), row("b"), row("c")];
    const out = moveWithinVisible(wide, ["a", "b", "c"], "c", -2);
    expect(ids(out)).toEqual(["h1", "c", "h2", "a", "b"]);
  });

  it("reorders correctly when nothing is filtered out", () => {
    const out = moveWithinVisible(all, ["a", "b", "c", "d"], "c", -2);
    expect(ids(out)).toEqual(["c", "a", "b", "d"]);
  });

  it("clamps at the top rather than wrapping, because arrow keys repeat", () => {
    expect(ids(moveWithinVisible(all, visible, "a", -1))).toEqual(["a", "b", "c", "d"]);
    expect(ids(moveWithinVisible(all, visible, "a", -99))).toEqual(["a", "b", "c", "d"]);
  });

  it("clamps at the bottom rather than wrapping", () => {
    expect(ids(moveWithinVisible(all, visible, "d", 5))).toEqual(["a", "b", "c", "d"]);
  });

  it("is a no-op copy for a zero, fractional or non-finite delta", () => {
    for (const delta of [0, 0.4, NaN, Infinity]) {
      const out = moveWithinVisible(all, visible, "a", delta);
      expect(ids(out)).toEqual(["a", "b", "c", "d"]);
      expect(out).not.toBe(all);
    }
  });

  it("is a no-op when the id is filtered out, so a keypress on a hidden row does nothing", () => {
    expect(ids(moveWithinVisible(all, visible, "b", 1))).toEqual(["a", "b", "c", "d"]);
  });

  it("is a no-op when a visible id names a row that is not in the list at all", () => {
    const out = moveWithinVisible(all, [...visible, "ghost"], "ghost", -1);
    expect(ids(out)).toEqual(["a", "b", "c", "d"]);
  });
});

describe("idAtVisiblePosition", () => {
  const visible = ["c", "a", "d"];

  it("maps a 1-based position back to an id across a filter boundary", () => {
    expect(idAtVisiblePosition(visible, 1)).toBe("c");
    expect(idAtVisiblePosition(visible, 3)).toBe("d");
  });

  it("returns null outside the range rather than undefined", () => {
    for (const n of [0, -1, 4, 99]) expect(idAtVisiblePosition(visible, n)).toBeNull();
  });

  it("returns null for an empty list, so a keypress on nothing selects nothing", () => {
    expect(idAtVisiblePosition([], 1)).toBeNull();
  });

  it("returns null for a fractional or non-finite position", () => {
    for (const n of [1.5, NaN, Infinity]) expect(idAtVisiblePosition(visible, n)).toBeNull();
  });
});
