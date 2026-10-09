from pathlib import Path
import datetime, hashlib, json

BASE = Path('/workspace/.luci-tools')
HERE = BASE/'publish-cloud060-copy-retirement'
PLAN = BASE/'cloud060-copy-retirement-plan'
RUN = BASE/'cloud060-copy-retirement-operation-20261009-root1'
TREE = HERE/'tree'
DOCS = TREE/'docs/luci/evidence'
NAME = 'cloud060-generated-copy-retirement-20261009'
OUT = DOCS/NAME
PRODUCT = '5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
SCRIPT_SHA = 'b50fdde1a24745da33b72c970bf0cc606065b095994a17c710acbbc3acfa39a4'
PROOF_SHA = 'cbab4651da95badd8f04ea40153587ae92790d3b2ac831a8a197de2b0903bce4'
sha = lambda b: hashlib.sha256(b).hexdigest()
def read(p):
    assert p.is_file() and not p.is_symlink() and p.stat().st_size < 2*1024*1024, str(p)
    return p.read_bytes()
def obj(p): return json.loads(read(p))
def write(p, value): p.write_text(json.dumps(value, indent=2)+'\n')
assert not TREE.exists(), 'Do not overwrite an existing publication overlay'
OUT.mkdir(parents=True)
originals = []
def copy(source, relative):
    b = read(source); p = OUT/relative; p.parent.mkdir(parents=True,exist_ok=True); p.write_bytes(b)
    originals.append({'source':str(source),'publishedPath':relative,'sourceSHA256':sha(b),'sourceBytes':len(b),
                      'sha256':sha(b),'bytes':len(b),'transformation':'none'})

prepared = obj(PLAN/'manifest.json')
assert len(prepared['files']) == 16 and prepared['preparationOnly'] is True and prepared['realRetirementExecuted'] is False
for row in prepared['files']:
    p=PLAN/row['path']; b=read(p); assert sha(b)==row['sha256'] and len(b)==row['bytes']
    copy(p,'preparation/'+row['path'])
copy(PLAN/'manifest.json','preparation/manifest.json')
assert sha(read(PLAN/'retire.py'))==SCRIPT_SHA and sha(read(PLAN/'proof.json'))==PROOF_SHA
proof = obj(PLAN/'proof.json')
names=['cloud060-package-reuse-5f4a966f','cloud060-package-reuse-v2-5f4a966f','cloud060-package-reuse-v3-5f4a966f']
assert [p['root'] for p in proof['copies']]==[str(BASE/n)for n in names]
assert proof['readOnly'] is True and proof['passed'] is True and proof['sourceCommit']==PRODUCT
reference = proof['retainedReference']['outputs']
assert len(reference)==425 and 'luci'in reference
assert all(n=='luci' or n.startswith('web/')for n in reference)
for c in proof['copies']:
    assert set(c['outputs'])==set(reference)
    assert all((v['sha256'],v['bytes'])==(reference[n]['sha256'],reference[n]['bytes'])for n,v in c['outputs'].items())
archive_members = proof['retainedArchive']['selectedMembers']
assert set(archive_members)==set(reference)
assert all((v['sha256'],v['bytes'])==(reference[n]['sha256'],reference[n]['bytes'])for n,v in archive_members.items())
journal = sorted(RUN.glob('*.json'))
assert len(journal)==28 and [int(p.name[:3])for p in journal]==list(range(1,29))
for p in journal:copy(p,'operation/'+p.name)
intent=obj(journal[0]); before_unlink=obj(journal[14]); final=obj(journal[-1]); preflight=obj(journal[1])
expected_links=[{'path':str(BASE/n/k),'target':str(Path(proof['retainedReference']['root'])/k)}for n in names for k in ['luci','web']]
assert intent['sourceSHA256']==SCRIPT_SHA and intent['proofSHA256']==PROOF_SHA
assert intent['onlyPayloadRoots']==[r['path']for r in expected_links]
assert before_unlink['physicalOriginalsStillPresent'] is True and before_unlink['count']==6
assert final['passed'] is True and final['links']==expected_links
assert final['freeBefore']=={'/workspace':6360612864,'/tmp':2387849216}
assert final['freeAfter']=={'/workspace':6609715200,'/tmp':2386538496}
assert final['measuredRootFreeDelta']==final['freeAfter']['/workspace']-final['freeBefore']['/workspace']==249102336
assert final['candidateAllocatedBytes']==proof['candidateAllocatedBytes']==260227072
assert len(preflight['processObservation']['acceptedUnknownIdentities'])==3
for i in range(3,15,2):
    planned=obj(journal[i-1]); done=obj(journal[i])
    assert (planned['original'],planned['holding'],planned['target'])==(done['original'],done['holding'],done['target'])
