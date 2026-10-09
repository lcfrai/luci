from pathlib import Path
import subprocess,json,hashlib,gzip,re

commit='67a5b7c75ca6354d43085f5bb81ec53ee4291f34'
repo=Path('/workspace/luci')
src=repo/'artifacts/quality-swarm/candidate-67a5b7c7/package'
release=repo/'artifacts/releases/0.5.0'
base=Path('/workspace/.luci-tools/publish-package-67a5/docs/luci/evidence')
out=base/'package-67a5b7c7'
tools=Path('/workspace/.luci-tools')
sha=lambda b:hashlib.sha256(b).hexdigest()
required=['build-invocation.json','build-result.json','build.log','package-integrity.json','package-provenance.json','previous-release-preservation.json','compact-package-integrity.json']
assert all((src/name).is_file() for name in required), 'Incomplete original receipts'
build=json.loads((src/'build-result.json').read_text())
v=json.loads((src/'compact-package-integrity.json').read_text())
integrity=json.loads((src/'package-integrity.json').read_text())
provenance=json.loads((src/'package-provenance.json').read_text())
manifest=json.loads((release/'release-manifest.json').read_text())
preservation=json.loads((src/'previous-release-preservation.json').read_text())
assert build['sourceCommit']==commit and build['exitCode']==0 and not build['timedOut'] and build['cleanBefore'] and build['cleanAfter'] and build['sameHead']
assert v['status']=='passed' and v['sourceCommit']==commit and v['sourceDirty'] is False
assert integrity['sourceCommit']==commit and integrity['allMembersMatched'] and integrity['missingLicenses']==0
assert provenance['sourceCommit']==commit and provenance['sourceDirty'] is False and manifest['sourceCommit']==commit and manifest['sourceDirty'] is False
assert sha((src/'package-provenance.json').read_bytes())==v['originalProvenanceSHA256']
assert sha((release/'release-manifest.json').read_bytes())==v['releaseManifestSHA256']
assert preservation['oldSource']=='99007404a5ddd8592dee376ea00d99c6180d26fe' and preservation['originalProvenanceUnchanged'] is True
assert not out.exists(), 'Refusing to overwrite frozen capsule'
out.mkdir(parents=True)
bindings=[]

def put(data,target,origin,extra=None):
    p=out/target
    p.parent.mkdir(parents=True,exist_ok=True)
    p.write_bytes(data)
    row={'publishedPath':target,'originalPath':origin,'originalBytes':len(data),'originalSHA256':sha(data),'publishedBytes':len(data),'publishedSHA256':sha(data),'transformation':'none'}
    if extra:row.update(extra)
    bindings.append(row)

def cp(source,target):put(source.read_bytes(),target,str(source))

for name in required:cp(src/name,'receipts/'+name)
for name in ['build-cloud048051-candidate.py','verify-local-package.py','verify-compact-local-package.py']:
    cp(tools/name,'helpers/'+name)
cp(Path(__file__),'helpers/assemble-package-67a5.py')
for name in ['release-manifest.json','SHA256SUMS']:cp(release/name,'release/'+name)
for path in ['scripts/build-release.sh','scripts/stage-release-docs.py','scripts/package.py','scripts/collect-licenses.py','scripts/launch-unix.sh']:
    data=subprocess.check_output(['git','show',commit+':'+path],cwd=repo)
    put(data,'source/'+path,'git:'+commit+':'+path,{'sourceCommit':commit})
(out/'original-bindings.json').write_text(json.dumps({'scope':'Exact copied receipts, helpers and selected Git-source bytes only. Publication did not rehash the large archives or execute build, verifiers or runtime.','files':bindings},indent=2)+'\n')
artifacts={x['name']:x for x in v['artifacts']}
portable=artifacts['luci-linux-amd64.tar.gz']
cloud=artifacts['luci-cloud-linux-amd64.tar.gz']

