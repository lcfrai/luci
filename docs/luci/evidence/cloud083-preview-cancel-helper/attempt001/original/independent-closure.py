"""One bounded read-only exact closure; writes only a new external receipt."""
import hashlib,json,os,pathlib,re,subprocess,time
os.umask(0o077)
root=pathlib.Path(__file__).resolve().parent
run=root/'runs/cancel-1791728111119-b08a77ca-70f2-4483-ab3d-dd680b3ab246'
prior=pathlib.Path('/workspace/.luci-tools/cloud080-installed-cancel/runs/cancel-1791727278123-5ac80fb0-871c-44fe-a402-d5533c4e0edd')
out=root/'independent-closure-001';out.mkdir(mode=0o700)
digest=lambda b:hashlib.sha256(b).hexdigest()
def manifest(base):
    values={}
    for p in sorted(base.rglob('*')):
        if p.is_file():
            assert not p.is_symlink()
            raw=p.read_bytes();values[str(p.relative_to(base))]={'bytes':len(raw),'sha256':digest(raw)}
    return values
def write(name,data):
    (out/name).write_text(json.dumps(data,indent=2)+'\n')
def docker(*args):
    x=subprocess.run(['docker','--host','unix:///var/run/docker.sock',*args],capture_output=True,timeout=8)
    assert len(x.stdout)<=2**21 and len(x.stderr)<=16384
    return x
def identity(pid):
    try:
        raw=pathlib.Path('/proc',str(pid),'stat').read_text();fields=raw[raw.rfind(')')+2:].split();return {'startTicks':fields[19],'state':fields[0],'parentPID':int(fields[1])}
    except FileNotFoundError:return None
before=manifest(run/'private');prior_before=manifest(prior/'private')
write('private-files-before.private.json',before)
original_manifest=json.loads(pathlib.Path('/workspace/.luci-tools/cloud080-installed-cancel/independent-closure-001/private-files-before.private.json').read_text())
assert prior_before==original_manifest and len(prior_before)==58
result=json.loads((run/'result.json').read_text());saved=json.loads((run/'owned-process-closure.json').read_text())
processes=[]
for row in saved['processes']:
    current=identity(row['pid']);status='absent' if current is None else 'different-process' if current['startTicks']!=row['startTicks'] else 'still-present'
    processes.append({'pid':row['pid'],'startTicks':row['startTicks'],'status':status,'current':current})
engine=docker('info','--format','{{.ID}}');assert engine.returncode==0 and engine.stdout.decode().strip()=='ff40514f-d7cc-4ee7-8a1a-1cfafaeaf259'
exact=[]
for cid in ['6835128545d6bf8cb799b53a51f430446f6891f6d6c61b3644cfc979705bc5f7','649369e7c13e8c7352388e79f194a68dc06daa7c7e9a13e9e04de4688f595b19']:
    x=docker('inspect',cid);absent=x.returncode==1 and x.stdout.strip()==b'[]' and x.stderr.decode().strip()=='Error: No such object: '+cid
    exact.append({'immutableID':cid,'absent':absent,'exitCode':x.returncode,'stdoutBytes':len(x.stdout),'stdoutSHA256':digest(x.stdout),'stderrBytes':len(x.stderr),'stderrSHA256':digest(x.stderr)})
inventories={}
for name,args in [('containers',('ps','-a','-q','--no-trunc')),('volumes',('volume','ls','-q')),('images',('image','ls','-q','--no-trunc'))]:
    x=docker(*args);assert x.returncode==0;values=sorted(set(x.stdout.decode().splitlines()));inventories[name]={'count':len(values),'values':values if name!='volumes' else None,'namesSHA256':digest(x.stdout)}
leases=[]
for label,base in [('original',prior),('c083-attempt001',run)]:
    path=base/'private/state/control-plane/execution/leases.json';raw=path.read_bytes();doc=json.loads(raw);expected='preview-build-be6b957056fb49cba06e18f3' if label=='original' else 'preview-build-0e13e03053a1eb9d7770a7b9';rows=[x for x in doc.get('previews',[]) if x['id']==expected];assert len(rows)==1
    row=rows[0];leases.append({'fixture':label,'ledgerSHA256':digest(raw),'id':row['id'],'kind':row['kind'],'state':row['state'],'resources':row['resources'],'revision':row['revision'],'held':True})
after=manifest(run/'private');prior_after=manifest(prior/'private');assert before==after and prior_before==prior_after
receipt={'atUnix':time.time(),'scope':'Independent bounded read-only exact observed controller/container and selected daemon; no cleanup or lease mutation, no whole-machine process closure claim','runResultSHA256':digest((run/'result.json').read_bytes()),'wrapperResultSHA256':digest((root/'wrapper-001/result.json').read_bytes()),'controllerExit':result['controllerExit'],'controllerEscalationAttempted':result['cleanup'].get('controllerEscalationAttempted',False),'processes':processes,'exactContainers':exact,'engineID':engine.stdout.decode().strip(),'daemon':inventories,'leases':leases,'currentPrivateFiles':len(before),'currentPrivateFilesUnchanged':before==after,'priorPrivateFiles':len(prior_before),'prior58FilesMatchOriginalClosure':prior_before==original_manifest,'priorPrivateFilesUnchanged':prior_before==prior_after,'noHTTPCancelSent':not result['cleanup']['cancelRequestSent'],'startRequestUnconfirmed':not result['cleanup']['startRequestAcknowledged'],'pendingMutationCount':len(result['pendingMutations']),'archiveGETObserved':(run/'proof-archive-observations.json').exists(),'scenarioPassed':result['scenarioPassed'],'overallPassed':result['passed'],'zeroChargeClaim':False,'manualDockerMutation':False,'manualLedgerMutation':False}
write('closure.json',receipt);print(json.dumps(receipt))