for i in range(16,28,2):
    planned=obj(journal[i-1]); done=obj(journal[i])
    assert done['holding']==planned['holding'] and done['absence'] is True and planned['manifestSHA256']==PROOF_SHA
for name in ['cloud060-generated-copy-storage-review.json','cloud060-generated-copy-storage-review.md']:
    copy(BASE/name,'prior-readonly/'+name)
copy(Path(__file__),'repro/assemble.py')
assert obj(PLAN/'failure-controls-original.json')['failed']==3
assert obj(PLAN/'failure-controls-fixed.json')['passed']==3
assert obj(PLAN/'controls-result.json')['passed'] is True and obj(PLAN/'controls-result.json')['cases']==7
summary={'productCommit':PRODUCT,'operationPassed':True,'preparationManifestSHA256':sha(read(PLAN/'manifest.json')),
         'proofSHA256':PROOF_SHA,'retirementScriptSHA256':SCRIPT_SHA,'journalFiles':28,'copies':3,'selectedFilesPerCopy':425,
         'compatibilityLinks':6,'recordedRootFreeBefore':6360612864,'recordedRootFreeAfter':6609715200,
         'recordedRootFreeDelta':249102336,'recordedTmpFreeBefore':2387849216,'recordedTmpFreeAfter':2386538496,
         'recordedTmpFreeDelta':-1310720,'recordedCandidateAllocatedBytes':260227072,'recordedCandidateLogicalBytes':257436648,
         'recordedDistinctFixedControls':10,'originalAdversarialFailures':3,
         'scope':'Retained run-owner payload-retirement proof and journals. No repeated payload/archive hashing, process scan, operation, tests, runtime or admission check at publication.',
         'admissionScope':'6,609,715,200 free root bytes are below root-reported approximately 6.736 GB Create floor. No exact refusal operands or new admission decision are inferred.',
         'preservationScope':'Original preparation files retain their historical not-yet-executed labels. V4 and fixtures are outside allowed mutation paths; no full v4/private-tree byte audit is claimed.',
         'originalOverlayUnchangedSHA256':'0e46f6a9dbc49912a8782ea28663d7cb24aca39899151ae182f4835d527f5ce9'}
