#!/usr/bin/env python3
"""Launch only when root provides the clean matching source/package in an exclusive window."""
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

def controlled_environment(inherited, endpoint):
    assert isinstance(endpoint,str) and re.fullmatch(r'unix:///[^\x00-\x20\x7f?#]+', endpoint), 'Explicit absolute local Unix Docker endpoint required'
    result=dict(inherited)
    for key in ('DOCKER_CONTEXT','DOCKER_TLS_VERIFY','DOCKER_CERT_PATH','DOCKER_TLS','DOCKER_API_VERSION'):
        result.pop(key,None)
    result.update(DOCKER_HOST=endpoint,LUCI_ACCEPTANCE_REPO=str(REPO))
    return result

def docker_command(values,env):
    return command(['docker','--host',env['DOCKER_HOST'],*values],env=env)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',required=True)
    parser.add_argument('--docker-host',required=True)
    parser.add_argument('--window-granted',action='store_true')
    parser.add_argument('--helper-source')
    parser.add_argument('--package',type=Path,required=True)
    parser.add_argument('--provenance',type=Path,required=True)
    parser.add_argument('--image',required=True)
    parser.add_argument('--expected-image-id',required=True)
    parser.add_argument('--image-provenance',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--fixture-root',type=Path,default=Path('/workspace/.c32'))
    parser.add_argument('--trace-after',choices=['yes','no'],default='yes')
    args=parser.parse_args();helper=args.helper_source or args.source
    assert args.window_granted,'Parent-coordinated exclusive runtime window required'
    runtime_env=controlled_environment(os.environ,args.docker_host)
    docker=lambda values:docker_command(values,runtime_env)
    assert args.source=='edd0cb121ff6847f681ab6bf41509a497f043d41' and helper=='8c7bf16aa035b27dbea185b565052495eb60777f','Frozen edd0 product / 8c7 helper binding required'
    assert args.trace_after=='yes','Original complete 400 + 32 action counts required'
    assert args.package.resolve()==Path('/tmp/luci-retained-tools/package-0.5.0-edd0cb12').resolve()
    assert args.provenance.resolve()==Path('/workspace/luci/artifacts/quality-swarm/candidate-edd0cb12/package/package-provenance.json').resolve()
    assert sha(args.provenance)=='5ea084d505e292d418b7ab9b2d3fa97d83a0510158fbb3ef8ef66096603aa5f6','Original edd0 provenance bytes required'
    assert args.expected_image_id=='sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98' and args.image==args.expected_image_id
    assert args.image_provenance.resolve()==Path('/workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json').resolve()
    assert sha(args.image_provenance)=='5af726e6670fd6f04c0f67cf3b26b3f6072ed89c1e1e3481c652a631ae6d0a3b','Exact retained image evidence required'
    assert re.fullmatch('[0-9a-f]{40}',args.source) and re.fullmatch('[0-9a-f]{40}',helper),'Exact immutable source revisions required'
    assert re.fullmatch('sha256:[0-9a-f]{64}',args.expected_image_id)
    assert os.getuid()!=0 and sys.platform=='linux'
    assert command(['git','rev-parse','HEAD'],cwd=REPO)==helper
    assert command(['git','status','--porcelain'],cwd=REPO)=='','Clean committed source required'
    capture=json.loads((HERE/'template-capture.json').read_text())
    for name,expected in capture['sources'].items():
        path=REPO/name;assert path.is_file() and sha(path)==expected['sha256'] and path.stat().st_size==expected['bytes'],name+' changed after preparation'
        committed=subprocess.check_output(['git','show',helper+':'+name],cwd=REPO,timeout=30)
        assert hashlib.sha256(committed).hexdigest()==expected['sha256'],name+' not bound to exact helper revision'
    binding=json.loads((HERE/'actual/external-source-binding.json').read_text())
    assert binding['executionDirectory']==str(HERE/'actual')
    for name,expected in binding['localSources'].items():assert sha(HERE/'actual'/name)==expected,name
    for name,expected in binding['repositoryDependencies'].items():assert sha(REPO/name)==expected,name
    provenance=json.loads(args.provenance.read_text())
    assert provenance['sourceCommit']==args.source and provenance['sourceDirty'] is False
    package=args.package.resolve();assert sha(package/'luci')==provenance['binarySHA256'] and sha(package/'web/index.html')==provenance['webIndexSHA256']
    # Driver also verifies the entire archive/extracted package before launching.
    assert docker(['ps','-aq'])=='','Exclusive Docker must be empty; never run on shared Windows Docker'
    assert docker(['volume','ls','-q'])==''
    assert json.loads(docker(['image','inspect',args.image]))[0]['Id']==args.expected_image_id
    engine=docker(['info','--format','{{.ID}}']);assert engine
    volume=os.statvfs('/');free=volume.f_bavail*volume.f_frsize;assert free>=6*1024**3,'Unchanged six GiB root floor'
    output=args.output.resolve();assert not output.exists(),'Fresh output directory required; failed evidence must remain'
    output.mkdir(mode=0o700,parents=True)
    result_dir=output/'driver';result=result_dir/'result.json'
    cmd=['node',str(HERE/'actual/sustained.mjs'),'--task-context-version','2','--binary',str(package/'luci'),'--web',str(package/'web'),'--provenance',str(args.provenance.resolve()),'--expected-source',args.source,'--expected-helper-source',helper,'--image',args.image,'--expected-image-id',args.expected_image_id,'--image-provenance',str(args.image_provenance.resolve()),'--fixture-root',str(args.fixture_root.resolve()),'--report',str(result),'--trace-after',args.trace_after]
    invocation={'version':1,'preparedAt':now(),'command':cmd,'productSource':args.source,'helperSource':helper,'sourceBinding':capture,'driverSHA256':sha(HERE/'actual/sustained.mjs'),'bindingSHA256':sha(HERE/'actual/external-source-binding.json'),'launcherSHA256':sha(Path(__file__)),'provenanceSHA256':sha(args.provenance),'environment':{'LUCI_ACCEPTANCE_REPO':str(REPO),'DOCKER_HOST':args.docker_host,'DockerContextTLSOverridesRemoved':True,'DockerCLIHostExplicit':True},'scope':'Fresh installed idle diagnostic only; no runtime allocation, no full sustained pass, no model quality or Windows acceptance.','fixedScope':{'quietSeconds':60,'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'extraTraceRounds':8 if args.trace_after=='yes' else 0,'switchP95Ms':300,'inputP95Ms':100,'p99StrictlyBelowMs':1000,'allowedCreates':0,'allowedStarts':0,'preparationSeconds':60,'workflowSeconds':210,'cleanupSeconds':120,'outerDriverWaitSeconds':450,'outerTermGraceSeconds':30,'outerKillWaitSeconds':10,'linuxSampler':False},'engineIdentitySHA256':hashlib.sha256(engine.encode()).hexdigest(),'rootFreeBytes':free,'rawEvidence':'Retain result interactions and node/browser spans verbatim. Raw trace/private fixture may contain context; publish only separately bound allowlisted projections.'}
    write(output/'invocation.json',invocation)
    env=runtime_env;start=time.monotonic();started=now();timeout=False;forced=False;events=[]
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
    receipt={'startedAt':started,'finishedAt':now(),'elapsedSeconds':time.monotonic()-start,'exitCode':child.poll(),'externalMonitorTimeout':timeout,'forcedExactDriver':forced,'driverIdentity':driver,'finalDriverObservation':identity(child.pid),'events':events,'ownedCleanupRequiresFinalReceipt':True,'cleanupUnconfirmedByWrapper':True,'resultExists':result.is_file(),'rawResultSHA256':sha(result) if result.is_file() else None,'noAutomaticFixtureOrLedgerMutation':True}
    write(output/'wrapper-result.json',receipt);print(json.dumps(receipt),flush=True)
    return 124 if timeout else child.returncode if child.returncode is not None else 1
if __name__=='__main__':sys.exit(main())
