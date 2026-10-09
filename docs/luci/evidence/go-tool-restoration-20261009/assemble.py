"""Copy only completed small Go-restoration evidence; never run an operation."""
from pathlib import Path
import hashlib, json, os, stat

BASE = Path('/workspace/.luci-tools')
OUT = BASE / 'publish-go-tool-restoration'
TREE = OUT / 'tree'
REL = Path('docs/luci/evidence/go-tool-restoration-20261009')
CAP = TREE / REL
PREP = BASE / 'go-tool-restore-v1'
OP = 'operation-304739566509590'
WRAP = 'root-operation-304739566509590'
MAX_SMALL = 256 * 1024

def sha(b): return hashlib.sha256(b).hexdigest()
def enc(v): return (json.dumps(v, indent=2) + '\n').encode()
def read(p):
    s=p.lstat()
    assert stat.S_ISREG(s.st_mode) and s.st_size <= MAX_SMALL, str(p)
    return p.read_bytes()
def write(p,b):
    p.parent.mkdir(parents=True, exist_ok=True)
    with p.open('xb') as f: f.write(b)
    p.chmod(0o600)

assert not TREE.exists(), 'Fresh publication directory required'
os.umask(0o077)
prep_manifest = json.loads(read(PREP/'manifest.json'))
assert sha(read(PREP/'manifest.json')) == 'e1789442e5a84e2e068de8c91c1feaa797337faf428bf466eb4da2016cb5301d'
assert len(prep_manifest['members']) == 16
sources=[]
for item in prep_manifest['members']:
    p=PREP/item['path']; b=read(p)
    assert len(b)==item['bytes'] and sha(b)==item['sha256']
    sources.append((p,Path('preparation')/item['path']))
sources.append((PREP/'manifest.json',Path('preparation/manifest.json')))
journals=sorted((PREP/OP).glob('*.json'))
assert len(journals)==11
sources += [(p,Path('actual')/OP/p.name) for p in journals]
sources += [(PREP/WRAP/n,Path('actual')/WRAP/n) for n in ['coordination.json','wrapper.json','stdout.log','stderr.log']]
sources.append((BASE/'go-tool-restore-review-preserved/run-pre-settlement.py',Path('original/run-pre-settlement.py')))
sources += [(BASE/n,Path('reviews')/n) for n in ['go-tool-restore-v1-source-review.json','go-tool-restore-v1-watchdog-review-initial.json','go-tool-restore-v1-watchdog-review-final.json']]
sources.append((BASE/'historical-go-build-cache-clean-20261009.json',Path('separate/historical-go-build-cache-clean-20261009.json')))
assert len(sources)==37
bindings=[]
for original,relative in sources:
    b=read(original);write(CAP/relative,b)
    bindings.append({'originalPath':str(original),'publicationPath':relative.as_posix(),'bytes':len(b),'sha256':sha(b),'transformation':'none'})

final=json.loads(read(journals[-1])); wrapper=json.loads(read(PREP/WRAP/'wrapper.json'))
coord=json.loads(read(PREP/WRAP/'coordination.json')); cache=json.loads(read(BASE/'historical-go-build-cache-clean-20261009.json'))
assert final['passed'] is True and wrapper['passed'] is True and wrapper['exitCode']==0
assert not wrapper['forced'] and not wrapper['timedOut']
assert final['sourceBefore']==final['sourceAfter']
pins={Path(k).name:v for k,v in final['sourceBefore'].items()}
assert pins==coord['operationSources']
for n,h in pins.items(): assert sha(read(CAP/'preparation'/n))==h
assert wrapper['wrapperSHA256']==sha(read(CAP/'preparation/run.py'))
assert coord==final['coordination']
assert wrapper['command'][wrapper['command'].index('--quiescence-sha256')+1]==sha(read(PREP/WRAP/'coordination.json'))
for i,p in enumerate(journals,1):
    j=json.loads(read(p)); assert j['steps']==final['steps'][:i]
    assert j['sourceBefore']==final['sourceBefore'] and j['stage']==final['steps'][i-1]['stage']