write(OUT/'analysis.json',summary)
(OUT/'README.md').write_text('''# C060 generated-copy retirement: completed maintenance only

Root's **2026-10-09** operation records successful retirement of three exact duplicate generated `luci`/`web` payloads at product `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`. [The final journal](operation/028-final.json) is separate from the frozen preparation manifest, whose `preparationOnly`/`realRetirementExecuted: false` and README describe its earlier preparation stage. No original receipt has been edited to erase that chronology.

## Exact operation and scope

The three roots are `cloud060-package-reuse-5f4a966f`, `cloud060-package-reuse-v2-5f4a966f`, and `cloud060-package-reuse-v3-5f4a966f` under `/workspace/.luci-tools`. Each selects **425 files**: the service binary and 424 web files. [The complete proof](preparation/proof.json) retains every selected reference/archive/copy hash and size, generated-copy file/directory metadata, and nonpayload sibling metadata. It is a record of root's pre-operation verification; this publication did not repeat binary/archive hashing or read private application state.

The final script is `b50fdde1a24745da33b72c970bf0cc606065b095994a17c710acbbc3acfa39a4`; proof is `cbab4651da95badd8f04ea40153587ae92790d3b2ac831a8a197de2b0903bce4`. All **28 actual journal files** are retained, including intent, byte-verification checkpoint, six link intentions/completions, the all-links-before-unlink checkpoint, six unlink intentions/completions, and final status. Compatibility links now point to the retained exact `/tmp/luci-retained-tools/package-0.5.0-5f4a966f/{luci,web}` bytes. Physical duplicate inode identity is not preserved; reconstruction can preserve bytes and recorded metadata, not recover the original inode.

Only those six payload paths were eligible. Receipts, cache/home/tmp siblings, helpers, fixture state/ledgers/evidence, the reference/archive and **v4** were excluded. The script rechecks the build receipts' bytes and immediate nonpayload sibling metadata; this is not recursive content verification of all sibling trees. The final preservation statements are run-owner assertions supported by the narrow source/journal/proof, not a universal private-tree or v4 byte census. No Docker/image/cache mutation, application acceptance or product reservation release is credited to this storage operation.

## Storage values and remaining gate

| Observation | Before, bytes | After, bytes | Difference, bytes |
| --- | ---: | ---: | ---: |
| `/workspace` available | 6,360,612,864 | 6,609,715,200 | +249,102,336 |
| `/tmp` available | 2,387,849,216 | 2,386,538,496 | -1,310,720 |

The **260,227,072 candidate allocated bytes** and **257,436,648 logical duplicate bytes** are different measurements from the observed available-space change. Directory/filesystem activity and concurrent accounting prevent substituting one for another or claiming all measured change is uniquely attributable. The final root value remains below root's stated approximately **6.736 GB Create floor**. No new readiness/admission decision, workload outcome, latency/RAM improvement or acceptance pass follows. These are historical values, not promised current headroom.

## Preserved failure and validation evidence

The original `else424` syntax error is retained and did not execute. Initial seven controls passed, after which independent source review found two gaps: untracked partial link transitions and loss of a known busy reference after a later permission failure. Three added controls fail against the original source and pass against the corrected source; the corrected existing seven also pass, giving **10 distinct controlled cases**, not a cumulative count of repeated runs. [Original review](preparation/source-review-v1.json) and [final review](preparation/source-review-final.json) retain their exact source bindings. These small disposable fixtures are separate from the actual root operation.

The script's 180-second deadline is cooperative between I/O operations; the documented outer timeout handles stalled I/O. The journal confirms root coordination and accepts three exact unreadable PID/start identities. The process check covers readable cmdline/exe/cwd only, with no environment/fd/maps census or global/future-launch guarantee. Controlled success does not erase these limits. Failure after unlink begins must remain partial/uncertain; no implicit replay or original-inode restoration is supported.

The capsule contains only prepared sources, inert-control receipts, full public metadata proof, prior read-only review, actual journals and publication material. No binaries, archive bytes, fixture state or private logs are included. Standard task paths and PID/start identities are intentionally retained. [Original bindings](original-bindings.json) and [checksums](SHA256SUMS) bind the exact copies. The earlier 293-file package/terminal overlay remains separate and unchanged. This supplement is maintenance evidence, not `LOCAL TEST HANDOFF READY`.
''')
(OUT/'REPRODUCE.md').write_text('''# Review and bounded reproduction

Verify the publication checksums from this capsule directory with `sha256sum -c SHA256SUMS`. `preparation/manifest.json` lists its own earlier 16 prepared members; compare their hashes without rewriting that manifest. `original-bindings.json` maps exact copied originals. `analysis.json` separates original preparation, controlled fixtures and actual journals.

The recorded one-time retirement must **not** be replayed against the same paths: they now contain compatibility links, and its preconditions intentionally no longer describe their original inode state. The old preparation README contains a historical root invocation, not an instruction to run it again. Do not invoke `retire.py` or `verify.py` merely to review this publication.

To reproduce only the tiny controls, first copy `preparation` into a new disposable directory outside the capsule so generated control receipts cannot overwrite evidence. In that copy run `python3 -B controls.py`, then `python3 -B failure-controls.py retire.py fixed-rerun.json`. Both use tiny temporary files/fake observations, not the real candidate roots. For the original adversarial demonstration use `python3 -B failure-controls.py source-v1/retire.py original-rerun.json`; exit 1 and three failures are expected. Original receipts must remain unchanged, and new results must be labelled new controlled runs. No controls were rerun when this supplement was assembled.

Source and reference/archive identities remain in the original proof. Byte reconstruction after retirement is a separately reviewed action requiring those retained exact bytes, expected paths, modes and consent; it is not supplied or executed by this publication.
''')
write(OUT/'original-bindings.json',{'scope':'Exact copies only; no redaction or reconstruction. Original task paths are retained. No private state/log or binary/archive payload copied.','files':originals})
assert all(sha(read(Path(r['source'])))==r['sourceSHA256'] for r in originals)
for p in OUT.rglob('*'):
    if p.is_file():
        n=p.relative_to(OUT).as_posix().lower()
        assert not any(t in n for t in ['private-controller.stderr','private-transport-errors','/fixture/','/state/','credentials'])
