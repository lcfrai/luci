// CLOUD-054 prospective installed scenario. No startup, allocation or cleanup ownership.
import assert from 'node:assert/strict';
import {randomBytes,createHash} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';

export function fixtureMarkers() {
  const nonce=randomBytes(12).toString('hex');
  return {nonce,ready:`C054_READY_${nonce}`,first:`C054_FIRST_${nonce}`,second:`C054_SECOND_${nonce}`,body:`C054_BODY_EXIT_${nonce}`,blocked:`C054_NO_STDIN_${nonce}`};
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

export async function installTerminalMonitor(page,{base,attachPath,markers}) {
  const target=new URL(base);target.protocol=target.protocol==='https:'?'wss:':'ws:';
  await page.addInitScript(terminalMonitor,{origin:target.origin,attachPath,markers});
  const sent=[];let observerFailure=null;
  const hook=socket=>{
    const url=new URL(socket.url());if(url.origin!==target.origin||url.pathname!==attachPath)return;
    const row={ordinal:sent.length+1,stdinFrames:0,stdinBytes:0,resizeFrames:0,otherFrames:0};sent.push(row);
    if(sent.length>8)observerFailure='sent-socket-limit';
    socket.on('framesent',frame=>{
      if(!Buffer.isBuffer(frame.payload)){observerFailure='non-binary-sent-frame';return;}
      const bytes=frame.payload;if(!bytes.length){observerFailure='empty-sent-frame';return;}
      if(bytes[0]===1){row.stdinFrames++;row.stdinBytes+=bytes.length-1;}
      else if(bytes[0]===2)row.resizeFrames++;else row.otherFrames++;
    });
  };
  page.on('websocket',hook);
  return {snapshot:async()=>({browser:await page.evaluate(()=>globalThis.__luciC054Terminal()),sent:structuredClone(sent),observerFailure}),dispose:()=>page.off('websocket',hook)};
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

export async function runTerminalScenario({page,base,workspace,sessionId,projectId,request,record,health,monitor,markers,evidenceDir}) {
  assert.match(sessionId,/^s-[a-f0-9]{8}$/);assert.match(projectId,/^p-[a-f0-9]+$/);
  const path=workspace+'/v1/sessions/'+sessionId, pane=page.locator('section.pane[data-session-id="'+sessionId+'"]');
  const gates=[];let primaryFailure,step='pane-visible';
  const stepTo=value=>{step=value;record({kind:'scenario-step',step});};
  const until=async(label,fn,ms=15000)=>{const end=Date.now()+ms;while(Date.now()<end){health();const value=await fn();if(value)return value;await delay(50);}throw Error(label+' timed out');};
  const snapshot=async()=>{const value=await monitor.snapshot();assert.equal(value.observerFailure,null);assert.deepEqual(value.browser.failures,[]);return value;};
  const state=async()=>pane.evaluate(element=>({label:element.getAttribute('aria-label'),footer:element.querySelector('.pane-foot')?.textContent??'',overlay:element.querySelector('[data-attach-phase]')?.getAttribute('data-attach-phase')??null,termPresent:!!element.querySelector('.xterm'),restartCount:[...element.querySelectorAll('button')].filter(button=>button.textContent==='Restart').length}));
  const command=async text=>{health();const textarea=pane.locator('textarea.xterm-helper-textarea');await textarea.focus();await page.keyboard.type(text,{delay:0});await page.keyboard.press('Enter');};
  const fresh=async()=>{const row=await request('GET',path);assert.equal(row.id,sessionId);assert.equal(row.projectId,projectId);assert.equal(row.harness,'shell');return row;};
  try{
    await pane.waitFor({state:'visible',timeout:20000});await until('Real SessionPane attached',async()=>{const s=await state();return /attached/.test(s.footer)&&!s.overlay;});
    stepTo('initial-authoritative-running');const before=await fresh();assert.match(before.executionGeneration,/^[a-f0-9]{64}$/);assert.equal(before.status,'running');
    await pane.locator('.xterm').evaluate(element=>{globalThis.__c054OriginalTerminal=element;});
    record({kind:'initial-session',sessionId,projectId,executionGeneration:before.executionGeneration,ui:await state()});
    stepTo('ready-command');await command(`stty -echo; printf 'C054_READY_%s\\n' '${markers.nonce}'`);
    await until('Shell command ready marker',async()=>{const s=await snapshot();return s.browser.rows.length===1&&s.browser.rows[0].markers.ready;});
    // A text body resembling a terminal event must not settle the attachment.
    stepTo('text-exit-lookalike');await command(`printf 'C054_BODY_EXIT_%s {"type":"exit","code":0}\\n' '${markers.nonce}'`);
    await until('Textual exit lookalike printed',async()=>!!(await snapshot()).browser.rows[0].markers.body);
    assert.equal((await snapshot()).browser.rows[0].exit.length,0);assert.match((await state()).footer,/attached/);
    stepTo('first-running-poll-hold');const firstGate=await holdNextSessionList(page,{base,workspace,sessionId,health,record});gates.push(firstGate);
    const held=await Promise.race([firstGate.ready,delay(7000).then(()=>{throw Error('First list gate not reached');})]);assert.equal(held.session.status,'running');assert.equal(held.session.executionGeneration,before.executionGeneration);
    stepTo('first-exit-command');await command(`printf 'C054_FIRST_%s\\n' '${markers.nonce}'; exit 0`);
    stepTo('first-wire-close');await until('First actual typed exit and normal close',async()=>{const row=(await snapshot()).browser.rows[0];return row?.closed&&row.pending===0;},5000);
    stepTo('first-wire-assertions');let observed=await snapshot(),first=observed.browser.rows[0];assert.equal(observed.browser.rows.length,1);assert.equal(first.error,false);assert.equal(first.closed.code,1000);assert.equal(first.closed.wasClean,true);assert.deepEqual(first.exit.map(value=>value.reason),[{kind:'exit-status',code:0}]);assert(first.markers.first.sequence<first.exit[0].sequence);assert(first.exit[0].sequence<first.events.find(event=>event.kind==='close').sequence);
    stepTo('ended-before-poll');await until('Ended footer without next poll',async()=>(await state()).footer.includes('ended'),1000);
    stepTo('ended-stdin-and-reconnect-guard');const sentBefore=observed.sent[0].stdinFrames;
    await command(`printf 'C054_NO_STDIN_%s\\n' '${markers.nonce}'`);
    await delay(2300);health();observed=await snapshot();assert.equal(observed.browser.rows.length,1);assert.equal(observed.sent[0].stdinFrames,sentBefore);assert.equal((await state()).overlay,null);assert.match((await state()).footer,/ended/);assert.match((await state()).label,/ — running$/);
    await page.screenshot({path:evidenceDir+'/first-ended-poll-held.png',fullPage:true});record({kind:'typed-exit-before-poll',ui:await state(),wire:observed});
    stepTo('first-poll-release');await firstGate.release();await firstGate.dispose();await until('Fresh exited state and Restart',async()=>(await state()).restartCount===1,20000);
    stepTo('second-old-poll-hold');const secondGate=await holdNextSessionList(page,{base,workspace,sessionId,health,record});gates.push(secondGate);
    await Promise.race([secondGate.ready,delay(7000).then(()=>{throw Error('Second list gate not reached');})]);
    stepTo('single-ui-restart');const responsePromise=page.waitForResponse(response=>new URL(response.url()).pathname===path+'/restart'&&response.request().method()==='POST',{timeout:150000});
    await pane.getByRole('button',{name:'Restart',exact:true}).click();const restartResponse=await responsePromise;assert.equal(restartResponse.status(),200);const restarted=await restartResponse.json();assert.equal(restarted.id,sessionId);assert.match(restarted.executionGeneration,/^[a-f0-9]{64}$/);assert.notEqual(restarted.executionGeneration,before.executionGeneration);
    stepTo('replacement-socket');await until('Replacement browser socket attached',async()=>{const s=await snapshot();return s.browser.rows.length===2&&s.browser.rows[1].open&&(await state()).footer.includes('attached');},6000);
    stepTo('old-poll-release-after-restart');await secondGate.release();await secondGate.dispose();await delay(250);health();
    assert.equal((await snapshot()).browser.rows.length,2);assert.match((await state()).footer,/attached/);assert.equal((await state()).restartCount,0);
    assert.equal(await pane.locator('.xterm').evaluate(element=>element===globalThis.__c054OriginalTerminal),true,'Terminal DOM retained through same-ID restart');
    const current=await until('Replacement authoritative running row',async()=>{const row=await fresh();assert.equal(row.executionGeneration,restarted.executionGeneration);assert(['starting','running'].includes(row.status));return row.status==='running'?row:null;},15000);assert.equal(current.status,'running');
    record({kind:'restart-after-held-old-read',sessionId,executionGeneration:current.executionGeneration,ui:await state(),wire:await snapshot()});
    stepTo('second-exit-command');await command(`stty -echo; printf 'C054_SECOND_%s\\n' '${markers.nonce}'; exit 7`);
    await until('Second actual typed exit and normal close',async()=>{const row=(await snapshot()).browser.rows[1];return row?.closed&&row.pending===0;},15000);
    stepTo('second-wire-assertions');const final=await snapshot(),second=final.browser.rows[1];assert.equal(final.browser.rows.length,2);assert.equal(second.error,false);assert.equal(second.closed.code,1000);assert.equal(second.closed.wasClean,true);assert.deepEqual(second.exit.map(value=>value.reason),[{kind:'exit-status',code:7}]);assert(second.markers.second.sequence<second.exit[0].sequence);
    await until('Nonzero completion says ended',async()=>(await state()).footer.includes('ended'));await delay(2300);health();assert.equal((await snapshot()).browser.rows.length,2);assert.equal((await state()).overlay,null);
    await page.screenshot({path:evidenceDir+'/second-nonzero-ended.png',fullPage:true});
    record({kind:'scenario-complete',passed:true,ui:await state(),wire:await snapshot(),limitations:['Actual interactive shell transport, not model-provider output quality','No 1013 overflow induced; separately controlled regression applies','Screenshots preserve actual terminal rendering for manual review; wire marker receipt is not OCR','Current scope verifies terminal-ended stdin suppression, not project-permission revocation','A same-ID UI restart is exercised; external-actor restart and late old-socket callbacks remain controlled-source scope','No responsiveness/throughput/whole-computer claim']});
    return {passed:true,sessionId,initialExecutionGeneration:before.executionGeneration,restartedExecutionGeneration:restarted.executionGeneration};
  }catch(error){
    primaryFailure=error;
    // Preserve the primary failure before route disposal can fail, then make a
    // bounded listener-only observation. No command/API request is issued here.
    try{record({kind:'scenario-failure',step,error:scenarioError(error),gates:gates.map(gate=>gate.snapshot())});}catch{}
    try{const observed=await Promise.race([Promise.all([monitor.snapshot(),state()]),delay(2000).then(()=>{throw Error('Passive failure snapshot timed out');})]);assert(Buffer.byteLength(JSON.stringify(observed))<=2*1024**2);record({kind:'scenario-failure-observation',step,wire:observed[0],ui:observed[1],passive:true});}
    catch(snapshotError){try{record({kind:'scenario-failure-observation-unavailable',step,error:scenarioError(snapshotError)});}catch{}}
    throw error;
  }finally{
    let cleanupFailure;
    for(const [index,gate]of gates.entries())try{await gate.dispose();}catch(error){cleanupFailure??=error;try{record({kind:'scenario-gate-cleanup-failure',step,index,error:scenarioError(error),gate:gate.snapshot()});}catch{}}
    if(cleanupFailure&&!primaryFailure)throw cleanupFailure;
  }
}
