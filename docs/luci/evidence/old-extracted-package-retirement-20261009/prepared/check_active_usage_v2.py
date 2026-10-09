"""One bounded read-only /proc snapshot for six exact old extracted packages.

Never reads environ, file-descriptor contents, target directories or archives;
never signals a process or changes a package. Stdout contains only a summary.
"""
from pathlib import Path
import datetime,errno,hashlib,json,os,re,stat,time

ROOT=Path('/workspace/.luci-tools/old-extracted-packages-retirement-plan')
TARGETS=tuple('/tmp/luci-retained-tools/package-0.5.0-'+x for x in
              ('0dcafb14','2da1a821','67a5b7c7','99007404','d20ebd29','edd0cb12'))
KEEP=('b94','299','5f')
MAX_PROCESSES=8192;MAX_FDS=512;MAX_STAT=8192;MAX_CMDLINE=65536;MAX_MAPS=2*1024*1024
MAX_TOTAL_BYTES=16*1024*1024;MAX_SECONDS=20
began=time.monotonic();read_bytes=0;read_calls=0;link_calls=0;bounds=[]
patterns={p:re.compile(r'(?<![A-Za-z0-9_./-])'+re.escape(p)+r'(?=$|[/\x00\s\'":,;)\]])')for p in TARGETS}

class Bound(Exception):pass
def budget():
 if time.monotonic()-began>MAX_SECONDS:raise Bound('cooperative-time-limit')
def read(p,cap):
 global read_bytes,read_calls
 budget();remaining=MAX_TOTAL_BYTES-read_bytes
 if remaining<=0:raise Bound('total-read-byte-limit')
 size=min(cap+1,remaining+1);read_calls+=1
 with open(p,'rb',buffering=0)as f:b=f.read(size)
 read_bytes+=len(b)
 if len(b)>cap:raise Bound('field-byte-limit')
 if read_bytes>MAX_TOTAL_BYTES:raise Bound('total-read-byte-limit')
 return b
def identity(pid):
 b=read(Path('/proc')/str(pid)/'stat',MAX_STAT).decode('ascii',errors='replace')
 f=b.rsplit(')',1)[1].split()
 return {'pid':pid,'startTicks':f[19],'state':f[0],'ppid':int(f[1])}
def link(p):
 global link_calls
 budget();link_calls+=1;return os.readlink(p)
def matches(value):return [p for p,rx in patterns.items()if rx.search(value)]
def classify(e):
 if isinstance(e,Bound):return str(e)
 if isinstance(e,PermissionError):return 'permission-denied'
 if isinstance(e,(FileNotFoundError,ProcessLookupError)):return 'absent-during-observation'
 return type(e).__name__

