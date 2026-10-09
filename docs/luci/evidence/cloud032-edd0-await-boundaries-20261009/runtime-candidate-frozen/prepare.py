#!/usr/bin/env python3
"""Source-only integration; never invokes Docker, app, browser or benchmark."""
from pathlib import Path
import difflib,hashlib,json,shutil,subprocess
HERE=Path(__file__).resolve().parent
BASE=Path('/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/capsule')
MARKS=Path('/workspace/.luci-tools/cloud032-await-boundary-diagnostic-v1/capsule')
REPO=Path('/workspace/luci')
PRODUCT='edd0cb121ff6847f681ab6bf41509a497f043d41'
HELPER='8c7bf16aa035b27dbea185b565052495eb60777f'
PROVENANCE=REPO/'artifacts/quality-swarm/candidate-edd0cb12/package/package-provenance.json'
IMAGE=Path('/workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json')
IMAGEID='sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98'
PACKAGE=Path('/tmp/luci-retained-tools/package-0.5.0-edd0cb12')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def put(name,value):(HERE/name).write_text(json.dumps(value,indent=2)+'\n')
def once(source,before,after):
 assert source.count(before)==1,before
 return source.replace(before,after)
def git(*args):return subprocess.check_output(['git',*args],cwd=REPO,timeout=15)
def main():
 assert git('rev-parse','HEAD').decode().strip()==HELPER and git('status','--porcelain')==b''
 assert sha(BASE/'manifest.json')=='789ecae04845d42c32378143e3013fac778d3d91c963899c2219e60cc656c9e0'
 assert sha(MARKS/'manifest.json')=='626002af523552bf4059a1c08aa5ab3a1a51a9b83794f6807350ee8e90a74281'
 shutil.copytree(BASE/'actual',HERE/'actual')
 shutil.copytree(MARKS,HERE/'mark-candidate')
 for name in ['interaction-metrics.mjs','latency-diagnostics.mjs']:shutil.copyfile(MARKS/'candidate'/name,HERE/'actual'/name)
 capture=json.loads((BASE/'template-capture.json').read_text())
 for name,expected in capture['sources'].items():
  data=git('show',HELPER+':'+name);assert hashlib.sha256(data).hexdigest()==expected['sha256'] and len(data)==expected['bytes'];assert sha(REPO/name)==expected['sha256']
 capture['capturedHEAD']=HELPER;capture['scope']='All 17 unchanged helper inputs rechecked against exact clean 8c7 committed bytes. External marker modules are separately bound, not represented as committed originals.';put('template-capture.json',capture)
 before=(BASE/'actual/sustained.mjs').read_text();driver=before
 driver=once(driver,"mode:'installed-four-project-autonomous-readiness-v2-diagnostic',","mode:'installed-four-project-await-boundary-edd0-diagnostic',")
 driver=once(driver,"const sourceBindingOptions=parseSourceBindingOptions(args);","const sourceBindingOptions=parseSourceBindingOptions(args);\nassert.equal(sourceBindingOptions.expectedProduct,externalBinding.prospectiveProduct);assert.equal(sourceBindingOptions.expectedHelper,externalBinding.prospectiveHelper);assert.equal(provenance.sourceCommit,externalBinding.prospectiveProduct);")
 driver=once(driver,"const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: Math.ceil(commandBudget(30_000)) }).trim();", "const dockerEndpoint=process.env.DOCKER_HOST;assert(typeof dockerEndpoint==='string'&&dockerEndpoint.startsWith('unix:///')&&dockerEndpoint.length>8&&!/[\\x00-\\x20\\x7f?#]/.test(dockerEndpoint));assert.equal(process.env.DOCKER_CONTEXT,undefined);\nconst docker = (...values) => execFileSync('docker', ['--host',dockerEndpoint,...values], { encoding: 'utf8', timeout: Math.ceil(commandBudget(30_000)) }).trim();")
 driver=once(driver,"const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));","const upstream=dockerEndpoint;")
 (HERE/'actual/sustained.mjs').write_text(driver)
 binding=json.loads((BASE/'actual/external-source-binding.json').read_text());binding.update(executionDirectory=str(HERE/'actual'),prospectiveProduct=PRODUCT,prospectiveHelper=HELPER,scope='Explicit edd0 package / 8c7 helper diagnostic. V2 observer and three Node await-boundary marks; no primary action, cap, threshold, guard or lifecycle changes. Docker CLI/proxy endpoint is explicit and common.',priorV2ManifestSHA256=sha(BASE/'manifest.json'),markCandidateManifestSHA256=sha(MARKS/'manifest.json'),nodeAwaitBoundaryMarks=['open-project-click-start','idea-field-wait-end','idea-context-evaluate-end'],nodeStageCap=12)
 for name in binding['localSources']:binding['localSources'][name]=sha(HERE/'actual'/name)
 put('actual/external-source-binding.json',binding)
 original=(BASE/'launch.py').read_text();launcher=original
 launcher=once(launcher,"def main():",'''def controlled_environment(inherited, endpoint):
    assert isinstance(endpoint,str) and re.fullmatch(r'unix:///[^\\x00-\\x20\\x7f?#]+', endpoint), 'Explicit absolute local Unix Docker endpoint required'
    result=dict(inherited)
    for key in ('DOCKER_CONTEXT','DOCKER_TLS_VERIFY','DOCKER_CERT_PATH','DOCKER_TLS','DOCKER_API_VERSION'):
        result.pop(key,None)
    result.update(DOCKER_HOST=endpoint,LUCI_ACCEPTANCE_REPO=str(REPO))
    return result

def docker_command(values,env):
    return command(['docker','--host',env['DOCKER_HOST'],*values],env=env)

def main():''')
 launcher=once(launcher,"parser.add_argument('--source',required=True)","parser.add_argument('--source',required=True)\n    parser.add_argument('--docker-host',required=True)\n    parser.add_argument('--window-granted',action='store_true')")
 launcher=once(launcher,"args=parser.parse_args();helper=args.helper_source or args.source", "args=parser.parse_args();helper=args.helper_source or args.source\n    assert args.window_granted,'Parent-coordinated exclusive runtime window required'\n    runtime_env=controlled_environment(os.environ,args.docker_host)\n    docker=lambda values:docker_command(values,runtime_env)")
 launcher=once(launcher,"assert args.source=='d20ebd292f3f1de04d6726bab0998c210987bcb5','Frozen prospective product is d20; no silent package rebase'",f"assert args.source=='{PRODUCT}' and helper=='{HELPER}','Frozen edd0 product / 8c7 helper binding required'\n    assert args.trace_after=='yes','Original complete 400 + 32 action counts required'\n    assert args.package.resolve()==Path('{PACKAGE}').resolve()\n    assert args.provenance.resolve()==Path('{PROVENANCE}').resolve()\n    assert sha(args.provenance)=='{sha(PROVENANCE)}','Original edd0 provenance bytes required'\n    assert args.expected_image_id=='{IMAGEID}' and args.image==args.expected_image_id\n    assert args.image_provenance.resolve()==Path('{IMAGE}').resolve()\n    assert sha(args.image_provenance)=='{sha(IMAGE)}','Exact retained image evidence required'")
 for old,new in [("command(['docker','ps','-aq'])","docker(['ps','-aq'])"),("command(['docker','volume','ls','-q'])","docker(['volume','ls','-q'])"),("command(['docker','image','inspect',args.image])","docker(['image','inspect',args.image])"),("command(['docker','info','--format','{{.ID}}'])","docker(['info','--format','{{.ID}}'])")]:launcher=once(launcher,old,new)
 launcher=once(launcher,"'environment':{'LUCI_ACCEPTANCE_REPO':str(REPO)}", "'environment':{'LUCI_ACCEPTANCE_REPO':str(REPO),'DOCKER_HOST':args.docker_host,'DockerContextTLSOverridesRemoved':True,'DockerCLIHostExplicit':True}")
 launcher=once(launcher,"env=dict(os.environ,LUCI_ACCEPTANCE_REPO=str(REPO));", "env=runtime_env;")
 (HERE/'launch.py').write_text(launcher)
 patches=[]
 for name,before,after in [('sustained.mjs',before,driver),('launch.py',original,launcher)]:patches.extend(difflib.unified_diff(before.splitlines(keepends=True),after.splitlines(keepends=True),fromfile='sealed-v2/'+name,tofile='edd0-await/'+name))
 (HERE/'integration.patch').write_text(''.join(patches))
 provenance=json.loads(PROVENANCE.read_text());assert provenance['sourceCommit']==PRODUCT and provenance['sourceDirty'] is False and Path(provenance['packageDir']).resolve()==PACKAGE.resolve()
 image=json.loads(IMAGE.read_text());assert image['candidateImage']==IMAGEID and image['sourcesBefore']==image['sourcesAfter'] and len(image['sourcesBefore'])==104 and image['exportVerification']['status']=='passed'
 reviewpath=Path('/workspace/.luci-tools/cloud054-terminal-close/outer-v2/image-source-review.json');review=json.loads(reviewpath.read_text());preferred=review['preferredCandidate'];assert review['targetSourceCommit']==PRODUCT and preferred['sha256']==sha(IMAGE) and preferred['all104SourcesMatch'] is True
 shutil.copyfile(PROVENANCE,HERE/'package-provenance.original.json')
 put('image-source-binding.json',{'scope':'Exact unchanged retained image receipt plus already completed edd0 source review. No Docker inspection/rebuild/load/export performed in preparation. Driver repeats source/input/current-daemon checks at launch.','receiptSHA256':sha(IMAGE),'receiptBytes':IMAGE.stat().st_size,'candidateImage':IMAGEID,'sourceCount':104,'sourceReviewSHA256':sha(reviewpath),'targetSource':PRODUCT,'all104Match':True,'historicalExportVerification':image['exportVerification']['status']})
 put('preparation.json',{'productSource':PRODUCT,'helperSource':HELPER,'cleanHelper':True,'sourceInputCount':len(capture['sources']),'installedRun':False,'packageProvenanceSHA256':sha(PROVENANCE),'packageArchiveSHA256':provenance['archiveSHA256'],'packageBytesNotRehashedInPreparation':True,'driverSHA256':sha(HERE/'actual/sustained.mjs'),'bindingSHA256':sha(HERE/'actual/external-source-binding.json'),'launcherSHA256':sha(HERE/'launch.py'),'sameV2Observer':sha(HERE/'actual/autonomous-readiness.mjs')==sha(BASE/'actual/autonomous-readiness.mjs'),'fullPrimaryAndTraceActionsUnchanged':True,'all17RepositoryInputsUnchanged':True})
 command=['python3',str(HERE/'launch.py'),'--source',PRODUCT,'--helper-source',HELPER,'--package',str(PACKAGE),'--provenance',str(PROVENANCE),'--image',IMAGEID,'--expected-image-id',IMAGEID,'--image-provenance',str(IMAGE),'--docker-host','unix:///var/run/docker.sock','--fixture-root','/workspace/.c32','--output','/workspace/luci/artifacts/quality-swarm/idle-edd0cb12-await-boundary-attempt1','--trace-after','yes','--window-granted']
 put('launch-command.json',{'scope':'Prepared only; parent must coordinate exclusive runtime window before invocation.','command':command})
 print(json.dumps({'prepared':True,'helper':HELPER,'product':PRODUCT,'sources':len(capture['sources']),'driverSHA256':sha(HERE/'actual/sustained.mjs'),'launcherSHA256':sha(HERE/'launch.py'),'installedRun':False}))
if __name__=='__main__':main()
