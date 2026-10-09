from pathlib import Path
import datetime,hashlib,json,subprocess,time
root=Path('/workspace/luci');out=Path(__file__).parent
paths=['docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md','docs/SEAMS.md','desktop/src/seams.test.ts','desktop/src/store/types.ts','desktop/package-lock.json','desktop/vitest.config.ts']
def inputs():return {p:{'sha256':hashlib.sha256((root/p).read_bytes()).hexdigest(),'bytes':(root/p).stat().st_size} for p in paths}
before=inputs();cmd=['npm','test','--','src/seams.test.ts','--maxWorkers=1','--reporter=json','--outputFile='+str(out/'vitest.json')]
start=time.monotonic();at=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (out/'stdout.log').open('w') as so,(out/'stderr.log').open('w') as se:r=subprocess.run(cmd,cwd=root/'desktop',stdout=so,stderr=se,timeout=120)
after=inputs();v=json.loads((out/'vitest.json').read_text())
d={'command':cmd,'startedAt':at,'elapsedSeconds':time.monotonic()-start,'exitCode':r.returncode,'before':before,'after':after,'inputsStable':before==after,'counts':{k:v.get(k) for k in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests']},'scope':'Update canonical domain-model documentation to match new optional executionGeneration. Test and application source unchanged from original full run; original full2826pass/1fail retained. This affected follow-up is not a second full-suite run.'}
(out/'result.json').write_text(json.dumps(d,indent=2)+'\n');print(json.dumps(d));raise SystemExit(r.returncode if before==after else 1)
