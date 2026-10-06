// Run the actual packaged service with isolated state; no model or cloud calls.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { mkdir, readFile, writeFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { randomUUID } from 'node:crypto';
import { evaluatePortableLauncher } from './portable-launcher-eval.mjs';

const expectedVersion = process.env.LUCI_VERSION;
assert.match(expectedVersion || '', /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/, 'Set LUCI_VERSION to the exact candidate version before package acceptance');
const packageDir = path.resolve(process.argv[2] || '');
const evidence = path.resolve(process.argv[3] || `artifacts/package-eval/${Date.now()}`);
assert.ok(process.argv[2], 'Usage: node scripts/verify-portable.mjs <extracted package> [evidence directory]');
assert.match(evidence.replaceAll('\\', '/'), /\/artifacts\/package-eval\//, 'Use a disposable artifacts/package-eval directory');
await mkdir(evidence, { recursive: true });
const projects = path.join(evidence, 'projects'), state = path.join(evidence, 'state');
await mkdir(projects); // Refuse reusing a previous eval's repositories.
for (const file of ['package-identity', 'web/index.html', 'runtime/images/sandbox/Dockerfile', 'runtime/sandboxd/vendor/modules.txt', 'cloud/luci-cloud-linux-amd64.tar.gz', 'licenses/manifest.json', 'docs/luci/MANAGED-UPGRADE.md']) {
  assert.ok((await stat(path.join(packageDir, file))).size > 0, `Missing shipped file ${file}`);
}
const licenses = JSON.parse(await readFile(path.join(packageDir, 'licenses/manifest.json'), 'utf8'));
assert.equal(licenses.missingLicenses.length, 0);
assert.equal((await readFile(path.join(packageDir, 'licenses/MISSING-LICENSES.txt'), 'utf8')).trim(), '');
const listener = createServer();
await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
const address = `127.0.0.1:${listener.address().port}`;
await new Promise(resolve => listener.close(resolve));
const base = `http://${address}`;
const binary = path.join(packageDir, process.platform === 'win32' ? 'luci.exe' : 'luci');
const checks = [], logs = [];
let child;
let token = '';
const password = randomUUID() + randomUUID();
async function request(method, route, body) {
  const headers = body ? {'Content-Type':'application/json'} : {};
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(base + route, { method, headers, body: body ? JSON.stringify(body) : undefined, signal: AbortSignal.timeout(45000) });
  const value = await res.json();
  assert.ok(res.ok, `${method} ${route}: ${res.status} ${JSON.stringify(value)}`);
  return value;
}
async function start() {
  child = spawn(binary, [], {cwd:packageDir, windowsHide:true, env:{...process.env, LUCI_ADDR:address, LUCI_STATE_DIR:state, LUCI_PROJECTS_DIR:projects, LUCI_WEB_DIR:path.join(packageDir,'web'), LUCI_RUNTIME_DIR:path.join(packageDir,'runtime')}});
  child.on('error', error => logs.push(String(error)));
  for (const stream of [child.stdout, child.stderr]) stream.on('data', data => { if (logs.join('').length < 65536) logs.push(data.toString()); });
  const until = Date.now() + 60000;
  while (Date.now() < until) {
    assert.equal(child.exitCode, null, 'Packaged service exited before readiness');
    try {
      const control = await request('GET','/v1/control-plane/status');
      assert.equal(control.product,'Luci');assert.equal(control.version,expectedVersion);
      if (control.bootstrapRequired) {
        const code = (await readFile(path.join(state,'control-plane/bootstrap-token'),'utf8')).trim();
        token = (await request('POST','/v1/auth/bootstrap',{bootstrapToken:code,username:'package-owner',displayName:'Package Owner',password})).token;
      }
      return await request('GET','/v1/setup/status');
    } catch {}
    await delay(300);
  }
  throw new Error('Packaged service did not become ready');
}
async function stop() {
  if (!child || child.exitCode !== null) return;
  await request('POST','/v1/setup/shutdown');
  const until=Date.now()+15000;
  while(child.exitCode===null&&Date.now()<until)await delay(100);
  assert.notEqual(child.exitCode,null,'Owned service did not stop cleanly');
  assert.equal(child.exitCode,0,'Owned service shutdown failed');
}
try {
  await start();
  const anonymous = await fetch(base+'/v1/product/projects');assert.equal(anonymous.status,401);
  assert.equal((await request('GET','/v1/auth/me')).user.role,'super-admin');
  checks.push('Created the first Super Admin and rejected unauthenticated project access');
  const page = await fetch(base + '/');
  assert.ok(page.ok);assert.match(await page.text(), /Luci/);
  checks.push('Started packaged service and served shipped web assets without Go or Node compilation');
  await request('POST','/v1/setup/config',{projectsDir:projects,mode:'local',cloudProvider:'',region:'',domain:'',monthlyBudgetUsd:30});
  const project = await request('POST','/v1/product/projects',{name:'package-eval'});
  const route = `/v1/product/projects/${encodeURIComponent(project.id)}`;
  await request('POST',route+'/brief',{revision:0,purpose:'Verify packaged Luci persistence',audience:'People using the download',constraints:'No cloud or model calls',criteria:['Survive service restart']});
  await request('POST',route+'/design',{revision:0,frames:[],scene:{elements:[{id:'eval-rectangle',type:'rectangle',x:20,y:20,width:120,height:80}],appState:{},files:{}}});
  await request('POST',route+'/plan',{expectedRevision:0,tasks:[{id:'persist',title:'Verify full project persistence',status:'doing'}]});
  assert.match(await readFile(path.join(project.path,'LUCI.md'),'utf8'),/Verify packaged Luci/);
  checks.push('Created a real Git project, saved brief/design and generated LUCI.md');
  await stop();
  const restarted = await start();
  assert.equal(restarted.configured,true);
  assert.equal(restarted.config.projectsDir,projects);
  assert.equal((await request('GET',route+'/brief')).revision,1);
  const design = await request('GET',route+'/design');
  assert.equal((await request('GET',route+'/plan')).tasks[0].title,'Verify full project persistence');
  assert.equal((await request('GET','/v1/auth/me')).user.username,'package-owner');
  assert.equal(design.briefRevision,1);assert.equal(design.scene.elements[0].id,'eval-rectangle');
  checks.push('Recovered configured project, brief and drawing after a clean process restart');
  const foreign = await fetch(base+'/v1/product/projects',{method:'POST',headers:{Origin:'https://unrelated.example','Content-Type':'application/json'},body:JSON.stringify({name:'foreign'})});
  assert.equal(foreign.status,403);
  checks.push('Rejected a foreign browser origin at the shipped control API');
  await stop();
  const launcher = await evaluatePortableLauncher(packageDir, evidence);
  checks.push(`Verified shipped launcher ownership, account recovery, reuse and cleanup (${launcher.checks.length} outcomes)`);
  await writeFile(path.join(evidence,'results.json'),JSON.stringify({status:'passed',platform:process.platform,packageDir,checks,licenseComponents:licenses.components.length,cloudCalls:0,modelCalls:0,dockerAcceptance:false},null,2));
  console.log(JSON.stringify({status:'passed',checks,evidence}));
} finally {
  if(child&&child.exitCode===null)child.kill();
  await writeFile(path.join(evidence,'service.log'),logs.join(''));
}
