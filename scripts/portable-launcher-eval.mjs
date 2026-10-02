// Exercise the shipped portable launcher, with explicit disposable state and no browser.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { mkdir, readFile, stat, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { pathToFileURL } from 'node:url';

export async function evaluatePortableLauncher(packageDirectory, evidenceDirectory) {
  const packageDir = path.resolve(packageDirectory), evidence = path.resolve(evidenceDirectory);
  assert.match(evidence.replaceAll('\\', '/'), /\/artifacts\/package-eval\//, 'Use a disposable package-eval directory');
  const fixture = path.join(evidence, 'launcher'), state = path.join(fixture, 'state');
  const projects = path.join(fixture, 'projects'), home = path.join(fixture, 'home');
  await mkdir(evidence, { recursive: true });
  await mkdir(fixture, { mode: 0o700 }); // Refuse reuse of a previous acceptance fixture.
  for (const dir of [state, projects, home]) await mkdir(dir, { mode: 0o700 });
  const listener = createServer();
  await new Promise((resolve, reject) => listener.once('error', reject).listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  const windows = process.platform === 'win32';
  const launcher = path.join(packageDir, windows ? 'Start-Luci.ps1' : 'start-luci.sh');
  const binary = path.join(packageDir, windows ? 'luci.exe' : 'luci');
  const hash = data => createHash('sha256').update(data).digest('hex');
  const result = { passed: false, platform: process.platform, architecture: process.arch, nodeVersion: process.version,
    packageDir, binarySha256: hash(await readFile(binary)), launcherSha256: hash(await readFile(launcher)),
    harnessSha256: hash(await readFile(new URL(import.meta.url))),
    startedAt: new Date().toISOString(), checks: [], cloudCalls: 0, modelCalls: 0,
    scope: 'Actual shipped launcher and service; explicit disposable profile, no browser opening or native GUI.' };
  const secrets = new Set(), launched = [];
  const secret = () => { const value = randomUUID() + randomUUID(); secrets.add(value); return value; };
  const redact = value => { let text = String(value); for (const item of secrets) text = text.replaceAll(item, '[redacted]'); return text; };
  const pass = name => result.checks.push({ name, status: 'passed' });
  const proofPath = path.join(state, 'control-plane', 'owner-access-token');
  const windowsSystemKeys = /^(PROGRAMFILES(?:\(X86\))?|PROGRAMW6432|PROGRAMDATA|ALLUSERSPROFILE|SYSTEMDRIVE)$/i;
  const inherited = Object.fromEntries(Object.entries(process.env).filter(([key]) => /^(PATH|SYSTEMROOT|WINDIR|COMSPEC|PATHEXT|TEMP|TMP|LANG|LC_ALL)$/i.test(key) || (windows && windowsSystemKeys.test(key))));
  const environment = { ...inherited, HOME: home, USERPROFILE: home, LOCALAPPDATA: path.join(home, 'Local'), APPDATA: path.join(home, 'Roaming'),
    XDG_CONFIG_HOME: path.join(home, '.config'), XDG_DATA_HOME: path.join(home, '.local/share'), XDG_STATE_HOME: path.join(home, '.local/state'),
    GIT_CONFIG_GLOBAL: path.join(home, 'gitconfig'), GIT_CONFIG_NOSYSTEM: '1',
    GH_CONFIG_DIR: path.join(home, 'gh'), AZURE_CONFIG_DIR: path.join(home, 'azure'),
    AWS_CONFIG_FILE: path.join(home, 'aws-config'), AWS_SHARED_CREDENTIALS_FILE: path.join(home, 'aws-credentials'), AWS_EC2_METADATA_DISABLED: 'true' };
  if (windows) {
    // Keep the account/credential profile private without withholding Windows'
    // own runtime paths. Never inherit user or PowerShell 7 module search paths.
    const systemRoot = Object.entries(inherited).find(([key]) => /^SYSTEMROOT$/i.test(key))?.[1];
    assert.ok(systemRoot && path.isAbsolute(systemRoot), 'Windows system root is required');
    environment.PSModulePath = path.join(systemRoot, 'System32', 'WindowsPowerShell', 'v1.0', 'Modules');
    for (const directory of [environment.LOCALAPPDATA, environment.APPDATA]) await mkdir(directory, { mode: 0o700 });
    result.environmentProfile = { kind: 'private-user-profile-with-windows-system-modules',
      systemPaths: Object.fromEntries(Object.entries(inherited).filter(([key]) => windowsSystemKeys.test(key) || /^SYSTEMROOT$/i.test(key))),
      powershellModulePath: environment.PSModulePath, inheritedModulePath: false, privateProfileDirectoriesCreated: true,
      privatePaths: { HOME: '<fixture>/home', USERPROFILE: '<fixture>/home', LOCALAPPDATA: '<fixture>/home/Local', APPDATA: '<fixture>/home/Roaming' },
      credentialPaths: 'Git, GitHub, AWS and Azure remain under the private fixture home', preInvocationWarmup: false };
  }
  let ownerToken = '', proof = '', owned;
  const ownerPassword = secret(), recoveredPassword = secret();
  async function api(method, route, body, { token = ownerToken, capability = '', status = 200 } = {}) {
    const response = await fetch(base + route, { method,
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(capability ? { 'X-Luci-Local-Access-Token': capability } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(15000) });
    assert.equal(response.status, status, `${method} ${route} status`);
    const value = await response.json();
    if (value.token) secrets.add(value.token);
    return value;
  }
  async function waitUntil(check, message, timeout = 45000) {
    const end = Date.now() + timeout;
    while (Date.now() < end) { if (await check()) return; await delay(100); }
    throw new Error(message);
  }
  function start(stateDirectory = state) {
    const executable = windows ? 'powershell.exe' : 'bash';
    const args = windows ? ['-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', launcher, '-Port', String(port), '-StateDir', stateDirectory, '-NoBrowser']
      : [launcher, '--port', String(port), '--state-dir', stateDirectory, '--no-browser'];
    const startedAt = Date.now();
    const child = spawn(executable, args, { cwd: packageDir, env: environment, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
    const record = { child, output: '', spawnError: null, closed: false, index: launched.length, stateDirectory, startedAt };
    launched.push(record);
    child.once('error', error => { record.spawnError = error; });
    child.once('close', () => { record.closed = true; });
    child.stdin.on('error', () => {});
    for (const stream of [child.stdout, child.stderr]) stream.on('data', data => { if (record.output.length < 131072) record.output += data.toString(); });
    return record;
  }
  const exited = record => record.child.exitCode !== null || record.child.signalCode !== null || record.spawnError;
  async function rememberFixtureCodes() {
    for (const record of launched) for (const name of ['owner-access-token', 'bootstrap-token']) {
      const filename = path.join(record.stateDirectory, 'control-plane', name);
      try {
        if ((await stat(filename)).size > 128) continue;
        const code = (await readFile(filename, 'utf8')).trim();
        if (/^[A-Za-z0-9_-]{43}$/.test(code)) secrets.add(code);
      } catch (error) { if (error.code !== 'ENOENT') (result.diagnosticErrors ||= []).push(`Cannot inspect private fixture code: ${error.code || 'unknown'}`); }
    }
  }
  async function failureDiagnostics() {
    // Read only this acceptance fixture's code files so a failure before the
    // normal proof check cannot accidentally publish a private startup code.
    await rememberFixtureCodes();
    let service;
    try {
      const response = await fetch(base + '/v1/control-plane/status', { signal: AbortSignal.timeout(2000) });
      const value = await response.json();
      service = { status: response.status, product: String(value.product || '').slice(0, 80), version: String(value.version || '').slice(0, 80), configured: value.configured, bootstrapRequired: value.bootstrapRequired };
    } catch (error) { service = { error: redact(error.message) }; }
    const logs = [];
    for (const name of ['service.out.log', 'service.err.log']) {
      try {
        const filename = path.join(state, name), info = await stat(filename);
        logs.push(info.size > 1048576 ? { name, bytes: info.size, omitted: 'over diagnostic bound' }
          : { name, bytes: info.size, tail: redact(await readFile(filename, 'utf8')).slice(-16384) });
      } catch (error) { logs.push({ name, error: error.code || 'unknown' }); }
    }
    return { observedBeforeCleanup: true, service, serviceLogs: logs, launchers: launched.map(record => ({ index: record.index, pid: record.child.pid, exitCode: record.child.exitCode, signal: record.child.signalCode, closed: record.closed, spawnError: record.spawnError ? redact(record.spawnError.message) : null, outputTail: redact(record.output).slice(-16384) })) };
  }
  async function ready(record) {
    // Hosted Windows cold command discovery took 29.9s; genuine verified
    // launcher readiness followed by clean exit was observed by 48.5s. Allow
    // this first fixture start more time; later starts and Unix retain 45s.
    const startupBudgetMs = windows && record.index === 0 ? 90000 : 45000;
    if (record.index === 0) result.startupBudgetMs = startupBudgetMs;
    await waitUntil(async () => {
      if (record.spawnError) throw record.spawnError;
      assert.ok(!exited(record), 'Owned launcher exited before readiness');
      return record.output.includes(`Luci is running at ${base}`);
    }, 'Launcher did not announce a ready, verified workspace', startupBudgetMs);
    const status = await api('GET', '/v1/control-plane/status', undefined, { token: '' });
    assert.equal(status.product, 'Luci');
    assert.equal(status.version, process.env.LUCI_VERSION || '0.4.2');
    if (record.index === 0) result.startupElapsedMs = Date.now() - record.startedAt;
    result.version = status.version;
    return status;
  }
  async function readProof() {
    const value = (await readFile(proofPath, 'utf8')).trim();
    assert.match(value, /^[A-Za-z0-9_-]{43}$/);
    secrets.add(value);
    if (!windows) assert.equal((await stat(proofPath)).mode & 0o777, 0o600);
    return value;
  }
  async function closeLauncher(record, isOwner) {
    if (!exited(record)) {
      if (windows) record.child.stdin.end('\n');
      else record.child.kill('SIGINT');
    }
    // close follows exit only after stdout/stderr drain, including short Unix reuse.
    await waitUntil(() => record.closed, 'Launcher did not exit after normal close', 20000);
    if (windows) assert.equal(record.child.exitCode, 0, 'Windows launcher close failed');
    else assert.ok([0, 130].includes(record.child.exitCode) || record.child.signalCode === 'SIGINT', 'Unix launcher close failed');
    if (isOwner) {
      await waitUntil(async () => { try { await stat(proofPath); return false; } catch (error) { if (error.code === 'ENOENT') return true; throw error; } }, 'Owner proof survived service shutdown', 10000);
      await assert.rejects(fetch(base + '/v1/control-plane/status', { signal: AbortSignal.timeout(1500) }), 'Owned listener survived launcher shutdown');
    }
  }
  try {
    owned = start();
    const initial = await ready(owned);
    assert.equal(initial.bootstrapRequired, true);
    proof = await readProof();
    assert.equal((await api('GET', '/v1/auth/local-owner', undefined, { token: '', capability: proof })).owner, null);
    await api('GET', '/v1/auth/local-owner', undefined, { token: '', status: 403 });
    const bootstrap = (await readFile(path.join(state, 'control-plane/bootstrap-token'), 'utf8')).trim();
    secrets.add(bootstrap);
    const auth = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'launcher-owner', displayName: 'Launcher Owner', password: ownerPassword }, { token: '', status: 201 });
    ownerToken = auth.token;
    const ownerId = auth.user.id;
    pass('Shipped launcher started its own loopback service and exposed only privately proven owner access');
    await api('POST', '/v1/setup/config', { projectsDir: projects, mode: 'local', cloudProvider: '', region: '', domain: '', monthlyBudgetUsd: 30 });
    const project = await api('POST', '/v1/product/projects', { name: 'launcher-retained-work' }, { status: 201 });
    await api('POST', `/v1/product/projects/${project.id}/brief`, { revision: 0, purpose: 'Portable launcher owner recovery retains real work', audience: 'Portable users', constraints: 'No cloud or model calls', criteria: ['Retain this brief'] });
    const projectFile = path.join(project.path, 'LUCI.md'), projectHash = hash(await readFile(projectFile));
    const extra = await api('POST', '/v1/auth/local-account', { username: 'launcher-second', displayName: 'Second Account', password: secret() }, { token: '', capability: proof, status: 201 });
    assert.notEqual(extra.user.id, ownerId);
    assert.equal((await api('GET', '/v1/auth/local-owner', undefined, { token: '', capability: proof })).owner.id, ownerId);
    const oldToken = ownerToken;
    const recovered = await api('POST', '/v1/auth/local-owner/recover', { newPassword: recoveredPassword }, { token: '', capability: proof });
    assert.equal(recovered.user.id, ownerId);
    ownerToken = recovered.token;
    await api('GET', '/v1/auth/me', undefined, { token: oldToken, status: 401 });
    await api('POST', '/v1/auth/login', { username: 'launcher-owner', password: ownerPassword }, { token: '', status: 401 });
    assert.equal(hash(await readFile(projectFile)), projectHash);
    pass('Private launcher proof created a distinct account and recovered the original owner without changing saved work');
    const reused = start();
    if (windows) await ready(reused);
    else await waitUntil(() => reused.closed, 'Reused Unix launcher did not return');
    assert.match(reused.output, /already running/);
    await closeLauncher(reused, false);
    assert.equal((await api('GET', '/v1/auth/me')).user.id, ownerId);
    pass('Closing a second verified launcher preserved the already running service');
    const foreignState = path.join(fixture, 'other-state');
    await mkdir(path.join(foreignState, 'control-plane'), { recursive: true, mode: 0o700 });
    const wrongProof = randomBytes(32).toString('base64url'); secrets.add(wrongProof);
    await writeFile(path.join(foreignState, 'control-plane/owner-access-token'), wrongProof + '\n', { mode: 0o600 });
    const rejected = start(foreignState);
    await waitUntil(() => rejected.closed, 'Launcher did not reject a different state directory');
    assert.notEqual(rejected.child.exitCode, 0, 'Another state directory acquired the running service');
    assert.match(rejected.output, /does not belong to this launcher/);
    assert.equal((await api('GET', '/v1/auth/me')).user.id, ownerId);
    pass('Another state directory failed private proof verification without resetting data or stopping the service');
    await closeLauncher(owned, true);
    assert.equal(hash(await readFile(projectFile)), projectHash);
    pass('Normal launcher close removed its proof and listener while retaining project data');
    owned = start();
    assert.equal((await ready(owned)).bootstrapRequired, false);
    const oldProof = proof;
    proof = await readProof();
    assert.notEqual(proof, oldProof);
    await api('GET', '/v1/auth/local-owner', undefined, { token: '', capability: oldProof, status: 403 });
    assert.equal((await api('GET', '/v1/auth/local-owner', undefined, { token: '', capability: proof })).owner.id, ownerId);
    ownerToken = (await api('POST', '/v1/auth/login', { username: 'launcher-owner', password: recoveredPassword }, { token: '' })).token;
    assert.equal((await api('GET', `/v1/product/projects/${project.id}/brief`)).revision, 1);
    assert.equal(hash(await readFile(projectFile)), projectHash);
    await closeLauncher(owned, true);
    pass('Restart rotated private proof and retained the recovered account and saved project');
    for (const record of launched) for (const item of secrets) assert.ok(!record.output.includes(item), 'Launcher printed a private credential');
    pass('Launcher output contained no owner proof, bootstrap token, password or bearer');
    result.passed = true;
  } catch (error) {
    try { result.failureDiagnostics = await failureDiagnostics(); }
    catch (diagnosticError) { (result.diagnosticErrors ||= []).push(redact(diagnosticError.message)); }
    result.error = redact(error.stack || error);
    throw new Error(result.error);
  } finally {
    // Only children created above are eligible for cleanup. Preserve the first failure.
    for (const record of launched) if (!exited(record)) {
      try { await closeLauncher(record, record === owned); }
      catch (error) { (result.cleanupErrors ||= []).push(redact(error.message)); record.child.kill(); }
    }
    await rememberFixtureCodes();
    if (!result.passed) result.launcherOutputAfterCleanup = launched.map(record => ({ index: record.index, exitCode: record.child.exitCode, signal: record.child.signalCode, outputTail: redact(record.output).slice(-16384) }));
    result.finishedAt = new Date().toISOString();
    await writeFile(path.join(evidence, 'launcher-results.json'), JSON.stringify(result, null, 2));
    await writeFile(path.join(evidence, 'launcher.log'), redact(launched.map(record => `Launcher ${record.index}: exit=${record.child.exitCode}, signal=${record.child.signalCode}\n${record.output}`).join('\n')));
  }
  return result;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  assert.ok(process.argv[2] && process.argv[3], 'Usage: node scripts/portable-launcher-eval.mjs <extracted package> <artifacts/package-eval/new-directory>');
  const result = await evaluatePortableLauncher(process.argv[2], process.argv[3]);
  console.log(JSON.stringify({ passed: result.passed, platform: result.platform, checks: result.checks.length }));
}