members=[{'path':str(p.relative_to(OUT)),'sha256':sha(read(p)),'bytes':p.stat().st_size}for p in sorted(OUT.rglob('*'))if p.is_file()]
write(OUT/'publication.json',{'version':1,'productSource':PRODUCT,'scope':'Historical completed storage maintenance, source and controlled evidence only; no acceptance/admission pass.','members':members})
files=[p for p in sorted(OUT.rglob('*'))if p.is_file()]
(OUT/'SHA256SUMS').write_text(''.join(sha(read(p))+'  '+str(p.relative_to(OUT))+'\n'for p in files))
companion=DOCS/'2026-10-09-cloud060-generated-copy-retirement.md'
companion.write_text(f'''# C060 exact generated-copy retirement

Root's retained [maintenance capsule]({NAME}/README.md) records retirement of three generated binary/web copies at clean product `{PRODUCT}`. All 425 selected files per copy remain reconstructible from the retained exact package/archive, with six compatibility links. V4, receipts, private fixtures/ledgers, cache/home/tmp siblings and reference/archive bytes are outside the mutation target. The original duplicate inode identities are not preserved.

The 28 journals finish with passed=true. Recorded root free space changes **6,360,612,864 → 6,609,715,200 bytes** (**+249,102,336**); tmp changes **2,387,849,216 → 2,386,538,496 bytes**. Planned allocated bytes **260,227,072** remain a separate measurement. The final root value is still below the stated approximately 6.736 GB Create floor; no admission success or later outcome is inferred.

The exact full metadata proof, initial syntax failure, original three adversarial failures, corrected 10 distinct inert controls, original/final reviews and source are retained. Preparation's earlier “not executed” labels remain unchanged and separate from actual operation evidence. Process observation and cooperative I/O limits remain explicit. This supplement does not rerun payload hashes, tests or operations and does not modify the prior 293-file package/terminal publication overlay.

Capsule publication SHA256 `{sha(read(OUT/'publication.json'))}`. This is bounded storage-maintenance evidence, not an installed workflow, product resource-release, Windows/provider/performance or final local acceptance pass.
''')
copy_rows=[{'source':str(p),'destination':str(p.relative_to(TREE)),'sha256':sha(read(p)),'bytes':p.stat().st_size}for p in sorted(TREE.rglob('*'))if p.is_file()]
for row in copy_rows:
    target=Path('/workspace/luci')/row['destination'];assert not target.exists()and not target.is_symlink(), 'Repository target collision'
write(HERE/'copy-plan.json',{'version':1,'sourceProduct':PRODUCT,'externalTree':str(TREE),'repository':'/workspace/luci','action':'Copy only listed new publication files after root review; no repo writes by assembler.','files':copy_rows,'totals':{'files':len(copy_rows),'bytes':sum(r['bytes']for r in copy_rows)},'capsule':{'name':NAME,'publicationSHA256':sha(read(OUT/'publication.json')),'members':len(members),'sumEntries':len(files),'originalBindings':len(originals)},'priorOverlayPreserved':{'path':str(BASE/'publish-5f4-package-terminal-overlay/copy-plan.json'),'sha256':sha(read(BASE/'publish-5f4-package-terminal-overlay/copy-plan.json'))}})
print(json.dumps({'tree':str(TREE),'capsule':str(OUT),'publicationSHA256':sha(read(OUT/'publication.json')),'copyPlanSHA256':sha(read(HERE/'copy-plan.json')),'members':len(members),'sumEntries':len(files),'originalBindings':len(originals),'overlayFiles':len(copy_rows),'overlayBytes':sum(r['bytes']for r in copy_rows)},indent=2))
