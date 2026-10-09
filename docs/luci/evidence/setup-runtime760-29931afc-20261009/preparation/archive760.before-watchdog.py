#!/usr/bin/env python3
"""Root-only exact760 retained archive staging/publication; no operations on import."""
from pathlib import Path
import argparse,contextlib,fcntl,gzip,hashlib,importlib.util,json,os,signal,subprocess,time,uuid
from budget import admission,EXPORT_FLOOR,GZIP_CAP
ROOT=Path('/workspace/.luci-tools');HERE=Path(__file__).resolve().parent
HELPER=ROOT/'cloud054-runtime-helper/prepare-runtime.py';HELPER_SHA='85fccb3b256ee373f795b4a6c00e4b108610227545c3b889cf1fdec4d08171dc'
PRIMITIVES=ROOT/'refresh-runtime-archive-333eff.py';PRIMITIVES_SHA='bb58be6474e68749ff09e30bf7b9cf1463355b57cfeaef7333a182cef42cfd2e'
CANDIDATE='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
CANDIDATE_RECEIPT=ROOT/'sandbox-candidate-20261009T134118Z-4a347241/receipt.json';CANDIDATE_PIN='a6fb7d93c1ce81b726923a155e44a5adbd53ed6da1198bafb76e4a82da009331'
ARCHIVE=ROOT/'luci-sandbox-dev.tar.gz';TAG='luci/sandbox:dev';PINS=['sandbox-image-provenance.json','install.sh','start.md']
SAVED={'install.sh':'7686a8d3108b63a0994bfab135dcd75e4c9d6a00b6cb88c997f636f620bc43ca','start.md':'11d06ab5b3585a7909bca2405cc7f6a4694f9bc097f6be82370fc515065333ee','sandbox-image-provenance.json':'12f976de654e63d7fefd1e851e00b05b079508d0d6fc105b2a18cfba6f8ba4d5'}

def digest(p):
 with Path(p).open('rb')as f:return hashlib.file_digest(f,'sha256').hexdigest()
def need(ok,message):
 if not ok:raise RuntimeError(message)
def load(p,pin,name):
 need(digest(p)==pin,'reviewed-source-pin-changed');spec=importlib.util.spec_from_file_location(name,p);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

def stop_owned(proc):
 """Bounded exact spawned-client group settlement; daemon drain stays separate."""
 result={'leaderExited':proc.poll()is not None,'daemonDrainConfirmed':False,'descendantCensus':False}
 if proc.poll()is not None:return result
 try:
  os.killpg(proc.pid,signal.SIGTERM);result['termSent']=True
  try:proc.wait(timeout=10)
  except subprocess.TimeoutExpired:
   if proc.poll()is None:os.killpg(proc.pid,signal.SIGKILL);result['killSent']=True
   try:proc.wait(timeout=10)
   except subprocess.TimeoutExpired:result['leaderSettlementUnknown']=True
 except ProcessLookupError:pass
 result['leaderExited']=proc.poll()is not None;return result

def exact_inputs(h,p):
 p.regular(CANDIDATE_RECEIPT);need(digest(CANDIDATE_RECEIPT)==CANDIDATE_PIN,'original-candidate-receipt-changed');cr=p.read_json(CANDIDATE_RECEIPT)
 need(cr['candidateImage']==CANDIDATE and cr['exportVerification']['status']=='passed','original-export-verification-required')
 h.source_fence();need(h.source_hashes()==cr['sourcesAfter']==cr['sourcesBefore'],'all-runtime-inputs-must-match-original-candidate')
 for n,v in SAVED.items():p.regular(ROOT/n);need(digest(ROOT/n)==v,'saved-setup-input-changed')
 need(h.preserved_state()==cr['preservedBefore'],'retained333-and-saved-pins-changed');return cr