(out/'README.md').write_text(f'''# Matching clean Linux package — 67a5b7c7

**Packaging verification passed for `{commit}`; integrated local-product acceptance remains open.** This capsule binds the clean build and package verification. It does not establish matching installed workload, responsiveness, Windows native, live-provider or remaining recovery acceptance. Exact source is retrievable on `codex/luci-production-runs`; the binaries are retained private cloud artifacts, not downloadable GitHub releases.

## Exact artifact identity

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | {portable['bytes']:,} | `{portable['sha256']}` |
| Embedded/separate `luci-cloud-linux-amd64.tar.gz` | {cloud['bytes']:,} | `{cloud['sha256']}` |

[Original provenance](receipts/package-provenance.json) records binary `{provenance['binarySHA256']}`, web index `{provenance['webIndexSHA256']}`, and extracted package location. Portable package identity is `{v['packageIdentity']}`. [Release manifest](release/release-manifest.json) SHA256 `{v['releaseManifestSHA256']}` records the same clean source. No binaries or complete staged guides were copied into this capsule.

## Build and verification

The [final build](receipts/build-result.json) records exit zero, **{build['elapsedSeconds']:.2f} seconds**, clean source before and after, unchanged HEAD, `GOMAXPROCS=2`, and `GOFLAGS=-mod=readonly -p=1`. The 600-second timeout did not fire. [Raw build output](receipts/build.log) retains warnings. [Original invocation](receipts/build-invocation.json) retains its historical `running` status; the final result supersedes that status. The frozen build wrapper requires an existing official remote-tracking ref containing the exact source. This run contains no failed-preflight receipt; the earlier 990 tracking failure belongs to its separate capsule and is not attributed to this build.

[Original member verification](receipts/package-integrity.json) records **{integrity['verifiedExtractedFiles']} extracted files**, **{integrity['licenseComponents']} licensed components**, and zero missing licences. [Additive verification](receipts/compact-package-integrity.json) checks **{v['verifiedPackageFiles']} portable / {v['verifiedCloudFiles']} cloud files**, **{v['verifiedRuntimeSourceFiles']} runtime source files**, **{v['verifiedGuideFiles']} staged guide/index files**, and **{v['newHostedURLsBoundToExactGitObjects']} new hosted URLs** against exact source objects and types. Complete archive/extraction equality including modes, embedded cloud equality, licence bytes and original-provenance preservation are checked.

Guide regeneration uses the **same committed staging implementation**; it is not independent Markdown rewriting proof. Separate inventory/hash/URL checks retain their stated scope. Offline source-object association is not live URL availability or heading-anchor verification. The build elapsed time is not application responsiveness, throughput or RAM evidence.

## Preservation and acceptance limits

[Previous-release preservation](receipts/previous-release-preservation.json) records the exact old **99007404** release artifacts and byte-preserving directory rename before this build. The old candidate, archive targets, original provenance and failed fixtures remain separate and unchanged according to that run-owner receipt. This publication task did not rename, delete or mutate release directories, failed fixtures or ledgers.

The new package includes the CLOUD-048 retained-retry guidance, CLOUD-050 deferred editor rendering and CLOUD-051 draft-dialog focus correction. The retrievable source also contains the CLOUD-049 phone acceptance-helper correction; that acceptance helper is not bundled in the portable runtime. The package receipt does not execute those UI workflows. The prior sustained run used **99007404 product / b81d6454 helper**, not this package: five scripted natural finishes and reviewed integrations remain scoped evidence in [its preserved capsule](../sustained-99007404-attempt7/README.md), whose overall result failed on the phone helper and responsiveness targets. Its two exited Chromium processes adopted by PID 1 remain explicit; no current-package runtime or whole-machine cleanup result is inferred.

[Original bindings](original-bindings.json) preserve all copied receipts/helpers and five selected exact packaging sources. This is **selected-source/package evidence, not a complete compiler-input capture**. [SHA256SUMS](SHA256SUMS) binds every capsule file except itself; the companion report is bound by publication.json. Standard task paths are retained for precise provenance. Publication read no private application-state, credential or failed-fixture files, and did not rebuild, run the verifiers, start Luci or run acceptance workloads.

[Reproduction](REPRODUCE.md) supplies exact retrieval and supported Linux/Windows build routes. This is a useful retrievable-source checkpoint, **not `LOCAL TEST HANDOFF READY` or completion of the local product**. Windows testing stays held while the tester's machine is below its declared memory gates.
''')

repro=(tools/'publish-package-990/docs/luci/evidence/package-99007404/REPRODUCE.md').read_text()
repro=repro.replace('99007404a5ddd8592dee376ea00d99c6180d26fe',commit)
repro=repro.replace('99007404','67a5b7c7').replace('the 990 source','the 67a5 source').replace('fresh-990-verification','fresh-67a5-verification')
repro=repro.replace('build-cloud046047-candidate.py','build-cloud048051-candidate.py')
repro=repro.replace('task-specific preservation/preflight paths','task-specific preservation paths and clean-source/remote-ref preconditions')
repro=repro.replace('Matching idle/sustained results are separate evidence and remain open at package publication. This capsule establishes package verification, not a four-project or local-release pass. The prior failed workloads remain preserved.', 'Matching 67a5 idle/sustained results remain open at package publication. The previous installed sustained attempt used product 99007404 and helper b81d6454, so it is not this package\'s runtime verification. This capsule establishes package verification, not a four-project or local-release pass. All prior failed workloads and their cleanup limits remain preserved. No setup/configuration change is required by CLOUD-048/049/050/051; Windows acceptance must still use fresh isolated directories and wait for its established resource gates.')
(out/'REPRODUCE.md').write_text(repro)

