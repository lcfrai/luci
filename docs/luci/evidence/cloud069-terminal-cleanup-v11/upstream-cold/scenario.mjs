// CLOUD-054 prospective installed scenario. No startup, allocation or cleanup ownership.
import assert from 'node:assert/strict';
import {randomBytes,createHash} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';

export function fixtureMarkers() {
  const nonce=randomBytes(12).toString('hex');
  return {nonce,ready:`C054_READY_${nonce}`,first:`C054_FIRST_${nonce}`,second:`C054_SECOND_${nonce}`,body:`C054_BODY_EXIT_${nonce}`,blocked:`C054_NO_STDIN_${nonce}`};
}

// Passive diagnostic projections. No URL, query, header value, close reason or
// arbitrary error text is retained; hashes bind only the observed private text.
export function transportErrorEvidence(input) {
  if(typeof input!=='string')return {available:false,kind:'unavailable'};
  const bytes=Buffer.from(input),known=[['ERR_WS_PROTOCOL_ERROR','protocol-error'],['ERR_CONNECTION_RESET','connection-reset'],['ERR_CONNECTION_CLOSED','connection-closed'],['ERR_SOCKET_NOT_CONNECTED','socket-not-connected']];
  return {available:true,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),kind:known.find(([needle])=>input.includes(needle))?.[1]??'unclassified',rawTextRetained:false,classificationIsTextOnly:true};
}
export function closeFrameEvidence(response,direction) {
  assert(['received','sent'].includes(direction));
  if(response?.opcode!==8)return null;
  const text=response.payloadData;
  if(typeof text!=='string'||text.length>344||text.length%4!==0||!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(text))return {direction,available:false,kind:'invalid-or-over-limit-base64'};
  const bytes=Buffer.from(text,'base64');
  if(bytes.length>256||bytes.toString('base64')!==text)return {direction,available:false,kind:'invalid-or-over-limit-base64'};
  return {direction,available:true,opcode:8,masked:typeof response.mask==='boolean'?response.mask:null,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),statusCode:bytes.length>=2?bytes.readUInt16BE(0):null,invalidOneBytePayload:bytes.length===1,reasonRetained:false};
}
export function handshakeEvidence(response) {
  const headers=response?.headers;
  if(!headers||typeof headers!=='object'||Array.isArray(headers))return {available:false};
  const value=name=>Object.entries(headers).filter(([key])=>key.toLowerCase()===name).map(([,v])=>typeof v==='string'?v:null);
  const upgrade=value('upgrade'),connection=value('connection'),protocol=value('sec-websocket-protocol'),extensions=value('sec-websocket-extensions');
  return {available:true,status:Number.isSafeInteger(response.status)?response.status:null,upgradeWebSocket:upgrade.length===1&&upgrade[0]?.trim().toLowerCase()==='websocket',connectionUpgrade:connection.length===1&&!!connection[0]?.split(',').some(v=>v.trim().toLowerCase()==='upgrade'),protocolPresent:protocol.length>0,protocolEmpty:protocol.length===1&&protocol[0]==='',extensionPresent:extensions.length>0,perMessageDeflate:extensions.length===1&&typeof extensions[0]==='string'&&/^permessage-deflate(?:\s*;\s*(?:server_no_context_takeover|client_no_context_takeover|(?:server_max_window_bits|client_max_window_bits)(?:=(?:[89]|1[0-5]))?))*$/i.test(extensions[0]),headerValuesRetained:false};
}
export async function installTransportDiagnostics(page,{origin,attachPath,retainTransportError}) {
  assert.equal(typeof retainTransportError,'function','Private transport error capture required before traffic');
  let client,closed=false,setupComplete=false;const rows=[],known=new Map(),failures=[],listeners=[];
  const fail=code=>{if(failures.length<16&&!failures.includes(code))failures.push(code);};
  const bounded=async(promise,ms,code)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(code)),ms);})]);}finally{clearTimeout(timer);}};
  const at=()=>({wall:new Date().toISOString(),monotonicMs:performance.now()});
  const add=(name,fn)=>{const guarded=event=>{if(closed)return;try{fn(event);}catch{fail('transport-event-projection-failed');}};client.on(name,guarded);listeners.push([name,guarded]);};
  const acquisition=page.context().newCDPSession(page);
  acquisition.then(value=>{if(closed)void bounded(value.detach(),1500,'late-detach-deadline').catch(()=>fail('late-cdp-detach-unconfirmed'));},()=>{});
  try{
    client=await bounded(acquisition,2000,'cdp-acquisition-deadline');
    add('Network.webSocketCreated',event=>{let url;try{url=new URL(event.url);}catch{return;}if(url.origin!==origin||url.pathname!==attachPath)return;if(known.has(event.requestId)){fail('duplicate-cdp-socket');return;}if(rows.length>=8){fail('cdp-socket-limit');return;}const row={ordinal:rows.length+1,created:at(),handshakes:[],errors:[],closeFrames:[],closed:null};known.set(event.requestId,row);rows.push(row);});
    add('Network.webSocketHandshakeResponseReceived',event=>{const row=known.get(event.requestId);if(!row)return;if(row.handshakes.length>=2){fail('handshake-limit');return;}row.handshakes.push({...at(),...handshakeEvidence(event.response)});});
    add('Network.webSocketFrameError',event=>{const row=known.get(event.requestId);if(!row)return;if(row.errors.length>=8){fail('cdp-error-limit');return;}const retained=retainTransportError({channel:'cdp',ordinal:row.ordinal,error:event.errorMessage});row.errors.push({...at(),...transportErrorEvidence(event.errorMessage),privateError:retained});});
    for(const [name,direction]of [['Network.webSocketFrameReceived','received'],['Network.webSocketFrameSent','sent']])add(name,event=>{const row=known.get(event.requestId);if(!row||event.response?.opcode!==8)return;if(row.closeFrames.length>=8){fail('close-frame-limit');return;}const value=closeFrameEvidence(event.response,direction);if(!value.available)fail('close-frame-projection-unavailable');row.closeFrames.push({...at(),...value});});
    add('Network.webSocketClosed',event=>{const row=known.get(event.requestId);if(row)row.closed=at();});
    await bounded(client.send('Network.enable'),2000,'cdp-enable-deadline');setupComplete=true;
  }catch{fail('transport-setup-unavailable');closed=true;if(client){for(const [name,fn]of listeners)client.off(name,fn);try{await bounded(client.detach(),1500,'cdp-detach-deadline');}catch{fail('cdp-detach-unconfirmed');}}}
  return {snapshot:()=>({setupComplete,rows:structuredClone(rows),failures:[...failures],scope:'Passive exact-path CDP observation only; request identifiers and URLs remain private. No stdout frame capture, direction/causality inference or performance claim.'}),dispose:async()=>{closed=true;if(!client)return;for(const [name,fn]of listeners)client.off(name,fn);try{await bounded(client.detach(),1500,'cdp-detach-deadline');}catch{fail('cdp-detach-unconfirmed');throw Error('Passive transport detach unconfirmed');}}};
}

