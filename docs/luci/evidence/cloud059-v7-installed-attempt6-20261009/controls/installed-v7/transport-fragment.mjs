// Passive diagnostic projections. No URL, query, header value, close reason or
// arbitrary error text is retained; hashes bind only the observed private text.
export function transportErrorEvidence(input) {
  if(typeof input!=='string')return {available:false,kind:'unavailable'};
  const bytes=Buffer.from(input),known=[['ERR_WS_PROTOCOL_ERROR','protocol-error'],['ERR_CONNECTION_RESET','connection-reset'],['ERR_CONNECTION_CLOSED','connection-closed'],['ERR_SOCKET_NOT_CONNECTED','socket-not-connected']];
  return {available:true,bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),kind:known.find(([needle])=>input.includes(needle))?.[1]??'unclassified',rawTextRetained:false};
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
export async function installTransportDiagnostics(page,{origin,attachPath}) {
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
    add('Network.webSocketFrameError',event=>{const row=known.get(event.requestId);if(!row)return;if(row.errors.length>=8){fail('cdp-error-limit');return;}row.errors.push({...at(),...transportErrorEvidence(event.errorMessage)});});
    for(const [name,direction]of [['Network.webSocketFrameReceived','received'],['Network.webSocketFrameSent','sent']])add(name,event=>{const row=known.get(event.requestId);if(!row||event.response?.opcode!==8)return;if(row.closeFrames.length>=8){fail('close-frame-limit');return;}const value=closeFrameEvidence(event.response,direction);if(!value.available)fail('close-frame-projection-unavailable');row.closeFrames.push({...at(),...value});});
    add('Network.webSocketClosed',event=>{const row=known.get(event.requestId);if(row)row.closed=at();});
    await bounded(client.send('Network.enable'),2000,'cdp-enable-deadline');setupComplete=true;
  }catch{fail('transport-setup-unavailable');closed=true;if(client){for(const [name,fn]of listeners)client.off(name,fn);try{await bounded(client.detach(),1500,'cdp-detach-deadline');}catch{fail('cdp-detach-unconfirmed');}}}
  return {snapshot:()=>({setupComplete,rows:structuredClone(rows),failures:[...failures],scope:'Passive exact-path CDP observation only; request identifiers and URLs remain private. No stdout frame capture, direction/causality inference or performance claim.'}),dispose:async()=>{closed=true;if(!client)return;for(const [name,fn]of listeners)client.off(name,fn);try{await bounded(client.detach(),1500,'cdp-detach-deadline');}catch{fail('cdp-detach-unconfirmed');throw Error('Passive transport detach unconfirmed');}}};
}
