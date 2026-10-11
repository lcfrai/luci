// Diagnostic overlay only. Product actions, saved data and acceptance stay unchanged.
// Private target/value/object/element bindings exist only for the current action.
export type DestinationReadToken = Readonly<object>;
type EvaluationToken = Readonly<object>;
type SavedDestination = { projectId: string; purpose: string; revision: number; briefRevision: number };
type Control = { tagName: string; isConnected: boolean; value: string };
type Row = { sequence: number; action: number; read: number; kind: number; at: number; connected?: number; match?: number; samples?: number };
type Read = { token: DestinationReadToken; ordinal: number; generation: number; value: SavedDestination | null; purpose: string | null; revision: number; briefRevision: number; element: Control | null };
type Evaluation = { token: EvaluationToken; expected: string; element: Control; samples: number; lastMatch: boolean };
type Action = { ordinal: number; project: string | null; read: Read | null; evaluation: Evaluation | null; done: boolean };
const enabled = typeof window !== "undefined" && (window as unknown as { __luciDestinationPhaseRequested?: number }).__luciDestinationPhaseRequested === 1;
export const DESTINATION_MAX_ACTIONS = 100, DESTINATION_MAX_READS = 100, DESTINATION_MAX_ROWS = 2048, DESTINATION_MAX_SAMPLES = 2048;
let armed = false, current: Action | null = null, rows: Row[] = [];
let actions = 0, destinations = 0, reads = 0, accepted = 0, commits = 0, evaluations = 0, samples = 0, witnesses = 0;
let observed = 0, dropped = 0, errors = 0, overlaps = 0, mismatches = 0;
const active = () => enabled && armed;
const integer = (value: number) => Number.isSafeInteger(value) && value >= 0;
function fail(kind: "overlap" | "mismatch" | "overflow" = "mismatch") {
  errors++;
  if (kind === "overlap") overlaps++;
  else if (kind === "overflow") dropped++;
  else mismatches++;
}
// Diagnostic mistakes never escape into the product or its existing oracle.
function protect<T>(fallback: T, operation: () => T): T {
  try { return operation(); } catch { fail(); return fallback; }
}
function record(action: Action, kind: number, extra: Partial<Pick<Row, "connected" | "match" | "samples">> = {}) {
  const at = performance.now();
  observed++;
  if (!Number.isFinite(at) || observed > DESTINATION_MAX_ROWS) { fail("overflow"); return; }
  rows.push({ sequence: observed, action: action.ordinal, read: action.read?.ordinal ?? 0, kind, at, ...extra });
}
function control(element: unknown): element is Control {
  return !!element && typeof element === "object" && (element as Control).tagName === "TEXTAREA" &&
    typeof (element as Control).isConnected === "boolean" && typeof (element as Control).value === "string";
}
function saved(value: unknown): value is SavedDestination {
  const item = value as SavedDestination | null;
  return !!item && typeof item === "object" && typeof item.projectId === "string" && typeof item.purpose === "string" &&
    integer(item.revision) && integer(item.briefRevision);
}
function stillAccepted(action: Action, read: Read) {
  return read.value !== null && read.value.projectId === action.project && read.value.purpose === read.purpose &&
    read.value.revision === read.revision && read.value.briefRevision === read.briefRevision;
}
export function destinationNavigate(projectId: string | null, lens: string): void {
  if (!active()) return;
  protect<void>(undefined, () => {
    if (projectId === null) {
      if (current && !current.done) fail("overlap");
      if (actions >= DESTINATION_MAX_ACTIONS) { fail("overflow"); current = null; return; }
      current = { ordinal: ++actions, project: null, read: null, evaluation: null, done: false };
      record(current, 1);
      return;
    }
    if (!current || current.done || current.project !== null) { fail("overlap"); return; }
    if (typeof projectId !== "string" || !projectId || lens !== "discover") { fail(); return; }
    current.project = projectId; destinations++; record(current, 2);
  });
}
export function destinationReadBegin(projectId: string, generation: number): DestinationReadToken | null {
  if (!active() || !current || current.done) return null;
  return protect<DestinationReadToken | null>(null, () => {
    if (current!.project === null || projectId !== current!.project || !integer(generation) || generation === 0) { fail(); return null; }
    if (current!.read) { fail("overlap"); return null; }
    if (reads >= DESTINATION_MAX_READS) { fail("overflow"); return null; }
    const token = Object.freeze({});
    current!.read = { token, ordinal: ++reads, generation, value: null, purpose: null, revision: -1, briefRevision: -1, element: null };
    record(current!, 3); return token;
  });
}
export function destinationReadAccepted(token: DestinationReadToken | null, value: unknown): void {
  if (!active() || token === null || current?.done) return;
  protect<void>(undefined, () => {
    const action = current, read = action?.read;
    if (!action || !read || read.token !== token) { fail(); return; }
    if (read.value !== null) { fail("overlap"); return; }
    if (!saved(value) || value.projectId !== action.project || (action.evaluation && value.purpose !== action.evaluation.expected)) { fail(); return; }
    read.value = value; read.purpose = value.purpose; read.revision = value.revision; read.briefRevision = value.briefRevision;
    accepted++; record(action, 4);
  });
}
export function destinationLayoutCommit(token: DestinationReadToken | null, projectId: string, generation: number, spec: unknown, element: unknown): void {
  if (!active() || token === null || current?.done) return;
  protect<void>(undefined, () => {
    const action = current, read = action?.read;
    if (!action || !read || read.token !== token || projectId !== action.project || generation !== read.generation) { fail(); return; }
    // Loading/interview commits can legitimately have no destination control yet.
    if (spec === null || element === null) return;
    if (spec !== read.value || !stillAccepted(action, read) || !control(element) || !element.isConnected || element.value !== read.purpose ||
        (action.evaluation && action.evaluation.element !== element)) { fail(); return; }
    if (read.element !== null) { if (read.element !== element) fail(); return; }
    read.element = element; commits++; record(action, 5);
  });
}
function evaluationEnter(actionOrdinal: number, projectId: string, expectedValue: string, element: unknown): EvaluationToken | null {
  if (!active()) return null;
  return protect<EvaluationToken | null>(null, () => {
    const action = current;
    if (!action || action.done || action.ordinal !== actionOrdinal || action.project === null || projectId !== action.project ||
        typeof expectedValue !== "string" || !control(element)) { fail(); return null; }
    if (action.evaluation) { fail("overlap"); return null; }
    const read = action.read;
    if (read?.value && (!stillAccepted(action, read) || read.purpose !== expectedValue)) { fail(); return null; }
    if (read?.element && read.element !== element) { fail(); return null; }
    const token = Object.freeze({});
    action.evaluation = { token, expected: expectedValue, element, samples: 0, lastMatch: false };
    evaluations++; record(action, 6); return token;
  });
}
function evaluationSample(token: EvaluationToken | null, element: unknown, match: boolean): void {
  if (!active() || token === null) return;
  protect<void>(undefined, () => {
    const action = current, evaluation = action?.evaluation;
    if (!action || action.done || !evaluation || evaluation.token !== token || evaluation.element !== element || !control(element) ||
        typeof match !== "boolean" || match !== (element.isConnected && element.value === evaluation.expected)) { fail(); return; }
    if (evaluation.samples >= DESTINATION_MAX_SAMPLES) { fail("overflow"); return; }
    evaluation.samples++; samples++; evaluation.lastMatch = match;
    if (evaluation.samples === 1) record(action, 7, { connected: Number(element.isConnected), match: Number(match), samples: 1 });
  });
}
function evaluationWitness(token: EvaluationToken | null, element: unknown): void {
  if (!active() || token === null) return;
  protect<void>(undefined, () => {
    const action = current, read = action?.read, evaluation = action?.evaluation;
    if (!action || action.done || !read || !evaluation || evaluation.token !== token || evaluation.element !== element || read.element !== element ||
        !stillAccepted(action, read) || read.purpose !== evaluation.expected || !control(element) || !element.isConnected || element.value !== evaluation.expected ||
        !evaluation.samples || !evaluation.lastMatch) { fail(); return; }
    witnesses++; record(action, 8, { connected: 1, match: 1, samples: evaluation.samples });
    // Retain only the completed numeric ordinal; lens-reset reads are unmeasured.
    action.done = true; action.project = null; action.read = null; action.evaluation = null;
  });
}
if (enabled) protect<void>(undefined, () => {
  Object.defineProperty(window, "__luciDestinationPhase", { configurable: false, value: {
    arm() { if (armed || actions || rows.length) { fail("overlap"); return; } armed = true; },
    snapshot() {
      const result = { version: 1, timeOrigin: performance.timeOrigin, armed: Number(armed), actions, destinations, reads, accepted, commits, evaluations, samples, witnesses,
        observed, dropped, errors, overlaps, mismatches, active: current && !current.done ? current.ordinal : 0,
        pendingReads: Number(!!current && !current.done && current.project !== null && current.read?.value == null),
        pendingEvaluations: Number(!!current?.evaluation && !current.done), rows };
      rows = []; return result;
    },
    evaluationEnter, evaluationSample, evaluationWitness,
  } });
});
