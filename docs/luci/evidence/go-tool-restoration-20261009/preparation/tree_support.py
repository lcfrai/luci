"""Prepared ONLY. Reversible relocation of two pinned task tool trees, no general mover."""
from pathlib import Path
import argparse,ctypes,json,os,shutil,stat,subprocess,sys
from operation_common import ROOT,Journal,need,read_bounded,pinned_json,hash_file,identity,quiescence,process_scan,require_idle,Refusal
TMP=Path('/tmp/luci-retained-tools');COMPAT=Path('/workspace/.luci-tools');STAGE=ROOT/'retained-tools'
TOOLS={'rust-1.99.0':{'manifest':COMPAT/'rust-relocation-20261009/tree-before.json','sha256':'e42e818305a6409e61aca69020707f9703747d06debdf681377b6a5715440e1d','bytes':620087163},'powershell-7.5.5':{'manifest':COMPAT/'tool-relocation-20261009T1158/powershell-7.5.5/tree-before.json','sha256':'5ba3b502a34bb6c8978cbc09dd9fc229f082ba26eb5a801922d4d50af64d215b','bytes':192479038}}
def fixed_paths(tool,direction):
 need(tool in TOOLS and direction in ['to-root','return-to-tmp'],'unsupported-tool-or-direction');source=TMP/tool;dest=STAGE/tool;scratch=TMP/('.'+tool+'-cloud054-'+direction)
 return {'tmp':source,'root':dest,'scratch':scratch,'compat':COMPAT/tool,'source':source if direction=='to-root' else dest,'destination':dest if direction=='to-root' else scratch}
def check_link(path,target):need(path.is_symlink() and os.readlink(path)==str(target),'exact-compatibility-link-required')
def canonical_directory(p):s=p.lstat();need(p.resolve()==p and stat.S_ISDIR(s.st_mode) and s.st_uid==os.getuid(),'owned-canonical-directory-required');return s
def manifest(root):
 canonical_directory(root);rows=[];inodes=set();logical=0
 for p in [root]+sorted(root.rglob('*')):
  need(len(rows)<20000,'tool-entry-bound');s=p.lstat();row={'path':p.relative_to(root).as_posix(),'mode':stat.S_IMODE(s.st_mode),'uid':s.st_uid,'gid':s.st_gid,'mtimeNs':s.st_mtime_ns}
  if stat.S_ISREG(s.st_mode):
   need(s.st_nlink==1 and (s.st_dev,s.st_ino) not in inodes,'unexpected-hardlink');inodes.add((s.st_dev,s.st_ino));digest,after=hash_file(p);need(identity(s)==identity(after),'tree-file-changed');row.update(type='file',bytes=s.st_size,sha256=digest);logical+=s.st_size
  elif stat.S_ISDIR(s.st_mode):row['type']='directory'
  else:raise Refusal('unexpected-symlink-or-special-entry')
  rows.append(row)
 return {'entries':rows,'logicalBytes':logical,'symlinks':0,'repeatedRegularInodes':0,'externalHardlinks':0,'scope':'Exact known trees historically contain only ordinary directories/single-link files; symlink/hardlink surprises refuse, never normalize.'}
def verify(root,expected):
 m=manifest(root);need(m['entries']==expected,'tool-manifest-mismatch');return m
def exchange(a,b):
 need(a.parent==b.parent==TMP,'exchange-outside-approved-parent');libc=ctypes.CDLL(None,use_errno=True);fn=libc.renameat2;fn.argtypes=[ctypes.c_int,ctypes.c_char_p,ctypes.c_int,ctypes.c_char_p,ctypes.c_uint];fn.restype=ctypes.c_int
 if fn(-100,os.fsencode(a),-100,os.fsencode(b),2):raise OSError(ctypes.get_errno(),'Atomic exchange refused')
