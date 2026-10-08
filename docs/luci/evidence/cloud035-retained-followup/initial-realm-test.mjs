// Real temporary exports, captured files, transcripts and Git repositories.
// Executes the installed adapter's retained(), bundleSnapshot() and transcriptFiles().
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,appendFileSync,readdirSync,realpathSync,lstatSync,existsSync,rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {contextFileNames} from '../sustained-four-project/task-context.mjs';
import {projectExportPaths} from './project-context.mjs';

const source=readFileSync(new URL('./installed-worker-recovery.mjs',import.meta.url),'utf8');
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const plain=value=>JSON.parse(JSON.stringify(value));
function slice(startText,endText){const start=source.indexOf(startText),end=source.indexOf(endText,start);assert(start>=0&&end>start);return source.slice(start,end);}
const body=slice('function bundleSnapshot(','\nfunction holdFinalResponse(')+'\n'+slice(' function transcriptFiles(','\n async function snapshot(');
function fixture(t){
 const root=realpathSync(mkdtempSync(join(tmpdir(),'luci-retained-context-')));t.after(()=>rmSync(root,{recursive:true,force:true}));
 const primary=join(root,'primary'),checkout=join(root,'task'),capture=join(root,'capture'),native=join(root,'native');
 const names=contextFileNames(2),canonicalBytes=Object.fromEntries(projectExportPaths.map(path=>[path,Buffer.from(path.endsWith('.json')?JSON.stringify({projectId:'p-target',revision:1}):'Reviewed p-target '+path+'\n')]));
 const gitEnv={PATH:process.env.PATH,LANG:'C',GIT_CONFIG_GLOBAL:'/dev/null',GIT_CONFIG_SYSTEM:'/dev/null',GIT_CONFIG_NOSYSTEM:'1',GIT_TERMINAL_PROMPT:'0',GIT_AUTHOR_NAME:'Controlled fixture',GIT_AUTHOR_EMAIL:'fixture@example.invalid',GIT_COMMITTER_NAME:'Controlled fixture',GIT_COMMITTER_EMAIL:'fixture@example.invalid'};
 const git=(cwd,...args)=>execFileSync('git',['-c','core.hooksPath=/dev/null',...args],{cwd,env:gitEnv,encoding:'utf8',timeout:5000}).trim();
 for(const dir of [primary,checkout,capture]){mkdirSync(join(dir,'.luci'),{recursive:true});for(const [path,bytes] of Object.entries(canonicalBytes))writeFileSync(join(dir,path),bytes);}
 for(const dir of [primary,checkout]){git(dir,'init','--quiet','--initial-branch=main');git(dir,'add','--','.');git(dir,'-c','commit.gpgsign=false','commit','--quiet','-m','Controlled reviewed context');}
 const primaryHead=git(primary,'rev-parse','HEAD'),output=Buffer.from('Controlled build-a result\n');
 writeFileSync(join(checkout,'build-a-result.txt'),output);git(checkout,'add','--','build-a-result.txt');git(checkout,'-c','commit.gpgsign=false','commit','--quiet','-m','Controlled task result');
 const checkoutHead=git(checkout,'rev-parse','HEAD');
 appendFileSync(join(capture,'LUCI.md'),'\nAssigned task contract\n');
 writeFileSync(join(capture,'.luci/brief.json'),JSON.stringify({projectId:'p-target',revision:1,purpose:'Controlled brief'}));
 writeFileSync(join(capture,'.luci/task-handoffs.json'),JSON.stringify({version:1,projectId:'p-target',runId:'run-target',taskId:'build-a',operationId:'op-target',dependencies:[]}));
 const files=Object.fromEntries(names.map(path=>[path,readFileSync(join(capture,path))]));
 writeFileSync(join(capture,'manifest.json'),JSON.stringify({version:2,files:names.map(path=>({path,bytes:files[path].length,digest:sha(files[path])}))}));
 const expectedContext=Object.fromEntries([...names,'manifest.json'].map(path=>[path,sha(readFileSync(join(capture,path)))]));
 const transcriptDir=join(native,'sessions','day');mkdirSync(transcriptDir,{recursive:true});
 const transcriptPath=join(transcriptDir,'native.jsonl'),transcript=Buffer.from('{"type":"controlled-native-output","task":"build-a"}\n');writeFileSync(transcriptPath,transcript);
 const observations=new Map([['build-a',{task:'build-a',operation:'op-target',commit:checkoutHead,outputSHA256:sha(output)}]]);
 const retained=vm.runInNewContext(body+'\nretained',{
  assert,join,readFileSync,lstatSync,readdirSync,realpathSync,existsSync,sha,names,projectExportPaths,
  targetWorker:{session:{id:'s-target'},checkout,task:'build-a'},projectStates:[{owner:{path:primary}}],
  nativeHomes:new Map([['s-target',native]]),bundles:new Map([['s-target',capture]]),observations,
  transcriptPrefixes:new Map(),retainedBaseline:undefined,
  execFileSync:(command,args,options)=>{assert.equal(command,'git');return execFileSync(command,args,{...options,env:gitEnv});},
 },{timeout:1000});
 return {retained:()=>plain(retained()),primary,checkout,capture,transcriptPath,observations,git,canonicalBytes,expected:{
  context:expectedContext,checkoutHead,checkoutStatus:'',outputSHA256:sha(output),primaryHead,primaryStatus:'',
  canonical:Object.fromEntries(Object.entries(canonicalBytes).map(([path,bytes])=>[path,sha(bytes)])),
  observationSHA256:sha(JSON.stringify(observations.get('build-a'))),
  transcript:[{relative:'sessions/day/native.jsonl',retainedPrefixBytes:transcript.length,retainedPrefixSHA256:sha(transcript)}],
 }};
}