// Serialized into the actual page before navigation. Listener-only native sockets;
// no server frames, close codes, product fields or socket sends are rewritten.
export function terminalMonitor({origin,attachPath,markers}) {
  if(Object.hasOwn(globalThis,'__luciC054Terminal'))throw Error('Monitor already installed');
  const Native=globalThis.WebSocket, rows=[], failures=[];
  const knownMarkers=Object.entries(markers).filter(([key])=>key!=='nonce');
  const failure=code=>{if(failures.length<16)failures.push(code);};
  const now=()=>({wall:new Date().toISOString(),monotonicMs:performance.now()});
  let sequence=0;
  globalThis.WebSocket=new Proxy(Native,{construct(target,args,newTarget){
    const socket=Reflect.construct(target,args,newTarget);
    let url;try{url=new URL(String(args[0]),location.href);}catch{return socket;}
    if(url.origin!==origin||url.pathname!==attachPath)return socket;
    if(rows.length>=8){failure('socket-limit');return socket;}
    const row={ordinal:rows.length+1,created:now(),open:null,stdoutFrames:0,stdoutBytes:0,receivedFrames:0,otherFrames:0,exit:[],markers:{},closed:null,error:false,pending:0,events:[]};
    rows.push(row);let tail='',chain=Promise.resolve();
    const event=(kind,details={})=>{if(row.events.length>=512){failure('event-limit');return;}row.events.push({sequence:++sequence,kind,...now(),...details});};
    socket.addEventListener('open',()=>{row.open=now();event('open');});
    socket.addEventListener('message',message=>{
      if(row.receivedFrames>=512||row.pending>=32){failure('message-queue-limit');return;}
      row.receivedFrames++;const received=now();row.pending++;
      chain=chain.then(async()=>{
        let bytes;
        if(message.data instanceof ArrayBuffer)bytes=new Uint8Array(message.data);
        else if(message.data instanceof Blob){if(message.data.size>262144)throw Error('frame-limit');bytes=new Uint8Array(await message.data.arrayBuffer());}
        else throw Error('non-binary-frame');
        if(bytes.length===0||bytes.length>262144)throw Error('frame-limit');
        const kind=bytes[0],payload=bytes.subarray(1);
        if(kind===0){
          row.stdoutFrames++;row.stdoutBytes+=payload.length;
          if(row.stdoutBytes>262144)throw Error('stdout-limit');
          tail=(tail+new TextDecoder().decode(payload)).slice(-65536);
          for(const [name,marker]of knownMarkers)if(tail.includes(marker)&&!row.markers[name])row.markers[name]={...received,sequence:sequence+1};
          event('stdout',{bytes:payload.length});
        }else if(kind===3){
          const text=new TextDecoder().decode(payload),match=/^exit status ([0-9]{1,3})$/.exec(text);
          const reason=match?{kind:'exit-status',code:Number(match[1])}:['killed','signal: killed','signal: terminated'].includes(text)?{kind:'terminated'}:{kind:'unclassified'};
          const digest=await crypto.subtle.digest('SHA-256',payload),payloadSHA256=[...new Uint8Array(digest)].map(value=>value.toString(16).padStart(2,'0')).join('');
          const payloadEvidence={bytes:payload.length,sha256:payloadSHA256,knownStaticRepresentation:match||['killed','signal: killed','signal: terminated'].includes(text)?text:null,unclassifiedTextRetained:false};
          row.exit.push({reason,payload:payloadEvidence,...received,sequence:sequence+1});event('typed-exit',{reason,payload:payloadEvidence});
        }else{row.otherFrames++;event('other-frame',{kind,bytes:payload.length});}
      }).catch(error=>failure(['frame-limit','non-binary-frame','stdout-limit'].includes(error.message)?error.message:'frame-observer-failure')).finally(()=>row.pending--);
    });
    socket.addEventListener('error',()=>{row.error=true;event('error');});
    socket.addEventListener('close',closed=>{chain=chain.then(()=>{row.closed={code:closed.code,wasClean:closed.wasClean,...now()};event('close',{code:closed.code,wasClean:closed.wasClean});tail='';});});
    return socket;
  }});
  Object.defineProperty(globalThis,'__luciC054Terminal',{value:()=>JSON.parse(JSON.stringify({version:1,failures,rows})),configurable:false});
}

