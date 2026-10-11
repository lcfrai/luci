import assert from 'node:assert/strict';
import {request as httpRequest} from 'node:http';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

export const archiveLimits=Object.freeze({bytes:65536,proofBytes:16384,timeoutMs:3000,parserMs:2000});
export async function readProofArchive(containerID,{signal,request=httpRequest,onResponse=()=>{}}={}){
 assert.match(containerID,/^[a-f0-9]{64}$/);
 if(signal)signal.throwIfAborted();
 const path='/containers/'+containerID+'/archive?path='+encodeURIComponent('/cancel/stage-proof.json');
 return await new Promise((resolve,reject)=>{
  let done=false,response,req;
  const settle=(error,value)=>{if(done)return;done=true;clearTimeout(timer);signal?.removeEventListener('abort',onAbort);if(error){response?.destroy();req?.destroy();reject(error);}else resolve(value);};
  const onAbort=()=>settle(new Error('Proof archive request cancelled'));
  const timer=setTimeout(()=>settle(new Error('Proof archive request deadline exceeded')),archiveLimits.timeoutMs);
  signal?.addEventListener('abort',onAbort,{once:true});
  try{
   req=request({socketPath:'/var/run/docker.sock',path,method:'GET'},res=>{
    response=res;let bytes=0;const chunks=[];
    const length=res.headers?.['content-length'];
    if(length!==undefined&&(!/^\d+$/.test(String(length))||Number(length)>archiveLimits.bytes)){settle(new Error('Proof archive declared body exceeds bound'));return;}
    res.on('data',chunk=>{if(done)return;bytes+=chunk.length;if(bytes>archiveLimits.bytes){settle(new Error('Proof archive body exceeds bound'));return;}chunks.push(chunk);});
    res.once('error',()=>settle(new Error('Proof archive response failed')));
    res.once('aborted',()=>settle(new Error('Proof archive response interrupted')));
    res.once('end',()=>{
     if(done)return;const body=Buffer.concat(chunks,bytes);
     try{onResponse({method:'GET',routeClass:'exact-container-proof-archive',containerID,status:res.statusCode,bytes,sha256:createHash('sha256').update(body).digest('hex')});}catch{settle(new Error('Proof archive observation could not be retained'));return;}
     if(res.statusCode===404){settle(null,null);return;}
     if(res.statusCode!==200){settle(new Error('Proof archive HTTP status refused'));return;}
     settle(null,body);
    });
   });
   req.once('error',()=>settle(new Error('Proof archive request failed')));req.end();
  }catch{settle(new Error('Proof archive request setup failed'));}
 });
}

export function parseProofArchive(bytes){
 assert.ok(Buffer.isBuffer(bytes)&&bytes.length>0&&bytes.length<=archiveLimits.bytes);
 const result=spawnSync('python3',['-I','-B',fileURLToPath(new URL('./proof-archive.py',import.meta.url))],{input:bytes,encoding:'utf8',timeout:archiveLimits.parserMs,maxBuffer:archiveLimits.proofBytes,killSignal:'SIGKILL',env:{PATH:process.env.PATH,LANG:'C.UTF-8'}});
 assert.ok(!result.error&&result.status===0,'Exact bounded proof archive parser refused');
 assert.ok(Buffer.byteLength(result.stdout)<=archiveLimits.proofBytes);
 return JSON.parse(result.stdout);
}
