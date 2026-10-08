#!/usr/bin/env python3
"""Read-only owned-process + exact-label Docker sampler; no command lines/environments."""
import argparse, collections, datetime, hashlib, http.client, json, os, pathlib, re, socket, time
P=argparse.ArgumentParser()
P.add_argument('--driver-pid',type=int,required=True);P.add_argument('--driver-start-ticks',type=int,required=True)
P.add_argument('--instance-file',type=pathlib.Path);P.add_argument('--instance-path-file',type=pathlib.Path);P.add_argument('--image-id',required=True)
P.add_argument('--output',type=pathlib.Path,required=True);P.add_argument('--seconds',type=int,default=900)
P.add_argument('--settle-seconds',type=int,default=15);P.add_argument('--socket',default='/var/run/docker.sock')
a=P.parse_args(); assert 1<=a.seconds<=3600 and 0<=a.settle_seconds<=60
assert re.fullmatch(r'sha256:[0-9a-f]{64}',a.image_id)
assert (a.instance_file is None) != (a.instance_path_file is None)
instance=None
def resolve_instance():
 global instance
 if instance is not None:return instance
 try:
  path=a.instance_file if a.instance_file is not None else pathlib.Path(a.instance_path_file.read_text().strip())
  value=path.read_text().strip()
  if not re.fullmatch(r'[A-Za-z0-9_-]{1,128}',value):return None
  instance=value
 except OSError:return None
 return instance
a.output.mkdir(parents=True,exist_ok=False,mode=0o700)
HZ=os.sysconf('SC_CLK_TCK');PAGE=os.sysconf('SC_PAGE_SIZE');started=time.monotonic();now=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
tracked={a.driver_pid:a.driver_start_ticks}; identities={};previous={};docker_previous={};ever_containers=set();samples=0;driver_exit=None;peak=collections.defaultdict(float);counters=collections.defaultdict(float)
class UnixHTTP(http.client.HTTPConnection):
 def connect(self):
  self.sock=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM);self.sock.settimeout(.6);self.sock.connect(a.socket)
def get(path):
 conn=UnixHTTP('localhost',timeout=.6)
 try:
  conn.request('GET',path);r=conn.getresponse();body=r.read(2*1024*1024+1)
  if r.status!=200 or len(body)>2*1024*1024: raise ValueError('http-status-or-body-bound')
  return json.loads(body)
 finally:conn.close()
def stat(pid):
 raw=pathlib.Path('/proc',str(pid),'stat').read_text();end=raw.rfind(')');v=raw[end+2:].split()
 return {'pid':pid,'startTicks':int(v[19]),'parentPid':int(v[1]),'state':v[0],'comm':raw[raw.find('(')+1:end], 'cpuTicks':int(v[11])+int(v[12]),'residentBytes':max(0,int(v[21]))*PAGE}
def role(row):
 if row['pid']==a.driver_pid:return 'test-driver'
 name=row['comm'].lower()
 if 'chrome' in name or 'chromium' in name:return 'browser'
 if name=='luci' or name.startswith('luci-'):return 'control-plane'
 return 'other-owned-descendant'
def processes():
 allrows={};unreadable=0
 for p in pathlib.Path('/proc').iterdir():
  if not p.name.isdecimal():continue
  try:allrows[int(p.name)]=stat(int(p.name))
  except (OSError,ValueError,IndexError):unreadable+=1
 present={pid for pid,start in tracked.items() if pid in allrows and allrows[pid]['startTicks']==start}
 change=True
 while change:
  change=False
  for pid,row in allrows.items():
   parent=row['parentPid']
   if pid not in present and parent in present and allrows[parent]['startTicks']<=row['startTicks']:
    tracked[pid]=row['startTicks'];present.add(pid);change=True
 result=[]
 for pid in sorted(present):
  row=allrows[pid];identity=(pid,row['startTicks']);row['role']=role(row);row.pop('comm',None)
  try:
   v={p.split(':')[0]:int(p.split(':')[1]) for p in pathlib.Path('/proc',str(pid),'io').read_text().splitlines()}
   row['io']={k:v.get(k) for k in ('read_bytes','write_bytes','rchar','wchar','cancelled_write_bytes')}
  except (OSError,ValueError):row['io']=None
  try:
   if stat(pid)['startTicks']!=row['startTicks']:continue
  except (OSError,ValueError,IndexError):continue
  old=previous.get(identity)
  if old:
   dt=time.monotonic()-old['time'];cpu=(row['cpuTicks']-old['cpuTicks'])/HZ
   row['cpuCoreEquivalent']=max(0,cpu/dt) if dt>0 else None
   counters['observedProcessCPUSeconds']+=max(0,cpu)
   if row['io'] and old['io']:
    row['ioDelta']={k:max(0,row['io'][k]-old['io'][k]) for k in ('read_bytes','write_bytes','rchar','wchar')}
    for k,v in row['ioDelta'].items():counters['observedProcessIO_'+k]+=v
  previous[identity]={'time':time.monotonic(),'cpuTicks':row['cpuTicks'],'io':row['io']}
  identities[str(pid)+':'+str(row['startTicks'])]={'pid':pid,'startTicks':row['startTicks'],'role':row['role']}
  result.append(row)
 return result,unreadable
