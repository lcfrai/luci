#!/usr/bin/env python3
"""Read-only content/metadata verification of six exact generated package trees."""
from pathlib import Path, PurePosixPath
import datetime,hashlib,json,os,stat,tarfile,time

B=Path(__file__).parent
ROOT=Path('/tmp/luci-retained-tools')
REPO=Path('/workspace/luci')
NAMES=('0dcafb14','2da1a821','67a5b7c7','99007404','d20ebd29','edd0cb12')
PRESERVED={
 '0dcafb14':'0.5.0-0dcafb14-preserved-before-99007404',
 '2da1a821':'0.5.0-2da1a821-preserved-before-0dcafb14',
 '67a5b7c7':'0.5.0-67a5b7c7-preserved-before-d20ebd29',
 '99007404':'0.5.0-99007404-preserved-before-67a5b7c7',
 'd20ebd29':'0.5.0-d20ebd29-preserved-before-edd0cb12',
 'edd0cb12':'0.5.0-edd0cb12-preserved-before-5f4a966f',
}
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def hashfile(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def metadata(p):
 s=p.lstat()
 return {'mode':stat.S_IMODE(s.st_mode),'uid':s.st_uid,'gid':s.st_gid,'size':s.st_size,'allocatedBytes':s.st_blocks*512,'device':s.st_dev,'inode':s.st_ino,'nlink':s.st_nlink,'mtimeNS':s.st_mtime_ns,'atimeNS':s.st_atime_ns,'ctimeNS':s.st_ctime_ns,'kind':'file' if stat.S_ISREG(s.st_mode) else 'directory' if stat.S_ISDIR(s.st_mode) else 'symlink' if stat.S_ISLNK(s.st_mode) else 'other'}
def unchanged(old,new):
 return all(old[k]==new[k] for k in ('mode','uid','gid','size','device','inode','nlink','mtimeNS','ctimeNS','kind'))
def save(n,x):
 p=B/n;p.parent.mkdir(parents=True,exist_ok=True);assert not p.exists();p.write_text(json.dumps(x,indent=2)+'\n');return {'path':str(p),'sha256':hashfile(p),'bytes':p.stat().st_size}

assert os.getuid()==1000
os.umask(0o077)
started=now();results=[]
for name in NAMES:
 t=time.monotonic();root=ROOT/('package-0.5.0-'+name)
 r={'name':name,'path':str(root),'startedAt':now(),'status':'verifying','errors':[]}
 try:
  assert root.is_dir() and not root.is_symlink()
  pp=REPO/'artifacts/quality-swarm'/('candidate-'+name)/'package/package-provenance.json'
  ip=pp.with_name('package-integrity.json')
  prov=json.loads(pp.read_bytes());integ=json.loads(ip.read_bytes())
  assert prov['packageDir']==str(root) and prov['sourceCommit'].startswith(name) and not prov['sourceDirty'] and integ['allMembersMatched']
  alias=REPO/'artifacts/releases'/PRESERVED[name]/'luci-linux-amd64.tar.gz'
  archive=alias.resolve(strict=True)
  am=metadata(archive);alm=metadata(alias)
  assert am['kind']=='file' and am['uid']==1000
  expected=next(x for x in prov['subjects'] if x['name'].endswith('/luci-linux-amd64.tar.gz'))
  assert am['size']==expected['bytes']
  archive_hash=hashfile(archive)
  assert archive_hash==prov['archiveSHA256']==expected['sha256']
  assert unchanged(am,metadata(archive))
  r.update(sourceCommit=prov['sourceCommit'],provenance={'path':str(pp),'sha256':hashfile(pp)},integrity={'path':str(ip),'sha256':hashfile(ip)},archive={'alias':str(alias),'aliasMetadataBefore':alm,'aliasLink':os.readlink(alias) if alias.is_symlink() else None,'path':str(archive),'sha256':archive_hash,'bytes':am['size'],'metadataBefore':am})
  entries={'.':metadata(root)}
  for b,dirs,files in os.walk(root,followlinks=False):
   for n in sorted(dirs+files):
    p=Path(b)/n;rel=p.relative_to(root).as_posix();m=metadata(p)
    assert m['kind'] in ('directory','file') and m['uid']==1000 and (m['kind']!='file' or m['nlink']==1),rel
    entries[rel]=m
  archived={};modes=[];mtimes=[];ownerDifferences=[]
  with tarfile.open(archive,mode='r|gz') as tf:
   for m in tf:
    raw=m.name;parts=PurePosixPath(raw).parts
    assert raw and not raw.startswith('/') and '..' not in parts and '\\' not in raw,raw
    rel=PurePosixPath(raw).as_posix()
    assert rel not in archived,rel
    assert m.isfile() or m.isdir(),rel
    archived[rel]={'kind':'file' if m.isfile() else 'directory','mode':m.mode,'uid':m.uid,'gid':m.gid,'mtime':m.mtime,'size':m.size}
    assert rel in entries and entries[rel]['kind']==archived[rel]['kind'],rel
    actual=entries[rel]
    if actual['mode'] != m.mode:modes.append({'path':rel,'archive':m.mode,'extracted':actual['mode']})
    if actual['mtimeNS'] != int(m.mtime*1_000_000_000):mtimes.append({'path':rel,'archive':m.mtime,'extractedNS':actual['mtimeNS']})
    if actual['uid']!=m.uid or actual['gid']!=m.gid:ownerDifferences.append(rel)
    if m.isfile():
     assert actual['size']==m.size,rel
     f=tf.extractfile(m);assert f is not None
     h=hashlib.file_digest(f,'sha256').hexdigest()
     assert hashfile(root/rel)==h,rel
     actual['sha256']=h;archived[rel]['sha256']=h
  files={n for n,m in entries.items() if m['kind']=='file'}
  archiveFiles={n for n,m in archived.items() if m['kind']=='file'}
  assert files==archiveFiles and len(files)==integ['verifiedExtractedFiles']
  assert not modes,modes[:3]
  for rel,m in entries.items():assert unchanged(m,metadata(root if rel=='.' else root/rel)),rel
  assert unchanged(am,metadata(archive)) and unchanged(alm,metadata(alias))
  manifest={'sourceCommit':prov['sourceCommit'],'root':str(root),'archive':r['archive'],'entries':entries,'archiveMembers':archived,'modeDifferences':modes,'mtimeDifferences':mtimes,'archiveOwnershipDifferences':ownerDifferences,'metadataNote':'Original extracted metadata is retained before reads. Archive extraction filters ownership to the current user; restoration must apply the recorded extracted metadata. Reads may change access time. Inode, ctime and deletion topology cannot be restored. No removal or restoration has occurred.'}
  bound=save('manifests/'+name+'.json',manifest)
  r.update(status='passed',manifest=bound,files=len(files),directories=sum(x['kind']=='directory' for x in entries.values()),logicalBytes=sum(entries[n]['size'] for n in files),allocatedFileBytes=sum(entries[n]['allocatedBytes'] for n in files),allocatedDirectoryBytes=sum(x['allocatedBytes'] for x in entries.values() if x['kind']=='directory'),allContentsMatch=True,allOriginalMetadataStable=True,modeDifferenceCount=len(modes),mtimeDifferenceCount=len(mtimes),ownershipDifferenceCount=len(ownerDifferences),implicitDirectories=sorted(n for n,m in entries.items() if m['kind']=='directory' and n not in archived))
 except Exception as e:
  r.update(status='failed',errors=[type(e).__name__+': '+str(e)[:2000]])
 r.update(finishedAt=now(),elapsedSeconds=time.monotonic()-t);results.append(r);save('results/'+name+'.json',r);print(json.dumps({k:r.get(k) for k in ('name','status','files','allocatedFileBytes','elapsedSeconds','errors')}),flush=True)
 if r['status']!='passed':break
summary={'startedAt':started,'finishedAt':now(),'passed':len(results)==len(NAMES) and all(x['status']=='passed' for x in results),'packages':results,'allocatedFileBytes':sum(x.get('allocatedFileBytes',0) for x in results),'logicalBytes':sum(x.get('logicalBytes',0) for x in results),'kept':['b94ad37e','29931afc','5f4a966f'],'operation':'read-only verification; no removal, archive mutation, runtime, Docker or private-state read'}
print(json.dumps(save('verification.json',summary)),flush=True)
