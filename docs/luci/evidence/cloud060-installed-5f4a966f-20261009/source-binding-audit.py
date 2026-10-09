from pathlib import Path
import hashlib,json,subprocess,time,tarfile,io,re,os
out=Path(__file__).resolve().parent;repo=Path('/workspace/luci');revision='5f4a966f6d70264c92ee2176dde912e1b43fc3c7';art=Path('/workspace/.luci-tools/cloud060-package-reuse-v4-5f4a966f');helpers=Path('/workspace/.luci-tools/cloud060-real-browser-v4');receipt_path=art/'build-receipt.json';provenance_path=repo/'artifacts/quality-swarm/candidate-5f4a966f/package/package-provenance.json'
def sha(data):return hashlib.sha256(data).hexdigest()
def file_binding(path):
 before=path.stat();h=hashlib.sha256();count=0
 with path.open('rb') as f:
  for chunk in iter(lambda:f.read(1024*1024),b''):count+=len(chunk);h.update(chunk)
 after=path.stat();assert(before.st_dev,before.st_ino,before.st_size,before.st_mtime_ns)==(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns),str(path)+' changed while read'
 return {'sha256':h.hexdigest(),'bytes':count}
def valid_path(value):return isinstance(value,str) and not value.startswith('/') and '..' not in value.split('/') and '\\' not in value and '\n' not in value
errors=[];started=time.time();raw=receipt_path.read_bytes();receipt=json.loads(raw);provenance_raw=provenance_path.read_bytes();provenance=json.loads(provenance_raw)
result={'version':1,'scope':'Read-only exact Git product, original archive/extracted package, copied outputs, helper and current dependency binding audit; no tests/build/app/browser/Docker','product':revision,'startedAt':started,'buildReceipt':{'path':str(receipt_path),'sha256':sha(raw),'bytes':len(raw)},'packageProvenance':{'path':str(provenance_path),'sha256':sha(provenance_raw),'bytes':len(provenance_raw)},'errors':errors}
assert receipt['product']==revision and receipt['passed'] is True and receipt['sourceStable'] is True
assert receipt['reuse']['sourceCommit']==revision and receipt['reuse']['provenanceSHA256']==sha(provenance_raw) and provenance['sourceCommit']==revision and provenance['sourceDirty'] is False
assert provenance['archiveSHA256']==receipt['reuse']['archiveSHA256'];assert receipt['commands']==[]
for key in ['sourceInputs','helperInputs','runtimeDependencyInputs','outputs']:
 rows=receipt[key];assert len({r['path']for r in rows})==len(rows) and all(valid_path(r['path']) for r in rows)
# Immutable Git object reads; never substitute the concurrently changing worktree.
queries=''.join(revision+':'+r['path']+'\n'for r in receipt['sourceInputs']).encode()
batch=subprocess.run(['git','-C',str(repo),'cat-file','--batch'],input=queries,capture_output=True,timeout=30,check=True);stream=io.BytesIO(batch.stdout);source_rows=[];git_content={}
for expected in receipt['sourceInputs']:
 header=stream.readline().decode().rstrip('\n');parts=header.split();assert len(parts)==3 and parts[1]=='blob',header;size=int(parts[2]);data=stream.read(size);assert len(data)==size and stream.read(1)==b'\n';actual={'path':expected['path'],'sha256':sha(data),'bytes':size,'gitBlob':parts[0]};source_rows.append(actual)
 if actual['sha256']!=expected['sha256'] or size!=expected['bytes']:errors.append({'scope':'source','path':expected['path'],'expected':expected,'actual':actual})
 if expected['path'] in receipt['helperRepositoryClosure']:git_content[expected['path']]=data.decode()
assert stream.read()==b''
tracked=subprocess.check_output(['git','-C',str(repo),'ls-tree','-r','--name-only','-z',revision],timeout=30).decode().split('\0');expected_set={x for x in tracked if x and(x.startswith(('control-plane/','sandboxd/','desktop/'))or x in ['go.work','go.work.sum','LICENSE'])}|set(receipt['helperRepositoryClosure']);actual_set={r['path']for r in receipt['sourceInputs']}
if actual_set!=expected_set:errors.append({'scope':'source-set','missing':sorted(expected_set-actual_set),'unexpected':sorted(actual_set-expected_set)})
closure=set();pending=['scripts/acceptance/launch-recovery/acceptance-lifecycle.mjs','scripts/acceptance/launch-recovery/acceptance-startup.mjs','scripts/acceptance/check-create-recovery/installed-recovery.mjs']
while pending:
 path=pending.pop()
 if path in closure:continue
 closure.add(path);text=git_content[path]
 for spec in re.findall(r"(?:from\s*|import\s*\(|import\s*)['\"](\.[^'\"]+)['\"]",text):pending.append(os.path.normpath(str(Path(path).parent/spec)))
