import datetime, hashlib, json, os, pathlib, subprocess, sys, time

root = pathlib.Path('/workspace/luci')
stage = pathlib.Path('/workspace/.luci-tools/check-history-liveness')
run = stage / sys.argv[1]
run.mkdir()
def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()
def snapshot():
    paths = [p for p in (root/'control-plane').rglob('*') if p.is_file() and (p.suffix == '.go' or p.name in ('go.mod', 'go.sum') or ('testdata' in p.parts and p.suffix == '.json'))]
    paths += [pathlib.Path('/workspace/.luci-tools/go.work')]
    return {'head': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip(), 'inputs': {str(p):sha(p) for p in sorted(paths)}, 'overlayInputs': {str(p):sha(p) for p in sorted((stage/'tree').rglob('*.go'))}}
mapping = {str(root/p.relative_to(stage/'tree')):str(p) for p in (stage/'tree').rglob('*.go')}
overlay = run/'overlay.json'
overlay.write_text(json.dumps({'Replace':mapping}, indent=2)+'\n')
before = snapshot()
(run/'before.json').write_text(json.dumps(before, indent=2)+'\n')
selection = '^TestProjectCheck(HTTPSettledUnknownHistoryAllowsNextCheck|History)' if sys.argv[1] == 'unfixed' else '^Test(ProjectCheck|ProjectRunCheck|ProjectEvidence)'
command = ['/workspace/.luci-tools/go1.26.3/bin/go','test','-race','-p','1','-count','1','-timeout=5m','-json','-overlay='+str(overlay),'-run',selection,'./internal/api']
command += ['./internal/product']
env=os.environ.copy()
env['GOMAXPROCS']='2'
env['LUCI_CHECK_HTTP_CAPTURE']=str(run/'project-check-http.json')
env['LUCI_CHECK_HTTP_EXPECTED']=str(root/'control-plane/internal/product/testdata/project-check-http.json')
(run/'command.json').write_text(json.dumps({'command':command, 'cwd':str(root/'control-plane'), 'env':{k:env[k] for k in ('GOMAXPROCS','LUCI_CHECK_HTTP_CAPTURE','LUCI_CHECK_HTTP_EXPECTED')}, 'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(), 'scope':'Controlled API/product race regression; no Docker or installed execution'}, indent=2)+'\n')
started=time.monotonic()
with (run/'events.jsonl').open('w') as out, (run/'stderr.log').open('w') as err:
    proc=subprocess.Popen(command, cwd=root/'control-plane', env=env, stdout=out, stderr=err)
    (run/'pid').write_text(str(proc.pid)+'\n')
    code=proc.wait()
after=snapshot()
(run/'after.json').write_text(json.dumps(after, indent=2)+'\n')
counts={'pass':0,'fail':0,'skip':0}; failures=[]; packages=[]
for line in (run/'events.jsonl').read_text().splitlines():
    try: row=json.loads(line)
    except json.JSONDecodeError: continue
    action=row.get('Action')
    if row.get('Test') and action in counts:
        counts[action]+=1
        if action=='fail': failures.append(row['Test'])
    elif not row.get('Test') and action in counts:
        packages.append({'package':row['Package'],'action':action,'elapsed':row.get('Elapsed')})
report={'exitCode':code,'counts':counts,'failures':failures,'packages':packages,'wallSeconds':round(time.monotonic()-started,3),'inputsUnchanged':before==after,'inputCount':len(before['inputs']),'overlayInputCount':len(before['overlayInputs']),'headBefore':before['head'],'headAfter':after['head']}
capture=run/'project-check-http.json'
if capture.exists(): report['goldenExactBytesMatch']=capture.read_bytes()==(root/'control-plane/internal/product/testdata/project-check-http.json').read_bytes()
(run/'report.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
sys.exit(code)
