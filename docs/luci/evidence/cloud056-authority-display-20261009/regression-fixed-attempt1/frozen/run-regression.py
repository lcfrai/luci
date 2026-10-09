import os,pathlib,json,hashlib,subprocess,time,signal,sys
root=pathlib.Path('/workspace/luci');stage=pathlib.Path('/workspace/.luci-tools/session-authority-review-edd0');mode=sys.argv[1];assert mode in ('original','fixed');out=stage/('regression-'+mode+'-attempt1');out.mkdir(exist_ok=False)
bindings=json.loads((stage/'source-bindings.json').read_text())['files']
def capture():return [{'path':x['path'],'sha256':hashlib.sha256((root/x['path']).read_bytes()).hexdigest()} for x in bindings]
before=capture();assert all(x['sha256']==y['sha256'] for x,y in zip(before,bindings))
(out/'source-before.json').write_text(json.dumps(before,indent=2)+'\n')
helper=[]
for name in ['authority-regression.test.tsx','regression.config.mts','run-regression.py']+[str(p.relative_to(stage)) for p in sorted((stage/'after').rglob('*')) if p.is_file()]:
 b=(stage/name).read_bytes();helper.append({'path':name,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()});p=out/'frozen'/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(b)
(out/'input-bindings.json').write_text(json.dumps(helper,indent=2)+'\n')
versions={k:subprocess.check_output(v,text=True).strip() for k,v in {'node':['node','--version'],'npm':['npm','--version']}.items()};(out/'tool-versions.json').write_text(json.dumps(versions,indent=2)+'\n')
cmd=['node','node_modules/vitest/vitest.mjs','run','--config',str(stage/'regression.config.mts'),'--maxWorkers=1','--reporter=json','--outputFile='+str(out/'report.json')];env=os.environ.copy();env['CLOUD056_SOURCE_OVERLAY']='1' if mode=='fixed' else '0';start=time.time();timed=False
with (out/'stdout.log').open('wb') as a,(out/'stderr.log').open('wb') as b:
 p=subprocess.Popen(cmd,cwd=root/'desktop',stdout=a,stderr=b,env=env,start_new_session=True)
 try:code=p.wait(timeout=120)
 except subprocess.TimeoutExpired:
  timed=True;os.killpg(p.pid,signal.SIGTERM)
  try:code=p.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);code=p.wait(timeout=5)
after=capture();(out/'source-after.json').write_text(json.dumps(after,indent=2)+'\n');assert after==before
receipt={'mode':mode,'command':cmd,'elapsedSeconds':time.time()-start,'exitCode':code,'timedOut':timed,'sourceStable':True,'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'sourceCount':len(before),'overlayEnabled':env['CLOUD056_SOURCE_OVERLAY']}
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
