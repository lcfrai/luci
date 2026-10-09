#!/usr/bin/env python3
"""Only pinned425 reversible placement and historical333 after verified760 publication."""
from pathlib import Path
import argparse,hashlib,importlib.util,json,os,stat,time
ROOT=Path('/workspace/.luci-tools');TMP=Path('/tmp/luci-retained-tools')
OLD_HELPER=ROOT/'relocate-historical425-v1/relocate.py';OLD_PIN='5a5335c6d980c4744fbc2c134cef282d4c2c594a771eb5ca170e63d72d8c7f18'
PAIR425=(ROOT/'runtime-archive-7e1b1f608cfa/verified-luci-sandbox-dev.tar.gz',ROOT/'runtime-archive-1f845cfd2b45/old-luci-sandbox-dev.tar.gz')
FILE425=TMP/'historical-runtime-42597a-c593f37e.tar.gz';PIN425='c593f37eaa3795358aa1b76dd355828ddaadfcf503b9a5e3a1aea6976c947c91';SIZE425=1013359941
FILE333=TMP/'historical-runtime-333eff-7caf7aab.tar.gz';PIN333='7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3';SIZE333=1013367768
PROCESS_HELPER=ROOT/'go-tool-restore-v1/operation_common.py';PROCESS_PIN='1863d7908931bfd4463b73139a550930a3f3092ce0408a707411c2726956478c'
ACCEPTED_UNKNOWN=[{'pid':203,'startTicks':'1521'},{'pid':254,'startTicks':'1534'},{'pid':260700,'startTicks':'2331192'}]
IMAGE760='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8';MINIMUM=500<<20;GUARD=64<<20

def need(ok,text):
 if not ok:raise RuntimeError(text)
def digest(p):
 with Path(p).open('rb')as f:return hashlib.file_digest(f,'sha256').hexdigest()
def load():
 need(digest(OLD_HELPER)==OLD_PIN,'reviewed-placement-primitives-changed');s=importlib.util.spec_from_file_location('original425_move',OLD_HELPER);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

def observe(paths):
 need(digest(PROCESS_HELPER)==PROCESS_PIN,'process-observer-source-changed');spec=importlib.util.spec_from_file_location('archive_process_observer',PROCESS_HELPER);c=importlib.util.module_from_spec(spec);spec.loader.exec_module(c);scan=c.process_scan(paths);c.require_idle(scan,{'scope':'cloud054-exclusive-tools','acceptedUnknownProcesses':ACCEPTED_UNKNOWN});return scan

def pair_meta(m,pair,size):
 rows=[m.met(p)for p in pair];need(all(not p.is_symlink()and stat.S_ISREG(p.lstat().st_mode)for p in pair),'regular-pair-required');need(rows[0]==rows[1]and rows[0]['nlink']==2 and rows[0]['mode']==0o600 and rows[0]['uid']==os.getuid()and rows[0]['bytes']==size,'exact-owned-hardlink-pair-required');return rows[0]
def link(p,target):need(p.is_symlink()and os.readlink(p)==str(target),'exact-alias-required')

def copy(m,source,destination,size,pin,minimum):
 need(not os.path.lexists(destination),'copy-destination-occupied');before=m.met(source);need(not source.is_symlink()and stat.S_ISREG(source.lstat().st_mode)and before['bytes']==size and before['mode']==0o600 and before['uid']==os.getuid(),'owned-exact-file-required');need(m.sha(source)==pin,'original-byte-pin-mismatch');need(m.free(destination.parent)>=size+minimum+GUARD,'destination-copy-budget-refused')
 fd=os.open(destination,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600);total=0;h=hashlib.sha256()
 with source.open('rb')as src,os.fdopen(fd,'wb')as out:
  while b:=src.read(1<<20):
   m.bound();need(m.free(destination.parent)>=minimum+GUARD,'destination-working-floor-refused');total+=len(b);need(total<=size,'copy-byte-cap-exceeded');h.update(b);out.write(b)
  out.flush();os.fsync(out.fileno())
 need(total==size and h.hexdigest()==pin and m.sha(destination)==pin and m.met(source)==before,'copy-or-source-changed');os.chmod(destination,before['mode']);os.utime(destination,ns=(destination.stat().st_atime_ns,before['mtimeNs']));need(destination.stat().st_uid==before['uid']and destination.stat().st_gid==before['gid'],'copy-owner-mismatch');m.fsyncdir(destination.parent);return before

