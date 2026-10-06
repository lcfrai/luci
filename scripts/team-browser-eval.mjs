import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const options = {};
for (let index = 2; index < process.argv.length; index++) {
 const name = process.argv[index];
 if (name === '--help') { console.log('Usage: node scripts/team-browser-eval.mjs [--browser /path/to/chromium] [--report /path/to/results.json] [--https]\nRequires the repository Go toolchain, desktop npm dependencies and a Playwright-compatible Chromium. Creates private fixtures and no external services.'); process.exit(0); }
 if(name==='--https'){options.https=true;continue;}
 if (!['--browser','--report'].includes(name) || !process.argv[index+1]) throw new Error('Unknown or incomplete argument: '+name);
 options[name.slice(2)] = process.argv[++index];
}
const report = options.report ? path.resolve(options.report) : path.join(root,'artifacts','uat',`team-browser-${Date.now()}-${process.pid}`,'results.json');
fs.mkdirSync(path.dirname(report),{recursive:true,mode:0o700});
const require = createRequire(path.join(root, 'desktop/package.json'));
const {chromium} = require('playwright');
const fixtureDir = fs.mkdtempSync(path.join(path.dirname(report),'.private-fixture-'));
const fixture = path.join(fixtureDir, 'fixture.json');
const go = spawn('go', ['test', './internal/management', '-run', '^TestTeamBrowserChromiumFixture$', '-count=1', '-timeout=60s'], {cwd:path.join(root,'control-plane'),env:{...process.env,TMPDIR:fixtureDir,LUCI_TEAM_BROWSER_FIXTURE_FILE:fixture,...(options.https?{LUCI_TEAM_BROWSER_PUBLIC_HTTPS:'1'}:{})},stdio:['ignore','pipe','pipe']});
let goOutput = ''; go.stdout.on('data',data=>goOutput+=data);go.stderr.on('data',data=>goOutput+=data);
const ended = new Promise(resolve=>go.on('close',code=>resolve(code)));
const executablePath=options.browser || process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE || (fs.existsSync('/usr/bin/chromium')?'/usr/bin/chromium':undefined);
const result={passed:false,checks:[],browser:executablePath || 'Playwright Chromium',scope:`Actual Chromium through production management and preview gateway handlers${options.https?' behind a local TLS reverse proxy using fixture-only certificate pin and DNS mapping':''}; deterministic application/data upstream fixtures`,externalModelCalls:0,cloudResourcesCreated:0};
const checked=name=>result.checks.push({name,passed:true});
let browser;
try {
 const deadline=Date.now()+30000;
 while(!fs.existsSync(fixture)) { if(go.exitCode!==null||Date.now()>deadline)throw new Error('Browser fixture did not start: '+goOutput);await new Promise(r=>setTimeout(r,100)); }
 const config=JSON.parse(fs.readFileSync(fixture,'utf8'));
 browser=await chromium.launch({...(executablePath?{executablePath}:{}),headless:true,args:[...(process.platform==='linux'?['--no-sandbox']:[]),...(config.tlsAddress?['--no-proxy-server',`--host-resolver-rules=MAP *.luci.test ${config.tlsAddress}, MAP *.projects.test ${config.tlsAddress}`,`--ignore-certificate-errors-spki-list=${config.tlsSPKI}`]:[])]});
 const context=await browser.newContext();const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(config.page);const frame=page.frameLocator('iframe').first();
 await frame.getByRole('heading',{name:'Authenticated team application'}).waitFor({timeout:10000});checked('One-time handoff authenticates sandboxed team app iframe');
 await frame.locator('body[data-asset="loaded"]').waitFor();checked('App subresource requests retain isolated browser authentication');
 await frame.locator('body[data-data="authorized"]').waitFor();checked('Automatic same-project data bootstrap supports credentialed CORS fetch and preflight');
 await frame.locator('body[data-realtime="authorized realtime"]').waitFor();checked('Automatic same-project data bootstrap supports realtime WebSocket');
 const cookies=await context.cookies();if(cookies.length!==2||cookies.some(c=>!c.httpOnly||!c.secure||c.sameSite!=='None'||!c.partitionKey||c.domain.startsWith('.')))throw new Error('Browser cookie isolation did not match contract');checked('Both origins use host-only Secure HttpOnly partitioned cookies');
 await page.evaluate(origin=>Promise.all(['image','frame'].map(kind=>new Promise((resolve,reject)=>{const element=kind==='image'?new Image():document.createElement('iframe');element.onload=element.onerror=()=>resolve();setTimeout(()=>reject(new Error('Foreign resource did not settle')),10000);element.src=origin+'/cross-origin-'+kind;document.body.append(element);}))),config.origin);
 if((await page.evaluate(async()=> (await fetch('/foreign-count')).json())).count!==0)throw new Error('Cross-origin GET reached the authenticated application');checked('Foreign image and iframe GETs never reach the app inside the authenticated browser partition');
 const separate=await context.newPage();await separate.goto(config.origin);if(!(await separate.textContent('body')).includes('Open this team preview'))throw new Error('Copied application origin retained access');checked('Copied app URL outside its authorized browser partition is denied');
 await separate.goto(config.dataOrigin+'/rest/v1/notes');if(!(await separate.textContent('body')).includes('Open this team preview'))throw new Error('Copied data origin retained access');checked('Copied data URL outside its authorized browser partition is denied');
 await page.evaluate(async()=>fetch('/revoke')); await frame.locator('body[data-closed="true"]').waitFor({timeout:10000});checked('Permission removal closes an already active realtime WebSocket');
 const status=await page.frames()[1].evaluate(async()=> (await fetch('/asset')).status);if(status!==403)throw new Error('Revoked app cookie remained usable');checked('Permission removal rejects the previously authorized browser cookie');
 if(errors.length)throw new Error('Unexpected browser script error');checked('No browser script errors');result.passed=true;
} catch(error) {result.failure=error.message;process.exitCode=1;}
finally {if(browser)await browser.close();if(fs.existsSync(fixture))fs.unlinkSync(fixture);const code=await ended;if(code!==0){result.passed=false;result.fixtureFailure=goOutput;process.exitCode=1;}fs.rmSync(fixtureDir,{recursive:true,force:true});fs.writeFileSync(report,JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify({...result,report},null,2));}
