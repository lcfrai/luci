// The ONE clock. Contract §7.1; docs/DESIGN-SYSTEM.md: "never a spinner per row".
//
// Two functions and no third mechanism. beatDelayStyle() aligns a CSS animation's PHASE to a
// shared epoch so ten dots that mounted at ten different moments breathe together.
// useHeartbeat() is a 1 Hz tick for LIVE NUMBERS ONLY — elapsed, waiting — behind one interval
// for the whole window. EPOCH, BEAT_MS and the __resetHeartbeat seam are exported for the test.
import { useSyncExternalStore } from "react";

/** The shared epoch. Module load time, once per window. */
export const EPOCH = performance.now();

/** Must equal --beat in src/styles/tokens.css. heartbeat.test.ts reads the file and pins it. */
export const BEAT_MS = 2400;

/** Returns the style that aligns a CSS animation's PHASE to the shared epoch.
 *
 *  WHY a negative animation-delay rather than a JS-driven opacity: a CSS animation runs on the
 *  compositor and costs nothing per frame, but it starts its cycle when the ELEMENT mounts — so
 *  ten rows appearing at ten moments breathe out of phase, which is exactly the failure
 *  DESIGN-SYSTEM names. A negative delay of (now - EPOCH) mod BEAT_MS starts the animation
 *  already part-way through its cycle, so every element, whenever it mounted, sits at the same
 *  point of the same breath. One line, zero JS per frame, perfect phase. */
export function beatDelayStyle(): { animationDelay: string } {
  const offset = Math.round(performance.now() - EPOCH) % BEAT_MS;
  return { animationDelay: `-${offset}ms` };
}

let tick = Math.floor(Date.now() / 1000) * 1000;
let timer: ReturnType<typeof setInterval> | null = null;
const listeners = new Set<() => void>();

function start(): void {
  if (timer !== null) return;
  timer = setInterval(() => {
    const next = Math.floor(Date.now() / 1000) * 1000;
    if (next === tick) return;
    tick = next;
    for (const listener of listeners) listener();
  }, 1000);
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  start();
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

/** getSnapshot must be referentially stable across calls with no tick between them, or React
 *  throws "The result of getSnapshot should be cached". Hence the module variable. */
function getSnapshot(): number {
  return tick;
}

/** A 1 Hz tick for LIVE NUMBERS ONLY. Returns epoch-ms rounded down to the second, so two
 *  components rendering in the same tick print the same number. A component that needs neither
 *  this nor beatDelayStyle subscribes to neither: this hook re-renders its caller once a second
 *  and a rail row doing that for 40 rows is a performance defect, not a heartbeat. */
export function useHeartbeat(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** Test seam. Clears the interval and every listener so module state does not leak between
 *  files — vitest's restoreMocks/clearMocks do not reset module state (§1.6). */
export function __resetHeartbeat(): void {
  if (timer !== null) clearInterval(timer);
  timer = null;
  listeners.clear();
  tick = Math.floor(Date.now() / 1000) * 1000;
}
