from pathlib import Path
import json,shutil,hashlib,subprocess,difflib
R=Path(__file__).resolve().parent;NAV=Path('/workspace/.luci-tools/cloud032-navigation-verifier-v1/capsule');CONTROL=Path('/workspace/.luci-tools/cloud032-settled-evaluate-control-v1/capsule-launch-guard-v2');REPO=Path('/workspace/luci');SOURCE='5f4a966f6d70264c92ee2176dde912e1b43fc3c7';sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
assert sha(NAV/'manifest.json')=='38f18bd770dd3bf3c672070ae42cd6453bd5c93ef0038165ad1e3fe3343cdc74';assert sha(CONTROL/'manifest.json')=='d6423f5d170f03169305fcedd27464ce3012563352cce52216cc54dc197dd628'
def put(p,v):p.write_text(json.dumps(v,indent=2)+'\n')
def once(t,a,b):assert t.count(a)==1,a;return t.replace(a,b)
shutil.copytree(NAV/'actual',R/'actual');shutil.copytree(NAV/'controlled-validation',R/'original-controlled-validation')
for n in ['original-callback.js','original-interaction-metrics.mjs','fixture.mjs','controlled.test.mjs','before-only.test.mjs','navigation-verifier.mjs']:shutil.copyfile(NAV/n,R/n)
for n in ['checkout-binding.py','source-binding.mjs','dependency-binding.py','dependency-manifest.json']:shutil.copyfile(CONTROL/'actual'/n,R/'actual'/n)
capture=json.loads((NAV/'original-template-capture.json').read_text());checks=[]
for name,entry in capture['sources'].items():
 data=subprocess.check_output(['git','show',SOURCE+':'+name],cwd=REPO,timeout=10);assert len(data)==entry['bytes'] and hashlib.sha256(data).hexdigest()==entry['sha256'];assert (REPO/name).read_bytes()==data;checks.append(name)
