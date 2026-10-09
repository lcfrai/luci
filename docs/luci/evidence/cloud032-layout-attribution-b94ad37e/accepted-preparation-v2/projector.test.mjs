import test from 'node:test';
import assert from 'node:assert/strict';
import { projectLayoutTrace, TRACE_LIMITS, REFUSAL_CODES } from './layout-trace-projector.mjs';

const origin = 'http://127.0.0.1:43210';
const bundleSHA256 = 'a'.repeat(64);
const bundlePath = '/assets/index-fixture.js';
const SECRET = 'PRIVATE_SECRET_DO_NOT_PUBLISH';
function fixture() {
  const markers = [], traceEvents = [];
  for (let round = 0; round < 8; round++) {
    for (const [edge, delta] of [['start', 0], ['end', 100]]) {
      const name = `luci-layout-boundary-${round}-${edge}`, startTime = 10 + round * 200 + delta;
      markers.push({ name, round, edge, timeOrigin: 1700000000000, startTime });
      traceEvents.push({ name, cat: 'blink.user_timing', ph: 'R', pid: 101, tid: 102, ts: 1000000 + startTime * 1000, args: { data: { detail: SECRET } } });
    }
  }
  return {
    trace: { traceEvents },
    options: { appOrigin: origin, bundles: [{ path: bundlePath, sha256: bundleSHA256 }], markers, expectedRounds: 8, capture: { complete: true, dataLossOccurred: false } },
  };
}
function layout(name = 'Layout', extra = {}) {
  return { name, cat: 'devtools.timeline', ph: 'X', pid: 101, tid: 102, ts: 1030000, dur: 2000, ...extra };
}
function project(value) { return projectLayoutTrace(JSON.stringify(value.trace), value.options); }
function refuses(value, code) {
  assert.throws(() => project(value), error => error.code === code && error.message === code && REFUSAL_CODES.includes(error.code));
}

test('all sixteen receipt anchors bind one renderer clock without marker details', () => {
  const value = fixture();
  for (const event of value.trace.traceEvents) delete event.args;
  const result = project(value);
  assert.deepEqual(result.clock, { renderer: { pid: 101, tid: 102 }, documentTimeOriginMs: 1700000000000, anchor: 'first-round-start', offsetUs: 1000000, spreadUs: 0, anchorCount: 16 });
  assert.equal(result.rounds.length, 8);
  assert.equal(result.rounds[7].end.translatedBrowserNowMs, 1510);
  assert.equal(result.counts.allEvents, 16);
  assert.equal(result.validCapture, true);
  assert.equal(result.status, 'no-attributable-layout-stack');
  assert.equal(result.attributionAvailable, false);
  assert.equal(result.usableLayoutStackCount, 0);
});

test('unrelated Chrome clock-sync phase is counted without exposing its private args', () => {
  const value = fixture();
  value.trace.traceEvents.push({ name: 'clock_sync', cat: '__metadata', ph: 'c', pid: 101, tid: 102, ts: 1000000, args: { sync_id: SECRET, issue_ts: 123, url: SECRET } });
  const result = project(value);
  assert.equal(result.counts.allEvents, 17);
  assert.equal(result.counts.otherEvents, 17);
  assert.equal(result.counts.byPhase.c, 1);
  assert.equal(result.clock.anchorCount, 16);
  assert.equal(result.clock.offsetUs, 1000000);
  assert.equal(result.events.length, 0);
  assert.equal(JSON.stringify(result).includes(SECRET), false);
});

test('retains every required event, every phase and events outside rounds/on other threads', () => {
  const value = fixture();
  const names = ['Layout', 'RecalculateStyles', 'UpdateLayoutTree', 'InvalidateLayout', 'ScheduleStyleRecalculation'];
  value.trace.traceEvents.push(...names.map(name => layout(name)));
  value.trace.traceEvents.push(layout('Layout', { pid: 202, tid: 203 }), layout('Layout', { ts: 100, ph: 'B', dur: undefined }));
  value.trace.traceEvents.push(layout('Layout', { ts: 2700000, ph: 'E', dur: undefined }));
  value.trace.traceEvents.push(layout(SECRET, { cat: SECRET, args: { url: SECRET } }));
  const result = project(value);
  assert.equal(result.counts.allEvents, 25);
  assert.equal(result.counts.allowedLayoutEvents, 8);
  assert.equal(result.counts.otherEvents, 17);
  assert.equal(result.events.length, 8);
  assert.deepEqual(result.events[0].overlappingRounds, [0]);
  assert.deepEqual(result.events[5].overlappingRounds, []);
  assert.equal(result.events[5].sameRendererThread, false);
  assert.deepEqual(result.events[6].overlappingRounds, []);
  assert.deepEqual(result.events[7].overlappingRounds, []);
  assert.equal(result.events[6].ph, 'B');
  assert.equal(result.events[7].ph, 'E');
  assert.equal(Object.hasOwn(result.events[6], 'durUs'), false);
  assert.equal(JSON.stringify(result).includes(SECRET), false);
});

