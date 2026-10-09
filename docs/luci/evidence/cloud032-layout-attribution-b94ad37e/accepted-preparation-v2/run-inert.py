from pathlib import Path
import hashlib,json,subprocess,time
R=Path(__file__).resolve().parent
names=['layout-trace-capture.mjs','layout-trace-projector.mjs','layout-trace-capture.test.mjs','projector.test.mjs','integration.test.mjs','actual/sustained.mjs','actual/layout-trace-capture.mjs','actual/layout-trace-projector.mjs','actual/external-source-binding.json','launch.py','prepare.py']
def hashes():return {n:hashlib.sha256((R/n).read_bytes()).hexdigest() for n in names}
before=hashes();started=time.time();commands=[['node','--check',str(R/'actual/sustained.mjs')],['node','--test','--test-concurrency=1',*[str(R/n) for n in ['layout-trace-capture.test.mjs','projector.test.mjs','integration.test.mjs']]]];results=[]
for i,command in enumerate(commands):
 with (R/f'inert-final-attempt2-{i}.stdout.log').open('x') as out,(R/f'inert-final-attempt2-{i}.stderr.log').open('x') as err:
  run=subprocess.run(command,stdout=out,stderr=err,timeout=20)
 results.append({'command':command,'exitCode':run.returncode})
after=hashes();receipt={'scope':'Node syntax parse plus inert CDP/EventEmitter/controlled-stream/clock/driver-orchestration tests only; no browser/app/Docker/compiler/package build.','commands':results,'inputHashesBefore':before,'inputHashesAfter':after,'inputsUnchanged':before==after,'elapsedSeconds':time.time()-started,'passed':before==after and all(x['exitCode']==0 for x in results)}
(R/'inert-final-attempt2.receipt.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'passed':receipt['passed'],'inputsUnchanged':receipt['inputsUnchanged'],'elapsedSeconds':receipt['elapsedSeconds']}));assert receipt['passed']