assert closure==set(receipt['helperRepositoryClosure'])
result['immutableGit']={'matched':sum(x['sha256']==y['sha256']and x['bytes']==y['bytes']for x,y in zip(source_rows,receipt['sourceInputs'])),'declared':len(source_rows),'declaredSourceSetMatchesSelectedGitTree':actual_set==expected_set,'relativeHelperClosure':sorted(closure),'workingTreeUsedForSourceProof':False}
def audit_files(name,root,rows):
 actual=[]
 for expected in rows:
  path=root/expected['path'];assert path.resolve()==path and path.is_file();found=dict(path=expected['path'],**file_binding(path));actual.append(found)
  if found['sha256']!=expected['sha256'] or found['bytes']!=expected['bytes']:errors.append({'scope':name,'path':expected['path'],'expected':expected,'actual':found})
 return actual
helper_rows=audit_files('helper',helpers,receipt['helperInputs']);dependency_rows=audit_files('dependency',repo,receipt['runtimeDependencyInputs']);output_rows=audit_files('output',art,receipt['outputs']);package=Path(provenance['packageDir']);package_rows=audit_files('extracted-package',package,receipt['outputs']);archive=Path(provenance['archive']);archive_binding=file_binding(archive);assert archive_binding['sha256']==provenance['archiveSHA256']
archive_rows={}
with tarfile.open(archive,'r:*')as tar:
 for member in tar:
  if member.name!='luci' and not member.name.startswith('web/'):continue
  assert valid_path(member.name)
  if member.isdir():continue
  assert member.isfile() and member.name not in archive_rows;h=hashlib.sha256();count=0;f=tar.extractfile(member)
  for chunk in iter(lambda:f.read(1024*1024),b''):h.update(chunk);count+=len(chunk)
  assert count==member.size;archive_rows[member.name]={'path':member.name,'sha256':h.hexdigest(),'bytes':count}
assert set(archive_rows)=={x['path']for x in output_rows}
for expected in output_rows:
 if archive_rows[expected['path']]!=expected:errors.append({'scope':'archive-output','expected':expected,'actual':archive_rows[expected['path']]})
assert file_binding(archive)==archive_binding;assert sha(receipt_path.read_bytes())==sha(raw) and sha(provenance_path.read_bytes())==sha(provenance_raw)
# Check the *sets* and repeat external byte bindings; no later helper/document
# edit can silently inherit the exact earlier run's helper receipt.
helper_names={x.name for x in helpers.iterdir()if x.is_file()and x.suffix in ['.mjs','.py','.md']};assert helper_names=={x['path']for x in helper_rows}
output_names={'luci'}|{str(x.relative_to(art))for x in (art/'web').rglob('*')if x.is_file()};assert output_names==set(archive_rows)
assert helper_rows==audit_files('helper-final',helpers,receipt['helperInputs']);assert dependency_rows==audit_files('dependency-final',repo,receipt['runtimeDependencyInputs'])
result.update({'helperInputs':{'declared':len(helper_rows),'matched':len(helper_rows),'setMatched':True},'currentRuntimeDependencies':{'declared':len(dependency_rows),'matched':len(dependency_rows),'stableAcrossAudit':True,'scope':'Actual pinned Playwright/playwright-core/axe file bytes, not all node_modules'},'copiedOutputs':{'declared':len(output_rows),'matched':len(output_rows),'setMatched':True,'archiveAndExtractedAndCopyParity':True},'originalArchive':{'path':str(archive),**archive_binding},'binarySHA256':next(x['sha256']for x in output_rows if x['path']=='luci'),'webIndexSHA256':next(x['sha256']for x in output_rows if x['path']=='web/index.html'),'noNewBuildExecuted':True,'originLimit':'Publisher clean-build assertion retained; this audit verifies artifact bytes against that declared immutable product, not an independent recompilation','passed':not errors,'endedAt':time.time(),'seconds':time.time()-started})
bindings={'immutableGit':source_rows,'helpers':helper_rows,'runtimeDependencies':dependency_rows,'copiedOutputs':output_rows,'extractedPackage':package_rows,'archiveMembers':list(archive_rows.values())}
(out/'source-binding-details.json').write_text(json.dumps(bindings,indent=2)+'\n');result['details']={'path':'source-binding-details.json',**file_binding(out/'source-binding-details.json')};(out/'source-binding-audit.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'passed':result['passed'],'seconds':result['seconds'],'matched':{'immutableGit':len(source_rows),'helpers':len(helper_rows),'dependencies':len(dependency_rows),'outputs':len(output_rows)},'sha256':file_binding(out/'source-binding-audit.json')['sha256'],'path':str(out/'source-binding-audit.json')}))
