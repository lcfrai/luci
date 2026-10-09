#!/usr/bin/env python3
"""Root-only byte-preserving placement of one exact old runtime hardlink pair."""
from pathlib import Path
import argparse,datetime,hashlib,json,os,stat,time
ROOT=Path('/workspace/.luci-tools')
SOURCES=(ROOT/'runtime-archive-7e1b1f608cfa/verified-luci-sandbox-dev.tar.gz',ROOT/'runtime-archive-1f845cfd2b45/old-luci-sandbox-dev.tar.gz')
TARGET=Path('/tmp/luci-retained-tools/historical-runtime-42597a-c593f37e.tar.gz')
EXPECTED='c593f37eaa3795358aa1b76dd355828ddaadfcf503b9a5e3a1aea6976c947c91';SIZE=1013359941
FLOOR=500*1024*1024;GUARD=64*1024*1024;DUE=0
PROTECTED=(ROOT/'luci-sandbox-dev.tar.gz',ROOT/'runtime-archive-1f845cfd2b45/verified-luci-sandbox-dev.tar.gz')
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def bound():
 if DUE and time.monotonic()>DUE:raise TimeoutError('180-second cooperative operation budget expired')
def met(p):
 s=p.lstat();return {'dev':s.st_dev,'ino':s.st_ino,'mode':stat.S_IMODE(s.st_mode),'uid':s.st_uid,'gid':s.st_gid,'bytes':s.st_size,'blocksBytes':s.st_blocks*512,'nlink':s.st_nlink,'mtimeNs':s.st_mtime_ns}
def fsyncdir(p):
 fd=os.open(p,os.O_RDONLY|os.O_DIRECTORY)
 try:os.fsync(fd)
 finally:os.close(fd)
def write(p,v):
 fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'wb')as f:f.write((json.dumps(v,indent=2)+'\n').encode());f.flush();os.fsync(f.fileno())
 fsyncdir(p.parent)
def free(p):return os.statvfs(p).f_bavail*os.statvfs(p).f_frsize
def sha(p):
 before=met(p);assert not p.is_symlink()and stat.S_ISREG(p.lstat().st_mode);h=hashlib.sha256()
 with p.open('rb')as f:
  while b:=f.read(1048576):bound();h.update(b)
 assert met(p)==before;return h.hexdigest()
def chain(p):
 for x in [p,*p.parents]:assert not x.is_symlink(),'Linked path or parent refused'
def guard_space():assert free(TARGET.parent)>=FLOOR+GUARD,'TMP500MiB minimum plus64MiB working guard refused'
def validate_pair(paths):
 rows=[met(p)for p in paths]
 assert all(not p.is_symlink()and stat.S_ISREG(p.lstat().st_mode)for p in paths)
 assert rows[0]==rows[1]and rows[0]['nlink']==2 and rows[0]['uid']==os.getuid()and rows[0]['mode']==0o600 and rows[0]['bytes']==SIZE,'Exact owned hardlink pair mismatch'
 return rows[0]
def publish_link(source,target,temporary):
 assert not temporary.exists()and not temporary.is_symlink();os.symlink(str(target),temporary);fsyncdir(source.parent);os.replace(temporary,source);fsyncdir(source.parent)
