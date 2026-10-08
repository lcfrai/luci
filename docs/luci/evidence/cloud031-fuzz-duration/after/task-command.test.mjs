import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {mixedTaskCommand,parseGoFuzzProgress} from './task-command.mjs';

const retained=readFileSync(new URL('./fixtures/go-fuzz-progress.txt',import.meta.url),'utf8');
const line=(duration,executions=1)=>`fuzz: elapsed: ${duration}, execs: ${executions} (1/sec), new interesting: 0 (total: 3)`;
function generatedAssertion(output,elapsedMs=182028.482164){
 const command=mixedTaskCommand({task:'build-a',durationSeconds:150});
 const script=Buffer.from(command.cmd.match(/Buffer.from\('([^']+)'/)[1],'base64').toString();
 const parserStart=script.indexOf('const parseGoFuzzProgress='),parserEnd=script.indexOf('\nconst events=',parserStart);
 const guardStart=script.indexOf('   assert(fuzz.row.elapsedMs>=config.durationSeconds*1000);');
 const guardEnd=script.indexOf('\n  }\n  // Bounded diagnostic',guardStart);
 assert(parserStart>=0&&parserEnd>parserStart&&guardStart>=0&&guardEnd>guardStart);
 const fuzz={output,row:{elapsedMs}};
 vm.runInNewContext(script.slice(parserStart,parserEnd)+'\n'+script.slice(guardStart,guardEnd),{assert,Buffer,fuzz,config:{durationSeconds:150}},{timeout:1000});
 return fuzz.row;
}

test('Retained installed Go output has exact byte binding and150seconds of fuzz progress',()=>{
 assert.equal(Buffer.byteLength(retained),4316);
 assert.equal(createHash('sha256').update(retained).digest('hex'),'506a10848d01b2bb297279b64634aef1bf641c3dcef52080f5683c8cf99161aa');
 assert.deepEqual(parseGoFuzzProgress(retained),{activeFuzzSeconds:150,fuzzExecutions:6993018});
 assert.deepEqual(generatedAssertion(retained),{elapsedMs:182028.482164,activeFuzzSeconds:150,fuzzExecutions:6993018});
});
test('Seconds, minutes and hours preserve elapsed units',()=>{
 for(const [duration,seconds] of [['0s',0],['57s',57],['150s',150],['1m0s',60],['2m30s',150],['1h0m0s',3600],['2h3m4s',7384]]){
  assert.deepEqual(parseGoFuzzProgress(line(duration)),{activeFuzzSeconds:seconds,fuzzExecutions:1});
 }
});
test('Final newline is optional, CRLF is accepted and summaries preserve maxima',()=>{
 const output=[line('2m30s',900),line('2m27s',950),line('2m30s',940)].join('\r\n');
 assert.deepEqual(parseGoFuzzProgress(output),{activeFuzzSeconds:150,fuzzExecutions:950});
 assert.deepEqual(parseGoFuzzProgress(line('2m30s',900)+'\n'),{activeFuzzSeconds:150,fuzzExecutions:900});
});
test('Baseline coverage and package/wall duration cannot stand in for active progress',()=>{
 assert.throws(()=>parseGoFuzzProgress('fuzz: elapsed: 2m30s, gathering baseline coverage: 3/3 completed\nPASS\nok fixture.local/workload 150.078s\n'),/No Go fuzz execution progress/);
 assert.throws(()=>generatedAssertion(line('57s',2560985)+'\nPASS\nok fixture.local/workload 182.028s\n',182028.482164));
 assert.throws(()=>generatedAssertion(line('2m30s',6993018)+'\nPASS\n',149999));
 assert.throws(()=>generatedAssertion(line('2m30s',0)+'\nPASS\n'));
 assert.throws(()=>generatedAssertion(line('2m30s',6993018)+'\nFAIL\n'));
});
test('Malformed progress, unsupported duration units and unsafe numbers are refused',()=>{
 for(const duration of ['-1s','NaNs','Infinitys','1m','1d0s','1m60s','1h60m0s','2m30s-junk','9007199254740992s'])assert.throws(()=>parseGoFuzzProgress(line(duration)));
 for(const value of ['1.5','-1','NaN','9007199254740992'])assert.throws(()=>parseGoFuzzProgress(line('2m30s',value)));
 assert.throws(()=>parseGoFuzzProgress('fuzz: elapsed: 2m30s, execs:not-a-number'));
 assert.throws(()=>parseGoFuzzProgress('x'.repeat(1024**2+1)),/exceeds1MiB/);
 assert.throws(()=>parseGoFuzzProgress(null));
});
