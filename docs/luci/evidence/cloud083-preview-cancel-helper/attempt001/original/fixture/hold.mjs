import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFileSync,openSync,writeFileSync,fsyncSync,closeSync,renameSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';

const manifest=JSON.parse(readFileSync('/cancel/manifest.json','utf8'));
assert.match(manifest.nonce,/^[a-f0-9-]{36}$/);
assert.match(manifest.projectId,/^p-[a-f0-9]+$/);
assert.equal(manifest.holdMs,45_000);
const sourceSHA256=createHash('sha256').update(readFileSync('/cancel/hold.mjs')).digest('hex');
assert.equal(sourceSHA256,manifest.holdSourceSHA256);
const paths=['/sys/fs/cgroup/memory.max','/sys/fs/cgroup/memory.swap.max','/sys/fs/cgroup/cpu.max','/sys/fs/cgroup/memory/memory.limit_in_bytes','/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes','/sys/fs/cgroup/cpu/cpu.cfs_quota_us','/sys/fs/cgroup/cpu/cpu.cfs_period_us'];
const cgroup={};
for(const path of paths){
 try{const text=readFileSync(path,'utf8');assert.ok(Buffer.byteLength(text)<1024);cgroup[path]={status:'read',text:text.trim()};}
 catch(error){cgroup[path]={status:'unavailable',code:error.code??error.name};}
}
const proof={schema:1,nonce:manifest.nonce,projectId:manifest.projectId,sourceSHA256,holdMs:manifest.holdMs,startedUnixMs:Date.now(),node:process.version,platform:process.platform,architecture:process.arch,uid:process.getuid(),cgroup};
const bytes=Buffer.from(JSON.stringify(proof)+'\n');assert.ok(bytes.length<=16384);
const temporary='/cancel/.stage-proof-'+manifest.nonce+'.tmp';
const fd=openSync(temporary,'wx',0o600);try{writeFileSync(fd,bytes);fsyncSync(fd);}finally{closeSync(fd);}
renameSync(temporary,'/cancel/stage-proof.json');
await new Promise((resolve,reject)=>process.stdout.write('LUCI_PREVIEW_CANCEL_STAGE '+JSON.stringify(proof)+'\n',error=>error?reject(error):resolve()));
await delay(manifest.holdMs);
// Even if the cancellation mechanism fails, this fixture cannot reach image
// success or serving allocation. It performs no network requests; its only writes are this bounded proof in the owned RUN filesystem.
process.exitCode=79;
