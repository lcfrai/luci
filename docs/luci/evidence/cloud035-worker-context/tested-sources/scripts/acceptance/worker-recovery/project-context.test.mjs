import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {createHash} from 'node:crypto';
import {mkdtempSync,mkdirSync,readFileSync,writeFileSync,rmSync,existsSync,realpathSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,sep} from 'node:path';
import {contextFileNames} from '../sustained-four-project/task-context.mjs';
import {captureProjectContext,verifyCapturedProjectContext,projectExportPaths} from './project-context.mjs';

function fixture(t){
 const root=mkdtempSync(join(tmpdir(),'luci-recovery-context-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 const projects=['a','b'].map(letter=>{
  const project={id:'p-'+letter,path:join(root,letter)};mkdirSync(join(project.path,'.luci'),{recursive:true});
  const brief={projectId:project.id,revision:1,purpose:'Own project '+letter,criteria:[],structured:true};
  const context={projectId:project.id,exports:projectExportPaths.map(path=>({path,status:'current'}))};
  for(const path of projectExportPaths)writeFileSync(join(project.path,path),path.endsWith('.json')?JSON.stringify({projectId:project.id,revision:1}):'Own '+project.id+' '+path+'\n');
  return {project,context,brief,read:path=>readFileSync(join(project.path,path))};
 });
 return {root,projects};
}
const capture=owner=>captureProjectContext({projectId:owner.project.id,briefRevision:1,context:owner.context,brief:owner.brief,read:owner.read});
function captured(owner){return Object.fromEntries([...projectExportPaths.map(path=>[path,path==='LUCI.md'?Buffer.concat([owner.read(path),Buffer.from('\nAssigned task contract\n')]):owner.read(path)]),['.luci/brief.json',Buffer.from(JSON.stringify(owner.brief))]]);}

test('Actual installed seeding reads public checkout exports without inventing brief.json',async t=>{
 const {root,projects}=fixture(t),source=readFileSync(new URL('./installed-worker-recovery.mjs',import.meta.url),'utf8');
 const start=source.indexOf('async function seedProjects('),end=source.indexOf('\n async function pause(',start);assert(start>0&&end>start);
 const projectStates=[],calls=[];let selected=-1;
 async function api(method,path,body){
  calls.push({method,path});
  if(method==='POST'&&path==='/v1/product/projects')return projects[++selected].project;
  const owner=projects[selected],prefix='/v1/product/projects/'+owner.project.id;
  if(method==='GET'&&path===prefix+'/context')return owner.context;
  if(path===prefix+'/brief')return owner.brief;
  if(path===prefix+'/specification'||path===prefix+'/specification/approve')return {projectId:owner.project.id,revision:1,briefRevision:1};
  if(path===prefix+'/design-plan'||path===prefix+'/design-plan/approve'||path===prefix+'/plan')return {projectId:owner.project.id,revision:1};
  if(path===prefix+'/git'||path===prefix+'/git/commit')return {reviewDigest:'a'.repeat(64)};
  if(path==='/v1/projects/'+owner.project.id+'/branches')return {branches:[{name:'main',id:'branch-'+selected}]};
  if(path==='/v1/projects/'+owner.project.id+'/worktrees?branchId=branch-'+selected)return {worktrees:[{id:'tree-'+selected,isPrimary:true}]};
  if(method==='POST'&&path===prefix+'/runs')return {run:{id:'run-'+selected,input:{route:{permissionMode:'bypass'}}}};
  assert.fail('Unexpected seed API request: '+method+' '+path);
 }
 const seed=vm.runInNewContext(source.slice(start,end)+'\nseedProjects',{
  assert,api,workspace:'',fixture:root,sep,realpathSync,readFileSync,join,titles:new Map(),projectStates,
  canonicalNames:contextFileNames(1),sha:bytes=>createHash('sha256').update(bytes).digest('hex'),
  execFileSync:()=> 'a'.repeat(40),captureProjectContext,
 },{timeout:1000});
 await seed('connection');assert.equal(projectStates.length,2);
 for(const owner of projects){
  assert(!existsSync(join(owner.project.path,'.luci/brief.json')));
  assert(calls.some(call=>call.method==='GET'&&call.path==='/v1/product/projects/'+owner.project.id+'/context'));
  assert(calls.some(call=>call.method==='GET'&&call.path==='/v1/product/projects/'+owner.project.id+'/brief'));
 }
});

test('Public persisted exports and captured-only brief preserve their full own-project contents',t=>{
 const {projects:[owner]}=fixture(t),snapshot=capture(owner),files=captured(owner);
 verifyCapturedProjectContext({snapshot,read:path=>files[path]});
 assert(!Object.hasOwn(snapshot.canonicalBefore,'.luci/brief.json'));
 assert(!existsSync(join(owner.project.path,'.luci/brief.json')));
 owner.brief.purpose='later mutation';assert.equal(snapshot.brief.purpose,'Own project a');
});

test('Foreign identity, stale revision and invalid public exports fail before reading any path',t=>{
 const {projects:[owner]}=fixture(t);
 const changes=[value=>value.context.projectId='p-foreign',value=>value.brief.projectId='p-foreign',
  value=>value.brief.revision=2,value=>value.context.exports.pop(),value=>value.context.exports.push(value.context.exports[0]),
  value=>value.context.exports[0].path='../../outside',value=>value.context.exports[0].status='changed',
  value=>value.context.exports[0].status='missing',value=>value.context.exports.push({path:'.luci/brief.json',status:'current'})];
 for(const change of changes){
  const value={projectId:owner.project.id,briefRevision:1,brief:structuredClone(owner.brief),context:structuredClone(owner.context),read:()=>assert.fail('Invalid public contract read a file')};
  change(value);assert.throws(()=>captureProjectContext(value),error=>error.code==='ERR_ASSERTION'&&!error.message.includes('Invalid public contract read'));
 }
});

test('Missing real exports and altered captured exports, brief or project prefix are refused',t=>{
 const {projects:[owner]}=fixture(t),snapshot=capture(owner);
 for(const path of [...projectExportPaths,'.luci/brief.json']){
  const files=captured(owner);files[path]=Buffer.from(path==='.luci/brief.json'?JSON.stringify({...owner.brief,purpose:'wrong context'}):'altered');
  assert.throws(()=>verifyCapturedProjectContext({snapshot,read:name=>files[name]}));
 }
 const files=captured(owner);files['.luci/brief.json']=Buffer.from(JSON.stringify({...owner.brief,projectId:'p-other'}));
 assert.throws(()=>verifyCapturedProjectContext({snapshot,read:path=>files[path]}),/selected project/);
 rmSync(join(owner.project.path,'.luci/design-handoff.json'));
 assert.throws(()=>capture(owner),error=>error.code==='ENOENT');
});
