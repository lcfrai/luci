import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer, request} from 'node:http';
import {mkdtempSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createMetadataExtractor, startObserver} from './opencode-ollama-observer.mjs';

const token = 'local-observer-test-token-32-characters';
const model = 'luci-acceptance-qwen2.5-general-8k:latest';
const body = (stream = true) => JSON.stringify({model, stream, messages:[{role:'user',content:'PRIVATE PROMPT'}],
  tools:[{type:'function',function:{name:'read',parameters:{type:'object'}}},{type:'function',function:{name:'bash',parameters:{type:'object'}}}],tool_choice:'auto'});
const fixtures = [];

async function fixture(handler, options = {}) {
  const dir = mkdtempSync(join(tmpdir(), 'luci-observer-'));
  const upstream = createServer(handler);
  await new Promise(resolve => upstream.listen(0,'127.0.0.1',resolve));
  const receipt = join(dir, 'metadata.jsonl');
  const observer = await startObserver({listenPort:0, upstreamPort:upstream.address().port, model, token, receiptPath:receipt,...options});
  const f = {dir, receipt, observer, upstream}; fixtures.push(f);
  return f;
}
async function cleanup(f) {
  await f.observer.close();
  await new Promise(resolve => f.upstream.close(resolve));
  rmSync(f.dir,{recursive:true,force:true});
}
function send(port, payload = body(), auth = `Bearer ${token}`) {
  return new Promise((resolve,reject) => {
    const req = request({hostname:'127.0.0.1',port,path:'/v1/chat/completions',method:'POST',
      headers:{authorization:auth,'content-type':'application/json','content-length':Buffer.byteLength(payload)}}, res => {
      const chunks=[]; let ended=false;
      res.on('data',c=>chunks.push(c));
      res.on('end',()=>{ended=true;const rawBody=Buffer.concat(chunks);resolve({status:res.statusCode,body:rawBody.toString(),rawBody});});
      res.on('error',reject);
      res.on('close',()=>{if(!ended)reject(new Error('response_incomplete'));});
    }); req.on('error',reject); req.end(payload);
  });
}
function health(port, auth) {
  return new Promise((resolve,reject)=>{
    const req=request({hostname:'127.0.0.1',port,path:'/health',method:'GET',headers:{authorization:auth}},res=>{
      res.resume(); res.on('end',()=>resolve(res.statusCode));
    });req.on('error',reject);req.end();
  });
}
function rows(f) { return readFileSync(f.receipt,'utf8').trim().split('\n').filter(Boolean).map(JSON.parse); }

test('streaming tool delta is forwarded exactly and only metadata persists', async () => {
  const raw = ['data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"read","arguments":"PRIVATE ARG"}}]},"finish_reason":null}]}\n\n',
    'data: {"choices":[{"delta":{"content":"PRIVATE ANSWER"},"finish_reason":"stop"}]}\n\n','data: [DONE]\n\n'].join('');
  const f = await fixture((req,res) => {
    assert.equal(req.headers.authorization,undefined);
    res.writeHead(200,{'content-type':'text/event-stream'});
    for (const chunk of [raw.slice(0,29),raw.slice(29,97),raw.slice(97)]) res.write(chunk);
    res.end();
  });
  try {
    const result = await send(f.observer.port);
    assert.equal(result.status,200); assert.equal(result.body,raw);
    const [r] = rows(f); assert.equal(r.nativeToolCallCount,1); assert.deepEqual(r.nativeToolNames,['read']);
    assert.equal(r.textPresent,true); assert.equal(r.textBytes,Buffer.byteLength('PRIVATE ANSWER'));
    assert.equal(r.finishReason,'stop'); assert.equal(r.streamDone,true); assert.equal(r.outcome,'response_flushed');
    assert(!readFileSync(f.receipt,'utf8').includes('PRIVATE'));
  } finally { await cleanup(f); }
});

test('nonstreaming text-shaped command stays text with zero native calls', async () => {
  const answer = JSON.stringify({choices:[{message:{role:'assistant',content:'```json PRIVATE bash-shaped text```'},finish_reason:'stop'}]});
  const f = await fixture((_req,res) => {res.writeHead(200,{'content-type':'application/json'}); res.end(answer);});
  try {
    const result=await send(f.observer.port,body(false)); assert.equal(result.body,answer);
    const [r]=rows(f); assert.equal(r.nativeToolCallCount,0); assert.equal(r.textPresent,true);
    assert.equal(r.textBytes,Buffer.byteLength('```json PRIVATE bash-shaped text```'));
    assert(!readFileSync(f.receipt,'utf8').includes('PRIVATE'));
  } finally { await cleanup(f); }
});

