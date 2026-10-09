#!/usr/bin/env python3
"""Read-only exact generated-payload proof. Never moves, removes or signals."""
from pathlib import Path
import datetime,hashlib,json,os,stat,tarfile,time
ROOT=Path('/workspace/.luci-tools');OUT=Path(__file__).parent
COMMIT='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
DIRS=[ROOT/n for n in ['cloud060-package-reuse-5f4a966f','cloud060-package-reuse-v2-5f4a966f','cloud060-package-reuse-v3-5f4a966f']]
PROV=Path('/workspace/luci/artifacts/quality-swarm/candidate-5f4a966f/package/package-provenance.json')
started=time.monotonic();deadline=started+90;os.umask(0o077)
def checktime():assert time.monotonic()<deadline,'90-second bounded proof expired'
def h(b):return hashlib.sha256(b).hexdigest()
def sm(s):return {'dev':s.st_dev,'ino':s.st_ino,'mode':stat.S_IMODE(s.st_mode),'uid':s.st_uid,'gid':s.st_gid,'bytes':s.st_size,'allocatedBytes':s.st_blocks*512,'nlink':s.st_nlink,'mtimeNs':s.st_mtime_ns}
def metadata(p):return sm(p.lstat())
def digest(p):
 checktime();a=p.lstat();assert stat.S_ISREG(a.st_mode) and not p.is_symlink();z=hashlib.sha256();n=0
 with p.open('rb')as f:
  while b:=f.read(1024*1024):checktime();z.update(b);n+=len(b)
 assert sm(a)==metadata(p),'file changed during read'
 return {'sha256':z.hexdigest(),'bytes':n,'metadata':sm(a)}
provraw=PROV.read_bytes();assert h(provraw)=='2e378e550fa41a68af2477f472c92f6c15cd1bbe4048e453c769db2e08dca8be';prov=json.loads(provraw)
assert prov['sourceCommit']==COMMIT and prov['sourceDirty']is False
ref=Path(prov['packageDir']);assert str(ref)=='/tmp/luci-retained-tools/package-0.5.0-5f4a966f'and not ref.is_symlink()
archive=Path(prov['archive']);originalArchive=str(archive);archive=archive.resolve(strict=True);am=digest(archive);assert am['sha256']==prov['archiveSHA256']=='bc7d8b76b76d3c291c651c3f46423f75f7e63f31f49ddde59372fe6de97e6290'
receipts=[];expected=None
for d in DIRS:
 assert d.is_dir()and not d.is_symlink();p=d/'build-receipt.json';raw=p.read_bytes();r=json.loads(raw)
 assert r['product']==COMMIT and r['passed']is True and r['sourceStable']is True and r['commands']==[]
 assert r['reuse']['selectedMemberCount']==425 and r['reuse']['archiveSHA256']==am['sha256']and r['reuse']['provenanceSHA256']==h(provraw)
 e={x['path']:{k:x[k]for k in ['sha256','bytes']}for x in r['outputs']};assert len(e)==425 and all(n=='luci'or n.startswith('web/')for n in e)
 assert all(not Path(n).is_absolute()and '..'not in Path(n).parts for n in e)
 if expected is None:expected=e
 else:assert expected==e
 receipts.append({'root':str(d),'receipt':{'path':str(p),'sha256':h(raw),'bytes':len(raw)},'reuse':r['reuse']})
reference={}
for n,want in expected.items():
 m=digest(ref/n);assert {k:m[k]for k in ['sha256','bytes']}==want;reference[n]=m
