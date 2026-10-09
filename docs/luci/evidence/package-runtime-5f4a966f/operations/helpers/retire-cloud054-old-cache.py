from pathlib import Path
import subprocess,os,json,hashlib,datetime
os.umask(0o077)
out=Path('/workspace/.luci-tools/cloud054-old-cache-retirement-attempt2');out.mkdir(mode=0o700)
client=out/'docker-client';client.mkdir(mode=0o700)
env={k:os.environ[k] for k in ['PATH','LANG','LC_ALL','TZ'] if k in os.environ};env.update(HOME=str(client),DOCKER_CONFIG=str(client))
selected='jxury1xmsdvxr24fm76wfizm5';engine='ff40514f-d7cc-4ee7-8a1a-1cfafaeaf259';source='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
def sha(p):
 with Path(p).open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def save(n,v):
 p=out/n
 with p.open('x') as f:json.dump(v,f,indent=2);f.write('\n');f.flush();os.fsync(f.fileno())
 fd=os.open(out,os.O_RDONLY|os.O_DIRECTORY);os.fsync(fd);os.close(fd)
def docker(*args):return subprocess.check_output(['/usr/local/bin/docker','--host','unix:///var/run/docker.sock',*args],env=env,timeout=60,text=True).strip()
def cache():return {x['ID']:x for x in map(json.loads,docker('buildx','du','--builder','default','--format','json').splitlines())}
def images():
 ids=sorted(set(docker('image','ls','-q','--no-trunc').splitlines()))
 return {x['Id']:x for x in json.loads(docker('image','inspect',*ids))}
def free():return {p:os.statvfs(p).f_bavail*os.statvfs(p).f_frsize for p in ['/workspace','/tmp']}
def empty():assert docker('info','--format','{{.ID}}')==engine and not docker('ps','-aq') and not docker('volume','ls','-q')
record={'source':source,'helperSHA256':sha(__file__),'status':'preflight','scope':'Root-exclusive exact task-owned old COPY cache record only. Associated lease/unreferenced storage GC possible. No fixture, ledger, product capacity, runtime or whole-machine-idle claim.'}
save('001-started.json',record)
try:
 assert subprocess.check_output(['git','rev-parse','HEAD'],cwd='/workspace/luci',text=True).strip()==source
 assert not subprocess.check_output(['git','status','--porcelain'],cwd='/workspace/luci',text=True).strip()
 empty();before=cache();im=images();topology=docker('buildx','inspect','default')
 assert 'Driver: docker' in topology and topology.count('Endpoint:')==1 and 'Endpoint:         default' in topology and 'BuildKit version: v0.24.0' in topology
 row=before[selected];assert row['Type']=='regular' and not row['Mutable'] and row['Reclaimable'] and not row['Shared']
 assert row['Parents']==['a0z9tfoq25ub9hkicmah62g4c'] and row['CreatedAt']=='2026-10-08 14:01:24.232384903 +0000 UTC'
 assert not any(selected in (x.get('Parents') or []) for x in before.values())
 assert 'ue36gvnn6iqhphceohjhzwae5' in before and 'a0z9tfoq25ub9hkicmah62g4c' in before
 assert 'sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8' in im and 'sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351' in im
 archive=Path('/workspace/.luci-tools/luci-sandbox-dev.tar.gz');archive_sha=sha(archive);assert archive_sha=='7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3'
 record.update(cacheBefore=before,imagesBefore=im,topology=topology,freeBefore=free(),archiveSHA256=archive_sha)
 save('002-preflight.json',record);empty()
 args=['buildx','prune','--builder','default','--force','--filter','id=^'+selected+'$']
 save('003-pending.json',{'command':['/usr/local/bin/docker','--host','unix:///var/run/docker.sock',*args],'status':'pending','automaticRetry':False,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()})
 result=subprocess.run(['/usr/local/bin/docker','--host','unix:///var/run/docker.sock',*args],env=env,timeout=60,capture_output=True,text=True)
 save('004-returned.json',{'exitCode':result.returncode,'stdout':result.stdout,'stderr':result.stderr})
 assert result.returncode==0
 after=cache();im_after=images();empty()
 assert set(after)==set(before)-{selected}, {'unexpectedMissing':sorted(set(before)-set(after)-{selected}),'unexpectedAdded':sorted(set(after)-set(before))}
 for key,row in after.items():
  assert {k:v for k,v in row.items() if k!='LastUsedAt'}=={k:v for k,v in before[key].items() if k!='LastUsedAt'},key
 assert im_after==im
 assert sha(archive)==archive_sha
 record.update(status='passed',cacheAfter=after,imagesAfter=im_after,freeAfter=free(),archiveUnchanged=True)
 save('005-final.json',record);print(json.dumps({'status':record['status'],'receipt':str(out/'005-final.json'),'freeBefore':record['freeBefore'],'freeAfter':record['freeAfter']}))
except BaseException as e:
 record.update(status='failed-or-uncertain',errorType=type(e).__name__,automaticRetry=False);save('999-failure.json',record);raise
