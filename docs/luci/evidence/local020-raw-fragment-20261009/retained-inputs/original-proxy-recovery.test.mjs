// Controlled in-process HTTP/Unix-socket fixtures only. Never contacts Docker.
// Run: node --test --test-timeout=30000 proxy-recovery.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import {mkdtemp, rm, readFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {startFencedDockerProxy} from '../launch-recovery/fenced-docker-proxy.mjs';

const helperPin = '81f2ba5b194a81a4f2ad167e466793f59162b387';
const pinned = name => execFileSync('git', ['show', `${helperPin}:scripts/acceptance/launch-recovery/${name}`],
  {cwd: '/workspace/luci', encoding: 'utf8', timeout: 5000, maxBuffer: 1024 * 1024});
const hash = text => createHash('sha256').update(text).digest('hex');
const dataModule = source => 'data:text/javascript;base64,' + Buffer.from(source).toString('base64');
const proxyURL = new URL('../launch-recovery/fenced-docker-proxy.mjs', import.meta.url);
const api = '/v1.47/containers/';
const checkName = labels => 'luci-check-' + labels['luci.check-allocation'];
const createPath = labels => api + 'create?name=' + checkName(labels);
const labelsFor = digit => ({
  'luci.check': 'true', 'luci.check-protocol': '1', 'luci.instance': 'fixture-instance',
  'luci.check-engine': 'fixture-engine', 'luci.check-allocation': 'checkexec-' + digit.repeat(24),
  'luci.check-id': 'check-' + digit.repeat(24), 'luci.check-command-index': '0',
  'luci.check-scope': '1'.repeat(64), 'luci.check-project': 'project-' + digit,
  'luci.check-command-digest': '2'.repeat(64), 'luci.check-source-digest': '3'.repeat(64),
  'luci.check-image': 'sha256:' + '4'.repeat(64),
});
const fenceLabelsFor = labels => ({
  ...Object.fromEntries(Object.entries(labels).filter(([key]) => !['luci.check-source-digest', 'luci.check-image'].includes(key))),
  'luci.launch-fence': '1', 'luci.fence-intent': '5'.repeat(32), 'luci.fence-binding': '6'.repeat(64),
});
const identify = body => {
  const labels = JSON.parse(body).Labels;
  return labels?.['luci.launch-fence'] === '1' ? 'fence' : labels?.['luci.check'] === 'true' ? 'check' : 'worker';
};
const recoveryOptions = {
  allowCheckTraffic: true, checkTrafficProtocol: 'sdk-ledger-v1', allowCheckRecovery: true,
  checkCreateLimit: 3, maxCreates: 4, maxCreatesByKind: {worker: 1, check: 3, fence: 1}, maxStarts: 2,
};
const json = (res, status, body) => {res.writeHead(status, {'content-type': 'application/json'}); res.end(JSON.stringify(body));};

function send(socketPath, method, path, body) {
  let request;
  const promise = new Promise(resolve => {
    request = http.request({socketPath, method, path, agent: false,
      headers: body === undefined ? {} : {'content-type':'application/json','content-length':Buffer.byteLength(JSON.stringify(body))}}, response => {
      const chunks = [];
      response.on('data', value => chunks.push(value));
      response.once('end', () => resolve({status: response.statusCode, body: Buffer.concat(chunks).toString()}));
      response.once('error', error => resolve({error: error.code}));
    });
    request.setTimeout(2500, () => request.destroy(Object.assign(new Error('test-request-timeout'), {code: 'TEST_TIMEOUT'})));
    request.once('error', error => resolve({error: error.code}));
    request.end(body === undefined ? undefined : JSON.stringify(body));
  });
  return {promise, destroy: () => request.destroy()};
}

