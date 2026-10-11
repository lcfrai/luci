from pathlib import Path
import json,subprocess,hashlib,datetime
base=Path(__file__).parent;run=base/'preview-1791719014644-5018e36f-1e33-4fbb-8391-9ddd3be5772c';r=json.loads((run/'result.json').read_text());assert r['passed']
def cmd(*args):return subprocess.run(['docker','--host','unix:///var/run/docker.sock',*args],capture_output=True,text=True,timeout=10)
info=cmd('info','--format','{{.ID}}');assert info.returncode==0 and info.stdout.strip()==r['runtime']['engineId']
ref=r['runtime']['containerId'];ins=cmd('inspect',ref);assert ins.returncode==1 and ref in ins.stderr and 'No such' in ins.stderr
inventory=cmd('ps','-a','--no-trunc','--format','{{.ID}}');assert inventory.returncode==0
assert not inventory.stdout.strip()
ledger=run/'private/state/control-plane/execution/leases.json';raw=ledger.read_bytes();data=json.loads(raw);assert not data['previews'];assert not data['leases'];assert not data['checks'];assert not data.get('runtimeImages',[])
retained=json.loads((run/'after-stop-release.json').read_text());assert retained['runtime']['cleanupConfirmed'] and not retained['runtime']['resourcesHeld'] and retained['runtime']['phase']=='finished'
processes=json.loads((run/'owned-process-closure.json').read_text())['processes'];current=[]
for p in processes:
 path=Path('/proc')/str(p['pid'])/'stat'
 if path.exists():
  fields=path.read_text().rsplit(')',1)[1].split();assert fields[19]!=p['startTicks'];current.append({'pid':p['pid'],'startTicks':p['startTicks'],'observation':'PID reused; original absent'})
 else:current.append({'pid':p['pid'],'startTicks':p['startTicks'],'observation':'absent'})
image=cmd('image','inspect',r['runtime']['image'],'--format','{{json .}}');assert image.returncode==0;image=json.loads(image.stdout);assert image['Config']['Labels']['luci.instance']==r['runtime']['instanceId']
assert subprocess.check_output(['git','status','--porcelain'],cwd='/workspace/luci',text=True)==''
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'productSource':'577f7a0fdb039b7f875901a6a567d62fdfee88c8','resultSHA256':hashlib.sha256((run/'result.json').read_bytes()).hexdigest(),'exactContainer':ref,'immutableInspectAbsent':True,'globalContainerIDs':[],'engineID':info.stdout.strip(),'rawPrivateLedgerSHA256':hashlib.sha256(raw).hexdigest(),'ledgerRows':{k:len(data.get(k,[])) for k in ['previews','leases','checks','runtimeImages']},'observedProcessIdentities':current,'retainedOutputImage':{'id':image['Id'],'bytes':image['Size']},'normalAPIRetiredOnlyExactLineage':True,'manualDockerMutation':False,'sourceClean':True,'scope':'Independent read-only exact object, ledger and previously observed PID/start closure. Prior failed environments not read or modified. COPY-only fixture preview; no full checked-project/model/RUN workload or performance claim.'}
(base/'independent-closure.json').write_text(json.dumps(result,indent=2));print(json.dumps(result))
