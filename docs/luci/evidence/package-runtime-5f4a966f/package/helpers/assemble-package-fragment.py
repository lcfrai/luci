from pathlib import Path
import ast, hashlib, json, re, subprocess

ROOT = Path('/workspace/luci')
TOOLS = Path('/workspace/.luci-tools')
COMMIT = '5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
OUT = TOOLS / 'publish-package-runtime-5f4/package-fragment'
SRC = ROOT / 'artifacts/quality-swarm/candidate-5f4a966f/package'
RELEASE = ROOT / 'artifacts/releases/0.5.0'
sha = lambda b: hashlib.sha256(b).hexdigest()
def data(p):
    assert p.is_file() and not p.is_symlink(), str(p)
    assert p.stat().st_size < 4 * 1024 * 1024, str(p)
    return p.read_bytes()
def git(path):
    return subprocess.check_output(['git', 'show', COMMIT + ':' + path], cwd=ROOT, timeout=30)
def write_json(path, obj):
    path.write_text(json.dumps(obj, indent=2) + '\n')

assert not OUT.exists(), 'Frozen output must not be overwritten'
OUT.mkdir(parents=True)
bindings = []
before = {}
def put(content, target, original, **extra):
    p = OUT / target
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_bytes(content)
    bindings.append(dict(publishedPath=target, originalPath=str(original), originalBytes=len(content),
                         originalSHA256=sha(content), publishedBytes=len(content), publishedSHA256=sha(content),
                         transformation='none', **extra))
def copy(p, target):
    b = data(p); before[str(p)] = sha(b); put(b, target, p)

for name in ['build-invocation.json', 'build-result.json', 'build.log', 'package-integrity.json',
             'package-provenance.json', 'previous-release-preservation.json']:
    copy(SRC / name, 'receipts/' + name)
for name in ['release-manifest.json', 'SHA256SUMS']:
    copy(RELEASE / name, 'release/' + name)
copy(TOOLS / 'verify-local-package.py', 'helpers/verify-local-package.py')
copy(Path(__file__), 'helpers/assemble-package-fragment.py')
selected = ['scripts/build-release.sh', 'scripts/stage-release-docs.py', 'scripts/package.py',
            'scripts/collect-licenses.py', 'scripts/launch-unix.sh']
for path in selected:
    b = git(path)
    assert data(ROOT / path) == b, 'Selected source no longer matches pinned Git object'
    before[str(ROOT / path)] = sha(b)
    put(b, 'source/' + path, 'git:' + COMMIT + ':' + path, sourceCommit=COMMIT)

build = json.loads(data(SRC / 'build-result.json'))
integrity = json.loads(data(SRC / 'package-integrity.json'))
provenance = json.loads(data(SRC / 'package-provenance.json'))
manifest = json.loads(data(RELEASE / 'release-manifest.json'))
preservation = json.loads(data(SRC / 'previous-release-preservation.json'))
assert build['sourceCommit'] == COMMIT and build['exitCode'] == 0 and not build['timedOut']
assert build['cleanBefore'] and build['cleanAfter'] and build['sameHead']
assert integrity['sourceCommit'] == COMMIT and integrity['sourceDirty'] is False
assert integrity['verifiedExtractedFiles'] == 961 and integrity['licenseComponents'] == 347
assert integrity['missingLicenses'] == 0 and integrity['allMembersMatched']
assert provenance['sourceCommit'] == COMMIT and provenance['sourceDirty'] is False
assert manifest['sourceCommit'] == COMMIT and manifest['sourceDirty'] is False and not manifest['nativeRequested']
for artifact in manifest['artifacts']:
    assert {'name':'linux-amd64/'+artifact['name'], 'bytes':artifact['bytes'], 'sha256':artifact['sha256']} in integrity['subjects']
assert integrity['subjects'] == provenance['subjects']
assert {'name':'linux-amd64/release-manifest.json', 'bytes':len(data(RELEASE/'release-manifest.json')),
        'sha256':sha(data(RELEASE/'release-manifest.json'))} in integrity['subjects']
assert preservation['oldSource'] == 'edd0cb121ff6847f681ab6bf41509a497f043d41'
assert preservation['newSource'] == COMMIT and preservation['status'] == 'preserved-verified'

# Cross-fragment sources are copied by the parent, not duplicated here.
external = []
for name in ['build-package.py', 'package-attempt1-preflight-refusal.json']:
    p = TOOLS/'cloud054-runtime-helper'/name; b = data(p); before[str(p)] = sha(b)
    external.append({'originalPath':str(p), 'name':name, 'sha256':sha(b), 'bytes':len(b),
                     'publicationOwner':'main package-runtime capsule; not copied in this fragment'})
assert external[0]['sha256'] == build['helperSHA256'] == '150b7e999d7238eb765f3025e9e42f904001828e37defdde39e780a937d463f1'
refusal = json.loads(data(TOOLS/'cloud054-runtime-helper/package-attempt1-preflight-refusal.json'))
assert refusal['source'] == COMMIT and refusal['buildOrReleaseRenameStarted'] is False
assert refusal['helperSHA256'] == build['helperSHA256']
write_json(OUT/'external-helper-bindings.json', {'scope':'Referenced originals to be included by parent capsule; no executable helper was run by this publication.', 'files':external})

# Inventory source Git blobs only, not rewritten/staged package bytes.
paths = ['LICENSE','NOTICE','README.md','DESIGN.md','docs/UAT.md','docs/RUNBOOK.md','docs/luci']
tree = subprocess.check_output(['git','ls-tree','-r','-l','-z',COMMIT,'--',*paths], cwd=ROOT, timeout=30)
guides = []
for line in tree.split(b'\0'):
    if not line: continue
    head, raw_path = line.split(b'\t',1); path=raw_path.decode()
    if path.startswith('docs/luci/evidence/'): continue
    mode,kind,oid,size=head.decode().split()
    assert mode in ('100644','100755') and kind=='blob'
    guides.append({'path':path,'gitBlob':oid,'gitMode':mode,'bytes':int(size)})