async function fixture(t, overrides = {}, customReply) {
  const directory = await mkdtemp(join(tmpdir(), 'check-proxy-test-'));
  const upstreamSocketPath = join(directory, 'up.sock'), socketPath = join(directory, 'p.sock');
  const received = [], sockets = new Set(), containers = new Map(), names = new Map();
  let nextID = 0, proxy;
  function reply(entry, res) {
    const url = new URL(entry.path, 'http://fixture.invalid');
    if (entry.method === 'POST' && url.pathname === api + 'create') {
      const name = url.searchParams.get('name');
      if (names.has(name)) return json(res, 409, {message: 'synthetic name conflict'});
      const id = (++nextID).toString(16).padStart(64, '0');
      containers.set(id, {Id: id, Name: '/' + name, Config: entry.body, State: {Status: 'created'}}); names.set(name, id);
      return json(res, 201, {Id: id});
    }
    const parts = url.pathname.slice(api.length).split('/'), id = containers.has(parts[0]) ? parts[0] : names.get(parts[0]);
    if (entry.method === 'GET' && parts[1] === 'json') return json(res, id ? 200 : 404, id ? containers.get(id) : {message: 'synthetic absence'});
    if (entry.method === 'DELETE') {
      if (!id) return json(res, 404, {message: 'synthetic absence'});
      names.delete(containers.get(id).Name.slice(1)); containers.delete(id);
    }
    res.writeHead(204); res.end();
  }
  const server = http.createServer((req, res) => {
    const chunks = [];
    req.on('data', value => chunks.push(value));
    req.once('end', () => {
      const bytes = Buffer.concat(chunks).toString();
      const entry = {method: req.method, path: req.url, body: bytes ? JSON.parse(bytes) : undefined};
      received.push(entry); (customReply ?? reply)(entry, res, reply);
    });
  });
  server.on('connection', socket => {sockets.add(socket); socket.once('close', () => sockets.delete(socket));});
  t.after(async () => {
    if (proxy) await proxy.close().catch(() => {});
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
    await rm(directory, {recursive: true, force: true});
  });
  await new Promise(resolve => server.listen(upstreamSocketPath, resolve));
  proxy = await startFencedDockerProxy({socketPath, upstreamSocketPath, identifyCreate: identify,
    beforeCreate() {}, upstreamTimeoutMs: 1000, holdTimeoutMs: 1000, ...overrides});
  return {proxy, received, containers, request: (method, path, body) => send(socketPath, method, path, body)};
}

async function createCheck(f, labels) {
  const response = await f.request('POST', createPath(labels), {Labels: labels}).promise;
  assert.equal(response.status, 201); return JSON.parse(response.body).Id;
}

test('assembled proxy is based on the exact pinned helper source', async () => {
  assert.equal(hash(pinned('fenced-docker-proxy.mjs')), '48e97d6d5c915cf80c99f0c79a8fb16261eb219616f878fa1ce103ee2c59303d');
  assert.equal(hash(await readFile(proxyURL)), '2b6887af0bad7975c74bac42c8ed7988813e3c72ba615281c727c0ac268b1606');
});

test('recovery opt-in is explicit and check budget is bounded to three', async () => {
  for (const options of [
    {allowCheckRecovery: true},
    {...recoveryOptions, checkCreateLimit: 0}, {...recoveryOptions, checkCreateLimit: 4},
    {...recoveryOptions, checkCreateLimit: 1.5}, {...recoveryOptions, allowCheckRecovery: false},
    {...recoveryOptions, maxCreatesByKind: {worker: 1, check: 2, fence: 1}},
  ]) await assert.rejects(startFencedDockerProxy({socketPath: '/tmp/absent-p.sock', upstreamSocketPath: '/tmp/absent-u.sock',
    identifyCreate: identify, beforeCreate() {}, ...options}), /explicit-sdk-check-recovery|required|budget/);
});

