// Prepared only. This module runs no browser, application, controller, or compiler.
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {webcrypto} from 'node:crypto';
import {mkdtempSync,chmodSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {createContext,runInContext} from 'node:vm';

export const ROOT=resolve(dirname(fileURLToPath(import.meta.url)),'..');
export const VARIANT=process.env.C077_VARIANT??'fixed';
assert(['before','fixed'].includes(VARIANT),'C077_VARIANT must select the exact before or fixed copy');

export async function loadVariant(){
  const root=join(ROOT,VARIANT);
  return {
    scenario:await import(pathToFileURL(join(root,'scenario.mjs'))),
    launch:await import(pathToFileURL(join(root,'launch.mjs'))),
  };
}

export const BASE='http://127.0.0.1:18730';
export const ORIGIN='ws://127.0.0.1:18730';
export const ATTACH='/v1/sessions/s-1234abcd/attach';
export const GENERATION_A='a'.repeat(64);
export const GENERATION_B='b'.repeat(64);
export const MARKERS={nonce:'controlled',ready:'C077_READY',first:'C077_FIRST',second:'C077_SECOND',body:'C077_BODY',blocked:'C077_BLOCKED'};

export function privateCapture(launch){
  const root=mkdtempSync(join(tmpdir(),'luci-c077-controlled-'));
  chmodSync(root,0o700);
  const failures=[];
  const path=join(root,'errors.jsonl');
  const capture=launch.createPrivateTransportErrors({path,onFailure:row=>failures.push(row)});
  let disposed=false;
  return {capture,path,failures,dispose(){if(disposed)return;disposed=true;try{capture.close();}finally{rmSync(root,{recursive:true,force:true});}}};
}

// Every synthetic observation is delivered to the copied production helper's
// exported listener implementation. These fakes do not calculate its budgets,
// phases, successful-stream selection, or evidence projections.
export function fakePage(){
  const page=new EventEmitter(),cdp=new EventEmitter(),sockets=[];
  cdp.commands=[];cdp.detachCalls=0;
  cdp.send=async method=>{assert.equal(method,'Network.enable');cdp.commands.push(method);return {};};
  cdp.detach=async()=>{cdp.detachCalls++;};
  const listeners=new WeakMap();
  class NativeSocket {
    constructor(url){
      this.url=String(url);this.readyState=0;this.binaryType='blob';
      this.requestId='controlled-socket-'+(sockets.length+1);
      this.playwright=new EventEmitter();this.playwright.url=()=>this.url;
      listeners.set(this,new Map());sockets.push(this);
      page.emit('websocket',this.playwright);
      cdp.emit('Network.webSocketCreated',{requestId:this.requestId,url:this.url});
    }
    addEventListener(name,callback){const map=listeners.get(this);if(!map.has(name))map.set(name,[]);map.get(name).push(callback);}
    removeEventListener(name,callback){const map=listeners.get(this);map.set(name,(map.get(name)??[]).filter(value=>value!==callback));}
    emit(name,event={}){for(const callback of [...(listeners.get(this).get(name)??[])])callback(event);}
    send(data){this.playwright.emit('framesent',{payload:Buffer.from(data)});}
    close(code=1000){this.readyState=3;this.emit('close',{code,wasClean:code===1000});cdp.emit('Network.webSocketClosed',{requestId:this.requestId});}
  }
  const context=createContext({WebSocket:NativeSocket,location:{href:BASE+'/'},URL,Date,performance,crypto:webcrypto,TextDecoder,TextEncoder,ArrayBuffer,Uint8Array,Blob,JSON,Object,Number,String,Promise,Reflect,Proxy,Error,Map,Set,console:undefined});
  const evaluate=(fn,arg)=>{context.__controlledArgument=arg;try{return runInContext('('+fn.toString()+')(__controlledArgument)',context);}finally{delete context.__controlledArgument;}};
  page.initScripts=[];
  page.addInitScript=async(fn,arg)=>{page.initScripts.push({fn,arg});evaluate(fn,arg);};
  page.evaluate=async(fn,arg)=>structuredClone(await evaluate(fn,arg));
  page.context=()=>({newCDPSession:async selected=>{assert.equal(selected,page);return cdp;}});
  const create=()=>{context.__controlledSocketURL=ORIGIN+ATTACH;try{return runInContext('new WebSocket(__controlledSocketURL)',context);}finally{delete context.__controlledSocketURL;}};
  const handshake=(socket,status)=>cdp.emit('Network.webSocketHandshakeResponseReceived',{requestId:socket.requestId,response:{status,headers:status===101?{Upgrade:'websocket',Connection:'Upgrade'}:{}}});
  const open=socket=>{handshake(socket,101);socket.readyState=1;socket.emit('open');};
  const message=(socket,kind,text)=>{const bytes=Buffer.concat([Buffer.from([kind]),Buffer.from(text)]);socket.emit('message',{data:bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)});};
  const errors=(socket,text='net::ERR_CONNECTION_CLOSED controlled fixture')=>{
    socket.playwright.emit('socketerror',text);
    cdp.emit('Network.webSocketFrameError',{requestId:socket.requestId,errorMessage:text});
    socket.emit('error');
  };
  const refuse=socket=>{handshake(socket,409);errors(socket);socket.close(1006);};
  return {page,cdp,context,sockets,evaluate,create,open,message,errors,refuse};
}

export async function controlledMonitor(scenario,launch){
  const fake=fakePage(),privateErrors=privateCapture(launch);
  try{
    const monitor=await scenario.installTerminalMonitor(fake.page,{base:BASE,attachPath:ATTACH,markers:MARKERS,retainTransportError:row=>privateErrors.capture.record(row)});
    assert.equal(fake.page.initScripts[0].fn,scenario.terminalMonitor,'Exercise the exported native observer through its actual installer');
    const snapshot=async()=>{
      for(let i=0;i<1000;i++){
        const value=await monitor.snapshot();
        if(value.browser.rows.every((row,index)=>row.pending===0&&(fake.sockets[index]?.readyState!==3||row.closed)))return value;
        await new Promise(resolve=>setTimeout(resolve,1));
      }
      throw Error('Controlled native message observation did not settle');
    };
    return {...fake,monitor,privateErrors,snapshot,async dispose(){try{await monitor.dispose();}finally{privateErrors.dispose();}}};
  }catch(error){privateErrors.dispose();throw error;}
}

export function assertHealthy(snapshot){
  assert.equal(snapshot.observerFailure,null,'The actual installed sent/CDP observer must remain healthy');
  assert.deepEqual(snapshot.protocol.failures,[],'The actual CDP observer must accept bounded refused attempts');
  assert.deepEqual(snapshot.browser.failures,[],'The actual native observer must accept bounded refused attempts');
}

export function hasObservationFailure(snapshot){
  return !!snapshot.observerFailure||snapshot.protocol.failures.length>0||snapshot.browser.failures.length>0;
}
