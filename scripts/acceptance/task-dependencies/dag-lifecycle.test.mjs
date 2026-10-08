// Exact driver functions/handler, controlled promises and injected effects only.
// Orphan tests use one short Node child at a time for real rejection semantics.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {spawnSync} from 'node:child_process';
import vm from 'node:vm';

const source=readFileSync(process.env.LUCI_DAG_LIFECYCLE_TEST_SOURCE??new URL('./dag.mjs',import.meta.url),'utf8');
function extract(start,end){const a=source.indexOf(start);assert(a>=0,start);const b=source.indexOf(end,a+start.length);assert(b>a,end);return source.slice(a,b);}
const action=extract('async function action(','async function openProject(');
const review=extract('async function readReview(','async function integrate(');
const checkWait=extract(' const pending=page.waitForResponse(response=>',' assert.deepEqual(response.request().postDataJSON()');
const provider=extract('function startProvider(){','async function action(');
const nextTool=extract('function nextTool(','function startProvider(');
const stream=extract('function streamResponse(','async function attach(');
const outputs=extract('function outputs(','function message(');
const message=extract('function message(','function commandFor(');
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return{promise,resolve,reject};};

for(const name of ['action','readReview','checkWait'])test('actual '+name+' keeps failed-click response rejection observed through cleanup',()=>{
 const selected=name==='action'?action:name==='readReview'?review:'async function checkWait(){\n'+checkWait+'\n}';
 const invocation=name==='action'?'\x27/controlled\x27,click':name==='readReview'?'\x27a\x27':'';
 const program=`
import assert from 'node:assert/strict';
import vm from 'node:vm';
const clickFailure=Object.assign(Error('controlled-click-failure'),{code:'controlled-click-failure'});
const click=async()=>{throw clickFailure;};
const page={waitForResponse(){return new Promise((_resolve,reject)=>{setTimeout(()=>reject(Error('controlled-orphan-response')),5);});}};
const context=vm.createContext({assert,URL,page,region:{getByRole:()=>({click})},health(){},remainingBudget:()=>50,setStage(){},openProject:async()=>{},owner:{route:'/project',run:{id:'run'}},taskData:new Map([['a',{taskId:'task-a'}]]),taskRow:()=>({getByRole:()=>({click})})});
vm.runInContext(${JSON.stringify(selected)},context);
try{await context[${JSON.stringify(name)}](${invocation});assert.fail('Expected click failure');}catch(error){assert.equal(error.code,'controlled-click-failure');}
await new Promise(resolve=>setTimeout(resolve,30));
console.log('bounded-cleanup-reached');
`;
 const child=spawnSync(process.execPath,['--unhandled-rejections=strict','--input-type=module'],{input:program,encoding:'utf8',timeout:2000,maxBuffer:64*1024});
 assert.equal(child.error,undefined);assert.equal(child.status,0,'Orphan waiter interrupted cleanup: '+child.stderr);assert.match(child.stdout,/bounded-cleanup-reached/);
});

function providerFixture() {
 const state={task:'a',taskId:'task-a',count:0,attached:true,operation:{id:'operation'},session:{id:'session'}},entered=deferred(),release=deferred(),calls=new Map(),writes=[],failures=[];
 let handler;
 const context=vm.createContext({assert,Buffer,provider:undefined,key:'synthetic-key',closing:false,failed:false,providerCalls:0,auxiliaryCalls:0,calls,receipt:{},authorityRoot:'/controlled-context',titles:new Map([['task-a','Controlled task']]),taskData:new Map([['a',state]]),initialBundles:new Map([['session',{}]]),observations:new Map(),
  health(){if(context.failed||context.closing)throw Object.assign(Error('controlled cancellation'),{code:'controlled-cancel'});},
  httpServer:value=>{handler=value;return{};},stopOnFailure:error=>failures.push(error.code??error.name),commandFor:()=>({cmd:'controlled-command'}),until:async()=>state,
 });
 vm.runInContext([outputs,message,nextTool,stream,provider].join('\n'),context);context.startProvider();
 const body=Buffer.from(JSON.stringify({model:'swarm-mechanics-fixture',tools:[{name:'exec_command'}],input:'Reviewed context directory: /controlled-context task-a: Controlled task'}));
 const response={headersSent:false,writeHead(status){this.headersSent=true;writes.push({status});},end(text){writes.push({text});}};
 return{state,entered,release,calls,writes,failures,context,body,response,handler};
}

for(const boundary of ['body','bundle'])test('actual provider refuses new tool dispatch after pending '+boundary+' resumes during cancellation',async()=>{
 const f=providerFixture();
 const request={method:'POST',url:'/v1/responses',headers:{authorization:'Bearer synthetic-key'},async *[Symbol.asyncIterator](){if(boundary==='body'){f.entered.resolve();await f.release.promise;}yield f.body;}};
 if(boundary==='bundle')f.context.until=async()=>{f.entered.resolve();await f.release.promise;return f.state;};
 const pending=f.handler(request,f.response);await f.entered.promise;f.context.failed=true;f.context.closing=true;f.release.resolve();await pending;
 assert.equal(f.calls.size,0,'No tool-dispatch authority may be recorded after cancellation');assert.equal(f.state.codingDispatches,undefined);assert.equal(f.writes.some(row=>row.status===200),false);assert.deepEqual(f.failures,['controlled-cancel']);
});

test('actual response writer refuses an already-built tool response after cancellation',()=>{
 const f=providerFixture();f.context.failed=true;f.context.closing=true;
 assert.throws(()=>f.context.streamResponse(f.response,{id:'synthetic-call',type:'function_call',name:'exec_command',arguments:'{}'},1),{code:'controlled-cancel'});assert.deepEqual(f.writes,[]);
});

test('healthy exact provider dispatch still emits one native tool response',async()=>{
 const f=providerFixture();const request={method:'POST',url:'/v1/responses',headers:{authorization:'Bearer synthetic-key'},async *[Symbol.asyncIterator](){yield f.body;}};
 await f.handler(request,f.response);assert.deepEqual(f.failures,[]);assert.equal(f.calls.size,1);assert.equal(f.state.codingDispatches,1);assert.equal(f.writes[0].status,200);assert.match(f.writes.at(-1).text,/exec_command/);
});

test('observing a response waiter never suppresses its awaited failure',async()=>{
 const rejected=Object.assign(Error('response failed'),{code:'controlled-response-failure'});
 const context=vm.createContext({assert,URL,health(){},remainingBudget:()=>50,page:{waitForResponse:()=>Promise.reject(rejected)}});
 vm.runInContext(action,context);await assert.rejects(context.action('/controlled',async()=>{}),{code:'controlled-response-failure'});
});