export async function installTerminalMonitor(page,{base,attachPath,markers,retainTransportError}) {
  const target=new URL(base);target.protocol=target.protocol==='https:'?'wss:':'ws:';
  await page.addInitScript(terminalMonitor,{origin:target.origin,attachPath,markers});
  const transport=await installTransportDiagnostics(page,{origin:target.origin,attachPath,retainTransportError});
  const sent=[];let observerFailure=transport.snapshot().setupComplete?null:'transport-setup-unavailable';
  const hook=socket=>{
    const url=new URL(socket.url());if(url.origin!==target.origin||url.pathname!==attachPath)return;
    const row={ordinal:sent.length+1,stdinFrames:0,stdinBytes:0,resizeFrames:0,otherFrames:0,socketErrors:[],sendEvents:[]};sent.push(row);
    if(sent.length>8)observerFailure='sent-socket-limit';
    socket.on('socketerror',error=>{if(row.socketErrors.length>=8){observerFailure='socket-error-limit';return;}try{const retained=retainTransportError({channel:'playwright',ordinal:row.ordinal,error});row.socketErrors.push({wall:new Date().toISOString(),monotonicMs:performance.now(),...transportErrorEvidence(error),privateError:retained});}catch{observerFailure='private-transport-error-capture-failed';}});
    socket.on('framesent',frame=>{
      if(!Buffer.isBuffer(frame.payload)){observerFailure='non-binary-sent-frame';return;}
      const bytes=frame.payload;if(!bytes.length){observerFailure='empty-sent-frame';return;}
      const kind=bytes[0]===1?'stdin':bytes[0]===2?'resize':'other';
      if(row.sendEvents.length>=256){observerFailure='send-event-limit';return;}row.sendEvents.push({wall:new Date().toISOString(),monotonicMs:performance.now(),kind,bytes:bytes.length-1});
      if(bytes[0]===1){row.stdinFrames++;row.stdinBytes+=bytes.length-1;}
      else if(bytes[0]===2)row.resizeFrames++;else row.otherFrames++;
    });
  };
  page.on('websocket',hook);
  return {snapshot:async()=>{const protocol=transport.snapshot();return {browser:await page.evaluate(()=>globalThis.__luciC054Terminal()),sent:structuredClone(sent),protocol,observerFailure:observerFailure??(protocol.failures.length?'transport-observation-failed':null)};},dispose:async()=>{page.off('websocket',hook);await transport.dispose();}};
}

