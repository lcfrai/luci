#!/usr/bin/env python3
"""Read-only owned-process + exact-label Docker sampler; no command lines/environments."""
import argparse, collections, datetime, hashlib, http.client, json, os, pathlib, re, socket, time
P=argparse.ArgumentParser()
P.add_argument('--driver-pid',type=int,required=True);P.add_argument('--driver-start-ticks',type=int,required=True)
P.add_argument('--instance-file',type=pathlib.Path);P.add_argument('--instance-path-file',type=pathlib.Path);P.add_argument('--image-id',required=True)
P.add_argument('--output',type=pathlib.Path,required=True);P.add_argument('--seconds',type=int,default=900)
P.add_argument('--include-checks',action='store_true',help='Also attribute exact-instance protocol-1 checks; strict identity/stat/PID/cgroup failures remain explicit.')
P.add_argument('--settle-seconds',type=int,default=15);P.add_argument('--socket',default='/var/run/docker.sock')
instance=None
def resolve_instance():
 global instance
 if instance is not None and not a.include_checks:return instance
 try:
  path=a.instance_file if a.instance_file is not None else pathlib.Path(a.instance_path_file.read_text().strip())
  value=path.read_text().strip()
  if not re.fullmatch(r'[A-Za-z0-9_-]{1,128}',value):return None
  if instance is not None and value!=instance:raise ValueError('instance-identity-changed')
  instance=value
 except OSError:return None
 return instance
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
CHECK_LABELS=('luci.check','luci.check-protocol','luci.instance','luci.check-engine','luci.check-allocation','luci.check-id','luci.check-command-index','luci.check-scope','luci.check-project','luci.check-command-digest','luci.check-source-digest','luci.check-image')
class MeasurementError(Exception):pass
def require_measurement(condition,code):
 if not condition:raise MeasurementError(code)
def check_identity(labels,engine_id):
 require_measurement(isinstance(labels,dict) and set(labels)==set(CHECK_LABELS),'check-label-set-invalid')
 require_measurement(all(isinstance(value,str) for value in labels.values()),'check-label-type-invalid')
 require_measurement(labels['luci.check']=='true' and labels['luci.check-protocol']=='1','check-protocol-invalid')
 require_measurement(labels['luci.instance']==instance and labels['luci.check-engine']==engine_id,'check-instance-or-engine-mismatch')
 require_measurement(labels['luci.check-image']==a.image_id,'check-label-image-mismatch')
 for key,prefix in (('luci.check-allocation','checkexec-'),('luci.check-id','check-')):
  require_measurement(re.fullmatch(prefix+'[0-9a-f]{24}',labels[key]) is not None,'check-allocation-or-check-id-invalid')
 require_measurement(re.fullmatch(r'[0-7]',labels['luci.check-command-index']) is not None,'check-command-index-invalid')
 require_measurement(re.fullmatch(r'[a-z0-9_-]{1,200}',labels['luci.check-project']) is not None,'check-project-invalid')
 for key in ('luci.check-scope','luci.check-command-digest','luci.check-source-digest'):
  require_measurement(re.fullmatch(r'[0-9a-f]{64}',labels[key]) is not None,'check-digest-invalid')
 return {key:labels[key] for key in CHECK_LABELS}
def read_cgroup(pid):
 with pathlib.Path('/proc',str(pid),'cgroup').open() as stream:
  value=stream.read(65537)
 require_measurement(len(value)<=65536,'check-cgroup-body-bound')
 return value
def current_check_process(pid,cid):
 require_measurement(type(pid) is int and pid>0,'check-running-pid-invalid')
 current=stat(pid)
 require_measurement(current['pid']==pid and current['state']!='Z','check-process-not-current')
 groups=read_cgroup(pid);entries=[line.split(':',2) for line in groups.splitlines() if len(line.split(':',2))==3]
 paths=[path for _,controllers,path in entries if not controllers or set(controllers.split(','))&{'memory','cpu','cpuacct','pids'}]
 require_measurement(paths and all(re.search(r'(?:^|/)(?:docker-)?'+re.escape(cid)+r'(?:\.scope)?$',path) for path in paths),'check-cgroup-identity-mismatch')
 return current['startTicks'],hashlib.sha256(groups.encode()).hexdigest()
def check_inspection(value,cid,labels):
 require_measurement(value.get('Id')==cid,'check-inspect-id-mismatch')
 require_measurement(value.get('Image')==a.image_id and value.get('Config',{}).get('Labels')==labels,'check-inspect-identity-changed')
 require_measurement(value.get('Name')=='/luci-check-'+labels['luci.check-allocation'],'check-container-name-mismatch')