test('Actual retained path records canonical exports, frozen context, Git state, output and native transcript',t=>{
 const f=fixture(t),before=f.retained();assert.deepEqual(before,f.expected);
 assert(!existsSync(join(f.primary,'.luci/brief.json')));assert(!existsSync(join(f.checkout,'.luci/brief.json')));
 appendFileSync(f.transcriptPath,'{"type":"later-native-event"}\n');assert.deepEqual(f.retained(),before);
});

test('Retained snapshot exposes canonical, captured-context, output, checkout and observation changes',t=>{
 const f=fixture(t),before=f.retained();
 for(const [path,bytes] of Object.entries(f.canonicalBytes)){
  appendFileSync(join(f.primary,path),'changed');const after=f.retained();assert.notEqual(after.canonical[path],before.canonical[path]);assert.notEqual(after.primaryStatus,'');writeFileSync(join(f.primary,path),bytes);
 }
 appendFileSync(join(f.capture,'.luci/brief.json'),' ');assert.notEqual(f.retained().context['.luci/brief.json'],before.context['.luci/brief.json']);
 appendFileSync(join(f.checkout,'build-a-result.txt'),'changed');let after=f.retained();assert.notEqual(after.outputSHA256,before.outputSHA256);assert.notEqual(after.checkoutStatus,'');
 f.git(f.checkout,'add','--','build-a-result.txt');f.git(f.checkout,'-c','commit.gpgsign=false','commit','--quiet','-m','Changed controlled result');after=f.retained();assert.notEqual(after.checkoutHead,before.checkoutHead);assert.equal(after.checkoutStatus,'');
 f.observations.get('build-a').operation='op-changed';assert.notEqual(f.retained().observationSHA256,before.observationSHA256);
});

test('Retained path refuses altered transcript prefixes and missing captured files',t=>{
 const f=fixture(t);f.retained();const transcript=readFileSync(f.transcriptPath);writeFileSync(f.transcriptPath,Buffer.alloc(transcript.length,120));
 assert.throws(()=>f.retained(),/Recorded transcript prefix changed/);writeFileSync(f.transcriptPath,transcript);
 rmSync(join(f.capture,'.luci/brief.json'));assert.throws(()=>f.retained(),error=>error.code==='ENOENT');
});
