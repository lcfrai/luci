from pathlib import Path
import datetime,hashlib,json,subprocess
BASE=Path('/workspace/luci/artifacts/quality-swarm/header-delivery-99007404-v2-attempt1');OUT=Path(__file__).resolve().parent;REPO=Path('/workspace/luci')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
files=[p for p in BASE.rglob('*') if p.is_file()];before={str(p.relative_to(BASE)):sha(p) for p in files}
d=json.loads((BASE/'driver/result.json').read_text());wrap=json.loads((BASE/'wrapper-result.json').read_text());journal=[json.loads(l) for l in (BASE/'driver/diagnostic-journal.jsonl').read_text().splitlines()]
known=[wrap['driverIdentity']]+[r for r in journal if r['event']=='controller-start'];observed=[]
for row in known:
 expected={'pid':row['pid'],'startTicks':row['startTicks']}
 try:
  fields=Path('/proc',str(row['pid']),'stat').read_text().rsplit(') ',1)[1].split();same=int(fields[19])==row['startTicks'];current={'state':fields[0],'startTicks':int(fields[19]),'parentPid':int(fields[1])};state='same-identity-zombie' if same and fields[0]=='Z' else 'same-identity-live' if same else 'different-identity'
 except FileNotFoundError:state='absent';current=None
 except (PermissionError,OSError,ValueError) as e:state='unknown';current={'errorType':type(e).__name__}
 observed.append({'expected':expected,'observation':state,'current':current})
def cmd(args):return subprocess.check_output(args,cwd=REPO,text=True,timeout=30).strip()
containers=cmd(['docker','ps','-aq','--no-trunc']).splitlines();volumes=cmd(['docker','volume','ls','-q']).splitlines();image=json.loads(cmd(['docker','image','inspect',d['image']['id']]))[0]['Id'];engine=cmd(['docker','info','--format','{{.ID}}']);after={str(p.relative_to(BASE)):sha(p) for p in files};assert before==after
receipt={'observedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Independent read-only exact known driver/controller identity and exclusive daemon inventory check. No cleanup mutations or signals.','sourceHEAD':cmd(['git','rev-parse','HEAD']),'sourceStatus':cmd(['git','status','--porcelain']).splitlines(),'originalResultSHA256':before['driver/result.json'],'allOriginalArtifactsStable':before==after,'originalArtifacts':before,'identities':observed,'containers':containers,'volumes':volumes,'imageID':image,'engineIdentitySHA256':hashlib.sha256(engine.encode()).hexdigest(),'engineMatchesDriver':hashlib.sha256(engine.encode()).hexdigest()==d['engineIdentitySHA256'],'driverCleanupAssertions':{k:d[k] for k in ['controllersStopped','dockerRequestsDrained','dockerMutationStateCertain','fixtureRemoved','cleanup']},'limits':['Only journal-recorded driver and controller PID/start identities were independently observed. No browser PID/start census or independent Linux observer was captured in this idle diagnostic.','Browser/zombie absence and whole-machine closure are not established by empty Docker inventory or absent controller. Historical unrelated zombies are not assigned to this run.','Fixture removal is a retained driver assertion; the original private fixture path is not present in the successful final result. No retained failed fixture/lease was read or modified.']}
(OUT/'independent-closure.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'identities':observed,'containers':containers,'volumes':volumes,'sourceHEAD':receipt['sourceHEAD'],'sourceStatusRows':len(receipt['sourceStatus']),'originalStable':before==after}))
