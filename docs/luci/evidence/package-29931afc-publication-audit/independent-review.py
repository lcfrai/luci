from pathlib import Path
import ast, hashlib, json, re, subprocess

BASE=Path(__file__).parent;CAP=BASE/'capsule';REPO=Path('/workspace/luci')
COMMIT='29931afc5b31ae9b9bd7b8973a7e3fdb840609a3'
sha=lambda b:hashlib.sha256(b).hexdigest()
checks=0;errors=[]
def check(ok,label):
    global checks
    checks+=1
    if not ok:errors.append(label)
def read(p):
    assert p.is_file() and not p.is_symlink() and p.stat().st_size<1024*1024
    return p.read_bytes()
def obj(n):return json.loads(read(CAP/n))
pub=obj('publication.json');check(sha(read(CAP/'publication.json'))=='d55b0fea126ee3e1fcc43b0f853b651f7e6da41634bbaa1918c8615922d993db','publication pin')
check(pub['sourceCommit']==COMMIT,'product pin')
files=pub['files'];check(len(files)==48,'48 manifest members')
for n,r in files.items():
    b=read(CAP/n);check(sha(b)==r['sha256']and len(b)==r['bytes'],'member '+n)
sums=read(CAP/'SHA256SUMS').decode().splitlines();check(len(sums)==49,'49 sums')
for line in sums:
    h,n=line.split('  ',1);check(sha(read(CAP/n))==h,'sum '+n)
actual={str(p.relative_to(CAP))for p in CAP.rglob('*')if p.is_file()}
check(actual==set(files)|{'publication.json','SHA256SUMS'},'exact50 files')
bindings=obj('original-bindings.json')['originals'];check(len(bindings)==39,'39 original bindings')
for row in bindings:
    original=read(Path(row['source']));published=read(CAP/row['publishedPath'])
    check(original==published and sha(published)==row['sha256']and len(published)==row['bytes'],'original '+row['publishedPath'])
selected=obj('source-bindings.json');check(selected['commit']==COMMIT and len(selected['inputs'])==5,'five selected source bindings')
for r in selected['inputs']:
    b=subprocess.check_output(['git','show',COMMIT+':'+r['path']],cwd=REPO,timeout=30)
    check(b==read(CAP/'selected-source'/r['path'])and sha(b)==r['sha256']and len(b)==r['bytes'],'Git selected '+r['path'])
for directory in ['helpers/build-package-next-v1','helpers/relocate-historical425-v1']:
    manifest=obj(directory+'/manifest.json')
    for r in manifest['files']:
        b=read(CAP/directory/r['path']);check(sha(b)==r['sha256']and len(b)==r['bytes'],'nested '+directory+'/'+r['path'])
build=obj('receipts/build-result.json');integrity=obj('receipts/package-integrity.json');provenance=obj('receipts/package-provenance.json');release=obj('release/release-manifest.json');scope=obj('scope.json');refusal=obj('receipts/first-candidate-refusal.json')
check(build['sourceCommit']==COMMIT and build['cleanBefore']and build['cleanAfter']and build['sameHead']and build['exitCode']==0 and not build['timedOut'],'clean build result')
check(build['helperSHA256']==refusal['scriptSHA256']==sha(read(CAP/'helpers/build-package-next-v1/build-package.py'))=='8771e5c661745b4cb4bfefc9aea0839f947dec534d3a438459bda0ecc409e418','exact original/final package helper')
check(refusal['sourceCommit']==COMMIT and refusal['exitCode']==1 and refusal['stage']=='Before output directory creation or release rename/build','first refusal retained')
check(build['environment']=={'GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1'}and build['timeoutSeconds']==600,'unchanged constrained build settings')
check(integrity['sourceCommit']==provenance['sourceCommit']==release['sourceCommit']==COMMIT,'package source join')
check(integrity['verifiedExtractedFiles']==scope['packageFiles']==962 and integrity['licenseComponents']==scope['licenses']==347 and integrity['missingLicenses']==0 and integrity['allMembersMatched'],'962 files and347 licences receipt')
check(integrity['subjects']==provenance['subjects'],'integrity/provenance subject equality')
for r in release['artifacts']:
    check({'name':'linux-amd64/'+r['name'],'bytes':r['bytes'],'sha256':r['sha256']}in integrity['subjects'],'release subject '+r['name'])