def prepare_pins(h,p,stage,r,cr):
 archive=r['archiveVerification'];scope=('Exact760 archive config/layers/runtime files verified; controlled and installed evidence remains linked at its original scope. No complete product, live-provider, Windows, performance or fresh-daemon-load acceptance follows from this archive operation.')
 provenance={'version':2,'retainedImage':CANDIDATE,'tag':TAG,'sourceCommit':cr['sourceCommit'],'runtimeSourceReuseAt':r['sourceCommit'],'runtimeInputHashes':cr['sourcesAfter'],'trustedBase':cr['baseImage'],'sandboxdSHA256':cr['inputFiles']['sandboxd']['sha256'],'entrypointSHA256':cr['inputFiles']['entrypoint.sh']['sha256'],'archiveSHA256':archive['archiveSHA256'],'archiveBytes':archive['archiveBytes'],'gzipVerified':True,'dockerLoadVerified':False,'archiveManifestVerified':True,'candidateReceipt':str(CANDIDATE_RECEIPT),'candidateReceiptSHA256':CANDIDATE_PIN,'reviewedRuntimeEvidence':r['reviewedRuntimeEvidence'],'previousProvenance':str(stage/'old-sandbox-image-provenance.json'),'scope':scope}
 p.durable_bytes(stage/'new-sandbox-image-provenance.json',(json.dumps(provenance,indent=2)+'\n').encode())
 for name in ['install.sh','start.md']:
  old=(stage/('old-'+name)).read_text();need(old.count(h.ARCHIVE_SHA)==1 and old.count(h.OLD)==1,'exact-old-pin-count-required');new=old.replace(h.ARCHIVE_SHA,archive['archiveSHA256']).replace(h.OLD,CANDIDATE)
  if name=='start.md':new+='\nCurrent retained runtime scope\n------------------------------\n\n'+scope+' The Go toolchain is a real retained workspace directory; optional historical archives in TMP are not startup prerequisites. All historical fixture evidence and failures retain their original revisions. The current exact760 full-sandbox pre-Create disk requirement is6,736,759,858 bytes, in addition to fresh memory/CPU/authority checks. The separate archive export guard remains3,905,250,841 bytes on BOTH known filesystems, plus bounded output space on the archive destination. Runtime/image operations must use the intended environment-provided local Unix Docker endpoint.\n'
  p.durable_bytes(stage/('new-'+name),new.encode(),p.regular(ROOT/name).st_mode&0o777)
 r['newPins']={n:digest(stage/('new-'+n))for n in PINS}

