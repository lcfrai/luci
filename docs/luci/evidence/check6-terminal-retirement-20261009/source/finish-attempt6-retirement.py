from pathlib import Path
import importlib.util,json,hashlib,os,subprocess,stat
base=Path('<PRIVATE_TOOL_ROOT>/terminal-retirement-preparation');checker=base/'preflight.py';sha=lambda b:hashlib.sha256(b).hexdigest();assert sha(checker.read_bytes())=='f0357901cdfa808e3693b6ee508627b5d3330c421cc87f8aab031ecad5b68e3c'
spec=importlib.util.spec_from_file_location('checks',checker);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
c=json.loads((base/'attempt6-root-review-config-retry2.json').read_text());pre=Path(c['output']);r=json.loads((pre/'preflight-result.json').read_text());out=Path('<PRIVATE_TOOL_ROOT>/check6-terminal-retirement-retry1');prior=out/'retirement.json';previous=json.loads(prior.read_text());assert r['passed'] and previous['passed'] is False and [v['stage'] for v in previous['steps']]==['archive-content-and-closure-reverified','fixture-atomically-moved-forensic-only']
root=Path(c['fixtureRoot']);dest=Path(previous['retiredFixture']);assert dest==Path('<FORENSIC_ONLY_PRIVATE_FIXTURE>') and dest.is_dir() and stat.S_ISREG(root.lstat().st_mode) and root.lstat().st_nlink==1 and root.lstat().st_mode & 0o222==0
tombstone=root.read_bytes();t=json.loads(tombstone);assert t['terminallyRetired'] and t['doNotResume'] and t['receipt']==str(prior)
try:os.stat(root/'state');raise AssertionError('Old state root still traversable')
except NotADirectoryError:pass
manifest=json.loads((pre/'fixture-manifest.json').read_text());assert m.fixture_manifest(dest)==manifest;archive=Path(r['archive']['path']);assert archive.stat().st_size==r['archive']['bytes'] and m.file_digest(archive,m.MAX_BYTES+64*1024**2)==r['archive']['sha256']
for key in ['targetHistoryRelativePath','historyHistoryRelativePath']:
 row=r['partialHistoryAfter'][key];assert sha(m.ordinary(dest/row['relativePath']))==row['rawSHA256']
values={}
for key,pin in c['inputs'].items():
 b=m.ordinary(pin['path']);assert sha(b)==pin['sha256']
 if key in ['recoveryReceipt','wrapperExit','preexistingProcessProof']:values[key]=json.loads(b)
m.check_receipts(c,values);lb=m.ordinary(dest/'state/control-plane/execution/leases.json');assert sha(lb)==r['ledgerRawSHA256'];fence=json.loads(lb)['checkFences'][0]
rec={'at':m.now(),'scope':'Roll forward separately archived terminal fixture cleanup; original acceptance remains FAILED. No product recovery or release outcome changed.','previousJournalSHA256':sha(prior.read_bytes()),'preDeleteHelperFailureSHA256':sha((out/'post-move-pre-delete-failure.json').read_bytes()),'originalRecoverySHA256':c['inputs']['recoveryReceipt']['sha256'],'markerId':c['markerId'],'fixture':str(root),'retiredFixture':str(dest),'tombstoneSHA256':sha(tombstone),'archive':r['archive'],'ledgerSHA256':sha(lb),'passed':False}
rec['closureBefore']={};m.closure(c,rec['closureBefore'],values['preexistingProcessProof']);rec['markerBefore']=m.marker(c,fence);m.write_new(out/'pre-delete-confirmation.json',rec)
env={'PATH':os.environ['PATH'],'DOCKER_HOST':c['dockerHost']};cmd=['docker','container','rm',c['markerId']];deleted=subprocess.run(cmd,env=env,capture_output=True,text=True,timeout=15);rec['delete']={'command':cmd,'exitCode':deleted.returncode,'stdout':deleted.stdout.strip(),'stderr':deleted.stderr.strip(),'at':m.now()};m.write_new(out/'delete-receipt.json',rec['delete']);assert deleted.returncode==0 and deleted.stdout.strip()==c['markerId']
q=subprocess.run(['docker','inspect',c['markerId']],env=env,capture_output=True,text=True,timeout=15);assert q.returncode==1 and ('No such object: '+c['markerId']) in q.stderr;assert m.docker(c,'info','--format','{{.ID}}')==c['engineId'];assert m.docker(c,'ps','-aq','--no-trunc')=='' and m.docker(c,'volume','ls','-q')=='';assert m.fixture_manifest(dest)==manifest and m.ordinary(dest/'state/control-plane/execution/leases.json')==lb and root.read_bytes()==tombstone
rec['closureAfter']={};m.closure(c,rec['closureAfter'],values['preexistingProcessProof']);rec.update(passed=True,finishedAt=m.now(),markerAbsent=True,containers=[],volumes=[],fixtureManifestUnchanged=True,ledgerUnchanged=True,originalAcceptanceStillFailed=True,archiveForensicOnlyNeverResume=True);m.write_new(out/'retirement-final.json',rec)
print(json.dumps({'passed':True,'markerRemoved':c['markerId'],'containers':0,'volumes':0,'fixtureAndLedgerBytesPreserved':True,'originalPathTombstoned':True,'originalAcceptanceStillFailed':True,'receiptSHA256':sha((out/'retirement-final.json').read_bytes())}))
