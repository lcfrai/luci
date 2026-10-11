import assert from 'node:assert/strict';
import {test} from 'node:test';
import {readFileSync} from 'node:fs';
import {EventEmitter} from 'node:events';
import {createHash} from 'node:crypto';
import {readProofArchive,parseProofArchive} from './proof-archive.mjs';

const id='a'.repeat(64);
function tar(payload,{name='stage-proof.json',type='0',link='',declared,pax,duplicate=false}={}){
 const record=(name,body,type='0',link='',size=body.length)=>{
  const h=Buffer.alloc(512);h.write(name,0,100);h.write('0000600\0',100);h.write('0000000\0',108);h.write('0000000\0',116);h.write(size.toString(8).padStart(11,'0')+'\0',124);h.write('00000000000\0',136);h.fill(32,148,156);h.write(type,156);h.write(link,157,100);h.write('ustar\0',257);h.write('00',263);
  const sum=[...h].reduce((a,b)=>a+b,0);h.write(sum.toString(8).padStart(6,'0')+'\0 ',148);
  return Buffer.concat([h,body,Buffer.alloc((512-body.length%512)%512)]);
 };
 const chunks=[];
 if(pax){let text='';for(const [key,value] of Object.entries(pax)){const row=key+'='+value+'\n';let n=row.length+2;while(String(n).length+1+row.length!==n)n=String(n).length+1+row.length;text+=n+' '+row;}chunks.push(record('PaxHeader',Buffer.from(text),'x'));}
 const body=Buffer.isBuffer(payload)?payload:Buffer.from(JSON.stringify(payload));chunks.push(record(name,body,type,link,declared??body.length));if(duplicate)chunks.push(record(name,body));chunks.push(Buffer.alloc(1024));return Buffer.concat(chunks);
}
function mockTransport({status=200,body=tar({schema:1}),headers={},error,aborted=false,hold=false}={}){
 let seen;
 return {request(options,callback){seen=options;const req=new EventEmitter();req.destroy=()=>{};req.end=()=>queueMicrotask(()=>{if(error){req.emit('error',new Error('private transport content'));return;}const res=new EventEmitter();res.statusCode=status;res.headers=headers;res.destroy=()=>{};callback(res);if(hold)return;if(aborted){res.emit('aborted');return;}res.emit('data',body);res.emit('end');});return req;},get seen(){return seen;}};
}
test('read-only endpoint pins full immutable ID and proof path, with bounded hashed metadata',async()=>{
 const transport=mockTransport(),rows=[];const bytes=await readProofArchive(id,{request:transport.request,onResponse:x=>rows.push(x)});assert.deepEqual(parseProofArchive(bytes),{schema:1});assert.deepEqual(transport.seen,{socketPath:'/var/run/docker.sock',path:'/containers/'+id+'/archive?path=%2Fcancel%2Fstage-proof.json',method:'GET'});assert.equal(rows[0].sha256,createHash('sha256').update(bytes).digest('hex'));assert.equal(rows.length,1);
});
test('proof not created yet returns null only on bounded404',async()=>assert.equal(await readProofArchive(id,{request:mockTransport({status:404,body:Buffer.from('{}')}).request}),null));
for(const [label,opts] of Object.entries({foreignStatus:{status:500},declaredOversize:{headers:{'content-length':'65537'}},invalidDeclaredSize:{headers:{'content-length':'private'}},materializedOversize:{body:Buffer.alloc(65537)},transportError:{error:true},interrupted:{aborted:true}}))test('archive transport refuses '+label,async()=>assert.rejects(readProofArchive(id,{request:mockTransport(opts).request})));
test('parent cancellation interrupts held archive read',async()=>{const ac=new AbortController();const pending=readProofArchive(id,{signal:ac.signal,request:mockTransport({hold:true}).request});ac.abort();await assert.rejects(pending);});
test('wrong immutable identity refuses before transport',async()=>{let sent=false;await assert.rejects(readProofArchive('short',{request(){sent=true;}}));assert.equal(sent,false);});
test('observation retention failure refuses success',async()=>assert.rejects(readProofArchive(id,{request:mockTransport().request,onResponse(){throw new Error('private');}})));
test('timestamp-only PAX metadata is accepted',()=>assert.deepEqual(parseProofArchive(tar({schema:1},{pax:{mtime:'1720000000.000000001'}})),{schema:1}));
for(const [label,bytes] of Object.entries({traversal:tar({},{name:'../stage-proof.json'}),absolute:tar({},{name:'/stage-proof.json'}),wrongName:tar({},{name:'wrong.json'}),symlink:tar({},{type:'2',link:'/private'}),hardlink:tar({},{type:'1',link:'private'}),duplicate:tar({},{duplicate:true}),oversizedMember:tar(Buffer.alloc(16385)),malformedJSON:tar(Buffer.from('private malformed')),malformedUTF8:tar(Buffer.from([0xff])),arrayShape:tar([]),paxPath:tar({},{pax:{path:'stage-proof.json'}}),trailingData:Buffer.concat([tar({}),Buffer.alloc(512,1)]),missingEnd:tar({}).subarray(0,1024),archiveOversize:Buffer.alloc(66048),checksum:Buffer.from(tar({})).fill(0,148,156)}))test('in-memory parser refuses '+label+' without reflecting content',()=>assert.throws(()=>parseProofArchive(bytes),/bounded proof archive parser refused|falsy value/));

// Same body against exact original/final handler. The original classic builder
// logging channel rejects; the corrected handler reads the in-memory archive.
test('actual RUN observation remains possible with unsupported container logs',async()=>{
 const source=readFileSync(process.env.STAGE_SOURCE??new URL('./cancel-preview.mjs',import.meta.url),'utf8');
 const start=source.indexOf("await until('Exact nonce RUN',async()=>{");const end=source.indexOf('\n },120_000);',start);assert.ok(start>=0&&end>start);
 const body=source.slice(start+"await until('Exact nonce RUN',async()=>{".length,end);
 const info={State:{Status:'running',Running:true},HostConfig:{LogConfig:{Type:'none'}}};
 const bindings={assert,Buffer,Date,startOutcome:null,nonceContainers:()=>[id],dockerJSON:()=>[info],write:()=>{},verifyStageContainer:x=>{assert.equal(x.HostConfig.LogConfig.Type,'none');return{id};},docker:()=>{throw new Error('configured logging driver does not support reading');},verifyProof:x=>x,parseProofArchive,readProofArchive:x=>readProofArchive(x,{request:mockTransport({body:tar({architecture:'x64'})}).request}),abort:new AbortController(),archiveObservations:[],arch:'amd64',result:{}};
 const fn=Function(...Object.keys(bindings),'return (async()=>{let stageProof,containerID;'+body+'})')(...Object.values(bindings));assert.equal(await fn(),true);
});
test('shutdown allowance covers10sHTTP plus5sPreviewClose with bounded margin',()=>{const source=readFileSync(new URL('./cancel-preview.mjs',import.meta.url),'utf8');assert.match(source,/Date\.now\(\)\+20_000\);while\(!childClosed/);assert.match(source,/controllerEscalationAttempted=true;fail\('controller-shutdown'/);assert.doesNotMatch(source,/controllerKillRequired/);});
