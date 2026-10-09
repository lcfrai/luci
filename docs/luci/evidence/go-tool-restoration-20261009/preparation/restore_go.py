"""Prepared Go-only durable workspace restoration, never a generic tool mover."""
from pathlib import Path
import argparse,ctypes,json,os,sys
from operation_common import ROOT,Journal,need,read_bounded,sha,quiescence,process_scan,require_idle,Refusal
from tree_support import canonical_directory,manifest,verify,copy_tree,fsync_directory,check_link,remove_verified_duplicate
TMP=Path('/tmp/luci-retained-tools');WORK=Path('/workspace/.luci-tools')
ORIGINAL=TMP/'go1.26.3';FINAL=WORK/'go1.26.3';STAGE=WORK/'.go1.26.3-workspace-restore-stage';SCRATCH=TMP/'.go1.26.3-retired-original'
MANIFEST=WORK/'tool-relocation-20261009T1158/go1.26.3/tree-before.json';PIN='50b4a84d580c4b072378c2cea371854fa750c29ef512c634b0282c365a57f242';BYTES=232365810;COUNT=16688
ROOT_RUNTIME_FLOOR=6736759858;WORKING_GUARD=64*1024*1024
SOURCES=[ROOT/n for n in ['operation_common.py','restore_go.py','tree_support.py']]

def exchange(a,b,parent):
 need(parent in [TMP,WORK]and a.parent==b.parent==parent,'exchange-outside-fixed-parent')
 libc=ctypes.CDLL(None,use_errno=True);fn=libc.renameat2;fn.argtypes=[ctypes.c_int,ctypes.c_char_p,ctypes.c_int,ctypes.c_char_p,ctypes.c_uint];fn.restype=ctypes.c_int
 if fn(-100,os.fsencode(a),-100,os.fsencode(b),2):raise OSError(ctypes.get_errno(),'Atomic exchange refused')
def free(p):return os.statvfs(p).f_bavail*os.statvfs(p).f_frsize

def main(argv=None):
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--execute-restoration',action='store_true');p.add_argument('--quiescence',type=Path,required=True);p.add_argument('--quiescence-sha256',required=True);p.add_argument('--receipt',type=Path,required=True);a=p.parse_args(argv)
 need(a.execute_restoration,'explicit-root-restoration-required');need(a.receipt.parent==ROOT and not os.path.lexists(a.receipt),'fresh-private-operation-path-required');j=Journal(a.receipt,'restore-exact-go-to-real-workspace',SOURCES)
 paths={'originalTMP':str(ORIGINAL),'finalRealWorkspace':str(FINAL),'stage':str(STAGE),'scratch':str(SCRATCH)}
 try:
  raw=read_bounded(MANIFEST,8*1024*1024);need(sha(raw)==PIN,'exact-go-manifest-pin-required');expected=json.loads(raw);need(len(expected)==COUNT and sum(x.get('bytes',0)for x in expected)==BYTES,'exact-go-tree-size-required')
  canonical_directory(TMP);canonical_directory(WORK);canonical_directory(ORIGINAL);check_link(FINAL,ORIGINAL);need(not os.path.lexists(STAGE)and not os.path.lexists(SCRATCH),'restoration-staging-path-already-occupied')
  coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');scan=process_scan([ORIGINAL,FINAL,STAGE,SCRATCH]);j.save('preflight-observations',paths=paths,manifestSHA256=PIN,coordination=coordination,processObservation=scan);require_idle(scan,coordination)
  before=verify(ORIGINAL,expected);source_inode=ORIGINAL.lstat().st_ino;initial={'/workspace':free(WORK),'/tmp':free(TMP)}
  need(initial['/workspace']>=BYTES+ROOT_RUNTIME_FLOOR+WORKING_GUARD,'root-headroom-before-copy-refused');j.save('copy-pending',beforeManifestBinding={'path':str(MANIFEST),'sha256':PIN,'entries':len(before['entries']),'logicalBytes':before['logicalBytes'],'observedEqualsPinnedManifest':True},freeBefore=initial,workspaceFloorBytes=ROOT_RUNTIME_FLOOR,workingGuardBytes=WORKING_GUARD)
  copy_tree(ORIGINAL,STAGE);verify(STAGE,expected);verify(ORIGINAL,expected);need(free(WORK)>=ROOT_RUNTIME_FLOOR+WORKING_GUARD,'root-headroom-after-copy-refused');j.save('copy-verified')
  coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');scan=process_scan([ORIGINAL,FINAL,STAGE,SCRATCH]);j.save('workspace-switch-preflight',processObservation=scan);require_idle(scan,coordination);check_link(FINAL,ORIGINAL);verify(ORIGINAL,expected);verify(STAGE,expected)
  j.save('workspace-exchange-pending');exchange(FINAL,STAGE,WORK);fsync_directory(WORK);canonical_directory(FINAL);check_link(STAGE,ORIGINAL);verify(FINAL,expected);verify(ORIGINAL,expected);j.save('workspace-real-tree-observed')
  STAGE.unlink();fsync_directory(WORK);need(not os.path.lexists(STAGE),'old-workspace-alias-retirement-failed')
  os.symlink(str(FINAL),SCRATCH);j.save('tmp-exchange-pending');exchange(ORIGINAL,SCRATCH,TMP);fsync_directory(TMP);check_link(ORIGINAL,FINAL);verify(SCRATCH,expected);verify(FINAL,expected);j.save('tmp-compatibility-observed')
  coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');scan=process_scan([ORIGINAL,FINAL,STAGE,SCRATCH]);j.save('duplicate-retirement-pending',processObservation=scan);require_idle(scan,coordination);remove_verified_duplicate(SCRATCH,expected,source_inode);verify(FINAL,expected);canonical_directory(FINAL);check_link(ORIGINAL,FINAL)
  after={'/workspace':free(WORK),'/tmp':free(TMP)};need(after['/workspace']>=ROOT_RUNTIME_FLOOR,'runtime-root-floor-no-longer-observed');j.finish(True,paths=paths,manifestSHA256=PIN,logicalBytes=BYTES,freeBefore=initial,freeAfter=after,finalWorkspaceRealDirectory=True,tmpCompatibilityTarget=str(FINAL),metadataLimits='All bytes/mode/uid/gid/mtime verified against pinned historical Go manifest. Cross-filesystem inode/ctime/atime are not preserved. Real workspace tree no longer depends on TMP retention. No Go compiler/version command or app started by this operation; root checks tool execution separately. Neither export readiness nor future admission is certified.')
  return 0
 except Exception as e:
  j.save('failed',failureType=type(e).__name__,failureCode=str(e)if isinstance(e,Refusal)else'operation-failed-or-uncertain',retainedPartialPaths=paths,automaticRollback=False);return 1
if __name__=='__main__':
 try:sys.exit(main())
 except Exception as e:print(json.dumps({'passed':False,'code':str(e)if isinstance(e,Refusal)else'operation-preflight-failed'}));sys.exit(1)
