import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {mixedTaskCommand} from './before/task-command.mjs';

test('Retained successful 150-second Go fuzz output passes the generated duration assertion',()=>{
 const command=mixedTaskCommand({task:'build-a',durationSeconds:150});
 const script=Buffer.from(command.cmd.match(/Buffer.from\('([^']+)'/)[1],'base64').toString();
 const start=script.indexOf('   assert(fuzz.row.elapsedMs>=config.durationSeconds*1000);');
 const end=script.indexOf('\n  }\n  // Bounded diagnostic',start);
 assert(start>=0&&end>start);
 const binding=JSON.parse(readFileSync(new URL('./input-binding.json',import.meta.url)));
 const fuzz={row:{elapsedMs:binding.recordedFuzzCommand.elapsedMs},output:readFileSync(new URL('./original-fuzz.log',import.meta.url),'utf8')};
 try{vm.runInNewContext(script.slice(start,end),{assert,fuzz,config:{durationSeconds:150}},{timeout:1000});}
 finally{console.log(JSON.stringify({scope:'Only the exact generated post-fuzz assertions execute; no child process or wall-time substitution',fuzzRow:fuzz.row}));}
});