def host():
 out={}
 try:
  mem={line.split(':')[0]:int(line.split()[1])*1024 for line in pathlib.Path('/proc/meminfo').read_text().splitlines()}
  out['memory']={k:mem.get(k) for k in ('MemTotal','MemAvailable','SwapTotal','SwapFree','CommitLimit','Committed_AS')}
  out['memory']['commitBasis']='Linux overcommit accounting; not equivalent to Windows committed private memory'
 except (OSError,ValueError):out['memory']=None
 try:
  line=pathlib.Path('/proc/stat').read_text().splitlines()[0].split()[1:9];out['cpuTicks']=[int(v) for v in line]
 except (OSError,ValueError):out['cpuTicks']=None
 out['pressure']={}
 for name in ('cpu','memory','io'):
  try:out['pressure'][name]=pathlib.Path('/proc/pressure',name).read_text().strip()
  except OSError:out['pressure'][name]=None
 out['disks']={}
 for path in ('/','/workspace','/tmp'):
  try:
   s=os.statvfs(path);out['disks'][path]={'availableBytes':s.f_bavail*s.f_frsize,'totalBytes':s.f_blocks*s.f_frsize}
  except OSError:out['disks'][path]=None
 return out
def containers():
 result=[];issues=[]
 if resolve_instance() is None:return [],['exact-owned-instance-not-yet-provided']
 try:rows=get('/containers/json?all=1')
 except Exception as err:return [],['inventory-'+type(err).__name__]
 candidates=[r for r in rows if r.get('Labels',{}).get('luci.instance')==instance and r.get('Labels',{}).get('luci.managed')=='true']
 if len(candidates)>8:return [],['owned-inventory-exceeded-eight-container-bound']
 for r in candidates:
  cid=r.get('Id','')
  if not re.fullmatch(r'[0-9a-f]{64}',cid):issues.append('invalid-container-id');continue
  try:
   value=get('/containers/'+cid+'/json')
   labels=value.get('Config',{}).get('Labels',{})
   if labels.get('luci.instance')!=instance or labels.get('luci.managed')!='true' or value.get('Image')!=a.image_id:
    issues.append('container-ownership-mismatch');continue
   v={'id':cid,'sandboxId':labels.get('luci.sandbox-id'),'pid':value['State'].get('Pid'),'running':value['State'].get('Running'),'memoryLimitBytes':value['HostConfig'].get('Memory'),'nanoCPULimit':value['HostConfig'].get('NanoCpus')}
   ever_containers.add(cid)
   if v['pid']:
    try:v['pidStartTicks']=stat(v['pid'])['startTicks']
    except (OSError,ValueError,IndexError):v['pidStartTicks']=None
   if v['running']:
    stats=get('/containers/'+cid+'/stats?stream=false&one-shot=true');mem=stats.get('memory_stats',{});cpu=stats.get('cpu_stats',{}).get('cpu_usage',{}).get('total_usage')
    v.update({'statsReadAt':stats.get('read'),'memoryUsageBytes':mem.get('usage'),'memoryUsageBasis':'Docker cgroup usage; not added to process RSS','cpuUsageNanoseconds':cpu,'pidsCurrent':stats.get('pids_stats',{}).get('current')})
    inactive=mem.get('stats',{}).get('inactive_file',mem.get('stats',{}).get('total_inactive_file'))
    v['workingSetBytes']=max(0,mem['usage']-inactive) if isinstance(mem.get('usage'),int) and isinstance(inactive,int) else None
    old=docker_previous.get(cid);t=time.monotonic()
    if old and isinstance(cpu,int) and cpu>=old[1]:v['cpuCoreEquivalent']=(cpu-old[1])/1e9/(t-old[0])
    if isinstance(cpu,int):docker_previous[cid]=(t,cpu)
    io=stats.get('blkio_stats',{}).get('io_service_bytes_recursive');v['blockIOBytes']=None if io is None else dict(collections.Counter({op:sum(x.get('value',0) for x in io if x.get('op','').lower()==op) for op in ('read','write')}))
   result.append(v)
  except Exception as err:issues.append('container-observation-'+type(err).__name__)
 return result,issues
