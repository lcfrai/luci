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
# Additive, read-only visible control-plane memory diagnostics. No Docker queries.
# Formula mirrors resourcebudget samplePlatform/applyCgroup/applyMemoryLimit;
# observation/namespace/identity bounds below can conservatively make it unavailable.
CGROUP_MEMORY_BOUNDS={'targets':8,'hierarchyLevels':128,'nodeRows':128,'reads':512,'bytes':1048576,'seconds':0.05,'outputBytes':131072}
CGROUP_MEMORY_PRODUCT_BASIS={
 'control-plane/internal/resourcebudget/probe_linux.go':'ed0294860d6c721b1fd8f96da8c09a0f4fecb68b991c66c3b8f524cda985c405',
 'control-plane/internal/resourcebudget/cgroup_linux.go':'1843acdc15db9f9876545a1393bf6c21b93bc01873a64d8651be3adb26b5a4d1',
}
CGROUP_MEMORY_STAT_KEYS=frozenset(('anon','file','kernel','kernel_stack','pagetables','percpu','sock','shmem','file_mapped','file_dirty','file_writeback','inactive_anon','active_anon','inactive_file','active_file','unevictable','slab_reclaimable','slab_unreclaimable','slab','pgfault','pgmajfault','cache','rss','rss_huge','mapped_file','dirty','writeback','swap','total_cache','total_rss','total_inactive_file','total_active_file','hierarchical_memory_limit'))
class CgroupMemoryUnavailable(Exception):pass
def cgroup_memory_read(path,limit):
 with pathlib.Path(path).open('rb') as stream:return stream.read(limit+1)
class CgroupMemoryBudget:
 def __init__(self,read,link,clock,bounds):
  self.reader=read;self.linker=link;self.clock=clock;self.bounds=bounds;self.started=clock();self.reads=0;self.bytes=0;self.nodes=0
 def check(self):
  if self.clock()-self.started>self.bounds['seconds']:raise CgroupMemoryUnavailable('observation-time-bound')
  if self.reads>=self.bounds['reads']:raise CgroupMemoryUnavailable('observation-read-bound')
 def read(self,path,limit=65536):
  self.check();self.reads+=1
  value=self.reader(path,limit)
  if not isinstance(value,bytes):raise CgroupMemoryUnavailable('observation-reader-type')
  self.bytes+=len(value)
  if len(value)>limit:raise CgroupMemoryUnavailable('observation-file-bound')
  if self.bytes>self.bounds['bytes']:raise CgroupMemoryUnavailable('observation-byte-bound')
  self.check_time()
  try:return value.decode('utf-8')
  except UnicodeError:raise CgroupMemoryUnavailable('observation-encoding')
 def check_time(self):
  if self.clock()-self.started>self.bounds['seconds']:raise CgroupMemoryUnavailable('observation-time-bound')
 def link(self,path):
  self.check();self.reads+=1;value=self.linker(path);self.check_time()
  if not isinstance(value,str) or not re.fullmatch(r'(?:mnt|cgroup):\[[0-9]+\]',value):raise CgroupMemoryUnavailable('namespace-identity-invalid')
  return value

def cgroup_memory_locations(membership,mountinfo):
 # Keep CPU membership resolution: production rejects unresolved relevant mounts
 # globally before applying memory limits, even though this observer measures no CPU quota.
 groups={}
 for line in membership.splitlines():
  if not line:continue
  fields=line.split(':',2)
  if len(fields)!=3 or not fields[2].startswith('/') or os.path.normpath(fields[2])!=fields[2] or fields[2].startswith('//'):raise CgroupMemoryUnavailable('membership-invalid')
  if fields[1]=='':groups['2']=fields[2]
  else:
   for controller in fields[1].split(','):
    if controller in ('memory','cpu'):groups[controller]=fields[2]
 found=set();result=[]
 unescape=lambda value:re.sub(r'\\(040|011|012|134)',lambda match:{'040':' ','011':'\t','012':'\n','134':'\\'}[match[1]],value)
 for line in mountinfo.splitlines():
  parts=line.split(' - ',1)
  if len(parts)!=2:continue
  left,right=parts[0].split(),parts[1].split()
  if len(left)<6 or len(right)<3 or right[0] not in ('cgroup2','cgroup'):continue
  root,mount=unescape(left[3]),unescape(left[4])
  if not mount.startswith('/') or os.path.normpath(mount)!=mount or mount.startswith('//'):raise CgroupMemoryUnavailable('mount-invalid')
  version='2' if right[0]=='cgroup2' else '1'
  for controller in (['2'] if version=='2' else right[2].split(',')):
   if controller not in groups or controller in found:continue
   group=groups[controller]
   if group=='/' or group==root:relative='.'
   elif root=='/':relative=group[1:]
   elif group.startswith(root+'/'):relative=group[len(root)+1:]
   else:raise CgroupMemoryUnavailable('namespace-mount-mismatch')
   directory=os.path.normpath(os.path.join(mount,relative))
   if os.path.commonpath([mount,directory])!=mount:raise CgroupMemoryUnavailable('mount-escape')
   result.append((directory,mount,version,controller));found.add(controller)
 if set(groups)-found:raise CgroupMemoryUnavailable('controller-mount-unavailable')
 return result

