#!/usr/bin/env node
// Real Settings source and CSS; storage/readiness callbacks are controlled fixtures.
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
  console.log('node scripts/acceptance/storage-refresh-ui/source-browser.mjs --report FRESH_DIR/result.json [--chromium /usr/bin/chromium] [--fixture-root SHORT_WRITABLE_DIR]\nLinux Chromium Settings witness at 1440 and 390 CSS pixels. Storage/readiness callbacks are explicit fixtures. No backend mutation, Windows probe, Docker, agent, model or cloud execution.');
  process.exit(0);
}
assert.equal(process.platform, 'linux', 'This helper records Linux Chromium, not native Windows acceptance.');
assert.notEqual(process.getuid(), 0, 'Run with an unprivileged account and the browser sandbox enabled.');
assert(option('--report'), '--report is required');
const report = resolve(option('--report')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Choose a fresh evidence directory.');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixtureParent = realpathSync(resolve(option('--fixture-root', tmpdir())));
const profile = mkdtempSync(join(fixtureParent, 'luci-settings-ui-'));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const inputs = [...new Set(execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', 'desktop'], {cwd:repo,encoding:'utf8'}).trim().split('\n'))].filter(name => !name.startsWith('desktop/src-tauri/')).sort();
const fingerprint = () => Object.fromEntries(inputs.map(path => [path, sha(readFileSync(join(repo, path)))]));
const receipt = {
  passed: false, startedAt: new Date().toISOString(),
  scope: 'Actual source SetupPanel, storage confirmation and runtime preparation in Linux Chromium with controlled HTTP callbacks; dependency installer panel omitted.',
  limitations: ['No installed-package, native Windows, real management API, physical storage, Docker, agent, model or cloud execution evidence.', 'The fixture confirms rendering and interaction. Native process-start traces, physical storage and real management HTTP behavior are separate checks.', 'Automated axe checks supplement keyboard and geometry assertions; they are not an accessibility certification.'],
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
      name: 'explicit-settings-fixture-identity', enforce: 'pre',
      resolveId(id, importer) {
        if (importer?.endsWith('/SetupPanel.tsx') && id === './DependencySetup') return '\0settings-fixture-dependencies';
        if (['/DockerStorageConfirmation.tsx','/SetupPanel.tsx','/LocalTools.tsx'].some(name=>importer?.endsWith(name)) && id.endsWith('/AuthProvider')) return '\0settings-fixture-auth';
        if (importer?.endsWith('/DockerStorageConfirmation.tsx') && id.endsWith('/TeamWorkspaceProvider')) return '\0settings-fixture-team';
      },
      load(id) {
        if (id === '\0settings-fixture-dependencies') return 'export const DependencySetup=()=>null;';
        if (id === '\0settings-fixture-auth') return 'export const useAuth=()=>({profile:{user:{id:"controlled-owner",role:"super-admin"},permissions:{manageBacking:true},policy:{allowCloudProvisioning:false}}});';
        if (id === '\0settings-fixture-team') return 'export const useTeamWorkspace=()=>null;';
      },
      configureServer(vite) {
        vite.middlewares.use('/settings-fixture', async (_request, response) => {
          response.setHeader('Content-Type', 'text/html');
          const html = '<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled storage refresh Settings fixture</title></head><body><div id="root"></div><script type="module" src="/@fs' + join(here, 'fixture.tsx') + '"></script></body></html>';
          response.end(await vite.transformIndexHtml('/settings-fixture', html));
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
  const readChecks = () => page.evaluate(()=>window.__settingsFixture.calls.filter(call=>call.path==='/v1/setup/check').length);
  for(const width of [1440,390]) {
    await page.setViewportSize({width,height:width===390?844:1080});
    await page.goto(base+'/settings-fixture?scenario=settings');
    await page.getByText('Controlled cached storage observation.',{exact:false}).waitFor();
    await page.getByRole('button',{name:'Refresh Docker storage status',exact:true}).click();
    await page.getByText('Refreshing Docker storage…',{exact:true}).waitFor({state:'hidden'});
    assert.equal(await readChecks(),0);
    for(const name of ['Close settings','Back to project','Appearance','New project']) assert(await page.getByRole('button',{name,exact:true}).isEnabled(),name+' must remain usable');
    await page.evaluate(()=>window.__settingsFixture.deferStorage());
    await page.getByRole('button',{name:'Refresh Docker storage status',exact:true}).click();
    await page.getByText('Refreshing Docker storage…',{exact:true}).waitFor();
    assert(await page.getByRole('button',{name:'Refresh Docker storage status',exact:true}).isDisabled());
    assert(await page.getByRole('button',{name:'Close settings',exact:true}).isEnabled());
    assert(await page.getByRole('button',{name:'Back to project',exact:true}).isEnabled());
    assert.equal(await page.locator('.setup-content').getAttribute('aria-busy'),'false');
    await page.getByRole('heading',{name:'Docker storage capacity',exact:true}).scrollIntoViewIfNeeded();
    await page.screenshot({path:join(evidence,`settings-storage-pending-${width}.png`)});
    const dialog=page.getByRole('dialog',{name:'Settings',exact:true});
    const geometry=await dialog.evaluate(element=>({clientWidth:element.clientWidth,scrollWidth:element.scrollWidth,left:element.getBoundingClientRect().left,right:element.getBoundingClientRect().right,viewport:innerWidth}));
    receipt.geometry.push({width,...geometry});assert(geometry.scrollWidth<=geometry.clientWidth+1 && geometry.left>=0 && geometry.right<=width+1);
    await page.getByRole('button',{name:'Appearance',exact:true}).click();
    await page.getByRole('heading',{name:'Make room for your way of working.',exact:true}).waitFor();
    await page.evaluate(()=>window.__settingsFixture.completeStorage());
    assert.equal(await readChecks(),0);
    assert.equal(await page.evaluate(()=>window.__settingsFixture.statuses.length),0);
    await page.addScriptTag({path:join(repo,'desktop/node_modules/axe-core/axe.min.js')});
    const violations=await page.evaluate(async()=> (await window.axe.run(document.querySelector('[role="dialog"]'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(value=>({id:value.id,impact:value.impact,nodes:value.nodes.map(node=>node.target)})));
    assert.deepEqual(violations,[]);
    await page.getByRole('button',{name:'Back to project',exact:true}).click();
    assert.equal(await page.evaluate(()=>window.__settingsFixture.closed),1);
    receipt.checks.push({width,storageRefreshNoReadinessProbe:true,navigationUsableWhileStorageReadPending:true,lateResultIgnoredAfterLeavingStorage:true,axeViolations:violations,calls:await page.evaluate(()=>window.__settingsFixture.calls)});
  }
  await page.goto(base+'/settings-fixture?scenario=confirmation');
  await page.getByLabel('Active Docker VHD file',{exact:true}).fill(String.raw`D:\Docker\disk.vhdx`);
  await page.getByRole('checkbox',{name:/I checked that this is Docker Desktop/}).check();
  await page.getByRole('button',{name:'Review storage location',exact:true}).click();
  await page.getByRole('button',{name:'Confirm Docker storage location',exact:true}).click();
  await page.getByText('Controlled confirmation saved; waiting for measurement.',{exact:false}).waitFor();
  assert.equal(await readChecks(),0);
  assert.equal(await page.evaluate(()=>window.__settingsFixture.statuses.at(-1).checks.find(check=>check.id==='docker-backing-storage').status),'unknown');
  assert(await page.getByRole('button',{name:'Close settings',exact:true}).isEnabled());
  await page.evaluate(()=>window.__settingsFixture.markConfirmed());
  await page.getByRole('button',{name:'Refresh Docker storage status',exact:true}).click();
  await page.getByText('Controlled location measured; readiness remains separate.',{exact:false}).waitFor();
  assert.equal(await readChecks(),0);
  assert.equal(await page.evaluate(()=>window.__settingsFixture.statuses.at(-1).checks.find(check=>check.id==='docker-backing-storage').status),'unknown');
  await page.getByRole('button',{name:'Check readiness',exact:true}).click();
  await page.getByText('Readiness check finished.',{exact:true}).waitFor();
  assert.equal(await readChecks(),1);
  assert.equal(await page.evaluate(()=>window.__settingsFixture.statuses.at(-1).checks.find(check=>check.id==='docker-backing-storage').status),'ready');
  receipt.checks.push({scenario:'explicit confirmation and separate readiness',noReadinessProbeUntilExplicitChoice:true,calls:await page.evaluate(()=>window.__settingsFixture.calls)});
  await page.goto(base+'/settings-fixture?scenario=portal');
  await page.getByRole('button',{name:'On this computer',exact:true}).click();
  await page.getByText('Checking dependencies and preparing your workspace…',{exact:true}).waitFor({state:'hidden'});
  await page.getByText('Controlled cached storage observation.',{exact:false}).waitFor();
  assert.equal(await readChecks(),1);
  await page.getByRole('button',{name:'Refresh Docker storage status',exact:true}).click();
  await page.getByText('Refreshing Docker storage…',{exact:true}).waitFor({state:'hidden'});
  assert.equal(await readChecks(),1);
  assert(await page.getByRole('button',{name:'Continue to projects',exact:true}).isEnabled());
  assert(await page.getByRole('button',{name:'Choose another setup option',exact:true}).isEnabled());
  await page.getByRole('button',{name:'Continue to projects',exact:true}).click();
  assert.equal(await page.evaluate(()=>window.__settingsFixture.closed),1);
  receipt.checks.push({scenario:'portal cached refresh',onlyInitialPreparationCheck:true,continuationAndSetupChoiceUsable:true,calls:await page.evaluate(()=>window.__settingsFixture.calls)});
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
