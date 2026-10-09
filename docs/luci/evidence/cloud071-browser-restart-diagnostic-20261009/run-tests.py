from pathlib import Path
import os,subprocess,json,hashlib,re,time
root=Path(__file__).parent;test=root/'response.test.mjs'
runs=[]
for variant in ['warm','cold']:
 for mode in ['before','after']:
  source=root/'before' if mode=='before' else root
  env=os.environ.copy();env.update(C071_SOURCE_ROOT=str(source),C071_VARIANT=variant)
  args=['node','--test','--test-reporter=tap']
  if mode=='before':args+=['--test-name-pattern=^exact known 409 retains']
  args+=[str(test)]
  prefix=root/'evidence'/(variant+'-'+mode+'-'+os.environ.get('C071_RUN_TAG','final'))
  paths=[test,source/variant/'launch.mjs',source/variant/'scenario.mjs']
  hashes=lambda:{str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}
  r={'variant':variant,'mode':mode,'argv':args,'cwd':str(root),'startedAt':time.time(),'beforeSHA256':hashes()}
  with prefix.with_suffix('.tap').open('wb') as out,prefix.with_suffix('.stderr').open('wb') as err:p=subprocess.run(args,cwd=root,env=env,stdout=out,stderr=err)
  text=prefix.with_suffix('.tap').read_text();r.update(exitCode=p.returncode,endedAt=time.time(),afterSHA256=hashes(),counts={k:int(v) for k,v in re.findall(r'^# (tests|pass|fail|cancelled|skipped) (\d+)$',text,re.M)},expectedOriginalFailure=('known browser refusal lost its diagnostic before success assertion' in text) if mode=='before' else None,tapSHA256=hashlib.sha256(prefix.with_suffix('.tap').read_bytes()).hexdigest(),stderrBytes=prefix.with_suffix('.stderr').stat().st_size)
  prefix.with_suffix('.json').write_text(json.dumps(r,indent=2)+'\n');runs.append(r)
  print(json.dumps({k:r[k] for k in ['variant','mode','exitCode','counts','expectedOriginalFailure','stderrBytes']}))
  assert r['beforeSHA256']==r['afterSHA256']
  assert p.returncode==(1 if mode=='before' else 0)
  assert mode!='before' or r['expectedOriginalFailure']
(root/('runs-'+os.environ.get('C071_RUN_TAG','final')+'.json')).write_text(json.dumps(runs,indent=2)+'\n')
