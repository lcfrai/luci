import hashlib,json,os,pathlib,subprocess,sys,time
ROOT=pathlib.Path('/workspace/luci'); BASE=pathlib.Path('/workspace/.luci-tools/cloud080-preview-cancel')
mode=sys.argv[1]; out=BASE/mode;out.mkdir(exist_ok=False)
overlay={}
if mode=='before':
 for p in (BASE/'original/control-plane/internal/product').glob('*.go'):
  overlay[str(ROOT/'control-plane/internal/product'/p.name)]=str(p)
 overlay[str(ROOT/'control-plane/internal/product/preview_cancel.go')]=''
 overlay[str(ROOT/'control-plane/internal/product/preview_cancel_regression_test.go')]=str(BASE/'tests/preview_cancel_regression_test.go')
 (out/'overlay.json').write_text(json.dumps({'Replace':overlay},indent=2)+'\n')
def inputs():
 rows=[]
 paths=set(p for p in (ROOT/'control-plane').rglob('*') if p.is_file() and (p.suffix=='.go' or p.name in ('go.mod','go.sum')))
 paths.update(pathlib.Path(p) for p in overlay)
 for p in sorted(paths):
  selected=overlay.get(str(p),str(p))
  if not selected:continue
  b=pathlib.Path(selected).read_bytes();rows.append({'path':str(p.relative_to(ROOT)),'selected':selected,'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)})
 for p in [ROOT/'go.work',ROOT/'go.work.sum',pathlib.Path('/workspace/.luci-tools/go.work')]:
  if p.exists():
   b=p.read_bytes();rows.append({'path':str(p),'selected':str(p),'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)})
 return rows
before=inputs();(out/'inputs-before.json').write_text(json.dumps(before,indent=2)+'\n')
cmd=['go','test','-json','-count=1','-p=1','-timeout=100s']
if mode=='before':cmd+=['-overlay='+str(out/'overlay.json'),'-run=TestPreview(CancelLiveRouteRetainsUnknownAndDoesNotReplay|CloseCancelsOwnedBuildAndJoinsFinalPublication)$']
elif mode in ('reopen-before','reopen-fixed'):cmd+=['-race','-run=TestPreviewCancellationReopenDoesNotLeaveInactiveNoDemandCancelling$']
elif mode=='authority-before':cmd+=['-race','-run=TestPreviewCancellation(FreshGrantAfterMutexWaitWithoutContextRevocation|RequiresCurrentAuthorityRevisionAndOwnedIdentity/changed-allocation)$']
else:cmd+=['-race','-run=Test(Preview|LocalRelease|StopRelease|RuntimeInstallerRequestDisconnectDoesNotCancelButCloseJoinsAndPersists)']
cmd+=['./internal/product']
identity={'argv':cmd,'cwd':str(ROOT/'control-plane'),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'go':subprocess.check_output(['go','version'],text=True).strip(),'env':{k:os.environ.get(k) for k in ('GOMAXPROCS','GOCACHE','GOMODCACHE','GOTMPDIR','GOWORK','GOFLAGS')},'start':time.time()}
(out/'invocation.json').write_text(json.dumps(identity,indent=2)+'\n')
with (out/'go-test.jsonl').open('w') as log:
 try: code=subprocess.run(cmd,cwd=ROOT/'control-plane',stdout=log,stderr=subprocess.STDOUT,timeout=150).returncode
 except subprocess.TimeoutExpired:code=124
after=inputs();(out/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n')
events=[]
for l in (out/'go-test.jsonl').read_text().splitlines():
 try:v=json.loads(l)
 except ValueError:continue
 if v.get('Test') and v.get('Action') in ('pass','fail','skip'):events.append({k:v[k] for k in ('Action','Test','Elapsed') if k in v})
result={'exit':code,'inputsStable':before==after,'inputCount':len(before),'outcomes':events,'end':time.time()}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result));sys.exit(code)
