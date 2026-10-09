from pathlib import Path
import datetime, hashlib, json, subprocess, shutil
BASE=Path('/workspace/.luci-tools/cloud032-summary-delivery-v2-20261009')
REPO=Path('/workspace/luci')
PRODUCT='99007404a5ddd8592dee376ea00d99c6180d26fe'
HELPER='a31e1ac485c95bdbcc9ebb011581da5ce3f5c67a'
def sha(b): return hashlib.sha256(b).hexdigest()
def read(p): return json.loads(p.read_text())
def git(*args): return subprocess.check_output(['git','-C',str(REPO),*args],timeout=10)
def flat(path):
 p=Path(path)
 return p.name if p.parent.name=='sustained-four-project' else p.parent.name+'_'+p.name
assert git('rev-parse','HEAD').decode().strip()==HELPER
assert not git('status','--porcelain').strip()
assert not (BASE/'preparation-manifest.json').exists(), 'already sealed'
plan=BASE/'PLAN-v2.md'
old=plan.read_text()
assert '--helper-source 67a5b7c75ca6354d43085f5bb81ec53ee4291f34' in old
preserved=BASE/'variants/preseal-plan'
preserved.mkdir()
shutil.copy2(plan,preserved/'PLAN-v2.md')
plan.write_text(old.replace('--helper-source 67a5b7c75ca6354d43085f5bb81ec53ee4291f34','--helper-source '+HELPER).replace('Only after root reviews the frozen artifact and grants a clean exclusive runtime window. This is **not executed**.','Root approved one fresh 990 v2 diagnostic after its independent 67a5 idle measurement and closure ended. Root owns launch in a clean exclusive runtime window. This is **not executed** at preparation seal.'))
binding=read(BASE/'actual/external-source-binding.json')
templates=read(BASE/'template-capture.json')['sources']
for path, expected in templates.items():
 assert sha(git('show',PRODUCT+':'+path))==expected['sha256'],path
 assert sha((BASE/'original'/flat(path)).read_bytes())==expected['sha256'],path
for path, expected in binding['repositoryDependencies'].items():
 assert sha(git('show',HELPER+':'+path))==expected,path
 assert sha((REPO/path).read_bytes())==expected,path
 assert sha((BASE/'actual'/flat(path)).read_bytes())==expected,path
for path,expected in binding['localSources'].items(): assert sha((BASE/'actual'/path).read_bytes())==expected,path
assert (BASE/'header-delivery.mjs').read_bytes()==(BASE/'actual/header-delivery.mjs').read_bytes()
def guard(p):
 s=p.read_text(); a=s.index("await context.route('**/*',async route=>"); b=s.index('await route.continue();});',a)+len('await route.continue();});'); return s[a:b].encode()
parent=Path('/workspace/.luci-tools/cloud032-idle-integrated-candidate-20261009/actual/sustained.mjs')
assert sha(parent.read_bytes())==binding['parentIdleDriverSHA256']
assert guard(parent)==guard(BASE/'actual/sustained.mjs')
assert sha(guard(parent))==binding['originalRouteGuardSHA256']
for path in ['interaction-metrics.mjs','latency-diagnostics.mjs']:
 assert (BASE/'original'/path).read_bytes()==(BASE/'actual'/path).read_bytes()
validation={}
for name in ['final-units','final-syntax','pinned-input-refusals']:
 result=read(BASE/'validation'/name/'result.json')
 assert result['exitCode']==0
 assert result['inputsBefore']==result['inputsAfter']
 for path,expected in result['inputsAfter'].items(): assert sha((BASE/path).read_bytes())==expected,path
 validation[name]={'result':'validation/'+name+'/result.json','sha256':sha((BASE/'validation'/name/'result.json').read_bytes()),'exitCode':0}
