import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {captureSourceBinding,helperPaths} from './source-binding.mjs';

const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(here,'../../..');
const product='a'.repeat(40),helper='b'.repeat(40);
function fixture(overrides={}){
 return {repo,actualDirectory:here,provenance:{sourceCommit:product,sourceDirty:false},expectedProduct:product,expectedHelper:helper,
  readGit:args=>{if(args[0]==='rev-parse')return helper+'\n';if(args[0]==='status')return '';assert.equal(args[0],'show');assert(args[1].startsWith(helper+':'));return readFileSync(join(repo,args[1].slice(helper.length+1)));},...overrides};
}
test('Selected product and separate clean helper commit bind every executed shared file',()=>{
 const binding=captureSourceBinding(fixture());assert.equal(binding.checkout.productSource,product);assert.equal(binding.checkout.commit,helper);assert.equal(binding.checkout.separateHelperRevision,true);
 assert.deepEqual(binding.files.map(row=>row.path),helperPaths);assert(binding.files.every(row=>row.bytes>0&&/^[a-f0-9]{64}$/.test(row.sha256)));
 assert(helperPaths.includes('scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'));assert(!helperPaths.some(path=>path.includes('/worker-recovery/runtime/')));
});
test('No helper omission, dirty checkout, product substitution or staged outside script is accepted',()=>{
 assert.throws(()=>captureSourceBinding(fixture({expectedHelper:undefined})),/Explicit --expected-helper-source/);
 assert.throws(()=>captureSourceBinding(fixture({expectedProduct:'c'.repeat(40)})),/Packaged product source must match/);
 const normal=fixture().readGit;assert.throws(()=>captureSourceBinding(fixture({readGit:args=>args[0]==='status'?' M helper.mjs\n':normal(args)})),/helper source must be clean/);
 assert.throws(()=>captureSourceBinding(fixture({actualDirectory:repo})),/declared clean checkout/);
});
test('A changed committed helper byte is refused even if HEAD and porcelain claim clean',()=>{
 const normal=fixture().readGit;assert.throws(()=>captureSourceBinding(fixture({readGit:args=>args[0]==='show'&&args[1].endsWith('/fenced-docker-proxy.mjs')?Buffer.from('different shared proxy'):normal(args)})),/differs from its selected commit/);
});
test('Runtime local import graph is completely covered by source binding',()=>{
 const visited=new Set();function visit(path){if(visited.has(path))return;visited.add(path);assert(helperPaths.includes(path),'Unbound runtime module '+path);const source=readFileSync(join(repo,path),'utf8');for(const [,target] of source.matchAll(/\bfrom\s+['"](\.[^'"]+\.mjs)['"]/g)){const resolved=resolve(repo,dirname(path),target);assert(resolved.startsWith(repo+'/'));visit(resolved.slice(repo.length+1));}}
 visit('scripts/acceptance/worker-recovery/installed-worker-recovery.mjs');assert.deepEqual([...visited].sort(),[...helperPaths].sort());
});