staging = [ast.literal_eval(line) for line in data(SRC/'build.log').decode().splitlines() if line.startswith("{'compact':")]
assert len(staging)==1 and staging[0]['compact'] is True and staging[0]['sourceCommit']==COMMIT
assert len(guides)+1 == staging[0]['files'] == 50
write_json(OUT/'source-guide-inventory.json', {'sourceCommit':COMMIT,'scope':'49 selected source Git blobs (not staged/rewritten hashes); the recorded build adds docs/PACKAGE-VALIDATION.md. No guide regeneration or archive read was performed for publication.', 'sourceGuideFiles':len(guides),'recordedStage':staging[0],'files':guides})

assert all(sha(data(Path(p))) == digest for p,digest in before.items()), 'An original changed while publication was assembled'
write_json(OUT/'original-bindings.json', {'sourceCommit':COMMIT,'scope':'Exact completed receipt/helper and selected Git-source copies, checked unchanged after assembly; no archive/binary reread, build, verifier, runtime or test execution.', 'files':bindings})
(OUT/'README.md').write_text('''# Clean Linux package fragment — 5f4a966f

The completed package build and verifier record clean source `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`, exit 0, **961 extracted files**, **347 licensed components**, and **zero missing licences**. This is Linux packaging evidence, not installed terminal/recovery, responsiveness, Windows native, live-provider or full local acceptance.

The portable archive is **69,108,246 bytes**, SHA256 `bc7d8b76b76d3c291c651c3f46423f75f7e63f31f49ddde59372fe6de97e6290`. The cloud archive is **34,646,330 bytes**, SHA256 `bd5bb64f920ca577e482e47bcb3e24646d6612951b00b7108f7908adde3bba38`. Original provenance binds service binary `5d1404766d21fe74eac4f87fc4ec2177b3bb3bca2bee0347afb21eaefb25adf1` and web index `7c3a938b41384326d857b8c923de79172286a2b8318cff6237e0f9637ad6d7c3`. These are completed run-owner verification receipts; publication did not independently read or hash the archives/binary, and does not make them publicly downloadable.

Build command: `bash scripts/build-release.sh 0.5.0 linux-amd64`, with `GOMAXPROCS=2`, `GOFLAGS=-mod=readonly -p=1`, and 600-second bound. The final result records 85.266210 seconds, no timeout, clean before/after and unchanged HEAD. The invocation's historical `status: running` is superseded by the final result. Build duration is not application performance. Raw build warnings remain in `receipts/build.log`.

The original verifier freezes and hashes both declared archives, extracts the portable archive, and compares all 961 regular member contents to their extracted bytes. Its licence check asserts that the licence manifest reports no missing entries; this is not independent legal analysis. The build log records compact guide staging of **50 files** and **290 rewritten links**. `source-guide-inventory.json` independently enumerates **49 source Git blobs**, excluding the evidence subtree, plus the generated index documented by the staging source. This inventory is not a staged-byte or link-resolution verification. No new `compact-package-integrity.json` exists for this candidate, so prior additive guide/cloud/mode checks are not carried forward as a current pass.

The main capsule must include the frozen wrapper and initial refusal named in `external-helper-bindings.json`; they are not duplicated here. Wrapper SHA256 is `150b7e999d7238eb765f3025e9e42f904001828e37defdde39e780a937d463f1`. Its first attempt refused the stale local remote-tracking ref before any build or release rename. That preserved refusal is a root tool-history projection, not independently captured original stderr. The corrected attempt followed a remote-ref fetch, retaining the same helper bytes and exact source requirement.

`previous-release-preservation.json` records the byte-preserving rename of the previous **edd0cb12** generated release directory and exact prior archive hashes/targets. It is separate run-owner evidence; this publication did not move archives or inspect private retained fixtures/ledgers. No old failure or cleanup obligation is closed. Runtime export/image verification belongs to the parent's separate fragment; this package fragment neither assumes the old 104-file runtime inventory nor claims the new image was executed.

Five selected packaging Git sources and the original verifier are frozen here, with exact source/binding manifests. This is not a full compiler-input/dependency capture. Standard task paths and process IDs in original receipts are retained as provenance. No credentials or private application-state files were read. Source retrieval/build instructions and Windows isolation/runtime prerequisites belong to the combined handoff. This fragment is not `LOCAL TEST HANDOFF READY`.
''')
members=[{'path':str(p.relative_to(OUT)),'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())} for p in sorted(OUT.rglob('*')) if p.is_file()]
write_json(OUT/'publication.json', {'sourceCommit':COMMIT,'scope':'Completed Linux package receipts plus selected sources; archive/binary checks are retained run-owner results, not repeated at publication.', 'members':members,'manifestConvention':'Members exclude publication.json and SHA256SUMS. SHA256SUMS also binds publication.json. Parent-owned cross-fragment sources are separately listed.'})
all_files=[p for p in sorted(OUT.rglob('*')) if p.is_file()]
(OUT/'SHA256SUMS').write_text(''.join(sha(p.read_bytes())+'  '+str(p.relative_to(OUT))+'\n' for p in all_files))
print(json.dumps({'path':str(OUT),'files':len(all_files)+1,'members':len(members),'sums':len(all_files),'originalBindings':len(bindings),'externalBindings':len(external),'sourceGuideFiles':len(guides),'publicationSHA256':sha((OUT/'publication.json').read_bytes()),'sumsSHA256':sha((OUT/'SHA256SUMS').read_bytes())},indent=2))
