from pathlib import Path
import hashlib,json,os,subprocess,time
B=Path(__file__).parent;E=B/'evidence';E.mkdir(exist_ok=True)
def meta(p):
 b=p.read_bytes();return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
node=subprocess.check_output(['node','--version'],text=True).strip();runs=[]
for variant in ['warm','cold']:
 for label,root,pattern in [('before',B/'before','late route callback after main catch is bypassed'),('after',B,None)]:
  command=['node','--test','--test-reporter=tap']
  if pattern:command+=['--test-name-pattern='+pattern]
  command+=[str(B/'failure.test.mjs')]
  name=variant+'-'+label;bindings={str(p):meta(p) for p in [root/variant/'launch.mjs',B/'failure.test.mjs']}
  env={**os.environ,'C072_SOURCE_ROOT':str(root),'C072_VARIANT':variant};start=time.time();r=subprocess.run(command,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
  (E/(name+'.tap')).write_bytes(r.stdout);(E/(name+'.stderr')).write_bytes(r.stderr)
  result={'name':name,'node':node,'command':command,'sourceRoot':str(root),'variant':variant,'elapsedSeconds':time.time()-start,'exitCode':r.returncode,'expectedExitCode':1 if label=='before' else 0,'sourceBefore':bindings,'sourceAfter':{p:meta(Path(p)) for p in bindings},'stdout':meta(E/(name+'.tap')),'stderr':meta(E/(name+'.stderr')),'scope':'Inert Node callbacks and temporary private-writer fixture only. No browser, Docker, product server, or compiler.'}
  assert result['sourceBefore']==result['sourceAfter'];(E/(name+'.json')).write_text(json.dumps(result,indent=2)+'\n');runs.append(result);print(json.dumps({k:result[k] for k in ['name','exitCode','elapsedSeconds']}))
  assert r.returncode==result['expectedExitCode']
(B/'runs.json').write_text(json.dumps(runs,indent=2)+'\n')
