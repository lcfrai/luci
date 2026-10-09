// Prepared durable local evidence helpers. Importing has no side effects.
import assert from 'node:assert/strict';
import {createHash,randomUUID} from 'node:crypto';
import {openSync,closeSync,writeSync,fsyncSync,readFileSync,renameSync,constants} from 'node:fs';
import {dirname} from 'node:path';
export const sha=value=>createHash('sha256').update(value).digest('hex');
function syncDirectory(path){const fd=openSync(dirname(path),constants.O_RDONLY|constants.O_DIRECTORY|constants.O_NOFOLLOW);try{fsyncSync(fd);}finally{closeSync(fd);}}
export function durableJSON(path,value,{replace=false,maxBytes=65536}={}){
 const bytes=Buffer.from(JSON.stringify(value,null,2)+'\n');assert(bytes.length<=maxBytes,'Receipt exceeds declared byte budget');
 const target=replace?path+'.'+randomUUID()+'.tmp':path;const fd=openSync(target,constants.O_WRONLY|constants.O_CREAT|constants.O_EXCL|constants.O_NOFOLLOW,0o600);
 try{let offset=0;while(offset<bytes.length){const written=writeSync(fd,bytes,offset);assert(written>0,"Receipt write made no progress");offset+=written;}fsyncSync(fd);}finally{closeSync(fd);}
 if(replace)renameSync(target,path);syncDirectory(path);return {sha256:sha(bytes),bytes:bytes.length};
}
export function createJournal(path,{maxEvents=2048,maxBytes=4*1024**2,maxEventBytes=8192,secrets=[]}={}){
 const fd=openSync(path,constants.O_WRONLY|constants.O_CREAT|constants.O_EXCL|constants.O_NOFOLLOW,0o600);syncDirectory(path);
 let count=0,bytes=0,closed=false;
 return {
  record(input){assert(!closed);const data=Buffer.from(JSON.stringify({sequence:count+1,at:new Date().toISOString(),...input})+'\n');assert(data.length<=maxEventBytes&&bytes+data.length<=maxBytes&&count<maxEvents,'Journal budget exhausted');for(const secret of secrets)assert(!secret||!data.includes(Buffer.from(secret)),'Secret refused in evidence');let offset=0;while(offset<data.length){const written=writeSync(fd,data,offset);assert(written>0,"Journal write made no progress");offset+=written;}fsyncSync(fd);count++;bytes+=data.length;},
  close(){if(!closed){fsyncSync(fd);closeSync(fd);closed=true;}},
  summary(){return {path:'events.jsonl',count,bytes,sha256:sha(readFileSync(path))};},
 };
}
export function createBackgroundFence(fail){const pending=new Set();let problem=null;return {
 track(operation){const task=Promise.resolve().then(operation);pending.add(task);void task.catch(error=>{problem??=error;fail(error);}).finally(()=>pending.delete(task));},
 check(){if(problem)throw problem;},
 async drain(){await Promise.all([...pending]);if(problem)throw problem;},
 get size(){return pending.size;},
};}

export function controllerExitObserver({isClosing,record,fail}){return (code,signal)=>{
 try{record({kind:'controller-exit',code,signal});}catch(error){fail(error);}
 if(!isClosing())fail(Object.assign(new Error('Owned controller exited before cleanup'),{code:'controller-exited'}));
};}
