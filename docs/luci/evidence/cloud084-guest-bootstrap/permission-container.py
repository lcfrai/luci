"""Controlled Linux ownership test only; no engine, cloud VM or model request."""
import hashlib,json,pathlib,subprocess,time,uuid,sys
out=pathlib.Path(__file__).resolve().parent
binary=(out/'microvmguest.test').resolve()
image='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
report={'scope':'existing-image Linux ownership and real Git fixture; not live microVM or provider acceptance','image':image,'binarySHA256':hashlib.sha256(binary.read_bytes()).hexdigest(),'steps':[],'passed':False,'forced':False}
cid=None
label='luci-guest-permission-'+uuid.uuid4().hex

def save():
 temp=out/'permission-container.pending.json';temp.write_text(json.dumps(report,indent=2)+'\n');temp.replace(out/'permission-container.json')
def cmd(args, timeout=25, check=True):
 started=time.time()
 try:
  p=subprocess.run(['docker',*args],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,timeout=timeout)
  row={'args':args,'started':started,'elapsed':time.time()-started,'exit':p.returncode,'stdout':p.stdout,'stderr':p.stderr};report['steps'].append(row);save()
  if check and p.returncode:raise RuntimeError('Docker command failed: '+args[0])
  return p
 except subprocess.TimeoutExpired as e:
  report['steps'].append({'args':args,'started':started,'elapsed':time.time()-started,'timeout':True,'stdout':(e.stdout or b'').decode(errors='replace') if isinstance(e.stdout,bytes) else e.stdout,'stderr':(e.stderr or b'').decode(errors='replace') if isinstance(e.stderr,bytes) else e.stderr});save();raise
try:
 report['inventoryBefore']=cmd(['ps','-a','--no-trunc','--format','{{.ID}} {{.State}}']).stdout.splitlines()
 assert cmd(['image','inspect',image,'--format','{{.Id}}']).stdout.strip()==image
 cid=cmd(['create','--name',label,'--label','luci.controlled-guest-test='+label,'--network','none','--memory','384m','--memory-swap','384m','--cpus','0.5','--pids-limit','64','--read-only','--cap-drop','ALL','--cap-add','CHOWN','--cap-add','DAC_OVERRIDE','--cap-add','FOWNER','--cap-add','SETUID','--cap-add','SETGID','--security-opt','no-new-privileges','--user','0:0','--tmpfs','/tmp:rw,nosuid,nodev,size=64m,mode=1777','--mount','type=bind,src='+str(binary)+',dst=/probe/guest.test,readonly','--entrypoint','/probe/guest.test',image,'-test.run','^TestProtectedGuestContextCannotBeReplacedByUpstreamUser$','-test.v','-test.timeout','20s']).stdout.strip()
 assert len(cid)==64 and all(c in '0123456789abcdef' for c in cid)
 report['containerId']=cid;save()
 run=cmd(['start','--attach',cid],timeout=30,check=False)
 state=json.loads(cmd(['inspect',cid]).stdout)[0]
 assert state['Id']==cid and state['Config']['Labels']['luci.controlled-guest-test']==label
 report['inspectedState']={k:state['State'].get(k) for k in ['Status','Running','ExitCode','OOMKilled','Error']}
 report['limits']={k:state['HostConfig'].get(k) for k in ['Memory','MemorySwap','NanoCpus','PidsLimit','ReadonlyRootfs','NetworkMode','CapAdd','CapDrop']}
 assert run.returncode==0 and state['State']['Status']=='exited' and state['State']['ExitCode']==0 and '--- PASS: TestProtectedGuestContextCannotBeReplacedByUpstreamUser' in run.stdout and '--- SKIP' not in run.stdout
 report['passed']=True
except BaseException as e:
 report['error']=str(e)
finally:
 if cid:
  try:
   state=json.loads(cmd(['inspect',cid]).stdout)[0]
   assert state['Id']==cid and state['Config']['Labels']['luci.controlled-guest-test']==label
   if state['State']['Running']:
    report['forced']=True;report['passed']=False;cmd(['stop','--time','3',cid],timeout=10)
   assert not json.loads(cmd(['inspect',cid]).stdout)[0]['State']['Running']
   cmd(['rm',cid]);absent=cmd(['inspect',cid],check=False)
   report['ownedContainerAbsent']=absent.returncode!=0 and 'No such object' in absent.stderr
  except BaseException as e:report['cleanupError']=str(e);report['passed']=False
 report['inventoryAfter']=cmd(['ps','-a','--no-trunc','--format','{{.ID}} {{.State}}']).stdout.splitlines()
 report['unrelatedInventoryUnchanged']=report.get('inventoryBefore')==report['inventoryAfter']
 save()
sys.exit(0 if report['passed'] and report.get('ownedContainerAbsent') and report['unrelatedInventoryUnchanged'] else 1)
