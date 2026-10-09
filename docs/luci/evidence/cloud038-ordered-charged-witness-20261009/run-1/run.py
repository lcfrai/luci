import pathlib,subprocess,json,hashlib,datetime,time,os,signal
repo=pathlib.Path('/workspace/luci'); out=pathlib.Path(__file__).parent
stamp=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
sha=lambda b:hashlib.sha256(b).hexdigest()
def capture(name):
    paths=subprocess.check_output(['git','ls-files','-z'],cwd=repo).decode().split('\0')
    chosen=[p for p in paths if p.endswith('.go') or pathlib.PurePosixPath(p).name in ('go.mod','go.sum')]
    rows=[]
    for p in chosen:
        b=(repo/p).read_bytes();rows.append({'path':p,'bytes':len(b),'sha256':sha(b)})
    for p in [pathlib.Path('/workspace/.luci-tools/go.work'),out/'formatted-draft.go',out/'overlay.json']:
        b=p.read_bytes(); rows.append({'path':str(p),'bytes':len(b),'sha256':sha(b),'external':True})
    receipt={'at':stamp(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'status':subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True),'scope':'All tracked repository .go and go.mod/go.sum files plus exact external workspace, additive draft and overlay. Selected source binding; NOT complete compiled inputs, installed module bytes, standard library, generated source or binary provenance.','inputs':rows}
    (out/name).write_text(json.dumps(receipt,indent=2)+'\n');return receipt
before=capture('source-before.json')
assert before['status']==''
command=['go','test','-race','-p','1','-count=1','-timeout=170s','-json','-overlay='+str(out/'overlay.json'),'-run','^TestProjectCheckOriginalReviewPreservesSeparatePendingWorkspace$','./internal/api']
env=os.environ.copy();env['GOMAXPROCS']='2';env['GOFLAGS']='-mod=readonly'
record={'startedAt':stamp(),'cwd':str(repo/'control-plane'),'command':command,'env':{'GOMAXPROCS':env['GOMAXPROCS'],'GOFLAGS':env['GOFLAGS'],'GOWORK':env.get('GOWORK'),'GOTOOLCHAIN':env.get('GOTOOLCHAIN')},'goVersion':subprocess.check_output(['go','version'],env=env,text=True).strip(),'outerDeadlineSeconds':180,'goTestTimeoutSeconds':170,'scope':'Single controlled fixture regression; no application/browser/Docker/provider execution.','forced':False}
(out/'command.json').write_text(json.dumps(record,indent=2)+'\n');start=time.monotonic()
with (out/'test.jsonl').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
    proc=subprocess.Popen(command,cwd=repo/'control-plane',env=env,stdout=stdout,stderr=stderr,start_new_session=True)
    record['pid']=proc.pid
    try:record['exitCode']=proc.wait(timeout=180)
    except subprocess.TimeoutExpired:
        record['forced']=True;os.killpg(proc.pid,signal.SIGKILL);record['exitCode']=proc.wait(timeout=5)
record.update(finishedAt=stamp(),elapsedSeconds=time.monotonic()-start)
after=capture('source-after.json');record['sourceInputsUnchanged']=before['inputs']==after['inputs'];record['headUnchanged']=before['head']==after['head'];record['workingTreeCleanAfter']=after['status']==''
rows=[]
for line in (out/'test.jsonl').read_text().splitlines():
    try:row=json.loads(line)
    except json.JSONDecodeError:continue
    if row.get('Action') in ('pass','fail','skip') and row.get('Test'): rows.append({k:row[k] for k in ['Action','Test','Elapsed'] if k in row})
record['testOutcomes']=rows;record['inputsCount']=len(before['inputs']);record['artifacts']=[{'path':p.name,'sha256':sha(p.read_bytes()),'bytes':p.stat().st_size} for p in sorted(out.iterdir()) if p.is_file() and p.name!='result.json']
(out/'result.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record,indent=2))