def main():
 global DUE
 ap=argparse.ArgumentParser();ap.add_argument('--window-granted',action='store_true');ap.add_argument('--output',type=Path,required=True);a=ap.parse_args();assert a.window_granted
 assert a.output.parent==ROOT and a.output.name.startswith('historical425-relocation-operation-')and not a.output.exists();chain(ROOT);chain(TARGET.parent)
 os.umask(0o077);a.output.mkdir(mode=0o700);DUE=time.monotonic()+180;ordinal=0
 def record(n,v):
  nonlocal ordinal
  ordinal+=1;write(a.output/f'{ordinal:03d}-{n}.json',{'at':now(),**v})
 protected={str(p):met(p)for p in PROTECTED};receiptpaths=[ROOT/'runtime-archive-7e1b1f608cfa/receipt.json',ROOT/'runtime-archive-1f845cfd2b45/receipt.json'];receipts={str(p):sha(p)for p in receiptpaths};copied=False;replacements=[]
 try:
  assert not TARGET.exists()and not TARGET.is_symlink()and TARGET.parent.stat().st_uid==os.getuid()
  for p in SOURCES:chain(p)
  before=validate_pair(SOURCES);first=json.loads(receiptpaths[0].read_bytes());second=json.loads(receiptpaths[1].read_bytes())
  assert first['archiveVerification']['archiveSHA256']==second['preservedBefore']['archiveSHA256']==EXPECTED
  assert first['archiveVerification']['archiveBytes']==second['preservedBefore']['archiveBytes']==SIZE
  assert first['archiveVerification']['candidateImage']=='sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b'
  beforefree={p:free(p)for p in ['/workspace','/tmp']};assert beforefree['/tmp']>=SIZE+FLOOR+GUARD,'Insufficient TMP capacity for copy plus unchanged floor/guard'
  record('intent',{'sourceSHA256':sha(Path(__file__)),'sources':[str(p)for p in SOURCES],'target':str(TARGET),'originalMetadata':before,'expectedSHA256':EXPECTED,'expectedBytes':SIZE,'freeBefore':beforefree,'tmpMinimumBytes':FLOOR,'tmpWorkingGuardBytes':GUARD,'protected333Metadata':protected,'receiptHashes':receipts,'coordination':'Root confirms current build has settled and no task will use/change these exact archive paths. No global process-idle claim.','retention':'TMP is not guaranteed snapshot retention. Original inode/hardlink topology will change; exact archive bytes and both compatibility paths remain retained.'})
  assert sha(SOURCES[0])==EXPECTED;assert validate_pair(SOURCES)==before
  partial=TARGET.with_name(TARGET.name+'.copying-'+a.output.name);assert not partial.exists()and not partial.is_symlink()
  fd=os.open(partial,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600);h=hashlib.sha256();total=0
  with SOURCES[0].open('rb')as src,os.fdopen(fd,'wb')as out:
   while b:=src.read(1048576):bound();guard_space();out.write(b);h.update(b);total+=len(b)
   out.flush();os.fsync(out.fileno())
  assert total==SIZE and h.hexdigest()==EXPECTED and sha(partial)==EXPECTED;assert validate_pair(SOURCES)==before
  os.chmod(partial,before['mode']);os.utime(partial,ns=(partial.stat().st_atime_ns,before['mtimeNs']));assert partial.stat().st_uid==before['uid']and partial.stat().st_gid==before['gid'];guard_space();assert not TARGET.exists();partial.rename(TARGET);fsyncdir(TARGET.parent);copied=True
  record('copy-verified',{'target':str(TARGET),'sha256':EXPECTED,'bytes':SIZE,'metadata':met(TARGET),'originalPairUnchanged':validate_pair(SOURCES)==before,'free':{p:free(p)for p in ['/workspace','/tmp']}})
  for i,p in enumerate(SOURCES):
   bound();guard_space();current=met(p);assert current['dev']==before['dev']and current['ino']==before['ino']and current['bytes']==SIZE and current['nlink']==2-i
   temporary=p.with_name(p.name+'.linking-'+a.output.name);record('link-intent',{'source':str(p),'temporary':str(temporary),'target':str(TARGET),'expectedCurrentNlink':2-i});publish_link(p,TARGET,temporary);replacements.append(str(p));assert p.is_symlink()and os.readlink(p)==str(TARGET);record('linked',{'source':str(p),'target':str(TARGET)})
  assert sha(TARGET)==EXPECTED and TARGET.stat().st_size==SIZE;guard_space();assert protected=={str(p):met(p)for p in PROTECTED}and receipts=={str(p):sha(p)for p in receiptpaths}
  afterfree={p:free(p)for p in ['/workspace','/tmp']};record('final',{'passed':True,'links':replacements,'target':str(TARGET),'targetMetadata':met(TARGET),'sha256':EXPECTED,'bytes':SIZE,'freeBefore':beforefree,'freeAfter':afterfree,'measuredRootFreeDelta':afterfree['/workspace']-beforefree['/workspace'],'protected333MetadataUnchanged':True,'originalReceiptsUnchanged':True,'limits':'No Docker/image mutation or loaded-image verification. Protected333 metadata parity is not a new333 byte-hash pass. No fixture/state/ledger bytes touched. TMP retention is not guaranteed; original hardlink topology/inode no longer exists.'})
 except BaseException as e:
  record('failure',{'passed':False,'errorType':type(e).__name__,'message':str(e)[:500],'verifiedTargetEstablished':copied,'recordedReplacements':replacements,'free':{p:free(p)for p in ['/workspace','/tmp']},'scope':'No automatic rollback/deletion. Inspect exact source symlink state after any interrupted fsync/replace. Retained original bytes remain in the original hardlink(s) or verified target; partial copy and journal retained.'});raise
if __name__=='__main__':main()
