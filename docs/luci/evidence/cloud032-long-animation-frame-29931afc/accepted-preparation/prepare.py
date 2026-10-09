#!/usr/bin/env python3
"""Prepare exact299 external long-frame diagnostic; never starts runtime."""
from pathlib import Path
import json,hashlib,shutil,subprocess,difflib
R=Path(__file__).resolve().parent;REPO=Path('/workspace/luci')
T=REPO/'docs/luci/evidence/cloud032-navigation-callback-5f4a966f/prepared-runtime'
SOURCE='29931afc5b31ae9b9bd7b8973a7e3fdb840609a3';OLD='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def put(p,value):p.write_text(json.dumps(value,indent=2)+'\n')
def once(text,before,after):
    assert text.count(before)==1,before
    return text.replace(before,after)
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip()==SOURCE
assert not subprocess.check_output(['git','status','--porcelain'],cwd=REPO,text=True)
assert not (R/'actual').exists(),'Preserve previous preparation'
shutil.copytree(T/'actual',R/'actual')
capture=json.loads((T/'template-capture.json').read_text())
for name,entry in capture['sources'].items():
    data=subprocess.check_output(['git','show',SOURCE+':'+name],cwd=REPO,timeout=10)
    assert hashlib.sha256(data).hexdigest()==entry['sha256'] and len(data)==entry['bytes'] and (REPO/name).read_bytes()==data,name
capture.update(capturedHEAD=SOURCE,scope='Original17 helper inputs unchanged at exact299; new external setup-only observer is separately bound.')
put(R/'template-capture.json',capture);put(R/'actual/template-capture.json',capture)
package=Path('/tmp/luci-retained-tools/package-0.5.0-29931afc')
prov=REPO/'artifacts/quality-swarm/candidate-29931afc/package/package-provenance.json';provenance=json.loads(prov.read_text())
assert provenance['sourceCommit']==SOURCE and provenance['sourceDirty'] is False and Path(provenance['packageDir'])==package
# Bounded9MiB JS-only read for attribution identities; full package/archive
# verification remains mandatory in the existing launcher before any runtime.
assets=sorted((package/'web/assets').glob('*.js'))
assert 0<len(assets)<=512 and sum(p.stat().st_size for p in assets)<=16*1024*1024
bundles=[{'path':'/assets/'+p.name,'sha256':sha(p)} for p in assets]
put(R/'actual/long-frame-bundles.json',bundles)
shutil.copyfile(R/'long-frame-observer.mjs',R/'actual/long-frame-observer.mjs')
before=(T/'actual/sustained.mjs').read_text();driver=before
driver=once(driver,"import {createReadinessLedger} from './readiness-ledger.mjs';","import {createReadinessLedger} from './readiness-ledger.mjs';\nimport {installLongFrameTiming} from './long-frame-observer.mjs';")
driver=once(driver,"const driverNames=['checkout-binding.py'","const driverNames=['long-frame-observer.mjs','long-frame-bundles.json','checkout-binding.py'")
driver=once(driver,'installRoutes:async context=>{await installBrowserTiming(context);','installRoutes:async context=>{await installBrowserTiming(context);await installLongFrameTiming(context,{receipt,timingLedger,bundles:JSON.parse(readFileSync(join(here,\'long-frame-bundles.json\'),\'utf8\'))});')
(R/'actual/sustained.mjs').write_text(driver)
binding=json.loads((R/'actual/external-source-binding.json').read_text())
binding.update(executionDirectory=str(R/'actual'),prospectiveProduct=SOURCE,prospectiveHelper=SOURCE)
binding['navigationVerificationDiagnostic']['runtimeBindingStatus']='Exact299 product/helper binding; original navigation callback and400+32 actions unchanged. Setup-only Long Animation Frame capture added with separate limitations.'
binding['longFrameDiagnostic']={'version':1,'templateDriverSHA256':hashlib.sha256(before.encode()).hexdigest(),'integration':'Setup-only import, source registration and install hook; original measured actions/metrics/markers are byte-identical. Existing passive snapshot/ledger carries added data; no extra RPC in spans.','noRuntimeDuringPreparation':True,'readiness':'Requires explicit coordinated runtime window','bundleCount':len(bundles),'bundleBytesRead':sum(p.stat().st_size for p in assets),'packageProvenanceSHA256':sha(prov)}
for name in ['long-frame-observer.mjs','long-frame-bundles.json']:binding['localSources'][name]=sha(R/'actual'/name)
for name in binding['localSources']:binding['localSources'][name]=sha(R/'actual'/name)
put(R/'actual/external-source-binding.json',binding)
launcher=(T/'launch.py').read_text()
old_prov=T/'package-provenance.json'
for a,b in [(OLD,SOURCE),('5f4a966f','29931afc'),('Frozen matching5f4 product and helper binding required','Frozen matching299 product and helper binding required'),(sha(old_prov),sha(prov)),('Original matching5f4 provenance bytes required','Original matching299 provenance bytes required')]:
    assert a in launcher,a
    launcher=launcher.replace(a,b)