def fresh_check_stats(stats,cid):
 require_measurement(stats.get('id')==cid,'check-stats-id-mismatch')
 try:stamp=datetime.datetime.fromisoformat(stats.get('read','').replace('Z','+00:00'));age=(datetime.datetime.now(datetime.timezone.utc)-stamp).total_seconds()
 except (ValueError,TypeError):raise MeasurementError('check-stats-time-invalid')
 require_measurement(-1<=age<=5,'check-stats-not-fresh')
 mem=stats.get('memory_stats',{});cpu=stats.get('cpu_stats',{}).get('cpu_usage',{}).get('total_usage')
 require_measurement(type(mem.get('usage')) is int and 0<=mem['usage']<2**64 and type(cpu) is int and 0<=cpu<2**64,'check-stats-measurement-unavailable')

def containers():
 result=[];issues=[]
 try:owned_instance=resolve_instance()
 except ValueError:return [],['exact-owned-instance-identity-changed']
 if owned_instance is None:return [],['exact-owned-instance-not-yet-provided']
 try:rows=get('/containers/json?all=1')
 except Exception as err:return [],['inventory-'+type(err).__name__]
 if not isinstance(rows,list):return [],['inventory-response-invalid']
 candidates=[]
 for row in rows:
  if not isinstance(row,dict):issues.append('inventory-entry-invalid');continue
  labels=row.get('Labels',{})
  if not isinstance(labels,dict):issues.append('inventory-labels-invalid');continue
  if labels.get('luci.instance')!=instance:continue
  check_candidate=a.include_checks and any(key=='luci.check' or key.startswith('luci.check-') for key in labels)
  if labels.get('luci.managed')=='true' or check_candidate:candidates.append((row,'check' if check_candidate else 'worker'))
 if len(candidates)>8:return [],['owned-inventory-exceeded-eight-container-bound']
 engine_id=None
 if any(kind=='check' for _,kind in candidates):
  try:
   info=get('/info');engine_id=info.get('ID');require_measurement(isinstance(engine_id,str) and 0<len(engine_id)<=200 and info.get('OSType')=='linux','check-engine-identity-unavailable')
  except Exception as err:return [],['check-engine-observation-'+(str(err) if isinstance(err,MeasurementError) else type(err).__name__)]
 seen=set()
 for r,kind in candidates:
  cid=r.get('Id','')
  if not isinstance(cid,str) or not re.fullmatch(r'[0-9a-f]{64}',cid):issues.append('invalid-container-id');continue
  if cid in seen:issues.append('duplicate-container-id');continue
  seen.add(cid)
  try:
   expected_labels=check_identity(r.get('Labels'),engine_id) if kind=='check' else None
   value=get('/containers/'+cid+'/json');labels=value.get('Config',{}).get('Labels',{})
   if kind=='check':check_inspection(value,cid,expected_labels)
   elif labels.get('luci.instance')!=instance or labels.get('luci.managed')!='true' or value.get('Image')!=a.image_id:
    issues.append('container-ownership-mismatch');continue
   v={'id':cid,'sandboxId':labels.get('luci.sandbox-id'),'pid':value['State'].get('Pid'),'running':value['State'].get('Running'),'memoryLimitBytes':value['HostConfig'].get('Memory'),'nanoCPULimit':value['HostConfig'].get('NanoCpus')}
   if a.include_checks:v.update({'kind':kind,'memorySwapLimitBytes':value['HostConfig'].get('MemorySwap'),'memorySwapLimitBasis':'Docker combined RAM+swap cap; -1 unlimited; 0 daemon default','pidsLimit':value['HostConfig'].get('PidsLimit')})
   if kind=='check':
    require_measurement(type(v['running']) is bool and type(v['pid']) is int and v['pid']>=0,'check-state-invalid')
    require_measurement(type(v['memoryLimitBytes']) is int and type(v['memorySwapLimitBytes']) is int,'check-configured-memory-limits-unavailable')
    v.update({'checkId':labels['luci.check-id'],'allocationId':labels['luci.check-allocation'],'commandIndex':int(labels['luci.check-command-index']),'protocol':1})
   if v['pid']:
    if kind=='check':v['pidStartTicks'],v['cgroupIdentitySHA256']=current_check_process(v['pid'],cid)
    else:
     try:v['pidStartTicks']=stat(v['pid'])['startTicks']
     except (OSError,ValueError,IndexError):v['pidStartTicks']=None
   if v['running']:
    if kind=='check':require_measurement(v.get('pidStartTicks') is not None,'check-running-pid-unverified')
    stats=get('/containers/'+cid+'/stats?stream=false&one-shot=true')
    if kind=='check':
     fresh_check_stats(stats,cid)
     fresh=get('/containers/'+cid+'/json');check_inspection(fresh,cid,expected_labels)
     require_measurement(fresh.get('State',{}).get('Running') is True and fresh['State'].get('Pid')==v['pid'],'check-restarted-or-exited-during-observation')
     start,cgroup=current_check_process(v['pid'],cid)
     require_measurement(start==v['pidStartTicks'] and cgroup==v['cgroupIdentitySHA256'],'check-process-or-cgroup-changed')
    mem=stats.get('memory_stats',{});cpu=stats.get('cpu_stats',{}).get('cpu_usage',{}).get('total_usage')
    v.update({'statsReadAt':stats.get('read'),'memoryUsageBytes':mem.get('usage'),'memoryUsageBasis':'Docker cgroup usage; not added to process RSS','cpuUsageNanoseconds':cpu,'pidsCurrent':stats.get('pids_stats',{}).get('current')})
    inactive=mem.get('stats',{}).get('inactive_file',mem.get('stats',{}).get('total_inactive_file'))
    if kind=='check' and inactive is not None and (type(inactive) is not int or inactive<0 or inactive>mem['usage']):
     issues.append('check-working-set-counter-invalid');v['workingSetBytes']=None
    else:v['workingSetBytes']=max(0,mem['usage']-inactive) if isinstance(mem.get('usage'),int) and isinstance(inactive,int) else None
    counter_identity=(cid,v['pid'],v['pidStartTicks']) if kind=='check' else cid
    old=docker_previous.get(counter_identity);t=time.monotonic()
    if old and isinstance(cpu,int) and cpu>=old[1] and t>old[0]:v['cpuCoreEquivalent']=(cpu-old[1])/1e9/(t-old[0])
    if isinstance(cpu,int):docker_previous[counter_identity]=(t,cpu)
    io=stats.get('blkio_stats',{}).get('io_service_bytes_recursive');v['blockIOBytes']=None if io is None else dict(collections.Counter({op:sum(x.get('value',0) for x in io if x.get('op','').lower()==op) for op in ('read','write')}))
   ever_containers.add(cid);container_kinds[cid]=kind;result.append(v)
  except MeasurementError as err:issues.append(str(err))
  except Exception as err:issues.append(('check' if kind=='check' else 'container')+'-observation-'+type(err).__name__)
 return result,issues
