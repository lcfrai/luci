import os,json,pathlib,subprocess,sys,time,hashlib
root=pathlib.Path(__file__).parent
mode=sys.argv[1]
out=root/'receipts'/mode
out.mkdir(parents=True,exist_ok=True)
env=os.environ.copy()
env.update(GOMAXPROCS='2',GOMODCACHE='/tmp/luci-retained-tools/go-mod',GOPATH='/tmp/luci-gopath',GOCACHE='/tmp/luci-retained-tools/go-build',GOTMPDIR='/workspace/.t',TMPDIR='/workspace/.t',GOTOOLCHAIN='local',GOWORK='/workspace/.luci-tools/go.work',GOFLAGS='-mod=readonly')
for k in ['GOCACHE','GOTMPDIR','TMPDIR']:pathlib.Path(env[k]).mkdir(parents=True,exist_ok=True)
args=['/workspace/.luci-tools/go1.26.3/bin/go','test','-p','2','-count=1','-timeout=90s','-json']
if mode=='before':args += ['-overlay='+str(root/'before-overlay.json'),'-run','^TestAttachTypedExitPublishesCurrentRoutesWithoutReaper$','./internal/api']
elif mode=='related':args += ['-race','-overlay='+str(root/'overlay.json'),'-run','Test(StoppedTaskCleanup|ObservedExit|ConfirmedStoppedTask|TaskCleanupKeeps|TaskCompletionObserver|ProjectRunChildObservation|SocketTicketsAreSingleUsePathBoundExpiringAndRevocable|TeamProxyResolvesSessionWorktreeAndResumeSourceGrants|TeamProxyDropsBufferedSessionResultAfterRevocation)','./internal/api','./internal/reaper','./internal/management']
else:args += ['-race','-overlay='+str(root/'overlay.json'),'-run','Test(Attach|Relay|NewActuallyRoutesTheAttachRoute|ExecutionAdmissionReleasesConfirmedSessionBeforeRegistryForget|ExecutionTask|SessionExecutionGeneration)','./internal/proxy','./internal/api']
receipt={'argv':args,'cwd':'/workspace/luci/control-plane','sourceCommit':subprocess.check_output(['git','-C','/workspace/luci','rev-parse','HEAD'],text=True).strip(),'startedAt':time.time()}
(out/'command.json').write_text(json.dumps(receipt,indent=2))
with (out/'events.jsonl').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 p=subprocess.run(args,cwd=receipt['cwd'],env=env,stdout=stdout,stderr=stderr)
receipt.update(exitCode=p.returncode,endedAt=time.time())
counts={};failed=[]
for line in (out/'events.jsonl').read_text().splitlines():
 try:x=json.loads(line)
 except ValueError:continue
 if 'Test' in x and x.get('Action') in ('pass','fail','skip'):
  counts[x['Action']]=counts.get(x['Action'],0)+1
  if x['Action']=='fail':failed.append(x['Test'])
receipt.update(testActions=counts,failedTests=failed,eventsSHA256=hashlib.sha256((out/'events.jsonl').read_bytes()).hexdigest())
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps({'mode':mode,'exitCode':p.returncode,'testActions':counts,'failedTests':failed}))
sys.exit(p.returncode)