check({'name':'linux-amd64/release-manifest.json','bytes':len(read(CAP/'release/release-manifest.json')),'sha256':sha(read(CAP/'release/release-manifest.json'))}in integrity['subjects'],'manifest identity joined')
check(not release['nativeRequested']and release['nativeLicenseComponents']==0,'no native build')
stages=[ast.literal_eval(x)for x in read(CAP/'receipts/build.log').decode().splitlines()if x.startswith("{'compact':")]
check(len(stages)==1 and stages[0]['sourceCommit']==COMMIT and stages[0]['compact']and stages[0]['files']==51 and stages[0]['rewrittenLinks']==296,'recorded guide stage51/296')
preserved=obj('receipts/previous-release-preservation.json');check(preserved['oldSource']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'and preserved['newSource']==COMMIT and preserved['status']=='preserved-verified','prior5f4 preservation scope')
package_controls=obj('helpers/build-package-next-v1/controls-result.json');move_controls=obj('helpers/relocate-historical425-v1/controls-result.json')
for row,n in [(package_controls,10),(move_controls,6)]:check(row['passed']and row['cases']==len(row['results'])==n and all(x['passed']for x in row['results']),'recorded controlled cases '+str(n))
check(package_controls['sourceSHA256']==build['helperSHA256'],'ten controls bind build helper')
check(move_controls['sourceSHA256']==sha(read(CAP/'helpers/relocate-historical425-v1/relocate.py'))=='5a5335c6d980c4744fbc2c134cef282d4c2c594a771eb5ca170e63d72d8c7f18','six controls bind move helper')
cache=obj('storage/post-package-cache-clean.json');check(cache['exitCode']==0 and cache['GOCACHE']=='/tmp/luci-go-build-cache','separate exact cache result')
check(cache['freeAfter']['/tmp']-cache['freeBefore']['/tmp']==990539776 and cache['freeBefore']['/workspace']==cache['freeAfter']['/workspace']==6627311616,'cache arithmetic')
journals=sorted((CAP/'storage/older425-operation').glob('*.json'));check(len(journals)==7 and [int(p.name[:3])for p in journals]==list(range(1,8)),'seven move journals')
first=json.loads(read(journals[0]));last=json.loads(read(journals[-1]));check(first['sourceSHA256']==move_controls['sourceSHA256'],'actual move source identity')
check(last['passed']and last['bytes']==1013359941 and last['sha256']=='c593f37eaa3795358aa1b76dd355828ddaadfcf503b9a5e3a1aea6976c947c91'and len(last['links'])==2,'move retained identity/aliases')
check(last['freeAfter']['/workspace']-last['freeBefore']['/workspace']==last['measuredRootFreeDelta']==1013334016,'move root arithmetic')
check(last['freeBefore']['/tmp']==2414612480 and last['freeAfter']['/tmp']==1401249792,'move TMP observations')
check(last['freeAfter']['/tmp']<3905250841,'export TMP floor remains unmet at historical snapshot')
env_findings=[]
for p in(CAP/'storage/historical-archive-references').glob('*.json'):
    v=json.loads(read(p));env=v['candidate']['Config']['Env'];pairs=[x.split('=',1)for x in env]
    check(len(pairs)==3 and {x[0]for x in pairs}=={'PATH','NODE_VERSION','YARN_VERSION'},'finite imageEnv keys '+p.name)
    for key,value in pairs:
        allowed=bool(re.fullmatch(r'(?:/[A-Za-z0-9_./+-]+)(?::/[A-Za-z0-9_./+-]+)*',value))if key=='PATH'else bool(re.fullmatch(r'\d+\.\d+\.\d+',value))
        check(allowed,'static imageEnv type '+p.name+'/'+key)
    env_findings.append({'path':str(p.relative_to(CAP)),'allowedStaticKeys':[x[0]for x in pairs],'valuesNotPrinted':True})
for n in ['receipts/build-invocation.json','receipts/build-result.json']:check(obj(n)['environment']=={'GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1'},'finite buildEnv '+n)
patterns=[re.compile(rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),re.compile(rb'gh[pousr]_[A-Za-z0-9]{30,}'),re.compile(rb'AKIA[0-9A-Z]{16}')]
format_hits=[]
for n in actual:
    b=read(CAP/n)
    for i,pattern in enumerate(patterns):
        if pattern.search(b):format_hits.append({'path':n,'format':i})
check(not format_hits,'selected private-key/access-token formats absent')
name_hits=[n for n in actual if any(s in n.lower()for s in ['private-controller.stderr','private-transport-errors','/fixture/','/state/','credentials'])]
check(not name_hits,'obvious excluded private filenames absent')
report={'status':'passed'if not errors else'failed','checks':checks,'errors':errors,'scope':'Independent small-file source/provenance/checksum and selected privacy-field review. No archive/binary read/hash, test execution, compiler/build, runtime or storage action. Published verifier/operation results remain original run-owner evidence.','publicationSHA256':sha(read(CAP/'publication.json')),'files':len(actual),'bytes':sum((CAP/n).stat().st_size for n in actual),'members':len(files),'sumEntries':len(sums),'originalBindings':len(bindings),'selectedGitSources':5,'recordedPackageFiles':962,'recordedLicenceComponents':347,'recordedDistinctPackageControls':10,'recordedDistinctRelocationControls':6,'imageEnvironmentFields':env_findings,'privacyFormatMatches':format_hits,'privateFilenameMatches':name_hits,'privacyLimit':'Finite static Env/name/format check, not a universal secret scan. Source contains variable names/templates, not evaluated user environment.','remainingLimits':['No downloadable binaries, new runtime build/export, installed terminal or Windows acceptance supplied by this capsule.','Helper prep manifests retain historical not-executed status; actual operation/build receipts are later distinct evidence.','TMP/archive persistence and original hardlink topology limits retained; protected333 is metadata-only.','No original failed fixture, capacity charge, live-provider or response-time gate closed.','Export3,905,250,841B per-volume threshold and separate6,736,759,858B root Create floor stay unchanged; later free space needs a fresh observation.'],'checkerSHA256':sha(Path(__file__).read_bytes())}
out=BASE/'independent-review.json';out.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'path':str(out),'sha256':sha(read(out)),**report},indent=2))