patterns=[re.compile(rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),re.compile(rb'gh[pousr]_[A-Za-z0-9]{30,}'),re.compile(rb'AKIA[0-9A-Z]{16}')]
findings=[]
scanned=0
for p in out.rglob('*'):
    if not p.is_file():continue
    data=p.read_bytes()
    if p.suffix=='.gz':data=gzip.decompress(data)
    scanned+=1
    for i,pattern in enumerate(patterns):
        if pattern.search(data):findings.append({'path':str(p.relative_to(out)),'pattern':i})
assert not findings,findings
(out/'privacy-scan.json').write_text(json.dumps({'scope':'Selected private-key/access-token formats in public candidate and decoded gzip only; not a universal privacy scan. Standard task paths intentionally retained. No private application/credential files read.','scannedFiles':scanned,'findings':findings},indent=2)+'\n')

companion=base/'2026-10-09-package-67a5b7c7.md'
companion.write_text(f'''# Clean matching Linux package at 67a5b7c7

Exact source `{commit}` builds cleanly and passes the original and additive package checks: **{v['verifiedPackageFiles']} portable files / {v['licenseComponents']} licensed components / {v['verifiedGuideFiles']} staged guide/index files / {v['newHostedURLsBoundToExactGitObjects']} new source-bound hosted URLs**. Portable archive: **{portable['bytes']:,} bytes**, SHA256 `{portable['sha256']}`.

[The capsule](package-67a5b7c7/README.md) preserves exact raw build/verification receipts, frozen helpers, five selected packaging sources, and byte-preserving old-990-release relocation. This build has no failed-preflight receipt. Guide regeneration shares the committed implementation; independent checks retain their narrower scope. Recorded clean build time is {build['elapsedSeconds']:.2f} seconds, not an app-performance measurement.

No binary is publicly downloadable through this receipt; [exact source/build instructions](package-67a5b7c7/REPRODUCE.md) are retrievable. Archives remain cloud-private. This is package evidence only: matching installed UI/workload, Windows native, live-model quality, responsiveness and remaining recovery acceptance stay open. The prior sustained run's product990/helperb81 and five scripted finishes remain separate; its overall failure and two PID-1 Chromium zombies are not erased by this build. There are no setup/configuration changes for CLOUD-048/049/050/051, and Windows runtime/build work stays behind the tester's resource gates.
''')
members=[{'path':str(p.relative_to(out)),'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())} for p in sorted(out.rglob('*')) if p.is_file()]
pub={'sourceCommit':commit,'sourceDirty':False,'scope':'Recorded clean Linux package verification only; no installed workload/native/latency/recovery pass inferred.','portableArchive':portable,'cloudArchive':cloud,'packageIdentity':v['packageIdentity'],'verifiedPackageFiles':v['verifiedPackageFiles'],'licenseComponents':v['licenseComponents'],'verifiedGuideFiles':v['verifiedGuideFiles'],'newHostedURLsBoundToExactGitObjects':v['newHostedURLsBoundToExactGitObjects'],'companionReport':{'path':companion.name,'bytes':companion.stat().st_size,'sha256':sha(companion.read_bytes())},'members':members,'manifestConvention':'Members exclude publication.json and SHA256SUMS; SHA256SUMS also binds publication.json; companion report bound separately.'}
(out/'publication.json').write_text(json.dumps(pub,indent=2)+'\n')
files=[p for p in sorted(out.rglob('*')) if p.is_file()]
(out/'SHA256SUMS').write_text(''.join(f'{sha(p.read_bytes())}  {p.relative_to(out)}\n' for p in files))
summary={'path':str(out),'manifestMembers':len(members),'sumEntries':len(files),'files':len(files)+1,'originalBindings':len(bindings),'publicationSHA256':sha((out/'publication.json').read_bytes()),'sumsSHA256':sha((out/'SHA256SUMS').read_bytes()),'companionSHA256':sha(companion.read_bytes()),'portableArchive':portable,'guideFiles':v['verifiedGuideFiles'],'scope':'Only external publication files written; no repository/runtime changes'}
(tools/'publish-package-67a5-result.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
