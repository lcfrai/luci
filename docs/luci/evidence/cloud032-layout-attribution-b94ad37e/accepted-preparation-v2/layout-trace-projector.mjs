import { TextDecoder } from 'node:util';

// Pure projection only. The caller owns raw private retention and CDP capture.
export const TRACE_LIMITS = Object.freeze({ bytes: 64 * 1024 * 1024, events: 200000, rounds: 8, stackFrames: 256, stackDepth: 8, clockSpreadUs: 1000 });
export const REFUSAL_CODES = Object.freeze([
  'invalid-options', 'capture-incomplete', 'capture-data-loss', 'trace-too-large',
  'trace-invalid-utf8', 'trace-invalid-json', 'trace-invalid-shape', 'trace-too-many-events',
  'trace-invalid-event', 'trace-negative-time', 'trace-invalid-stack', 'trace-stack-limit',
  'marker-invalid-receipt', 'marker-missing', 'marker-duplicate', 'marker-unexpected',
  'marker-invalid-event', 'marker-wrong-document', 'marker-wrong-renderer',
  'marker-order', 'marker-clock-spread',
]);
const ALLOWED_EVENTS = Object.freeze([
  'Layout', 'RecalculateStyles', 'UpdateLayoutTree', 'InvalidateLayout',
  'ScheduleStyleRecalculation', 'LayoutInvalidationTracking',
  'ScheduleStyleInvalidationTracking', 'StyleRecalcInvalidationTracking',
  'StyleInvalidatorInvalidationTracking', 'StyleInvalidatorTracing',
  'StyleInvalidationTracking', 'PaintInvalidationTracking',
  'ScrollInvalidationTracking', 'LayerInvalidationTracking',
]);
const ALLOWED_CATEGORIES = Object.freeze([
  'devtools.timeline', 'disabled-by-default-devtools.timeline',
  'disabled-by-default-devtools.timeline.stack',
  'disabled-by-default-devtools.timeline.invalidationTracking', 'blink.user_timing',
]);
const EVENT_NAMES = new Set(ALLOWED_EVENTS);
const ATTRIBUTABLE_LAYOUT_EVENTS = new Set(['Layout', 'UpdateLayoutTree', 'RecalculateStyles']);
const PHASES = new Set(['B', 'E', 'X', 'I', 'i', 'P', 'p', 'N', 'O', 'D', 'R', 'C', 'S', 'T', 'F', 'b', 'e', 'n', 's', 't', 'f', 'M', 'v', 'V', 'c', '(', ')']);
const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
const record = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const nonnegativeNumber = value => Number.isFinite(value) && value >= 0 && value <= Number.MAX_SAFE_INTEGER;
const integer = value => Number.isSafeInteger(value) && value >= 0;
function refuse(code) { const error = new Error(code); error.code = code; throw error; }
function check(value, code) { if (!value) refuse(code); }
function clockNumber(value) {
  if (typeof value === 'number' && value < 0) refuse('trace-negative-time');
  check(nonnegativeNumber(value), 'trace-invalid-event');
}

function parse(rawUtf8) {
  let text;
  if (typeof rawUtf8 === 'string') {
    check(Buffer.byteLength(rawUtf8, 'utf8') <= TRACE_LIMITS.bytes, 'trace-too-large');
    text = rawUtf8;
  } else {
    check(rawUtf8 instanceof Uint8Array, 'trace-invalid-shape');
    check(rawUtf8.byteLength <= TRACE_LIMITS.bytes, 'trace-too-large');
    try { text = new TextDecoder('utf-8', { fatal: true }).decode(rawUtf8); }
    catch { refuse('trace-invalid-utf8'); }
  }
  let value;
  try { value = JSON.parse(text); } catch { refuse('trace-invalid-json'); }
  check(record(value) && own(value, 'traceEvents') && Array.isArray(value.traceEvents), 'trace-invalid-shape');
  check(value.traceEvents.length <= TRACE_LIMITS.events, 'trace-too-many-events');
  return value.traceEvents;
}