metadata={'startedAt':now(),'driver':{'pid':a.driver_pid,'startTicks':a.driver_start_ticks},'intervalSeconds':1,'baseline':'post-start; no pre-run idle baseline observed','imageId':a.image_id,'instanceIdentitySHA256':None,'scope':'Exact driver descendants by PID/starttime, plus exact instance/managed/image-verified Docker containers. No command lines, environments, mount paths or provider payloads recorded.','limitations':['Sum of process RSS double-counts shared pages; it is not unique physical memory.','Container cgroup memory is separate and must not be added to container process RSS.','Docker daemon/shared kernel/VM overhead is unassigned; host memory includes unrelated work.','Processes starting and exiting between 1 Hz samples may be missed; delta counters cover observed intervals only.','Post-driver-exit settlement is not an application idle baseline.','No UI input latency, other-app responsiveness or model reasoning quality is measured.'],'helperSHA256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
(a.output/'metadata.json').write_text(json.dumps(metadata,indent=2)+'\n');os.chmod(a.output/'metadata.json',0o600)
(a.output/'accepted-sampler.py').write_bytes(pathlib.Path(__file__).read_bytes());os.chmod(a.output/'accepted-sampler.py',0o600)
with (a.output/'samples.jsonl').open('x') as stream:
 os.chmod(a.output/'samples.jsonl',0o600)
 while time.monotonic()-started<a.seconds:
  tick=time.monotonic();proc,unreadable=processes();owned,issues=containers();living=any(p['pid']==a.driver_pid and p['startTicks']==a.driver_start_ticks and p['state']!='Z' for p in proc)
  if not living and driver_exit is None:driver_exit=time.monotonic()
  row={'observedAt':now(),'elapsedSeconds':time.monotonic()-started,'phase':'post-start-running' if driver_exit is None else 'post-driver-exit-settle','driverAlive':living,'host':host(),'processes':proc,'containers':owned,'containerIssues':issues,'processStatUnreadableOrRacedCount':unreadable,'sampleDurationSeconds':time.monotonic()-tick}
  groups=collections.Counter()
  for p in proc:groups[p['role']]+=p['residentBytes']
  row['summedProcessRSSBytesByRole']=dict(groups);row['summedProcessRSSBytes']=sum(groups.values())
  for k,v in groups.items():peak['summedRSS_'+k]=max(peak['summedRSS_'+k],v)
  peak['summedRSS_allOwnedProcesses']=max(peak['summedRSS_allOwnedProcesses'],row['summedProcessRSSBytes'])
  for c in owned:
   if c.get('memoryUsageBytes') is not None:peak['singleContainerCgroupUsageBytes']=max(peak['singleContainerCgroupUsageBytes'],c['memoryUsageBytes'])
  stream.write(json.dumps(row,separators=(',',':'))+'\n');stream.flush();samples+=1
  if driver_exit is not None and time.monotonic()-driver_exit>=a.settle_seconds:break
  time.sleep(max(0,1-(time.monotonic()-tick)))
summary={'finishedAt':now(),'samples':samples,'durationSeconds':time.monotonic()-started,'driverExitObserved':driver_exit is not None,'peakObserved':dict(peak),'observedCounterDeltas':dict(counters),'processIdentities':list(identities.values()),'containerIds':sorted(ever_containers),'limitations':metadata['limitations'],'baseline':metadata['baseline'],'instanceIdentitySHA256':hashlib.sha256(instance.encode()).hexdigest() if instance else None}
(a.output/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');os.chmod(a.output/'summary.json',0o600)
print(json.dumps({'samples':samples,'durationSeconds':summary['durationSeconds'],'containerCount':len(ever_containers),'driverExitObserved':summary['driverExitObserved']}))
