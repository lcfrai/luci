from pathlib import Path
import argparse,hashlib,json,os,signal,subprocess,sys,time
root=Path('/workspace/.luci-tools/cloud054-runtime-operations-v2')
sys.path.insert(0,str(root));import operation_common as c
p=argparse.ArgumentParser();p.add_argument('--tool',choices=['rust-1.99.0','powershell-7.5.5'],required=True);p.add_argument('--direction',choices=['to-root','return-to-tmp'],required=True);p.add_argument('--forward');a=p.parse_args();os.umask(0o077)
accepted=[{'pid':203,'startTicks':'1521'},{'pid':254,'startTicks':'1534'},{'pid':260700,'startTicks':'2331192'}]
expected_common='9ba913d11cf08e0696b0d47c7da02687ccb0b09ef1aaf1aff185e1c12bc9a77c'
assert c.sha(c.read_bounded(root/'operation_common.py'))==expected_common
stamp=str(time.monotonic_ns());out=root/('root-'+a.tool+'-'+a.direction+'-'+stamp);out.mkdir(mode=0o700)
mono=time.monotonic_ns();coord={'version':2,'scope':'cloud054-exclusive-tools','exclusive':True,'activeOperations':[],'bootID':Path('/proc/sys/kernel/random/boot_id').read_text().strip(),'monotonicNS':mono,'expiresMonotonicNS':mono+600_000_000_000,'ownerPID':os.getpid(),'ownerStartTicks':c.process_start(os.getpid()),'acceptedUnknownProcesses':accepted,'operationSources':{name:c.sha(c.read_bounded(root/name)) for name in ['operation_common.py','relocate_tools.py']},'limits':'Operator tool window; exact unreadable dockerd/containerd/gpg identities remain unobserved. No universal inactivity or right to inspect their environments/signal them. All known task compilers/tool users are stopped.'}
coordpath=out/'coordination.json';coordpath.write_text(json.dumps(coord,indent=2)+'\n');pin=c.sha(coordpath.read_bytes());receipt=root/('operation-'+a.tool+'-'+a.direction+'-'+stamp)
cmd=[sys.executable,'-B',str(root/'relocate_tools.py'),'--execute-relocation','--tool',a.tool,'--direction',a.direction,'--quiescence',str(coordpath),'--quiescence-sha256',pin,'--receipt',str(receipt)]
if a.forward:
 f=Path(a.forward);cmd+=['--forward-receipt',str(f),'--forward-receipt-sha256',c.sha(c.read_bounded(f))]
meta={'command':cmd,'coordination':str(coordpath),'operationReceipt':str(receipt),'status':'starting','wrapperSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'hardEnvelopeSeconds':420}
(out/'wrapper.json').write_text(json.dumps(meta,indent=2)+'\n');timed=False
with (out/'stdout.log').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 proc=subprocess.Popen(cmd,stdout=stdout,stderr=stderr,start_new_session=True);meta.update(pid=proc.pid,startTicks=c.process_start(proc.pid));(out/'wrapper.json').write_text(json.dumps(meta,indent=2)+'\n')
 try:code=proc.wait(timeout=400)
 except subprocess.TimeoutExpired:
  timed=True;meta.update(status='timeout-cleanup',operationState='uncertain; retain partial receipts; no automatic retry');(out/'wrapper.json').write_text(json.dumps(meta,indent=2)+'\n');os.killpg(proc.pid,signal.SIGTERM)
  try:code=proc.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);code=proc.wait(timeout=10)
meta.update(status='finished',exitCode=code,timedOut=timed);(out/'wrapper.json').write_text(json.dumps(meta,indent=2)+'\n')
print(json.dumps({'exitCode':code,'timedOut':timed,'wrapper':str(out/'wrapper.json'),'receipt':str(receipt)}),flush=True)
raise SystemExit(code if not timed else 1)
