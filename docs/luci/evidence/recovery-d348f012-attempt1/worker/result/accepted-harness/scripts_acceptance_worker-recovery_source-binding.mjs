// Bind the executed adapter and reused helpers to one explicitly selected clean
// checkout. Product/package provenance remains a separately selected input.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {readFileSync,lstatSync,realpathSync} from 'node:fs';
import {join} from 'node:path';
import {validateSourceBinding,validateHelperLocation} from '../sustained-four-project/source-binding.mjs';

export const helperPaths=Object.freeze([
 'worker-recovery/installed-worker-recovery.mjs',
 'worker-recovery/worker-recovery-scenario.mjs',
 'worker-recovery/provenance.mjs',
 'worker-recovery/source-binding.mjs',
 'launch-recovery/fenced-docker-proxy.mjs',
 'launch-recovery/worker-create-guard.mjs',
 'launch-recovery/acceptance-lifecycle.mjs',
 'launch-recovery/acceptance-startup.mjs',
 'launch-recovery/durable-acceptance-journal.mjs',
 'four-project-queue/native-terminal-diagnostics.mjs',
 'sustained-four-project/task-context.mjs',
 'sustained-four-project/source-binding.mjs',
].map(path=>'scripts/acceptance/'+path));
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');

export function captureSourceBinding({repo,actualDirectory,provenance,expectedProduct,expectedHelper,
 readGit=args=>execFileSync('git',args,{cwd:repo,timeout:15000,maxBuffer:4*1024**2})}){
 assert.equal(typeof expectedHelper,'string','Explicit --expected-helper-source is required');
 validateHelperLocation(realpathSync(actualDirectory),join(realpathSync(repo),'scripts/acceptance/worker-recovery'));
 const checkout=validateSourceBinding({provenance,expectedProduct,expectedHelper,
  helperCommit:String(readGit(['rev-parse','HEAD'])).trim(),helperDirty:String(readGit(['status','--porcelain'])).trim()!==''});
 const files=helperPaths.map(path=>{
  const file=join(repo,path),stat=lstatSync(file);assert(stat.isFile()&&!stat.isSymbolicLink()&&stat.nlink===1,'Executed helper must be an ordinary tracked file');assert.equal(realpathSync(file),file);
  const bytes=readFileSync(file),committed=readGit(['show',expectedHelper+':'+path]);assert(bytes.equals(Buffer.from(committed)),'Executed helper differs from its selected commit: '+path);
  return {path,bytes:bytes.length,sha256:sha(bytes)};
 });
 return {version:1,checkout,files};
}