function configuration(options) {
  check(record(options), 'invalid-options');
  const { appOrigin, bundles, markers, expectedRounds, capture } = options;
  check(record(capture) && capture.complete === true, 'capture-incomplete');
  check(capture.dataLossOccurred === false, 'capture-data-loss');
  check(expectedRounds === TRACE_LIMITS.rounds, 'invalid-options');
  let origin;
  try { origin = new URL(appOrigin); } catch { refuse('invalid-options'); }
  check(typeof appOrigin === 'string' && ['http:', 'https:'].includes(origin.protocol) && origin.origin === appOrigin, 'invalid-options');
  check(Array.isArray(bundles) && bundles.length > 0 && bundles.length <= 4096, 'invalid-options');
  const byURL = new Map();
  for (const bundle of bundles) {
    check(record(bundle) && typeof bundle.path === 'string' && /^\/assets\/[^?#\s\\]+\.m?js$/.test(bundle.path)
      && /^[a-f0-9]{64}$/.test(bundle.sha256 ?? ''), 'invalid-options');
    let url;
    try { url = new URL(bundle.path, appOrigin); } catch { refuse('invalid-options'); }
    check(url.origin === appOrigin && url.href === appOrigin + bundle.path && !byURL.has(url.href), 'invalid-options');
    byURL.set(url.href, bundle.sha256);
  }
  check(Array.isArray(markers) && markers.length === expectedRounds * 2, 'marker-invalid-receipt');
  const receipts = new Map();
  let timeOrigin;
  for (const marker of markers) {
    check(record(marker) && integer(marker.round) && marker.round < expectedRounds
      && ['start', 'end'].includes(marker.edge)
      && marker.name === `luci-layout-boundary-${marker.round}-${marker.edge}`
      && nonnegativeNumber(marker.startTime) && nonnegativeNumber(marker.timeOrigin) && marker.timeOrigin > 0, 'marker-invalid-receipt');
    check(!receipts.has(marker.name), 'marker-duplicate');
    if (timeOrigin === undefined) timeOrigin = marker.timeOrigin;
    check(marker.timeOrigin === timeOrigin, 'marker-wrong-document');
    receipts.set(marker.name, marker);
  }
  return { byURL, receipts, expectedRounds, timeOrigin };
}

function validateEvent(event) {
  check(record(event) && typeof event.name === 'string' && event.name.length <= 4096
    && typeof event.cat === 'string' && event.cat.length <= 4096
    && PHASES.has(event.ph) && integer(event.pid) && integer(event.tid), 'trace-invalid-event');
  clockNumber(event.ts);
  for (const field of ['dur', 'tts', 'tdur']) if (own(event, field)) clockNumber(event[field]);
  if (event.ph === 'X') check(own(event, 'dur'), 'trace-invalid-event');
  if (own(event, 'dur')) check(nonnegativeNumber(event.ts + event.dur), 'trace-invalid-event');
  if (own(event, 'args')) check(record(event.args), 'trace-invalid-event');
}

function stackProjection(event, byURL, totals) {
  const stacks = [];
  const candidates = [['args', event.args], ['data', event.args?.data], ['begin-data', event.args?.beginData], ['end-data', event.args?.endData]];
  let framesSeen = 0;
  function frames(value, depth) {
    check(depth <= TRACE_LIMITS.stackDepth, 'trace-stack-limit');
    const array = Array.isArray(value) ? value : record(value) ? value.callFrames : null;
    check(Array.isArray(array), 'trace-invalid-stack');
    framesSeen += array.length;
    check(framesSeen <= TRACE_LIMITS.stackFrames, 'trace-stack-limit');
    const safe = array.map(frame => {
      check(record(frame), 'trace-invalid-stack');
      const bundleSHA256 = typeof frame.url === 'string' ? byURL.get(frame.url) : undefined;
      if (bundleSHA256 && integer(frame.lineNumber) && integer(frame.columnNumber)) {
        totals.attributed++;
        return { attribution: 'bundle', bundleSHA256, lineNumber: frame.lineNumber, columnNumber: frame.columnNumber };
      }
      totals.unattributed++;
      return { attribution: 'unattributed' };
    });
    // CDP async parent frames are retained; opaque parent IDs are not exported.
    if (!Array.isArray(value) && own(value, 'parent')) safe.push(...frames(value.parent, depth + 1));
    if (!Array.isArray(value) && own(value, 'parentId')) {
      totals.unresolvedParents++;
      safe.push({ attribution: 'unattributed-async-parent' });
    }
    return safe;
  }
  for (const [location, holder] of candidates) {
    if (record(holder) && own(holder, 'stackTrace')) stacks.push({ location, frames: frames(holder.stackTrace, 0) });
  }
  return stacks;
}

/**
 * Project a complete private CDP trace. Marker receipts are returned directly by
 * performance.mark calls outside the unchanged original interaction rounds.
 * Source coordinates are retained as supplied by CDP; no source-map lookup or
 * function/CPU attribution is inferred here. Unknown frames stay unattributed.
 */
export function projectLayoutTrace(rawUtf8, options) {
  const config = configuration(options);
  const raw = parse(rawUtf8);
  const counts = {
    allEvents: raw.length, allowedLayoutEvents: 0, otherEvents: 0,
    byAllowedName: Object.fromEntries(ALLOWED_EVENTS.map(name => [name, 0])),
    byPhase: {},
    categoryMembership: Object.fromEntries(ALLOWED_CATEGORIES.map(name => [name, 0])),
    eventsWithOtherCategories: 0,
  };
  const found = new Map();
  for (const event of raw) {
    validateEvent(event);
    counts.byPhase[event.ph] = (counts.byPhase[event.ph] ?? 0) + 1;
    const categories = new Set(event.cat.split(',').map(value => value.trim()));
    for (const category of ALLOWED_CATEGORIES) if (categories.has(category)) counts.categoryMembership[category]++;
    if ([...categories].some(category => !ALLOWED_CATEGORIES.includes(category))) counts.eventsWithOtherCategories++;
    if (EVENT_NAMES.has(event.name)) { counts.allowedLayoutEvents++; counts.byAllowedName[event.name]++; }
    else counts.otherEvents++;
    if (!event.name.startsWith('luci-layout-boundary-')) continue;
    check(config.receipts.has(event.name), 'marker-unexpected');
    check(categories.has('blink.user_timing') && ['R', 'I', 'i'].includes(event.ph)
      && event.pid > 0 && event.tid > 0 && (!own(event, 'dur') || event.dur === 0), 'marker-invalid-event');
    check(!found.has(event.name), 'marker-duplicate');
    found.set(event.name, event);
  }
  check(found.size === config.receipts.size, 'marker-missing');
  const anchors = [];
  let previousTrace = -1, previousBrowser = -1, renderer;
  for (let round = 0; round < config.expectedRounds; round++) {
    for (const edge of ['start', 'end']) {
      const name = `luci-layout-boundary-${round}-${edge}`;
      const receipt = config.receipts.get(name), event = found.get(name);
      check(receipt && event, 'marker-missing');
      renderer ??= { pid: event.pid, tid: event.tid };
      check(renderer.pid === event.pid && renderer.tid === event.tid, 'marker-wrong-renderer');
      check(event.ts > previousTrace && receipt.startTime > previousBrowser, 'marker-order');
      const offsetUs = event.ts - receipt.startTime * 1000;
      check(Number.isFinite(offsetUs) && Math.abs(offsetUs) <= Number.MAX_SAFE_INTEGER, 'marker-clock-spread');
      anchors.push({ round, edge, traceTsUs: event.ts, browserStartTimeMs: receipt.startTime, offsetUs });
      previousTrace = event.ts; previousBrowser = receipt.startTime;
    }
  }
  const offsets = anchors.map(anchor => anchor.offsetUs);
  const spreadUs = Math.max(...offsets) - Math.min(...offsets);
  check(spreadUs <= TRACE_LIMITS.clockSpreadUs, 'marker-clock-spread');
  const offsetUs = anchors[0].offsetUs;
  const rounds = Array.from({ length: config.expectedRounds }, (_, round) => ({
    round,
    start: { ...anchors[round * 2], translatedBrowserNowMs: (anchors[round * 2].traceTsUs - offsetUs) / 1000 },
    end: { ...anchors[round * 2 + 1], translatedBrowserNowMs: (anchors[round * 2 + 1].traceTsUs - offsetUs) / 1000 },
  }));
  const stackFrames = { attributed: 0, unattributed: 0, unresolvedParents: 0 };
  const events = [];
  let usableLayoutStackCount = 0;
  for (let index = 0; index < raw.length; index++) {
    const event = raw[index];
    if (!EVENT_NAMES.has(event.name)) continue;
    const sameRendererThread = event.pid === renderer.pid && event.tid === renderer.tid;
    const end = event.ts + (event.dur ?? 0);
    const overlappingRounds = sameRendererThread ? rounds.filter(round => event.ts <= round.end.traceTsUs && end >= round.start.traceTsUs).map(round => round.round) : [];
    const stacks = stackProjection(event, config.byURL, stackFrames);
    if (sameRendererThread && overlappingRounds.length && ATTRIBUTABLE_LAYOUT_EVENTS.has(event.name)
      && stacks.some(stack => stack.frames.some(frame => frame.attribution === 'bundle'))) usableLayoutStackCount++;
    events.push({
      rawEventIndex: index, name: event.name, ph: event.ph, pid: event.pid, tid: event.tid, tsUs: event.ts,
      ...(own(event, 'dur') ? { durUs: event.dur } : {}),
      ...(own(event, 'tts') ? { threadTsUs: event.tts } : {}),
      ...(own(event, 'tdur') ? { threadDurUs: event.tdur } : {}),
      sameRendererThread, overlappingRounds,
      ...(sameRendererThread ? { translatedBrowserNowMs: (event.ts - offsetUs) / 1000 } : {}),
      stacks,
    });
  }
  return {
    version: 1, projection: 'cloud032-layout-timing-only', validCapture: true,
    status: usableLayoutStackCount > 0 ? 'layout-stack-position-available' : 'no-attributable-layout-stack',
    attributionAvailable: usableLayoutStackCount > 0, usableLayoutStackCount,
    capture: { complete: true, dataLossOccurred: false }, limits: { ...TRACE_LIMITS },
    clock: { renderer, documentTimeOriginMs: config.timeOrigin, anchor: 'first-round-start', offsetUs, spreadUs, anchorCount: anchors.length },
    counts, rounds, stackFrames, events,
    limitations: [
      'All raw events are counted; every allowlisted layout/invalidation event is retained, including other threads and events outside rounds.',
      'Category counts overlap when an event belongs to multiple categories. Unknown event names/categories are counted without publishing their strings.',
      'Round overlap uses event wall timestamps, not causality. B/E phases remain separate; missing durations are not inferred.',
      'Thread CPU fields are only retained when supplied. Wall duration alone is not CPU execution or proof of host scheduling.',
      'Exact bundle URLs map to hashes and source coordinates only. Function names, raw arguments, URLs, frame IDs and opaque async parent IDs are omitted.',
      'A usable layout stack is a known bundle position on the anchored thread overlapping a round. It may identify only a React/framework dispatcher; no component, root cause or causal link is inferred.',
      'The caller retains raw private bytes and establishes bundle integrity and process ownership. This projection does not validate source maps or alter acceptance timings.',
    ],
  };
}