assert cache['at']<final['startedAt']
assert final['beforeManifestBinding']['observedEqualsPinnedManifest'] is True
assert final['freeAfter']=={'/workspace':7352782848,'/tmp':2260738048}
external=final['beforeManifestBinding'].copy()
external.update({'statBytes':Path(external['path']).stat().st_size,'included':False,
                'independentlyRehashedForPublication':False,
                'scope':'Original historical manifest SHA and metadata are retained in exact operation receipts and pinned source. This publication did not read/hash the full manifest or Go tree.'})
write(CAP/'historical-manifest-reference.json',enc(external))
write(CAP/'original-bindings.json',enc({'version':1,'members':bindings,'allCopiedOriginalsUnchangedBeforeAfter':True,'omittedLargeOriginal':external}))

summary={
 'scope':'Publication-side small-file consistency audit; same publication author, not an independent live operation verifier.',
 'actualRestoration':{'startedAt':final['startedAt'],'finishedAt':final['steps'][-1]['at'],'passed':True,'wrapperExitCode':0,'timedOut':False,'forced':False,'journalCount':11,'sourceBeforeAfterEqual':True,'pinnedPreparationCount':16,'beforeManifest':final['beforeManifestBinding'],'freeBefore':final['freeBefore'],'freeAfter':final['freeAfter'],'freeDelta':{k:final['freeAfter'][k]-v for k,v in final['freeBefore'].items()},'logicalBytes':final['logicalBytes'],'finalWorkspaceRealDirectory':True,'tmpCompatibilityTarget':final['tmpCompatibilityTarget']},
 'separateEarlierCacheClean':{'at':cache['at'],'path':cache['GOCACHE'],'exitCode':cache['exitCode'],'freeBefore':cache['freeBefore'],'freeAfter':cache['freeAfter'],'freeDelta':{k:cache['freeAfter'][k]-v for k,v in cache['freeBefore'].items()}},
 'laterOwnerReportedObservation':{'/workspace':7352774656,'/tmp':2260738048,'scope':'Parent coordination message only; exact timestamp and separate machine receipt were not supplied. Never substituted for original final-operation observation.'},
 'limits':['No application, compiler, Docker, tool mutation or operation/control rerun by publication.','16,688 Go entries and bytes/mode/uid/gid/mtime equality are operation-recorded; historical 4,334,445-byte manifest and Go payload are excluded.','Exact accepted unknown process identities remain unknown; readable snapshots plus bounded live-root coordination are not whole-machine inactivity or future-launch exclusion.','420 seconds bounds process waits, not synchronous OS I/O; own leader exit is not a descendant census.','Root final floor 6,736,759,858 bytes was met in the receipt; future admission is not certified. TMP 2,260,738,048 bytes is below the separate 3,905,250,841-byte export floor.','No saved-runtime export, fresh environment, Windows, live provider or product acceptance follows.']}
write(CAP/'publication-checks.json',enc(summary))

