// Pre-authored deterministic fixture. Importing this module performs no I/O.
export const taskContracts = [
 {id:'task-a',title:'Build alpha branch',dependsOn:[],writePaths:['alpha.mjs','shared.txt','receipt-a.json'],acceptance:'Export alpha=17 and record alpha in shared.txt; pass the alpha check.'},
 {id:'task-b',title:'Build beta branch',dependsOn:[],writePaths:['beta.mjs','shared.txt','receipt-b1.json','receipt-b2.json'],acceptance:'Export beta=25 and record beta in shared.txt; when starting after alpha integration preserve alpha. Pass the corresponding beta check. Overlapping write scope requires serial execution.'},
 {id:'task-join',title:'Join reviewed branch results',dependsOn:['task-a','task-b'],writePaths:['result.json','receipt-join.json'],acceptance:'Only after both current task integrations, read alpha=17 and beta=25, confirm shared alpha and beta, and emit exactly {"alpha":17,"beta":25,"sum":42} plus newline; pass final checks.'},
];
export const variants={a:{taskId:'task-a',attempt:1,files:{'alpha.mjs':'export const alpha = 17;\n','shared.txt':'alpha\n'},receipt:'receipt-a.json'},b1:{taskId:'task-b',attempt:2,files:{'beta.mjs':'export const beta = 25;\n','shared.txt':'beta\n'},receipt:'receipt-b1.json'},b2:{taskId:'task-b',attempt:3,files:{'beta.mjs':'export const beta = 25;\n','shared.txt':'alpha\nbeta\n'},receipt:'receipt-b2.json'},join:{taskId:'task-join',attempt:4,files:{'result.json':'{"alpha":17,"beta":25,"sum":42}\n'},receipt:'receipt-join.json'}};
export const expectedResult='{"alpha":17,"beta":25,"sum":42}\n';
export const verifySource=`import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
const mode=process.argv[2];assert(['a','b1','b2','join','final'].includes(mode));
const text=name=>readFileSync(name,'utf8');
const load=name=>import(pathToFileURL(resolve(name)).href);
if(mode==='a'){assert.equal((await load('alpha.mjs')).alpha,17);assert.equal(text('shared.txt'),'alpha\\n');}
if(mode==='b1'){assert.equal((await load('beta.mjs')).beta,25);assert.equal(text('shared.txt'),'beta\\n');}
if(['b2','join','final'].includes(mode)){assert.equal((await load('alpha.mjs')).alpha,17);assert.equal((await load('beta.mjs')).beta,25);assert.equal(text('shared.txt'),'alpha\\nbeta\\n');}
if(['join','final'].includes(mode)){assert.equal(text('result.json'),'${expectedResult.replaceAll('\n','\\n')}');const result=JSON.parse(text('result.json'));assert.deepEqual(result,{alpha:17,beta:25,sum:42});}
if(mode==='final'){for(const name of ['receipt-a.json','receipt-b2.json','receipt-join.json']){const r=JSON.parse(text(name));assert.equal(r.check.code,0);assert.match(r.baseCommit,/^[a-f0-9]{40,64}$/);}assert.equal(existsSync('receipt-b1.json'),false);}
console.log('DAG_CHECK_OK '+mode);
`;
export const seedFiles={'verify.mjs':verifySource,'README.md':'Controlled DAG fixture: A and B have no dependency edge but deliberately overlap shared.txt, so writes are serial. Join depends on reviewed integrations, not process exit. No model quality claim.\n'};