capture.update(capturedHEAD=SOURCE,scope='Exactly17 original accepted helper input files match immutable5f4 and clean working source; external diagnostic callback/source binding separately captured.')
put(R/'template-capture.json',capture);put(R/'actual/template-capture.json',capture)
before=(R/'actual/sustained.mjs').read_text();driver=before;guard=(CONTROL/'actual/sustained.mjs').read_text()
a=driver[driver.index('function verifySourceBinding()'):driver.index('const harnessCheckout=verifySourceBinding();')];b=guard[guard.index('function verifySourceBinding()'):guard.index('const harnessCheckout=verifySourceBinding();')];driver=once(driver,a,b)
a=driver[driver.index('const packageIntegrity = JSON.parse(execFileSync'):driver.index('assert.equal(realpathSync(web),')];b=guard[guard.index('const verifyPackageIntegrity = '):guard.index('assert.equal(realpathSync(web),')];driver=once(driver,a,b)
a="const driverNames=['navigation-verifier.mjs','sustained.mjs',";b="const driverNames=['checkout-binding.py','template-capture.json','dependency-binding.py','dependency-manifest.json','navigation-verifier.mjs','sustained.mjs',";driver=once(driver,a,b)
needle="try{assert(startupScope.snapshot().every(row=>row.state==='closed'&&!row.closeTimedOut),'Startup resource remains unsettled');"
guardStart=guard.index('try{receipt.packageIntegrityAfter=verifyPackageIntegrity();');guardEnd=guard.index("'Startup resource remains unsettled');",guardStart)+len("'Startup resource remains unsettled');");driver=once(driver,needle,guard[guardStart:guardEnd])
driver=once(driver,'assertWorkflowActive();receipt.browserVersion=browser.version();page.setDefaultTimeout(15000);',"assertWorkflowActive();receipt.browserVersion=browser.version();receipt.browserProcessIdentitiesAtSetup=ownedProcesses('browser');page.setDefaultTimeout(15000);")
driver=once(driver,"mode:'installed-four-project-await-boundary-edd0-diagnostic',","mode:'installed-four-project-navigation-callback-5f4-diagnostic',")
(R/'actual/sustained.mjs').write_text(driver)
binding=json.loads((R/'actual/external-source-binding.json').read_text());binding.update(executionDirectory=str(R/'actual'),prospectiveProduct=SOURCE,prospectiveHelper=SOURCE)
binding['navigationVerificationDiagnostic'].update(runtimeBindingStatus='Explicit matching clean5f4 product/helper,106-input76043c image and unchanged original400+32 actions. Root exclusive window required.',runtimeAuthorized=False)
for n in ['checkout-binding.py','template-capture.json','dependency-binding.py','dependency-manifest.json']:binding['localSources'][n]=sha(R/'actual'/n)
for n in binding['localSources']:binding['localSources'][n]=sha(R/'actual'/n)
put(R/'actual/external-source-binding.json',binding)
prov=REPO/'artifacts/quality-swarm/candidate-5f4a966f/package/package-provenance.json';image=Path('/workspace/.luci-tools/sandbox-candidate-20261009T134118Z-4a347241/receipt.json');im=json.loads(image.read_text());assert len(im['sourcesBefore'])==106 and im['sourcesAfter']==im['sourcesBefore']
for n,h in im['sourcesBefore'].items():assert hashlib.sha256(subprocess.check_output(['git','show',SOURCE+':'+n],cwd=REPO,timeout=10)).hexdigest()==h
launcher=(CONTROL/'launch.py').read_text()
for a,b in [('edd0cb121ff6847f681ab6bf41509a497f043d41',SOURCE),('4d6595072888bfa06fed0070d9c63b4cc1e6d1a9',SOURCE),('edd0cb12','5f4a966f'),('Frozen edd0 product / 4d6595 helper binding required','Frozen matching5f4 product and helper binding required'),('5ea084d505e292d418b7ab9b2d3fa97d83a0510158fbb3ef8ef66096603aa5f6',sha(prov)),('Original edd0 provenance bytes required','Original matching5f4 provenance bytes required'),('sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98',im['candidateImage']),('/workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json',str(image)),('5af726e6670fd6f04c0f67cf3b26b3f6072ed89c1e1e3481c652a631ae6d0a3b',sha(image))]:assert a in launcher;launcher=launcher.replace(a,b)
launcher=once(launcher,"    parser.add_argument('--fixture-root',type=Path,default=Path('/workspace/.c32'))", "    parser.add_argument('--trace-after',choices=['yes'],required=True)\n    parser.add_argument('--fixture-root',type=Path,default=Path('/workspace/.c32'))")
launcher=once(launcher,"'--report',str(result)]","'--report',str(result),'--trace-after',args.trace_after]")
launcher=once(launcher,"'primarySwitches':0,'primaryInputs':0,'primaryResets':0,'extraTraceRounds':0,'originalActionCohortExecuted':False,'settledControlCalls':32,'settledControlPhaseSeconds':60,'settledControlCallSeconds':5,", "'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'extraTraceRounds':8,'originalActionCohortExecuted':True,'navigationVerificationResponsesExpected':108,")
(R/'launch.py').write_text(launcher)
cmd=json.loads((CONTROL/'launch-command.json').read_text());argv=cmd['command'];argv[1]=str(R/'launch.py')
for flag,val in [('--source',SOURCE),('--helper-source',SOURCE),('--package','/tmp/luci-retained-tools/package-0.5.0-5f4a966f'),('--provenance',str(prov)),('--image',im['candidateImage']),('--expected-image-id',im['candidateImage']),('--image-provenance',str(image)),('--output','/workspace/luci/artifacts/quality-swarm/idle-5f4a966f-navigation-callback-attempt1')]:argv[argv.index(flag)+1]=val
argv+=['--trace-after','yes'];cmd['scope']='Prepared matching5f4 callback-response diagnostic. Original400+32 cohort/thresholds unchanged. Parent-exclusive runtime window required; no runtime in preparation.';put(R/'launch-command.json',cmd)
patch=[]
for name,before_,after in [('sustained.mjs',before,driver),('launch.py',(CONTROL/'launch.py').read_text(),launcher)]:patch.extend(difflib.unified_diff(before_.splitlines(keepends=True),after.splitlines(keepends=True),fromfile='frozen-before/'+name,tofile='matching5f4/'+name))
(R/'runtime-rebind.patch').write_text(''.join(patch))
put(R/'runtime-binding.json',{'source':SOURCE,'packageProvenanceSHA256':sha(prov),'packageDirectory':json.loads(prov.read_text())['packageDir'],'archiveSHA256':json.loads(prov.read_text())['archiveSHA256'],'image':im['candidateImage'],'imageReceiptSHA256':sha(image),'imageRuntimeInputCount':106,'allImageInputsMatchDeclaredCommit':True,'selectedOriginalHelperInputs':checks,'all17HelperInputsUnchanged':True,'callbackUnchangedFrom11TestCapsule':sha(R/'actual/navigation-verifier.mjs')==sha(NAV/'actual/navigation-verifier.mjs'),'metricModuleUnchangedFrom11TestCapsule':sha(R/'actual/interaction-metrics.mjs')==sha(NAV/'actual/interaction-metrics.mjs'),'originalNavigationSourceCapsuleManifestSHA256':sha(NAV/'manifest.json'),'reusedGuardCapsuleManifestSHA256':sha(CONTROL/'manifest.json'),'runtimeLaunched':False})
shutil.copyfile(prov,R/'package-provenance.json');shutil.copyfile(image,R/'image-provenance.json');print('Matching5f4 runtime candidate assembled; no runtime launched.')