def stage(h,p,a):
 cr=exact_inputs(h,p);e=a.reviewed_runtime_receipt.resolve(strict=True);p.regular(e);need(e.stat().st_size<=16<<20 and digest(e)==a.reviewed_runtime_sha256,'reviewed-evidence-pin-required')
 out=ROOT/('runtime-archive-760-'+uuid.uuid4().hex[:12]);out.mkdir(mode=0o700);p.sync_dir(ROOT);rp=out/'receipt.json';r={'status':'preflight','sourceCommit':a.expected_source,'stage':str(out),'candidateImage':CANDIDATE,'candidateReceipt':str(CANDIDATE_RECEIPT),'candidateReceiptSHA256':CANDIDATE_PIN,'reviewedRuntimeEvidence':{'path':str(e),'sha256':a.reviewed_runtime_sha256,'scope':'Root-selected immutable reviewed context only; this helper does not reinterpret its pass/fail status.'},'verifierSHA256':digest(Path(__file__)),'budgetSourceSHA256':digest(HERE/'budget.py'),'primitivesSHA256':PRIMITIVES_SHA,'runtimeHelperSHA256':HELPER_SHA,'oldPins':SAVED,'preservedBefore':cr['preservedBefore'],'inputFiles':cr['inputFiles'],'startedAt':h.now()};p.journal(rp,r);proc=None;partial=out/'archive.partial.tar.gz'
 try:
  r['connection']=h.configure_docker(out);candidate=h.inspect(TAG);need(candidate['Id']==CANDIDATE,'root-must-first-explicitly-tag-exact760');need({k:candidate[k]for k in cr['candidate']}==cr['candidate'],'candidate-image-config-changed');r['candidate']=cr['candidate']
  need(not subprocess.check_output(h.docker_argv('ps','-aq'),env=h.DOCKER_ENV,text=True,timeout=30).strip(),'exclusive-empty-daemon-required')
  r['budget']=admission(candidate['Size'],h.free('/workspace'),h.free('/tmp'));r['freeBefore']={'root':h.free('/workspace'),'tmp':h.free('/tmp')};r['status']='staging';p.journal(rp,r)
  os.link(ARCHIVE,out/'old-luci-sandbox-dev.tar.gz',follow_symlinks=False);p.sync_dir(out);need(digest(out/'old-luci-sandbox-dev.tar.gz')==h.ARCHIVE_SHA,'old333-rollback-byte-mismatch')
  for n in PINS:p.durable_bytes(out/('old-'+n),(ROOT/n).read_bytes(),p.regular(ROOT/n).st_mode&0o777)
  with partial.open('xb')as output,(out/'docker-save.stderr').open('xb')as stderr:
   bounded=p.LimitedWriter(output,GZIP_CAP);proc=subprocess.Popen(h.docker_argv('image','save',TAG),env=h.DOCKER_ENV,stdout=subprocess.PIPE,stderr=stderr,start_new_session=True)
   try:
    with h.Monitor(proc,out,'docker-save',600,temp_reserve=h.STOP_RESERVE)as monitor:
     raw_count=0
     with gzip.GzipFile(filename='',fileobj=bounded,mode='wb',compresslevel=6,mtime=0)as compressed:
      for chunk in iter(lambda:proc.stdout.read(h.CHUNK),b''):
       raw_count+=len(chunk);need(raw_count<=r['budget']['rawLimitBytes'],'raw-export-cap-exceeded');need(monitor.failure is None,monitor.failure or'export-monitor-failed');compressed.write(chunk)
     code=proc.wait(timeout=20)
    need(monitor.failure is None and code==0,monitor.failure or'docker-save-failed');output.flush();os.fsync(output.fileno());r['rawExportBytes']=raw_count;r['compressedBytes']=bounded.count
   except BaseException:
    r['boundedClientSettlement']=stop_owned(proc);raise
   finally:proc.stdout.close()
  p.sync_dir(out);r['archiveVerification']=p.verify_archive(partial,candidate,cr['inputFiles'],out);exact_inputs(h,p);need(digest(e)==a.reviewed_runtime_sha256,'reviewed-evidence-changed');need(h.inspect(TAG)['Id']==CANDIDATE,'tag-changed')
  complete=out/'verified-luci-sandbox-dev.tar.gz';os.replace(partial,complete);p.sync_dir(out);r['stagedArchive']=str(complete);prepare_pins(h,p,out,r,cr);r.update(status='verified-staged-not-published',finishedAt=h.now(),freeAfter={'root':h.free('/workspace'),'tmp':h.free('/tmp')});p.journal(rp,r)
  print(json.dumps({'receipt':str(rp),'status':r['status'],'archiveSHA256':r['archiveVerification']['archiveSHA256']}))
 except BaseException as error:
  if proc is not None and proc.poll()is None:r['boundedClientSettlement']=stop_owned(proc)
  r.update(status='failed',errorType=type(error).__name__,failure=str(error)[:512],failedAt=h.now(),partialRetained=partial.exists(),daemonDrainConfirmed=False,scope='No automatic retry/pin publication. Incomplete output, original333, setup files and diagnostics remain retained; root must establish exact client/daemon settlement before any other operation.');p.journal(rp,r);raise