// One real read-only response held for <=8s, below the existing10s client budget.
// A slow restart may make this race unexercised; that fails this scenario rather
// than changing the application timeout or publishing a manufactured stale row.
export function scenarioError(error) {
  const message=String(error?.message??error),knownName=['Error','AssertionError','TimeoutError','TypeError','SyntaxError'].includes(error?.name)?error.name:'OtherError';
  return {name:knownName,code:error?.code==='ERR_ASSERTION'?'ERR_ASSERTION':null,messageSHA256:createHash('sha256').update(message).digest('hex'),messageBytes:Buffer.byteLength(message),messageRetained:false,kind:/Route is already handled/.test(message)?'route-already-handled':/ordering unexercised|window expired/.test(message)?'ordering-unexercised':/timed out|timeout/i.test(message)?'timeout':'other'};
}

export async function holdNextSessionList(page,{base,workspace,sessionId,health,record}) {
  const origin=new URL(base).origin,path=workspace+'/v1/sessions';
  let captured,resolveCapture,rejectCapture,releaseGate,gateTimer,used=false,disposing=false,disposePromise,resolveCompleted,routeFailure,entry,deadlineExpired=false;
  const ready=new Promise((resolve,reject)=>{resolveCapture=resolve;rejectCapture=reject;});ready.catch(()=>{});
  const completed=new Promise(resolve=>{resolveCompleted=resolve;}),gate=new Promise(resolve=>{releaseGate=resolve;});
  const settlement={attempted:null,acknowledged:false,requestFailed:false};
  const handler=async route=>{
    const req=route.request(),url=new URL(req.url());
    if(used||req.method()!=='GET'||url.origin!==origin||url.pathname!==path||url.search){await route.continue();return;}
    used=true;const started=performance.now(),deadline=started+8000;
    entry={at:new Date().toISOString(),monotonicMs:started,deadlineMonotonicMs:deadline,budgetMs:8000};
    const expire=()=>{deadlineExpired=true;if(captured)captured.expired=true;const error=Error('Absolute read-only gate budget expired: ordering unexercised');routeFailure??=error;rejectCapture(error);releaseGate();};
    const remaining=()=>{const value=deadline-performance.now();if(value<=0||deadlineExpired){expire();throw routeFailure;}return value;};
    const requestFailed=failed=>{if(failed!==req)return;settlement.requestFailed=true;routeFailure??=Error('Intercepted read failed: ordering unexercised');try{record({kind:'poll-request-failed',entry,settlement:{...settlement}});}catch(error){settlement.checkpointFailed=true;routeFailure=error;}finally{rejectCapture(routeFailure);releaseGate();}};
    page.on('requestfailed',requestFailed);
    gateTimer=setTimeout(expire,Math.max(1,deadline-performance.now()));
    try{
      record({kind:'poll-gate-entry',...entry});if(disposing)throw Error('Read-only gate closed before handler entry: ordering unexercised');
      health();const response=await route.fetch({timeout:Math.max(1,Math.min(5000,remaining())),maxRetries:0});remaining();
      assert.equal(response.status(),200);const body=await response.body();remaining();assert(body.length<=2*1024**2);
      const parsed=JSON.parse(body),matches=parsed.sessions.filter(row=>row.id===sessionId);assert.equal(matches.length,1);
      captured={entry,...entry,bodyObservedMonotonicMs:performance.now(),bodySHA256:createHash('sha256').update(body).digest('hex'),bodyBytes:body.length,session:{id:matches[0].id,status:matches[0].status,executionGeneration:matches[0].executionGeneration??null},released:null,expired:false};
      record({kind:'poll-held',...captured,settlement:{...settlement}});remaining();if(routeFailure)throw routeFailure;resolveCapture(captured);
      await gate;remaining();health();if(routeFailure)throw routeFailure;
      // Claim exactly one terminal operation. A rejected fulfill has unknown
      // transport settlement, so it must never be followed by an abort/replay.
      settlement.attempted='fulfill';await route.fulfill({response,body});settlement.acknowledged=true;remaining();
      captured.released=new Date().toISOString();captured.releasedMonotonicMs=performance.now();record({kind:'poll-released',...captured,settlement:{...settlement}});
    }catch(error){
      routeFailure??=error;rejectCapture(routeFailure);record({kind:'poll-gate-error',error:scenarioError(routeFailure),entry,expired:deadlineExpired,settlement:{...settlement}});
      if(settlement.attempted===null&&!settlement.requestFailed){settlement.attempted='abort';try{await Promise.race([route.abort('failed'),delay(2000).then(()=>{throw Error('Read-only abort settlement timed out');})]);settlement.acknowledged=true;}catch(abortError){record({kind:'poll-abort-unconfirmed',error:scenarioError(abortError),entry,settlement:{...settlement}});}}
    }finally{clearTimeout(gateTimer);page.off('requestfailed',requestFailed);resolveCompleted();}
  };
  const pattern=url=>url.origin===origin&&url.pathname===path;await page.route(pattern,handler);
  const settled=async()=>{if(used)await Promise.race([completed,delay(7000).then(()=>{throw Error('Read-only gate settlement timed out');})]);if(routeFailure)throw routeFailure;};
  return {ready,release:async()=>{releaseGate();await settled();if(captured)assert.equal(captured.expired,false,'Stale-read window expired: ordering unexercised');},dispose:()=>{
    if(disposePromise)return disposePromise;disposing=true;
    disposePromise=(async()=>{releaseGate();const ownerBeforeRemoval=used;let primary;try{await settled();}catch(error){primary=error;}
      // Remove interception only after the owned handler has settled (or its
      // existing settlement bound failed). Removal is not proof of settlement.
      try{await Promise.race([page.unroute(pattern,handler),delay(2000).then(()=>{throw Error('Read-only route removal timed out');})]);}catch(error){record({kind:'poll-unroute-failed',error:scenarioError(error),entry});primary??=error;}
      if(!ownerBeforeRemoval&&used)try{await settled();}catch(error){primary??=error;}
      if(primary)throw primary;
    })();return disposePromise;
  },snapshot:()=>({...captured??{entry,expired:deadlineExpired},settlement:{...settlement}})};
}