export function variantFor(taskId,attempt){const matches=Object.entries(variants).filter(([,v])=>v.taskId===taskId&&v.attempt===attempt);if(matches.length!==1)throw Error('Only four exact DAG attempts are authorized');return matches[0][0];}
export function changedPaths(variant){const v=variants[variant];if(!v)throw Error('Unknown variant');return [...Object.keys(v.files),v.receipt].sort();}
export function taskHandoff(variant){
 const v=variants[variant];if(!v)throw Error("Unknown variant");return {version:1,summary:"Controlled DAG "+variant+" checked and committed",changedFiles:changedPaths(variant),decisions:[{decision:variant==='b2'?'Preserve reviewed alpha while adding beta':'Implement the exact saved DAG fixture contract',rationale:'Scripted acceptance fixture; this is a worker claim, not independent project verification.'}],reportedChecks:[{command:"node verify.mjs "+variant,outcome:"passed",detail:"Exact checked fixture outputs match the approved variant."}],questions:[],limitations:["Scripted provider; no model-quality or deployment claim. Human integration and whole-build checks remain separate."]};
}
export function taskCommand({variant,operation,projectId,runId,baseCommit,contextHashes,dependencyDigest,dependencies=[],expectedUid}){
 const v=variants[variant];if(!v||!Number.isSafeInteger(expectedUid)||expectedUid<=0||!/^[a-f0-9]{40,64}$/.test(baseCommit))throw Error('Invalid exact task command identity');
 for(const id of [operation,projectId,runId])if(!/^[A-Za-z0-9_-]{1,200}$/.test(id))throw Error('Invalid scope');
 const config={variant,operation,projectId,runId,baseCommit,contextHashes,dependencyDigest,dependencies,expectedUid,report:taskHandoff(variant),...v};
 const code=`const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict'),cp=require('node:child_process');
const c=${JSON.stringify(config)},root='/home/dev/.luci-reviewed-context';
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
assert.equal(process.cwd(),'/home/dev/work');assert.equal(process.getuid(),c.expectedUid);
const git=(...args)=>cp.execFileSync('git',['-c','core.hooksPath=/dev/null',...args],{encoding:'utf8',timeout:15000,maxBuffer:1024*1024}).trim();
assert.equal(git('rev-parse','HEAD'),c.baseCommit);assert.equal(git('status','--porcelain'),'');
const files={};for(const [name,digest] of Object.entries(c.contextHashes)){const file=path.join(root,name),st=fs.lstatSync(file);assert(st.isFile()&&!st.isSymbolicLink()&&st.size<2*1024*1024);const bytes=fs.readFileSync(file);assert.equal(hash(bytes),digest);files[name]={sha256:hash(bytes),bytes:bytes.length};}
const manifest=JSON.parse(fs.readFileSync(path.join(root,'manifest.json'),'utf8'));assert.equal(manifest.version,2);assert.equal(manifest.binding.projectId,c.projectId);assert.deepEqual(manifest.binding.taskIds,[c.taskId]);assert.equal(manifest.binding.dependencyContextDigest,c.dependencyDigest);
const packet=JSON.parse(fs.readFileSync(path.join(root,'.luci/task-handoffs.json'),'utf8'));assert.equal(packet.version,1);assert.equal(packet.projectId,c.projectId);assert.equal(packet.runId,c.runId);assert.equal(packet.taskId,c.taskId);assert.equal(packet.operationId,c.operation);assert.deepEqual(packet.dependencies.map(d=>({taskId:d.taskId,operationId:d.operationId,commit:d.integration.commit,sourceCommit:d.integration.sourceCommit,childId:d.receipt.childId,contextDigest:d.receipt.contextDigest,reportDigest:d.digest,report:d.receipt.reported})),c.dependencies);
let readOnly=false;try{fs.writeFileSync(path.join(root,'.dag-write-probe'),'must refuse',{flag:'wx'});}catch(e){assert.equal(e.code,'EROFS');readOnly=true;}assert(readOnly);
if(c.variant==='b2'||c.variant==='join'){assert.equal(fs.readFileSync('alpha.mjs','utf8'),'export const alpha = 17;\\n');}if(c.variant==='join'){assert.equal(fs.readFileSync('beta.mjs','utf8'),'export const beta = 25;\\n');}
for(const [name,text] of Object.entries(c.files)){assert(!name.includes('/'));if(fs.existsSync(name)){const s=fs.lstatSync(name);assert(s.isFile()&&!s.isSymbolicLink()&&s.nlink===1);}fs.writeFileSync(name,text,{mode:0o600});}
const began=Date.now(),check=cp.spawnSync(process.execPath,['verify.mjs',c.variant],{encoding:'utf8',timeout:15000,maxBuffer:1024*1024});assert.equal(check.status,0,check.stderr);assert.equal(check.signal,null);assert.equal(check.stdout,'DAG_CHECK_OK '+c.variant+'\\n');
const artifact={variant:c.variant,taskId:c.taskId,operation:c.operation,baseCommit:c.baseCommit,contextHashes:c.contextHashes,check:{code:check.status,signal:check.signal,stdout:check.stdout,elapsedMs:Date.now()-began},outputs:Object.fromEntries(Object.keys(c.files).map(name=>[name,hash(fs.readFileSync(name))]))};
fs.writeFileSync(c.receipt,JSON.stringify(artifact,null,2)+'\\n',{flag:'wx',mode:0o600});
const changed=[...Object.keys(c.files),c.receipt].sort();git('add','--',...changed);git('-c','user.name=Luci DAG fixture','-c','user.email=dag@example.invalid','commit','-m','Controlled DAG '+c.variant);assert.equal(git('status','--porcelain'),'');
assert.deepEqual(git('diff','--name-only',c.baseCommit,'HEAD','--').split('\\n').sort(),changed);
const reportRoot='/home/dev/.luci-task-output';assert.equal(fs.realpathSync(reportRoot),reportRoot);assert(fs.statSync(reportRoot).isDirectory());assert.deepEqual(fs.readdirSync(reportRoot),[]);fs.writeFileSync(path.join(reportRoot,'handoff.json'),JSON.stringify(c.report,null,2)+'\\n',{flag:'wx',mode:0o600});
const result={variant:c.variant,taskId:c.taskId,operation:c.operation,uid:process.getuid(),cwd:process.cwd(),readOnly,files,changed,artifact,commit:git('rev-parse','HEAD'),receiptSHA256:hash(fs.readFileSync(c.receipt)),reported:c.report};console.log('LUCI_DAG_OBSERVATION='+Buffer.from(JSON.stringify(result)).toString('base64'));
`;
 return {cmd:"node <<'LUCI_DAG_NODE'\n"+code+'\nLUCI_DAG_NODE',workdir:'/home/dev/work',yield_time_ms:1000,max_output_tokens:12000};
}