test('SDK generated one-shot stats request forwards one bounded read for the exact owned check', async t => {
  const wire=JSON.parse(await readFile(new URL('./sdk-stats-wire.json',import.meta.url),'utf8'));
  assert.equal(wire.calls,1);assert.equal(wire.request.method,'GET');assert.equal(wire.request.bodyBytes,0);
  assert.equal(wire.request.path,'/v1.51/containers/'+'a'.repeat(64)+'/stats');
  assert.equal(wire.request.rawQuery,'one-shot=1&stream=0');
  const f=await fixture(t,recoveryOptions,(entry,res,reply)=>{
    if(entry.method==='GET'&&entry.path.includes('/stats?'))return json(res,200,{read:'controlled single response',memory_stats:{usage:42}});
    reply(entry,res);
  });
  const labels=labelsFor('a'),id=await createCheck(f,labels),before=f.received.length;
  const path=wire.request.requestURI.replace('a'.repeat(64),id);
  const response=await f.request(wire.request.method,path).promise;
  assert.equal(response.status,200);assert.deepEqual(JSON.parse(response.body),{read:'controlled single response',memory_stats:{usage:42}});
  assert.equal(f.received.length,before+1);assert.deepEqual(f.received.at(-1),{method:'GET',path,body:undefined});
  assert.equal(f.proxy.observations.counts.start,0);assert.equal(f.proxy.observations.counts.check,1);
  assert.equal(f.proxy.observations.requests.at(-1).kind,'check');assert.equal(f.proxy.observations.requests.at(-1).ref,id);
  assert.deepEqual(f.proxy.observations.failures,[]);assert.equal((await f.proxy.close()).drained,true);
});

test('owned SDK stats refuses streaming malformed queries bodies methods and logical names',async t=>{
  const f=await fixture(t,recoveryOptions),labels=labelsFor('a'),id=await createCheck(f,labels),prefix=api+id+'/stats';
  const before=f.received.length;
  const cases=[
    ['GET',prefix],['GET',prefix+'?stream=0'],['GET',prefix+'?one-shot=1'],
    ['GET',prefix+'?one-shot=1&stream=1'],['GET',prefix+'?one-shot=0&stream=0'],
    ['GET',prefix+'?one-shot=true&stream=false'],['GET',prefix+'?one-shot=1&stream=0&extra=1'],
    ['GET',prefix+'?one-shot=1&stream=0&stream=0'],['GET',prefix+'?one-shot=1&one-shot=1&stream=0'],
    ['HEAD',prefix+'?one-shot=1&stream=0'],['POST',prefix+'?one-shot=1&stream=0'],['DELETE',prefix+'?one-shot=1&stream=0'],
    ['GET',prefix+'?one-shot=1&stream=0',{unexpected:'body'}],
    ['GET',api+checkName(labels)+'/stats?one-shot=1&stream=0'],
    ['GET',api+checkName(labelsFor('f'))+'/stats?one-shot=1&stream=0'],
  ];
  for(const [method,path,body] of cases)assert.equal((await f.request(method,path,body).promise).status,503,method+' '+path);
  assert.equal(f.received.length,before);assert.equal(f.proxy.observations.counts.start,0);
  assert(f.proxy.observations.failures.includes('check-query-shape-refused'));
  assert(f.proxy.observations.failures.includes('check-lifecycle-body-refused'));
});

test('versioned and unversioned owned stats retain only the validated bounded query',async t=>{
  const f=await fixture(t,recoveryOptions,(entry,res,reply)=>{
    if(entry.method==='GET'&&entry.path.includes('/stats?'))return json(res,200,{read:'controlled one-shot'});
    reply(entry,res);
  });
  const id=await createCheck(f,labelsFor('a'));
  for(const [prefix,query] of [[api,'?one-shot=1&stream=0'],['/containers/','?stream=%30&one%2Dshot=%31']]){
    const path=prefix+id+'/stats'+query,before=f.received.length;
    assert.equal((await f.request('GET',path).promise).status,200);assert.equal(f.received.length,before+1);
    const receipt=f.proxy.observations.requests.at(-1);assert.equal(receipt.kind,'check');assert.equal(receipt.ref,id);assert.equal(receipt.checkStatsQuery,query);
    assert.equal(f.received.at(-1).path,path);
  }
  assert.deepEqual(f.proxy.observations.failures,[]);assert.equal(f.proxy.observations.counts.start,0);
});

