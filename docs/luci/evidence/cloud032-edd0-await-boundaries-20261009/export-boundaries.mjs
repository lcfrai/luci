#!/usr/bin/env node
// Read-only, bounded export from a retained diagnostic; does not launch anything.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import crypto from 'node:crypto';
import {projectBoundaryRows} from './boundaries.mjs';
const [input,output]=process.argv.slice(2);assert(input&&output,'Usage: node export-boundaries.mjs PRIVATE_RESULT FRESH_OUTPUT');
const stat=await fs.lstat(input);assert(stat.isFile()&&!stat.isSymbolicLink()&&stat.size<=64*1024**2,'Bounded ordinary input required');
const bytes=await fs.readFile(input),raw=JSON.parse(bytes),rows=projectBoundaryRows(raw.interactions);
const phases=['idle-interactive-calibration','idle-actionability-trace-followup'];
const phaseCounts=Object.fromEntries(phases.map(phase=>[phase,Object.fromEntries(['project-switch','input','draft-reset'].map(kind=>[kind,raw.interactions.filter(r=>r.phase===phase&&r.kind===kind).length]))]));
const expected={ 'idle-interactive-calibration':{'project-switch':100,input:200,'draft-reset':100}, 'idle-actionability-trace-followup':{'project-switch':8,input:16,'draft-reset':8}};
const issues=[];
if(rows.length!==432||JSON.stringify(phaseCounts)!==JSON.stringify(expected)||raw.interactions.some(r=>!phases.includes(r.phase)))issues.push('action-count-or-phase-mismatch');
if(rows.some(r=>r.issues.length))issues.push('invalid-node-boundary-evidence');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const oid=value=>typeof value==='string'&&/^[a-f0-9]{40}$/.test(value)?value:null;
const result={scope:'Secondary Node command await boundaries. Full original spans remain unchanged. Inclusive waits do not identify internal Playwright actionability, app CPU, paint or pure transport. No cross-clock subtraction.',inputSHA256:hash(bytes),inputBytes:bytes.length,productSource:oid(raw.sourceCommit),helperSource:oid(raw.harnessCheckout?.commit),actualRows:rows.length,expectedRows:432,expectedPhaseCounts:expected,actualPhaseCounts:phaseCounts,successful:rows.filter(r=>r.success).length,failed:rows.filter(r=>!r.success).length,issues,rows};
assert(hash(await fs.readFile(input))===hash(bytes),'Original changed during export');
await fs.writeFile(output,JSON.stringify(result,null,2)+'\n',{flag:'wx',mode:0o600});
console.log(JSON.stringify({rows:rows.length,issues:issues.length,originalUnchanged:true}));
