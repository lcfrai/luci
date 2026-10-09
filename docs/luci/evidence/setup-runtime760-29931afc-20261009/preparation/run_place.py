"""Root-only300s watchdog around exact archive placement; no operations on import."""
from pathlib import Path
import argparse,hashlib,importlib.util,json,os,subprocess,sys,time
ROOT=Path(__file__).resolve().parent;PINS={'place_archives.py': 'bae67ba8f1d6907e0347d62bdd44b9d201c8d781d402bace72efe9904df81ec0', 'run_archive.py': 'd92d065966d142033f37fb1a3aeaeb7631d4f701a98772cf450ddc64d1f22689'}
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main(argv=None):
 p=argparse.ArgumentParser();p.add_argument('--window-granted',action='store_true',required=True);p.add_argument('--mode',choices=['425-to-root','425-to-tmp','333-to-tmp'],required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--published760',type=Path);p.add_argument('--published760-sha256');a=p.parse_args(argv);os.umask(0o077)
 if len(PINS)!=2 or any(sha(ROOT/n)!=v for n,v in PINS.items()):raise RuntimeError('sealed-placement-source-changed')
 spec=importlib.util.spec_from_file_location('reviewed_archive_watchdog',ROOT/'run_archive.py');w=importlib.util.module_from_spec(spec);spec.loader.exec_module(w)
 out=ROOT/('place-watchdog-'+str(time.monotonic_ns()));out.mkdir(mode=0o700);cmd=[sys.executable,'-B',str(ROOT/'place_archives.py'),'--window-granted','--mode',a.mode,'--output',str(a.output)]
 if a.published760:cmd+=['--published760',str(a.published760),'--published760-sha256',a.published760_sha256 or'']
 meta={'command':cmd,'wrapperSHA256':sha(Path(__file__)),'processWaitSeconds':300,'maximumSettlementWaitSeconds':20,'scope':'Only exact spawned Python/group; no Docker subprocess. OS I/O not preemptively bounded. Forced or unknown result fails; retain partial topology.'};path=out/'wrapper.json';w.write(path,meta);proc=None;ticks=None;failed=False
 try:
  with(out/'stdout.log').open('xb')as stdout,(out/'stderr.log').open('xb')as stderr:
   proc=subprocess.Popen(cmd,stdout=stdout,stderr=stderr,start_new_session=True);ticks=w.start(proc.pid);meta.update(pid=proc.pid,startTicks=ticks);w.write(path,meta);code=proc.wait(timeout=300);failed=code!=0;meta.update(exitCode=code,passed=code==0)
 except BaseException as e:failed=True;meta.update(passed=False,errorType=type(e).__name__,leaderIdentityUnavailable=proc is not None and ticks is None)
 finally:
  if failed and proc is not None and ticks is not None:meta['settlement']=w.settle(proc.pid,ticks)
  try:w.write(path,meta)
  except BaseException:print(json.dumps({'passed':False,'durableFinalReceiptUnavailable':True,'settlement':meta.get('settlement')}),file=sys.stderr);return 1
 print(json.dumps({'passed':meta.get('passed',False),'wrapper':str(path),'operation':str(a.output)}));return 0 if meta.get('passed')else 1
if __name__=='__main__':raise SystemExit(main())
