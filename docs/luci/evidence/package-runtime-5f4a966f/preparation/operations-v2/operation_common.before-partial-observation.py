"""External operations support. Importing performs no host observations or mutation."""
from pathlib import Path
import datetime,hashlib,json,os,stat,time
ROOT=Path('/workspace/.luci-tools/cloud054-runtime-operations-v2')
class Refusal(RuntimeError):pass
def need(value,code):
 if not value:raise Refusal(code)
def sha(data):return hashlib.sha256(data).hexdigest()
def identity(s):return (s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_mode,s.st_uid,s.st_gid)
def read_bounded(path,limit=2*1024*1024):
 fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
 try:
  s=os.fstat(fd);need(stat.S_ISREG(s.st_mode) and s.st_size<=limit,'input-not-bounded-regular-file')
  data=os.read(fd,limit+1);need(len(data)==s.st_size and len(data)<=limit,'input-size-changed');need(identity(s)==identity(os.fstat(fd)),'input-changed');return data
 finally:os.close(fd)
def pinned_json(path,pin):
 data=read_bounded(path);need(sha(data)==pin,'input-pin-mismatch');return json.loads(data)
def hash_file(path):
 fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
 try:
  s=os.fstat(fd);need(stat.S_ISREG(s.st_mode),'non-regular-file');h=hashlib.sha256()
  while True:
   b=os.read(fd,1024*1024)
   if not b:break
   h.update(b)
  need(identity(s)==identity(os.fstat(fd))==identity(os.lstat(path)),'file-changed-during-hash');return h.hexdigest(),s
 finally:os.close(fd)
def private_dir(path):
 p=Path(path);s=p.lstat();need(p.resolve()==p and stat.S_ISDIR(s.st_mode) and s.st_uid==os.getuid() and not(s.st_mode&0o077),'private-directory-required');return s