readme='''# Required Go restoration — completed cloud operation

The exact root-run restoration completed on 2026-10-09 at 15:11:42Z. Its eleven numbered journals record the verified Go 1.26.3 tree becoming a real directory at `/workspace/.luci-tools/go1.26.3`, with `/tmp/luci-retained-tools/go1.26.3` retained as a compatibility symlink. The wrapper exited 0 without timeout or forced settlement. This is cloud storage preparation, not product or Windows acceptance.

`preparation/` retains all sixteen frozen files and their original manifest, including source, ten passing disposable-tree controls and the same three watchdog controls failing against the original draft then passing after correction. `original/` keeps that original watchdog unchanged. The read-only source reviews remain in `reviews/`; the initial blocker is preserved separately from the final scoped review. No controls were rerun for publication. `preparation/README.md` is the original preparation-time document and therefore still says "prepared only"; the actual later operation is independently separated under `actual/`.

All thirty-seven selected originals are copied byte-for-byte and bound in `original-bindings.json`. The three operation-source pins equal the frozen source, coordination record, and journal before/after maps; the wrapper pin and coordination-file hash also match. Every numbered journal is retained, including preliminary `passed=false` records before the final success. These checks verify receipt consistency, not an independent live observation of the filesystem operation.

The historical manifest is bound at SHA256 `50b4a84d580c4b072378c2cea371854fa750c29ef512c634b0282c365a57f242`: 16,688 entries and 232,365,810 logical bytes. Its 4,334,445-byte original and the Go tree are deliberately omitted. The exact operation receipt reports comparison of all bytes, mode, uid/gid and mtime against that manifest. The publisher did not independently rehash the large manifest/tree. Cross-filesystem inode, ctime and access time are not preserved or claimed.

The operation's free-space observations are `/workspace` 7,634,825,216 → 7,352,782,848 bytes (−282,042,368) and `/tmp` 1,985,839,104 → 2,260,738,048 (+274,898,944). These observed deltas differ from logical tree bytes and are not solely-attributable allocated-size measurements. A later root message reports 7,352,774,656 / 2,260,738,048; its 8 KiB root difference is kept separately as owner-reported without a supplied timestamp/receipt.

`separate/` preserves the distinct historical build-cache clean at 14:56:59Z, **before** restoration. It records only `/tmp/luci-retained-tools/go-build`, exit 0, unchanged root free bytes, and TMP 1,401,249,792 → 1,985,839,104 (+584,589,312). The receipt records command `go clean -cache` but does not serialize the resolved executable path. Do not conflate it with restoration, a generic cache purge, removal of module/toolchain/fixture/evidence data, or later application behavior.

Root's 6,736,759,858-byte runtime floor was met at the final operation observation. The 64 MiB working guard applies at pre/post-copy stages; final success checks the floor alone. TMP remained below the distinct 3,905,250,841-byte export floor by 1,644,512,793 bytes. This capsule does not prove image export readiness, future admission, saved setup, restored Go execution, or a new environment's readiness.

Readable exact exe/cwd/argv/maps/fd observations plus a source-bound live-owner receipt permit only three explicitly accepted unknown PID/start pairs. They remain unknown; this is not a global inactivity claim or exclusion of future launches. The watchdog's 400 + 10 + 10 second process waits exclude synchronous OS I/O and scheduling delays. Leader settlement is not a descendant census. No actual app/Docker/compiler operation or signal was performed by this publication.

For portable **read-only capsule integrity verification**, run `python3 -B verify.py` inside this directory. It hashes only published small files and validates copied-source/receipt relationships. It does not import or execute restoration, cleanup or test code. `assemble.py` records the original fixed-source copying procedure; it refuses to overwrite its publication tree and is not a deployment script. Never replay the completed restoration merely to check these receipts.
'''
write(CAP/'README.md',readme.encode())