// CLOUD-065 sibling scenario. Response-stage pausing never reissues the POST:
// the unchanged context route guard owns admission and the single forward.
export async function holdNextRestartResponse(page,{base,workspace,sessionId,projectId,health,record}) {
  const expected=new URL(workspace+'/v1/sessions/'+sessionId+'/restart',base).href;
  let client,closed=false,used=false,handlerComplete=false,releaseGate,resolveReady,rejectReady,resolveDone,failure,timer,disposePromise;
  const ready=new Promise((resolve,reject)=>{resolveReady=resolve;rejectReady=reject;});ready.catch(()=>{});
  const gate=new Promise(resolve=>{releaseGate=resolve;}),done=new Promise(resolve=>{resolveDone=resolve;});
  const observation={kind:'restart-response-gate',mode:'real-CDP-response-stage',budgetMs:8000,entered:null,body:null,settlement:{attempted:null,acknowledged:false},disposed:false};
  const bounded=async(promise,ms,label)=>{let timeout;try{return await Promise.race([promise,new Promise((_,reject)=>{timeout=setTimeout(()=>reject(Error(label)),ms);})]);}finally{clearTimeout(timeout);}};
  const fail=error=>{failure??=error;rejectReady(failure);releaseGate();};
  const publish=kind=>record({kind,observation:structuredClone(observation),failure:failure?scenarioError(failure):null});
  const acquisition=page.context().newCDPSession(page);
  acquisition.then(value=>{if(closed)void bounded(value.detach(),1500,'Late response-gate detach timed out').catch(error=>{fail(error);try{publish('restart-response-late-detach-unconfirmed');}catch{}});},()=>{});
  const paused=event=>{
    if(closed)return;
    if(used){fail(Error('More than one Restart response paused'));return;}
    used=true;
    void(async()=>{
      const started=performance.now(),deadline=started+8000;
      observation.entered={wall:new Date().toISOString(),monotonicMs:started,deadlineMonotonicMs:deadline};
      const left=()=>{const remaining=deadline-performance.now();if(remaining<=0)throw Error('Restart response gate window expired: ordering unexercised');return remaining;};
      timer=setTimeout(()=>fail(Error('Restart response gate window expired: ordering unexercised')),8000);
      try{
        publish('restart-response-gate-entered');health();
        assert.equal(event.request?.url,expected);assert.equal(event.request?.method,'POST');
        assert.equal(event.responseStatusCode,200);assert.equal(event.responseErrorReason,undefined);
        const response=await bounded(client.send('Fetch.getResponseBody',{requestId:event.requestId}),Math.min(2000,left()),'Paused Restart response body timed out');
        assert.equal(typeof response.body,'string');assert(response.body.length<=3*1024**2);
        const bytes=Buffer.from(response.body,response.base64Encoded?'base64':'utf8');assert(bytes.length<=2*1024**2);
        if(response.base64Encoded)assert.equal(bytes.toString('base64'),response.body,'Noncanonical response-body encoding');
        const session=JSON.parse(bytes);assert.equal(session.id,sessionId);assert.equal(session.projectId,projectId);assert.equal(session.harness,'shell');
        assert(['starting','running'].includes(session.status));assert.match(session.executionGeneration,/^[a-f0-9]{64}$/);
        observation.body={status:200,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),sessionJSONSHA256:createHash('sha256').update(JSON.stringify(session)).digest('hex'),session:{id:session.id,projectId:session.projectId,status:session.status,executionGeneration:session.executionGeneration}};
        left();health();publish('restart-response-authentic-body-held');resolveReady(structuredClone(observation.body));
        await gate;left();health();if(failure)throw failure;
      }catch(error){fail(error);try{publish('restart-response-gate-failure');}catch(recordError){fail(recordError);}}
      finally{
        // Claim once before transport settlement. No retry/abort, body rewrite,
        // alternate forward, forced success or allocation cleanup occurs here.
        if(observation.settlement.attempted===null){
          observation.settlement.attempted='Fetch.continueResponse';
          try{await bounded(client.send('Fetch.continueResponse',{requestId:event.requestId}),2000,'Restart response continuation unconfirmed');observation.settlement.acknowledged=true;}
          catch(error){fail(error);}
        }
        clearTimeout(timer);handlerComplete=true;resolveDone();try{publish('restart-response-gate-settled');}catch(error){fail(error);}
      }
    })();
  };
  try{
    client=await bounded(acquisition,2000,'Response-gate CDP acquisition timed out');health();
    client.on('Fetch.requestPaused',paused);
    await bounded(client.send('Fetch.enable',{patterns:[{urlPattern:expected,requestStage:'Response'}]}),2000,'Response-gate interception setup timed out');health();
  }catch(error){fail(error);closed=true;if(client){client.off('Fetch.requestPaused',paused);try{await bounded(client.detach(),1500,'Response-gate setup detach timed out');}catch(detachError){fail(detachError);}}throw error;}
  const settle=async()=>{if(used&&!handlerComplete)await bounded(done,4500,'Restart response-gate handler settlement timed out');if(failure)throw failure;};
  return {ready,assertHealthy:()=>{if(failure)throw failure;},snapshot:()=>structuredClone(observation),release:async()=>{releaseGate();await settle();},dispose:()=>{
    if(disposePromise)return disposePromise;
    disposePromise=(async()=>{
      releaseGate();let primary;
      try{await settle();}catch(error){primary=error;}
      // Disabling/detaching settles interception, not the remote mutation. The
      // unchanged launcher retains its own pending-mutation proof independently.
      closed=true;client.off('Fetch.requestPaused',paused);
      try{await bounded(client.send('Fetch.disable'),1500,'Response-gate disable unconfirmed');}catch(error){primary??=error;}
      try{await bounded(client.detach(),1500,'Response-gate detach unconfirmed');}catch(error){primary??=error;}
      observation.disposed=true;publish('restart-response-gate-disposed');if(primary)throw primary;
    })();return disposePromise;
  }};
}

