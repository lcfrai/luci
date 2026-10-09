"""Root-run bounded watchdog for this exact Go restoration only; never runs on import."""
from pathlib import Path
import argparse,hashlib,json,os,signal,subprocess,sys,time
ROOT=Path('/workspace/.luci-tools/go-tool-restore-v1')
PINS={'operation_common.py': '1863d7908931bfd4463b73139a550930a3f3092ce0408a707411c2726956478c', 'restore_go.py': '993c530c77a53bbe13d1cab9b3e9210a8f2aeac3d35d7ea83be7b3663d03aec3', 'tree_support.py': 'a86951497592b707ff2b157cf4c6e193180f18eedf5af42e09f98994242a3f78'}
ACCEPTED=[{'pid':203,'startTicks':'1521'},{'pid':254,'startTicks':'1534'},{'pid':260700,'startTicks':'2331192'}]
WORK_SECONDS=400;TERM_SECONDS=10;KILL_SECONDS=10

def save(path,value):
 data=(json.dumps(value,indent=2)+'\n').encode();tmp=path.with_name(path.name+'.pending')
 fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
 try:
  offset=0
  while offset<len(data):offset+=os.write(fd,data[offset:])
  os.fsync(fd)
 finally:os.close(fd)
 os.replace(tmp,path);fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
 try:os.fsync(fd)
 finally:os.close(fd)

def settle_owned(proc,meta,c):
 """Best effort bounded settlement independent of receipt writes; no descendant proof."""
 result={'attempted':True,'scope':'Exact live leader and its initial group only; leader exit is not a descendant census.'}
 try:
  if proc.poll()is not None:
   result.update(leaderExited=True,exitCode=proc.returncode,groupSettlementUnobserved=True);return result
  c.need(meta.get('startTicks')==c.process_start(proc.pid),'child-identity-unavailable-before-term')
  os.killpg(proc.pid,signal.SIGTERM);result['termSent']=True
  try:result['exitCode']=proc.wait(timeout=TERM_SECONDS)
  except subprocess.TimeoutExpired:
   if proc.poll()is None:
    c.need(meta.get('startTicks')==c.process_start(proc.pid),'child-identity-unavailable-before-kill');os.killpg(proc.pid,signal.SIGKILL);result['killSent']=True
   try:result['exitCode']=proc.wait(timeout=KILL_SECONDS)
   except subprocess.TimeoutExpired:result['leaderSettlementUnconfirmed']=True
  result['leaderExited']=proc.poll()is not None;result['groupSettlementUnobserved']=True
 except BaseException as e:result.update(settlementErrorType=type(e).__name__,leaderSettlementUnconfirmed=True,groupSettlementUnobserved=True)
 return result

def main(argv=None):
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--acknowledge-exclusive-window',action='store_true');a=p.parse_args(argv)
 if not a.acknowledge_exclusive_window:raise RuntimeError('explicit root coordination required')
 os.umask(0o077);sys.path.insert(0,str(ROOT));import operation_common as c
 c.private_dir(ROOT)
 actual={n:c.sha(c.read_bounded(ROOT/n))for n in PINS}
 c.need(len(PINS)==3 and actual==PINS,'exact-reviewed-source-pins-required')
 stamp=str(time.monotonic_ns());out=ROOT/('root-operation-'+stamp);out.mkdir(mode=0o700)
 mono=time.monotonic_ns();coord={'version':2,'scope':'cloud054-exclusive-tools','exclusive':True,'activeOperations':[],'bootID':Path('/proc/sys/kernel/random/boot_id').read_text().strip(),'monotonicNS':mono,'expiresMonotonicNS':mono+600_000_000_000,'ownerPID':os.getpid(),'ownerStartTicks':c.process_start(os.getpid()),'acceptedUnknownProcesses':ACCEPTED,'operationSources':PINS,'limits':'Root asserts this bounded tool window. Exact unreadable dockerd/containerd/gpg-agent identities remain unobserved; no global idle claim, no environ reads or signals to them. Known active tool references always refuse.'}
 cp=out/'coordination.json';save(cp,coord);receipt=ROOT/('operation-'+stamp)
 cmd=[sys.executable,'-B',str(ROOT/'restore_go.py'),'--execute-restoration','--quiescence',str(cp),'--quiescence-sha256',c.sha(c.read_bounded(cp)),'--receipt',str(receipt)]
 meta={'command':cmd,'operationReceipt':str(receipt),'status':'starting','wrapperSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'workSeconds':WORK_SECONDS,'termGraceSeconds':TERM_SECONDS,'killSettlementSeconds':KILL_SECONDS,'maximumProcessWaitSeconds':420,'scope':'420s bounds process waits; Python/OS scheduling and synchronous receipt I/O are not preemptively bounded. Partial operations never auto-retry or auto-rollback.'};mp=out/'wrapper.json';save(mp,meta);timed=False;forced=False;code=None;proc=None;settlement=None
 try:
  with (out/'stdout.log').open('xb')as stdout,(out/'stderr.log').open('xb')as stderr:
   proc=subprocess.Popen(cmd,stdout=stdout,stderr=stderr,start_new_session=True);meta['pid']=proc.pid
   try:meta['startTicks']=c.process_start(proc.pid)
   except (FileNotFoundError,ProcessLookupError,c.Refusal):meta['startIdentityUnavailable']=True
   save(mp,meta)
   try:code=proc.wait(timeout=WORK_SECONDS)
   except subprocess.TimeoutExpired:
    timed=True;meta.update(status='timeout-cleanup',operationState='uncertain; retain partial receipts; no automatic retry')
    try:save(mp,meta)
    finally:settlement=settle_owned(proc,meta,c)
    code=settlement.get('exitCode');forced=bool(settlement.get('killSent'));meta['boundedSettlement']=settlement
  meta.update(status='finished',exitCode=code,timedOut=timed,forced=forced,passed=(code==0 and not timed and not forced));save(mp,meta)
 except BaseException as e:
  # Settlement must run even if any durable checkpoint is unwritable.
  if proc is not None and settlement is None:settlement=settle_owned(proc,meta,c)
  meta.update(status='watchdog-failed',failureType=type(e).__name__,passed=False,operationState='uncertain; do not retry or remove partial paths',boundedSettlement=settlement)
  try:save(mp,meta)
  except BaseException as journal_error:
   print(json.dumps({'passed':False,'receiptWriteUnavailable':True,'errorType':type(journal_error).__name__,'settlement':settlement}),file=sys.stderr,flush=True)
  raise
 print(json.dumps({'passed':meta['passed'],'wrapper':str(mp),'receipt':str(receipt)}),flush=True);return 0 if meta['passed']else 1
if __name__=='__main__':raise SystemExit(main())
