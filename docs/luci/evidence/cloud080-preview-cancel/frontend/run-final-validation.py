from pathlib import Path
import hashlib,json,subprocess,time
root=Path('/workspace/.luci-tools/cloud080-preview-cancel/frontend');repo=Path('/workspace/luci');out=root/'final-validation';out.mkdir()
paths=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','desktop'],cwd=repo,text=True).splitlines();paths=sorted(set(x for x in paths if not x.startswith('desktop/src-tauri/') and (repo/x).is_file()))
def fingerprint():return {p:{'sha256':hashlib.sha256((repo/p).read_bytes()).hexdigest(),'bytes':(repo/p).stat().st_size} for p in paths}
before=fingerprint();(out/'inputs-before.json').write_text(json.dumps(before,indent=2)+'\n')
commands=[['node','node_modules/vitest/vitest.mjs','run','src/components/journey/preview-cancellation.test.tsx','src/components/journey/deployment-continuity.test.tsx','src/api/product.test.ts','--maxWorkers=1','--reporter=json','--outputFile='+str(out/'vitest.json')],['node','node_modules/typescript/bin/tsc','--noEmit']]
results=[]
for index,cmd in enumerate(commands):
 start=time.monotonic()
 with (out/f'{index}.stdout.log').open('w') as stdout,(out/f'{index}.stderr.log').open('w') as stderr: result=subprocess.run(cmd,cwd=repo/'desktop',stdout=stdout,stderr=stderr,timeout=180)
 results.append({'command':cmd,'exitCode':result.returncode,'elapsedSeconds':time.monotonic()-start})
after=fingerprint();(out/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n');assert before==after
(out/'result.json').write_text(json.dumps({'commands':results,'inputsUnchanged':True,'inputCount':len(before)},indent=2)+'\n')
for file in ['desktop/src/api/product.ts','desktop/src/api/product.test.ts','desktop/src/components/lens/DeployLens.tsx','desktop/src/components/journey/preview-cancellation.test.tsx']:
 dest=out/'frozen'/file;dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes((repo/file).read_bytes())
print(json.dumps({'commands':results,'inputsUnchanged':True,'inputCount':len(before)}));raise SystemExit(0 if all(x['exitCode']==0 for x in results) else 1)
