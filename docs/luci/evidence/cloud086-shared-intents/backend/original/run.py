import os,json,hashlib,pathlib,subprocess,time
root=pathlib.Path('/workspace/luci'); out=pathlib.Path('/workspace/.luci-tools/cloud086-coordination/evidence/original')
overlay=json.loads((out/'overlay.json').read_text())['Replace']
files=subprocess.check_output(['rg','--files','-g','*.go','-g','go.mod','-g','go.sum','-g','go.work','-g','go.work.sum',str(root/'control-plane')],text=True).splitlines()
files += [str(root/n) for n in ['go.work','go.work.sum'] if (root/n).exists()]
def manifest():
 result=[]
 for name in sorted(set(files)):
  actual=pathlib.Path(overlay.get(name,name)); data=actual.read_bytes()
  result.append({'path':name,'effectivePath':str(actual),'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()})
 return result
before=manifest();(out/'source.before.json').write_text(json.dumps(before,indent=2)+'\n')
meta={'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'status':subprocess.check_output(['git','status','--short'],cwd=root,text=True),'sourceScope':'All repository control-plane Go/module files plus root go.work files; this is not an external module/Go SDK manifest. Exactly two preserved original product source files overlaid on current integrated source.','command':['go','test','-overlay='+str(out/'overlay.json'),'-p','1','-count=1','-timeout','60s','-json','-run','^TestCoordinationHTTPDisjointIntentsPreserveBothPeopleAndReplay$','./internal/product'],'expected':'Nonzero exit at GET coordination404, same test body as fixed joined run; compilation failure is not accepted as counterexample.'}
(out/'invocation.json').write_text(json.dumps(meta,indent=2)+'\n')
start=time.time(); timed=False
with (out/'stdout.jsonl').open('wb') as stdout,(out/'stderr.txt').open('wb') as stderr:
 try: result=subprocess.run(meta['command'],cwd=root/'control-plane',env={**os.environ,'GOMAXPROCS':'2'},stdout=stdout,stderr=stderr,timeout=90);code=result.returncode
 except subprocess.TimeoutExpired:code=None;timed=True
rows=[]
for line in (out/'stdout.jsonl').read_text().splitlines():
 try: rows.append(json.loads(line))
 except: pass
after=manifest();(out/'source.after.json').write_text(json.dumps(after,indent=2)+'\n')
receipt={'exitCode':code,'timedOut':timed,'elapsedSeconds':time.time()-start,'repositoryInputCount':len(before),'inputsUnchanged':before==after,'passes':sum(r.get('Action')=='pass' and 'Test'in r for r in rows),'failures':sum(r.get('Action')=='fail' and 'Test'in r for r in rows),'missingRouteObserved':any('status 404, want 200' in r.get('Output','') for r in rows),'testBodySHA256':hashlib.sha256((root/'control-plane/internal/product/coordination_http_test.go').read_bytes()).hexdigest()}
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
