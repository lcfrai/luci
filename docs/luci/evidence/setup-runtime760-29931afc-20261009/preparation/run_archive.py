"""900s source-bound archive watchdog; root invokes only in an exclusive window."""
from pathlib import Path
import argparse,hashlib,json,os,signal,subprocess,sys,time
ROOT=Path(__file__).resolve().parent;TOOLS=ROOT.parent;PINS={'archive760.py': 'c328da3bd8b4148cceba74ce35bbc6726195da49add6777a3372943c49c4d986', 'budget.py': 'd700dcd37fca027c6f277842dc0d36eebec3748ec148b1ccf87e61d18df7e570', 'start.template.md': '08a97a78dc51b2ee18013f44d725bc3bf5b37e96dcdc1f6c938e4eeb6c517249'}

def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
class NonLive(ProcessLookupError):
 def __init__(self,state):self.state=state

def start(pid):
 f=Path('/proc',str(pid),'stat').read_text().rsplit(')',1)[1].split()
 if f[0]in ['Z','X']:raise NonLive(f[0])
 return f[19]
def write(p,o):
 b=(json.dumps(o,indent=2)+'\n').encode();t=p.with_name(p.name+'.new');fd=os.open(t,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
 with os.fdopen(fd,'wb')as f:f.write(b);f.flush();os.fsync(f.fileno())
 os.replace(t,p);fd=os.open(p.parent,os.O_RDONLY|os.O_DIRECTORY)
 try:os.fsync(fd)
 finally:os.close(fd)
def settle(pid,ticks):
 r={'pid':pid,'startTicks':ticks,'leaderAbsent':False,'daemonDrainConfirmed':False,'descendantCensus':False}
 try:
  if start(pid)!=ticks:r['identityChanged']=True;return r
  os.killpg(pid,signal.SIGTERM);r['termSent']=True
  for _ in range(100):
   time.sleep(.1)
   try:
    if start(pid)!=ticks:r['identityChanged']=True;return r
   except NonLive as e:r.update(leaderNonLive=True,observedState=e.state,leaderAbsent=False);return r
   except(FileNotFoundError,ProcessLookupError):r['leaderAbsent']=True;return r
  if start(pid)!=ticks:r['identityChanged']=True;return r
  os.killpg(pid,signal.SIGKILL);r['killSent']=True
  for _ in range(100):
   time.sleep(.1)
   try:
    if start(pid)!=ticks:r['identityChanged']=True;return r
   except NonLive as e:r.update(leaderNonLive=True,observedState=e.state,leaderAbsent=False);return r
   except(FileNotFoundError,ProcessLookupError):r['leaderAbsent']=True;return r
  r['settlementUnknown']=True
 except NonLive as e:r.update(leaderNonLive=True,observedState=e.state,leaderAbsent=False)
 except(FileNotFoundError,ProcessLookupError):r['leaderAbsent']=True
 except BaseException as e:r.update(errorType=type(e).__name__,settlementUnknown=True)
 return r

def main(argv=None):
 p=argparse.ArgumentParser();p.add_argument('--window-granted',action='store_true',required=True);p.add_argument('--expected-source',required=True);p.add_argument('--expected-engine',required=True);s=p.add_subparsers(dest='command',required=True);st=s.add_parser('stage');st.add_argument('--reviewed-runtime-receipt',type=Path,required=True);st.add_argument('--reviewed-runtime-sha256',required=True);pb=s.add_parser('publish');pb.add_argument('receipt',type=Path);a=p.parse_args(argv);os.umask(0o077)
 if len(PINS)!=3 or any(sha(ROOT/n)!=v for n,v in PINS.items()):raise RuntimeError('sealed-archive-sources-changed')
 out=ROOT/('watchdog-'+str(time.monotonic_ns()));out.mkdir(mode=0o700);record=out/'docker-client.json';cmd=[sys.executable,'-B',str(ROOT/'archive760.py'),'--watchdog-record',str(record),'--window-granted','--expected-source',a.expected_source,'--expected-engine',a.expected_engine,a.command]
 if a.command=='stage':cmd+=['--reviewed-runtime-receipt',str(a.reviewed_runtime_receipt),'--reviewed-runtime-sha256',a.reviewed_runtime_sha256]
 else:cmd+=[str(a.receipt)]
 meta={'command':cmd,'wrapperSHA256':sha(Path(__file__)),'processWaitSeconds':900,'maximumSettlementWaitSeconds':40,'startedAt':time.time(),'scope':'Synchronous OS I/O is not preemptively bounded; no daemon or descendant drain proof. Forced/unknown operation fails and original partial bytes remain.'};write(out/'wrapper.json',meta);proc=None;ticks=None;failed=False
 try:
  with(out/'stdout.log').open('xb')as stdout,(out/'stderr.log').open('xb')as stderr:
   proc=subprocess.Popen(cmd,stdout=stdout,stderr=stderr,start_new_session=True);ticks=start(proc.pid);meta.update(pid=proc.pid,startTicks=ticks);write(out/'wrapper.json',meta);code=proc.wait(timeout=900);failed=code!=0;meta.update(exitCode=code,passed=code==0)
 except BaseException as e:failed=True;meta.update(passed=False,errorType=type(e).__name__,leaderIdentityUnavailable=(proc is not None and ticks is None),uncertainty='Receipt may be incomplete; never retry blindly.')
 finally:
  if failed:
   settlements=[]
   if proc is not None and ticks is not None:settlements.append(settle(proc.pid,ticks))
   try:
    if record.is_symlink()or record.stat().st_size>4096:raise RuntimeError('untrusted-owned-client-record')
    r=json.loads(record.read_bytes());pid=r.get('pid');st=r.get('startTicks')
    if type(pid)is int and pid>1 and isinstance(st,str)and st.isdigit()and r.get('candidateImage')=='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8':settlements.append(settle(pid,st))
    else:meta['dockerClientIdentityUnavailable']=True
   except BaseException as e:meta.update(dockerClientIdentityUnavailable=True,clientRecordErrorType=type(e).__name__)
   meta['boundedSettlement']=settlements
  meta['finishedAt']=time.time()
  try:write(out/'wrapper.json',meta)
  except BaseException:print(json.dumps({'passed':False,'durableFinalReceiptUnavailable':True,'boundedSettlement':meta.get('boundedSettlement')}),file=sys.stderr);return 1
 print(json.dumps({'passed':meta.get('passed',False),'wrapper':str(out/'wrapper.json')}));return 0 if meta.get('passed')else 1
if __name__=='__main__':raise SystemExit(main())