package=REPO/'artifacts/quality-swarm/candidate-99007404/package/package-provenance.json'
preservation=REPO/'artifacts/quality-swarm/candidate-67a5b7c7/package/previous-release-preservation.json'
assert sha(package.read_bytes())==binding['pinnedPackageProvenanceSHA256']
p=read(package); r=read(preservation)
assert p['sourceCommit']==PRODUCT and r['oldSource']==PRODUCT
assert r['members']['luci-linux-amd64.tar.gz']['sha256']==p['archiveSHA256']
archive=Path(r['preservedPath'])/'luci-linux-amd64.tar.gz'
assert archive.is_file() and archive.stat().st_size==r['members']['luci-linux-amd64.tar.gz']['bytes']
source=BASE/'source-seal'; source.mkdir()
shutil.copy2(package,source/'package-provenance-original990.json')
shutil.copy2(preservation,source/'previous-release-preservation-original990.json')
receipt={'sealedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'helperCommit':HELPER,'cleanCheckout':True,'productCommit':PRODUCT,'verifiedFrozenGitTemplates':len(templates),'verifiedImportedDependencies':len(binding['repositoryDependencies']),'verifiedGeneratedLocalSources':len(binding['localSources']),'routeGuardUnchanged':True,'measurementModulesUnchanged':True,'finalValidationSourceBytesUnchanged':True,'packageProvenanceSHA256':sha(package.read_bytes()),'archive':str(archive),'archiveExpectedSHA256':p['archiveSHA256'],'archiveBytesByStat':archive.stat().st_size,'archiveDigestVerifiedNow':False,'archiveVerification':'Original root preservation receipt retained; complete archive/hash/member verification remains mandatory runtime preflight. No archive or image payload read by sealing.','imageExpectedID':binding['pinnedImageID'],'validation':validation,'commands':['git rev-parse HEAD','git status --porcelain','git show <990>:<17 frozen template paths>','git show <a31>:<7 imported dependency paths>','small source/metadata SHA256 checks; archive stat only'],'installedExperimentRun':False,'limits':'Response availability/backpressure diagnostic only; no frontend commit causality or natural performance/acceptance claim.'}
(source/'receipt.json').write_text(json.dumps(receipt,indent=2)+'\n')
assert not git('status','--porcelain').strip()
files=[]
for path in sorted(BASE.rglob('*')):
 assert not path.is_symlink(),str(path)
 if path.is_file():
  assert '__pycache__' not in path.parts and path.suffix!='.pyc',str(path)
  data=path.read_bytes(); files.append({'path':str(path.relative_to(BASE)),'sha256':sha(data),'bytes':len(data)})
manifest={'version':1,'scope':'Frozen external preparation; one root-launched fresh990 diagnostic authorized but not executed at seal. No source mutation or installed performance claim.','sealedAt':receipt['sealedAt'],'productCommit':PRODUCT,'helperCommit':HELPER,'templateCaptureHistoricalHEAD':'e2abd0e39e934e68ead3fd3e906a79336a8bda19','templateCaptureClarification':'Historical capture metadata retained exactly; all 17 frozen template bytes independently matched immutable Git990 at seal.','sourceSeal':'source-seal/receipt.json','packageProvenanceSHA256':receipt['packageProvenanceSHA256'],'archive':str(archive),'archiveExpectedSHA256':p['archiveSHA256'],'imageExpectedID':binding['pinnedImageID'],'driverSHA256':binding['localSources']['sustained.mjs'],'externalSourceBindingSHA256':sha((BASE/'actual/external-source-binding.json').read_bytes()),'transportProofManifestSHA256':sha((BASE/'transport-proof/publication-manifest.json').read_bytes()),'proofOutcome':'18 new headers-only checkpoints passed. Not cumulative with original18; v1 prebody failure remains preserved separately.','managerOutcome':'Initial8 passed; added receipt join regression failed8/9 before narrow correction; final same9 passed.','validation':validation,'priorV1Unchanged':{'directory':'../cloud032-summary-delivery-diagnostic-20261009/','preparationManifestSHA256':'950869356f71b12dd74acb5722d92399719d97fd9afe31fe4d1321026a3c909f','originalTransportProofManifestSHA256':'d394b0dbbc99f07830d4a5b0efcc7b108e9530c396fd2f3681d2734e3e0d7fc0','failedPrebodyProofManifestSHA256':'8f032289ef8dba4686cf764147da1af16d2cdc56e2af27caf38ee0a56a7909e4'},'memberCount':len(files),'totalBytes':sum(x['bytes'] for x in files),'members':files,'manifestExclusions':['preparation-manifest.json (self)','SHA256SUMS (contains manifest hash)']}
mp=BASE/'preparation-manifest.json'; mp.write_text(json.dumps(manifest,indent=2)+'\n')
lines=[f"{x['sha256']}  {x['path']}" for x in files]+[f'{sha(mp.read_bytes())}  preparation-manifest.json']
checksums=BASE/'SHA256SUMS'; checksums.write_text('\n'.join(lines)+'\n')
print(json.dumps({'manifestSHA256':sha(mp.read_bytes()),'checksumsSHA256':sha(checksums.read_bytes()),'payloadMembers':len(files),'totalMembersWithManifestAndChecksums':len(files)+2,'payloadBytes':manifest['totalBytes'],'driverSHA256':manifest['driverSHA256'],'helperCommit':HELPER,'sourceSeal':str(source/'receipt.json')}))
