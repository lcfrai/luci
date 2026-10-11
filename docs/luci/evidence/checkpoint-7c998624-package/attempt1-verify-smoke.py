import hashlib,json,os,pathlib,signal,socket,subprocess,tarfile,time,urllib.request,urllib.error,uuid
root=pathlib.Path(__file__).resolve().parents[2]
out=root/'artifacts/checkpoint-build'; pkg=out/'installed'; release=root/'artifacts/releases/0.5.0'; expected='7c998624d94720e84c8cef0236d0dafbe17a535b'
h=lambda b:hashlib.sha256(b).hexdigest()
manifest=json.loads((release/'release-manifest.json').read_text());assert manifest['sourceCommit']==expected and manifest['sourceDirty'] is False
report={'sourceCommit':expected,'scope':'Actual packaged Linux HTTP startup/policy/state smoke only; no browser, provider, runtime image build, task dispatch, microVM or performance pass.','archives':[],'requests':[],'checkpoints':[]}
for row in manifest['artifacts']:
 path=release/row['name'];assert path.stat().st_size==row['bytes'] and h(path.read_bytes())==row['sha256']
 count=0
 with tarfile.open(path,'r:gz') as tar:
  for member in tar:
   p=pathlib.PurePosixPath(member.name);assert not p.is_absolute() and '..' not in p.parts and (member.isdir() or member.isfile())
   if member.isfile():count+=1
  if row['name']=='luci-cloud-linux-amd64.tar.gz':tar.extractall(pkg,filter='data')
 report['archives'].append({**row,'regularMembers':count})
# Re-read every extracted member against the original archive, without trusting an inventory alone.
with tarfile.open(release/'luci-cloud-linux-amd64.tar.gz','r:gz') as tar:
 for member in tar:
  if member.isfile():
   target=pkg/member.name;assert target.is_file() and not target.is_symlink() and h(target.read_bytes())==h(tar.extractfile(member).read())
report['binarySHA256']=h((pkg/'luci').read_bytes());report['webIndexSHA256']=h((pkg/'web/index.html').read_bytes())
run=out/('private-smoke-'+uuid.uuid4().hex);run.mkdir(mode=0o700)
for name in ['home','state','projects']:(run/name).mkdir(mode=0o700)
s=socket.socket();s.bind(('127.0.0.1',0));port=s.getsockname()[1];s.close();base='http://127.0.0.1:'+str(port);token='';proc=None;identities=[]
env={k:os.environ[k] for k in ['PATH','LANG','LC_ALL','TZ','DOCKER_HOST'] if k in os.environ}
env.update(HOME=str(run/'home'),LUCI_STATE_DIR=str(run/'state'),LUCI_PROJECTS_DIR=str(run/'projects'),LUCI_ADDR='127.0.0.1:'+str(port),LUCI_WEB_DIR=str(pkg/'web'),LUCI_RUNTIME_DIR=str(pkg/'runtime'),LUCI_SANDBOX_IMAGE='luci/checkpoint-unallocated:'+uuid.uuid4().hex,LUCI_BACKEND='docker',LUCI_LOG_LEVEL='error',GIT_CONFIG_NOSYSTEM='1')
def identity(pid):
 try:return pathlib.Path('/proc',str(pid),'stat').read_text().rsplit(')',1)[1].split()[19]
 except FileNotFoundError:return None
def call(method,path,data=None,status=200,record=True):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request(base+path,data=None if data is None else json.dumps(data).encode(),headers=headers,method=method)
 try:response=urllib.request.urlopen(req,timeout=20)
 except urllib.error.HTTPError as e:response=e
 body=response.read();code=response.status
 if record:report['requests'].append({'method':method,'path':path,'status':code})
 assert code==status,(method,path,code,body[:250])
 return json.loads(body) if body else None
def start():
 global proc
 log=open(run/('service-'+str(len(identities))+'.log'),'wb');proc=subprocess.Popen([str(pkg/'luci')],env=env,cwd=pkg,stdout=log,stderr=subprocess.STDOUT,start_new_session=True);log.close();identities.append({'pid':proc.pid,'start':identity(proc.pid)})
 deadline=time.monotonic()+25
 while time.monotonic()<deadline:
  if proc.poll() is not None:raise AssertionError('service exited before health')
  try:call('GET','/v1/healthz',record=False);return
  except (OSError,AssertionError):time.sleep(.1)
 raise AssertionError('health deadline')
def stop():
 global proc
 if proc is None:return
 if proc.poll() is None:proc.terminate()
 try:proc.wait(timeout=20)
 except subprocess.TimeoutExpired:
  os.killpg(proc.pid,signal.SIGKILL);proc.wait();raise AssertionError('forced process cleanup required')
 proc=None
try:
 start();report['checkpoints'].append('packaged health')
 bootstrap=(run/'state/control-plane/bootstrap-token').read_text().strip()
 account=call('POST','/v1/auth/bootstrap',{'bootstrapToken':bootstrap,'username':'checkpointowner','displayName':'Checkpoint Owner','password':uuid.uuid4().hex+uuid.uuid4().hex},201);token=account['token']
 report['checkpoints'].append('fresh owner bootstrap')
 projects=[]
 for name in ['checkpoint-alpha','checkpoint-beta','checkpoint-gamma','checkpoint-delta']:
  project=call('POST','/v1/product/projects',{'name':name},201);pid=project['id'];projects.append(pid)
  route='/v1/product/projects/'+pid+'/orchestrator'
  policy=call('GET',route+'/policy');assert policy['revision']==0 and policy['mode']=='review' and policy['executionMode']=='manual'
  saved=call('POST',route+'/policy',{'expectedRevision':0,'mode':'review','allowedTargets':['specification'],'executionMode':'manual'});assert saved['revision']==1
  call('POST',route+'/policy',{'expectedRevision':0,'mode':'auto-drafts','allowedTargets':['specification']},409)
  assert call('GET',route+'/continuations')['continuations']==[]
  coordination=call('GET',route+'/coordination');assert coordination['projectId']==pid
 report['checkpoints'].append('four project default policy, save, stale CAS and private continuation routes')
 stop();start()
 for pid in projects:
  policy=call('GET','/v1/product/projects/'+pid+'/orchestrator/policy');assert policy['revision']==1 and policy['projectId']==pid and policy['executionMode']=='manual'
 report['checkpoints'].append('four policy records persist through normal service restart')
 report['passed']=True
except BaseException as e:
 report['passed']=False;report['failureType']=type(e).__name__;report['failure']=str(e)
finally:
 try:stop()
 except BaseException as e:report['cleanupFailure']=str(e)
 report['ownedIdentities']=[{**i,'sameIdentityRemaining':identity(i['pid'])==i['start']} for i in identities]
 with socket.socket() as sock:report['listenerRemains']=sock.connect_ex(('127.0.0.1',port))==0
 report['cleanupPassed']=not report.get('cleanupFailure') and not report['listenerRemains'] and all(not i['sameIdentityRemaining'] for i in report['ownedIdentities'])
 (out/'package-smoke.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({k:report.get(k) for k in ['passed','failureType','checkpoints','cleanupPassed']}))
 if not report.get('passed') or not report['cleanupPassed']:raise SystemExit(1)