test('both owned stats routes refuse unsafe requests without exporting rejected query text',async t=>{
  const f=await fixture(t,recoveryOptions),labels=labelsFor('a'),id=await createCheck(f,labels),secret='controlled-private-query-sentinel';
  const before=f.received.length;
  for(const prefix of [api,'/containers/']){
    const path=prefix+id+'/stats';
    for(const [method,url,body] of [
      ['GET',path+'?one-shot=1&stream=0&secret='+secret],['GET',path+'?one-shot=1&stream=1'],
      ['GET',path+'?one-shot=1&stream=0&stream=0'],['GET',path+'?one-shot=1'],
      ['GET',path+'?one-shot=1&stream=0'+'&'.repeat(200)],
      ['GET',path+'?one-shot=1&stream=0',{body:secret}],['HEAD',path+'?one-shot=1&stream=0'],
      ['POST',path+'?one-shot=1&stream=0'],['DELETE',path+'?one-shot=1&stream=0'],
      ['GET',prefix+checkName(labels)+'/stats?one-shot=1&stream=0'],
    ]){
      assert.equal((await f.request(method,url,body).promise).status,503,method+' '+url);
      assert.equal(f.proxy.observations.requests.at(-1).checkStatsQuery,undefined);
    }
  }
  assert.equal(f.received.length,before);assert(!JSON.stringify(f.proxy.observations).includes(secret));assert.equal(f.proxy.observations.counts.start,0);
});

test('stats followup leaves generic foreign worker and fence GET observation behavior unchanged',async t=>{
  const f=await fixture(t,recoveryOptions,(entry,res,reply)=>{
    if(entry.method==='GET'&&entry.path.includes('/stats?'))return json(res,200,{});
    reply(entry,res);
  });
  const labels=labelsFor('a'),id=await createCheck(f,labels);
  const worker=JSON.parse((await f.request('POST',api+'create?name=controlled-worker',{Labels:{'luci.managed':'true'}}).promise).body).Id;
  assert.equal((await f.request('DELETE',api+id+'?v=1').promise).status,204);
  const marker=JSON.parse((await f.request('POST',createPath(labels),{Labels:fenceLabelsFor(labels)}).promise).body).Id;
  for(const ref of [worker,marker,'f'.repeat(64)])for(const prefix of [api,'/containers/']){
    const path=prefix+ref+'/stats?stream=1&generic=unchanged',before=f.received.length;
    assert.equal((await f.request('GET',path).promise).status,200);assert.equal(f.received.length,before+1);
    assert.equal(f.proxy.observations.requests.at(-1).checkStatsQuery,undefined);
    assert.equal((await f.request('HEAD',path).promise).status,204);assert.equal(f.received.length,before+2);
    assert.equal(f.proxy.observations.requests.at(-1).checkStatsQuery,undefined);
  }
  assert.deepEqual(f.proxy.observations.failures,[]);assert.equal(f.proxy.observations.counts.start,0);
});

test('owned SDK stats retains the shared response byte bound',async t=>{
  const f=await fixture(t,{...recoveryOptions,maxResponseBytes:256},(entry,res,reply)=>{
    if(entry.method==='GET'&&entry.path.includes('/stats?'))return json(res,200,{payload:'x'.repeat(512)});
    reply(entry,res);
  });
  const id=await createCheck(f,labelsFor('a'));
  const response=await f.request('GET',api+id+'/stats?one-shot=1&stream=0').promise;
  assert(response.error);
  // reply.destroy can synchronously emit aborted before the byte-limit label;
  // both existing paths must terminate the read, never return oversized data.
  assert(f.proxy.observations.failures.some(code=>['response-body-limit','upstream-response-incomplete'].includes(code)));
  assert.equal(response.body,undefined);
  assert.equal(f.received.length,2);assert.equal(f.proxy.observations.counts.start,0);
});

