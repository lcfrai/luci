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
  console.log('node scripts/acceptance/launch-recovery-ui/source-browser.mjs --report FRESH_DIR/result.json [--chromium /usr/bin/chromium] [--fixture-root SHORT_WRITABLE_DIR]\nLinux Chromium source-component recovery witness at 1440 and 390 CSS pixels. Availability and mutation callbacks are explicit fixtures. No backend mutation, Windows probe, Docker, agent, model or cloud execution.');
  process.exit(0);
}
assert.equal(process.platform, 'linux', 'This helper records Linux Chromium, not native Windows acceptance.');
assert.notEqual(process.getuid(), 0, 'Run with an unprivileged account and the browser sandbox enabled.');
assert(option('--report'), '--report is required');
const report = resolve(option('--report')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Choose a fresh evidence directory.');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixtureParent = realpathSync(resolve(option('--fixture-root', tmpdir())));
const profile = mkdtempSync(join(fixtureParent, 'luci-recovery-ui-'));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const inputs = [...new Set(execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', 'desktop'], {cwd:repo,encoding:'utf8'}).trim().split('\n'))].sort();
const fingerprint = () => Object.fromEntries(inputs.map(path => [path, sha(readFileSync(join(repo, path)))]));
const receipt = {
  passed: false, startedAt: new Date().toISOString(),
  scope: 'Actual source ProjectSwarm and recovery dialog/controls/CSS in Linux Chromium with controlled API callbacks and the queue panel omitted.',
  limitations: ['No installed-package, native Windows, real management API, physical storage, Docker, agent, model or cloud execution evidence.', 'The fixture confirms rendering and interaction. Actual launch fencing, authority and backend HTTP behavior are separate checks.', 'Automated axe checks supplement keyboard and geometry assertions; they are not an accessibility certification.'],
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
      name: 'explicit-recovery-fixture-identity', enforce: 'pre',
      resolveId(id, importer) {
        if (importer?.endsWith('/ProjectSwarm.tsx') && id === './SwarmRunQueue') return '\0recovery-fixture-queue';
      },
      load(id) {
        if (id === '\0recovery-fixture-queue') return 'export const SwarmRunQueue=()=>null;';
      },
      configureServer(vite) {
        vite.middlewares.use('/recovery-fixture', async (_request, response) => {
          response.setHeader('Content-Type', 'text/html');
          const html = '<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled launch recovery UI fixture</title></head><body><div id="root"></div><script type="module" src="/@fs' + join(here, 'fixture.tsx') + '"></script></body></html>';
          response.end(await vite.transformIndexHtml('/recovery-fixture', html));
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
  const stopLabel = 'Stop uncertain launch and retain output';
  const calls = () => page.evaluate(() => window.__recoveryFixture.calls);
  for (const width of [1440, 390]) {
    await page.setViewportSize({width,height:width===390?844:1080});
    await page.goto(base+'/recovery-fixture?scenario=supported');
    const opener=page.getByRole('button',{name:stopLabel,exact:true});
    await opener.click();
    let dialog=page.getByRole('dialog',{name:stopLabel,exact:true});
    await dialog.getByRole('checkbox').waitFor();
    assert(await dialog.evaluate(element=>element.contains(document.activeElement)));
    for(let step=0;step<6;step++) {
      await page.keyboard.press('Tab');
      assert(await dialog.evaluate(element=>element.contains(document.activeElement)),'Focus must stay inside modal');
    }
    await page.keyboard.press('Escape');
    await dialog.waitFor({state:'hidden'});
    assert(await opener.evaluate(element=>element===document.activeElement),'Closing must restore opener focus');
    await opener.click();
    dialog=page.getByRole('dialog',{name:stopLabel,exact:true});
    await dialog.getByRole('checkbox').waitFor();
    assert(await dialog.getByRole('button',{name:stopLabel,exact:true}).isDisabled());
    assert.equal((await calls()).filter(call=>call.method==='POST stop').length,0);
    const geometry=await dialog.evaluate(element=>({clientWidth:element.clientWidth,scrollWidth:element.scrollWidth,left:element.getBoundingClientRect().left,right:element.getBoundingClientRect().right,top:element.getBoundingClientRect().top,bottom:element.getBoundingClientRect().bottom,viewport:innerWidth,height:innerHeight}));
    receipt.geometry.push({width,...geometry});
    assert(geometry.scrollWidth<=geometry.clientWidth+1 && geometry.left>=0 && geometry.right<=width+1 && geometry.top>=0 && geometry.bottom<=geometry.height+1);
    await page.addScriptTag({path:join(repo,'desktop/node_modules/axe-core/axe.min.js')});
    const violations=await page.evaluate(async()=> (await window.axe.run(document.querySelector('[role="dialog"]'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(value=>({id:value.id,impact:value.impact,nodes:value.nodes.map(node=>node.target)})));
    receipt.checks.push({width,axeViolations:violations});assert.deepEqual(violations,[]);
    await page.screenshot({path:join(evidence,`recovery-review-${width}.png`)});
    await dialog.getByRole('checkbox').check();
    const submit=dialog.getByRole('button',{name:stopLabel,exact:true});
    await submit.scrollIntoViewIfNeeded();
    await page.screenshot({path:join(evidence,`recovery-consent-${width}.png`)});
    await submit.click();
    await page.getByText('Attempt retired · output retained',{exact:true}).waitFor();
    const recorded=await calls();
    assert.equal(recorded.filter(call=>call.method==='POST stop').length,1);
    assert.deepEqual(recorded.find(call=>call.method==='POST stop'),{method:'POST stop',projectId:'p-1',runId:'run-1',taskId:'a',expectedRevision:1,operationId:'op-a',childId:'child-a',stopUncertainLaunch:true});
    receipt.checks.push({width,focusTrapAndRestoration:true,explicitConsentOnly:true,fixtureCalls:recorded});
  }
  await page.goto(base+'/recovery-fixture?scenario=legacy');
  await page.getByRole('button',{name:stopLabel,exact:true}).click();
  let dialog=page.getByRole('dialog',{name:stopLabel,exact:true});
  await dialog.getByText('This older launch has no recovery protocol. Its reservation stays held for host inspection.',{exact:true}).waitFor();
  assert.equal(await dialog.getByRole('checkbox').count(),0);
  assert(await dialog.getByRole('button',{name:stopLabel,exact:true}).isDisabled());
  await page.screenshot({path:join(evidence,'recovery-legacy-refusal-390.png')});
  receipt.checks.push({scenario:'legacy refusal',fixtureCalls:await calls(),noMutation:(await calls()).every(call=>call.method!=='POST stop')});
  assert((await calls()).every(call=>call.method!=='POST stop'));
  await page.goto(base+'/recovery-fixture?scenario=unknown-response');
  await page.getByRole('button',{name:stopLabel,exact:true}).click();
  dialog=page.getByRole('dialog',{name:stopLabel,exact:true});
  await dialog.getByRole('checkbox').check();
  await dialog.getByRole('button',{name:stopLabel,exact:true}).click();
  await page.getByText('Launch recovery pending · capacity held',{exact:true}).waitFor();
  assert.equal(await page.getByRole('button',{name:'Retry in a new workspace',exact:true}).count(),0);
  assert.equal((await calls()).filter(call=>call.method==='POST stop').length,1);
  assert.equal((await calls()).filter(call=>call.method==='GET readback').length,1);
  await page.getByRole('button',{name:'Continue launch recovery',exact:true}).scrollIntoViewIfNeeded();
  await page.screenshot({path:join(evidence,'recovery-pending-390.png')});
  receipt.checks.push({scenario:'lost fixture response',oneReadbackNoMutationRetry:true,fixtureCalls:await calls()});
  await page.goto(base+'/recovery-fixture?scenario=delayed');
  await page.getByRole('button',{name:stopLabel,exact:true}).click();
  await page.getByText('Checking whether this exact attempt supports recovery…',{exact:true}).waitFor();
  await page.evaluate(()=>window.__recoveryFixture.switchProject());
  await page.getByRole('dialog').waitFor({state:'hidden'});
  await page.evaluate(()=>window.__recoveryFixture.completeRead());
  assert.equal(await page.getByRole('dialog').count(),0);
  assert((await calls()).every(call=>call.method!=='POST stop'));
  receipt.checks.push({scenario:'controlled project switch with delayed availability',oldDialogNotRevived:true,fixtureCalls:await calls()});
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
