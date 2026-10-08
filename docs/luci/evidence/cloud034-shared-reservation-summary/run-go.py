from pathlib import Path
import json,os,subprocess,sys,hashlib,time,datetime
root=Path('/workspace/luci'); out=Path('/workspace/.luci-tools/cloud034-reservation-summary-review'); phase=sys.argv[1]
files=['control-plane/internal/api/execution_reservation_view_test.go','control-plane/internal/api/execution_reservation_view.go','control-plane/internal/api/swarm_dispatch.go','control-plane/internal/api/execution_leases.go']
def hashes():return {p:hashlib.sha256((root/p).read_bytes()).hexdigest() for p in files if (root/p).exists()}
env=os.environ.copy();env.update({'PATH':'/workspace/.luci-tools/go1.26.3/bin:'+env.get('PATH',''),'GOMAXPROCS':'2','GOCACHE':'/tmp/luci-cloud033-go-cache','GOMODCACHE':'/tmp/luci-retained-tools/go-mod','GOWORK':'/workspace/.luci-tools/go.work','TMPDIR':'/tmp/luci-cloud034-test-temp'})
cmd=['go','test','-race','-p=1','-count=1','-timeout=90s','-json','./internal/api','-run',sys.argv[2] if len(sys.argv)>2 else '^TestExecutionReservationViewPausedOwnerObservesTeamCheck$']
r={'command':cmd,'cwd':str(root/'control-plane'),'environment':{k:env[k] for k in ['GOMAXPROCS','GOCACHE','GOMODCACHE','GOWORK','TMPDIR']},'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceBefore':hashes()};start=time.monotonic()
with (out/(phase+'.stdout.jsonl')).open('wb') as stdout,(out/(phase+'.stderr.log')).open('wb') as stderr:
 try: p=subprocess.run(cmd,cwd=root/'control-plane',env=env,stdout=stdout,stderr=stderr,timeout=180);r['exitCode']=p.returncode
 except subprocess.TimeoutExpired:r['exitCode']=None;r['timedOut']=True
r.update({'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'seconds':time.monotonic()-start,'sourceAfter':hashes()});r['sourceStable']=r['sourceBefore']==r['sourceAfter'];r['outcomes']={k:[] for k in ['pass','fail','skip']}
for line in (out/(phase+'.stdout.jsonl')).read_text().splitlines():
 try:
  v=json.loads(line)
  if v.get('Test') and v.get('Action') in r['outcomes']:r['outcomes'][v['Action']].append(v['Test'])
 except ValueError:pass
for k in ['stdout.jsonl','stderr.log']:
 b=(out/(phase+'.'+k)).read_bytes();r[k]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
(out/(phase+'.result.json')).write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({k:r[k] for k in ['exitCode','seconds','sourceStable','outcomes']}))