test('owned SDK stats retains upstream deadline and request concurrency limits',async t=>{
  let entered,respond;const arrived=new Promise(resolve=>{entered=resolve;});
  const f=await fixture(t,{...recoveryOptions,maxConcurrentRequests:1,upstreamTimeoutMs:250},(entry,res,reply)=>{
    if(entry.method==='GET'&&entry.path.includes('/stats?')){respond=()=>json(res,200,{});entered();return;}
    reply(entry,res);
  });
  const id=await createCheck(f,labelsFor('a')),path=api+id+'/stats?one-shot=1&stream=0';
  const first=f.request('GET',path);await arrived;
  assert.equal((await f.request('GET',path).promise).status,503);
  const response=await first.promise;assert.equal(response.status,502);
  assert.equal(f.received.length,2);assert(f.proxy.observations.failures.includes('concurrent-request-limit'));assert(f.proxy.observations.failures.includes('upstream-timeout'));
  assert.equal(f.proxy.observations.counts.start,0);respond();assert.equal((await f.proxy.close()).drained,true);
});

test('default SDK behavior still refuses named reads and a second check', async t => {
  const f = await fixture(t, {...recoveryOptions, allowCheckRecovery: false, checkCreateLimit: 1,
    maxCreatesByKind: {worker: 1, check: 1, fence: 1}});
  const labels = labelsFor('a'); await createCheck(f, labels);
  assert.equal((await f.request('GET', api + checkName(labels) + '/json').promise).status, 503);
  assert.equal((await f.request('POST', createPath(labelsFor('b')), {Labels: labelsFor('b')}).promise).status, 503);
  assert.equal(f.received.length, 1);
});

test('SDK check fences require recovery opt-in even when the original identity is known', async t => {
  const f = await fixture(t, {...recoveryOptions, allowCheckRecovery: false, checkCreateLimit: 1,
    maxCreatesByKind: {worker: 1, check: 1, fence: 1}});
  const labels = labelsFor('a'), id = await createCheck(f, labels);
  assert.equal((await f.request('DELETE', api + id + '?v=1').promise).status, 204);
  assert.equal((await f.request('POST', createPath(labels), {Labels: fenceLabelsFor(labels)}).promise).status, 503);
  assert.equal(f.received.length, 2);
  assert.equal(f.proxy.observations.requests.at(-1).failure, 'check-recovery-opt-in-required');
});

test('opted recovery allows exactly three independent checks', async t => {
  const f = await fixture(t, recoveryOptions);
  for (const digit of ['a', 'b', 'c']) await createCheck(f, labelsFor(digit));
  assert.equal((await f.request('POST', createPath(labelsFor('d')), {Labels: labelsFor('d')}).promise).status, 503);
  assert.equal(f.received.length, 3); assert.equal(f.proxy.observations.counts.worker, 0);
});

test('known-name inspection is read-only; malformed/foreign names and queries never forward', async t => {
  const f = await fixture(t, recoveryOptions), labels = labelsFor('a');
  const id = await createCheck(f, labels), name = checkName(labels);
  const response = await f.request('GET', api + name + '/json').promise;
  assert.equal(response.status, 200); assert.equal(JSON.parse(response.body).Id, id);
  const before = f.received.length;
  for (const [method, path] of [
    ['POST', api + name + '/start'], ['POST', api + name + '/stop'], ['DELETE', api + name + '?v=1&force=1'],
    ['HEAD', api + name + '/json'], ['GET', api + name + '/json?size=1'],
    ['GET', api + checkName(labelsFor('f')) + '/json'], ['GET', api + 'luci-check-malformed/json'],
  ]) assert.equal((await f.request(method, path).promise).status, 503, method + ' ' + path);
  assert.equal(f.received.length, before);
});

