#!/usr/bin/env python3
"""Read-only bounded closure of exact known identities; no signals or mutations."""
from pathlib import Path
import argparse,datetime,hashlib,json,re,subprocess
sha=lambda data:hashlib.sha256(data).hexdigest()
def read(path,limit=64*1024**2):
 assert path.is_file() and not path.is_symlink() and path.stat().st_size<=limit
 return path.read_bytes()
def identity(pid,start):
 try:
  fields=Path('/proc',str(pid),'stat').read_text().rsplit(') ',1)[1].split();current={'pid':pid,'startTicks':int(fields[19]),'state':fields[0]}
  status='pid-reused' if current['startTicks']!=start else 'zombie' if current['state']=='Z' else 'same-identity-present'
  return {'expected':{'pid':pid,'startTicks':start},'observation':status,'current':current}
 except FileNotFoundError:return {'expected':{'pid':pid,'startTicks':start},'observation':'absent'}
 except (OSError,ValueError,IndexError):return {'expected':{'pid':pid,'startTicks':start},'observation':'unknown'}
def command(args):
 try:
  value=subprocess.run(args,capture_output=True,timeout=8)
  return {'command':args[:3],'exitCode':value.returncode,'stdoutSHA256':sha(value.stdout),'stderrSHA256':sha(value.stderr),'text':value.stdout.decode(errors='replace').strip() if value.returncode==0 and len(value.stdout)<65536 else None}
 except subprocess.TimeoutExpired:return {'command':args[:3],'timedOut':True,'text':None}
def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('run',type=Path);parser.add_argument('output',type=Path);args=parser.parse_args();assert not args.output.exists()
 files=[p for p in args.run.iterdir() if p.is_file() and p.suffix in ['.json','.log']]
 files += [p for p in (args.run/'driver').iterdir() if p.is_file() and p.suffix in ['.json','.jsonl','.zip']]
 files += [p for p in (args.run/'driver/accepted-harness').iterdir() if p.is_file()]
 assert len(files)<=64 and sum(p.stat().st_size for p in files)<=128*1024**2
 before={str(p.relative_to(args.run)):sha(read(p)) for p in sorted(files)}
 result=json.loads(read(args.run/'driver/result.json'));invocation=json.loads(read(args.run/'invocation.json'));wrapper=json.loads(read(args.run/'wrapper-result.json'));driver=json.loads(read(args.run/'driver-identity.json'))
 assert result['sourceCommit']==invocation['productSource'] and result['harnessCheckout']['commit']==invocation['helperSource']
 known={(driver['pid'],driver['startTicks'])}
 journal=read(args.run/'driver/diagnostic-journal.jsonl',4*1024**2).splitlines();assert len(journal)<=20000
 for line in journal:
  row=json.loads(line)
  if row.get('event') in ['driver-start','controller-start'] and isinstance(row.get('pid'),int) and isinstance(row.get('startTicks'),int):known.add((row['pid'],row['startTicks']))
 assert len(known)<=16
 observations=[identity(pid,start) for pid,start in sorted(known)]
 docker={};container=command(['docker','ps','-aq','--no-trunc']);volume=command(['docker','volume','ls','-q']);engine=command(['docker','info','--format','{{.ID}}']);imageid=(result.get('image') or {}).get('id');assert re.fullmatch('sha256:[a-f0-9]{64}',imageid)
 image=command(['docker','image','inspect',imageid,'--format','{{.Id}}'])
 for key,value in [('containers',container),('volumes',volume)]:
  text=value.pop('text');docker[key]={**value,'count':len(text.splitlines()) if text is not None else None,'empty':text=='' if text is not None else None}
 etext=engine.pop('text');itext=image.pop('text');docker['engine']={**engine,'identitySHA256':sha(etext.encode()) if etext else None,'matchesInvocation':sha(etext.encode())==invocation.get('engineIdentitySHA256') if etext else None};docker['image']={**image,'expectedID':imageid,'matches':itext==imageid if itext else None}
 after={str(p.relative_to(args.run)):sha(read(p)) for p in sorted(files)}
 observer=result.get('autonomousReadiness') or {};last=(observer.get('windows') or [{}])[-1]
 out={'observedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Independent read-only exact driver/controller identities and exclusive daemon inventory. No process environment scan, browser PID census, signals, cleanup changes or private fixture access.','runProductSource':result['sourceCommit'],'runHelperSource':result['harnessCheckout']['commit'],'originalResultSHA256':before['driver/result.json'],'originalArtifacts':before,'allOriginalArtifactsStable':before==after,'identities':observations,'docker':docker,'wrapper':{key:wrapper.get(key) for key in ['exitCode','externalMonitorTimeout','forcedExactDriver','cleanupUnconfirmedByWrapper']},'driverAssertions':{key:result.get(key) for key in ['controllersStopped','dockerRequestsDrained','dockerMutationStateCertain','fixtureRemoved']},'driverCleanupCount':len(result.get('cleanup',[])),'driverResourceAssertions':result.get('runtimeCleanup'),'observerAssertions':{'status':observer.get('status'),'closed':last.get('closed'),'activeGeneration':last.get('activeGeneration'),'overflow':last.get('overflow'),'lateCallbacks':last.get('lateCallbacks')},'limits':['Current repository is not used as run identity; root may integrate new source after the run.','Only exact journal-recorded driver/controller processes were independently inspected. No browser PID census, whole-machine absence or historical zombie attribution.','Browser and private fixture removal remain retained driver assertions, not independent private-path inspection.','Container/volume emptiness and engine/image identity describe this read-only instant. No package/archive/image payload rehash here.']}
 args.output.parent.mkdir(parents=True,exist_ok=True);args.output.write_text(json.dumps(out,indent=2)+'\n');print(json.dumps({'identities':[(x['expected']['pid'],x['observation']) for x in observations],'containers':docker['containers']['count'],'volumes':docker['volumes']['count'],'imageMatches':docker['image']['matches'],'engineMatches':docker['engine']['matchesInvocation'],'originalsStable':out['allOriginalArtifactsStable']}))
if __name__=='__main__':main()