class Journal:
 def __init__(self,path,operation,sources):
  private_dir(path.parent);path.mkdir(mode=0o700);private_dir(path);self.path=path;self.n=0;self.value={'version':1,'operation':operation,'passed':False,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceBefore':{str(p):sha(read_bounded(p)) for p in sources},'steps':[]};self.sources=sources;self.save('created')
 def save(self,stage,**values):
  if stage=='failed':
   try:values['sourceAfter']={str(p):sha(read_bounded(p)) for p in self.sources};values['sourceUnchanged']=values['sourceAfter']==self.value['sourceBefore']
   except Exception:values['sourceAfterUnavailable']=True
  self.n+=1;self.value.update(values);self.value['stage']=stage;self.value['steps'].append({'number':self.n,'stage':stage,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()});data=(json.dumps(self.value,indent=2)+'\n').encode();need(len(data)<8*1024*1024,'receipt-size-bound');p=self.path/(str(self.n).zfill(3)+'-'+stage+'.json');fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
  try:
   offset=0
   while offset<len(data):offset+=os.write(fd,data[offset:])
   os.fsync(fd)
  finally:os.close(fd)
  d=os.open(self.path,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
  try:os.fsync(d)
  finally:os.close(d)
 def finish(self,passed,**values):
  after={str(p):sha(read_bounded(p)) for p in self.sources};need(after==self.value['sourceBefore'],'operation-source-changed');self.save('finished',passed=passed,sourceAfter=after,**values)
def process_start(pid):
 text=Path('/proc',str(pid),'stat').read_text();rest=text.rsplit(')',1)[1].split();need(rest[0] not in ['Z','X'],'coordination-owner-not-live');return rest[19]
def validate_coordination(o,scope,now_ns,boot_id,owner_start,source_hashes):
 need(scope=='cloud054-exclusive-tools','tool-only-coordination-scope')
 need(o.get('version')==2 and o.get('scope')==scope and o.get('exclusive') is True and o.get('activeOperations')==[],'exclusive-coordination-receipt-required')
 need(o.get('bootID')==boot_id,'boot-identity-mismatch');issued=o.get('monotonicNS');expires=o.get('expiresMonotonicNS')
 need(type(issued) is int and type(expires) is int and 0<expires-issued<=600_000_000_000 and issued<=now_ns<=expires,'coordination-expired-or-invalid-window')
 pid=o.get('ownerPID');need(type(pid) is int and pid>1 and owner_start(pid)==str(o.get('ownerStartTicks')),'coordination-owner-changed')
 need(o.get('operationSources')==source_hashes,'coordination-source-pins-mismatch')
 allowed=o.get('acceptedUnknownProcesses');need(isinstance(allowed,list) and len(allowed)<=32,'explicit-bounded-unknown-list-required');seen=set()
 for x in allowed:
  need(isinstance(x,dict) and type(x.get('pid')) is int and x['pid']>1 and isinstance(x.get('startTicks'),str) and x['startTicks'].isdigit() and 1<=len(x['startTicks'])<=30,'invalid-unknown-identity')
  key=(x['pid'],x['startTicks']);need(key not in seen,'duplicate-unknown-identity');seen.add(key)
 return o

def quiescence(path,pin,scope):
 o=pinned_json(path,pin);sources={name:sha(read_bounded(ROOT/name)) for name in ['operation_common.py','relocate_tools.py']}
 return validate_coordination(o,scope,time.monotonic_ns(),Path('/proc/sys/kernel/random/boot_id').read_text().strip(),process_start,sources)

def process_scan(paths=(),docker=False,proc=Path('/proc')):
 """Finite observations; inaccessible live process observations conservatively refuse."""
 targets=[str(Path(p)) for p in paths];busy=[];unknown=[];observed=0
 def touches(value):
  value=value.removesuffix(' (deleted)');return any(value==p or value.startswith(p+'/') for p in targets)
 names=[p for p in proc.iterdir() if p.name.isdigit()];need(len(names)<=65536,'process-count-bound')
 for p in names:
  if int(p.name)==os.getpid():continue
  start=None
  try:
   statbytes=(p/'stat').read_text();fields=statbytes.rsplit(')',1)[1].split();start=fields[19]
   if fields[0]=='Z':continue
   observed+=1;matched=[]
   for name in ['exe','cwd']:
    value=os.readlink(p/name)
    if touches(value):matched.append(name)
    if docker and name=='exe' and Path(value.removesuffix(' (deleted)')).name in ['docker','buildctl','buildkitd','docker-buildx']:matched.append('docker-client-or-builder')
   with (p/'cmdline').open('rb') as handle:raw=handle.read(1024*1024+1)
   need(len(raw)<=1024*1024,'cmdline-bound')
   if any(touches(x.decode(errors='replace')) for x in raw.split(b'\0')):matched.append('argument-path')
   if targets:
    with (p/'maps').open('rb') as handle:maps=handle.read(8*1024*1024+1)
    need(len(maps)<=8*1024*1024,'maps-bound')
    if any(any(t.encode() in line for t in targets) for line in maps.splitlines()):matched.append('mapped-path')
    fds=list((p/'fd').iterdir());need(len(fds)<=8192,'fd-count-bound')
    for f in fds:
     try:
      if touches(os.readlink(f)):matched.append('open-path')
     except FileNotFoundError:pass
   after=(p/'stat').read_text().rsplit(')',1)[1].split()[19];need(after==start,'process-identity-changed')
   if matched:busy.append({'pid':int(p.name),'startTicks':start,'references':sorted(set(matched))})
  except (FileNotFoundError,ProcessLookupError):continue
  except (PermissionError,OSError,Refusal,IndexError):
   stable=False
   if start is not None:
    try:
     again=(p/'stat').read_text().rsplit(')',1)[1].split();stable=again[19]==start and again[0] not in ['Z','X']
    except (FileNotFoundError,ProcessLookupError):continue
    except (PermissionError,OSError,IndexError):pass
   unknown.append({'pid':int(p.name),'startTicks':start if stable else None,'identityStable':stable,'reason':'observation-unavailable-or-changed'})
 return {'observed':observed,'busy':busy,'unknown':unknown,'scope':'Readable exact executable/cwd/argv/maps/fd observations; no future-launch exclusion or universal daemon-operation proof'}
def require_idle(scan,coordination):
 need(not scan['busy'],'active-tool-reference')
 need(coordination.get('scope')=='cloud054-exclusive-tools','tool-only-coordination-scope')
 accepted={(x['pid'],x['startTicks']) for x in coordination['acceptedUnknownProcesses']}
 for x in scan['unknown']:
  need(x.get('identityStable') is True and x.get('startTicks') is not None and (x['pid'],x['startTicks']) in accepted,'new-missing-or-reused-unknown-process')
 return {'acceptedObservedUnknowns':scan['unknown'],'scope':'No readable active tool references observed. Exact unreadable identities explicitly accepted by root; not global inactivity proof or permission to stop/read environ of those processes.'}

