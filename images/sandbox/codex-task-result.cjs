'use strict';

// Structured final output only. Neither the candidate nor this same-UID receipt
// attests to checks or process retirement; the controller observes those itself.
const fs = require('node:fs');
const crypto = require('node:crypto');
const { spawn } = require('node:child_process');
const { TextDecoder } = require('node:util');
const { Transform } = require('node:stream');
const { finished } = require('node:stream/promises');
const CONTRACT = '/home/dev/.luci-task-terminal/contract.json';
const SCHEMA = '/usr/local/lib/luci/task-handoff.schema.json';
const OUTPUT = '/home/dev/.luci-task-output';
const CANDIDATE = OUTPUT + '/.codex-terminal/result.json';
const FAILURE = 78;
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
function commandDigest(values) { const h = crypto.createHash('sha256'); for (const value of values) { const b = Buffer.from(value); const n = Buffer.alloc(8); n.writeBigUInt64BE(BigInt(b.length)); h.update(n); h.update(b); } return h.digest('hex'); }

// JSON.parse alone silently accepts duplicate keys. This bounded grammar walk
// rejects duplicates at every depth before the canonical product decoder runs.
function strictJSON(bytes, limit = 65536) {
  if (!Number.isInteger(limit) || limit < 1 || limit > 1048576 || !Buffer.isBuffer(bytes) || !bytes.length || bytes.length > limit) throw Error('size');
  const s = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  let i = 0;
  const ws = () => { while (/\s/.test(s[i] || '') && i < s.length) { if (!' \t\n\r'.includes(s[i])) throw Error('space'); i++; } };
  const str = () => {
    const start = i++;
    while (i < s.length) { const c = s[i++]; if (c === '"') return JSON.parse(s.slice(start, i)); if (c === '\\') i++; }
    throw Error('string');
  };
  const value = depth => {
    if (depth > 16) throw Error('depth'); ws();
    if (s[i] === '"') return str();
    if (s[i] === '{') {
      i++; ws(); const result = Object.create(null), keys = new Set();
      if (s[i] === '}') { i++; return result; }
      for (;;) { ws(); if (s[i] !== '"') throw Error('key'); const key = str(); if (keys.has(key)) throw Error('duplicate'); keys.add(key);
        ws(); if (s[i++] !== ':') throw Error('colon'); result[key] = value(depth + 1); ws(); const end = s[i++]; if (end === '}') return result; if (end !== ',') throw Error('object'); }
    }
    if (s[i] === '[') {
      i++; ws(); const result = []; if (s[i] === ']') { i++; return result; }
      for (;;) { result.push(value(depth + 1)); ws(); const end = s[i++]; if (end === ']') return result; if (end !== ',') throw Error('array'); }
    }
    const m = /^(?:true|false|null|-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)/.exec(s.slice(i));
    if (!m) throw Error('value'); i += m[0].length; return JSON.parse(m[0]);
  };
  const result = value(0); ws(); if (i !== s.length) throw Error('trailing'); return result;
}

function text(v, limit, required = false) {
  return typeof v === 'string' && !v.includes('\0') && Buffer.byteLength(v) <= limit && (!required || v.trim() !== '') && !/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/u.test(v);
}
function keys(v, names) { return v && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).sort().join('|') === [...names].sort().join('|'); }
function reportValid(r) {
  const fields = ['version','summary','changedFiles','decisions','reportedChecks','questions','limitations','addressedMessages'];
  if (!keys(r, fields) || r.version !== 1 || !text(r.summary,4000,true)) return false;
  const arrays = [['changedFiles',128],['decisions',8],['reportedChecks',16],['questions',8],['limitations',16],['addressedMessages',8]];
  if (arrays.some(([k,n]) => !Array.isArray(r[k]) || r[k].length > n)) return false;
  if (!r.changedFiles.every(p => text(p,512,true) && !p.startsWith('/') && !p.includes('\\') && !p.includes(':') && p.split('/').every(s => s && s !== '.' && s !== '..' && !['.git','.luci'].includes(s.toLowerCase())))) return false;
  if (!r.decisions.every(v => keys(v,['decision','rationale']) && text(v.decision,2000,true) && text(v.rationale,2000))) return false;
  if (!r.reportedChecks.every(v => keys(v,['command','outcome','detail']) && text(v.command,2000,true) && ['passed','failed','not-run','unknown'].includes(v.outcome) && text(v.detail,2000))) return false;
  if (!r.questions.every(v => keys(v,['question','impact','blocking']) && text(v.question,2000,true) && text(v.impact,2000) && typeof v.blocking === 'boolean')) return false;
  if (!r.limitations.every(v => text(v,2000,true))) return false;
  return r.addressedMessages.every(v => keys(v,['kind','recipientTaskId','causalMessageId','summary']) &&
    ['discovery','question','dependency-result','review-finding','decision','lifecycle-request'].includes(v.kind) &&
    typeof v.recipientTaskId === 'string' && /^[a-zA-Z0-9_-]{1,80}$/.test(v.recipientTaskId) &&
    typeof v.causalMessageId === 'string' && (v.causalMessageId === '' || /^[A-Za-z0-9_-]{1,128}$/.test(v.causalMessageId)) && text(v.summary,1000,true));
}

