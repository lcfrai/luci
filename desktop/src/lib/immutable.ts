// desktop/src/lib/immutable.ts
//
// Contract §3.9. Known defects 7 and 10 are the same mistake in different
// clothes: an index computed against one array and applied to another. These
// helpers make "never doing that" the path of least resistance.
//
// The rule they encode, and it belongs in every task that touches a list:
// NEVER pass an array index across a filter boundary; pass the id.
//
// Every helper that returns an array returns a fresh one, even when nothing
// changed. Returning the input would mean casting away the readonly on the
// parameter and handing the caller a mutable alias to state it does not own.

/** Replaces exactly one element. Known defect 7: the prototype's editAnswer splices with
 *  `splice(i, 1)` and then pushes, which drops every later element. An index outside the
 *  array is a no-op copy — these run inside reducers driven by render events, where a
 *  throw is a blank window and an append is defect 7 in a different hat. */
export function replaceAt<T>(items: readonly T[], index: number, next: T): T[] {
  const out = items.slice();
  if (!Number.isInteger(index) || index < 0 || index >= out.length) return out;
  out[index] = next;
  return out;
}

/** Replaces the element whose key matches, by identity rather than position. Preferred over
 *  replaceAt wherever the element has an id. An unmatched id is a no-op copy and never an
 *  append: upsert is a different operation, and a hidden append here would let a stale poll
 *  response resurrect a session the user just forgot. */
export function replaceById<T extends { id: string }>(items: readonly T[], next: T): T[] {
  const index = items.findIndex((item) => item.id === next.id);
  if (index === -1) return items.slice();
  return replaceAt(items, index, next);
}

/** Moves `id` by `delta` places WITHIN the subsequence `visibleIds`, and returns the full
 *  list reordered to match. Known defect 10: the prototype's `reorder` finds the index in the
 *  FILTERED ready list and applies it to the UNFILTERED worktrees array, so reordering two
 *  ready worktrees swaps two unrelated ones whenever an unready worktree sits between them.
 *  The signature makes the filtered/unfiltered distinction explicit and impossible to
 *  conflate.
 *
 *  Rows that are not visible are fixed anchors: the visible rows permute among the absolute
 *  slots they already occupy. Moves past either end clamp rather than wrap, because the
 *  arrow keys repeat. */
export function moveWithinVisible<T extends { id: string }>(
  all: readonly T[],
  visibleIds: readonly string[],
  id: string,
  delta: number,
): T[] {
  const out = all.slice();

  // The absolute positions the visible rows occupy, ascending, and the rows themselves.
  const visible = new Set(visibleIds);
  const slots: number[] = [];
  const byId = new Map<string, T>();
  for (let i = 0; i < all.length; i += 1) {
    const item = all[i];
    if (visible.has(item.id)) {
      slots.push(i);
      byId.set(item.id, item);
    }
  }

  // A non-integer delta must not reach splice, which coerces NaN to 0 and would
  // silently move the row to the front.
  const step = Math.trunc(delta);
  if (!Number.isFinite(step) || step === 0) return out;

  const from = visibleIds.indexOf(id);
  if (from === -1 || !byId.has(id)) return out;

  const to = Math.min(Math.max(from + step, 0), visibleIds.length - 1);
  if (to === from) return out;

  const order = visibleIds.slice();
  order.splice(from, 1);
  order.splice(to, 0, id);

  let slot = 0;
  for (const orderedId of order) {
    if (slot >= slots.length) break;
    const item = byId.get(orderedId);
    if (item === undefined) continue;
    out[slots[slot]] = item;
    slot += 1;
  }
  return out;
}

/** Maps a 1-based position in a FILTERED list back to an entity id, or null. Every keyboard
 *  and palette path uses this instead of indexing an array. */
export function idAtVisiblePosition(
  visibleIds: readonly string[],
  oneBased: number,
): string | null {
  if (!Number.isInteger(oneBased) || oneBased < 1 || oneBased > visibleIds.length) return null;
  return visibleIds[oneBased - 1];
}