(R/'launch.py').write_text(launcher)
cmd=json.loads((T/'launch-command.json').read_text());argv=cmd['command'];argv[1]=str(R/'launch.py')
for flag,value in [('--source',SOURCE),('--helper-source',SOURCE),('--package',str(package)),('--provenance',str(prov)),('--output','/workspace/luci/artifacts/quality-swarm/idle-29931afc-long-frame-attempt1')]:argv[argv.index(flag)+1]=value
cmd['scope']='Prepared-only helper instrumentation of original400+32 actions on exact299 package; capture scopes and overhead remain separate. Explicit root runtime window mandatory.'
put(R/'launch-command.json',cmd)
image=Path(argv[argv.index('--image-provenance')+1]);image_value=json.loads(image.read_text());assert image_value['sourcesBefore']==image_value['sourcesAfter'] and len(image_value['sourcesBefore'])==106
# No image/Docker inspection or broad package rehash during preparation.
subprocess.run(['git','diff','--exit-code',OLD,SOURCE,'--',*image_value['sourcesBefore']],cwd=REPO,check=True,stdout=subprocess.DEVNULL,timeout=15)
for name in ['interaction-metrics.mjs','navigation-verifier.mjs','latency-diagnostics.mjs','autonomous-readiness.mjs','readiness-ledger.mjs']:
    assert (R/'actual'/name).read_bytes()==(T/'actual'/name).read_bytes(),name
put(R/'runtime-binding.json',{'source':SOURCE,'originalTemplateSource':OLD,'packageDirectory':str(package),'packageProvenanceSHA256':sha(prov),'archiveSHA256':provenance['archiveSHA256'],'binarySHA256':provenance['binarySHA256'],'webIndexSHA256':provenance['webIndexSHA256'],'bundleCount':len(bundles),'bundleMapSHA256':sha(R/'actual/long-frame-bundles.json'),'image':image_value['candidateImage'],'imageReceiptSHA256':sha(image),'imageInputsUnchangedFrom5f4':True,'selectedRepositoryInputs':len(capture['sources']),'all17RepositoryInputsUnchanged':True,'originalActionModulesUnchanged':True,'templateDriverSHA256':hashlib.sha256(before.encode()).hexdigest(),'driverSHA256':sha(R/'actual/sustained.mjs'),'runtimeLaunched':False})
shutil.copyfile(prov,R/'package-provenance.json');shutil.copyfile(image,R/'image-provenance.json')
patch=[]
for name,a,b in [('actual/sustained.mjs',before,driver),('launch.py',(T/'launch.py').read_text(),launcher)]:patch.extend(difflib.unified_diff(a.splitlines(True),b.splitlines(True),fromfile='original5f4/'+name,tofile='long-frame299/'+name))
(R/'setup-and-rebind.patch').write_text(''.join(patch))
print(json.dumps({'prepared':str(R),'runtimeLaunched':False,'source':SOURCE,'unchangedActionModules':True,'selectedInputs':len(capture['sources']),'bundleCount':len(bundles)}))
