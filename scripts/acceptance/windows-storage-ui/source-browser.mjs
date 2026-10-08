#!/usr/bin/env node
// Real source components and CSS; identity and storage observations are controlled fixtures.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url)), repo = resolve(here, '../../..');
const args = process.argv.slice(2);
const option = (name, fallback) => {
  const index = args.indexOf(name);
  if (index < 0) return fallback;
  assert(args[index + 1] && !args[index + 1].startsWith('--'), 'Missing ' + name);
  return args[index + 1];
};
if (args.includes('--help')) {
  console.log('node scripts/acceptance/windows-storage-ui/source-browser.mjs --report FRESH_DIR/result.json [--chromium /usr/bin/chromium] [--fixture-root SHORT_WRITABLE_DIR]\nLinux Chromium source-component witness at 1440 and 390 CSS pixels. Identity, measurements and confirmation callbacks are explicit fixtures. No backend mutation, Windows probe, Docker, agent, model or cloud execution.');
  process.exit(0);
}
assert.equal(process.platform, 'linux', 'This helper records Linux Chromium, not native Windows acceptance.');
assert.notEqual(process.getuid(), 0, 'Run with an unprivileged account and the browser sandbox enabled.');
assert(option('--report'), '--report is required');
const report = resolve(option('--report')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Choose a fresh evidence directory.');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixtureParent = realpathSync(resolve(option('--fixture-root', tmpdir())));
const profile = mkdtempSync(join(fixtureParent, 'luci-storage-ui-'));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const inputs = [
  'desktop/src/api/executionStorage.ts', 'desktop/src/components/setup/DockerStorageConfirmation.tsx',
  'desktop/src/components/setup/setup.css', 'desktop/src/components/ui/button.tsx', 'desktop/src/components/ui/input.tsx',
  'desktop/src/styles/app.css', 'desktop/src/styles/tokens.css', 'desktop/src/styles/fluid.css', 'desktop/src/styles/shadcn.css',
  'desktop/vite.config.ts', 'desktop/package.json', 'desktop/package-lock.json',
];
const fingerprint = () => Object.fromEntries(inputs.map(path => [path, sha(readFileSync(join(repo, path)))]));
const receipt = {
  passed: false, startedAt: new Date().toISOString(),
  scope: 'Actual source DockerStorageConfirmation and shared controls/CSS in Linux Chromium with explicitly controlled identity and storage DTO callbacks.',
  limitations: ['No installed-package, native Windows, real management API, physical storage, Docker, agent, model or cloud execution evidence.', 'The fixture confirms rendering and interaction. Native storage validation and HTTP handler authorization are separate checks.', 'Automated axe checks supplement keyboard and geometry assertions; they are not an accessibility certification.'],
  sourceCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(),
  sourceDirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim(),
  sourceBefore: fingerprint(), helperSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  fixtureSHA256: sha(readFileSync(join(here, 'fixture.tsx'))), checks: [], geometry: [], errors: [], cleanup: [],
};
for (const name of ['source-browser.mjs', 'fixture.tsx']) writeFileSync(join(evidence, name), readFileSync(join(here, name)), { mode: 0o600, flag: 'wx' });
let server, browser;
try {
  const { createServer } = await import(pathToFileURL(join(repo, 'desktop/node_modules/vite/dist/node/index.js')).href);
  const { chromium } = await import(pathToFileURL(join(repo, 'desktop/node_modules/playwright/index.mjs')).href);
  server = await createServer({
    configFile: join(repo, 'desktop/vite.config.ts'), root: join(repo, 'desktop'),
    server: { host: '127.0.0.1', port: 0, strictPort: false, fs: { allow: [repo] }, watch: null },
    plugins: [{
      name: 'explicit-storage-fixture-identity', enforce: 'pre',
      resolveId(id, importer) {
        if (!importer?.endsWith('/DockerStorageConfirmation.tsx')) return;
        if (id.endsWith('/AuthProvider')) return '\0storage-fixture-auth';
        if (id.endsWith('/TeamWorkspaceProvider')) return '\0storage-fixture-team';
      },
      load(id) {
        if (id === '\0storage-fixture-auth') return 'export const useAuth=()=>({profile:{user:{id:"controlled-owner",role:"super-admin"}}});';
        if (id === '\0storage-fixture-team') return 'export const useTeamWorkspace=()=>null;';
      },
      configureServer(vite) {
        vite.middlewares.use('/storage-fixture', async (_request, response) => {
          response.setHeader('Content-Type', 'text/html');
          const html = '<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled Windows storage settings fixture</title></head><body><div id="root"></div><script type="module" src="/@fs' + join(here, 'fixture.tsx') + '"></script></body></html>';
          response.end(await vite.transformIndexHtml('/storage-fixture', html));
        });
      },
    }],
  });
  await server.listen();
  const base = 'http://127.0.0.1:' + server.httpServer.address().port;
  browser = await chromium.launch({ headless: true, executablePath: option('--chromium', '/usr/bin/chromium'),
    env: { PATH: process.env.PATH, HOME: profile, TMPDIR: profile }, args: ['--no-proxy-server'] });
  receipt.browserVersion = browser.version();
  const context = await browser.newContext({ serviceWorkers: 'block' });
  await context.route('**/*', route => {
    const url = new URL(route.request().url());
    if (url.origin !== base) { receipt.errors.push('External request refused: ' + url.origin); return route.abort(); }
    return route.continue();
  });
  const page = await context.newPage();
  page.setDefaultTimeout(12_000);
  page.on('pageerror', error => receipt.errors.push(error.message));
  const path = String.raw`D:\Local application data\Docker Desktop\wsl\disk\docker_data.vhdx`;
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1080 });
    await page.goto(base + '/storage-fixture');
    await page.getByText('12.0 GiB', { exact: true }).waitFor();
    await page.getByText('220.0 GiB', { exact: true }).waitFor();
    await page.screenshot({ path: join(evidence, `storage-initial-${width}.png`) });
    await page.getByLabel('Active Docker VHD file', { exact: true }).fill(path);
    await page.getByRole('checkbox', { name: /I checked that this is Docker Desktop/ }).check();
    await page.getByRole('button', { name: 'Review storage location', exact: true }).click();
    const title = page.getByRole('heading', { name: 'Confirm this storage layout', exact: true });
    await title.waitFor(); assert(await title.evaluate(element => element === document.activeElement));
    await page.keyboard.press('Tab');
    assert(await page.getByRole('button', { name: 'Change location', exact: true }).evaluate(element => element === document.activeElement));
    await page.keyboard.press('Tab');
    assert(await page.getByRole('button', { name: 'Confirm Docker storage location', exact: true }).evaluate(element => element === document.activeElement));
    assert.equal((await page.evaluate(() => window.__storageFixture.calls)).filter(call => call.method === 'POST').length, 0);
    const group = page.getByRole('group', { name: 'Review Docker storage confirmation' });
    await group.scrollIntoViewIfNeeded();
    await page.screenshot({ path: join(evidence, `storage-review-${width}.png`) });
    const geometry = await group.evaluate(element => ({ clientWidth: element.clientWidth, scrollWidth: element.scrollWidth,
      left: element.getBoundingClientRect().left, right: element.getBoundingClientRect().right, viewport: innerWidth }));
    receipt.geometry.push({ width, ...geometry });
    assert(geometry.scrollWidth <= geometry.clientWidth + 1 && geometry.left >= 0 && geometry.right <= width + 1);
    await page.addScriptTag({ path: join(repo, 'desktop/node_modules/axe-core/axe.min.js') });
    const axe = await page.evaluate(async () => await window.axe.run(document.querySelector('main'), { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa'] } }));
    const violations = axe.violations.map(value => ({ id: value.id, impact: value.impact, nodes: value.nodes.map(node => node.target) }));
    receipt.checks.push({ width, axeViolations: violations }); assert.deepEqual(violations, []);
    await page.getByRole('button', { name: 'Confirm Docker storage location', exact: true }).click();
    await page.getByText('Waiting for a fresh background measurement. Confirmation alone does not establish enough free space.', { exact: true }).waitFor();
    await page.getByText('Current local layout confirmed and physical space measured.', { exact: false }).waitFor();
    const calls = await page.evaluate(() => window.__storageFixture.calls);
    assert.equal(calls.filter(call => call.method === 'POST').length, 1);
    assert.deepEqual(calls.find(call => call.method === 'POST'), { method: 'POST', expectedRevision: 3, vhdPath: path, confirmActiveAndPendingLocal: true });
    await page.getByRole('button', { name: 'Review or change storage location' }).waitFor();
    await page.getByRole('heading', { name: 'Docker storage capacity', exact: true }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: join(evidence, `storage-confirmed-${width}.png`) });
    receipt.checks.push({ width, reviewFocusAndTabOrder: true, noWriteBeforeConfirmation: true, oneExplicitFixturePostThenReadOnlyPolling: true, fixtureCalls: calls });
  }
  assert.deepEqual(receipt.errors, []);
  receipt.sourceAfter = fingerprint(); receipt.sourceUnchanged = JSON.stringify(receipt.sourceBefore) === JSON.stringify(receipt.sourceAfter);
  assert(receipt.sourceUnchanged, 'Source changed during this browser check.');
  receipt.passed = true;
} catch (error) {
  receipt.error = String(error.stack || error).replaceAll(profile, '[private-fixture]'); process.exitCode = 1;
} finally {
  await browser?.close(); await server?.close();
  assert.equal(dirname(realpathSync(profile)), fixtureParent); rmSync(profile, { recursive: true });
  receipt.cleanup.push('Owned browser and Vite server closed; private browser files removed.');
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(report, JSON.stringify(receipt, null, 2) + '\n', { mode: 0o600, flag: 'wx' });
  console.log((receipt.passed ? 'PASS ' : 'FAIL ') + report);
}
