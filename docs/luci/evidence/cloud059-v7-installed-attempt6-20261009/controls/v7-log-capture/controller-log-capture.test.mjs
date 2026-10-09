import test from 'node:test';
import assert from 'node:assert/strict';
import {PassThrough} from 'node:stream';
import * as fs from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {createControllerLogCapture,projectAttachRelayLine} from './controller-log-capture.mjs';
const id='s-1234abcd',hash=b=>createHash('sha256').update(b).digest('hex');
const log=(error='failed to get reader: context canceled',session=id)=>Buffer.from('time=2026-10-09T12:34:56.123Z level=INFO msg="attach relay ended" route=attach session='+session+' err='+JSON.stringify(error)+'\n');
function fixture(t,callbacks={}){
 const dir=fs.mkdtempSync(join(tmpdir(),'luci-controller-log-fixture-'));fs.chmodSync(dir,0o700);
 const path=join(dir,'stderr.private.log'),failures=[],relays=[],capture=createControllerLogCapture({path,expectedSession:()=>id,onFailure:x=>failures.push(x),onRelay:x=>relays.push(x),...callbacks}),stream=new PassThrough();capture.attach(stream);
 t.after(()=>{stream.destroy();fs.rmSync(dir,{recursive:true,force:true});});return {dir,path,failures,relays,capture,stream};
}
test('genuine streamed private0600 bytes and both hashes settle only on end',async t=>{
 const f=fixture(t),body=log();f.stream.write(body.subarray(0,19));f.stream.write(body.subarray(19));
 assert.equal(f.capture.snapshot().completeStreamSHA256,null);assert.equal(f.capture.snapshot().closed,false);f.stream.end();const r=await f.capture.settle(200);
 assert.equal(r.passed,true);assert.equal(r.observedBytes,body.length);assert.equal(r.completeStreamSHA256,hash(body));assert.equal(r.retainedPrefixSHA256,hash(body));assert.deepEqual(fs.readFileSync(f.path),body);assert.equal(fs.statSync(f.path).mode&0o777,0o600);assert.equal(r.relayRows.length,1);assert.equal(r.relayRows[0].flags.contextCanceled,true);assert.equal(r.relayRows[0].errorSHA256,hash(Buffer.from('failed to get reader: context canceled')));assert.equal(r.identityVerified,true);
});
test('exact relay session/route/message required; arbitrary stderr remains private',async t=>{
 const f=fixture(t),secret='S3CR3T https://example.invalid/?ticket=PRIVATE /private/root';const body=Buffer.concat([log(secret,'s-ffffffff'),Buffer.from(log(secret).toString().replace('route=attach','route=other')),Buffer.from(log(secret).toString().replace('attach relay ended','unrelated text')),log(secret)]);f.stream.end(body);const r=await f.capture.settle(200);assert.equal(r.passed,true);assert.equal(r.relayRows.length,1);assert.equal(r.relayRows[0].flags.unclassified,true);assert(!JSON.stringify(r).includes(secret));assert(!JSON.stringify(f.relays).includes('ticket='));assert.deepEqual(fs.readFileSync(f.path),body);
});
test('capture cap retains exact256KiB prefix but drains/hashes all observed bytes and fails',async t=>{
 const f=fixture(t),body=Buffer.from('opaque\n'.repeat(45000));f.stream.end(body);const r=await f.capture.settle(300);assert.equal(r.passed,false);assert.equal(r.retainedBytes,262144);assert.equal(r.observedBytes,body.length);assert.equal(r.completeStreamSHA256,hash(body));assert.equal(r.truncated,true);assert.deepEqual(fs.readFileSync(f.path),body.subarray(0,262144));assert.equal(f.failures.filter(x=>x.code==='private_log_byte_limit').length,1);
});
test('overlong line never projected, and following bounded relay still captured',async t=>{
 const f=fixture(t);f.stream.write(Buffer.alloc(16384,120));f.stream.write(Buffer.from('x\n'));f.stream.end(log('EOF'));const r=await f.capture.settle(200);assert.equal(r.oversizedLines,1);assert.equal(r.relayRows.length,1);assert.equal(r.relayRows[0].flags.eof,true);assert.equal(r.passed,false);assert(r.failures.includes('line_byte_limit'));
});
test('matching rows capped at16 with explicit overflow and no dropped-stream hash',async t=>{
 const f=fixture(t);const body=Buffer.concat(Array.from({length:17},()=>log('EOF')));f.stream.end(body);const r=await f.capture.settle(200);assert.equal(r.relayRows.length,16);assert.equal(r.matchingLines,17);assert.equal(r.omittedRelayRows,1);assert.equal(f.relays.length,16);assert.equal(r.completeStreamSHA256,hash(body));assert.equal(r.passed,false);
});
test('unfinished stream times out boundedly, closes file, and never claims complete hash',async t=>{
 const f=fixture(t);f.stream.write(log());const r=await f.capture.settle(10);assert.equal(r.settlementTimedOut,true);assert.equal(r.endObserved,false);assert.equal(r.closed,true);assert.equal(r.completeStreamSHA256,null);assert.equal(r.passed,false);assert(r.failures.includes('stderr_settlement_timeout'));f.stream.write(Buffer.from('late\n'));assert.equal(f.capture.snapshot().retainedBytes,r.retainedBytes);assert(f.capture.snapshot().observedBytes>r.observedBytes);
});
test('stream error and close without end are explicit incomplete observations',async t=>{
 const f=fixture(t);f.stream.write(log());f.stream.destroy(Error('S3CR3T stderr transport failure'));const r=await f.capture.settle(200);assert.equal(r.errorObserved,true);assert.equal(r.endObserved,false);assert.equal(r.completeStreamSHA256,null);assert.equal(r.passed,false);assert(!JSON.stringify(r).includes('S3CR3T'));assert(r.failures.includes('stderr_stream_error'));
});
test('plain close without end is never accepted',async t=>{const f=fixture(t);f.stream.destroy();const r=await f.capture.settle(200);assert.equal(r.endObserved,false);assert.equal(r.completeStreamSHA256,null);assert.equal(r.passed,false);assert(r.failures.includes('stderr_closed_without_end'));});
test('trailing incomplete line retained privately but not projected',async t=>{const f=fixture(t),body=log().subarray(0,-1);f.stream.end(body);const r=await f.capture.settle(200);assert.equal(r.trailingPartialLine,true);assert.equal(r.relayRows.length,0);assert.equal(r.completeStreamSHA256,hash(body));assert.equal(r.passed,false);});
test('unattached capture is closed as explicit failure; cannot signal or launch anything',async t=>{
 const dir=fs.mkdtempSync(join(tmpdir(),'luci-unattached-'));fs.chmodSync(dir,0o700);t.after(()=>fs.rmSync(dir,{recursive:true,force:true}));const c=createControllerLogCapture({path:join(dir,'private'),expectedSession:()=>id});const r=await c.settle(20);assert.equal(r.closed,true);assert.equal(r.passed,false);assert.equal(r.completeStreamSHA256,null);assert(r.failures.includes('stderr_not_attached'));
});
test('nonprivate parent, existing file and symlink are refused without overwrites',t=>{
 const dir=fs.mkdtempSync(join(tmpdir(),'luci-refuse-'));t.after(()=>fs.rmSync(dir,{recursive:true,force:true}));fs.chmodSync(dir,0o755);assert.throws(()=>createControllerLogCapture({path:join(dir,'new'),expectedSession:()=>id}),/initialization failed/);fs.chmodSync(dir,0o700);const file=join(dir,'existing');fs.writeFileSync(file,'keep',{mode:0o600});assert.throws(()=>createControllerLogCapture({path:file,expectedSession:()=>id}),/initialization failed/);const link=join(dir,'link');fs.symlinkSync(file,link);assert.throws(()=>createControllerLogCapture({path:link,expectedSession:()=>id}),/initialization failed/);assert.equal(fs.readFileSync(file,'utf8'),'keep');
});
test('hardlink added during capture invalidates ownership at settlement',async t=>{const f=fixture(t);f.stream.write(log());fs.linkSync(f.path,join(f.dir,'other'));f.stream.end();const r=await f.capture.settle(200);assert.equal(r.identityVerified,false);assert.equal(r.passed,false);assert(r.failures.includes('private_file_identity_changed'));});
test('path replacement cannot receive old descriptor writes or claim identity',async t=>{const f=fixture(t);f.stream.write(log());fs.renameSync(f.path,join(f.dir,'original'));fs.writeFileSync(f.path,'replacement',{mode:0o600});f.stream.end(log('EOF'));const r=await f.capture.settle(200);assert.equal(r.identityVerified,false);assert.equal(r.passed,false);assert.equal(fs.readFileSync(f.path,'utf8'),'replacement');});
test('session callback changes never merge foreign relay rows',async t=>{let current=null;const f=fixture(t,{expectedSession:()=>current});f.stream.write(log());current=id;f.stream.write(log());current='s-abcdef12';f.stream.end(log());const r=await f.capture.settle(200);assert.equal(r.relayRows.length,1);assert.equal(r.relayRows[0].session,id);assert.equal(r.unprojectedLines,2);});
test('callback exceptions are static capture failures and never escape event delivery',async t=>{const f=fixture(t,{onRelay:()=>{throw Error('PRIVATE_CALLBACK');},onFailure:()=>{throw Error('PRIVATE_FAILURE');}});f.stream.end(log());const r=await f.capture.settle(200);assert.equal(r.passed,false);assert(r.failures.includes('relay_callback_failed'));assert(r.failures.includes('failure_callback_failed'));assert(!JSON.stringify(r).includes('PRIVATE_'));});
test('strict Go quoted-byte hashes and conservative fixed classifications',()=>{
 const make=error=>projectAttachRelayLine(log(error).subarray(0,-1),id);
 assert.equal(make('failed to read frame header: EOF').flags.eof,true);assert.equal(make('failed to get reader: context deadline exceeded').flags.deadlineExceeded,true);
 const close=make('failed to get reader: received close frame: status = StatusNormalClosure and reason = "S3CR3T context canceled"');assert.equal(close.flags.websocketStatus,1000);assert.equal(close.flags.contextCanceled,false);assert(!JSON.stringify(close).includes('S3CR3T'));
 const bytes=Buffer.from('time=2026-10-09T12:34:56Z level=INFO msg="attach relay ended" route=attach session='+id+' err="\\xff"');const row=projectAttachRelayLine(bytes,id);assert.equal(row.errorBytes,1);assert.equal(row.errorSHA256,hash(Buffer.from([255])));assert.equal(row.flags.unclassified,true);
 const duplicate=Buffer.from(log().toString().trim()+' session='+id);assert.equal(projectAttachRelayLine(duplicate,id),null);assert.equal(projectAttachRelayLine(Buffer.from(log().toString().replace('level=INFO','level=SECRET').trim()),id),null);
});
test('already-flowing stream and second attachment are rejected conservatively',async t=>{const f=fixture(t);assert.throws(()=>f.capture.attach(new PassThrough()),/already attached/);f.stream.end();assert.equal((await f.capture.settle(200)).passed,true);assert.throws(()=>f.capture.settle(3001),/budget/);});
