// Run against actual styled responsive components, with no backend or Docker.
// node phone-overview.browser.mjs --out /absolute/evidence [--original /path/sustained.mjs]
import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, mkdirSync, mkdtempSync, rmSync, existsSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import vm from 'node:vm';

const here = dirname(fileURLToPath(import.meta.url)), repo = resolve(here, '../../..');
const option = (key, fallback) => { const i = process.argv.indexOf(key); return i < 0 ? fallback : process.argv[i + 1]; };
const out = resolve(option('--out', 'artifacts/quality-swarm/phone-overview-controlled'));
const original = option('--original', ''), port = Number(option('--port', '15149'));
const hash = value => createHash('sha256').update(value).digest('hex');
const sourcePath = join(here, 'sustained.mjs'), source = readFileSync(sourcePath, 'utf8');
const start = '// BEGIN SUSTAINED PHONE OVERVIEW:', end = '// END SUSTAINED PHONE OVERVIEW';
assert.equal(source.split(start).length, 2); assert.equal(source.split(end).length, 2);
const corrected = source.slice(source.indexOf('\n', source.indexOf(start)) + 1, source.indexOf(end));
const files = ['scripts/acceptance/sustained-four-project/sustained.mjs', 'scripts/acceptance/sustained-four-project/phone-overview.browser.mjs', 'scripts/acceptance/sustained-four-project/phone-overview.fixture.tsx', 'desktop/src/components/AppShell.tsx', 'desktop/src/components/shell.css', 'desktop/src/components/topbar/TopBar.tsx', 'desktop/src/components/topbar/topbar.css', 'desktop/src/components/rail/Rail.tsx', 'desktop/src/components/rail/rail.css', 'desktop/src/components/definition/DefinitionWorkspace.tsx', 'desktop/src/components/common/DraftNavigationDialog.tsx', 'desktop/src/components/projects/ProjectOverviewBoard.tsx', 'desktop/src/components/projects/project-overview.css'];
const binding = () => Object.fromEntries(files.map(path => [path, { sha256: hash(readFileSync(join(repo, path))), bytes: readFileSync(join(repo, path)).length }]));
const report = { scope: 'Actual styled AppShell, Rail, TopBar, Home and Definition with controlled read-only providers. Linux Chromium; no backend/worker/provider/runtime or performance claim.', passed: false, sourceBefore: binding(), correctedBlockSHA256: hash(corrected), checks: [], errors: [], requests: [] };
assert(!existsSync(out), 'Use a fresh output directory to preserve earlier evidence.'); mkdirSync(out, { recursive: true });
const { createServer } = await import(pathToFileURL(join(repo, 'desktop/node_modules/vite/dist/node/index.js')).href);
const { chromium } = await import(pathToFileURL(join(repo, 'desktop/node_modules/playwright/index.mjs')).href);
// Chromium's singleton Unix socket must fit the native sockaddr path limit.
const profile = mkdtempSync('/tmp/c049-');
let server, browser;
const cleanup = { browserClosed: false, serverClosed: false, profileRemoved: false };
const check = (name, details = {}) => report.checks.push({ name, passed: true, ...details });
try {
  assert.notEqual(process.getuid(), 0);
  server = await createServer({ configFile: join(repo, 'desktop/vite.config.ts'), root: join(repo, 'desktop'), server: { host: '127.0.0.1', port, strictPort: true, watch: null, fs: { allow: [repo] } }, plugins: [{ name: 'controlled-phone-overview', enforce: 'pre', resolveId(id) {
    if (id.endsWith('/AuthProvider')) return '\0phone-auth'; if (id.endsWith('/TeamWorkspaceProvider')) return '\0phone-team'; if (id.endsWith('/SetupProvider')) return '\0phone-setup'; if (id.endsWith('/useProjectAccess')) return '\0phone-access';
  }, load(id) {
    if (id === '\0phone-auth') return 'export const useAuth=()=>({profile:{user:{id:"fixture",username:"fixture",displayName:"Fixture owner",role:"super-admin"},permissions:{}}});';
    if (id === '\0phone-team') return 'export const useTeamWorkspace=()=>null;export const useTeamProjectPermissions=()=>({loading:false,error:"",permissions:{read:true,edit:true,execute:true}});';
    if (id === '\0phone-setup') return 'export const useSetup=()=>({workspace:{mode:"local"},status:{configured:true,access:{user:{username:"fixture"}},config:{projectsDir:"/fixture"}}});';
    if (id === '\0phone-access') return 'export const useProjectAccess=()=>({canRead:true,canEdit:true,canExecute:true,loading:false,error:"",message:""});';
  }, configureServer(vite) { vite.middlewares.use('/phone-overview-fixture', async (_req, res) => { res.setHeader('Content-Type', 'text/html'); res.end(await vite.transformIndexHtml('/phone-overview-fixture', '<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled responsive navigation</title></head><body><div id="root"></div><script type="module" src="/@fs' + join(here, 'phone-overview.fixture.tsx') + '"></script></body></html>')); }); } }] });
  await server.listen(); const base = 'http://127.0.0.1:' + port;
  browser = await chromium.launch({ headless: true, executablePath: '/usr/bin/chromium', env: { PATH: process.env.PATH, TMPDIR: profile, XDG_CONFIG_HOME: join(profile, 'config'), XDG_CACHE_HOME: join(profile, 'cache') }, args: ['--no-proxy-server'] });
  report.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1080 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => { const request = route.request(), url = new URL(request.url()); if (url.origin !== base || !['GET', 'HEAD'].includes(request.method()) || url.pathname.startsWith('/v1/')) { report.requests.push({ method: request.method(), path: url.pathname, refused: true }); await route.abort(); } else await route.continue(); });
  const page = await context.newPage(); page.setDefaultTimeout(15000); page.on('pageerror', error => report.errors.push(error.message));
  const fresh = async () => { await page.goto(base + '/phone-overview-fixture'); await page.getByRole('button', { name: 'Develop', exact: true }).waitFor(); assert.equal(await page.evaluate(() => window.__phoneOverviewFixture.selected()), 'alpha'); };
  const geometry = async () => { assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'No horizontal document overflow'); };
  const state = async () => page.evaluate(() => ({ project: window.__phoneOverviewFixture.selected(), lens: window.__phoneOverviewFixture.lens(), railInert: document.querySelector('#project-navigation').inert, drawerOpen: document.querySelector('.shell').classList.contains('shell-projects-open') }));
  const execute = async (block, evidence) => { mkdirSync(evidence, { recursive: true }); const timed = async (promise, ms, label) => { let timer; try { return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(Error(label + ' deadline')), ms); })]); } finally { clearTimeout(timer); } }; return vm.runInNewContext('(async()=>{' + block + '\n})()', { page, assert, timed, join, evidence, projectStates: ['alpha','beta','gamma','delta'].map(name => ({ owner: { name } })) }); };
  if (!process.argv.includes('--followup-only')) {
  await fresh();
  await page.getByRole('button', { name: 'All projects', exact: true }).click();
  await page.getByRole('heading', { name: 'Across your projects', exact: true }).waitFor();
  await geometry(); assert.equal((await state()).project, null); check('desktop actual rail opens overview');
  await page.screenshot({ path: join(out, 'desktop-overview-1440.png') });
  if (original) {
    const before = readFileSync(resolve(original), 'utf8'), a = before.indexOf('await page.setViewportSize({width:390,height:844});'), b = before.indexOf("await page.screenshot({path:join(evidence,'four-project-phone.png'),fullPage:true});", a);
    assert(a >= 0 && b > a); const block = before.slice(a, b + "await page.screenshot({path:join(evidence,'four-project-phone.png'),fullPage:true});".length);
    report.original = { sourceSHA256: hash(before), blockSHA256: hash(block), expectedFailure: false };
    await fresh(); let failure; try { await execute(block, join(out, 'before')); } catch (error) { failure = String(error); }
    assert(failure?.includes('All projects') && failure.includes('Timeout'), 'Exact original tail must fail waiting for inaccessible All projects');
    report.original.expectedFailure = true; report.original.failure = failure; report.original.state = await state();
    assert.equal(report.original.state.project, 'alpha'); assert.equal(report.original.state.railInert, true);
    await page.screenshot({ path: join(out, 'before-phone-failure-390.png') }); check('original exact tail reproduces closed-rail timeout', { timeoutMs: 15000 });
  }
  await page.setViewportSize({ width: 1440, height: 1080 }); await fresh();
  await execute(corrected, join(out, 'after')); assert.equal((await state()).project, null); assert.equal((await state()).drawerOpen, false);
  assert.equal(await page.getByRole('button', { name: 'Close projects', exact: true }).count(), 0); check('exact corrected resize/drawer/home tail passes and closes drawer');
  for (const name of ['alpha', 'beta', 'gamma', 'delta']) {
    await page.getByRole('button', { name: 'Open project ' + name, exact: true }).click();
    await page.getByRole('button', { name: 'Define', exact: true }).click();
    const idea = page.getByRole('textbox', { name: 'The idea', exact: true }); await idea.waitFor();
    assert.equal(await idea.inputValue(), 'Unique saved purpose for ' + name + '.'); assert.equal((await state()).project, name);
    await geometry(); check('phone exact saved context ' + name);
    await page.getByRole('button', { name: 'Projects', exact: true }).focus(); await page.keyboard.press('Enter');
    await page.getByRole('button', { name: 'Close projects', exact: true }).waitFor();
    await page.getByRole('button', { name: 'All projects', exact: true }).focus(); await page.keyboard.press('Enter');
    await page.getByRole('heading', { name: 'Across your projects', exact: true }).waitFor();
    assert.equal((await state()).project, null); check('phone keyboard drawer/home ' + name);
  }
  await page.getByRole('button', { name: 'Open project alpha', exact: true }).click();
  const idea = page.getByRole('textbox', { name: 'The idea', exact: true }); await idea.waitFor(); await idea.fill('Private unsaved alpha purpose');
  await page.getByRole('button', { name: 'Projects', exact: true }).click(); await page.getByRole('button', { name: 'All projects', exact: true }).click();
  await page.getByRole('dialog', { name: 'Keep your changes?', exact: true }).waitFor(); assert.equal((await state()).project, 'alpha');
  await page.getByRole('button', { name: 'Keep editing', exact: true }).click(); assert.equal(await idea.inputValue(), 'Private unsaved alpha purpose');
  check('phone navigation retains draft guard and Keep editing'); await page.screenshot({ path: join(out, 'phone-kept-draft-390.png') });
  await page.getByRole('button', { name: 'Projects', exact: true }).click(); await page.getByRole('button', { name: 'All projects', exact: true }).click();
  await page.getByRole('button', { name: 'Keep drafts and continue', exact: true }).click(); await page.getByRole('heading', { name: 'Across your projects', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Open project beta', exact: true }).click(); assert.equal(await idea.inputValue(), 'Unique saved purpose for beta.');
  await page.getByRole('button', { name: 'Projects', exact: true }).click(); await page.getByRole('button', { name: 'All projects', exact: true }).click();
  await page.getByRole('button', { name: 'Open project alpha', exact: true }).click(); await page.getByRole('button', { name: 'Restore draft', exact: true }).click();
  assert.equal(await idea.inputValue(), 'Private unsaved alpha purpose'); check('phone retained draft remains alpha scoped across project switch/reopen');
  await page.screenshot({ path: join(out, 'phone-restored-draft-390.png') });
  // Missing catalogue membership still fails the real helper's exact four-name
  // postcondition. This navigation correction must not waive the fourth project.
  await page.goto(base + '/phone-overview-fixture?missing=delta');
  await page.getByRole('button', { name: 'Develop', exact: true }).waitFor();
  let missingFailure; try { await execute(corrected, join(out, 'missing-project')); } catch (error) { missingFailure = String(error); }
  assert(missingFailure?.includes('Open project delta') && missingFailure.includes('Timeout'));
  check('corrected tail refuses missing fourth project', { failure: missingFailure, timeoutMs: 15000 });
  await page.screenshot({ path: join(out, 'missing-project-refusal-390.png') });
  }
  // Separate narrow follow-up: viewport reversals and actual focus behavior.
  // --followup-only retains the prior exact-tail receipt and avoids repeating
  // its two completed 15-second negative waits when only these cases change.
  await page.setViewportSize({ width: 1440, height: 1080 }); await fresh();
  await page.evaluate(() => localStorage.clear()); await page.reload();
  await page.getByRole('button', { name: 'Define', exact: true }).click();
  const focusedIdea = page.getByRole('textbox', { name: 'The idea', exact: true }); await focusedIdea.waitFor();
  await page.setViewportSize({ width: 390, height: 844 });
  const projects = page.getByRole('button', { name: 'Projects', exact: true });
  await projects.focus(); await page.keyboard.press('Enter');
  const close = page.getByRole('button', { name: 'Close projects', exact: true }); await close.waitFor();
  await page.waitForFunction(() => document.activeElement?.getAttribute('aria-label') === 'Close projects');
  assert.equal(await projects.getAttribute('aria-expanded'), 'true'); check('phone opening focuses Close projects');
  await page.screenshot({ path: join(out, 'phone-drawer-focused-390.png') });
  await page.keyboard.press('Escape'); await page.waitForFunction(() => document.activeElement?.getAttribute('aria-label') === 'Projects');
  assert.equal(await projects.getAttribute('aria-expanded'), 'false'); assert.equal((await state()).project, 'alpha'); check('phone Escape closes drawer and restores trigger focus');
  await projects.click(); await close.waitFor(); await close.click();
  await page.waitForFunction(() => document.activeElement?.getAttribute('aria-label') === 'Projects'); check('explicit Close projects restores trigger focus');
  await page.setViewportSize({ width: 1440, height: 1080 });
  await page.getByRole('button', { name: 'All projects', exact: true }).waitFor();
  assert.equal((await state()).railInert, false); assert.equal((await state()).project, 'alpha');
  assert.equal(await focusedIdea.inputValue(), 'Unique saved purpose for alpha.'); await geometry();
  await page.getByRole('button', { name: 'All projects', exact: true }).click();
  await page.getByRole('heading', { name: 'Across your projects', exact: true }).waitFor(); check('390 to 1440 restores desktop rail and correct Home navigation');
  await page.screenshot({ path: join(out, 'returned-desktop-overview-1440.png') });
  await page.getByRole('button', { name: 'Open project gamma', exact: true }).click(); await focusedIdea.waitFor();
  assert.equal(await focusedIdea.inputValue(), 'Unique saved purpose for gamma.');
  await page.setViewportSize({ width: 390, height: 844 }); await geometry();
  assert.equal((await state()).project, 'gamma'); check('1440 to 390 preserves current project and saved purpose');
  await focusedIdea.fill('Guarded gamma follow-up draft'); await projects.click(); await close.waitFor();
  await page.getByRole('button', { name: 'All projects', exact: true }).click();
  const draftDialog = page.getByRole('dialog', { name: 'Keep your changes?', exact: true }); await draftDialog.waitFor();
  await page.waitForFunction(() => !!document.activeElement?.closest('[role="dialog"]'));
  check('compact draft guard receives modal focus');
  await page.getByRole('button', { name: 'Keep editing', exact: true }).click();
  await draftDialog.waitFor({ state: 'hidden' });
  await page.waitForFunction(() => { const node = document.activeElement; return node instanceof HTMLElement && node !== document.body && !node.closest('[inert]') && node.getBoundingClientRect().width > 0; });
  assert.equal((await state()).project, 'gamma'); assert.equal(await focusedIdea.inputValue(), 'Guarded gamma follow-up draft');
  check('compact Keep editing retains draft and returns focus to usable control', { focus: await page.evaluate(() => ({ tag: document.activeElement?.tagName, label: document.activeElement?.getAttribute('aria-label'), text: document.activeElement?.textContent })) });
  await page.screenshot({ path: join(out, 'phone-draft-focus-390.png') });
  assert.deepEqual(report.errors, []); assert.deepEqual(report.requests, []); report.passed = true;
} catch (error) { report.failure = String(error.stack || error); }
finally {
  try { if (browser) { await browser.close(); cleanup.browserClosed = true; } } catch (error) { cleanup.browserError = String(error); }
  try { if (server) { await server.close(); cleanup.serverClosed = true; } } catch (error) { cleanup.serverError = String(error); }
  try { rmSync(profile, { recursive: true, force: true }); cleanup.profileRemoved = true; } catch (error) { cleanup.profileError = String(error); }
  report.cleanup = cleanup; report.sourceAfter = binding(); report.sourceUnchanged = JSON.stringify(report.sourceBefore) === JSON.stringify(report.sourceAfter);
  report.passed &&= report.sourceUnchanged && cleanup.browserClosed && cleanup.serverClosed && cleanup.profileRemoved;
  writeFileSync(join(out, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ passed: report.passed, checks: report.checks.length, failure: report.failure, cleanup, sourceUnchanged: report.sourceUnchanged })); process.exitCode = report.passed ? 0 : 1;
}
