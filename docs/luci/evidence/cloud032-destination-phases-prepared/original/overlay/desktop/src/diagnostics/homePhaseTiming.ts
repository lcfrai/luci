// Diagnostic overlay only. No production optimization or stored data change.
type Token = { id: number; start: number } | null;
type Row = { sequence: number; home: number; kind: number; start: number; end: number; ok: number; count?: number };
const enabled = typeof window !== "undefined" && (window as unknown as { __luciHomePhaseRequested?: number }).__luciHomePhaseRequested === 1;
let armed = false, home = 0, active = 0, pending = 0, sequence = 0, dropped = 0;
const starts = new Map<number, number>();
let rows: Row[] = [];
function record(id: number, kind: number, start: number, end: number, ok = 1, count?: number) {
  if (!id) return;
  sequence++;
  if (rows.length >= 2048) { dropped++; return; }
  rows.push({ sequence, home: id, kind, start, end, ok, ...(count === undefined ? {} : { count }) });
}
export function homeHandlerBegin(): number {
  if (!enabled || !armed) return 0;
  if (pending) dropped++;
  const id = ++home; active = pending = id; starts.set(id, performance.now());
  if (starts.size > 16) { starts.delete(starts.keys().next().value!); dropped++; }
  return id;
}
export function homeHandlerEnd(id: number): void {
  if (!id) return;
  record(id, 1, starts.get(id)!, performance.now());
  if (active === id) active = 0;
}
export function homeRenderBegin(): Token {
  const id = enabled && armed ? active || pending : 0;
  return id ? { id, start: performance.now() } : null;
}
export function homeCommit(token: Token): void {
  if (!token || pending !== token.id) return;
  const now = performance.now();
  record(token.id, 2, starts.get(token.id)!, now);
  record(token.id, 12, token.start, now);
  pending = 0;
}
export function homePhase<T>(kind: number, operation: () => T): T {
  const id = enabled && armed ? active || pending : 0;
  if (!id) return operation();
  const start = performance.now(); let ok = 0;
  try { const value = operation(); ok = 1; return value; }
  finally { record(id, kind, start, performance.now(), ok); }
}
export function homeCount(kind: number, count: number): void {
  const id = enabled && armed ? active || pending : 0;
  if (id) { const now = performance.now(); record(id, kind, now, now, 1, count); }
}
if (enabled) Object.defineProperty(window, "__luciHomePhase", { configurable: false, value: {
  arm() { if (armed || home || rows.length) throw Error("Home diagnostic already armed"); armed = true; },
  snapshot() { const result = { version: 1, timeOrigin: performance.timeOrigin, homes: home, observed: sequence, dropped, pending, active, rows }; rows = []; return result; },
} });
