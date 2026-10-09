import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {pathToFileURL} from 'node:url';
import {resolve} from 'node:path';
const root=new URL('.',import.meta.url).pathname,variant=process.env.C069_VARIANT??'warm',sourceRoot=resolve(process.env.C069_SOURCE_ROOT??root);
const path=sourceRoot+'/'+variant+'/scenario.mjs',source=readFileSync(path,'utf8'),module=await import(pathToFileURL(path)),wait=module.waitForOwnedTerminalSettlement;
const sessionId='s-1234abcd',projectId='p-5678abcd',id='a'.repeat(64),gen='b'.repeat(64);
const allocation={id,sandboxId:sessionId,sessionId,instanceId:'fixture-instance',imageID:'sha256:'+'c'.repeat(64),name:'/luci-'+sessionId,status:'running',memoryBytes:1024**3,cpuNano:1_000_000_000};
const lease={id:'manual-owned',sessionId,projectId,sandboxId:sessionId,state:'backend-settled',resources:{CPUMillis:1000,MemoryMiB:1024}};
const ledger=leases=>({present:true,sha256:'d'.repeat(64),bytes:498,leases:structuredClone(leases),checks:[],fences:[],checkFences:[]});
function value(status='exited',held=true){return {initial:{sessionId,executionGeneration:gen,containers:[structuredClone(allocation)],ledger:ledger([lease])},current:{session:{id:sessionId,projectId,status:'exited',sandboxId:sessionId,sandboxRef:id,credentialsCleared:true},containers:status===null?[]:[{...allocation,status}],ledger:ledger(held?[lease]:[]),rootFreeBytes:7*1024**3}};}
function freeze(v){if(v&&typeof v==='object'){Object.freeze(v);for(const x of Object.values(v))freeze(x);}return v;}
async function run(values,options={}){let clock=0,calls=0,error,result;const records=[],budgets=[],before=JSON.stringify(values);freeze(values);try{assert.equal(typeof wait,'function','Pinned exact waiter required');result=await wait({sessionId,projectId,initialExecutionGeneration:gen,health:()=>{},record:r=>records.push(r),observe:({budgetMs})=>{budgets.push(budgetMs);return values[Math.min(calls++,values.length-1)];},now:()=>clock,sleep:async ms=>{clock+=ms;},...options});}catch(e){error=e;}assert.equal(JSON.stringify(values),before,'Read-only fixture bytes preserved');return {clock,calls,error,result,records,budgets};}
test('owned removing stays pending through absent-but-charged and succeeds only after exact absence plus release',async()=>{const r=await run([value('removing'),value(null),value(null,false)]);assert.ifError(r.error);assert.equal(r.calls,3);assert.equal(r.clock,700);assert.equal(r.result.last.containers.length,0);assert.equal(r.result.last.ledger.leases.length,0);assert(r.records.some(x=>x.kind==='initial-settlement-lifecycle'&&x.statuses[0]?.status==='removing'));assert.equal(r.records.filter(x=>x.kind==='initial-settlement-confirmed').length,1);});
for(const status of ['removing','dead'])test('persistent exact owned '+status+' and charge time out without settlement or mutation',async()=>{const r=await run([value(status)]);assert.match(r.error?.message??'',/cleanup remains unconfirmed/);assert(r.clock>34000&&r.clock<35000);assert.equal(r.calls,100);assert(!r.records.some(x=>x.kind==='initial-settlement-confirmed'));assert.equal(r.records.filter(x=>x.kind==='initial-settlement-lifecycle').length,1);});
test('existing exited path remains pending before exact release',async()=>{const r=await run([value(),value(null,false)]);assert.ifError(r.error);assert.equal(r.calls,2);});
test('absent container with its charge never means success',async()=>{const r=await run([value(null)]);assert.match(r.error?.message??'',/cleanup remains unconfirmed/);assert(!r.result);});
for(const [name,change,message] of [
 ['foreign identity',v=>v.current.containers[0].id='f'.repeat(64),/ownership changed/],
 ['new allocation',v=>v.current.containers.push({...allocation,id:'f'.repeat(64)}),/Unknown or additional/],
 ['missing charge',v=>v.current.ledger.leases=[],/expected charge/],
 ['changed lease scope',v=>v.current.ledger.leases[0].projectId='other-project',/Unrelated or changed charge/],
 ['unavailable ledger',v=>v.current.ledger.present=false,/Ledger unavailable/],
 ['missing fence knowledge',v=>delete v.current.ledger.checkFences,/Incomplete held-resource/],
 ])test(name+' refuses without settlement',async()=>{const v=value('removing');change(v);const r=await run([v]);assert.match(r.error?.message??'',message);assert.equal(r.calls,1);assert(!r.result);});
test('unknown raw status is refused after bounded hash-only diagnosis',async()=>{const secret='unexpected-private-value\nwith-arbitrary-details';const r=await run([value(secret)]);assert.match(r.error?.message??'',/Allocation state unconfirmed/);const row=r.records.find(x=>x.kind==='initial-settlement-lifecycle');assert(row);assert.equal(row.statuses[0].status,null);assert.equal(row.statuses[0].bytes,Buffer.byteLength(secret));assert.match(row.statuses[0].sha256,/^[a-f0-9]{64}$/);assert(!JSON.stringify(row).includes(secret));assert.equal(row.heldLeaseCount,1);assert(!r.result);});
test('recognized but unaccepted paused state is diagnosed and refused',async()=>{const r=await run([value('paused')]);assert.match(r.error?.message??'',/Allocation state unconfirmed/);assert.equal(r.records.find(x=>x.kind==='initial-settlement-lifecycle')?.statuses[0].status,'paused');});
test('caller remaining budget is consumed without restarting the35s allowance',async()=>{const r=await run([value('removing')],{budgetMs:1000});assert.match(r.error?.message??'',/cleanup remains unconfirmed/);assert.equal(r.clock,700);assert.deepEqual(r.budgets,[1000,650,300]);});
test('expired caller budget refuses before observation',async()=>{const r=await run([value(null,false)],{budgetMs:0});assert.match(r.error?.message??'',/budget exhausted/);assert.equal(r.calls,0);});
test('waiter contains no command, Docker or registry write capability',()=>{assert.equal(typeof wait,'function');const body=String(wait);assert(!/execFile|spawn\(|docker\(|writeFile|\.stop\(|\.destroy\(/.test(body));});