test('arbitrary UTF-8 byte splits preserve stream and nonstream bytes and text counts', async () => {
  for (const stream of [true,false]) {
    const answer=stream
      ? 'data: '+JSON.stringify({choices:[{index:0,delta:{content:'🍎 café'},finish_reason:'stop'}]})+'\n\ndata: [DONE]\n\n'
      : JSON.stringify({choices:[{index:0,message:{role:'assistant',content:'🍎 café'},finish_reason:'stop'}]});
    const expected=Buffer.from(answer);
    const f=await fixture((_req,res)=>{
      res.writeHead(200,{'content-type':stream?'text/event-stream':'application/json'});
      for (const byte of expected) res.write(Buffer.from([byte]));
      res.end();
    });
    try {
      const result=await send(f.observer.port,body(stream));
      assert.deepEqual(result.rawBody,expected);
      const [r]=rows(f); assert.equal(r.textBytes,Buffer.byteLength('🍎 café'));
      assert.equal(r.metadataIncomplete,false);
      assert(!readFileSync(f.receipt,'utf8').includes('🍎 café'));
    } finally { await cleanup(f); }
  }
});

test('fragmented tool names are assembled per choice and tool index without arguments', async () => {
  const requestBody=JSON.parse(body());
  requestBody.tools.push({type:'function',function:{name:'edit',parameters:{type:'object'}}});
  const events=[
    {choices:[{index:0,delta:{tool_calls:[{index:0,function:{name:'ba',arguments:'PRIVATE ARG'}},{index:1,function:{name:'ed'}}]},finish_reason:null},
      {index:1,delta:{tool_calls:[{index:0,function:{name:'re'}}]},finish_reason:null}]},
    {choices:[{index:0,delta:{tool_calls:[{index:0,function:{name:'sh'}},{index:1,function:{name:'it'}}]},finish_reason:'tool_calls'},
      {index:1,delta:{tool_calls:[{index:0,function:{name:'ad'}}]},finish_reason:'tool_calls'}]},
  ];
  const raw=events.map(e=>'data: '+JSON.stringify(e)+'\n\n').join('')+'data: [DONE]\n\n';
  const f=await fixture((_req,res)=>{res.writeHead(200,{'content-type':'text/event-stream'});res.end(raw);});
  try {
    assert.equal((await send(f.observer.port,JSON.stringify(requestBody))).body,raw);
    const [r]=rows(f); assert.equal(r.nativeToolCallCount,3);
    assert.deepEqual(r.nativeToolNames,['bash','edit','read']); assert.equal(r.metadataIncomplete,false);
    assert(!readFileSync(f.receipt,'utf8').includes('PRIVATE ARG'));
  } finally { await cleanup(f); }
});

test('truncated SSE and oversized metadata line remain explicit without content retention', () => {
  const e=createMetadataExtractor(true,['read']); e.feed(Buffer.from('data: {"choices":[{"delta":{"content":"PRIVATE"'));
  assert.equal(e.finish().metadataIncomplete,true);
  const huge=createMetadataExtractor(true,['read']); huge.feed(Buffer.from('data: '+ 'x'.repeat(65*1024)));
  assert.equal(huge.finish().metadataIncomplete,true);
});

test('unknown finish, missing choices and malformed tool calls stay incomplete and sanitized', () => {
  for (const response of [
    {choices:[{message:{content:'PRIVATE'},finish_reason:'PRIVATE SECRET FINISH'}]},
    {},
    {choices:[{message:{content:null,tool_calls:{private:'PRIVATE ARG'}},finish_reason:'stop'}]},
    {choices:[{message:{},finish_reason:'stop'}]},
  ]) {
    const e=createMetadataExtractor(false,['read']);e.feed(Buffer.from(JSON.stringify(response)));
    const metadata=e.finish(); assert.equal(metadata.metadataIncomplete,true);
    assert(!JSON.stringify(metadata).includes('PRIVATE'));
  }
  const fragmented=createMetadataExtractor(true,['bash']);
  fragmented.feed(Buffer.from('data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"ba"}}]},"finish_reason":"tool_calls"}]}\n\ndata: [DONE]\n\n'));
  const metadata=fragmented.finish();assert.equal(metadata.metadataIncomplete,true);
  assert.deepEqual(metadata.nativeToolNames,['unoffered']);
  for (const stream of [true,false]) {
    const invalid=createMetadataExtractor(stream,['read']);
    invalid.feed(Buffer.from([0xc3,0x28]));
    assert.equal(invalid.finish().metadataIncomplete,true);
  }
});

