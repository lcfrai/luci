#!/usr/bin/env python3
"""Rebind the frozen v1 helper to one externally validated observer change only."""
from pathlib import Path
import argparse,hashlib,json,shutil,difflib,subprocess
BASE=Path('/workspace/.luci-tools/cloud032-autonomous-readiness-v1-20261009/capsule')
HERE=Path(__file__).resolve().parent.parent
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('observer',type=Path);args=parser.parse_args()
 before=(BASE/'actual/autonomous-readiness.mjs').read_text();after=args.observer.read_text()
 line="   if(selected.length!==1||selected[0].getAttribute('data-id')!==a.project.id||selected[0].getAttribute('aria-label')!==a.project.name)return {reason:'project-mismatch',visited,controls};\n"
 assert before.count(line)==1,'Frozen selected-project gate changed'
 assert before.split('const limits=Object.freeze(',1)[1].split('\n',1)[0]==after.split('const limits=Object.freeze(',1)[1].split('\n',1)[0],'Every observer cap must remain exact'
 # The actual driver changes only its declared variant name. Original action
 # module, route/origin/mutation guard, deadlines and targets remain exact.
 for directory in ['actual','original-v1']:
  dest=HERE/directory;dest.mkdir(exist_ok=True)
  for p in (BASE/'actual').iterdir():
   if p.is_file():shutil.copyfile(p,dest/p.name)
 shutil.copyfile(args.observer,HERE/'actual/autonomous-readiness.mjs')
 source=(HERE/'actual/sustained.mjs').read_text();old="mode:'installed-four-project-autonomous-readiness-diagnostic',";new="mode:'installed-four-project-autonomous-readiness-v2-diagnostic',";assert source.count(old)==1;source=source.replace(old,new);(HERE/'actual/sustained.mjs').write_text(source)
 binding=json.loads((BASE/'actual/external-source-binding.json').read_text());binding['executionDirectory']=str(HERE/'actual');binding['observerRevision']=2;binding['observerChangeScope']='CLOUD055: exact selected-project guard before scanning Define lens controls; caps/actions/targets/routes unchanged.';binding['frozenV1ObserverSHA256']=sha(BASE/'actual/autonomous-readiness.mjs')
 for name in binding['localSources']:binding['localSources'][name]=sha(HERE/'actual'/name)
 (HERE/'actual/external-source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')
 for name in ['launch.py','template-capture.json']:shutil.copyfile(BASE/name,HERE/name)
 # Inert preparation only; do not inspect Docker or launch the installed app.
 for name in ['sustained.mjs','autonomous-readiness.mjs','readiness-ledger.mjs']:
  result=subprocess.run(['node','--check',str(HERE/'actual'/name)],capture_output=True,text=True,timeout=10);assert result.returncode==0,(name,result.stderr)
 compile((HERE/'launch.py').read_text(),'launch.py','exec')
 for name in ['interaction-metrics.mjs','latency-diagnostics.mjs','readiness-ledger.mjs']:assert (HERE/'actual'/name).read_bytes()==(BASE/'actual'/name).read_bytes()
 assert source.replace(new,old)==(BASE/'actual/sustained.mjs').read_text()
 (HERE/'observer.patch').write_text(''.join(difflib.unified_diff(before.splitlines(keepends=True),after.splitlines(keepends=True),fromfile='frozen-v1/autonomous-readiness.mjs',tofile='v2/autonomous-readiness.mjs')))
 (HERE/'integration/preparation.json').write_text(json.dumps({'scope':'No installed run; frozen v1 preserved. One selected-project gate moved before Define scan, declared variant updated, source binding resealed.','originalActionModulesExact':True,'originalRouteGuardExact':True,'originalTargetsAndDeadlinesExact':True,'allCapsExact':True,'prospectiveProduct':binding['prospectiveProduct'],'observerBeforeSHA256':sha(BASE/'actual/autonomous-readiness.mjs'),'observerAfterSHA256':sha(HERE/'actual/autonomous-readiness.mjs'),'driverSHA256':sha(HERE/'actual/sustained.mjs'),'bindingSHA256':sha(HERE/'actual/external-source-binding.json'),'launcherSHA256':sha(HERE/'launch.py'),'installedRuntimeExecuted':False},indent=2)+'\n')
 print(json.dumps({'prepared':True,'observerSHA256':sha(HERE/'actual/autonomous-readiness.mjs'),'driverSHA256':sha(HERE/'actual/sustained.mjs'),'bindingSHA256':sha(HERE/'actual/external-source-binding.json')}))
if __name__=='__main__':main()
