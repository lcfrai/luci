import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {posix} from 'node:path';
const original=process.env.C058_ORIGINAL==='1';
const source=readFileSync(new URL(original?'../installed-v3/launch.mjs':'./launch.mjs',import.meta.url),'utf8');
const inspect=source.slice(source.indexOf(' function inspectOwned(')+1,source.indexOf('\n function inventory('));
const helper=source.includes('export function assertPrimaryGitAdminMount(')?source.slice(source.indexOf('export function assertPrimaryGitAdminMount(')+7,source.indexOf('\nfunction stableFile')):'';
const id='a'.repeat(64),imageID='sha256:'+'b'.repeat(64),root='/owned/project',state='/owned/state',git=root+'/.git',admin=git+'/worktrees';
function fixture(change=()=>{},paths={}) {
 const v={Id:id,Image:imageID,Config:{Labels:{'luci.instance':'instance-fixture','luci.managed':'true','luci.session-id':'s-1234abcd','luci.sandbox-id':'sandbox-fixture'}},Name:'/luci-sandbox-fixture',Mounts:[{Type:'bind',Source:root,Destination:'/home/dev/work',RW:true},{Type:'bind',Source:admin,Destination:'/home/dev/work/.git/worktrees',RW:false},{Type:'bind',Source:state+'/conversations/h/owned',Destination:'/home/dev/.luci-shell',RW:true}],HostConfig:{Memory:1073741824,MemorySwap:1073741824,NanoCpus:1000000000,PidsLimit:256,Privileged:false,SecurityOpt:['no-new-privileges:true'],RestartPolicy:{Name:'no'},AutoRemove:false,PortBindings:{'39200/tcp':[{HostIp:'127.0.0.1'}]}},State:{Status:'running',Pid:123},Created:'fixture',StartedAt:'fixture'};
 change(v);const calls=[],containers=new Map();
 const context=vm.createContext({assert,JSON,join:posix.join,sep:'/',project:{path:root},state,imageID,instance:'instance-fixture',containers,realpathSync:p=>paths[p]?.real??p,lstatSync:p=>{calls.push(['lstat',p]);if(paths[p]?.error)throw Error('fixture unavailable');return {isSymbolicLink:()=>paths[p]?.link??false,isDirectory:()=>paths[p]?.directory??true};},readRegistry:()=>[{id:'s-1234abcd',sandboxId:'sandbox-fixture'}],docker:(...args)=>{calls.push(args);assert.deepEqual(args,['inspect',id]);return JSON.stringify([v]);}});
 const run=vm.runInContext(helper+'\n('+inspect+')',context);let value,error;try{value=run(id);}catch(e){error=e;}return {value,error,calls,containers};
}
function reject(change,paths){const r=fixture(change,paths);assert(r.error,'unsafe inspected mount must fail');assert.equal(r.containers.size,0,'unconfirmed allocation not accepted');assert.equal(r.calls.filter(c=>c[0]==='inspect').length,1);assert(r.calls.every(c=>c[0]==='inspect'||c[0]==='lstat'),'no mutation or replay');}
test('exact primary RW + Git guard RO + private conversation binds are accepted',()=>{const r=fixture();assert.ifError(r.error);assert.equal(r.value.id,id);assert.equal(r.containers.size,1);assert.equal(r.calls.filter(c=>c[0]==='inspect').length,1);});
test('foreign source refused',()=>reject(v=>v.Mounts[1].Source='/foreign/worktrees'));
test('arbitrary project descendant refused',()=>reject(v=>{v.Mounts[1].Source=root+'/nested';v.Mounts[1].Destination='/home/dev/nested';}));
test('nested admin source refused',()=>reject(v=>v.Mounts[1].Source=admin+'/nested'));
test('wrong guard destination refused',()=>reject(v=>v.Mounts[1].Destination='/home/dev/work/.git/other'));
test('writable guard refused',()=>reject(v=>v.Mounts[1].RW=true));
test('nonboolean read-only property refused',()=>reject(v=>v.Mounts[1].RW=0));
test('state-backed guard destination impostor refused',()=>reject(v=>v.Mounts[1].Source=state+'/impostor'));
test('alias source resolving to guard refused',()=>reject(v=>v.Mounts[1].Source=root+'/alias',{[root+'/alias']:{real:admin}}));
test('git symlink refused even if resolution stays inside project',()=>reject(()=>{},{[git]:{link:true,real:root+'/decoy'}}));
test('admin symlink refused even if resolution stays inside project',()=>reject(()=>{},{[admin]:{link:true,real:root+'/decoy'}}));
test('git regular file refused for this primary-only fixture',()=>reject(()=>{},{[git]:{directory:false}}));
test('unreadable guard refuses observation',()=>reject(()=>{},{[admin]:{error:true}}));
test('canonical mismatch without symlink flag refused',()=>reject(()=>{},{[admin]:{real:root+'/decoy'}}));
test('duplicate guard refused',()=>reject(v=>v.Mounts.push({...v.Mounts[1]})));
test('missing primary guard refused',()=>reject(v=>v.Mounts.splice(1,1)));
test('non-bind guard refused',()=>reject(v=>v.Mounts[1].Type='volume'));
test('primary root must remain RW',()=>reject(v=>v.Mounts[0].RW=false));
test('immutable image identity remains required',()=>reject(v=>v.Image='sha256:'+'c'.repeat(64)));
test('foreign instance remains refused',()=>reject(v=>v.Config.Labels['luci.instance']='foreign'));
test('host resource limits remain exact',()=>reject(v=>v.HostConfig.Memory=2147483648));
