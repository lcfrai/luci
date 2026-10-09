import assert from 'node:assert/strict';
import {readFileSync,realpathSync,lstatSync} from 'node:fs';
import {dirname,join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {sha} from './fixture-guards.mjs';
export const helperSourcePaths=Object.freeze([
  'check-create-recovery/installed-recovery.mjs','check-create-recovery/recovery-scenario.mjs',
  'check-create-recovery/fixture-guards.mjs','check-create-recovery/check-create-guard.mjs',
  'check-create-recovery/provenance.mjs','check-create-recovery/run-with-observer.py',
  'launch-recovery/acceptance-lifecycle.mjs','launch-recovery/acceptance-startup.mjs',
  'launch-recovery/durable-acceptance-journal.mjs','launch-recovery/fenced-docker-proxy.mjs',
  'four-project-queue/sample-linux-owned.py',
]);
export function verifyHelperSourceBinding({repo,acceptanceRoot,bindingPath,helperCommit,pins}) {
  assert.match(helperCommit,/^[a-f0-9]{40}$/);
  assert.equal(realpathSync(repo),repo);assert.equal(realpathSync(join(repo,'scripts/acceptance')),join(repo,'scripts/acceptance'));
  assert.equal(realpathSync(acceptanceRoot),acceptanceRoot);assert.equal(acceptanceRoot,join(repo,'scripts/acceptance'));
  assert.equal(realpathSync(bindingPath),bindingPath);
  const stat=lstatSync(bindingPath);assert(stat.isFile()&&!stat.isSymbolicLink()&&stat.nlink===1&&stat.size<=2*1024**2);
  const bytes=readFileSync(bindingPath),after=lstatSync(bindingPath);assert.equal(after.ino,stat.ino);assert.equal(after.mtimeMs,stat.mtimeMs);assert.equal(bytes.length,stat.size);
  const binding=JSON.parse(bytes);assert.equal(binding.version,1);assert.equal(binding.sourceRoot,'scripts/acceptance');
  assert.equal(binding.helperSourceCommit,helperCommit);assert.deepEqual(binding.pins,pins);
  assert.deepEqual(binding.executedFiles.map(row=>row.path).sort(),[...helperSourcePaths].sort());
  for(const row of binding.executedFiles){
    const path=join(acceptanceRoot,row.path);assert.equal(realpathSync(path),path);const s=lstatSync(path);assert(s.isFile()&&!s.isSymbolicLink()&&s.nlink===1);assert.equal(s.size,row.bytes);
    assert.equal(row.repositoryPath,'scripts/acceptance/'+row.path);assert.match(row.sha256,/^[a-f0-9]{64}$/);
    assert.equal(sha(readFileSync(path)),row.sha256,'Executed helper differs from frozen binding');
    const committed=execFileSync('git',['show',helperCommit+':'+row.repositoryPath],{cwd:repo,timeout:10000,maxBuffer:8*1024**2});
    assert.equal(sha(committed),row.sha256,'Executed helper differs from selected committed source');
  }
  return binding;
}
export function verifyPackage({binary,web,provenancePath,imageProvenancePath,pins,repo}) {
  const raw=readFileSync(provenancePath),p=JSON.parse(raw),image=JSON.parse(readFileSync(imageProvenancePath));
  assert.equal(p.sourceCommit,pins.product);assert.equal(p.sourceDirty,false);
  assert.equal(sha(readFileSync(binary)),p.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),p.webIndexSHA256);
  assert.equal(realpathSync(web),realpathSync(join(dirname(binary),'web')));
  assert.equal(image.candidateImage,pins.image);assert.equal(image.exportVerification.status,'passed');
  assert.equal(image.exportVerification.candidateImage,pins.image);assert.deepEqual(image.sourcesBefore,image.sourcesAfter);
  for(const [path,digest] of Object.entries(image.sourcesBefore)){
    assert(/^(sandboxd\/|images\/sandbox\/)/.test(path)&&!path.split('/').includes('..'));
    assert.equal(sha(execFileSync('git',['show',pins.product+':'+path],{cwd:repo,maxBuffer:32*1024**2})),digest);
  }
  // Same frozen archive/member validation as the installed DAG helper.
  const verified=JSON.parse(execFileSync('python3',['-c',String.raw`import hashlib,json,pathlib,sys,tarfile
archive=pathlib.Path(sys.argv[1]); root=pathlib.Path(sys.argv[3]).resolve()
def digest(stream):
 h=hashlib.sha256()
 while block:=stream.read(1024*1024): h.update(block)
 return h.hexdigest()
with archive.open('rb') as stream:
 if digest(stream)!=sys.argv[2]: raise ValueError('Archive differs')
count=0
with tarfile.open(archive,'r:gz') as source:
 for row in source:
  rel=pathlib.PurePosixPath(row.name)
  if rel.is_absolute() or '..' in rel.parts: raise ValueError('Unsafe member')
  if row.isdir(): continue
  if not row.isfile(): raise ValueError('Nonordinary member')
  installed=root.joinpath(*rel.parts)
  if installed.is_symlink() or not installed.is_file() or installed.resolve()!=installed: raise ValueError('Linked or missing member')
  with source.extractfile(row) as a, installed.open('rb') as b:
   if digest(a)!=digest(b): raise ValueError('Installed member differs')
  count+=1
print(json.dumps({'verifiedMembers':count,'allMembersMatch':True}))`,p.archive,p.archiveSHA256,dirname(binary)],{encoding:'utf8',timeout:60000,maxBuffer:1024**2}));
  assert(verified.verifiedMembers>0);return {package:p,image,verified,provenanceBytes:raw};
}
