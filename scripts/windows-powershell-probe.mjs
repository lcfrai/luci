// Diagnostic only: localize cold Windows PowerShell startup in the same
// disposable environment as portable-launcher-eval. This is not app acceptance.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

assert.equal(process.platform, 'win32', 'This diagnostic is Windows-only');
assert.equal(process.env.GITHUB_ACTIONS, 'true', 'Use a disposable hosted CI runner');
assert.ok(process.argv[2] && process.argv[3], 'Usage: node windows-powershell-probe.mjs <package> <evidence>');
const packageDir = path.resolve(process.argv[2]), evidence = path.resolve(process.argv[3]);
assert.match(evidence.replaceAll('\\', '/'), /\/artifacts\/package-eval\//);
const fixture = path.join(evidence, 'powershell-probe'), home = path.join(fixture, 'home');
await mkdir(evidence, { recursive: true });
await mkdir(fixture, { mode: 0o700 }); // Refuse any previous fixture.
for (const directory of [home, path.join(fixture, 'state'), path.join(fixture, 'projects')]) await mkdir(directory, { mode: 0o700 });
const inherited = Object.fromEntries(Object.entries(process.env).filter(([key]) => /^(PATH|SYSTEMROOT|WINDIR|COMSPEC|PATHEXT|TEMP|TMP|LANG|LC_ALL)$/i.test(key)));
const environment = { ...inherited, HOME: home, USERPROFILE: home, LOCALAPPDATA: path.join(home, 'Local'), APPDATA: path.join(home, 'Roaming'),
  XDG_CONFIG_HOME: path.join(home, '.config'), XDG_DATA_HOME: path.join(home, '.local/share'), XDG_STATE_HOME: path.join(home, '.local/state'),
  GIT_CONFIG_GLOBAL: path.join(home, 'gitconfig'), GIT_CONFIG_NOSYSTEM: '1',
  GH_CONFIG_DIR: path.join(home, 'gh'), AZURE_CONFIG_DIR: path.join(home, 'azure'),
  AWS_CONFIG_FILE: path.join(home, 'aws-config'), AWS_SHARED_CREDENTIALS_FILE: path.join(home, 'aws-credentials'), AWS_EC2_METADATA_DISABLED: 'true' };
const reservation = createServer();
await new Promise((resolve, reject) => reservation.once('error', reject).listen(0, '127.0.0.1', resolve));
const port = reservation.address().port;
await new Promise(resolve => reservation.close(resolve));
const script = `$ErrorActionPreference = 'Stop'
[Console]::Out.WriteLine('HOST_READY')
[Console]::Out.Flush()
[Console]::Out.WriteLine('HTTP_PROBE_START')
[Console]::Out.Flush()
try {
  Invoke-RestMethod -Uri 'http://127.0.0.1:${port}/v1/control-plane/status' -TimeoutSec 2 | Out-Null
  [Console]::Out.WriteLine('HTTP_UNEXPECTED_SUCCESS')
  exit 2
} catch {
  if ($_.Exception -is [System.Net.WebException] -and $_.Exception.Status -eq [System.Net.WebExceptionStatus]::ConnectFailure) {
    [Console]::Out.WriteLine('HTTP_EXPECTED_CONNECTION_FAILURE')
  } else {
    [Console]::Out.WriteLine('HTTP_UNEXPECTED_ERROR:' + $_.Exception.GetType().FullName)
    exit 3
  }
}
[Console]::Out.WriteLine('HTTP_PROBE_DONE')
[Console]::Out.Flush()
exit 0`;
const result = { diagnosticOnly: true, appAcceptance: false, status: 'running', startedAt: new Date().toISOString(),
  packageDir, scriptSHA256: createHash('sha256').update(script).digest('hex'),
  inheritedEnvironmentKeys: Object.keys(inherited).sort(), fixtureEnvironmentKeys: Object.keys(environment).sort(),
  timeoutMs: 60000, output: '', events: [], cloudCalls: 0, modelCalls: 0,
  limitation: 'Any later launcher acceptance in this job is post-probe and is not cold-start evidence.' };
const started = Date.now();
const child = spawn('powershell.exe', ['-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', script],
  { cwd: packageDir, env: environment, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
result.pid = child.pid;
let closed = false, spawnError;
child.once('error', error => { spawnError = error; });
child.once('close', () => { closed = true; });
child.stdin.on('error', () => {});
for (const [name, stream] of [['stdout', child.stdout], ['stderr', child.stderr]]) stream.on('data', data => {
  const text = data.toString();
  if (!result.firstOutputAt) { result.firstOutputAt = new Date().toISOString(); result.firstOutputElapsedMs = Date.now() - started; }
  if (result.output.length < 16384) result.output += text.slice(0, 16384 - result.output.length);
  if (result.events.length < 50) result.events.push({ stream: name, at: new Date().toISOString(), elapsedMs: Date.now() - started, text: text.slice(0, 2048) });
});
try {
  while (!closed && !spawnError && Date.now() - started < result.timeoutMs) await delay(100);
  result.observedBeforeCleanup = { elapsedMs: Date.now() - started, exitCode: child.exitCode, signal: child.signalCode, closed,
    spawnError: spawnError?.message || null, hostReady: result.output.includes('HOST_READY'), httpProbeStarted: result.output.includes('HTTP_PROBE_START'), httpProbeDone: result.output.includes('HTTP_PROBE_DONE') };
  result.status = !closed && !spawnError ? 'timed_out' : child.exitCode === 0 && result.observedBeforeCleanup.httpProbeDone ? 'passed' : 'failed';
} finally {
  if (!closed) {
    child.stdin.end();
    const normalDeadline = Date.now() + 2000;
    while (!closed && Date.now() < normalDeadline) await delay(100);
    if (!closed) { result.forcedOwnedChildCleanup = true; child.kill('SIGKILL'); }
    const killDeadline = Date.now() + 5000;
    while (!closed && Date.now() < killDeadline) await delay(100);
  }
  result.childClosedAfterCleanup = closed;
  result.exitCodeAfterCleanup = child.exitCode;
  result.signalAfterCleanup = child.signalCode;
  result.finishedAt = new Date().toISOString();
  await writeFile(path.join(evidence, 'powershell-startup-probe.json'), JSON.stringify(result, null, 2));
}
console.log(JSON.stringify({ diagnosticOnly: true, status: result.status, hostReady: result.observedBeforeCleanup.hostReady, httpProbeDone: result.observedBeforeCleanup.httpProbeDone, childClosedAfterCleanup: closed }));
if (result.status !== 'passed' || !closed) process.exitCode = 1;
