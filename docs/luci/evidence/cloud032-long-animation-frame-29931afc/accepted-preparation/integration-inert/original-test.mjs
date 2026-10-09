import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {dirname,join} from 'node:path';
const here=dirname(fileURLToPath(import.meta.url)),repo='/workspace/luci',template=join(repo,'docs/luci/evidence/cloud032-navigation-callback-5f4a966f/prepared-runtime/actual');
const read=path=>readFileSync(path,'utf8'),sha=path=>createHash('sha256').update(readFileSync(path)).digest('hex');
test('executed candidate changes only setup import/registration/hook and preserves original action modules',()=>{
  let expected=read(join(template,'sustained.mjs'));
  for(const [before,after] of [
    ["import {createReadinessLedger} from './readiness-ledger.mjs';","import {createReadinessLedger} from './readiness-ledger.mjs';\nimport {installLongFrameTiming} from './long-frame-observer.mjs';"],
    ["const driverNames=['checkout-binding.py'","const driverNames=['long-frame-observer.mjs','long-frame-bundles.json','checkout-binding.py'"],
    ['installRoutes:async context=>{await installBrowserTiming(context);',"installRoutes:async context=>{await installBrowserTiming(context);await installLongFrameTiming(context,{receipt,timingLedger,bundles:JSON.parse(readFileSync(join(here,'long-frame-bundles.json'),'utf8'))});"]
  ]){assert.equal(expected.split(before).length,2);expected=expected.replace(before,after);}
  assert.equal(read(join(here,'actual/sustained.mjs')),expected);
  for(const name of ['interaction-metrics.mjs','navigation-verifier.mjs','latency-diagnostics.mjs','autonomous-readiness.mjs','readiness-ledger.mjs'])assert.equal(sha(join(here,'actual',name)),sha(join(template,name)),name);
});
test('candidate captures the tested observer, bundle map and all exact local/repository dependencies',()=>{
  const binding=JSON.parse(read(join(here,'actual/external-source-binding.json')));
  assert.equal(binding.executionDirectory,join(here,'actual'));assert.equal(binding.prospectiveProduct,'29931afc5b31ae9b9bd7b8973a7e3fdb840609a3');assert.equal(binding.prospectiveHelper,binding.prospectiveProduct);
  for(const [name,hash] of Object.entries(binding.localSources))assert.equal(sha(join(here,'actual',name)),hash,name);
  for(const [name,hash] of Object.entries(binding.repositoryDependencies))assert.equal(sha(join(repo,name)),hash,name);
  assert.equal(sha(join(here,'long-frame-observer.mjs')),sha(join(here,'actual/long-frame-observer.mjs')));
  const command=JSON.parse(read(join(here,'launch-command.json'))).command;assert(command.includes('--window-granted'));assert.equal(command[command.index('--trace-after')+1],'yes');
});
test('generated driver parses without executing it and launcher parses without launching',()=>{
  execFileSync(process.execPath,['--check',join(here,'actual/sustained.mjs')],{timeout:5000,stdio:'pipe'});
  execFileSync('python3',['-c','import ast,sys; ast.parse(open(sys.argv[1]).read())',join(here,'launch.py')],{timeout:5000,stdio:'pipe'});
});