function readOne(path, limit, io = fs) {
  const before = io.lstatSync(path); if (!before.isFile() || before.nlink !== 1 || before.size > limit) throw Error('file');
  const fd = io.openSync(path, io.constants.O_RDONLY | io.constants.O_NOFOLLOW);
  try {
    const first = io.fstatSync(fd);
    if (!first.isFile() || first.nlink !== 1 || first.dev !== before.dev || first.ino !== before.ino || first.size !== before.size) throw Error('identity');
    const bytes = Buffer.alloc(first.size); let n = 0;
    while (n < bytes.length) { const got = io.readSync(fd,bytes,n,bytes.length-n,n); if (!got) throw Error('short'); n += got; }
    const end = io.fstatSync(fd), named = io.lstatSync(path);
    for (const v of [end,named]) if (v.dev !== first.dev || v.ino !== first.ino || v.size !== first.size || v.nlink !== 1 || v.mtimeMs !== first.mtimeMs || v.ctimeMs !== first.ctimeMs) throw Error('changed');
    return bytes;
  } finally { io.closeSync(fd); }
}
function exclusive(path, bytes, io = fs) {
  const fd = io.openSync(path, 'wx', 0o600);
  try { io.writeFileSync(fd, bytes); io.fsyncSync(fd); } finally { io.closeSync(fd); }
}
function invocation(command, args, schema) {
  if (command === '/usr/local/bin/claude' || command === '/usr/local/bin/opencode') return otherInvocation(command,args,schema);
  if (!['/usr/local/bin/codex','/usr/local/bin/node'].includes(command) || !Array.isArray(args) || args.some(v => typeof v !== 'string' || v.includes('\0') || Buffer.byteLength(v) > 512*1024)) throw Error('argv');
  if (command === '/usr/local/bin/node' && args[0] !== '/usr/local/lib/luci/codex-native-task.cjs') throw Error('helper');
  // Never inspect an opaque prompt for delimiters/flags. The controller always
  // supplies it as the last two entries, after all authentication options.
  const options = args.slice(0,-2);
  if (args.length > 32 || args.at(-2) !== '--' || !args.at(-1) || !options.includes('exec') || options.includes('--output-schema') || options.includes('--output-last-message')) throw Error('argv');
  return [...args.slice(0,-2), '--output-schema', SCHEMA, '--output-last-message', CANDIDATE,
    '--', args.at(-1) + '\n\nController terminal contract: return only the final structured handoff JSON. Do not write handoff.json yourself; the controller adapter publishes it after termination. Checks are reported claims; use not-run/unknown honestly. Coordination messages remain reported claims.'];
}

// Claude exposes a schema-constrained final result. OpenCode's pinned CLI stream
// is not a reliable completion signal; retain its explicit untrusted file report.
function otherInvocation(command,args,schema) {
  if (!Array.isArray(args) || args.length>64 || args.some(v=>typeof v!=='string'||v.includes('\0')||Buffer.byteLength(v)>512*1024) || !args.at(-1)) throw Error('argv');
  const claude=command==='/usr/local/bin/claude';
  const options=args.slice(0,claude?-1:-2);
  if (claude) {
    if (!options.includes('--print') || options.some(v=>['--output-format','--json-schema','--resume','--continue','--fork-session'].includes(v))) throw Error('argv');
    strictJSON(schema);
    return [...options,'--output-format','json','--json-schema',schema.toString('utf8'),'--',args.at(-1)+'\n\nController terminal contract: return the final structured handoff as structured_output. Do not write handoff.json or terminal-receipt.json. The adapter publishes after termination. Checks are reported claims, not independent verification.'];
  }
  if(command!=='/usr/local/bin/opencode'||args.at(-2)!=='--'||options[0]!=='run'||options.some(v=>['--format','--session','--continue','--fork','--attach'].includes(v)))throw Error('argv');
  return [...options,'--',args.at(-1)+'\n\nController terminal contract: before ending this task, write exactly one JSON report to '+CANDIDATE+'. The parent directory already exists. Use the exact schema at '+SCHEMA+'. Do not write handoff.json or terminal-receipt.json; the adapter publishes after termination. This replaces the legacy LUCI.md handoff path. Checks are reported claims; record not-run or unknown honestly.'];
}
function claudeReport(bytes) {
  const envelope=strictJSON(bytes,1048576);
  if(!envelope||envelope.type!=='result'||envelope.subtype!=='success'||envelope.is_error!==false||!reportValid(envelope.structured_output))throw Error('result');
  const report=Buffer.from(JSON.stringify(envelope.structured_output)+'\n');
  strictJSON(report);return report;
}
async function capturedClaude(command,args,io) {
  const chunks=[];let bytes=0,overflow=false;
  const capture=new Transform({transform(chunk,encoding,done){const part=Buffer.isBuffer(chunk)?chunk:Buffer.from(chunk,encoding);bytes+=part.length;if(bytes>1048576){overflow=true;chunks.length=0}else if(!overflow){chunks.push(Buffer.from(part))};done(null,chunk);}});
  let failed=false;capture.on('error',()=>{failed=true});capture.pipe(io.stdout,{end:false});
  try{const ok=await settled(command,args,{...io,stdout:capture});if(capture.destroyed)return null;const flushed=finished(capture,{readable:false,cleanup:true});capture.end();await flushed;if(!ok||overflow||failed)return null;return claudeReport(Buffer.concat(chunks));}
  finally{capture.unpipe(io.stdout);capture.destroy();chunks.length=0;}
}

