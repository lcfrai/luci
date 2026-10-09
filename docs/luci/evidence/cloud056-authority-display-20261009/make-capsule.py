#!/usr/bin/env python3
"""Copy allowlisted retained C056 evidence into a new external publication capsule."""
import argparse,hashlib,json,pathlib,shutil
p=argparse.ArgumentParser();p.add_argument('--destination',required=True);p.add_argument('--validation');p.add_argument('--candidate-revision');a=p.parse_args();stage=pathlib.Path(__file__).resolve().parent;dest=pathlib.Path(a.destination).resolve()
if dest.exists():raise SystemExit('Refusing existing capsule; retain its original bytes')
dest.mkdir(parents=True)
selected=['source-bindings.json','preparation-manifest.json','source-review.json','correction-manifest.json','integration-preparation-manifest.json','authority-window.test.tsx','authority-regression.test.tsx','team_session_denial_contract_test.go','vitest.config.ts','regression.config.mts','run-original.py','run-regression.py','validate-integrated.py','make-capsule.py','cloud056.patch','HANDOFF.md','INTEGRATION-VALIDATION.md','frozen','after','repo-ready','repo-ready-final','original-attempt1','regression-original-attempt1','regression-fixed-attempt1']
for name in selected:
 src=stage/name;target=dest/name
 if src.is_symlink():raise SystemExit('Refusing symlink '+name)
 if src.is_dir():
  for x in src.rglob('*'):
   if x.is_symlink():raise SystemExit('Refusing nested symlink '+str(x))
  shutil.copytree(src,target)
 else:shutil.copyfile(src,target)
shutil.copyfile(stage/'README.md',dest/'PREPARATION-README.md')
validation=None
if a.validation:
 src=pathlib.Path(a.validation).resolve()
 if not (src/'result.json').is_file():raise SystemExit('Missing validation receipt')
 for x in src.rglob('*'):
  if x.is_symlink():raise SystemExit('Refusing validation symlink')
 shutil.copytree(src,dest/'integrated-validation');validation=json.loads((src/'result.json').read_text())
status={'issue':'CLOUD-056','baseProductRevision':'edd0cb121ff6847f681ab6bf41509a497f043d41','originalWorkingHEAD':'8c7bf16aa035b27dbea185b565052495eb60777f','integratedCandidateRevision':a.candidate_revision,'validation':validation,'recordedRuns':[{'path':'original-attempt1','passed':9,'failed':1,'scope':'Initial counterexample plus9controls'},{'path':'regression-original-attempt1','passed':9,'failed':5,'scope':'Stable14-case original regression'},{'path':'regression-fixed-attempt1','passed':14,'failed':0,'scope':'Same14-case regression with three source overlays'}],'overlap':'The ten initial tests overlap the fourteen-case stable regression. Original/fixed outcomes are not independent cumulative tests.','pending':['Integrated affected suite/TypeScript unless an explicit validation receipt is supplied','Unexecuted optional production registrar control unless included in validation','Actual installed/native/Windows authority workflow','No browser paint/timing or whole-machine closure claim'],'finalSourceDifference':'One comment-only clarification after fixed14/14; preserved frozen run copies show that original comment.','finalRepoTestAdaptation':'Relative import rewriting plus removal of unused default React import; test logic unchanged. Actual repo-ready-final execution is covered only by later integrated validation.'}
(dest/'STATUS.json').write_text(json.dumps(status,indent=2)+'\n')
(dest/'README.md').write_text('''# CLOUD-056: stale cached workspace display after a current membership refusal\n\nThe retained original counterexample and controlled source correction are distinct from integration, installed and Windows acceptance. See STATUS.json for exact revisions/counts and HANDOFF.md for behavior and limits. Backend revocation still refuses new private data; this finding concerns already delivered UI content after its next authoritative denial.\n\nThis capsule preserves original failed cases and every original log, along with selected42-file source bindings, exact controlled adapters, source overlays and final repository-ready test adaptation. `frozen/` is the selected original dependency scope, not a complete installed dependency export. Raw logs may be ignored by Git: explicitly add these selected capsule files and run the repository immutable/staged publication guard before claiming publication.\n\n`PREPARATION-README.md` and its manifest describe the earlier unexecuted preparation snapshot; later original/fixed receipts supersede its execution status without editing historical bytes. The optional Go contract file is preparation only unless a later validation explicitly records its execution. Source/application credentials and private runtime fixtures were never read or exported by these controlled tests.\n\nReproduction/integration commands: INTEGRATION-VALIDATION.md. Do not run checks during another installed performance measurement. No uncontrolled server or Docker workload is required.\n''')
sha=lambda b:hashlib.sha256(b).hexdigest()
rows=[]
for x in sorted(dest.rglob('*')):
 if x.is_file():
  b=x.read_bytes();rows.append({'path':str(x.relative_to(dest)),'bytes':len(b),'sha256':sha(b)})
manifest={'schema':1,'issue':'CLOUD-056','baseProductRevision':status['baseProductRevision'],'candidateRevision':a.candidate_revision,'scope':'Retained selected-source controlled evidence; integration acceptance only as explicitly supplied','files':rows}
(dest/'publication-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
checks=rows+[{'path':'publication-manifest.json','sha256':sha((dest/'publication-manifest.json').read_bytes())}]
(dest/'SHA256SUMS').write_text(''.join(x['sha256']+'  '+x['path']+'\n' for x in checks))
print(json.dumps({'destination':str(dest),'payloadFiles':len(rows),'checksumEntries':len(checks),'manifestSHA256':sha((dest/'publication-manifest.json').read_bytes()),'checksumFileSHA256':sha((dest/'SHA256SUMS').read_bytes())}))
