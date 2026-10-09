from pathlib import Path
import json,hashlib,subprocess,difflib,re,shutil,datetime
BASE=Path('/workspace/.luci-tools/cloud054-terminal-close')
OUT=Path('/workspace/.luci-tools/publish-cloud057058/capsule')
COMMIT='edd0cb121ff6847f681ab6bf41509a497f043d41'
OUT.mkdir(mode=0o700,exist_ok=False)
def sha(b):return hashlib.sha256(b).hexdigest()
def save(p,b):
 q=OUT/p;q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(b);return {'path':p,'bytes':len(b),'sha256':sha(b)}
def js(p,v):return save(p,(json.dumps(v,indent=2)+'\n').encode())
bindings=[]
def copy(p):
 b=(BASE/p).read_bytes();row=save(p,b);bindings.append({'originalPath':str(BASE/p),'publishedPath':p,'representation':'exact-original-bytes','bytes':len(b),'sha256':sha(b)});return row
for p in ['installed-v2/launch.mjs','installed-v2/scenario.mjs']:
 copy(p)
for folder,prefix,extra in [('installed-v3','problem-',['launch.mjs','launch-before-problem-diagnostic.mjs','scenario.mjs']),('installed-v4','mount-',['launch.mjs','launch-before-git-guard.mjs','scenario.mjs'])]:
 for name in sorted({p.name for p in (BASE/folder).iterdir() if p.is_file() and p.name.startswith(prefix)}|set(extra)):
  copy(folder+'/'+name)
for p in sorted((BASE/'installed-v3-draft-before-error-sanitization').iterdir()):
 if p.is_file():copy(str(p.relative_to(BASE)))
for name in ['cloud057-refusal-schema-review.json','cloud057-v3-source-review-initial.json','cloud057-v3-source-review-final.json','cloud058-mount-contract-review.json','cloud058-v4-source-review-final.json']:
 copy(name)
# Reconstruct the earlier patch exactly from unchanged retained source bytes.
a=(BASE/'installed-v2/launch.mjs').read_text();b=(BASE/'installed-v3-draft-before-error-sanitization/launch.mjs').read_text()
patch=''.join(difflib.unified_diff(a.splitlines(True),b.splitlines(True),fromfile='installed-v2/launch.mjs',tofile='installed-v3/launch.mjs')).encode()
old=json.loads((BASE/'installed-v3/problem-validation-before-error-sanitization.json').read_text())['inputs']['problem-diagnostic.patch']
assert sha(patch)==old['sha256'] and len(patch)==old['bytes']
patchrow=save('derived/problem-diagnostic-before-sanitization.patch',patch)
js('derived-patch-provenance.json',{'scope':'Derived byte-equivalent artifact, not a newly found original file','artifact':patchrow,'matchesHistoricalDeclaration':old,'from':'installed-v2/launch.mjs','to':'installed-v3-draft-before-error-sanitization/launch.mjs','method':'Python difflib.unified_diff, original installed-v2/launch.mjs and installed-v3/launch.mjs headers'})
# Freeze only independently declared selected product source, not a compilation closure.
refs=[]
refs+=json.loads((BASE/'cloud057-refusal-schema-review.json').read_text())['sourceBindings']
refs+=json.loads((BASE/'cloud058-mount-contract-review.json').read_text())['sources']
refs += [dict(path=k,**v) for k,v in json.loads((BASE/'installed-v3/problem-schema-source-binding.json').read_text())['inputs'].items()]
selected={}
for row in refs:
 prior=selected.setdefault(row['path'],row);assert prior['sha256']==row['sha256'] and prior['bytes']==row['bytes']
sourceRows=[]
for p,row in sorted(selected.items()):
 b=subprocess.check_output(['git','-C','/workspace/luci','show',COMMIT+':'+p]);assert sha(b)==row['sha256'] and len(b)==row['bytes']
 dest='product-source/'+p;save(dest,b);sourceRows.append({'sourceCommit':COMMIT,'repositoryPath':p,'publishedPath':dest,'bytes':len(b),'sha256':sha(b)})
js('selected-product-sources.json',{'sourceCommit':COMMIT,'scope':'Selected schema, mount implementation and existing test sources only; not complete compiled inputs or a new test/build','sources':sourceRows})
# Recount retained reporter summaries; no test execution.
counts=[]
for folder in ['installed-v3','installed-v3-draft-before-error-sanitization','installed-v4']:
 for p in sorted((OUT/folder).glob('*.stdout')):
  text=p.read_text(); vals={k:int(v) for k,v in re.findall(r'^ℹ (tests|pass|fail|skipped|cancelled|todo) (\d+)$',text,re.M)}
  if vals: counts.append({'path':str(p.relative_to(OUT)),'sha256':sha(p.read_bytes()),'counts':vals})