async function settled(command, args, io) {
  return new Promise(resolve => {
    let child, closed = false, code = null, signal = null, failed = false;
    const streams = [{end:false,close:false},{end:false,close:false}];
    let cleanup = () => {};
    const finish = () => { if (closed && streams.every(v => v.close)) { cleanup(); resolve(!failed && code === 0 && signal == null && streams.every(v => v.end)); } };
    try { child = io.spawn(command,args,{stdio:['ignore','pipe','pipe']}); } catch { resolve(false); return; }
    const forward = sig => { failed = true; if (!closed) { try { child.kill(sig); } catch {} } };
    const term = () => forward('SIGTERM'), interrupt = () => forward('SIGINT');
    io.process.on('SIGTERM',term); io.process.on('SIGINT',interrupt);
    cleanup = () => { io.process.off('SIGTERM',term); io.process.off('SIGINT',interrupt); };
    child.once('error',() => { failed = true; });
    [child.stdout,child.stderr].forEach((stream,index) => {
      if (!stream) { failed = true; streams[index].close = true; return; }
      // Bounded pipe backpressure, no transcript buffering or semantic parsing.
      stream.pipe(index ? io.stderr : io.stdout,{end:false});
      stream.once('end',() => { streams[index].end = true; });
      stream.once('error',() => { failed = true; });
      stream.once('close',() => { streams[index].close = true; finish(); });
    });
    child.once('close',(c,s) => { closed = true; code = c; signal = s; finish(); });
  });
}
async function run(args, io = {fs,spawn,process,stdout:process.stdout,stderr:process.stderr}) {
  try {
    const contractBytes = readOne(CONTRACT,16384,io.fs), contract = strictJSON(contractBytes);
    const command = args[1], original = args.slice(2);
    if (!/^[0-9a-f]{64}$/.test(args[0] || '') || args[0] !== hash(contractBytes)) return FAILURE;
    const schema=readOne(SCHEMA,16384,io.fs), kind=command==='/usr/local/bin/claude'?'claude-code':command==='/usr/local/bin/opencode'?'opencode':'codex';
    if ((kind==='codex' ? contract.version!==1 || contract.harness!==undefined : contract.version!==2 || contract.harness!==kind) || contract.commandDigest !== commandDigest([command,...original]) || contract.schemaDigest !== hash(schema)) return FAILURE;
    // Exclusive attempt directory is a durable no-replay marker. An interrupted
    // attempt is retained and cannot be repurposed by running this adapter again.
    io.fs.mkdirSync(OUTPUT+'/.codex-terminal',{mode:0o700});
    for (const name of ['handoff.json','terminal-receipt.json']) if (io.fs.existsSync(OUTPUT+'/'+name)) return FAILURE;
    let bytes;
    if(kind==='claude-code'){bytes=await capturedClaude(command,invocation(command,original,schema),io);if(!bytes)return FAILURE;}
    else {if(!await settled(command,invocation(command,original,schema),io))return FAILURE;bytes=readOne(CANDIDATE,65536,io.fs);}
    const report = strictJSON(bytes);
    if (!reportValid(report)) return FAILURE;
    const receipt = Buffer.from(JSON.stringify({version:1,contractDigest:hash(contractBytes),reportDigest:hash(bytes)})+'\n');
    exclusive(OUTPUT+'/terminal-receipt.json',receipt,io.fs);
    exclusive(OUTPUT+'/handoff.json',bytes,io.fs);
    return 0;
  } catch { return FAILURE; }
}
if (require.main === module) run(process.argv.slice(2)).then(code => { process.exitCode = code; },() => { process.exitCode = FAILURE; });
module.exports = {commandDigest,strictJSON,reportValid,readOne,exclusive,invocation,otherInvocation,claudeReport,capturedClaude,settled,run,CONTRACT,SCHEMA,CANDIDATE,OUTPUT};