test('full check fence identity survives and marker Start/Stop/Delete are refused', async t => {
  const events = [], f = await fixture(t, {...recoveryOptions, journal: {record(event, row) {events.push({event, ...structuredClone(row)});}}});
  const labels = labelsFor('a'), id = await createCheck(f, labels), fenceLabels = fenceLabelsFor(labels);
  assert.equal((await f.request('DELETE', api + id + '?v=1&force=1').promise).status, 204);
  const created = await f.request('POST', createPath(labels), {Labels: fenceLabels}).promise;
  assert.equal(created.status, 201); const marker = JSON.parse(created.body).Id;
  const ack = events.find(row => row.event === 'create-ack' && row.id === marker);
  assert.deepEqual(ack.labels, fenceLabels); assert.equal(Object.keys(ack.labels).length, 13);
  assert.equal(JSON.parse((await f.request('GET', api + checkName(labels) + '/json').promise).body).Id, marker);
  assert.equal(JSON.parse((await f.request('GET', api + marker + '/json').promise).body).Id, marker);
  const before = f.received.length;
  for (const [method, path] of [['POST', api + marker + '/start'], ['POST', api + marker + '/stop'],
    ['DELETE', api + marker], ['DELETE', api + marker + '?v=1&force=1'], ['DELETE', api + checkName(labels) + '?v=1']])
    assert.equal((await f.request(method, path).promise).status, 503);
  assert.equal(f.received.length, before); assert(f.containers.has(marker));
  assert(f.proxy.observations.failures.includes('permanent-check-fence-delete-forbidden'));
  assert.equal((await f.proxy.close()).drained, true);
});

test('malformed or unbound check fence labels cannot reach the upstream', async t => {
  const labels = labelsFor('a'), good = fenceLabelsFor(labels);
  for (const bad of [
    {...good, extra: 'value'}, {...good, 'luci.check-project': 'other-project'},
    {...good, 'luci.fence-intent': 'invalid'}, {...good, 'luci.fence-binding': 'invalid'},
    Object.fromEntries(Object.entries(good).filter(([key]) => key !== 'luci.check-engine')),
  ]) {
    const f = await fixture(t, recoveryOptions); await createCheck(f, labels);
    assert.equal((await f.request('POST', createPath(labels), {Labels: bad}).promise).status, 503);
    assert.equal(f.received.length, 1);
  }
  const unbound = await fixture(t, recoveryOptions);
  assert.equal((await unbound.request('POST', createPath(labels), {Labels: good}).promise).status, 503);
  assert.equal(unbound.received.length, 0);
});

test('one lost real ACK is drained once, retained for inspection, and never replayed', async t => {
  const f = await fixture(t, recoveryOptions), labels = labelsFor('a');
  f.proxy.armGate({id: 'lost-once', phase: 'after-response', method: 'POST', decision: 'lose-response', match: row => row.kind === 'check'});
  const request = f.request('POST', createPath(labels), {Labels: labels});
  const lost = await f.proxy.waitForGate('lost-once');
  assert.equal(lost.request.status, 201); assert.match(lost.request.id, /^[a-f0-9]{64}$/);
  assert((await request.promise).error); assert.equal(f.received.length, 1);
  assert.equal((await f.request('GET', api + checkName(labels) + '/json').promise).status, 200);
  await assert.rejects(f.proxy.releaseGate('lost-once'), /gate-not-held/);
  assert.throws(() => f.proxy.armGate({id: 'lost-once', phase: 'after-response', method: 'POST'}), /duplicate-gate/);
  assert.equal(f.received.filter(row => row.method === 'POST').length, 1);
  const cleanup = await f.proxy.close();
  assert.equal(cleanup.drained, true); assert.equal(cleanup.pendingRequests, 0); assert.equal(cleanup.activeHandlers, 0);
  assert.deepEqual(f.proxy.observations.failures, []);
});

