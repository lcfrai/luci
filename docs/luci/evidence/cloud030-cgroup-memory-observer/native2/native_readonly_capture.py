#!/usr/bin/env python3
"""Three tiny native observer-process reads; no application, sockets or Docker."""
import datetime, hashlib, importlib.util, json, os, pathlib, resource, sys, time
base=pathlib.Path(__file__).resolve().parent
out=pathlib.Path(sys.argv[1]).resolve() if len(sys.argv)==2 else base/'evidence/native2';out.mkdir(mode=0o700)
source=base/'staged/sample-linux-owned.py'
spec=importlib.util.spec_from_file_location('native_cgroup_probe',source)
sampler=importlib.util.module_from_spec(spec);spec.loader.exec_module(sampler)
sampler.PAGE=os.sysconf('SC_PAGE_SIZE')
pid=os.getpid();identity=sampler.stat(pid)
receipt={'version':1,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Read-only native kernel/cgroup observations of this Python fixture process, manually supplied as a control-plane-role candidate to exercise the observer. This is not an actual Luci process, installed workflow or admission decision.','sourceSHA256':hashlib.sha256(source.read_bytes()).hexdigest(),'driverSHA256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),'target':{'pid':pid,'startTicks':identity['startTicks'],'actualRole':'native-observer-fixture','suppliedRole':'control-plane'},'samples':[],'errors':[]}
try:
 for index in range(3):
  physical=sampler.host()['memory']
  observed=sampler.cgroup_memory_observations([dict(identity,role='control-plane')],physical)
  receipt['samples'].append({'observedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'physical':physical,'controlPlaneCgroupMemory':observed})
  if index<2:time.sleep(.25)
 assert hashlib.sha256(source.read_bytes()).hexdigest()==receipt['sourceSHA256']
 receipt['captureCompleted']=True
except Exception as error:
 receipt['errors'].append(type(error).__name__);receipt['captureCompleted']=False
finally:
 usage=resource.getrusage(resource.RUSAGE_SELF);hz=os.sysconf('SC_CLK_TCK');lifetime=time.clock_gettime(time.CLOCK_BOOTTIME)-identity['startTicks']/hz
 receipt['finishedAt']=datetime.datetime.now(datetime.timezone.utc).isoformat()
 receipt['selfCost']={'scope':'This interpreter RUSAGE_SELF including imports and capture script; no child processes, Docker or application workload. Linux peak RSS includes shared pages. This is not per-sample incremental overhead and is not subtracted from shared cgroup usage.','wallSeconds':lifetime,'userCPUSeconds':usage.ru_utime,'systemCPUSeconds':usage.ru_stime,'maxRSSBytes':usage.ru_maxrss*1024,'inputBlockOperations':usage.ru_inblock,'outputBlockOperations':usage.ru_oublock}
 (out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n');os.chmod(out/'result.json',0o600)
 (out/'accepted-sampler.py').write_bytes(source.read_bytes())
 (out/'native_readonly_capture.py').write_bytes(pathlib.Path(__file__).read_bytes())
 print(json.dumps({'captureCompleted':receipt['captureCompleted'],'samples':len(receipt['samples']),'statuses':[sample['controlPlaneCgroupMemory']['status'] for sample in receipt['samples']],'selfCost':receipt['selfCost']}))
 if not receipt['captureCompleted']:sys.exit(1)
