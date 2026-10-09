// Bounded scripted-fixture receipt transport, not a model/backend completion API.
import assert from 'node:assert/strict';
import * as crypto from 'node:crypto';
import * as zlib from 'node:zlib';

export const mixedReceiptLimits=Object.freeze({decodedBytes:64*1024,wireBytes:4096,toolOutputBytes:1024*1024,calls:48});
export const mixedReceiptPrefix='LUCI_MIXED_OBSERVATION_V1=';

// This exact function is embedded in the generated native task program.
export function encodeMixedObservation(value){
 const bytes=Buffer.from(JSON.stringify(value));assert(bytes.length<=mixedReceiptLimits.decodedBytes,'Complete observation exceeds decoded receipt bound');
 const compressed=zlib.deflateRawSync(bytes,{level:9});
 const wire=mixedReceiptPrefix+crypto.createHash('sha256').update(bytes).digest('hex')+':'+compressed.toString('base64');
 assert(Buffer.byteLength(wire)<=mixedReceiptLimits.wireBytes,'Complete observation exceeds wire receipt bound');
 return wire;
}

export function decodeMixedObservation(output){
 assert.equal(typeof output,'string');assert(Buffer.byteLength(output)<=mixedReceiptLimits.toolOutputBytes,'Tool output exceeds receipt transport bound');
 const occurrences=output.split('LUCI_MIXED_OBSERVATION').length-1;
 if(!occurrences)return null;
 assert.equal(occurrences,1,'Multiple completion receipt markers');
 const lines=output.split(/\r?\n/),matches=lines.filter(line=>line.startsWith(mixedReceiptPrefix));
 assert.equal(matches.length,1,'Malformed or unsupported completion receipt');
 const wire=matches[0];assert(Buffer.byteLength(wire)<=mixedReceiptLimits.wireBytes,'Receipt wire exceeds bound');
 const match=/^LUCI_MIXED_OBSERVATION_V1=([a-f0-9]{64}):([A-Za-z0-9+/]+={0,2})$/.exec(wire);assert(match,'Malformed completion envelope');
 const compressed=Buffer.from(match[2],'base64');assert.equal(compressed.toString('base64'),match[2],'Noncanonical completion encoding');
 const inflated=zlib.inflateRawSync(compressed,{maxOutputLength:mixedReceiptLimits.decodedBytes,info:true});
 assert.equal(inflated.engine.bytesWritten,compressed.length,'Trailing compressed receipt bytes');
 assert.equal(crypto.createHash('sha256').update(inflated.buffer).digest('hex'),match[1],'Completion receipt digest mismatch');
 const actual=JSON.parse(inflated.buffer.toString('utf8'));assert(actual&&typeof actual==='object'&&!Array.isArray(actual),'Completion receipt must be an object');
 return {actual,wire,sha256:match[1],decodedBytes:inflated.buffer.length};
}

function nativeResult(raw){
 // Pinned Codex native exec/poll textual contract, bound to retained fixtures.
 // Loose JSON status shapes are unsupported; body text never supplies status.
 assert.equal(typeof raw,'string');assert(Buffer.byteLength(raw)<=mixedReceiptLimits.toolOutputBytes,'Tool output exceeds receipt transport bound');
 const boundary=raw.indexOf('\nOutput:');assert(boundary>=0,'Native command status header is unavailable');
 const header=raw.slice(0,boundary),body=raw.slice(boundary+'\nOutput:'.length).replace(/^\r?\n/,'');
 const states=[...header.matchAll(/^Process (?:running with session ID (\d+)|exited with code (-?\d+))\r?$/gm)];
 assert.equal(states.length,1,'Exactly one authoritative native command status is required');
 const state=states[0];
 if(state[1]!==undefined){const sessionId=Number(state[1]);assert(Number.isSafeInteger(sessionId)&&sessionId>0,'Invalid native command identity');return {body,sessionId};}
 const exitCode=Number(state[2]);assert(Number.isSafeInteger(exitCode),'Invalid native command exit');return {body,exitCode};
}

// Call identity comes from the driver's exact unconsumed call map. A receipt
// alone is pending until this same command reports an authoritative exit 0.
export function consumeMixedToolResult(transport,raw,{callId,tool,sessionId,task,operation}){
 assert(transport&&typeof transport==='object');assert(!transport.completed,'Task command already completed');
 assert(typeof callId==='string'&&callId.length>0&&callId.length<=200,'Exact current call identity required');
 const consumed=transport.consumedCallIds??[];assert(consumed.length<mixedReceiptLimits.calls,'Receipt call budget exceeded');assert(!consumed.includes(callId),'Task tool call already consumed');
 assert(tool==='exec_command'||tool==='write_stdin','Unexpected task receipt tool');
 if(tool==='exec_command')assert.equal(consumed.length,0,'Task command cannot be re-executed');
 else {assert(Number.isSafeInteger(transport.commandId)&&transport.commandId>0,'No exact live command to poll');assert.equal(sessionId,transport.commandId,'Poll must preserve exact command identity');}
 const result=nativeResult(raw),receipt=decodeMixedObservation(result.body);
 if(receipt){assert.equal(receipt.actual.task,task,'Receipt task differs');assert.equal(receipt.actual.operation,operation,'Receipt operation differs');if(transport.pending)assert.equal(receipt.wire,transport.pending.wire,'Pending completion receipt changed');}
 if(result.sessionId!==undefined){
  if(transport.commandId!==undefined)assert.equal(result.sessionId,transport.commandId,'Native command identity changed');
  else assert.equal(tool,'exec_command','Only initial execution can establish command identity');
  transport.commandId=result.sessionId;
 }else assert.equal(result.exitCode,0,'Native task command did not exit successfully');
 const pending=receipt??transport.pending;
 if(result.exitCode===0)assert(pending,'Successful native exit lacks a complete bound receipt');
 transport.pending=pending;transport.consumedCallIds=[...consumed,callId];
 if(result.exitCode===0){transport.completed=true;return {kind:'complete',actual:pending.actual,receiptSHA256:pending.sha256,wireBytes:Buffer.byteLength(pending.wire),decodedBytes:pending.decodedBytes,commandExitCode:0};}
 return {kind:'running',sessionId:transport.commandId,receiptPending:!!pending};
}
