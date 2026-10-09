#!/usr/bin/env python3
"""Restore one absent, exact old extracted package from its retained archive.

Preparation only: no production restoration or deletion is performed by preparing
this file. It refuses existing destinations and retains a partial tree on failure.
"""
from pathlib import Path, PurePosixPath
import argparse,datetime,hashlib,json,os,stat,tarfile,time

ALLOWED=('0dcafb14','2da1a821','67a5b7c7','99007404','d20ebd29','edd0cb12')
BASE=Path('/tmp/luci-retained-tools')

def digest(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def main():
 p=argparse.ArgumentParser()
 p.add_argument('--plan',type=Path,required=True)
 p.add_argument('--plan-sha256',required=True)
 p.add_argument('--package',choices=ALLOWED,required=True)
 p.add_argument('--receipt',type=Path,required=True)
 a=p.parse_args()
 assert digest(a.plan)==a.plan_sha256,'Plan changed'
 plan=json.loads(a.plan.read_bytes())
 row=next(x for x in plan['packages'] if x['name']==a.package)
 manifest=Path(row['manifest']['path'])
 assert digest(manifest)==row['manifest']['sha256'],'Manifest changed'
 m=json.loads(manifest.read_bytes());entries=m['entries']
 root=BASE/('package-0.5.0-'+a.package)
 assert m['root']==str(root) and row['path']==str(root)
 assert os.getuid()==1000 and BASE.is_dir() and not BASE.is_symlink() and BASE.stat().st_uid==1000
 assert not root.exists() and not root.is_symlink(),'Refusing existing destination'
 assert not a.receipt.exists() and not a.receipt.is_symlink(),'Refusing receipt overwrite'
 a.receipt.parent.mkdir(parents=True,exist_ok=True)
 archive=Path(m['archive']['path'])
 st=archive.lstat()
 assert stat.S_ISREG(st.st_mode) and st.st_uid==1000 and st.st_size==m['archive']['bytes']
 assert digest(archive)==m['archive']['sha256'],'Archive changed'
 assert set(x['kind'] for x in entries.values())<={'file','directory'}
 for rel,e in entries.items():
  parts=PurePosixPath(rel).parts
  assert not rel.startswith('/') and '..' not in parts and '\\' not in rel
  assert e['uid']==os.getuid() and e['gid']==os.getgid()
  assert e['kind']!='file' or e['nlink']==1
 logical=sum(x['size'] for x in entries.values() if x['kind']=='file')
 free=os.statvfs(BASE).f_bavail*os.statvfs(BASE).f_frsize
 assert free>=logical+64*1024*1024,'Insufficient restoration space'
 started=time.monotonic()
 r={'startedAt':now(),'package':a.package,'planSHA256':a.plan_sha256,'manifestSHA256':row['manifest']['sha256'],'archiveSHA256':m['archive']['sha256'],'archive':str(archive),'destination':str(root),'status':'pending','files':0,'bytes':0,'scope':'Restore exact generated payload bytes and recorded modes/mtime/ownership only. New inode/ctime/topology; access times observer-sensitive. No original archive, fixture, ledger or Docker changes.'}
 def save():
  raw=(json.dumps(r,indent=2)+'\n').encode()
  flags=os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW
  fd=os.open(a.receipt,flags,0o600)
  with os.fdopen(fd,'wb') as f:f.write(raw);f.flush();os.fsync(f.fileno())
 def clock():assert time.monotonic()-started<300,'Cooperative restoration bound exceeded'
 save()
 try:
  root.mkdir(mode=0o700)  # exclusive mkdir refuses a late competing destination
  for rel,e in sorted(entries.items(),key=lambda x:(len(PurePosixPath(x[0]).parts),x[0])):
   if e['kind']=='directory' and rel!='.':(root/rel).mkdir(mode=0o700)
  written=set()
  with tarfile.open(archive,mode='r|gz') as tf:
   for member in tf:
    clock();rel=PurePosixPath(member.name).as_posix()
    assert rel in entries and member.isfile() or rel in entries and member.isdir()
    if member.isdir():continue
    e=entries[rel]
    assert e['kind']=='file' and member.size==e['size'] and rel not in written
    target=root/rel
    fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    h=hashlib.sha256();count=0
    with os.fdopen(fd,'wb') as out:
     inp=tf.extractfile(member);assert inp is not None
     while True:
      clock();chunk=inp.read(1024*1024)
      if not chunk:break
      count+=len(chunk);assert count<=e['size'];out.write(chunk);h.update(chunk)
     out.flush();os.fsync(out.fileno())
    assert count==e['size'] and h.hexdigest()==e['sha256'] and digest(target)==e['sha256']
    os.chmod(target,e['mode'],follow_symlinks=False)
    os.utime(target,ns=(e['atimeNS'],e['mtimeNS']),follow_symlinks=False)
    written.add(rel);r['files']+=1;r['bytes']+=count
  assert written=={n for n,e in entries.items() if e['kind']=='file'}
  for rel,e in sorted(entries.items(),key=lambda x:len(PurePosixPath(x[0]).parts),reverse=True):
   if e['kind']=='directory':
    target=root if rel=='.' else root/rel
    os.chmod(target,e['mode'],follow_symlinks=False)
    os.utime(target,ns=(e['atimeNS'],e['mtimeNS']),follow_symlinks=False)
  for rel,e in entries.items():
   target=root if rel=='.' else root/rel;s=target.lstat()
   assert stat.S_IMODE(s.st_mode)==e['mode'] and s.st_uid==e['uid'] and s.st_gid==e['gid'] and s.st_mtime_ns==e['mtimeNS']
   assert e['kind']!='file' or stat.S_ISREG(s.st_mode) and s.st_nlink==1 and s.st_size==e['size']
  assert digest(archive)==m['archive']['sha256'],'Archive changed during restoration'
  r.update(status='passed',finishedAt=now(),elapsedSeconds=time.monotonic()-started)
 except BaseException as e:
  r.update(status='failed-partial-destination-retained',finishedAt=now(),error=type(e).__name__+': '+str(e)[:1000])
  save();raise
 save()
 print(json.dumps({'status':r['status'],'files':r['files'],'bytes':r['bytes'],'receipt':str(a.receipt)}))

if __name__=='__main__':main()
