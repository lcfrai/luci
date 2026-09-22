// desktop/src/lib/ids.ts
//
// Ids the CLIENT owns. Every id the user can see comes from the server
// (control-plane/internal/ids, crypto/rand), so this file exists for the two
// things the server never names: toasts, and React keys for transient rows.
// Contract §3.8.

let counter = 0;

/** Monotonic within a window, unique across windows. The counter alone collides across two
 *  windows sharing localStorage; randomUUID alone is fine but 36 chars in every React key. */
export function makeId(prefix: string): string {
  counter += 1;
  return `${prefix}-${counter.toString(36)}-${crypto.randomUUID().slice(0, 8)}`;
}

/** Test seam: resets the counter so a test can assert monotonicity from a known point.
 *  vitest's restoreMocks and clearMocks do not reset module state, so this is the only
 *  way a test gets a known starting point. */
export function __resetIdCounter(): void {
  counter = 0;
}
