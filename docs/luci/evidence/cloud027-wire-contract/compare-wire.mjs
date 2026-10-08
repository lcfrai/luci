import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFileSync,writeFileSync} from 'node:fs';
import {checkCommandDigest,validateCheckCreate} from '/workspace/luci/scripts/acceptance/task-dependencies/project-check-guard.mjs';

const root='/workspace/.luci-tools/cloud027-wire-contract';
const read=name=>JSON.parse(readFileSync(`${root}/${name}`,'utf8'));
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const options=read('guard-input.json');
const input=read('production-create-wire.json');
const probe=read('production-probe-result.json');
const spec=read('probe-input.json').Spec;
const actualGoJSON=readFileSync(`${root}/production-command-digest-input.json`);
const helperDigest=checkCommandDigest({...options,identity:spec.Identity});
const result={scope:'Controlled production source and real SDK serialization with retained fixture parameters; not an original-request capture or installed rerun',checks:{},limitations:['Retained check history is post-refusal unknown. The Create-time comparison models only check.status and command.status as running; no original running snapshot or request body was retained.','Daemon responses are provided by the existing in-memory checkProbe. This does not prove real Docker execution.'],hashes:{}};
try {
 assert.equal(hash(actualGoJSON),probe.commandDigest);
 assert.equal(helperDigest,probe.commandDigest);
 assert.equal(probe.commandDigest,options.ledger.checks[0].identity.commandDigest);
 result.checks.productionGoStructDigestMatchesRetainedLease=true;
 result.checks.helperDigestMatchesProductionGoStruct=true;
 assert.throws(()=>validateCheckCreate(input,options));
 result.checks.retainedUnknownHistoryRefused=true;
 result.modeledLifecycleFields={before:{checkStatus:options.check.status,commandStatus:options.check.commands[0].status},after:{checkStatus:'running',commandStatus:'running'}};
 options.check.status='running';
 options.check.commands[0].status='running';
 const validated=validateCheckCreate(input,options);
 assert.equal(validated.commandDigest,probe.commandDigest);
 result.checks.productionSDKWireAcceptedByCurrentGuard=true;
 result.commandDigest=probe.commandDigest;
 result.passed=true;
} catch(error) {
 result.passed=false;
 result.error={name:error.name,message:error.message,stack:error.stack};
 process.exitCode=1;
}
for(const name of ['probe-input.json','guard-input.json','production-command-digest-input.json','production-create-wire.json','production-probe-result.json','wire_contract_test.go','compare-wire.mjs'])result.hashes[name]=hash(readFileSync(`${root}/${name}`));
for(const name of ['control-plane/internal/backend/checks.go','control-plane/internal/backend/backend.go','control-plane/internal/backend/dockerbe/checks.go','control-plane/internal/backend/dockerbe/checks_test.go','control-plane/internal/backend/dockerbe/check_resource_limits_test.go','control-plane/go.mod','control-plane/go.sum','scripts/acceptance/task-dependencies/project-check-guard.mjs'])result.hashes[name]=hash(readFileSync(`/workspace/luci/${name}`));
writeFileSync(`${root}/wire-parity-result.json`,JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result,null,2));
