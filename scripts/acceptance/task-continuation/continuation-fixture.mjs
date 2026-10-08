// Deterministic source/check definitions, without import-time I/O.
export const answer='Use exactly 42 as the final sum. Preserve the retained note and committed partial result.';
export const question='Should the final sum be exactly 42 while preserving this retained note?';
export const partialFiles={'committed.mjs':'export const partial = 17;\n','work.mjs':"export const phase = 'draft';\n",'note.txt':'Retain this unfinished note through continuation.\n'};
export const finalFiles={...partialFiles,'work.mjs':"export const phase = 'complete';\n",'result.json':'{"partial":17,"remaining":25,"sum":42}\n'};
export const variants={partial:{taskId:'task-work',attempt:1,files:partialFiles},finish:{taskId:'task-work',attempt:2,files:finalFiles}};
export const taskContracts=[
 {id:'task-work',title:'Continue the retained implementation',dependsOn:[],writePaths:Object.keys(finalFiles),acceptance:'Keep the partial result17 and retained note, resolve the reported final-sum question explicitly, and finish with sum42 in a separately reviewed continuation.'},
 {id:'task-join',title:'Wait for the reviewed implementation',dependsOn:['task-work'],writePaths:['later.txt'],acceptance:'Do not start until task-work is explicitly integrated. This acceptance fixture does not authorize this dependent worker.'},
];
export const verifySource=`import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {partial} from './committed.mjs';
import {phase} from './work.mjs';
const mode=process.argv[2];assert(['partial','finish','final'].includes(mode));assert.equal(partial,17);
assert.equal(readFileSync('note.txt','utf8'),'Retain this unfinished note through continuation.\\n');
if(mode==='partial')assert.equal(phase,'draft');else{assert.equal(phase,'complete');assert.equal(readFileSync('result.json','utf8'),'${finalFiles['result.json'].replaceAll('\n','\\n')}');}
console.log('CONTINUATION_CHECK_OK '+mode);
`;
export const seedFiles={'README.md':'Controlled two-worker continuation fixture. Worker claims are separate from integration and verification.\n','work.mjs':"export const phase = 'initial';\n",'verify.mjs':verifySource};
export function variantFor(taskId,attempt){const keys=Object.keys(variants).filter(key=>variants[key].taskId===taskId&&variants[key].attempt===attempt);if(keys.length!==1)throw Error('Only two exact continuation workers are authorized');return keys[0];}
export const changedPaths=key=>{if(!variants[key])throw Error('Unknown variant');return Object.keys(variants[key].files).sort();};
export function taskHandoff(key){if(!variants[key])throw Error('Unknown variant');return {version:1,summary:'Controlled continuation '+key,changedFiles:changedPaths(key),decisions:[{decision:key==='partial'?'Leave unfinished source for explicit reviewed continuation':'Follow the recorded answer in this attempt’s frozen previous-attempt context',rationale:'A worker report is a claim; checkpoint preparation, integration and independent checks are separate.'}],reportedChecks:[{command:'node verify.mjs '+key,outcome:'passed',detail:'Exact controlled fixture outputs checked in the native worker.'}],questions:key==='partial'?[{question,impact:'The retained implementation stays incomplete until an editor decides the intended result.',blocking:true}]:[],limitations:['Scripted provider. No live-model quality or deployment claim.']};}
export function taskCommand({variant,operation,projectId,runId,baseCommit,contextHashes,dependencyDigest,previousAttempt,expectedUid}){
 const v=variants[variant];if(!v||!Number.isSafeInteger(expectedUid)||expectedUid<=0||!/^[a-f0-9]{40}$/.test(baseCommit)||!/^[a-f0-9]{64}$/.test(dependencyDigest))throw Error('Invalid exact task identity');
 for(const id of [operation,projectId,runId])if(!/^[A-Za-z0-9_-]{1,200}$/.test(id))throw Error('Invalid scope');
 const c={variant,operation,projectId,runId,baseCommit,contextHashes,dependencyDigest,previousAttempt,expectedUid,...v,report:taskHandoff(variant),answer,partialFiles};
 const source=`const fs=require('node:fs'),crypto=require('node:crypto'),path=require('node:path'),assert=require('node:assert/strict'),cp=require('node:child_process');
const c=${JSON.stringify(c)},root='/home/dev/.luci-reviewed-context',hash=b=>crypto.createHash('sha256').update(b).digest('hex');
assert.equal(process.cwd(),'/home/dev/work');assert.equal(process.getuid(),c.expectedUid);
const git=(...args)=>cp.execFileSync('git',['-c','core.hooksPath=/dev/null',...args],{encoding:'utf8',timeout:15000,maxBuffer:1024*1024}).trim();
assert.equal(git('rev-parse','HEAD'),c.baseCommit);assert.equal(git('status','--porcelain'),'');
const files={};for(const [name,digest] of Object.entries(c.contextHashes)){const file=path.join(root,name),st=fs.lstatSync(file);assert(st.isFile()&&!st.isSymbolicLink()&&st.size<8*1024**2);const bytes=fs.readFileSync(file);assert.equal(hash(bytes),digest);files[name]={sha256:hash(bytes),bytes:bytes.length};}
const manifest=JSON.parse(fs.readFileSync(path.join(root,'manifest.json')));assert.equal(manifest.version,2);assert.equal(manifest.binding.projectId,c.projectId);assert.deepEqual(manifest.binding.taskIds,[c.taskId]);assert.equal(manifest.binding.dependencyContextDigest,c.dependencyDigest);
const packet=JSON.parse(fs.readFileSync(path.join(root,'.luci/task-handoffs.json')));assert.equal(packet.version,1);assert.equal(packet.projectId,c.projectId);assert.equal(packet.runId,c.runId);assert.equal(packet.taskId,c.taskId);assert.equal(packet.operationId,c.operation);assert.deepEqual(packet.dependencies,[]);
if(c.variant==='finish'){assert.deepEqual(packet.previousAttempt,c.previousAttempt);assert.equal(packet.previousAttempt.integration,null);assert.deepEqual(packet.previousAttempt.recordedChecks,[]);assert.equal(packet.previousAttempt.answers.length,1);assert.equal(packet.previousAttempt.answers[0].answer,c.answer);for(const [name,text] of Object.entries(c.partialFiles))assert.equal(fs.readFileSync(name,'utf8'),text);assert.deepEqual(git('ls-files','--',...Object.keys(c.partialFiles)).split('\\n').sort(),Object.keys(c.partialFiles).sort());}else assert.equal(packet.previousAttempt,undefined);
let readOnly=false;try{fs.writeFileSync(path.join(root,'.continuation-write-probe'),'must refuse',{flag:'wx'});}catch(e){assert.equal(e.code,'EROFS');readOnly=true;}assert(readOnly);
const write=(name,text)=>{if(fs.existsSync(name)){const st=fs.lstatSync(name);assert(st.isFile()&&!st.isSymbolicLink()&&st.nlink===1);}fs.writeFileSync(name,text,{mode:0o600});};
if(c.variant==='partial'){write('committed.mjs',c.files['committed.mjs']);git('add','--','committed.mjs');git('-c','user.name=Luci continuation fixture','-c','user.email=continuation@example.invalid','commit','-m','Retained committed partial result');write('work.mjs',c.files['work.mjs']);write('note.txt',c.files['note.txt']);assert.deepEqual(git('status','--porcelain').split('\\n').map(x=>x.trim()).sort(),['?? note.txt','M work.mjs']);}else{for(const [name,text] of Object.entries(c.files))write(name,text);}
const began=Date.now(),check=cp.spawnSync(process.execPath,['verify.mjs',c.variant],{encoding:'utf8',timeout:15000,maxBuffer:1024*1024});assert.equal(check.status,0,check.stderr);assert.equal(check.signal,null);assert.equal(check.stdout,'CONTINUATION_CHECK_OK '+c.variant+'\\n');
if(c.variant==='finish'){git('add','--',...Object.keys(c.files));git('-c','user.name=Luci continuation fixture','-c','user.email=continuation@example.invalid','commit','-m','Complete separately reviewed continuation');assert.equal(git('status','--porcelain'),'');}
const reportRoot='/home/dev/.luci-task-output';assert.equal(fs.realpathSync(reportRoot),reportRoot);assert.deepEqual(fs.readdirSync(reportRoot),[]);fs.writeFileSync(path.join(reportRoot,'handoff.json'),JSON.stringify(c.report,null,2)+'\\n',{flag:'wx',mode:0o600});
const artifact={variant:c.variant,baseCommit:c.baseCommit,check:{code:check.status,signal:check.signal,stdout:check.stdout,elapsedMs:Date.now()-began},outputs:Object.fromEntries(Object.keys(c.files).map(name=>[name,hash(fs.readFileSync(name))]))};
console.log('LUCI_CONTINUATION_OBSERVATION='+Buffer.from(JSON.stringify({variant:c.variant,taskId:c.taskId,operation:c.operation,uid:process.getuid(),cwd:process.cwd(),readOnly,files,changed:Object.keys(c.files).sort(),artifact,commit:git('rev-parse','HEAD'),gitStatus:git('status','--porcelain'),reported:c.report})).toString('base64'));
`;
 return {cmd:"node <<'LUCI_CONTINUATION_NODE'\n"+source+'\nLUCI_CONTINUATION_NODE',workdir:'/home/dev/work',yield_time_ms:1000,max_output_tokens:12000};
}