test('publishes exact bundle hash/coordinates, never frame names, URLs, args or opaque IDs', () => {
  const value = fixture();
  const known = { url: origin + bundlePath, lineNumber: 12, columnNumber: 34, functionName: SECRET, scriptId: SECRET };
  value.trace.traceEvents.push(layout('LayoutInvalidationTracking', { cat: 'disabled-by-default-devtools.timeline.invalidationTracking', args: {
    frame: SECRET, reason: SECRET,
    data: { nodeName: SECRET, stackTrace: [known, { ...known, url: 'https://untrusted.invalid/' + SECRET }] },
    beginData: { stackTrace: { callFrames: [known], parent: { callFrames: [{ ...known, url: origin + bundlePath + '?token=' + SECRET }] }, parentId: { id: SECRET } } },
  } }));
  const result = project(value), text = JSON.stringify(result);
  assert.equal(text.includes(SECRET), false);
  assert.equal(text.includes(origin), false);
  assert.equal(text.includes(bundlePath), false);
  assert.equal(text.includes('functionName'), false);
  assert.deepEqual(result.events[0].stacks[0].frames, [{ attribution: 'bundle', bundleSHA256, lineNumber: 12, columnNumber: 34 }, { attribution: 'unattributed' }]);
  assert.deepEqual(result.stackFrames, { attributed: 2, unattributed: 2, unresolvedParents: 1 });
});

test('same origin but unmatched path, query, fragment or missing coordinates stays unattributed', () => {
  const value = fixture();
  const urls = [origin + '/assets/other.js', origin + bundlePath + '?x=1', origin + bundlePath + '#fragment', origin.replace('43210', '43211') + bundlePath];
  value.trace.traceEvents.push(layout('Layout', { args: { data: { stackTrace: [...urls.map(url => ({ url, lineNumber: 0, columnNumber: 0 })), { url: origin + bundlePath }] } } }));
  assert.deepEqual(project(value).stackFrames, { attributed: 0, unattributed: 5, unresolvedParents: 0 });
  assert.equal(project(value).attributionAvailable, false);
});

test('known bundle stack positions distinguish usable attribution from valid but unhelpful capture', () => {
  const value = fixture();
  const args = { beginData: { stackTrace: [{ url: origin + bundlePath, lineNumber: 1, columnNumber: 2, functionName: 'ReactDispatcher' }] } };
  // These are retained but cannot satisfy the targeted original-round goal.
  value.trace.traceEvents.push(layout('Layout', { args, tid: 303 }));
  value.trace.traceEvents.push(layout('Layout', { args, ts: 9000000 }));
  value.trace.traceEvents.push(layout('LayoutInvalidationTracking', { args }));
  const unavailable = project(value);
  assert.equal(unavailable.validCapture, true);
  assert.equal(unavailable.attributionAvailable, false);
  assert.equal(unavailable.usableLayoutStackCount, 0);
  for (const name of ['Layout', 'UpdateLayoutTree', 'RecalculateStyles']) value.trace.traceEvents.push(layout(name, { args }));
  const available = project(value);
  assert.equal(available.validCapture, true);
  assert.equal(available.attributionAvailable, true);
  assert.equal(available.usableLayoutStackCount, 3);
  assert.equal(available.status, 'layout-stack-position-available');
  assert.equal(available.events.length, 6);
  assert.equal(JSON.stringify(available).includes('ReactDispatcher'), false);
});

test('retains thread timing and cross-round overlap without inferring CPU or causality', () => {
  const value = fixture();
  value.trace.traceEvents.push(layout('Layout', { dur: 400000, tts: 20, tdur: 10 }));
  const event = project(value).events[0];
  assert.deepEqual(event.overlappingRounds, [0, 1, 2]);
  assert.equal(event.threadTsUs, 20);
  assert.equal(event.threadDurUs, 10);
});

test('accepts bounded offset jitter and refuses spread above one millisecond', () => {
  const value = fixture();
  value.trace.traceEvents[4].ts += 1000;
  assert.equal(project(value).clock.spreadUs, 1000);
  value.trace.traceEvents[4].ts += 0.01;
  refuses(value, 'marker-clock-spread');
});

test('missing, duplicate and unexpected late boundary marks refuse', () => {
  const missing = fixture(); missing.trace.traceEvents.pop(); refuses(missing, 'marker-missing');
  const duplicate = fixture(); duplicate.trace.traceEvents.push({ ...duplicate.trace.traceEvents[0], ts: 9000000 }); refuses(duplicate, 'marker-duplicate');
  const late = fixture(); late.trace.traceEvents.push({ ...late.trace.traceEvents[0], name: 'luci-layout-boundary-8-start', ts: 9000000 }); refuses(late, 'marker-unexpected');
});