def main():
 global bounds
 source_before=Path(__file__).read_bytes()
 self_id=identity(os.getpid());rows=[];busy=[];unknown=[];identity_rows=[]
 pids=sorted(int(n)for n in os.listdir('/proc')if n.isdecimal())
 listed_count=len(pids)
 if len(pids)>MAX_PROCESSES:bounds.append('process-count-limit');pids=pids[:MAX_PROCESSES]
 for pid in pids:
  try:budget()
  except Bound as e:bounds.append(str(e));break
  row={'pid':pid,'identityBefore':None,'identityAfter':None,'identityStable':False,'matches':[],'unavailable':[]}
  try:row['identityBefore']=identity(pid)
  except Exception as e:
   row['unavailable'].append({'field':'identity-before','reason':classify(e)})
   rows.append(row);unknown.append({'pid':pid,'startTicks':None,'reason':'identity-unavailable-before'});continue
  identity_rows.append(row)
 # Gather bounded identities first, then read active process fields before zombies.
 identity_rows.sort(key=lambda row:(row['identityBefore']['state']in ('Z','X'),row['pid']!=self_id['pid'],row['pid']))
 for row in identity_rows:
  pid=row['pid']
  if row['identityBefore']['state']in ('Z','X'):
   row['fieldObservationSkipped']='initial identity is literal non-live Z/X; no exe/cwd/cmdline/maps/fd reads'
   row['inspector']=False;rows.append(row);continue
  try:budget()
  except Bound as e:
   row['unavailable'].append({'field':'live-fields','reason':str(e)})
   unknown.append({'pid':pid,'startTicks':row['identityBefore']['startTicks'],'state':row['identityBefore']['state'],'reasons':row['unavailable']});rows.append(row)
   if str(e)not in bounds:bounds.append(str(e))
   continue
  base=Path('/proc')/str(pid)
  for field in ('exe','cwd','cmdline','maps','fd'):
   try:
    budget()
    if field in ('exe','cwd'):values=[link(base/field)]
    elif field=='cmdline':values=[read(base/field,MAX_CMDLINE).decode('utf-8',errors='replace')]
    elif field=='maps':values=[read(base/field,MAX_MAPS).decode('utf-8',errors='replace')]
    else:
     fds=sorted(os.listdir(base/'fd'))
     if len(fds)>MAX_FDS:row['unavailable'].append({'field':'fd','reason':'descriptor-count-limit','observedCount':len(fds)})
     for fd in fds[:MAX_FDS]:
      try:
       value=link(base/'fd'/fd)
       for target in matches(value):row['matches'].append({'field':'fd','descriptor':fd,'target':target})
      except Exception as e:row['unavailable'].append({'field':'fd','descriptor':fd,'reason':classify(e)})
     continue
    for value in values:
     for target in matches(value):row['matches'].append({'field':field,'target':target})
   except Exception as e:row['unavailable'].append({'field':field,'reason':classify(e)})
  try:
   row['identityAfter']=identity(pid)
   row['identityStable']=row['identityBefore']['startTicks']==row['identityAfter']['startTicks']
   if not row['identityStable']:row['unavailable'].append({'field':'identity','reason':'generation-changed'})
  except Exception as e:row['unavailable'].append({'field':'identity-after','reason':classify(e)})
  row['inspector']=pid==self_id['pid']and row['identityBefore']['startTicks']==self_id['startTicks']
  # Preserve positive references even if a later field fails or identity changes.
  if row['matches']:busy.append({'pid':pid,'startTicks':row['identityBefore']['startTicks'],'state':row['identityBefore']['state'],'identityStable':row['identityStable'],'inspector':row['inspector'],'matches':row['matches']})
  if row['unavailable']:unknown.append({'pid':pid,'startTicks':row['identityBefore']['startTicks'],'state':row['identityBefore']['state'],'identityStable':row['identityStable'],'reasons':row['unavailable']})
  rows.append(row)
 source=Path(__file__).read_bytes()
 report={'version':2,'observedAtUTC':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source':{'path':str(Path(__file__).resolve()),'bytes':len(source),'sha256':hashlib.sha256(source).hexdigest(),'stableBeforeAfter':source==source_before},'targets':TARGETS,'protectedOutsideScopePrefixes':KEEP,'inspector':self_id,'result':'target-references-observed'if busy else'no-target-reference-observed-in-readable-fields','busy':busy,'unknown':unknown,'processes':rows,'literalZombieIdentities':[r['identityBefore']for r in rows if r['identityBefore']and r['identityBefore']['state']=='Z'],'bounds':{'secondsCooperative':MAX_SECONDS,'maxProcesses':MAX_PROCESSES,'maxFDsPerProcess':MAX_FDS,'maxStatBytes':MAX_STAT,'maxCmdlineBytes':MAX_CMDLINE,'maxMapsBytes':MAX_MAPS,'maxTotalReadBytes':MAX_TOTAL_BYTES,'reached':bounds},'cost':{'elapsedSeconds':time.monotonic()-began,'processesListed':listed_count,'identitiesAttempted':len(pids),'processesObserved':len(rows),'initialLiveIdentities':sum(r['identityBefore']['state']not in ('Z','X')for r in identity_rows),'bytesRead':read_bytes,'readCalls':read_calls,'readlinkCalls':link_calls},'priorIncompleteReceipt':'active-usage.json','limits':['Point-in-time readable /proc evidence only; no universal idle/no-future-launch proof.','Unknown/missing/changing observations remain explicit. No unobserved process is treated as absent or safe.','Positive references remain listed if another field is unreadable; exact identity instability limits attribution.','Zombie states are literal observed Z, not absence. Inspector identity is recorded separately. Fields are deliberately not read for initial Z/X identities; they are not included in readable active-field coverage.','No environ, private file contents, descriptor contents, package trees or archive payloads were read.','Exact string/path matches can miss application-retained or later-created references; current root coordination and separate full retention/member proof remain necessary.','All reads have cooperative bounds; synchronous proc read scheduling is not preemptively bounded. No signals, Docker, storage changes, tests or application starts.']}
 out=ROOT/'active-usage-v2.json'
 if out.exists():raise RuntimeError('prior result exists; preserve it and use a separately named source before another scan')
 b=(json.dumps(report,indent=2)+'\n').encode();fd=os.open(out,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
 with os.fdopen(fd,'wb')as f:f.write(b);f.flush();os.fsync(f.fileno())
 print(json.dumps({'path':str(out),'sha256':hashlib.sha256(b).hexdigest(),'sourceSHA256':report['source']['sha256'],'result':report['result'],'busyCount':len(busy),'unknownCount':len(unknown),'zombies':len(report['literalZombieIdentities']),'cost':report['cost']}))
if __name__=='__main__':main()
