#!/usr/bin/env python3
"""Root-reviewed one-time retirement of three exact duplicate generated payloads."""
from pathlib import Path
import argparse,datetime,hashlib,json,os,stat,sys,tarfile,time
ROOT=Path('/workspace/.luci-tools')
PROOF=ROOT/'cloud060-copy-retirement-plan/proof.json'
PROOF_SHA='cbab4651da95badd8f04ea40153587ae92790d3b2ac831a8a197de2b0903bce4'
NAMES=('cloud060-package-reuse-5f4a966f','cloud060-package-reuse-v2-5f4a966f','cloud060-package-reuse-v3-5f4a966f')
UNKNOWN={(203,1521),(254,1534),(260700,2331192)}
REF=Path('/tmp/luci-retained-tools/package-0.5.0-5f4a966f')
LIMIT=180
DUE=0

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def bound():
 if DUE and time.monotonic()>DUE:raise TimeoutError('180-second operation budget expired')
def sha(b):return hashlib.sha256(b).hexdigest()
def meta(p):
 s=p.lstat();return {'dev':s.st_dev,'ino':s.st_ino,'mode':stat.S_IMODE(s.st_mode),'uid':s.st_uid,'gid':s.st_gid,'bytes':s.st_size,'allocatedBytes':s.st_blocks*512,'nlink':s.st_nlink,'mtimeNs':s.st_mtime_ns}
def hashfile(p):
 bound();s=p.lstat();assert stat.S_ISREG(s.st_mode)and not p.is_symlink(),'Nonordinary file';before=meta(p);h=hashlib.sha256();n=0
 with p.open('rb')as f:
  while b:=f.read(1048576):bound();h.update(b);n+=len(b)
 assert before==meta(p),'File changed during hashing';return {'sha256':h.hexdigest(),'bytes':n}
def fsyncdir(p):
 fd=os.open(p,os.O_RDONLY|os.O_DIRECTORY)
 try:os.fsync(fd)
 finally:os.close(fd)
def write_new(p,value):
 b=(json.dumps(value,indent=2)+'\n').encode();fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 try:
  with os.fdopen(fd,'wb')as f:f.write(b);f.flush();os.fsync(f.fileno())
 finally:pass
 fsyncdir(p.parent)
def free():return {str(p):os.statvfs(p).f_bavail*os.statvfs(p).f_frsize for p in ['/workspace','/tmp']}
def ordinary_chain(p):
 for a in [p,*p.parents]:
  assert not a.is_symlink(),'Linked ancestor refused: '+str(a)
def expected_names(rows):
 assert len(rows)==425 and 'luci'in rows
 assert all(not Path(n).is_absolute()and '..'not in Path(n).parts and(n=='luci'or n.startswith('web/'))for n in rows)
 return set(rows)
def tree_inventory(root):
 ordinary_chain(root);out={};dirs={};web=root/'web';assert web.is_dir()and not web.is_symlink()
 for p in [web,*web.rglob('*')]:
  bound();assert not p.is_symlink();m=p.lstat();name=str(p.relative_to(root))
  if stat.S_ISREG(m.st_mode):out[name]=meta(p)
  elif stat.S_ISDIR(m.st_mode):dirs[name]=meta(p)
  else:raise ValueError('Special file in generated web tree')
 p=root/'luci';assert stat.S_ISREG(p.lstat().st_mode)and not p.is_symlink();out['luci']=meta(p)
 return out,dirs

def observe_paths(p):
 cmd=b'';exe='';cwd='';unreadable=False
 try:cmd=(p/'cmdline').read_bytes()
 except PermissionError:unreadable=True
 except FileNotFoundError:pass
 try:exe=os.readlink(p/'exe')
 except PermissionError:unreadable=True
 except FileNotFoundError:pass
 try:cwd=os.readlink(p/'cwd')
 except PermissionError:unreadable=True
 except FileNotFoundError:pass
 matched=[n for n in NAMES if str(ROOT/n).encode()in cmd or exe==str(ROOT/n/'luci')or cwd==str(ROOT/n)or cwd.startswith(str(ROOT/n)+'/')]
 return matched,unreadable

def processes():
 hits=[];unknown=[];excluded={os.getpid(),os.getppid()}
 for p in Path('/proc').iterdir():
  bound()
  if not p.name.isdigit()or int(p.name)in excluded:continue
  try:
   s=(p/'stat').read_text();a=s[s.rfind(')')+2:].split();state=a[0];identity=(int(p.name),int(a[19]))
   if state=='Z':continue
  except (OSError,ValueError,IndexError):continue
  matched,unreadable=observe_paths(p)
  if matched:hits.append({'pid':identity[0],'startTicks':identity[1],'matchedRoots':matched})
  if unreadable:unknown.append(identity)
 assert not hits,'Readable non-zombie process references a candidate'
 assert set(unknown)<=UNKNOWN,'New unknown process identity; refusal before mutation'
 return {'matchedReadableNonZombie':hits,'acceptedUnknownIdentities':[{'pid':p,'startTicks':s}for p,s in unknown],'scope':'Single cmdline/exe/cwd observation plus root coordination; no environment/fd/maps census, no global or future-launch exclusion.'}