js('retained-counts.json',{'method':'Recount reporter summary lines from exact copied originals, without execution','overlap':'Each original/draft/fixed round uses repeated related cases; counts are not additive unique coverage','rows':counts})
# Check all final validation inputs and map earlier changed-source references precisely.
validation=[]
for folder,name,earlier in [('installed-v3','problem-validation.json',False),('installed-v4','mount-validation.json',False),('installed-v3','problem-validation-before-error-sanitization.json',True)]:
 v=json.loads((OUT/folder/name).read_text())
 for filename,row in v['inputs'].items():
  candidates=[OUT/folder/filename]
  if earlier:candidates=[OUT/'installed-v3-draft-before-error-sanitization'/filename,OUT/'derived/problem-diagnostic-before-sanitization.patch' if filename=='problem-diagnostic.patch' else OUT/folder/filename]
  hit=next((p for p in candidates if p.is_file() and sha(p.read_bytes())==row['sha256'] and p.stat().st_size==row['bytes']),None)
  assert hit is not None,(folder,name,filename,row)
  validation.append({'validation':folder+'/'+name,'declaredInput':filename,'publishedPath':str(hit.relative_to(OUT)),'sha256':row['sha256'],'bytes':row['bytes']})
js('validation-input-mapping.json',{'scope':'Exact historical/final declared inputs mapped to their matching retained versions; no claims of new executions','bindings':validation})
js('original-bindings.json',{'scope':'Exact copies of selected external originals; excludes installed result or private fixture data','bindings':bindings})
readme='''# CLOUD-057 / CLOUD-058 controlled helper corrections

This capsule preserves selected external helper sources and inert fixture evidence for product source `edd0cb121ff6847f681ab6bf41509a497f043d41`. It does not contain an installed runtime result or establish Windows, browser, Docker, model-provider, recovery or performance acceptance. No app, fixture tests, Docker calls or package verification were rerun to assemble it.

CLOUD-057 records a bounded allowlisted HTTP Problem checkpoint before rejecting an unexpected response. Raw response bodies, arbitrary error messages/private paths and invented admission numbers are excluded; body length/SHA remain. The final correction also avoids JSON parser excerpts leaking through thrown errors and does not call an empty response parsed JSON. Original attempt 1's discarded response reason and numerical operands remain unavailable.

CLOUD-058 admits only the legitimate exact primary `.git/worktrees` read-only bind, with nonlinked/canonical directories and strict source/destination/type/RW checks. It preserves unrelated mount, allocation, resource and daemon guards. Original attempt 3's manual exact cleanup is not product lease release; no installed evidence or retained state is copied here. Static inspection does not close the production source's documented directory-swap race.

## Retained evidence and counts

Final same-test CLOUD-057 sequence: original **2 pass / 8 fail**, intermediate unsanitized draft **8 pass / 2 fail**, final **10 pass / 0 fail**. The earlier eight-case original **2 pass / 6 fail** and draft **8 pass** overlap those cases. The historical long-marker version did not reliably assert the parser's truncated excerpt; all its outcomes and exact test bytes remain separately retained. It is not stronger privacy evidence. Read `retained-counts.json` for each raw reporter summary; do not sum overlapping batches.

CLOUD-058 uses the same 21-case test file on original/fixed source: original **18 pass / 3 fail**, final **21 pass**. The three original failures are legitimate admin-bind refusal, accepting a state-backed admin-destination impostor and accepting missing primary protection. These are fake Docker/filesystem VM controls, not actual Docker observations.

`validation-input-mapping.json` binds every declared final input and the earlier eight-case inputs to exact retained files. The earlier draft patch had been overwritten in its source directory; `derived/problem-diagnostic-before-sanitization.patch` is reconstructed from retained sources and matches the original declared SHA256 and 16,727 bytes exactly. It is labeled derived, not claimed newly recovered original bytes. Original validation/review files retain their historical absolute metadata paths and limitations.

`product-source/` contains only the  selected schema/mount sources declared in `selected-product-sources.json`. It is not a Go compilation closure. `launch.mjs` files are preserved for exact-function extraction, not runnable installed launchers from this capsule: their external acceptance imports are deliberately absent. Never execute them as app launchers here.

## Verify and reproduce

Use Python 3.11+ for `python verify.py` (hash/size/member checks only). For controlled fixtures use supported Node 24.19.0 and `python reproduce-fixtures.py --output /absolute/new/disposable/receipts`. This explicitly launches only the builtin Node test runner, reading exact function slices into isolated VMs with fake fetch/Docker/filesystem callbacks. It performs five final before/draft/after invocations, expects the retained failures, and writes new results separately. It never imports/runs a launcher or contacts an app or daemon. Relative sibling directories are required and are preserved here. Reproductions are new evidence, not replacements for original logs; do not change frozen files or reclassify original installed attempts.

Source integrity, controlled semantics and actual installed acceptance remain separate. The CLOUD-059 route-settlement correction is outside this capsule.
'''
save('README.md',readme.encode())
verify='''from pathlib import Path\nimport hashlib,json\np=Path(__file__).resolve().parent\nv=json.loads((p/'publication.json').read_text())\nexpected={row['path'] for row in v['members']}\nactual={str(f.relative_to(p)) for f in p.rglob('*') if f.is_file()}-{'publication.json','SHA256SUMS'}\nassert actual==expected,(actual-expected,expected-actual)\nfor row in v['members']:\n b=(p/row['path']).read_bytes();assert len(b)==row['bytes'] and hashlib.sha256(b).hexdigest()==row['sha256'],row['path']\nfor line in (p/'SHA256SUMS').read_text().splitlines():\n h,name=line.split('  ',1);assert hashlib.sha256((p/name).read_bytes()).hexdigest()==h,name\nprint(json.dumps({'verifiedMembers':len(expected),'checksumEntries':len((p/'SHA256SUMS').read_text().splitlines()),'scope':'byte integrity only; no fixture/app execution'}))\n'''
save('verify.py',verify.encode())
reproduce='''from pathlib import Path\nimport argparse,hashlib,json,os,subprocess,sys,shutil\np=Path(__file__).resolve().parent\na=argparse.ArgumentParser();a.add_argument('--output',required=True,type=Path);a.add_argument('--node',default='node');args=a.parse_args()\nassert args.output.is_absolute();args.output.mkdir(mode=0o700,parents=False,exist_ok=False)\nnode=shutil.which(args.node);assert node,'Node executable unavailable'\nbase={k:v for k,v in os.environ.items() if k in ('PATH','SystemRoot','WINDIR','TEMP','TMP','LANG','LC_ALL')}\nrows=[]\ncases=[('problem-original','installed-v3','problem-diagnostic.test.mjs',{'C057_ORIGINAL':'1'},1),('problem-draft','installed-v3','problem-diagnostic.test.mjs',{'C057_DRAFT':'1'},1),('problem-fixed','installed-v3','problem-diagnostic.test.mjs',{},0),('mount-original','installed-v4','mount-contract.test.mjs',{'C058_ORIGINAL':'1'},1),('mount-fixed','installed-v4','mount-contract.test.mjs',{},0)]\nfor name,folder,test,flags,expected in cases:\n command=[node,'--max-old-space-size=64','--test','--test-concurrency=1',test]\n result=subprocess.run(command,cwd=p/folder,env=dict(base,**flags),capture_output=True,timeout=60)\n for suffix,b in [('stdout',result.stdout),('stderr',result.stderr)]:\n  assert len(b)<=262144;f=args.output/(name+'.'+suffix);f.write_bytes(b)\n rows.append({'name':name,'command':command,'cwd':folder,'envFlags':flags,'expectedExit':expected,'actualExit':result.returncode,'stdoutSHA256':hashlib.sha256(result.stdout).hexdigest(),'stderrSHA256':hashlib.sha256(result.stderr).hexdigest()})\n (args.output/'reproduction.json').write_text(json.dumps({'scope':'New inert VM fixture execution only','rows':rows},indent=2)+'\\n')\n if result.returncode!=expected:sys.exit(1)\nprint(json.dumps({'allExpectedExits':True,'invocations':len(rows),'output':str(args.output)}))\n'''
save('reproduce-fixtures.py',reproduce.encode())
save('assembly-source.py',Path(__file__).read_bytes())
members=[]
for p in sorted(OUT.rglob('*')):
 if p.is_file():members.append({'path':str(p.relative_to(OUT)),'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())})
js('publication.json',{'version':1,'productSource':COMMIT,'issues':['CLOUD-057','CLOUD-058'],'scope':'Selected controlled correction source/evidence; no installed result, test rerun or binary artifact','members':members})
sums=[]
for p in sorted(OUT.rglob('*')):
 if p.is_file():sums.append(sha(p.read_bytes())+'  '+str(p.relative_to(OUT)))
save('SHA256SUMS',('\n'.join(sums)+'\n').encode())
print(json.dumps({'files':len(members)+2,'members':len(members),'checksums':len(sums),'originalBindings':len(bindings),'selectedProductSources':len(sourceRows),'validationBindings':len(validation),'publicationSHA256':sha((OUT/'publication.json').read_bytes())}))