def main(argv=None):
 p=argparse.ArgumentParser();p.add_argument('--window-granted',action='store_true',required=True);p.add_argument('--mode',choices=['425-to-root','425-to-tmp','333-to-tmp'],required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--published760',type=Path);p.add_argument('--published760-sha256');a=p.parse_args(argv);m=load();m.DUE=time.monotonic()+180;os.umask(0o077)
 need(a.output.parent==ROOT and a.output.name.startswith('runtime760-placement-')and not os.path.lexists(a.output),'fresh-fixed-operation-path-required');m.chain(ROOT);m.chain(TMP);a.output.mkdir(mode=0o700);ordinal=0
 def record(stage,**v):
  nonlocal ordinal
  ordinal+=1;m.write(a.output/f'{ordinal:03d}-{stage}.json',{'stage':stage,'at':m.now(),**v})
 pair=PAIR425;target=FILE425;size=SIZE425;pin=PIN425;protected=ROOT/'luci-sandbox-dev.tar.gz';replaced=[]
 try:
  m.chain(protected);protected_meta=m.met(protected)
  if a.mode=='333-to-tmp':
   need(a.published760 is not None and a.published760_sha256 is not None,'published760-receipt-required');rp=a.published760.resolve(strict=True);need(rp.parent.parent==ROOT and rp.parent.name.startswith('runtime-archive-760-')and rp.stat().st_size<1<<20 and not a.published760.is_symlink(),'owned760-publication-receipt-required');need(m.sha(rp)==a.published760_sha256,'published760-receipt-pin-mismatch');r=json.loads(rp.read_bytes());need(r.get('status')=='published'and r.get('candidateImage')==IMAGE760 and r.get('archiveVerification',{}).get('status')=='passed'and r.get('stage')==str(rp.parent),'verified760-publication-required');need(m.sha(protected)==r['archiveVerification']['archiveSHA256'],'required760-archive-byte-mismatch');need(all(m.sha(ROOT/n)==v for n,v in r['newPins'].items()),'required760-pins-mismatch');pair=(ROOT/'runtime-archive-1f845cfd2b45/verified-luci-sandbox-dev.tar.gz',rp.parent/'old-luci-sandbox-dev.tar.gz');target=FILE333;size=SIZE333;pin=PIN333
  receipts=[ROOT/'runtime-archive-7e1b1f608cfa/receipt.json',ROOT/'runtime-archive-1f845cfd2b45/receipt.json'];receipt_pins={str(p):m.sha(p)for p in receipts};beforefree={p:m.free(p)for p in ['/workspace','/tmp']};scan=observe([*pair,target]);record('intent',processObservation=scan,mode=a.mode,sourceSHA256=m.sha(Path(__file__)),pair=[str(p)for p in pair],tmpPath=str(target),archiveSHA256=pin,archiveBytes=size,freeBefore=beforefree,protectedRequiredArchiveMetadata=protected_meta,originalReceiptHashes=receipt_pins,coordination='Root grants bounded tool/archive-only window; no Docker access, no process signals/environ reads, no global idle proof.',retention='Historical TMP is not guaranteed snapshot retention. Exact bytes and compatibility paths preserved; hardlink/inode topology changes are explicit.')
  if a.mode=='425-to-root':
   for p in pair:link(p,target)
   need(target.stat().st_nlink==1,'single-original-tmp-file-required');scratch=pair[0].with_name('.restore425-'+a.output.name);before=copy(m,target,scratch,size,pin,3905250841);record('root-copy-verified',originalMetadata=before,copy=str(scratch),sha256=pin)
   link(pair[0],target);link(pair[1],target);scan=observe([*pair,target,scratch]);record('first-root-replacement-pending',processObservation=scan);os.replace(scratch,pair[0]);m.fsyncdir(pair[0].parent);replaced.append(str(pair[0]));second=pair[1].with_name('.restore425-'+a.output.name);need(not os.path.lexists(second),'second-root-stage-exists');os.link(pair[0],second,follow_symlinks=False);os.replace(second,pair[1]);m.fsyncdir(pair[1].parent);replaced.append(str(pair[1]));pair_meta(m,pair,size);need(m.sha(pair[0])==pin and m.met(target)==before,'root-copy-or-original-changed');record('tmp-retirement-pending',replaced=replaced,processObservation=observe([*pair,target]));tmp=target.with_name('.restore425-link-'+a.output.name);m.publish_link(target,pair[0],tmp);replaced.append(str(target));link(target,pair[0]);pair_meta(m,pair,size)
  else:
   for p in pair:m.chain(p)
   before=pair_meta(m,pair,size)
   if a.mode=='425-to-tmp':link(target,pair[0])
   else:need(not os.path.lexists(target),'historical333-target-already-exists')
   scratch=target.with_name('.copy-'+target.name+'-'+a.output.name);copy(m,pair[0],scratch,size,pin,MINIMUM);record('tmp-copy-verified',originalMetadata=before,copy=str(scratch),sha256=pin)
   need(pair_meta(m,pair,size)==before,'pair-changed-before-switch');record('tmp-switch-preflight',processObservation=observe([*pair,target,scratch]));os.replace(scratch,target);m.fsyncdir(TMP)
   for i,p in enumerate(pair):
    s=m.met(p);need(s['dev']==before['dev']and s['ino']==before['ino']and s['nlink']==2-i,'pair-identity-changed');record('compatibility-replacement-pending',path=str(p),expectedNlink=2-i);m.publish_link(p,target,p.with_name('.archive-link-'+a.output.name));replaced.append(str(p));link(p,target)
   need(m.sha(target)==pin,'final-historical-byte-mismatch')
  need(protected_meta==m.met(protected),'required-archive-metadata-changed');need(receipt_pins=={str(p):m.sha(p)for p in receipts},'original-receipt-changed');afterfree={p:m.free(p)for p in ['/workspace','/tmp']};record('finished',passed=True,mode=a.mode,archiveSHA256=pin,archiveBytes=size,replaced=replaced,freeBefore=beforefree,freeAfter=afterfree,scope='Placement only, no Docker load/export/admission proof. Original archive bytes and original receipts preserved. Historical TMP retention not guaranteed.')
 except BaseException as e:record('failed',passed=False,errorType=type(e).__name__,failure=str(e)[:300],replaced=replaced,scope='Partial copies/topology retained, no automatic rollback/retry/deletion. Inspect exact paths and receipts before further operation.');raise
if __name__=='__main__':main()
