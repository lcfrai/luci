// desktop/src/lib/time.ts
//
// Contract §7.4, which quotes docs/AGENTIC-UX.md §10 rule 9: "No 'N ago' as a
// primary timestamp. Primary is 'since you last looked' or 'waiting Xh Ym'."
//
// Both functions take nowMs rather than calling Date.now() — so they are pure,
// so a test needs no fake timers, and so every caller in one render tick prints
// a consistent number. nowMs comes from useHeartbeat().
//
// Formatting rule, which generates every example in §7.4: the leading unit is
// unpadded and the trailing unit is zero-padded to two. Everything floors, so a
// duration is never shown before it has elapsed.

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

/** Rendered when a timestamp is absent or unparseable. "0m" would claim the session
 *  started now, "NaN m" ships the bug to the user, and throwing blanks the pane. */
const UNKNOWN = "—"; // em dash

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

/** "41m", "1h 06m", "1d 03h", "2d". For how long a session has been alive. */
export function formatElapsed(fromIso: string, nowMs: number): string {
  const fromMs = Date.parse(fromIso);
  if (!Number.isFinite(fromMs) || !Number.isFinite(nowMs)) return UNKNOWN;

  // Clock skew between the machine and the control plane must not print a minus sign.
  const elapsed = Math.max(0, nowMs - fromMs);

  if (elapsed < HOUR_MS) {
    return `${Math.floor(elapsed / MINUTE_MS)}m`;
  }
  if (elapsed < DAY_MS) {
    const hours = Math.floor(elapsed / HOUR_MS);
    const minutes = Math.floor((elapsed % HOUR_MS) / MINUTE_MS);
    return `${hours}h ${pad2(minutes)}m`;
  }
  const days = Math.floor(elapsed / DAY_MS);
  const hours = Math.floor((elapsed % DAY_MS) / HOUR_MS);
  return hours === 0 ? `${days}d` : `${days}d ${pad2(hours)}h`;
}

/** "waiting 5h 51m". For how long something has been idle without the user. Aging must be
 *  legible or wasted idle time is invisible. */
export function formatWaiting(sinceIso: string, nowMs: number): string {
  const elapsed = formatElapsed(sinceIso, nowMs);
  return elapsed === UNKNOWN ? UNKNOWN : `waiting ${elapsed}`;
}