copies=[]
for rec,d in zip(receipts,DIRS):
 web=d/'web';assert web.is_dir()and not web.is_symlink()
 entries=list(web.rglob('*'));assert all(not f.is_symlink()and(stat.S_ISDIR(f.lstat().st_mode)or stat.S_ISREG(f.lstat().st_mode))for f in entries)
 present={'luci'}|{str(f.relative_to(d))for f in entries if f.is_file()};assert present==set(expected)
 outputs={}
 for n,want in expected.items():
  m=digest(d/n);assert {k:m[k]for k in ['sha256','bytes']}==want and m['metadata']['nlink']==1 and m['metadata']['uid']==os.getuid();outputs[n]=m
 dirs={str(f.relative_to(d)):metadata(f)for f in [web,*entries]if f.is_dir()}
 siblings={f.name:metadata(f)for f in d.iterdir()if f.name not in ('web','luci')}
 copies.append({**rec,'outputs':outputs,'directoryMetadata':dirs,'untouchedSiblings':siblings,'selectedLogicalBytes':sum(v['bytes']for v in outputs.values()),'selectedAllocatedBytes':sum(v['metadata']['allocatedBytes']for v in outputs.values())})
archiveMembers={};names=set()
with tarfile.open(archive,'r|gz')as t:
 for m in t:
  checktime()
  if m.name not in expected:continue
  assert m.name not in names and m.isfile()and m.size==expected[m.name]['bytes'];names.add(m.name);f=t.extractfile(m);z=hashlib.sha256()
  while b:=f.read(1024*1024):checktime();z.update(b)
  assert z.hexdigest()==expected[m.name]['sha256'];archiveMembers[m.name]={'sha256':z.hexdigest(),'bytes':m.size,'mode':m.mode,'uid':m.uid,'gid':m.gid}
assert names==set(expected)and am['metadata']==metadata(archive)
# Names and linkage only; no private environment or fixture contents are read.
observed=[];unknown=[];own=os.getpid();parent=os.getppid()
for p in Path('/proc').iterdir():
 checktime()
 if not p.name.isdigit()or int(p.name)in(own,parent):continue
 try:
  s=(p/'stat').read_text();tail=s[s.rfind(')')+2:].split();state=tail[0];ident={'pid':int(p.name),'startTicks':int(tail[19]),'state':state}
  if state=='Z':continue
 except (OSError,ValueError,IndexError):continue
 try:cmd=(p/'cmdline').read_bytes();exe=os.readlink(p/'exe');cwd=os.readlink(p/'cwd')
 except PermissionError:unknown.append({**ident,'observation':'exe/cwd permission denied'});continue
 except FileNotFoundError:continue
 matched=[str(d)for d in DIRS if str(d).encode()in cmd or exe==str(d/'luci')or cwd==str(d)or cwd.startswith(str(d)+'/')]
 if matched:observed.append({**ident,'matchedRoots':matched})
report={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'readOnly':True,'elapsedSeconds':time.monotonic()-started,'sourceCommit':COMMIT,'provenance':{'path':str(PROV),'sha256':h(provraw),'bytes':len(provraw)},'retainedReference':{'root':str(ref),'outputs':reference},'retainedArchive':{'originalPath':originalArchive,'resolvedPath':str(archive),**am,'selectedMembers':archiveMembers},'copies':copies,'selectedFilesPerCopy':425,'candidateAllocatedBytes':sum(c['selectedAllocatedBytes']for c in copies),'candidateLogicalBytes':sum(c['selectedLogicalBytes']for c in copies),'processSnapshot':{'matchedReadableNonZombie':observed,'unknown':unknown,'scope':'One-time cmdline/exe/cwd check only, no environment/fd/mapping census and no future-launch exclusion. Root coordination/known-run closure still required before mutation.'},'excludedRoots':['/workspace/.luci-tools/cloud060-package-reuse-v4-5f4a966f','all fixture state/evidence','all non-payload siblings','reference package and archive'],'freeBytes':{p:os.statvfs(p).f_bavail*os.statvfs(p).f_frsize for p in ['/workspace','/tmp']},'passed':True}
q=OUT/'proof.json';assert not q.exists();q.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'path':str(q),'sha256':h(q.read_bytes()),'copies':len(copies),'filesPerCopy':425,'candidateAllocatedBytes':report['candidateAllocatedBytes'],'candidateLogicalBytes':report['candidateLogicalBytes'],'archiveSHA256':am['sha256'],'readableMatches':observed,'unknown':unknown,'elapsedSeconds':report['elapsedSeconds'],'freeBytes':report['freeBytes']},indent=2))
