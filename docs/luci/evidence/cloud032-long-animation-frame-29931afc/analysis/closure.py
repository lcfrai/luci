from pathlib import Path
import json,hashlib,subprocess,os,datetime
run=Path('/workspace/luci/artifacts/quality-swarm/idle-29931afc-long-frame-attempt1');out=Path(__file__).resolve().parent
raw=run/'driver/result.json';sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest();assert sha(raw)=='d71194560d8c6852b2561062a236ef2af92d32feb917f624a507e28d0977e73f'
r=json.loads(raw.read_text());w=json.loads((run/'wrapper-result.json').read_text());journal=[json.loads(x) for x in (run/'driver/diagnostic-journal.jsonl').read_text().splitlines()]
owned=[{'role':'driver','pid':w['driverIdentity']['pid'],'startTicks':w['driverIdentity']['startTicks']}]+[{'role':'controller','pid':j['pid'],'startTicks':j['startTicks']} for j in journal if j['event']=='controller-start']+[{'role':'browser-recorded','pid':j['pid'],'startTicks':int(j['start'])} for j in r['browserProcessIdentitiesAtSetup']]
for row in owned:
 try:
  values=Path('/proc',str(row['pid']),'stat').read_text().rsplit(') ',1)[1].split();row['observed']={'state':values[0],'parentPid':int(values[1]),'group':int(values[2]),'startTicks':int(values[19]),'sameIdentity':int(values[19])==row['startTicks']}
 except FileNotFoundError:row['observed']={'state':'absent'}
 except (OSError,ValueError,IndexError) as e:row['observed']={'state':'unknown','errorType':type(e).__name__}
env=dict(os.environ)
for k in ['DOCKER_CONTEXT','DOCKER_TLS_VERIFY','DOCKER_CERT_PATH','DOCKER_TLS','DOCKER_API_VERSION']:env.pop(k,None)
env['DOCKER_HOST']='unix:///var/run/docker.sock'
def docker(args):return subprocess.check_output(['docker','--host',env['DOCKER_HOST'],*args],env=env,text=True,timeout=10).strip()
daemon={'endpoint':'unix:///var/run/docker.sock'}
try:
 daemon['containerCount']=len(docker(['ps','-aq','--no-trunc']).split());daemon['volumeCount']=len(docker(['volume','ls','-q']).split());daemon['engineIdentitySHA256']=hashlib.sha256(docker(['info','--format','{{.ID}}']).encode()).hexdigest();daemon['imageId']=json.loads(docker(['image','inspect',r['image']['id']]))[0]['Id'];daemon['engineMatchesRun']=daemon['engineIdentitySHA256']==r['engineIdentitySHA256'];daemon['imageMatchesRun']=daemon['imageId']==r['image']['id'];daemon['status']='observed'
except (OSError,ValueError,TypeError,subprocess.SubprocessError) as e:daemon.update(status='unavailable',errorType=type(e).__name__)
parent=Path('/workspace/.c32');children=list(parent.iterdir()) if parent.exists() else [];fixture={'parentRole':'declared-private-fixture-parent','parentExists':parent.exists(),'children':len(children),'sustainedPrefixChildren':sum(p.name.startswith('luci-sustained-') for p in children),'childNameHashes':[hashlib.sha256(p.name.encode()).hexdigest() for p in children],'scope':'Directory entries only; no credential/private fixture contents. No matching prefix child means the declared parent contains no candidate sustained fixture now.'}
receipt={'observedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'rawResultSHA256':sha(raw),'wrapperSHA256':sha(run/'wrapper-result.json'),'ownedProcesses':owned,'docker':daemon,'fixtureParent':fixture,'journalRows':len(journal),'journalContiguous':all(j['sequence']==i+1 for i,j in enumerate(journal)),'journalLastEvent':journal[-1]['event'],'signalsSent':0,'mutations':0,'scope':'Independent read-only checks of recorded PID/start identities and explicitly selected daemon/fixture parent. Literal zombies retained. No whole-machine census or unrecorded child absence claim.'}
p=out/'independent-closure.json';p.write_text(json.dumps(receipt,indent=2)+'\n');p.chmod(0o600);print(json.dumps(receipt,indent=2))
