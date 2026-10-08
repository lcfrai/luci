import os,json,hashlib,subprocess,time,pathlib,datetime
root=pathlib.Path('/workspace/.luci-tools/cloud033-cleanup-message'); repo=pathlib.Path('/workspace/luci'); out=root/'go-evidence-final'; out.mkdir(exist_ok=True)
env=os.environ.copy(); env.update(GOMAXPROCS='2',GOCACHE='/tmp/luci-cloud033-go-cache')
def stamp(): return datetime.datetime.now(datetime.timezone.utc).isoformat()
def record(path):
 data=path.read_bytes(); return {'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data)}
def bind(mode):
 mapping=json.loads((root/('overlay-'+mode+'.json')).read_text())['Replace']
 tracked=subprocess.check_output(['git','ls-files','control-plane','sandboxd'],cwd=repo,text=True).splitlines()
 selected={p for p in tracked if pathlib.Path(p).suffix=='.go' or pathlib.Path(p).name in ('go.mod','go.sum','go.work','go.work.sum') or '/testdata/' in p or p.startswith('control-plane/internal/platform/supabase_assets/')}
 selected.update(str(pathlib.Path(p).relative_to(repo)) for p in mapping)
 files=[]
 for relative in sorted(selected):
  source=pathlib.Path(mapping.get(str(repo/relative),str(repo/relative)))
  files.append({'path':relative,'input':str(source),**record(source)})
 external=[]
 for path in [pathlib.Path('/workspace/.luci-tools/go.work'),pathlib.Path('/workspace/.luci-tools/go.work.sum')]:
  if path.exists(): external.append({'path':str(path),**record(path)})
 return {'capturedAt':stamp(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'scope':'All tracked Go/module files, testdata and the complete sole go:embed asset tree in both active workspace modules, including overlays; external workspace files separately bound. This is a conservative repository-input superset, not an external module-cache byte attestation.','files':files,'externalWorkspaceFiles':external,'overlay':mapping}
versions={'go':subprocess.check_output(['go','version'],env=env,text=True).strip(),'env':json.loads(subprocess.check_output(['go','env','-json','GOOS','GOARCH','GOROOT','GOWORK','GOMODCACHE','GOCACHE'],cwd=repo/'control-plane',env=env,text=True))}
(out/'versions.json').write_text(json.dumps(versions,indent=2)+'\n')
new='^TestProjectCheckHTTP(CleanupConfirmationReplacesPendingMessage|FinishedLegacyCleanupMessagePreservesDigest|CleanupConfirmationPreservesOtherDiagnoses)$'
affected='^TestProjectCheckHTTP(AccountsSharePendingCapacityBeforeCreate|CleanupFailureKeepsOutputAcrossReopenWithoutReplay|FinishedReceiptSurvivesFinalPublicationBoundary|FinishedObservationPreservesLaterCommandOutcome|UnacknowledgedCreateRequiresReviewedFenceAndFreshEditor|RechecksRevokedAuthorityBeforeCreate)$'
for name,mode,pattern in [('unfixed','before',new),('fixed','after',new),('affected','after',affected)]:
 before=bind(mode); (out/(name+'-source-before.json')).write_text(json.dumps(before,indent=2)+'\n')
 cmd=['go','test','-race','-p=1','-count=1','-timeout=8m','-json','-overlay='+str(root/('overlay-'+mode+'.json')),'./internal/api','-run',pattern]
 receipt={'startedAt':stamp(),'command':cmd,'cwd':str(repo/'control-plane'),'GOMAXPROCS':env['GOMAXPROCS'],'GOCACHE':env['GOCACHE']}
 start=time.monotonic()
 with (out/(name+'.jsonl')).open('w') as stdout, (out/(name+'.stderr')).open('w') as stderr:
  try: process=subprocess.run(cmd,cwd=repo/'control-plane',env=env,stdout=stdout,stderr=stderr,timeout=720); receipt['exitCode']=process.returncode
  except subprocess.TimeoutExpired: receipt['outerTimedOut']=True
 receipt.update(finishedAt=stamp(),elapsedSeconds=time.monotonic()-start)
 after=bind(mode); (out/(name+'-source-after.json')).write_text(json.dumps(after,indent=2)+'\n')
 receipt['inputsUnchanged']=before['files']==after['files'] and before['externalWorkspaceFiles']==after['externalWorkspaceFiles']
 outcomes={k:[] for k in ['pass','fail','skip']}
 for line in (out/(name+'.jsonl')).read_text().splitlines():
  try: event=json.loads(line)
  except json.JSONDecodeError: continue
  if event.get('Test') and event.get('Action') in outcomes: outcomes[event['Action']].append(event['Test'])
 receipt['testOutcomes']=outcomes
 (out/(name+'-result.json')).write_text(json.dumps(receipt,indent=2)+'\n')
 print(json.dumps({'name':name,'exitCode':receipt.get('exitCode'),'elapsedSeconds':receipt['elapsedSeconds'],'inputsUnchanged':receipt['inputsUnchanged'],'counts':{k:len(v) for k,v in outcomes.items()}}),flush=True)
 if receipt.get('outerTimedOut') or (name!='unfixed' and receipt.get('exitCode')!=0): break
size=sum(p.stat().st_size for p in pathlib.Path(env['GOCACHE']).rglob('*') if p.is_file())
(out/'cache-size.json').write_text(json.dumps({'path':env['GOCACHE'],'bytes':size,'recordedAt':stamp(),'scope':'Dedicated task-owned recomputable Go build cache; not removed.'},indent=2)+'\n')
print(json.dumps({'cacheBytes':size}),flush=True)