def restore_link(original,holding,target,expected):
 if holding.exists()or holding.is_symlink():
  assert not holding.is_symlink()and meta(holding)==expected,'Unexpected held original'
  if original.exists()or original.is_symlink():
   assert original.is_symlink()and os.readlink(original)==str(target),'Refuse foreign replacement'
   original.unlink()
  holding.rename(original);fsyncdir(original.parent)
 else:
  assert not original.is_symlink()and meta(original)==expected,'Original not restored'

def link_payload(original,holding,target):
 assert not holding.exists()and not holding.is_symlink()and target.exists()and not target.is_symlink()
 expected=meta(original)
 try:
  original.rename(holding);fsyncdir(original.parent)
  os.symlink(str(target),original);fsyncdir(original.parent)
 except BaseException as first:
  try:restore_link(original,holding,target,expected)
  except BaseException as second:raise RuntimeError('Link transition failed; rollback durability/state uncertain: '+type(second).__name__)from first
  raise

def remove_exact_payload(holding,kind,copy):
 # Only manifest-declared files; no recursive removal and no traversal of links.
 if kind=='luci':
  assert not holding.is_symlink()and meta(holding)==copy['outputs']['luci']['metadata'];holding.unlink();fsyncdir(holding.parent);return
 rows=copy['outputs'];dirs=copy['directoryMetadata']
 assert kind=='web'and holding.is_dir()and not holding.is_symlink()
 observed={str(p.relative_to(holding)):p for p in holding.rglob('*')if p.is_file()or p.is_symlink()}
 want={n.removeprefix('web/')for n in rows if n.startswith('web/')};assert set(observed)==want
 for n in sorted(want):
  bound();p=observed[n];assert not p.is_symlink()and meta(p)==rows['web/'+n]['metadata'];p.unlink()
 for n in sorted(dirs,key=lambda x:(len(Path(x).parts),x),reverse=True):
  bound();p=holding if n=='web'else holding/n.removeprefix('web/');assert p.is_dir()and not p.is_symlink();p.rmdir()
 fsyncdir(holding.parent)