verify='''"""Read-only published small-file/source receipt verification. No operation import."""
from pathlib import Path
import hashlib,json,stat
root=Path(__file__).resolve().parent
def sha(b): return hashlib.sha256(b).hexdigest()
def data(p):
    s=p.lstat(); assert stat.S_ISREG(s.st_mode) and s.st_size<=262144, str(p)
    return p.read_bytes()
manifest=json.loads(data(root/'publication.json')); count=0
for m in manifest['members']:
    p=root/m['path']; assert p.resolve().is_relative_to(root)
    b=data(p); assert len(b)==m['bytes'] and sha(b)==m['sha256'],m['path']; count+=1
expected={m['path'] for m in manifest['members']}|{'publication.json','SHA256SUMS'}
assert {p.relative_to(root).as_posix() for p in root.rglob('*') if p.is_file()}==expected
for line in data(root/'SHA256SUMS').decode().splitlines():
    h,n=line.split('  ',1); assert sha(data(root/n))==h,n;count+=1
for b in json.loads(data(root/'original-bindings.json'))['members']:
    raw=data(root/b['publicationPath']); assert len(raw)==b['bytes'] and sha(raw)==b['sha256'];count+=1
prep=json.loads(data(root/'preparation/manifest.json'))
for m in prep['members']:
    raw=data(root/'preparation'/m['path']); assert len(raw)==m['bytes'] and sha(raw)==m['sha256'];count+=1
final=json.loads(data(root/'actual/operation-304739566509590/011-finished.json'))
wrapper=json.loads(data(root/'actual/root-operation-304739566509590/wrapper.json'))
coordbytes=data(root/'actual/root-operation-304739566509590/coordination.json'); coord=json.loads(coordbytes)
assert final['sourceBefore']==final['sourceAfter']
for p,h in final['sourceBefore'].items(): assert sha(data(root/'preparation'/Path(p).name))==h;count+=1
assert coord==final['coordination']
assert coord['operationSources']=={Path(k).name:v for k,v in final['sourceBefore'].items()}
assert wrapper['wrapperSHA256']==sha(data(root/'preparation/run.py'))
assert wrapper['command'][wrapper['command'].index('--quiescence-sha256')+1]==sha(coordbytes)
assert final['passed'] and wrapper['passed'] and wrapper['exitCode']==0 and not wrapper['forced'] and not wrapper['timedOut']
for i,p in enumerate(sorted((root/'actual/operation-304739566509590').glob('*.json')),1):
    j=json.loads(data(p)); assert j['steps']==final['steps'][:i] and j['sourceBefore']==final['sourceBefore'];count+=1
print(json.dumps({'passed':True,'hashAndJournalChecks':count,'scope':'Published small-file and declared receipt/source bindings only; no operation/test or large-payload validation.'}))
'''
write(CAP/'verify.py',verify.encode())
write(CAP/'assemble.py',Path(__file__).read_bytes())
for original,relative in sources: assert read(original)==read(CAP/relative)
members=[]
for p in sorted(CAP.rglob('*')):
    if p.is_file():
        b=read(p);members.append({'path':p.relative_to(CAP).as_posix(),'bytes':len(b),'sha256':sha(b)})
write(CAP/'publication.json',enc({'version':1,'scope':'Completed exact cloud Go restoration receipts plus separate earlier cache clean and preserved source controls; no product acceptance.','members':members,'companion':'../2026-10-09-go-tool-restoration.md'}))
sums=members+[{'path':'publication.json','sha256':sha(read(CAP/'publication.json'))}]
write(CAP/'SHA256SUMS',''.join(f"{m['sha256']}  {m['path']}\n" for m in sums).encode())
companion=TREE/'docs/luci/evidence/2026-10-09-go-tool-restoration.md'
write(companion,b'''# Cloud Go tool restoration receipt

The [source-bound capsule](go-tool-restoration-20261009/README.md) retains the completed 2026-10-09 root operation, eleven journals, wrapper/coordination records, unchanged preparation sources, original watchdog defect and corrected controls, and independent read-only reviews. It reports Go as a real workspace tree with a compatible TMP alias; the large historical tree manifest remains hash-bound and is not republished.

The final operation observation was 7,352,782,848 bytes free on root and 2,260,738,048 on TMP. A later owner message reports 7,352,774,656 / 2,260,738,048; that is separately labelled without replacing original receipts. The historical build-cache clean happened earlier and has its own exact receipt.

This is cloud storage evidence, not application, Windows, compiler-execution or live-provider acceptance. Unknown process identities and watchdog/metadata limits remain explicit. Root met its recorded runtime floor; TMP remained below the separate image-export floor. Read-only publication checks do not rerun the operation or certify future admission.
''')
files=[]
for p in sorted(TREE.rglob('*')):
    if p.is_file():
        b=read(p);files.append({'path':p.relative_to(TREE).as_posix(),'bytes':len(b),'sha256':sha(b)})
plan={'version':1,'tree':str(TREE),'targetRepository':'/workspace/luci','scope':'Copy-only publication overlay; no repository edits made by assembler.','capsuleManifestSHA256':sha(read(CAP/'publication.json')),'files':files,'fileCount':len(files),'bytes':sum(f['bytes']for f in files),'originalBindings':len(bindings),'capsuleMembers':len(members),'checksumEntries':len(sums)}
write(OUT/'copy-plan.json',enc(plan))
print(json.dumps({k:v for k,v in plan.items() if k!='files'},indent=2))