def main():
 global a,instance,HZ,PAGE,started,now,tracked,identities,previous,docker_previous,ever_containers,samples,driver_exit,peak,counters,container_kinds
 instance=None
 a=P.parse_args(); assert 1<=a.seconds<=3600 and 0<=a.settle_seconds<=60
 assert re.fullmatch(r'sha256:[0-9a-f]{64}',a.image_id)
 assert (a.instance_file is None) != (a.instance_path_file is None)
 a.output.mkdir(parents=True,exist_ok=False,mode=0o700)
 HZ=os.sysconf('SC_CLK_TCK');PAGE=os.sysconf('SC_PAGE_SIZE');started=time.monotonic();now=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
 tracked={a.driver_pid:a.driver_start_ticks}; identities={};previous={};docker_previous={};ever_containers=set();samples=0;driver_exit=None;peak=collections.defaultdict(float);counters=collections.defaultdict(float)
 container_kinds={};issue_counts=collections.Counter();check_coverage={'identitySamples':0,'cgroupUsageSamples':0}
 metadata={'startedAt':now(),'driver':{'pid':a.driver_pid,'startTicks':a.driver_start_ticks},'intervalSeconds':1,'baseline':'post-start; no pre-run idle baseline observed','imageId':a.image_id,'includeChecks':a.include_checks,'instanceIdentitySHA256':None,'scope':('Exact driver descendants and managed workers; opted-in checks additionally require exact instance/image/protocol/engine/allocation identity and fresh stats bounded by current PID/start/cgroup checks.' if a.include_checks else 'Exact driver descendants by PID/starttime, plus exact instance/managed/image-verified Docker containers. No command lines, environments, mount paths or provider payloads recorded.'),'limitations':['Sum of process RSS double-counts shared pages; it is not unique physical memory.','Container cgroup memory is separate and must not be added to container process RSS.','Docker daemon/shared kernel/VM overhead is unassigned; host memory includes unrelated work.','Processes starting and exiting between 1 Hz samples may be missed; delta counters cover observed intervals only.','Post-driver-exit settlement is not an application idle baseline.','No UI input latency, other-app responsiveness or model reasoning quality is measured.'],'helperSHA256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
 if not a.include_checks:metadata.pop('includeChecks')
 (a.output/'metadata.json').write_text(json.dumps(metadata,indent=2)+'\n');os.chmod(a.output/'metadata.json',0o600)
 (a.output/'accepted-sampler.py').write_bytes(pathlib.Path(__file__).read_bytes());os.chmod(a.output/'accepted-sampler.py',0o600)
 with (a.output/'samples.jsonl').open('x') as stream:
  os.chmod(a.output/'samples.jsonl',0o600)
  while time.monotonic()-started<a.seconds:
   tick=time.monotonic();proc,unreadable=processes();owned,issues=containers();living=any(p['pid']==a.driver_pid and p['startTicks']==a.driver_start_ticks and p['state']!='Z' for p in proc)
   if not living and driver_exit is None:driver_exit=time.monotonic()
   row={'observedAt':now(),'elapsedSeconds':time.monotonic()-started,'phase':'post-start-running' if driver_exit is None else 'post-driver-exit-settle','driverAlive':living,'host':host(),'processes':proc,'containers':owned,'containerIssues':issues,'containerMeasurementStatus':'partial' if issues else 'observed','processStatUnreadableOrRacedCount':unreadable,'sampleDurationSeconds':time.monotonic()-tick}
   if not a.include_checks:row.pop('containerMeasurementStatus')
   groups=collections.Counter()
   for p in proc:groups[p['role']]+=p['residentBytes']
   row['summedProcessRSSBytesByRole']=dict(groups);row['summedProcessRSSBytes']=sum(groups.values())
   for k,v in groups.items():peak['summedRSS_'+k]=max(peak['summedRSS_'+k],v)
   peak['summedRSS_allOwnedProcesses']=max(peak['summedRSS_allOwnedProcesses'],row['summedProcessRSSBytes'])
   issue_counts.update(issues)
   for c in owned:
    if c.get('kind','worker')=='check':check_coverage['identitySamples']+=1
    if c.get('kind','worker')=='check' and c.get('memoryUsageBytes') is not None:check_coverage['cgroupUsageSamples']+=1
    if c.get('memoryUsageBytes') is not None:peak['singleContainerCgroupUsageBytes']=max(peak['singleContainerCgroupUsageBytes'],c['memoryUsageBytes'])
    if a.include_checks and c.get('memoryUsageBytes') is not None:peak['singleContainerCgroupUsageBytes_'+c['kind']]=max(peak['singleContainerCgroupUsageBytes_'+c['kind']],c['memoryUsageBytes'])
   stream.write(json.dumps(row,separators=(',',':'))+'\n');stream.flush();samples+=1
   if driver_exit is not None and time.monotonic()-driver_exit>=a.settle_seconds:break
   time.sleep(max(0,1-(time.monotonic()-tick)))
 summary={'finishedAt':now(),'samples':samples,'durationSeconds':time.monotonic()-started,'driverExitObserved':driver_exit is not None,'peakObserved':dict(peak),'observedCounterDeltas':dict(counters),'processIdentities':list(identities.values()),'containerIds':sorted(ever_containers),'containerKinds':container_kinds,'containerIssueCounts':dict(issue_counts),'checkCoverage':{'enabled':a.include_checks,**check_coverage,'status':('disabled' if not a.include_checks else 'cgroup-usage-observed' if check_coverage['cgroupUsageSamples'] else 'identity-only' if check_coverage['identitySamples'] else 'not-observed')},'limitations':metadata['limitations'],'baseline':metadata['baseline'],'instanceIdentitySHA256':hashlib.sha256(instance.encode()).hexdigest() if instance else None}
 if not a.include_checks:
  for key in ('containerKinds','containerIssueCounts','checkCoverage'):summary.pop(key)
 (a.output/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');os.chmod(a.output/'summary.json',0o600)
 print(json.dumps({'samples':samples,'durationSeconds':summary['durationSeconds'],'containerCount':len(ever_containers),'driverExitObserved':summary['driverExitObserved']}))

if __name__=='__main__':main()