test('held witness releases once through fresh guard and unsent cleanup cancels it', async t => {
  let guarded = 0;
  const f = await fixture(t, {...recoveryOptions, beforeCreate() {guarded++;}}), labels = labelsFor('b');
  f.proxy.armGate({id: 'held-witness', phase: 'before-forward', method: 'POST'});
  const request = f.request('POST', createPath(labels), {Labels: labels});
  await f.proxy.waitForGate('held-witness'); assert.equal(guarded, 0); assert.equal(f.received.length, 0);
  assert.equal((await f.proxy.releaseGate('held-witness')).status, 201);
  assert.equal((await request.promise).status, 201); assert.equal(guarded, 1);
  await assert.rejects(f.proxy.releaseGate('held-witness'), /gate-not-held/);
  f.proxy.armGate({id: 'cancel-witness', phase: 'before-forward', method: 'POST'});
  const unsent = f.request('POST', createPath(labelsFor('c')), {Labels: labelsFor('c')});
  await f.proxy.waitForGate('cancel-witness');
  const closed = await f.proxy.close(); await unsent.promise;
  assert.equal(closed.drained, true); assert.equal(closed.pendingRequests, 0); assert.equal(closed.activeHandlers, 0);
  assert.equal(guarded, 1); assert.equal(f.received.length, 1);
  assert(f.proxy.observations.requests.at(-1).cancelledBeforeForwardAt);
});

test('a held witness fails before forwarding when the fresh admission guard refuses', async t => {
  let currentAdmission = true, guardCalls = 0;
  const f = await fixture(t, {...recoveryOptions, beforeCreate() {
    guardCalls++; if (!currentAdmission) throw Object.assign(new Error('current-admission-refused'), {code: 'current-admission-refused'});
  }}), labels = labelsFor('b');
  f.proxy.armGate({id: 'held-admission', phase: 'before-forward', method: 'POST'});
  const request = f.request('POST', createPath(labels), {Labels: labels});
  await f.proxy.waitForGate('held-admission'); currentAdmission = false;
  assert.equal((await f.proxy.releaseGate('held-admission')).proxyStatus, 503);
  assert.equal((await request.promise).status, 503); assert.equal(guardCalls, 1);
  assert.equal(f.received.length, 0); assert.equal(f.proxy.observations.requests[0].forwardedAt, undefined);
  assert.equal((await f.proxy.close()).drained, true);
});

test('close drains an already forwarded check mutation after downstream disconnect', async t => {
  let entered, respond;
  const ready = new Promise(resolve => {entered = resolve;});
  const f = await fixture(t, recoveryOptions, (entry, response, reply) => {respond = () => reply(entry, response); entered();});
  const labels = labelsFor('a'), request = f.request('POST', createPath(labels), {Labels: labels});
  await ready; request.destroy(); await request.promise;
  const closed = f.proxy.close(); respond();
  assert.equal((await closed).drained, true); assert.equal(f.received.length, 1);
  assert.match(f.proxy.observations.requests[0].id, /^[a-f0-9]{64}$/);
});

// Execute all original pinned transport tests unchanged except their three
// import targets. Support helpers are loaded from the same pin into memory;
// no current checkout helper implementation is imported or copied.
let originalTests = pinned('fenced-docker-proxy.test.mjs');
originalTests = originalTests.replace("'./fenced-docker-proxy.mjs'", JSON.stringify(proxyURL.href))
  .replace("'./durable-acceptance-journal.mjs'", JSON.stringify(dataModule(pinned('durable-acceptance-journal.mjs'))))
  .replace("'./acceptance-lifecycle.mjs'", JSON.stringify(dataModule(pinned('acceptance-lifecycle.mjs'))));
await import(dataModule(originalTests));
