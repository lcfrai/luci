#!/usr/bin/env python3
"""Root-only bounded diagnostic: frozen packaged binary plus separately bound numeric-probe web."""
from pathlib import Path
import argparse, datetime, hashlib, json, os, re, signal, subprocess, sys, time

HERE=Path(__file__).resolve().parent
REPO=Path('/workspace/luci')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def now(): return datetime.datetime.now(datetime.timezone.utc).isoformat()
def write(path,value): path.write_text(json.dumps(value,indent=2)+'\n')
def command(values,**kw): return subprocess.check_output(values,text=True,timeout=30,**kw).strip()
def identity(pid):
    try:
        fields=Path('/proc',str(pid),'stat').read_text().rsplit(') ',1)[1].split()
        return {'pid':pid,'startTicks':int(fields[19]),'state':fields[0]}
    except FileNotFoundError:return {'pid':pid,'state':'absent'}
    except (PermissionError,OSError,ValueError,IndexError) as e:return {'pid':pid,'state':'unknown','errorType':type(e).__name__}
def same(current,expected): return current.get('startTicks')==expected.get('startTicks') and current['state'] not in ['absent','unknown']

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--contract-receipt',type=Path,default=HERE/'contract-check/result.json')
    parser.add_argument('--source',required=True)
    parser.add_argument('--helper-source')
    parser.add_argument('--package',type=Path,required=True)
    parser.add_argument('--provenance',type=Path,required=True)
    parser.add_argument('--image',required=True)
    parser.add_argument('--expected-image-id',required=True)
    parser.add_argument('--image-provenance',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--fixture-root',type=Path,default=Path('/workspace/.c32'))
    parser.add_argument('--trace-after',choices=['no'],default='no')
    args=parser.parse_args();helper=args.helper_source or args.source
    assert re.fullmatch('[0-9a-f]{40}',args.source) and re.fullmatch('[0-9a-f]{40}',helper),'Exact immutable source revisions required'
    assert re.fullmatch('sha256:[0-9a-f]{64}',args.expected_image_id)
    assert os.getuid()!=0 and sys.platform=='linux'
    assert command(['node','--version'])=='v24.19.0'
    manifest=json.loads((HERE/'prepared-files.json').read_text())
    dependencies=json.loads((HERE/'dependency-bindings.json').read_text())['files']
    for name,expected in dependencies.items():assert sha(REPO/'desktop/node_modules'/name)==expected,'Pinned dependency changed: '+name
    controlled=json.loads(args.contract_receipt.read_text())
    assert controlled['exitCode']==0 and controlled['inputsUnchanged'] and not controlled['timedOut'],'Controlled diagnostic contract checks required'
    assert controlled['preparedManifestSHA256']==sha(HERE/'prepared-files.json'),'Contract checks do not bind current preparation'
    for row in manifest['files']:
        p=HERE/row['path'];assert p.is_file() and p.stat().st_size==row['bytes'] and sha(p)==row['sha256'],row['path']+' prepared input changed'
    assert args.source=='577f7a0fdb039b7f875901a6a567d62fdfee88c8' and helper=='64829123a5799858b57616b711c84c61b945871a'
    observed={'head':command(['git','rev-parse','HEAD'],cwd=REPO),'statusPorcelain':command(['git','status','--porcelain'],cwd=REPO),'usedForProductOrHelpers':False}
    capture=json.loads((HERE/'template-capture.json').read_text())
    for name,expected in capture['sources'].items():
        path=HERE/'frozen-helpers'/name;assert path.is_file() and sha(path)==expected['sha256'] and path.stat().st_size==expected['bytes'],name+' frozen helper changed'
        committed=subprocess.check_output(['git','show',helper+':'+name],cwd=REPO,timeout=30)
        assert hashlib.sha256(committed).hexdigest()==expected['sha256'],name+' not bound to exact helper revision'
    binding=json.loads((HERE/'actual/external-source-binding.json').read_text())
    assert binding['executionDirectory']==str(HERE/'actual') and binding['helperSourceCommit']==helper
    assert binding['dependencyInputs']==dependencies,'Dependency binding differs'
    for name,expected in binding['localSources'].items():assert sha(HERE/'actual'/name)==expected,name
    for name,expected in binding['repositoryDependencies'].items():assert sha(HERE/'frozen-helpers'/name)==expected,name
    diagnostic_path=HERE/'diagnostic-web-binding.json';diagnostic=json.loads(diagnostic_path.read_text())
    assert diagnostic['diagnosticOnly'] is True and diagnostic['baseProductCommit']==args.source and diagnostic['helperSourceCommit']==helper
    assert diagnostic['overlayBindingSHA256']==sha(HERE/'overlay-binding.json')
    web=HERE/'web';assert diagnostic['webRoot']==str(web.resolve())
    found=[]
    for p in sorted(web.rglob('*')):
        assert not p.is_symlink(),'Diagnostic web symlinks refused'
        if p.is_file():found.append({'path':str(p.relative_to(web)),'bytes':p.stat().st_size,'sha256':sha(p)})
    assert found==diagnostic['webMembers'] and found
    assert sha(web/'index.html')==diagnostic['webIndexSHA256']
    provenance=json.loads(args.provenance.read_text())
    assert provenance['sourceCommit']==args.source and provenance['sourceDirty'] is False
    package=args.package.resolve();assert sha(package/'luci')==provenance['binarySHA256'] and sha(package/'web/index.html')==provenance['webIndexSHA256']
    # Driver also verifies the entire archive/extracted package before launching.
    assert command(['docker','ps','-aq'])=='','Exclusive Docker must be empty; never run on shared Windows Docker'
    assert command(['docker','volume','ls','-q'])==''
    assert json.loads(command(['docker','image','inspect',args.image]))[0]['Id']==args.expected_image_id
    engine=command(['docker','info','--format','{{.ID}}']);assert engine
    volume=os.statvfs('/');free=volume.f_bavail*volume.f_frsize;assert free>=6*1024**3,'Unchanged six GiB root floor'
    output=args.output.resolve();assert not output.exists(),'Fresh output directory required; failed evidence must remain'
    output.mkdir(mode=0o700,parents=True)
    result_dir=output/'driver';result=result_dir/'result.json'
    cmd=['node',str(HERE/'actual/sustained.mjs'),'--task-context-version','2','--binary',str(package/'luci'),'--web',str(web),'--diagnostic-web-binding',str(diagnostic_path),'--diagnostic-web-binding-sha256',sha(diagnostic_path),'--provenance',str(args.provenance.resolve()),'--expected-source',args.source,'--expected-helper-source',helper,'--image',args.image,'--expected-image-id',args.expected_image_id,'--image-provenance',str(args.image_provenance.resolve()),'--fixture-root',str(args.fixture_root.resolve()),'--report',str(result),'--trace-after',args.trace_after]
    invocation={'version':1,'preparedAt':now(),'command':cmd,'productSource':args.source,'helperSource':helper,'sourceBinding':capture,'observedMutableCheckout':observed,'frozenHelperCopyUsed':True,'diagnosticWebBindingSHA256':sha(diagnostic_path),'diagnosticWebIndexSHA256':diagnostic['webIndexSHA256'],'packagedWebIndexSHA256':provenance['webIndexSHA256'],'driverSHA256':sha(HERE/'actual/sustained.mjs'),'bindingSHA256':sha(HERE/'actual/external-source-binding.json'),'launcherSHA256':sha(Path(__file__)),'provenanceSHA256':sha(args.provenance),'environment':{'LUCI_ACCEPTANCE_REPO':str(REPO),'GOMAXPROCS':'2','NODE_OPTIONS':'--max-old-space-size=3072'},'scope':'Frozen installed binary with separately built diagnostic web; full100-round numeric Home/destination diagnostic only; no runtime allocation, no full sustained pass, no model quality or Windows acceptance.','fixedScope':{'quietSeconds':60,'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'extraTraceRounds':0,'homePhaseNumericOnly':True,'destinationPhaseNumericOnly':True,'locatorPollAttemptsObserved':False,'performanceAcceptance':False,'switchP95Ms':300,'inputP95Ms':100,'p99StrictlyBelowMs':1000,'allowedCreates':0,'allowedStarts':0,'preparationSeconds':60,'workflowSeconds':210,'cleanupSeconds':120,'outerDriverWaitSeconds':450,'outerTermGraceSeconds':30,'outerKillWaitSeconds':10,'linuxSampler':False},'engineIdentitySHA256':hashlib.sha256(engine.encode()).hexdigest(),'rootFreeBytes':free,'dependencyBindingsSHA256':sha(HERE/'dependency-bindings.json'),'contractCheckResultSHA256':sha(args.contract_receipt),'rawEvidence':'Retain result interactions and node/browser spans verbatim. Original raw evidence/private fixture may contain context; added Home phase fields are numeric only; publish only separately bound allowlisted projections.'}
    write(output/'invocation.json',invocation)
    env=dict(os.environ,LUCI_ACCEPTANCE_REPO=str(REPO),GOMAXPROCS='2',NODE_OPTIONS='--max-old-space-size=3072');start=time.monotonic();started=now();timeout=False;forced=False;events=[]
    with (output/'driver.stdout.log').open('w') as stdout,(output/'driver.stderr.log').open('w') as stderr:
        child=subprocess.Popen(cmd,cwd=REPO,env=env,stdout=stdout,stderr=stderr,start_new_session=True)
        driver=identity(child.pid);assert 'startTicks' in driver,'Owned driver identity unavailable'
        write(output/'driver-identity.json',driver)
        deadline=time.monotonic()+450
        while child.poll() is None and time.monotonic()<deadline:
            try:child.wait(timeout=min(30,max(.001,deadline-time.monotonic())))
            except subprocess.TimeoutExpired:pass
        if child.poll() is None:
            timeout=True;events.append({'event':'outer-timeout','at':now(),'identity':identity(child.pid)})
            # Fail durably BEFORE any further wait or signal. Never certify resource cleanup here.
            write(output/'wrapper-result.json',{'status':'outer-timeout','passed':False,'driverIdentity':driver,'events':events,'cleanupUnconfirmed':True})
            if same(identity(child.pid),driver):
                child.send_signal(signal.SIGTERM);events.append({'event':'TERM-exact-driver','at':now()})
                try:child.wait(timeout=30)
                except subprocess.TimeoutExpired:
                    if same(identity(child.pid),driver):
                        forced=True;child.kill();events.append({'event':'KILL-exact-driver-only','at':now()})
                        try:child.wait(timeout=10)
                        except subprocess.TimeoutExpired:events.append({'event':'driver-remains-unsettled','at':now()})
    receipt={'startedAt':started,'finishedAt':now(),'elapsedSeconds':time.monotonic()-start,'exitCode':child.poll(),'externalMonitorTimeout':timeout,'forcedExactDriver':forced,'driverIdentity':driver,'finalDriverObservation':identity(child.pid),'events':events,'ownedCleanupRequiresFinalReceipt':True,'cleanupUnconfirmedByWrapper':True,'resultExists':result.is_file(),'rawResultSHA256':sha(result) if result.is_file() else None,'preparedInputsUnchanged':all((HERE/row['path']).is_file() and sha(HERE/row['path'])==row['sha256'] for row in manifest['files']),'observedMutableCheckoutAfter':{'head':command(['git','rev-parse','HEAD'],cwd=REPO),'statusPorcelain':command(['git','status','--porcelain'],cwd=REPO),'usedForProductOrHelpers':False},'dependenciesUnchanged':all(sha(REPO/'desktop/node_modules'/name)==expected for name,expected in dependencies.items()),'noAutomaticFixtureOrLedgerMutation':True}
    write(output/'wrapper-result.json',receipt);print(json.dumps(receipt),flush=True)
    return 124 if timeout else 1 if not receipt['preparedInputsUnchanged'] or not receipt['dependenciesUnchanged'] else child.returncode if child.returncode is not None else 1
if __name__=='__main__':sys.exit(main())