def fsync_directory(p):
 fd=os.open(p,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
 try:os.fsync(fd)
 finally:os.close(fd)
def copy_tree(source,dest):
 need(not os.path.lexists(dest),'unknown-destination-exists');dest.mkdir(mode=0o700) # exclusive claim before cp; partial copies retained on failure
 subprocess.run(['/bin/cp','-a','--',str(source)+'/.',str(dest)],check=True,timeout=180,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 for entry in dest.rglob('*'):
  if entry.is_file() and not entry.is_symlink():
   fd=os.open(entry,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
   try:os.fsync(fd)
   finally:os.close(fd)
 for entry in sorted((x for x in dest.rglob('*') if x.is_dir()),key=lambda x:len(x.parts),reverse=True):fsync_directory(entry)
 fsync_directory(dest);fsync_directory(dest.parent)
def remove_verified_duplicate(path,expected,expected_inode):
 need(path.parent in [TMP,STAGE] and not path.is_symlink(),'duplicate-path-unsafe');need(path.lstat().st_ino==expected_inode,'duplicate-root-replaced');verify(path,expected);need(shutil.rmtree.avoids_symlink_attacks,'descriptor-safe-tree-removal-unavailable');shutil.rmtree(path);fsync_directory(path.parent)
def main(argv=None):
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--execute-relocation',action='store_true');p.add_argument('--tool',choices=list(TOOLS),required=True);p.add_argument('--direction',choices=['to-root','return-to-tmp'],required=True);p.add_argument('--quiescence',type=Path,required=True);p.add_argument('--quiescence-sha256',required=True);p.add_argument('--forward-receipt',type=Path);p.add_argument('--forward-receipt-sha256');p.add_argument('--receipt',type=Path,required=True);a=p.parse_args(argv)
 need(a.execute_relocation,'explicit-execute-relocation-required');need(a.receipt.parent==ROOT and not os.path.lexists(a.receipt),'fresh-receipt-under-operation-root-required');j=Journal(a.receipt,'relocate-exact-task-tool',[Path(__file__).resolve(),ROOT/'operation_common.py']);paths=fixed_paths(a.tool,a.direction);pins=TOOLS[a.tool]
 try:
  expected=pinned_json(pins['manifest'],pins['sha256']);need(sum(x.get('bytes',0) for x in expected)==pins['bytes'],'manifest-size-pin-mismatch');canonical_directory(TMP);check_link(paths['compat'],paths['tmp']);need(not os.path.lexists(paths['scratch']),'retained-scratch-exists')
  if a.direction=='to-root':
   need(not os.path.lexists(paths['root']),'destination-exists');canonical_directory(paths['tmp'])
   if not STAGE.exists():STAGE.mkdir(mode=0o700);fsync_directory(ROOT)
   canonical_directory(STAGE);need(not(STAGE.stat().st_mode&0o077),'private-stage-required')
  else:
   need(a.forward_receipt is not None and a.forward_receipt_sha256 is not None,'pinned-forward-receipt-required');forward=pinned_json(a.forward_receipt,a.forward_receipt_sha256)
   need(forward.get('passed') is True and forward.get('tool')==a.tool and forward.get('direction')=='to-root' and forward.get('manifestSHA256')==pins['sha256'],'forward-receipt-contract');check_link(paths['tmp'],paths['root']);canonical_directory(paths['root'])
  coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');observed=process_scan([paths['tmp'],paths['root'],paths['compat']]);j.save('preflight-observations',tool=a.tool,direction=a.direction,manifestSHA256=pins['sha256'],paths={k:str(v) for k,v in paths.items()},coordination=coordination,processObservation=observed);require_idle(observed,coordination)
  before=verify(paths['source'],expected);need(before['logicalBytes']==pins['bytes'],'source-byte-total-mismatch');source_inode=paths['source'].lstat().st_ino
  disk=os.statvfs(paths['destination'].parent);need(disk.f_bavail*disk.f_frsize>=pins['bytes']+1024**3,'destination-headroom-refused');j.save('copy-pending',beforeManifest=before,destinationAvailableBytes=disk.f_bavail*disk.f_frsize)
  copy_tree(paths['source'],paths['destination']);verify(paths['destination'],expected);verify(paths['source'],expected);j.save('copy-verified')
  coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');fresh=process_scan([paths['tmp'],paths['root'],paths['compat'],paths['scratch']]);j.save('switch-preflight',lastProcessObservation=fresh);require_idle(fresh,coordination);check_link(paths['compat'],paths['tmp']);verify(paths['source'],expected);verify(paths['destination'],expected)
  if a.direction=='to-root':
   os.symlink(str(paths['root']),paths['scratch']);j.save('exchange-pending');exchange(paths['tmp'],paths['scratch']);fsync_directory(TMP);check_link(paths['tmp'],paths['root']);verify(paths['scratch'],expected);verify(paths['root'],expected);j.save('exchange-observed')
   fresh=process_scan([paths['tmp'],paths['root'],paths['compat'],paths['scratch']]);j.save('duplicate-retirement-pending',lastProcessObservation=fresh);coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');require_idle(fresh,coordination);remove_verified_duplicate(paths['scratch'],expected,source_inode);verify(paths['root'],expected)
  else:
   check_link(paths['tmp'],paths['root']);j.save('exchange-pending');exchange(paths['tmp'],paths['scratch']);fsync_directory(TMP);check_link(paths['scratch'],paths['root']);verify(paths['tmp'],expected);verify(paths['root'],expected);j.save('exchange-observed');paths['scratch'].unlink();fsync_directory(TMP)
   fresh=process_scan([paths['tmp'],paths['root'],paths['compat']]);j.save('duplicate-retirement-pending',lastProcessObservation=fresh);coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-tools');require_idle(fresh,coordination);remove_verified_duplicate(paths['root'],expected,source_inode);verify(paths['tmp'],expected)
  check_link(paths['compat'],paths['tmp']);j.finish(True,tool=a.tool,direction=a.direction,manifestSHA256=pins['sha256'],compatibilityTarget=str(paths['tmp']),metadataLimits='Bytes/mode/uid/gid/mtime and absence of symlink/hardlink surprises verified. Cross-filesystem inode/ctime/atime not preserved. Disk observations not resource or performance acceptance.');return 0
 except Exception as e:j.save('failed',failureType=type(e).__name__,failureCode=str(e) if isinstance(e,Refusal) else 'operation-failed-or-uncertain',retainedPartialPaths={k:str(v) for k,v in paths.items()},automaticRollback=False);return 1
if __name__=='__main__':
 try:sys.exit(main())
 except Exception as e:print(json.dumps({'passed':False,'code':str(e) if isinstance(e,Refusal) else 'operation-preflight-failed'}));sys.exit(1)
