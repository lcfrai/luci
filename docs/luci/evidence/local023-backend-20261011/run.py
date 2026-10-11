from pathlib import Path
import datetime,hashlib,json,os,subprocess,time
ROOT=Path('/workspace/luci'); OUT=Path(__file__).resolve().parent
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def snap():
 paths=[p for p in (ROOT/'control-plane').rglob('*') if p.is_file() and (p.suffix=='.go' or p.name in ['go.mod','go.sum']) and 'vendor' not in p.parts]
 return {str(p.relative_to(ROOT)):{'sha256':sha(p),'bytes':p.stat().st_size}for p in sorted(paths)}
def save(n,v):(OUT/n).write_text(json.dumps(v,indent=2)+'\n')
source=snap();save('source-before.json',source)
cmd=['go','test','-race','-p=1','-count=1','-timeout=4m','-json','./control-plane/internal/product','./control-plane/internal/api','-run','^Test(ProjectOverview|OrchestratorUsesSelectedProvider|IndependentSignInAndDefineAssistant|DefineAssistant(Cancellation|Recovery))']
inv={'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'command':cmd,'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'goVersion':subprocess.check_output(['go','version'],text=True).strip(),'scope':'Selected existing overview/assistant/orchestrator regression cases; all control-plane repository Go/mod/sum bytes before/after, not full compiled dependency provenance or live provider proof.'};save('invocation.json',inv)
begin=time.monotonic()
with (OUT/'events.jsonl').open('w')as out,(OUT/'stderr.log').open('w')as err:
 p=subprocess.run(cmd,cwd=ROOT,env={**os.environ,'GOMAXPROCS':'2'},stdout=out,stderr=err,timeout=330)
after=snap();save('source-after.json',after);rows=[]
for line in (OUT/'events.jsonl').read_text().splitlines():
 try:rows.append(json.loads(line))
 except json.JSONDecodeError:pass
outcomes=[r for r in rows if r.get('Test')and r['Action']in ['pass','fail','skip']]
r={**inv,'exitCode':p.returncode,'elapsedSeconds':time.monotonic()-begin,'sourceStable':source==after,'sourceInputs':len(source),'outcomes':{k:sum(r['Action']==k for r in outcomes)for k in ['pass','fail','skip']},'topLevel':{k:sum(r['Action']==k and '/'not in r['Test']for r in outcomes)for k in ['pass','fail','skip']}}
save('result.json',r);print(json.dumps(r));raise SystemExit(p.returncode if p.returncode else 0 if source==after else 2)