export async function runTerminalScenario({page,base,workspace,sessionId,projectId,request,record,health,monitor,markers,evidenceDir,reopenPane,observeSettlement}) {
  assert.equal(typeof reopenPane,'function');assert.equal(typeof observeSettlement,'function');
  assert.match(sessionId,/^s-[a-f0-9]{8}$/);assert.match(projectId,/^p-[a-f0-9]+$/);
  const path=workspace+'/v1/sessions/'+sessionId, listPath=workspace+'/v1/sessions';
  const pane=page.locator('section.pane[data-session-id="'+sessionId+'"]');
  let step='initial-pane',gate,primaryFailure;
  const stepTo=value=>{step=value;record({kind:'cold-restart-step',step});};
  const active=()=>{health();gate?.assertHealthy();};
  const bounded=async(promise,ms,label)=>{let timeout;try{return await Promise.race([promise,new Promise((_,reject)=>{timeout=setTimeout(()=>reject(Error(label)),ms);})]);}finally{clearTimeout(timeout);}};
  const until=async(label,fn,ms=15000,interval=75)=>{const end=Date.now()+ms;while(Date.now()<end){active();const value=await fn();if(value)return value;await delay(interval);}throw Error(label+' timed out');};
  const snapshot=async()=>{const value=await monitor.snapshot();assert.equal(value.observerFailure,null);assert.deepEqual(value.browser.failures,[]);return value;};
  const state=async()=>pane.evaluate(element=>({label:element.getAttribute('aria-label'),footer:element.querySelector('.pane-foot')?.textContent??'',overlay:element.querySelector('[data-attach-phase]')?.getAttribute('data-attach-phase')??null,restartCount:[...element.querySelectorAll('button')].filter(button=>button.textContent==='Restart').length}));
  const command=async text=>{active();await pane.locator('textarea.xterm-helper-textarea').focus();await page.keyboard.type(text,{delay:0});await page.keyboard.press('Enter');};
  const fresh=async()=>{const row=await request('GET',path);assert.equal(row.id,sessionId);assert.equal(row.projectId,projectId);assert.equal(row.harness,'shell');return row;};
  const nextBrowserList=async()=>{
    const response=await page.waitForResponse(response=>{const url=new URL(response.url());return url.origin===new URL(base).origin&&url.pathname===listPath&&!url.search&&response.request().method()==='GET'&&response.status()===200;},{timeout:4500});
    const bytes=await bounded(response.body(),2000,'Actual list response body timed out');assert(bytes.length<=2*1024**2);const parsed=JSON.parse(bytes),rows=parsed.sessions.filter(row=>row.id===sessionId);assert.equal(rows.length,1);
    const row=rows[0];assert.equal(row.projectId,projectId);return {session:{id:row.id,projectId:row.projectId,status:row.status,executionGeneration:row.executionGeneration??null},sessionJSONSHA256:createHash('sha256').update(JSON.stringify(row)).digest('hex'),bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex')};
  };
  try{
    await pane.waitFor({state:'visible',timeout:20000});await until('Initial actual terminal attached',async()=>(await state()).footer.includes('attached'));
    const initial=await fresh();assert.equal(initial.status,'running');assert.match(initial.executionGeneration,/^[a-f0-9]{64}$/);
    stepTo('initial-natural-exit');await command(`stty -echo; printf 'C054_FIRST_%s\\n' '${markers.nonce}'; exit 0`);
    const first=await until('Initial typed Exit and normal close',async()=>{const value=await snapshot(),row=value.browser.rows[0];return row?.closed&&row.pending===0?value:null;});
    assert.equal(first.browser.rows.length,1);assert.equal(first.browser.rows[0].error,false);assert.equal(first.browser.rows[0].closed.code,1000);assert.equal(first.browser.rows[0].closed.wasClean,true);
    assert.deepEqual(first.browser.rows[0].exit.map(row=>row.reason),[{kind:'exit-status',code:0}]);assert(first.browser.rows[0].markers.first.sequence<first.browser.rows[0].exit[0].sequence);
    record({kind:'cold-restart-initial-exit',wire:first});
    stepTo('authoritative-cleaned-record');
    const cleaned=await until('Actual exited record, erased fingerprint and confirmed product release',async()=>{
      const row=await fresh();if(row.status!=='exited'||Object.hasOwn(row,'executionGeneration'))return null;
      const proof=observeSettlement();if(proof.containers.length||proof.ledger.leases.length||proof.ledger.checks.length||proof.ledger.fences.length||(proof.ledger.checkFences?.length??0))return null;
      return {session:{id:row.id,projectId:row.projectId,status:row.status,executionGenerationOmitted:true},proof};
    },35000,350);
    record({kind:'cold-restart-product-cleaned-before-reopen',...cleaned});
    stepTo('full-browser-document-reopen');await reopenPane();active();await pane.waitFor({state:'visible',timeout:20000});
    await until('Cold exited pane and Restart control',async()=>{const value=await state();return value.restartCount===1&&value.footer.includes('ended');});
    const cold=await snapshot();assert.equal(cold.browser.rows.length,0,'Fresh document must have no retained socket baseline');assert.equal(cold.sent.length,1,'No new socket on cold exited mount');
    await pane.locator('.xterm').evaluate(element=>{globalThis.__c065ColdTerminal=element;});
    for(let i=0;i<2;i++){
      const observed=await nextBrowserList();assert.equal(observed.session.status,'exited');assert.equal(observed.session.executionGeneration,null);
      const wire=await snapshot();assert.equal(wire.browser.rows.length,0);assert.equal(wire.sent.length,1);assert.match((await state()).footer,/ended/);
      record({kind:'cold-restart-before-request-list',ordinal:i+1,response:observed,ui:await state()});
    }
    await page.screenshot({path:evidenceDir+'/cold-exited-before-restart.png',fullPage:true});
    stepTo('single-acknowledgement-gated-ui-restart');
    gate=await holdNextRestartResponse(page,{base,workspace,sessionId,projectId,health,record});
    // responsePromise does not itself certify body consumption. The gate's actual
    // body and continuation acknowledgement are recorded independently.
    const responsePromise=page.waitForResponse(response=>new URL(response.url()).pathname===path+'/restart'&&response.request().method()==='POST',{timeout:150000});
    responsePromise.catch(()=>{});
    await pane.getByRole('button',{name:'Restart',exact:true}).click();
    const held=await bounded(gate.ready,150000,'Actual Restart response was not paused');active();
    assert.notEqual(held.session.executionGeneration,initial.executionGeneration);
    stepTo('real-new-allocation-poll-before-acknowledgement');
    const liveList=await nextBrowserList();active();assert(['starting','running'].includes(liveList.session.status));assert.equal(liveList.session.executionGeneration,held.session.executionGeneration);
    await until('Pending Restart poll was rendered',async()=>/ — (starting|running)$/.test((await state()).label),1500);
    const beforeAck=await snapshot();assert.equal(beforeAck.browser.rows.length,0);assert.equal(beforeAck.sent.length,1);assert.match((await state()).footer,/ended/);
    record({kind:'cold-restart-real-live-poll-without-acknowledgement',response:liveList,identicalParsedSessionJSON:liveList.sessionJSONSHA256===held.sessionJSONSHA256,ui:await state(),wire:beforeAck,gate:gate.snapshot()});
    stepTo('release-exact-acknowledgement');await gate.release();await gate.dispose();
    const restartedResponse=await responsePromise;assert.equal(restartedResponse.status(),200);const restarted=await restartedResponse.json();assert.equal(restarted.id,sessionId);assert.equal(restarted.projectId,projectId);assert.equal(restarted.executionGeneration,held.session.executionGeneration);
    stepTo('first-cold-entry-attachment');
    await until('One acknowledged cold restart attachment',async()=>{const value=await snapshot();return value.browser.rows.length===1&&value.browser.rows[0].open&&(await state()).footer.includes('attached');},6000);
    const attached=await snapshot();assert.equal(attached.browser.rows.length,1);assert.equal(attached.sent.length,2);assert.equal(attached.protocol.rows.length,2);assert.equal((await state()).restartCount,0);
    assert.equal(await pane.locator('.xterm').evaluate(element=>element===globalThis.__c065ColdTerminal),true,'Cold pane terminal retained through same-ID restart');
    const current=await until('Restarted exact generation running',async()=>{const row=await fresh();assert.equal(row.executionGeneration,restarted.executionGeneration);return row.status==='running'?row:null;},15000,350);
    record({kind:'cold-restart-acknowledged-attached',sessionId,executionGeneration:current.executionGeneration,wire:attached,ui:await state()});
    stepTo('restarted-natural-nonzero-exit');await command(`stty -echo; printf 'C054_SECOND_%s\\n' '${markers.nonce}'; exit 7`);
    const final=await until('Restarted output, typed Exit and normal close',async()=>{const value=await snapshot(),row=value.browser.rows[0];return row?.closed&&row.pending===0?value:null;});
    const ended=final.browser.rows[0];assert.equal(final.browser.rows.length,1);assert.equal(final.sent.length,2);assert.equal(ended.error,false);assert.equal(ended.closed.code,1000);assert.equal(ended.closed.wasClean,true);assert.deepEqual(ended.exit.map(row=>row.reason),[{kind:'exit-status',code:7}]);assert(ended.markers.second.sequence<ended.exit[0].sequence);assert(ended.exit[0].sequence<ended.events.find(row=>row.kind==='close').sequence);
    await until('Restarted terminal says ended',async()=>(await state()).footer.includes('ended'));
    await delay(2300);active();assert.equal((await snapshot()).browser.rows.length,1);assert.equal((await snapshot()).sent.length,2);assert.equal((await state()).overlay,null);
    await page.screenshot({path:evidenceDir+'/cold-restarted-nonzero-ended.png',fullPage:true});
    record({kind:'cold-restart-scenario-complete',passed:true,ui:await state(),wire:await snapshot(),limitations:['Real local shell only; no model/provider calls or output-quality claim','Browser document reload resets the attachment registry; this is not a native app/service crash scenario','Real pre-request polls are exited; the live poll is the newly created allocation while its authentic Restart response is withheld','CDP response-stage pacing is controlled network timing, not latency/performance acceptance','Unknown response transport settlement fails acceptance; the unchanged launcher owns exact allocation/ledger cleanup','No fabricated list/session/socket frame or mutation replay; actual terminal frame observation remains passive']});
    return {passed:true,sessionId,initialExecutionGeneration:initial.executionGeneration,restartedExecutionGeneration:restarted.executionGeneration,coldExitedFingerprintOmitted:true,realLivePollBeforeAcknowledgement:true};
  }catch(error){
    primaryFailure=error;try{record({kind:'cold-restart-scenario-failure',step,error:scenarioError(error),gate:gate?.snapshot()??null});}catch{}
    try{const observation=await Promise.race([Promise.all([snapshot(),state()]),delay(2000).then(()=>{throw Error('Passive failure snapshot timed out');})]);record({kind:'cold-restart-failure-observation',step,wire:observation[0],ui:observation[1],passive:true});}catch{}
    throw error;
  }finally{if(gate)try{await gate.dispose();}catch(error){try{record({kind:'cold-restart-gate-cleanup-failure',step,error:scenarioError(error),gate:gate.snapshot()});}catch{}if(!primaryFailure)throw error;}}
}