def main():
 global DUE
 ap=argparse.ArgumentParser();ap.add_argument('--window-granted',action='store_true');ap.add_argument('--accept-unreadable',required=True);ap.add_argument('--output',type=Path,required=True);a=ap.parse_args()
 assert a.window_granted,'Root-controlled no-launch window required'
 assert set(tuple(map(int,x.split(':')))for x in a.accept_unreadable.split(','))==UNKNOWN
 ordinary_chain(ROOT);assert a.output.parent==ROOT and a.output.name.startswith('cloud060-copy-retirement-operation-')and not a.output.exists()
 os.umask(0o077);a.output.mkdir(mode=0o700);DUE=time.monotonic()+LIMIT;count=0;staged=[];unlink_started=False
 def record(kind,data):
  nonlocal count
  count+=1;write_new(a.output/f'{count:03d}-{kind}.json',{'at':now(),**data})
 try:
  raw=PROOF.read_bytes();assert sha(raw)==PROOF_SHA;p=json.loads(raw);assert p['readOnly']is True and p['passed']is True
  assert [Path(x['root']).name for x in p['copies']]==list(NAMES)and p['retainedReference']['root']==str(REF)
  before=free();record('intent',{'sourceSHA256':sha(Path(__file__).read_bytes()),'proofSHA256':PROOF_SHA,'budgetSeconds':LIMIT,'rootCoordinationAssertion':True,'freeBefore':before,'onlyPayloadRoots':[str(ROOT/n/k)for n in NAMES for k in ['luci','web']],'exclusions':'All receipt/cache/home/tmp/fixture/evidence paths, v4, reference package/archive and unrelated paths remain untouched. Generated duplicate physical inodes retire; exact bytes remain reference/archive-backed.'})
  ordinary_chain(REF);rows=p['retainedReference']['outputs'];expected=expected_names(rows)
  for n,m in rows.items():assert hashfile(REF/n)=={k:m[k]for k in ['sha256','bytes']}
  archive=Path(p['retainedArchive']['resolvedPath']);ordinary_chain(archive);assert hashfile(archive)=={k:p['retainedArchive'][k]for k in ['sha256','bytes']}
  seen=set()
  with tarfile.open(archive,'r|gz')as t:
   for member in t:
    bound()
    if member.name not in expected:continue
    assert member.name not in seen and member.isfile()and member.size==rows[member.name]['bytes'];seen.add(member.name);h=hashlib.sha256();f=t.extractfile(member)
    while b:=f.read(1048576):bound();h.update(b)
    assert h.hexdigest()==rows[member.name]['sha256']
  assert seen==expected
  for copy in p['copies']:
   d=Path(copy['root']);files,dirs=tree_inventory(d);assert set(files)==expected and dirs==copy['directoryMetadata']
   assert hashfile(d/'build-receipt.json')=={k:copy['receipt'][k]for k in ['sha256','bytes']}
   assert {x.name:meta(x)for x in d.iterdir()if x.name not in('luci','web')}==copy['untouchedSiblings']
   for n,m in copy['outputs'].items():assert files[n]==m['metadata']and hashfile(d/n)=={k:m[k]for k in ['sha256','bytes']}
  scan=processes();record('preflight-verified',{'copies':3,'selectedMembersPerCopy':425,'archiveSHA256':p['retainedArchive']['sha256'],'referenceSHA256':p['provenance']['sha256'],'processObservation':scan,'candidateAllocatedBytes':p['candidateAllocatedBytes'],'free':free()})
  suffix=a.output.name.removeprefix('cloud060-copy-retirement-operation-')
  for copy in p['copies']:
   d=Path(copy['root'])
   for kind in ['luci','web']:
    bound();original=d/kind;holding=d/('.generated-payload-retiring-'+suffix+'-'+kind);target=REF/kind
    record('link-intent',{'original':str(original),'holding':str(holding),'target':str(target),'expectedFiles':1 if kind=='luci'else 424})
    staged.append((copy,kind,original,holding,target));link_payload(original,holding,target);record('linked',{'original':str(original),'holding':str(holding),'target':str(target)})
  for copy,kind,original,holding,target in staged:
   assert original.is_symlink()and os.readlink(original)==str(target)
   for n,m in copy['outputs'].items():
    if(kind=='luci'and n=='luci')or(kind=='web'and n.startswith('web/')):assert hashfile((Path(copy['root'])/n).resolve(strict=True))=={k:m[k]for k in ['sha256','bytes']}
  processes();record('all-links-verified-before-unlink',{'count':6,'physicalOriginalsStillPresent':True,'referenceAndArchiveAlreadyByteVerified':True,'warning':'After unlink, original generated copy inode/metadata objects cease to exist; bytes reconstructible from immutable retained package/archive and original metadata retained in proof.'})
  unlink_started=True
  for copy,kind,original,holding,target in staged:
   bound();record('unlink-intent',{'holding':str(holding),'kind':kind,'target':str(target),'manifestSHA256':PROOF_SHA});remove_exact_payload(holding,kind,copy);record('unlinked',{'holding':str(holding),'absence':not holding.exists()})
  for copy in p['copies']:
   d=Path(copy['root']);assert {x.name:meta(x)for x in d.iterdir()if x.name not in('luci','web')}==copy['untouchedSiblings']
   for n,m in copy['outputs'].items():assert hashfile((d/n).resolve(strict=True))=={k:m[k]for k in ['sha256','bytes']}
  assert hashfile(archive)=={k:p['retainedArchive'][k]for k in ['sha256','bytes']}
  after=free();record('final',{'passed':True,'links':[{'path':str(o),'target':str(t)}for _,_,o,_,t in staged],'freeBefore':before,'freeAfter':after,'measuredRootFreeDelta':after['/workspace']-before['/workspace'],'candidateAllocatedBytes':p['candidateAllocatedBytes'],'allOriginalReceiptsAndNonpayloadSiblingsUnchanged':True,'referenceAndArchiveBytesUnchanged':True,'privateFixtureEvidenceScope':'Not read, touched or republished; operation concerns generated binary/web copies only.','metadataScope':'Original generated file inode identity no longer exists; recorded metadata and retained byte source support reconstruction, not inode restoration.'});return 0
 except BaseException as e:
  rollback=[]
  if not unlink_started:
   for copy,kind,original,holding,target in reversed(staged):
    try:
     expected=copy['outputs']['luci']['metadata']if kind=='luci'else copy['directoryMetadata']['web']
     restore_link(original,holding,target,expected);rollback.append({'path':str(original),'restored':True})
    except BaseException as r:rollback.append({'path':str(original),'restored':False,'errorType':type(r).__name__})
  record('failure',{'passed':False,'errorType':type(e).__name__,'message':str(e)[:500],'unlinkStarted':unlink_started,'rollback':rollback,'free':free(),'uncertainty':'If unlink began, do not infer complete retirement; use durable exact stages and retained reference/archive to restore bytes. No hidden repair attempted.'});raise
if __name__=='__main__':sys.exit(main())
