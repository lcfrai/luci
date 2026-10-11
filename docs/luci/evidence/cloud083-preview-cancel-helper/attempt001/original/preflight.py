"""Read-only source/package/daemon/headroom preflight; never launches the app."""
import datetime,hashlib,json,os,pathlib,shutil,subprocess,sys
os.umask(0o077)
root=pathlib.Path(__file__).resolve().parent
cfg=json.loads((root/'run-config-prepared.json').read_text())
out=root/sys.argv[1]
assert out.parent==root and not out.exists()
digest=lambda p:hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
def command(argv):
    completed=subprocess.run(argv,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=12,check=True)
    assert len(completed.stdout)<2<<20
    return completed.stdout.decode().strip()
def docker(*args):return command(['docker','--host',cfg['dockerHost'],*args])
record={'scope':'Read-only static package, current daemon inventory and storage. No application/build/container/ledger mutation.','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'configurationSHA256':digest(root/'run-config-prepared.json'),'checks':{},'refusals':[]}
try:
    checks=record['checks']
    checks['binaryMatches']=digest(cfg['binary'])==cfg['binarySHA256']
    checks['provenanceMatches']=digest(cfg['provenanceFile'])==cfg['provenanceSHA256']
    checks['packageIdentityMatches']=(pathlib.Path(cfg['binary']).parent/'package-identity').read_text().strip()==cfg['packageIdentity']
    checks['sourceHEADMatches']=command(['git','-C',cfg['repository'],'rev-parse','HEAD'])==cfg['sourceCommit']
    checks['sourceClean']=not command(['git','-C',cfg['repository'],'status','--porcelain=v1','--untracked-files=normal'])
    info=json.loads(docker('info','--format','{{json .}}'))
    checks['engineMatches']=info['ID']==cfg['engineID'] and info['Driver']==cfg['engineDriver'] and info['OSType']=='linux'
    base=json.loads(docker('image','inspect',cfg['baseImageID']))[0]
    checks['baseMatches']=base['Id']==cfg['baseImageID'] and base['Size']>0 and not base['Config'].get('OnBuild') and not base['Config'].get('Volumes')
    images=sorted(set(docker('image','ls','-q','--no-trunc').splitlines()))
    containers=docker('ps','-a','-q','--no-trunc').splitlines()
    volumes=docker('volume','ls','-q').splitlines()
    record['daemon']={'engineID':info['ID'],'driver':info['Driver'],'images':images,'containerCount':len(containers),'containerIDsSHA256':hashlib.sha256('\n'.join(containers).encode()).hexdigest(),'volumeCount':len(volumes),'volumeNamesSHA256':hashlib.sha256('\n'.join(volumes).encode()).hexdigest()}
    checks['baselineMatches']=images==sorted(cfg['expectedImageIDs']) and len(set(cfg['expectedImageIDs']))==10 and len(containers)==0 and len(volumes)==0 and cfg['baseImageID'] in images
    required=3*2**30+4*base['Size'];free=shutil.disk_usage(root).free
    record['disk']={'cachedBaseBytes':base['Size'],'ancestorPreSpawnFloorBytes':required,'rootAvailableBytes':free,'differenceBytes':free-required,'tmpAvailableBytes':shutil.disk_usage('/tmp').free,'policyDiskReserveMiBUnchanged':2048,'scope':'The unchanged ancestor pre-spawn bound, not an atomic production admission proof. Product independently evaluates current capacity.'}
    checks['headroom']=free>required
    record['refusals']=[key for key,value in checks.items() if not value]
except Exception as error:
    record['refusals'].append('read-only-preflight-error')
    raw=(type(error).__name__+': '+str(error)).encode()
    record['error']={'kind':type(error).__name__,'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
record['passed']=not record['refusals']
out.write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps(record))
raise SystemExit(0 if record['passed'] else 1)