test('truncated upstream stream is forwarded but receipt does not claim complete metadata', async () => {
  const raw='data: {"choices":[{"delta":{"content":"PRIVATE"},"finish_reason":"stop"}]}\n\n';
  const f=await fixture((_req,res)=>{res.writeHead(200,{'content-type':'text/event-stream'});res.end(raw);});
  try {
    assert.equal((await send(f.observer.port)).body,raw);
    const [r]=rows(f); assert.equal(r.upstreamCompleted,true); assert.equal(r.streamDone,false);
    assert.equal(r.metadataIncomplete,true); assert(!readFileSync(f.receipt,'utf8').includes('PRIVATE'));
  } finally { await cleanup(f); }
});

test('wrong token and wrong model are refused before upstream dispatch', async () => {
  let dispatched=0;
  const f=await fixture((_req,res)=>{dispatched++;res.end('{}');});
  try {
    assert.equal((await send(f.observer.port,body(),'Bearer wrong')).status,401);
    assert.equal(await health(f.observer.port,'Bearer wrong'),401);
    assert.equal(await health(f.observer.port,`Bearer ${token}`),200);
    assert.equal((await send(f.observer.port,body().replace(model,'other-model'))).status,400);
    assert.equal(dispatched,0); assert.deepEqual(rows(f),[]);
  } finally { await cleanup(f); }
});

test('overlapping coding and title calls both pass through with separate bounded receipts', async () => {
  let requests=0;
  const answer=JSON.stringify({choices:[{message:{role:'assistant',content:'PRIVATE'},finish_reason:'stop'}]});
  const f=await fixture((_req,res)=>{
    requests++;
    res.writeHead(200,{'content-type':'application/json'});
    setTimeout(()=>res.end(answer),requests===1?45:5);
  });
  try {
    const [a,b]=await Promise.all([send(f.observer.port,body(false)),send(f.observer.port,body(false))]);
    assert.equal(a.body,answer);assert.equal(b.body,answer);assert.equal(requests,2);
    assert.deepEqual(rows(f).map(r=>r.sequence).sort(),[1,2]);
  } finally { await cleanup(f); }
});

test('client cancellation records an uncertain dispatch and closes upstream', async () => {
  let upstreamClosed;
  const closed=new Promise(resolve=>{upstreamClosed=resolve;});
  const f=await fixture((_req,res)=>{
    res.on('close',()=>{if(!res.writableEnded)upstreamClosed();});
    res.writeHead(200,{'content-type':'text/event-stream'});
    res.write('data: {"choices":[{"delta":{},"finish_reason":null}]}\n\n');
  });
  try {
    await new Promise((resolve,reject)=>{
      const payload=body(); const req=request({hostname:'127.0.0.1',port:f.observer.port,path:'/v1/chat/completions',method:'POST',
        headers:{authorization:`Bearer ${token}`,'content-length':Buffer.byteLength(payload)}},res=>res.once('data',()=>{req.destroy();resolve();}));
      req.once('error',e=>{if(e.code!=='ECONNRESET')reject(e);});req.end(payload);
    });
    await Promise.race([closed,new Promise((_,reject)=>setTimeout(()=>reject(new Error('upstream not cancelled')),1500))]);
    await new Promise(resolve=>setTimeout(resolve,30));
    assert.equal(rows(f)[0].outcome,'client_disconnected_uncertain');
  } finally { await cleanup(f); }
});

test('absolute deadline retains uncertainty despite a responding upstream', async () => {
  const f=await fixture((_req,res)=>{res.writeHead(200,{'content-type':'text/event-stream'});res.write('data: {"choices":[{"delta":{},"finish_reason":null}]}\n\n');}, {maxCallMs:50});
  try {
    await assert.rejects(send(f.observer.port));
    await new Promise(resolve=>setTimeout(resolve,20));
    assert.equal(rows(f)[0].outcome,'deadline_uncertain');
  } finally { await cleanup(f); }
});