def cgroup_memory_uint(raw,positive=False):
 text=raw.strip()
 if not re.fullmatch(r'\+?[0-9]+',text):raise CgroupMemoryUnavailable('counter-invalid')
 value=int(text)
 if value>=2**64 or positive and value==0:raise CgroupMemoryUnavailable('counter-invalid')
 return value

def cgroup_memory_observations(process_rows,physical,read=None,link=None,process_stat=None,clock=None,bounds=None):
 """Owned control-plane role candidates only; never infer admission or hidden ancestry."""
 budget=CgroupMemoryBudget(read or cgroup_memory_read,link or os.readlink,clock or time.monotonic,bounds or CGROUP_MEMORY_BOUNDS)
 targets=[row for row in process_rows if row.get('role')=='control-plane' and row.get('state')!='Z']
 output={'status':'not-observed' if not targets else 'observed','scope':'Visible hierarchy for live owned control-plane role candidates, bound to PID/start ticks; process-name role classification is not executable-hash verification.','observations':[],'issues':[]}
 if len(targets)>budget.bounds['targets']:output['issues'].append('control-plane-target-bound')
 def issue(row,code):
  if code not in row['issues']:row['issues'].append(code)
 def identity(pid,start):
  budget.check_time()
  if process_stat is not None:current=process_stat(pid)
  else:
   raw=budget.read('/proc/'+str(pid)+'/stat',4096);end=raw.rfind(')');fields=raw[end+2:].split()
   if end<0 or len(fields)<20:raise CgroupMemoryUnavailable('process-stat-invalid')
   current={'pid':int(raw.split(' ',1)[0]),'state':fields[0],'startTicks':int(fields[19])}
  budget.check_time()
  if current.get('pid')!=pid or current.get('startTicks')!=start or current.get('state')=='Z':raise CgroupMemoryUnavailable('process-identity-changed')
 def namespace(pid):
  value={}
  for kind in ('cgroup','mnt'):
   own=budget.link('/proc/self/ns/'+kind);target=budget.link('/proc/'+str(pid)+'/ns/'+kind)
   if own!=target:raise CgroupMemoryUnavailable(kind+'-namespace-differs')
   value[kind]=target
  return value
 for target in targets[:budget.bounds['targets']]:
  pid,start=target['pid'],target['startTicks']
  row={'pid':pid,'startTicks':start,'status':'observed','physicalTotalBytes':None,'physicalAvailableBytes':None,'productComparableEffectiveTotalBytes':None,'productComparableEffectiveAvailableBytes':None,'limitSource':'physical','nodes':[],'issues':[]}
  output['observations'].append(row);effective_valid=True
  try:
   identity(pid,start);namespaces=namespace(pid)
   proc='/proc/'+str(pid);membership=budget.read(proc+'/cgroup');mounts=budget.read(proc+'/mountinfo',131072)
   row['membershipSHA256']=hashlib.sha256(membership.encode()).hexdigest();row['mountTopologySHA256']=hashlib.sha256(mounts.encode()).hexdigest()
   row['namespaceIdentitySHA256']=hashlib.sha256(json.dumps(namespaces,sort_keys=True).encode()).hexdigest()
   locations=cgroup_memory_locations(membership,mounts)
   total=(physical or {}).get('MemTotal');available=(physical or {}).get('MemAvailable')
   if type(total) is not int or type(available) is not int or total<=0 or total>=2**64 or available<0 or available>total:
    effective_valid=False;issue(row,'physical-memory-unavailable')
   else:row.update(physicalTotalBytes=total,physicalAvailableBytes=available,productComparableEffectiveTotalBytes=total,productComparableEffectiveAvailableBytes=available)
   memory_locations=[value for value in locations if value[2]=='2' or value[3]=='memory']
   if not memory_locations:issue(row,'memory-controller-not-present')
   for hierarchy,(directory,mount,version,controller) in enumerate(memory_locations):
    current=directory
    for depth in range(budget.bounds['hierarchyLevels']):
     budget.check_time();budget.nodes+=1
     if budget.nodes>budget.bounds['nodeRows']:raise CgroupMemoryUnavailable('observation-node-bound')
     node={'hierarchy':hierarchy,'depth':depth,'version':version,'locationSHA256':hashlib.sha256((version+'\0'+current).encode()).hexdigest(),'mountRoot':current==mount,'limitStatus':'unavailable','limitBytes':None,'currentStatus':'unavailable','currentBytes':None,'statStatus':'unavailable','stat':None}
     row['nodes'].append(node);finite=False
     max_file,current_file=('memory.max','memory.current') if version=='2' else ('memory.limit_in_bytes','memory.usage_in_bytes')
     try:
      raw=budget.read(os.path.join(current,max_file))
      if version=='2' and raw.strip()=='max':node['limitStatus']='unlimited'
      else:node['limitBytes']=cgroup_memory_uint(raw,True);node['limitStatus']='finite';finite=True
     except FileNotFoundError:
      node['limitStatus']='missing';issue(row,'limit-file-missing-product-skips-level')
     except (OSError,CgroupMemoryUnavailable) as error:
      if isinstance(error,CgroupMemoryUnavailable) and str(error).startswith('observation-'):raise
      effective_valid=False;issue(row,'limit-unavailable')
     try:node['currentBytes']=cgroup_memory_uint(budget.read(os.path.join(current,current_file)));node['currentStatus']='observed'
     except (OSError,CgroupMemoryUnavailable) as error:
      if isinstance(error,CgroupMemoryUnavailable) and str(error).startswith('observation-'):raise
      issue(row,'current-unavailable')
      if finite:effective_valid=False
     if finite:
      row['limitSource']='physical-and-control-plane-cgroup-v'+version
      if node['currentBytes'] is not None:
       node['limitMinusCurrentBytes']=max(0,node['limitBytes']-node['currentBytes'])
       if effective_valid:
        row['productComparableEffectiveTotalBytes']=min(row['productComparableEffectiveTotalBytes'],node['limitBytes'])
        row['productComparableEffectiveAvailableBytes']=min(row['productComparableEffectiveAvailableBytes'],node['limitMinusCurrentBytes'])
     try:
      counters={};raw=budget.read(os.path.join(current,'memory.stat'))
      for line in raw.splitlines():
       fields=line.split()
       if fields and fields[0] in CGROUP_MEMORY_STAT_KEYS:
        if len(fields)!=2 or fields[0] in counters:raise CgroupMemoryUnavailable('stat-invalid')
        counters[fields[0]]=cgroup_memory_uint(fields[1])
      if not counters:raise CgroupMemoryUnavailable('stat-empty')
      node['stat']=counters;node['statStatus']='observed'
     except (OSError,CgroupMemoryUnavailable) as error:
      if isinstance(error,CgroupMemoryUnavailable) and str(error).startswith('observation-'):raise
      issue(row,'stat-unavailable')
     if current==mount:break
     parent=os.path.dirname(current)
     if parent==current or os.path.commonpath([mount,parent])!=mount:raise CgroupMemoryUnavailable('hierarchy-mount-escape')
     current=parent
    else:raise CgroupMemoryUnavailable('hierarchy-depth-bound')
   identity(pid,start)
   if namespace(pid)!=namespaces or budget.read(proc+'/cgroup')!=membership or budget.read(proc+'/mountinfo',131072)!=mounts:raise CgroupMemoryUnavailable('process-topology-changed')
   identity(pid,start);row['identityAndTopologyStable']=True
  except (OSError,ValueError,IndexError,CgroupMemoryUnavailable) as error:
   effective_valid=False;row['identityAndTopologyStable']=False
   issue(row,str(error) if isinstance(error,CgroupMemoryUnavailable) else 'process-or-topology-unavailable')
  if not effective_valid:row['productComparableEffectiveTotalBytes']=None;row['productComparableEffectiveAvailableBytes']=None
  if row['issues']:row['status']='partial' if row.get('identityAndTopologyStable') else 'unavailable'
 if output['issues'] or any(row['issues'] for row in output['observations']):output['status']='partial'
 output['readCount']=budget.reads;output['readBytes']=budget.bytes;output['durationSeconds']=max(0,budget.clock()-budget.started)
 if len(json.dumps(output,separators=(',',':')).encode())>budget.bounds['outputBytes']:
  output.update(status='unavailable',observations=[],issues=['observation-output-bound'])
 return output

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
 metadata={'controlPlaneCgroupMemory':{'version':1,'bounds':CGROUP_MEMORY_BOUNDS,'productBasisSHA256':CGROUP_MEMORY_PRODUCT_BASIS,'formula':'Independent minima of physical total and finite visible limits; physical available and max(0, limit-current) at every visible ancestor. No memory.stat, RSS, swap, high limit or scheduling reservation subtraction.','scope':'Same observer/target cgroup and mount namespaces; only already-owned live control-plane role candidates. Raw membership and mount paths are hash-only; missing values stay explicit.','selfCost':'Additional sampler reads/CPU are included in wrapper RUSAGE_SELF. Shared cgroup usage may include observer work; self-cost is not subtracted from memory counters.','limitations':['Sequential sampled counters are not atomic and do not reproduce any prior admission decision.','Visibility ends at the process-visible mount root; hidden ancestors are not inferred.','Missing limit files are recorded as missing and skipped only to match product semantics; they are not claimed unlimited.','Role follows existing process-name classification after descendant/PID-start ownership proof, not executable-byte verification.']},'startedAt':now(),'driver':{'pid':a.driver_pid,'startTicks':a.driver_start_ticks},'intervalSeconds':1,'baseline':'post-start; no pre-run idle baseline observed','imageId':a.image_id,'includeChecks':a.include_checks,'instanceIdentitySHA256':None,'scope':('Exact driver descendants and managed workers; opted-in checks additionally require exact instance/image/protocol/engine/allocation identity and fresh stats bounded by current PID/start/cgroup checks.' if a.include_checks else 'Exact driver descendants by PID/starttime, plus exact instance/managed/image-verified Docker containers. No command lines, environments, mount paths or provider payloads recorded.'),'limitations':['Sum of process RSS double-counts shared pages; it is not unique physical memory.','Container cgroup memory is separate and must not be added to container process RSS.','Docker daemon/shared kernel/VM overhead is unassigned; host memory includes unrelated work.','Processes starting and exiting between 1 Hz samples may be missed; delta counters cover observed intervals only.','Post-driver-exit settlement is not an application idle baseline.','No UI input latency, other-app responsiveness or model reasoning quality is measured.'],'helperSHA256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
 if not a.include_checks:metadata.pop('includeChecks')
 (a.output/'metadata.json').write_text(json.dumps(metadata,indent=2)+'\n');os.chmod(a.output/'metadata.json',0o600)
 (a.output/'accepted-sampler.py').write_bytes(pathlib.Path(__file__).read_bytes());os.chmod(a.output/'accepted-sampler.py',0o600)
 with (a.output/'samples.jsonl').open('x') as stream:
  os.chmod(a.output/'samples.jsonl',0o600)
  while time.monotonic()-started<a.seconds:
   tick=time.monotonic();proc,unreadable=processes();owned,issues=containers();living=any(p['pid']==a.driver_pid and p['startTicks']==a.driver_start_ticks and p['state']!='Z' for p in proc)
   if not living and driver_exit is None:driver_exit=time.monotonic()
   host_sample=host();cgroup_memory=cgroup_memory_observations(proc,host_sample.get('memory'))
   row={'observedAt':now(),'elapsedSeconds':time.monotonic()-started,'phase':'post-start-running' if driver_exit is None else 'post-driver-exit-settle','driverAlive':living,'host':host_sample,'controlPlaneCgroupMemory':cgroup_memory,'processes':proc,'containers':owned,'containerIssues':issues,'containerMeasurementStatus':'partial' if issues else 'observed','processStatUnreadableOrRacedCount':unreadable,'sampleDurationSeconds':time.monotonic()-tick}
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
