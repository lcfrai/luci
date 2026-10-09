from pathlib import Path
import subprocess,os,json,hashlib,time,datetime
os.umask(0o077)
base=Path('/workspace/.luci-tools');out=base/'cloud054-old-image-retirement-5f4a966f';out.mkdir(mode=0o700)
client=out/'docker-client';client.mkdir(mode=0o700)
env={k:os.environ[k] for k in ['PATH','LANG','LC_ALL','TZ'] if k in os.environ};env.update(HOME=str(client),DOCKER_CONFIG=str(client))
def docker(*args):return subprocess.check_output(['/usr/local/bin/docker','--host','unix:///var/run/docker.sock',*args],env=env,timeout=90,text=True).strip()
def digest(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def save(name,value):
 p=out/name;data=(json.dumps(value,indent=2)+'\n').encode();fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'wb') as f:f.write(data);f.flush();os.fsync(f.fileno())
 fd=os.open(out,os.O_RDONLY|os.O_DIRECTORY)
 try:os.fsync(fd)
 finally:os.close(fd)
def inventory():return sorted(tuple(json.loads(x)[k] for k in ['Repository','Tag','ID']) for x in docker('image','ls','--no-trunc','--format','{{json .}}').splitlines())
def inspect(ref):return json.loads(docker('image','inspect',ref))[0]
old='sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98';new='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8';trusted='sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351';engine='ff40514f-d7cc-4ee7-8a1a-1cfafaeaf259';tags=['luci/sandbox:dev','luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b']
archive=base/'luci-sandbox-dev.tar.gz';archive_sha='7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3';receipt=base/'sandbox-candidate-20261009T134118Z-4a347241/receipt.json';cv=json.loads(receipt.read_text())
audit={'source':'5f4a966f6d70264c92ee2176dde912e1b43fc3c7','helperSHA256':digest(Path(__file__)),'candidateReceiptSHA256':digest(receipt),'scope':'Root-coordinated exclusive Docker maintenance, exact task-owned tags only; no global process-idle assertion. Known unreadable dockerd/containerd/gpg identities remain unobserved. No fixture/ledger changes or capacity-release claim.','status':'preflight','steps':[]}
save('001-started.json',audit)
try:
 assert cv['status']=='built-config-verified-export-and-runtime-pending' and cv['candidateImage']==new and cv['baseImage']==trusted
 assert cv['sourcesBefore']==cv['sourcesAfter'] and cv['sourceFenceAfter']=={'sourceCommit':audit['source'],'clean':True}
 assert subprocess.check_output(['git','rev-parse','HEAD'],cwd='/workspace/luci',text=True).strip()==audit['source']
 assert not subprocess.check_output(['git','status','--porcelain'],cwd='/workspace/luci',text=True).strip()
 assert docker('info','--format','{{.ID}}')==engine and docker('ps','-aq')==''
 assert archive.is_file() and not archive.is_symlink() and archive.stat().st_size==1013367768 and digest(archive)==archive_sha
 before=inventory();old_info=inspect(old);assert set(old_info['RepoTags'])==set(tags)
 assert inspect(new)['Id']==new and inspect(trusted)['Id']==trusted
 audit.update(status='preflight-passed',archiveSHA256=archive_sha,archiveBytes=archive.stat().st_size,restoreScope='Full current compressed hash matches retained prior CRC/manifest/config proof; no fresh Docker load',inventoryBefore=before,rootFreeBefore=os.statvfs('/workspace').f_bavail*os.statvfs('/workspace').f_frsize)
 save('002-preflight.json',audit)
 for i,tag in enumerate(tags):
  assert docker('info','--format','{{.ID}}')==engine and docker('ps','-aq')=='' and inspect(tag)['Id']==old
  save(f'{3+i*2:03}-remove-pending.json',{**audit,'pendingTag':tag,'command':['docker','--host','unix:///var/run/docker.sock','image','rm','--no-prune',tag]})
  reply=docker('image','rm','--no-prune',tag);audit['steps'].append({'tag':tag,'result':reply,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()})
  save(f'{4+i*2:03}-remove-returned.json',audit)
 after=inventory();expected=[x for x in before if x[2]!=old];assert after==expected,(after,expected)
 assert docker('ps','-aq')=='' and inspect(new)['Id']==new and inspect(trusted)['Id']==trusted
 assert digest(archive)==archive_sha
 audit.update(status='passed',inventoryAfter=after,archiveUnchanged=True,rootFreeAfter=os.statvfs('/workspace').f_bavail*os.statvfs('/workspace').f_frsize,oldImageRemoved=True,applicationRuntimeVerified=False)
 save('007-final.json',audit);print(json.dumps({'status':'passed','receipt':str(out/'007-final.json'),'rootFreeAfter':audit['rootFreeAfter']}),flush=True)
except BaseException as error:
 audit.update(status='failed-or-uncertain',errorType=type(error).__name__,automaticRetry=False);save('999-failed.json',audit);raise
