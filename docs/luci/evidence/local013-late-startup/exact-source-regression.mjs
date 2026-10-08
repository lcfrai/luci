// Executes exact driver source slices. All external effects are injected spies.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {join,dirname} from 'node:path';
import vm from 'node:vm';
import {createWorkflowDeadline,runBoundedCleanup} from '../../../../scripts/acceptance/launch-recovery/acceptance-lifecycle.mjs';
import {createStartupScope,startAcceptanceServices} from '../../../../scripts/acceptance/launch-recovery/acceptance-startup.mjs';
const mode=process.argv[2];assert(['original','corrected'].includes(mode));
const extraction=JSON.parse(readFileSync(new URL('./orchestration-extraction.json',import.meta.url)))[mode];
const hash=value=>createHash('sha256').update(value).digest('hex');
for(const key of ['startup','service'])assert.equal(hash(extraction[key].source),extraction[key].SHA256);
const deferred=()=>{let resolve;const promise=new Promise(done=>{resolve=done;});return{promise,resolve};};
const entered=deferred(),factory=deferred(),cleanupEntered=deferred(),cleanupHeld=deferred();
const control=createWorkflowDeadline({timeoutMs:2000}),calls=[],failures=[];
const health=()=>control.assertActive();
const scope=createStartupScope({assertActive:health,onFailure:error=>failures.push(error.code),lateCloseTimeoutMs:15});
const context=vm.createContext({assert,join,dirname,health,startAcceptanceServices,startupScope:scope,
 process:{env:{}},fixture:'/controlled-fixture',binary:'/controlled-package/luci',web:'/controlled-package/web',image:'controlled-image',lifecycle:'controlled-token',bridge:'127.0.0.1',
 provider:undefined,proxy:undefined,service:undefined,env:undefined,base:undefined,providerBase:undefined,fixtureError:undefined,AbortSignal,
 journal:{record(){}},receipt:{bounds:{maxCreates:5,maxStarts:5}},controllerIdentities:[],
 identifyCreate(){},admission(){},stopOnFailure(){},remainingBudget:()=>45000,
 processIdentity:pid=>({pid,start:'1'}),journalProcessIdentity:pid=>({pid,startTicks:1}),
 freePort:async()=>1234,writeFileSync:()=>calls.push('config-write'),
 spawn:()=>{calls.push('service-spawn');return{pid:123,exitCode:null,on(){return this;}};},
 fetch:async()=>({ok:true,json:async()=>({product:'Luci',version:'0.5.0'})}),delay:async()=>{},
 startProvider(){context.provider={once(){return this;},listen(...args){args.at(-1)();return this;},address(){return{port:1234};},closeAllConnections(){},listening:false};},
 startFencedDockerProxy:()=>{entered.resolve();return factory.promise;},
});
const operation=vm.runInContext(extraction.service.source+'\n(async()=>{\n'+extraction.startup.source+'\n})()',context,{filename:'exact-'+mode+'-dag-startup.mjs'});
const raced=control.run(()=>operation);await entered.promise;
control.fail(Object.assign(Error('controlled cancellation'),{code:'controlled-cancel'}));
await assert.rejects(raced,{code:'controlled-cancel'});
// Same regression for both source revisions: controller cleanup completes while
// proxy creation is pending; another cleanup phase stays held when it returns.
const phases=runBoundedCleanup({checkpoint(){},phases:[
 {name:'controllers',timeoutMs:50,run:()=>{assert.equal(context.service,undefined);calls.push('controller-cleanup-complete');return true;}},
 {name:'proxy',timeoutMs:50,run:()=>scope.has('proxy')?scope.close('proxy',5):context.proxy?context.proxy.close():{drained:true,notStarted:true}},
 {name:'held-phase',timeoutMs:1000,run:()=>{cleanupEntered.resolve();return cleanupHeld.promise;}},
]});
await cleanupEntered.promise;
factory.resolve({close:async()=>{calls.push('proxy-close');return{drained:true,pendingRequests:0,activeHandlers:0};},observations:{}});
let outcome='resolved';try{await operation;}catch(error){outcome=error.code??error.name;}
cleanupHeld.resolve();const cleanup=await phases;control.close();
const result={mode,sourceSHA256:extraction.sourceSHA256,startupSliceSHA256:extraction.startup.SHA256,serviceSliceSHA256:extraction.service.SHA256,postCancellationSpawns:calls.filter(row=>row==='service-spawn').length,postCancellationConfigWrites:calls.filter(row=>row==='config-write').length,calls,outcome,cleanup,startupResources:scope.snapshot(),failures,externalProcessesStarted:0,DockerRequests:0};
console.log(JSON.stringify(result,null,2));
// This exact invariant must fail on the retained original source.
assert.equal(result.postCancellationSpawns,0,'No service may spawn after cancellation and completed controller cleanup');
assert.equal(result.postCancellationConfigWrites,0,'No configuration write may follow cancellation');
