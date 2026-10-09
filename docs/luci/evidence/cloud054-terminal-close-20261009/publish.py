#!/usr/bin/env python3
"""Copy a bounded source/controlled-test publication; never modifies repository files."""
import datetime,hashlib,json,os,pathlib,shutil,subprocess
HERE=pathlib.Path(__file__).resolve().parent
STAGE=pathlib.Path('/workspace/.luci-tools/cloud054-terminal-close')
REPO=pathlib.Path('/workspace/luci')
OUT=HERE/'capsule'
OUT.mkdir(exist_ok=False)
originals=[]
def bind(b):return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
def put(name,b):
 p=OUT/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(b)
def copy(p,name):
 assert p.is_file() and not p.is_symlink(),p
 b=p.read_bytes();put(name,b);originals.append({'source':str(p),'publication':name,**bind(b),'representation':'byte-exact'})
def copy_tree(p,prefix):
 for root,dirs,files in os.walk(p):
  dirs[:]=sorted(d for d in dirs if d not in {'tree','node_modules','.git','.cloud054-cache','cache'})
  for n in sorted(files):
   q=pathlib.Path(root)/n;copy(q,str(pathlib.PurePosixPath(prefix)/q.relative_to(p)))
for sub in ['backend','frontend','generation-followup','generation-followup-v2','frontend-overload-followup-v1','frontend-overload-followup-v2']:
 copy_tree(STAGE/sub,sub)
for sub in ['frontend-overload-followup-v1','frontend-overload-followup-v2']:
 rel='tree/desktop/src/api/attachRegistry.overload-generation.test.ts';copy(STAGE/sub/rel,sub+'/'+rel)
copy(STAGE/'frontend/tree/desktop/vitest.config.ts','frontend/tree/desktop/vitest.config.ts')
for n in ['DEFERRED-AUTHORITY-REVIEW.md','root-integration.json']:copy(STAGE/n,n)
COM=pathlib.Path('/workspace/.luci-tools/cloud054-combined-validation')
for kind in ['go','frontend']:copy_tree(COM/kind,'combined/'+kind)
copy(COM/'tool-versions.json','combined/tool-versions.json')
copy(pathlib.Path('/workspace/.luci-tools/cloud054-combined-validation.py'),'combined/capture.py')
copy_tree(pathlib.Path('/workspace/.luci-tools/cloud054-contract-followup'),'contract-followup')
CHECKS=pathlib.Path('/workspace/.luci-tools/cloud053-root-publication-checks')
for n in ['immutable-f435-result.json','immutable-f435.stdout.json','immutable-f435.stderr.log']:copy(CHECKS/n,'publication-guard/'+n)
copy(REPO/'scripts/acceptance/verify-publication.py','publication-guard/verify-publication.py')
copy(REPO/'docs/luci/API.md','documentation/API.after.md')
env={k:v for k,v in os.environ.items() if not k.startswith('GIT_')};env['GIT_NO_LAZY_FETCH']='1'
base='f43542a84bd9cd81c2b263e680c36d8225e10024'
old=subprocess.check_output(['git','-C',str(REPO),'show',base+':docs/luci/API.md'],env=env)
put('documentation/API.before.md',old)
integrated=json.loads((STAGE/'root-integration.json').read_text())
for row in integrated['files']:
 assert bind((REPO/row['path']).read_bytes())=={'bytes':row['bytes'],'sha256':row['afterSHA256']},row['path']
extra='desktop/src/api/attachRegistry.overload-generation.test.ts'
b=(REPO/extra).read_bytes();assert b==(STAGE/'frontend-overload-followup-v2/tree'/extra).read_bytes()
integrated['files'].append({'path':extra,'source':'frontend-overload-followup-v2/tree/'+extra,'beforeSHA256':None,'afterSHA256':bind(b)['sha256'],'bytes':len(b)})
for row in integrated['files']:
 if row['beforeSHA256']:
  old=subprocess.check_output(['git','-C',str(REPO),'show',base+':'+row['path']],env=env)
  assert bind(old)['sha256']==row['beforeSHA256'],row['path']
  put('integrated-original/'+row['path'],old)
put('integrated-source-binding.json',(json.dumps(integrated,indent=2)+'\n').encode())
# Verify immutable subordinate declarations before publishing; these are not test reruns.
checks=[]
for m in json.loads((STAGE/'backend/manifest.json').read_text())['members']:
 assert bind((STAGE/'backend'/m['path']).read_bytes())=={'bytes':m['bytes'],'sha256':m['sha256']};checks.append('backend/'+m['path'])
for p,v in json.loads((STAGE/'frontend-overload-followup-v2/handoff.json').read_text())['files'].items():
 assert bind((STAGE/p).read_bytes())==v;checks.append(p)
for kind in ['go','frontend']:
 before=json.loads((COM/kind/'inputs-before.json').read_text());after=json.loads((COM/kind/'inputs-after.json').read_text());assert before==after
 mismatches=[]
 for path,v in before.items():
  if path.startswith('/'):continue
  if bind((REPO/path).read_bytes())!=v:mismatches.append(path)
 assert not mismatches,mismatches
assert json.loads((COM/'go/result.json').read_text())['outcomes']=={'pass':74,'fail':0,'skip':0}
f=json.loads((COM/'frontend/result.json').read_text());assert f['vitest']['numPassedTests']==2826 and f['vitest']['numFailedTests']==1
c=json.loads((OUT/'contract-followup/result.json').read_text());assert c['counts']['numPassedTests']==24 and c['exitCode']==0
put('publication-input-check.json',(json.dumps({'createdAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'integratedSources':17,'subordinateBindingChecks':len(checks),'combinedGoInputs':1198,'combinedFrontendInputs':1199,'allCapturedRepositoryInputsMatchCurrent':True,'externalDependenciesNotRehashed':True,'testReruns':False,'subordinateChecks':checks},indent=2)+'\n').encode())
put('original-bindings.json',(json.dumps({'scope':'All copied originals are byte-exact. Tooling/source checkout locators and inert controlled fixture markers remain in raw records. No full isolated tree, installed private fixture, credentials, dependency tree or raw user session is published.','members':originals},indent=2)+'\n').encode())
copy(pathlib.Path(__file__),'publish.py')
print(json.dumps({'capsule':str(OUT),'originalFiles':len(originals),'files':sum(p.is_file() for p in OUT.rglob('*'))}))
