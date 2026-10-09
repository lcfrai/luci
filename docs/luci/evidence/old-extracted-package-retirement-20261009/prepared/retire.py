#!/usr/bin/env python3
"""Root-run exact six-package retirement; preparation does not execute it."""
from pathlib import Path, PurePosixPath
import argparse,datetime,hashlib,importlib.util,json,os,signal,stat,time

BASE=Path('/tmp/luci-retained-tools')
TOOLS=Path('/workspace/.luci-tools')
HERE=Path(__file__).parent
PLAN_SHA='c7e298a50984471954939b0d0f3f01e7f96a2e881cfb0252fcfaec5cf61287cd'
USAGE_SHA='60e7b78132c903bdea14ebf4b7a5311c1193f2abc10ca3920811a517df71654e'
NAMES=('0dcafb14','2da1a821','67a5b7c7','99007404','d20ebd29','edd0cb12')
KEEP=('b94ad37e','29931afc','5f4a966f')
ACCEPTED_UNKNOWN={(203,'1521'),(254,'1534'),(260700,'2331192')}
FIELDS=('mode','uid','gid','size','device','inode','nlink','mtimeNS','ctimeNS','kind')

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def digest(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def sm(s):return {'mode':stat.S_IMODE(s.st_mode),'uid':s.st_uid,'gid':s.st_gid,'size':s.st_size,'device':s.st_dev,'inode':s.st_ino,'nlink':s.st_nlink,'mtimeNS':s.st_mtime_ns,'ctimeNS':s.st_ctime_ns,'kind':'file' if stat.S_ISREG(s.st_mode) else 'directory' if stat.S_ISDIR(s.st_mode) else 'symlink' if stat.S_ISLNK(s.st_mode) else 'other'}
def same(p,e,fields=FIELDS):return all(sm(p.lstat())[k]==e[k] for k in fields)
def space():return {str(p):os.statvfs(p).f_bavail*os.statvfs(p).f_frsize for p in (Path('/workspace'),Path('/tmp'))}
def memory():
 result={}
 try:
  for line in Path('/proc/meminfo').read_text().splitlines():
   k,_,v=line.partition(':')
   if k in ('MemAvailable','Shmem','MemFree'):result[k+'Bytes']=int(v.split()[0])*1024
 except Exception as e:result['meminfoUnknown']=type(e).__name__
 for n in ('memory.current','memory.max','memory.swap.current'):
  try:result[n]=(Path('/sys/fs/cgroup')/n).read_text().strip()
  except Exception as e:result[n]={'unknown':type(e).__name__}
 result['scope']='Sequential visible host/root-cgroup observations only; not atomic effective admission operands.'
 return result

def main():
 p=argparse.ArgumentParser()
 p.add_argument('--operation',type=Path,required=True)
 p.add_argument('--closed-task-window',action='store_true',required=True)
 p.add_argument('--accept-known-unreadable-identities',choices=['203:1521,254:1534,260700:2331192'],required=True)
 a=p.parse_args();os.umask(0o077)
 def interrupted(signum,frame):raise InterruptedError('Owned retirement interrupted by signal '+str(signum))
 signal.signal(signal.SIGTERM,interrupted)
 signal.signal(signal.SIGINT,interrupted)
 assert os.getuid()==1000 and BASE.is_dir() and not BASE.is_symlink() and BASE.stat().st_uid==1000
 assert a.operation.parent==TOOLS and a.operation.name.startswith('old-extracted-package-retirement-operation-')
 a.operation.mkdir(mode=0o700,exist_ok=False)
 start=time.monotonic();seq=0;usage_seq=0;removed=[];finished=[]
 def clock():assert time.monotonic()-start<600,'Cooperative600s operation bound exceeded'
 def save(name,x):
  nonlocal seq
  seq+=1;target=a.operation/(f'{seq:03d}-'+name+'.json');b=(json.dumps(x,indent=2)+'\n').encode()
  fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
  with os.fdopen(fd,'wb') as f:f.write(b);f.flush();os.fsync(f.fileno())
  fd=os.open(a.operation,os.O_RDONLY|os.O_DIRECTORY)
  try:os.fsync(fd)
  finally:os.close(fd)
  return {'path':str(target),'sha256':hashlib.sha256(b).hexdigest()}
 def usage():
  nonlocal usage_seq
  clock();usage_seq+=1;src=HERE/'check_active_usage_v2.py';assert digest(src)==USAGE_SHA
  spec=importlib.util.spec_from_file_location('retirement_usage_'+str(usage_seq),src);mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod)
  out=a.operation/('usage-'+str(usage_seq));out.mkdir(mode=0o700);mod.ROOT=out;mod.main()
  r=json.loads((out/'active-usage-v2.json').read_bytes())
  assert not r['bounds']['reached'] and not r['busy'],'Target reference or incomplete usage scan'
  for u in r['unknown']:
   ident=(u['pid'],str(u.get('startTicks')))
   if ident in ACCEPTED_UNKNOWN:
    assert all(z['reason']=='permission-denied' and z['field'] in ('exe','cwd','maps','fd') for z in u['reasons'])
   elif ident==(r['inspector']['pid'],str(r['inspector']['startTicks'])):
    assert all(z['field']=='fd' and z['reason']=='absent-during-observation' for z in u['reasons'])
   else:raise AssertionError('New unknown process observation: '+str(ident))
  return {'path':str(out/'active-usage-v2.json'),'sha256':digest(out/'active-usage-v2.json'),'unknownCount':len(r['unknown']),'literalZombieCount':len(r['literalZombieIdentities'])}
 def verify_archive(row,m):
  archive=Path(row['archive']['path']);alias=Path(row['archive']['alias']);e=m['archive']
  assert same(archive,e['metadataBefore']) and same(alias,e['aliasMetadataBefore'])
  assert (os.readlink(alias) if alias.is_symlink() else None)==e['aliasLink']
  assert digest(archive)==row['archive']['sha256'] and same(archive,e['metadataBefore'])
 def verify_tree(row,m):
  root=Path(row['path']);entries=m['entries'];assert same(root,entries['.'])
  actual={'.'}
  for b,ds,fs in os.walk(root,followlinks=False):
   for n in ds+fs:
    clock();q=Path(b)/n;rel=q.relative_to(root).as_posix();assert rel in entries and same(q,entries[rel]),rel
    e=entries[rel];assert e['uid']==1000 and e['kind'] in ('file','directory')
    if e['kind']=='file':assert e['nlink']==1 and digest(q)==e['sha256'] and same(q,e),rel
    actual.add(rel)
  assert actual==set(entries),'New or missing tree members'
 def open_parent(rootfd,rel,entries):
  parts=PurePosixPath(rel).parts;fd=os.dup(rootfd);seen=[]
  try:
   for part in parts[:-1]:
    seen.append(part);nfd=os.open(part,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd);os.close(fd);fd=nfd
    e=entries['/'.join(seen)];s=os.fstat(fd)
    assert s.st_dev==e['device'] and s.st_ino==e['inode'] and s.st_uid==1000
   return fd,parts[-1]
  except BaseException:os.close(fd);raise

 r={'startedAt':now(),'status':'pending','helperSHA256':digest(Path(__file__)),'planSHA256':PLAN_SHA,'scope':'Only six exact generated extracted trees. No Docker, archive, fixture, ledger, model/module/tool-source or current-package mutation. Root-coordinated closed task window; exact known unreadable identities accepted without a universal idle claim.','freeBefore':space(),'memoryBefore':memory(),'packages':list(NAMES),'completed':finished}
 save('intent',r)
 try:
  plan=HERE/'plan.json';assert digest(plan)==PLAN_SHA;data=json.loads(plan.read_bytes());rows=data['packages'];assert tuple(x['name'] for x in rows)==NAMES
  kept={str(BASE/('package-0.5.0-'+n)):sm((BASE/('package-0.5.0-'+n)).lstat()) for n in KEEP}
  manifests={}
  for row in rows:
   assert row['path']==str(BASE/('package-0.5.0-'+row['name']))
   mp=Path(row['manifest']['path']);assert digest(mp)==row['manifest']['sha256'];m=json.loads(mp.read_bytes())
   assert m['root']==row['path'];manifests[row['name']]=m
   for rel,e in m['entries'].items():
    assert not rel.startswith('/') and '..' not in PurePosixPath(rel).parts and '\\' not in rel
    assert e['kind'] in ('file','directory') and e['uid']==1000 and (e['kind']!='file' or e['nlink']==1)
   verify_archive(row,m);verify_tree(row,m)
  scan=usage();save('all-targets-preflight-passed',{'at':now(),'usage':scan,'keptRootMetadata':kept,'archivesAndMembersMatched':True})
  for row in rows:
   clock();m=manifests[row['name']];entries=m['entries'];root=Path(row['path'])
   verify_archive(row,m);verify_tree(row,m);scan=usage()
   save('target-pending-'+row['name'],{'at':now(),'name':row['name'],'root':str(root),'manifest':row['manifest'],'archive':row['archive'],'usage':scan,'verifiedFiles':row['files']})
   rootfd=os.open(root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
   try:
    assert os.fstat(rootfd).st_ino==entries['.']['inode'] and os.fstat(rootfd).st_dev==entries['.']['device']
    for rel,e in sorted(entries.items()):
     if e['kind']!='file':continue
     clock();parent,name=open_parent(rootfd,rel,entries)
     try:
      fd=os.open(name,os.O_RDONLY|os.O_NOFOLLOW,dir_fd=parent)
      with os.fdopen(fd,'rb') as f:
       before=sm(os.fstat(f.fileno()));assert all(before[k]==e[k] for k in FIELDS)
       assert hashlib.file_digest(f,'sha256').hexdigest()==e['sha256']
       assert all(sm(os.fstat(f.fileno()))[k]==e[k] for k in FIELDS)
       assert all(sm(os.stat(name,dir_fd=parent,follow_symlinks=False))[k]==e[k] for k in FIELDS)
       os.unlink(name,dir_fd=parent)
      removed.append({'package':row['name'],'path':rel})
     finally:os.close(parent)
    for rel,e in sorted(entries.items(),key=lambda x:(len(PurePosixPath(x[0]).parts),x[0]),reverse=True):
     if e['kind']!='directory' or rel=='.':continue
     parent,name=open_parent(rootfd,rel,entries)
     try:
      s=os.stat(name,dir_fd=parent,follow_symlinks=False)
      assert stat.S_ISDIR(s.st_mode) and s.st_uid==1000 and s.st_dev==e['device'] and s.st_ino==e['inode']
      os.rmdir(name,dir_fd=parent)
     finally:os.close(parent)
    s=root.lstat();assert s.st_dev==entries['.']['device'] and s.st_ino==entries['.']['inode'] and stat.S_ISDIR(s.st_mode)
    root.rmdir()
   finally:os.close(rootfd)
   assert not root.exists() and not root.is_symlink()
   verify_archive(row,m)
   item={'name':row['name'],'at':now(),'removedFiles':sum(x['package']==row['name'] for x in removed),'originalArchiveUnchanged':True,'rootAbsent':True,'free':space()};finished.append(item);save('target-finished-'+row['name'],item)
  for path,e in kept.items():assert same(Path(path),e),'Protected current package root changed'
  for row in rows:verify_archive(row,manifests[row['name']])
  r.update(status='passed',finishedAt=now(),elapsedSeconds=time.monotonic()-start,removedFiles=len(removed),completed=finished,freeAfter=space(),memoryAfter=memory(),originalArchivesUnchanged=True,keptRootMetadataUnchanged=True,historyNotice='Old extracted package paths are intentionally absent until restored from their exact retained original archives. Original historical receipt paths and archives are unchanged. No admission or RAM-release success inferred.')
  save('final',r);print(json.dumps({'status':r['status'],'removedFiles':len(removed),'operation':str(a.operation),'freeBefore':r['freeBefore'],'freeAfter':r['freeAfter']}))
 except BaseException as e:
  r.update(status='failed-no-rollback-guarantee',finishedAt=now(),elapsedSeconds=time.monotonic()-start,error=type(e).__name__+': '+str(e)[:1500],removedFiles=len(removed),removedMembers=removed,completed=finished,freeAfter=space(),memoryAfter=memory(),uncertainty='Any partially retired exact target remains as observed. No claim of rollback or successful cleanup; original archives remain restoration sources.')
  save('failure',r);raise

if __name__=='__main__':main()
