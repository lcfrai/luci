import assert from 'node:assert/strict';
import {readFileSync,realpathSync} from 'node:fs';
import {dirname,join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
const sha=value=>createHash('sha256').update(value).digest('hex');
export function verifyPackage({binary,web,provenancePath,imageProvenancePath,pins,repo}) {
  const raw=readFileSync(provenancePath),p=JSON.parse(raw),image=JSON.parse(readFileSync(imageProvenancePath));
  assert.equal(p.sourceCommit,pins.product);assert.equal(p.sourceDirty,false);
  assert.equal(sha(readFileSync(binary)),p.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),p.webIndexSHA256);
  assert.equal(realpathSync(web),realpathSync(join(dirname(binary),'web')));
  assert.equal(image.candidateImage,pins.image);assert.equal(image.exportVerification.status,'passed');
  assert.equal(image.exportVerification.candidateImage,pins.image);assert.deepEqual(image.sourcesBefore,image.sourcesAfter);
  assert(Object.keys(image.sourcesBefore).length>10,'Image source fence is missing');
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