def publish(h,p,a):
 rp=a.receipt.resolve(strict=True);out=rp.parent;r=p.read_json(rp);need(out.parent==ROOT and out.name.startswith('runtime-archive-760-')and r.get('stage')==str(out),'exact-owned-archive-stage-required');need(r.get('status')=='verified-staged-not-published','verified-stage-required');need(r['verifierSHA256']==digest(Path(__file__))and r['budgetSourceSHA256']==digest(HERE/'budget.py'),'stage-helper-changed');cr=exact_inputs(h,p)
 e=r['reviewedRuntimeEvidence'];need(digest(Path(e['path']))==e['sha256'],'reviewed-evidence-changed');h.configure_docker(out);need(h.inspect(TAG)['Id']==CANDIDATE,'exact-tag-required');need(not subprocess.check_output(h.docker_argv('ps','-aq'),env=h.DOCKER_ENV,text=True,timeout=30).strip(),'exclusive-empty-daemon-required')
 archive=out/'verified-luci-sandbox-dev.tar.gz';p.regular(archive);need(digest(archive)==r['archiveVerification']['archiveSHA256'],'new-archive-changed');need(digest(out/'old-luci-sandbox-dev.tar.gz')==h.ARCHIVE_SHA,'old333-rollback-changed')
 for n in PINS:need(digest(out/('old-'+n))==SAVED[n]and digest(out/('new-'+n))==r['newPins'][n],'old-new-pins-changed')
 r.update(status='publishing',completedReplacements=[],replacementOrder=['archive',*PINS]);p.journal(rp,r)
 try:
  tmp=ROOT/('.'+out.name+'-new-archive');need(not os.path.lexists(tmp),'temporary-path-exists');os.link(archive,tmp,follow_symlinks=False);os.replace(tmp,ARCHIVE);p.sync_dir(ROOT);r['completedReplacements'].append('archive');p.journal(rp,r)
  for n in PINS:
   tmp=ROOT/('.'+out.name+'-'+n);p.durable_bytes(tmp,(out/('new-'+n)).read_bytes(),p.regular(ROOT/n).st_mode&0o777);os.replace(tmp,ROOT/n);p.sync_dir(ROOT);r['completedReplacements'].append(n);p.journal(rp,r)
  p.regular(ARCHIVE);need(digest(ARCHIVE)==r['archiveVerification']['archiveSHA256'],'published-archive-mismatch');need(all(digest(ROOT/n)==r['newPins'][n]for n in PINS),'published-pin-mismatch');r.update(status='published',publishedAt=h.now(),dockerLoadVerified=False,rollbackDirectory=str(out));p.journal(rp,r);print(json.dumps({'status':'published','receipt':str(rp)}))
 except BaseException as error:r.update(status='publication-interrupted',errorType=type(error).__name__,scope='Per-file replacements are atomic, transaction is not. No automatic rollback. Keep all original/new files and do not run installer until complete pins agree.');p.journal(rp,r);raise

def main(argv=None):
 os.umask(0o077);ap=argparse.ArgumentParser();ap.add_argument('--window-granted',action='store_true',required=True);ap.add_argument('--expected-source',required=True);ap.add_argument('--expected-engine',required=True);sub=ap.add_subparsers(dest='command',required=True);s=sub.add_parser('stage');s.add_argument('--reviewed-runtime-receipt',type=Path,required=True);s.add_argument('--reviewed-runtime-sha256',required=True);pub=sub.add_parser('publish');pub.add_argument('receipt',type=Path);a=ap.parse_args(argv)
 h=load(HELPER,HELPER_SHA,'exact760_runtime_helper');h.EXPECTED_SOURCE=a.expected_source;h.EXPECTED_ENGINE=a.expected_engine;p=load(PRIMITIVES,PRIMITIVES_SHA,'prior_checked_archive_primitives');p.h=h;p.CANDIDATE=CANDIDATE
 signal.signal(signal.SIGTERM,lambda sig,frame:(_ for _ in ()).throw(SystemExit(128+sig)))
 with(ROOT/'.runtime-archive.lock').open('a')as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  if a.command=='stage':stage(h,p,a)
  else:publish(h,p,a)
if __name__=='__main__':main()