test('wrong document, renderer, marker phase and category refuse', () => {
  const document = fixture(); document.options.markers[2].timeOrigin++; refuses(document, 'marker-wrong-document');
  const renderer = fixture(); renderer.trace.traceEvents[3].pid++; refuses(renderer, 'marker-wrong-renderer');
  const thread = fixture(); thread.trace.traceEvents[3].tid++; refuses(thread, 'marker-wrong-renderer');
  const phase = fixture(); phase.trace.traceEvents[0].ph = 'B'; refuses(phase, 'marker-invalid-event');
  const category = fixture(); category.trace.traceEvents[0].cat = 'devtools.timeline'; refuses(category, 'marker-invalid-event');
});

test('receipt count, identity, duplicates and nonmonotonic boundaries refuse', () => {
  const reduced = fixture(); reduced.options.expectedRounds = 7; refuses(reduced, 'invalid-options');
  const count = fixture(); count.options.markers.pop(); refuses(count, 'marker-invalid-receipt');
  const name = fixture(); name.options.markers[0].name = SECRET; refuses(name, 'marker-invalid-receipt');
  const duplicate = fixture(); duplicate.options.markers[1] = duplicate.options.markers[0]; refuses(duplicate, 'marker-duplicate');
  const order = fixture(); order.trace.traceEvents[1].ts = order.trace.traceEvents[0].ts; refuses(order, 'marker-order');
  const receiptOrder = fixture(); receiptOrder.options.markers[1].startTime = receiptOrder.options.markers[0].startTime; refuses(receiptOrder, 'marker-order');
});

test('complete JSON and explicit completed/no-loss capture flags are mandatory', () => {
  const value = fixture();
  assert.throws(() => projectLayoutTrace('{"traceEvents":[', value.options), { code: 'trace-invalid-json' });
  assert.throws(() => projectLayoutTrace('[]', value.options), { code: 'trace-invalid-shape' });
  const incomplete = fixture(); incomplete.options.capture.complete = false; refuses(incomplete, 'capture-incomplete');
  const lost = fixture(); lost.options.capture.dataLossOccurred = true; refuses(lost, 'capture-data-loss');
  const unknown = fixture(); delete unknown.options.capture.dataLossOccurred; refuses(unknown, 'capture-data-loss');
});

test('negative/nonfinite time and malformed events refuse even when not selected for projection', () => {
  const negative = fixture(); negative.trace.traceEvents.push(layout(SECRET, { ts: -1 })); refuses(negative, 'trace-negative-time');
  const duration = fixture(); duration.trace.traceEvents.push(layout(SECRET, { dur: -1 })); refuses(duration, 'trace-negative-time');
  const invalid = fixture(); invalid.trace.traceEvents.push(layout(SECRET, { ts: null })); refuses(invalid, 'trace-invalid-event');
  const missingDuration = fixture(); missingDuration.trace.traceEvents.push(layout(SECRET, { dur: undefined })); refuses(missingDuration, 'trace-invalid-event');
  const malformed = fixture(); malformed.trace.traceEvents.push(null); refuses(malformed, 'trace-invalid-event');
});

test('raw byte and event limits refuse before projection without returning a passing subset', () => {
  const value = fixture();
  assert.throws(() => projectLayoutTrace(Buffer.alloc(TRACE_LIMITS.bytes + 1), value.options), { code: 'trace-too-large' });
  value.trace.traceEvents = Array(TRACE_LIMITS.events + 1).fill(null);
  refuses(value, 'trace-too-many-events');
});

test('UTF-8 bytes are accepted; invalid UTF-8 is refused', () => {
  const value = fixture();
  assert.equal(projectLayoutTrace(Buffer.from(JSON.stringify(value.trace)), value.options).counts.allEvents, 16);
  assert.throws(() => projectLayoutTrace(Uint8Array.of(0xff), value.options), { code: 'trace-invalid-utf8' });
});

test('invalid or oversized stack data refuses without clipping frames', () => {
  const malformed = fixture(); malformed.trace.traceEvents.push(layout('Layout', { args: { data: { stackTrace: SECRET } } })); refuses(malformed, 'trace-invalid-stack');
  const oversized = fixture(); oversized.trace.traceEvents.push(layout('Layout', { args: { data: { stackTrace: Array(257).fill({}) } } })); refuses(oversized, 'trace-stack-limit');
});

test('bundle bindings reject duplicates and noncanonical or credential-bearing origins', () => {
  const duplicate = fixture(); duplicate.options.bundles.push({ ...duplicate.options.bundles[0] }); refuses(duplicate, 'invalid-options');
  const path = fixture(); path.options.bundles[0].path = '/assets/sub/../index-fixture.js'; refuses(path, 'invalid-options');
  const originWithPath = fixture(); originWithPath.options.appOrigin += '/private'; refuses(originWithPath, 'invalid-options');
  const credentials = fixture(); credentials.options.appOrigin = 'http://user:secret@127.0.0.1:43210'; refuses(credentials, 'invalid-options');
});
