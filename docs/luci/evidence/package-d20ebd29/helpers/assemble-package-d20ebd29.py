from pathlib import Path
import subprocess, json, hashlib, re

commit = 'd20ebd292f3f1de04d6726bab0998c210987bcb5'
previous = '67a5b7c75ca6354d43085f5bb81ec53ee4291f34'
repo = Path('/workspace/luci')
tools = Path('/workspace/.luci-tools')
src = repo / 'artifacts/quality-swarm/candidate-d20ebd29/package'
release = repo / 'artifacts/releases/0.5.0'
stage = tools / 'publish-package-d20ebd29/docs/luci'
out = stage / 'evidence/package-d20ebd29'
sha = lambda data: hashlib.sha256(data).hexdigest()
read = lambda path: json.loads(path.read_text())
names = ['build-invocation.json', 'build-result.json', 'build.log', 'package-integrity.json', 'package-provenance.json', 'previous-release-preservation.json', 'compact-package-verification.json']
assert all((src / name).is_file() for name in names)
build, integrity, provenance, compact, manifest, preservation = [read(path) for path in [src/'build-result.json', src/'package-integrity.json', src/'package-provenance.json', src/'compact-package-verification.json', release/'release-manifest.json', src/'previous-release-preservation.json']]
preflight = read(tools/'build-cloud052-preflight-attempt1.json')
relocation = read(tools/'archive-preservation-d20ebd29.json')
assert build['sourceCommit'] == commit and build['exitCode'] == 0 and not build['timedOut'] and build['cleanBefore'] and build['cleanAfter'] and build['sameHead']
assert all(value['sourceCommit'] == commit and value['sourceDirty'] is False for value in [integrity, provenance, compact, manifest])
assert compact['status'] == 'passed' and integrity['allMembersMatched'] and integrity['missingLicenses'] == 0
assert compact['originalProvenanceSHA256'] == sha((src/'package-provenance.json').read_bytes())
assert compact['releaseManifestSHA256'] == sha((release/'release-manifest.json').read_bytes())
assert compact['artifacts'] == manifest['artifacts']
assert preservation['oldSource'] == previous and preservation['originalProvenanceUnchanged']
assert preflight['sourceCommit'] == commit and preflight['status'] == 'failed-before-build'
assert preflight['wrapperSHA256'] == sha((tools/'build-cloud052-candidate.py').read_bytes())
assert relocation['sourceCommit'] == commit
artifacts = {row['name']: row for row in compact['artifacts']}
for member in relocation['members']:
    expected = artifacts[Path(member['originalPath']).name]
    assert member['compatibilityLink'] and all(member[key] == expected[key] for key in ['sha256', 'bytes'])
assert not out.exists(), 'Do not overwrite a frozen publication'
out.mkdir(parents=True)
bindings = []

def put(data, target, origin, **extra):
    path = out / target
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)
    bindings.append(dict(publishedPath=target, originalPath=origin, originalBytes=len(data), originalSHA256=sha(data), publishedBytes=len(data), publishedSHA256=sha(data), transformation='none', **extra))

def copy(source, target):
    put(source.read_bytes(), target, str(source))

for name in names:
    copy(src/name, 'receipts/'+name)
for name in ['build-cloud052-preflight-attempt1.json', 'archive-preservation-d20ebd29.json']:
    copy(tools/name, 'receipts/'+name)
for name in ['build-cloud052-candidate.py', 'verify-local-package.py', 'verify-compact-local-package.py']:
    copy(tools/name, 'helpers/'+name)
copy(Path(__file__), 'helpers/assemble-package-d20ebd29.py')
for name in ['release-manifest.json', 'SHA256SUMS']:
    copy(release/name, 'release/'+name)
for path in ['scripts/build-release.sh', 'scripts/stage-release-docs.py', 'scripts/package.py', 'scripts/collect-licenses.py', 'scripts/launch-unix.sh']:
    data = subprocess.check_output(['git', 'show', commit+':'+path], cwd=repo)
    put(data, 'source/'+path, 'git:'+commit+':'+path, sourceCommit=commit)
(out/'original-bindings.json').write_text(json.dumps({'scope': 'Exact copied receipts, current frozen helpers and five selected committed packaging sources. Publication does not rehash archives or run the build/verifiers/runtime.', 'files': bindings}, indent=2)+'\n')
portable = artifacts['luci-linux-amd64.tar.gz']
cloud = artifacts['luci-cloud-linux-amd64.tar.gz']

(out/'README.md').write_text(f'''# Matching clean Linux package — d20ebd29

**Clean packaging verification passed for `{commit}`; the complete local product remains unaccepted.** Exact source is retrievable on `codex/luci-production-runs`. Archive bytes remain private cloud artifacts, not downloadable Windows or GitHub release assets. This is not `LOCAL TEST HANDOFF READY`.

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | {portable['bytes']:,} | `{portable['sha256']}` |
| Embedded/separate `luci-cloud-linux-amd64.tar.gz` | {cloud['bytes']:,} | `{cloud['sha256']}` |

The [original provenance](receipts/package-provenance.json) binds binary `{provenance['binarySHA256']}`, web index `{provenance['webIndexSHA256']}` and extracted package identity. The additive package identity is `{compact['packageIdentity']}`. [Release manifest](release/release-manifest.json) SHA256 `{compact['releaseManifestSHA256']}` records the same source with `sourceDirty=false` and `nativeRequested=false`.

## Build and retained preflight failure

[Final build](receipts/build-result.json): exit 0 in **{build['elapsedSeconds']:.2f} seconds**, clean before/after, unchanged HEAD, `GOMAXPROCS=2`, `GOFLAGS=-mod=readonly -p=1`; the 600-second timeout did not fire. [Raw build output](receipts/build.log) retains warnings. The [invocation](receipts/build-invocation.json) retains its historical `running` status; the final result supersedes it.

The [initial preflight refusal](receipts/build-cloud052-preflight-attempt1.json) belongs to this exact source. A narrow fetch updated `FETCH_HEAD` without updating an official remote-tracking ref containing the commit. The unchanged wrapper correctly refused before build/output creation or package mutation. An actual fetch to the official tracking ref corrected that prerequisite; no synthetic ref or weakened clean-source requirement was used. Its recorded wrapper SHA matches the [frozen wrapper](helpers/build-cloud052-candidate.py). This retained failure is separate from the successful build.

## Verification scope

[Original member verification](receipts/package-integrity.json) reports **{integrity['verifiedExtractedFiles']} extracted files**, **{integrity['licenseComponents']} licensed components**, no missing licences and matching archive members. [Additive verification](receipts/compact-package-verification.json) checks **{compact['verifiedPackageFiles']} portable / {compact['verifiedCloudFiles']} cloud files**, **{compact['verifiedRuntimeSourceFiles']} runtime-source files**, **{compact['verifiedGuideFiles']} staged guide/index files**, and **{compact['newHostedURLsBoundToExactGitObjects']} new hosted URLs** against exact source objects/types. The staging implementation separately reports **{compact['stagingResult']['rewrittenLinks']} rewritten links**; these are different counts. Archive/extraction equality including modes, embedded-cloud equality, licence bytes and original-provenance preservation are checked by that recorded verifier.

Guide regeneration uses the same committed staging implementation; it is not independent Markdown rewriting proof. Inventory/hash/object checks retain their stated scope. Offline URL/object binding does not prove live URL or heading-anchor availability. This publication independently binds the retained receipts and selected source, **not the large private archive bytes**; it did not rerun either verifier or build. Build duration is not app responsiveness, throughput or RAM evidence.

## Preservation and resource tradeoff

[Previous-release preservation](receipts/previous-release-preservation.json) records a byte-preserving rename of the old **67a5b7c7** release directory before this build. Old package/extraction/provenance and failed fixtures remain separate according to that run-owner receipt.

The [new-archive preservation receipt](receipts/archive-preservation-d20ebd29.json) records verified copies of the two newly generated archives to retained `/tmp` paths, with compatibility links at their original release paths and unchanged hashes/lengths. The original manifests, receipts and failed fixtures were not altered. `/tmp` is memory-backed here: moving archives changes disk-versus-memory pressure; it is **not a total-resource gain or performance result**. This publication copied the receipt only and did not move/delete archives or inspect private failed fixtures.

## What is and is not ready

This product includes **CLOUD-052**: canceled overview waiters can leave the bounded cache-fill gate, and scheduler display reads retain their request context through the bounded reservation observation. Source exclusion, fresh authority, cache/file limits, capacity charges and worker/mutation lifecycles remain protected. [Final combined source validation](../cloud052-final-validation-20261009/README.md) records 31 top-level + 18 subtest race outcomes; the separate all-package compile command executed zero tests. Neither is a full-suite pass or installed cancellation/Windows result.

The previous [67a5 installed idle diagnostic](../cloud032-idle-67a5b7c7-20261009/README.md) failed both primary p95 targets: switch 519.762309 ms against 300 ms, input 114.250714 ms against 100 ms. Its 400 successful actions mean capture completion, not performance acceptance. This new package has no matching installed responsiveness or sustained result at publication. Earlier five scripted task finishes/integrations belong to product 99007404; that sustained run's phone/latency failure and cleanup limits remain preserved separately. No live-model quality, Windows native, whole-computer responsiveness or remaining recovery acceptance is inferred.

There are **no new configuration or secret names**. [Reproduction](REPRODUCE.md) and the [source checkpoint](../../TEST-CHECKPOINT-D20EBD29.md) use fresh isolated state/projects and the existing Windows resource gates. No local machine process was started by publication.

[Original bindings](original-bindings.json) preserve copied receipts/helpers and five selected packaging sources. This is selected-source/package evidence, not a complete compiler-input capture. `publication.json` binds all members and the companion/checkpoint; `SHA256SUMS` additionally binds that manifest. Standard task paths are retained. No private credentials or application-state files were read.
''')

repro = (tools/'publish-package-67a5/docs/luci/evidence/package-67a5b7c7/REPRODUCE.md').read_text()
repro = repro.replace(previous, commit).replace('67a5b7c7', 'd20ebd29').replace('the 67a5 source', 'the d20 source').replace('fresh-67a5-verification', 'fresh-d20-verification').replace('build-cloud048051-candidate.py', 'build-cloud052-candidate.py').replace('compact-package-integrity.json', 'compact-package-verification.json')
start = repro.index('Matching 67a5 idle/sustained results')
end = repro.index('\n\nThe exact build wrapper', start)
repro = repro[:start] + 'Matching d20ebd29 installed cancellation, idle/sustained responsiveness, Windows and live-provider results remain open. Previous 67a5 idle timing failed both targets; the previous sustained result used product 99007404/helper b81d6454 and also remains failed overall. This package verification does not supersede those results. CLOUD-052 introduces no setup/configuration or secret names. Keep fresh isolated directories and the tester\'s established resource gates.' + repro[end:]
(out/'REPRODUCE.md').write_text(repro)

checkpoint = stage/'TEST-CHECKPOINT-D20EBD29.md'
checkpoint.write_text(f'''# Windows source test checkpoint — d20ebd29

**Retrievable source checkpoint; full local acceptance is still open.** Exact source: [`{commit}`](https://github.com/lcfrai/luci/commit/{commit}), branch `codex/luci-production-runs`. [Matching clean Linux package](evidence/package-d20ebd29/README.md) passed original and additive checks: **956 portable files, 347 licensed components, 47 staged guides/index files and 251 source-bound hosted URLs**. The separate staging count is 250 rewritten links. Linux binaries remain private cloud artifacts; no matching Windows binary or native pass is claimed.

## Delta and evidence

CLOUD-052 stops obsolete Home display work: canceled overview waiters release their source admission instead of waiting for another project's fill, and scheduler display reads pass request cancellation through to bounded reservation observation. An initiating reader owns its fill; a live waiter can start a fresh read after it stops. Existing ordinary mutex waits and synchronous bounded reads remain cooperative, not forcibly preempted. Authority, source exclusion, cache limits, unknown capacity and exact held charges remain intact. [Final combined source check](evidence/cloud052-final-validation-20261009/README.md): 31 top-level +18 subtests pass under race; the subsequent all-control-plane compile check executes zero tests. This is not a full suite, native or latency pass.

There are **no setup/configuration/secret-name changes** from the [67a5 guide](TEST-CHECKPOINT-67A5B7C7.md). Keep its four-project, responsive navigation, draft/focus, large-context, reviewed retry, recovery and resource procedures, substituting this exact source revision. Do not carry older native results forward as newly executed results.

## Exact fresh retrieval, only after local resource admission

Windows testing remains governed by the originating tester's fresh memory/commit checks. This publication did not probe Windows headroom and does not request a launch while its hold persists. Retain the 2 GiB source and 2.5 GiB browser/native thresholds, task-local Node24.19/npm10.9.3/Go1.26.3, E: caches/temp, normal installed state/projects and unrelated Docker workloads. Never bypass the disposable-runner guard or run exclusive Linux helpers against shared Docker Desktop.

Use the 67a5 guide's checked PowerShell retrieval sequence with these values instead:

```powershell
$candidate = '{commit}'
$checkout = Join-Path 'E:\\Luci-local-acceptance\\checkouts' ('d20ebd29-' + [guid]::NewGuid().ToString('N'))
```

Clone into that new directory, fetch the real branch, detach at `$candidate`, verify exact clean HEAD, then run locked `npm ci` in `desktop` with the existing task-local tool activation. Do not overwrite a prior checkout or reuse normal application state. The [67a5 source-launch commands](TEST-CHECKPOINT-67A5B7C7.md#exact-retrieval-and-unchanged-setup-when-resources-permit) remain valid: `scripts/start-local-acceptance.ps1 -AcceptanceRoot E:\\Luci-local-acceptance\\runs -Port <free> -FrontendPort <distinct-free>` creates isolated state/projects and uses fresh loopback ports.

For a matching native build, use the same fresh checkout and `scripts/build-release.ps1 -Version 0.5.0 -Native` after its prerequisites/resource gates. Require its own release manifest to name `{commit}` with `sourceDirty=false`; retain the complete native/sidecar/resource tree. Use fresh non-linked `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `WEBVIEW2_USER_DATA_FOLDER` and no inherited `LUCI_API`; native `runtime_info.apiUrl` identifies its free API port. Do not install NSIS into the personal profile as part of this source checkpoint. No credential values should appear in handoff evidence.

## Focused additional acceptance

Carry the prior guide's four distinct projects and 1440→390→1440, drawer/Escape, draft recovery and exact reviewed-work controls. Additionally exercise delayed Home reads while opening another project: obsolete displays must not update the current project, and another live project read must still succeed. Check cancel/reopen with the same project, current authority, unavailable resource state and exact retained charges; no cancellation may start/stop a worker or release another allocation. Use source race tests as controlled evidence only; do not equate them with a socket-disconnect/native measurement.

The prior 67a5 idle diagnostic failed switch/input p95 at **519.762309/114.250714 ms** versus unchanged **300/100 ms** targets. No matching d20 responsiveness improvement is measured by this build. Full four-project sustained output quality/resources, live providers, Windows packaging/runtime and remaining reviewed-recovery gates remain open. Publish feedback with issue IDs, exact revision, reproduction and retained measurements while independent permitted coverage continues. This is **not LOCAL TEST HANDOFF READY**.
''')

companion = stage/'evidence/2026-10-09-package-d20ebd29.md'
companion.write_text(f'''# Clean matching Linux package at d20ebd29

Exact source `{commit}` built cleanly in {build['elapsedSeconds']:.2f} seconds and passed recorded original/additive packaging checks: **956 portable files, 347 licensed components, 47 guides/index files, 251 source-bound hosted URLs**. Staging separately reports 250 rewritten links. Portable archive: **{portable['bytes']:,} bytes**, SHA256 `{portable['sha256']}`.

[The capsule](package-d20ebd29/README.md) retains the initial remote-tracking preflight refusal, successful build/raw logs, both verifiers, source/release manifests, exact wrapper, old-67a5 preservation and new-archive copy/link receipt. The latter moves disk use to memory-backed storage, not a total-resource or performance gain. Publication did not rehash the private archives or rerun verification.

[Source retrieval and unchanged isolated Windows setup](../TEST-CHECKPOINT-D20EBD29.md) are available. No downloadable Windows binary, native test, matching installed workflow, latency improvement or complete local release is claimed. The previous 67a5 idle switch/input targets failed; C052 controlled cancellation tests and this package do not retroactively pass them. Windows work stays behind the local tester's resource gates.
''')

patterns = [re.compile(rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'), re.compile(rb'gh[pousr]_[A-Za-z0-9]{30,}'), re.compile(rb'AKIA[0-9A-Z]{16}')]
findings = []
scanned = 0
for path in out.rglob('*'):
    if path.is_file():
        scanned += 1
        for index, pattern in enumerate(patterns):
            if pattern.search(path.read_bytes()): findings.append({'path': str(path.relative_to(out)), 'pattern': index})
assert not findings, findings
(out/'privacy-scan.json').write_text(json.dumps({'scope': 'Selected private-key/access-token formats only, not universal privacy scanning. Standard task paths deliberately preserved; no private app/credential files read.', 'scannedFiles': scanned, 'findings': findings}, indent=2)+'\n')
record = lambda path: {'path': str(path.relative_to(stage)), 'bytes': path.stat().st_size, 'sha256': sha(path.read_bytes())}
members = [{'path': str(path.relative_to(out)), 'bytes': path.stat().st_size, 'sha256': sha(path.read_bytes())} for path in sorted(out.rglob('*')) if path.is_file()]
publication = {'sourceCommit': commit, 'sourceDirty': False, 'scope': 'Recorded Linux package verification and source checkpoint only; matching native/installed/latency/recovery acceptance remains open.', 'portableArchive': portable, 'cloudArchive': cloud, 'packageIdentity': compact['packageIdentity'], 'verifiedPackageFiles': compact['verifiedPackageFiles'], 'licenseComponents': compact['licenseComponents'], 'verifiedGuideFiles': compact['verifiedGuideFiles'], 'newHostedURLsBoundToExactGitObjects': compact['newHostedURLsBoundToExactGitObjects'], 'rewrittenLinks': compact['stagingResult']['rewrittenLinks'], 'companionReport': record(companion), 'checkpoint': record(checkpoint), 'members': members, 'manifestConvention': 'Members exclude publication.json/SHA256SUMS; checksums include publication.json; companion and checkpoint bound separately relative to docs/luci.'}
(out/'publication.json').write_text(json.dumps(publication, indent=2)+'\n')
files = [path for path in sorted(out.rglob('*')) if path.is_file()]
(out/'SHA256SUMS').write_text(''.join(f'{sha(path.read_bytes())}  {path.relative_to(out)}\n' for path in files))
summary = {'path': str(out), 'manifestMembers': len(members), 'sumEntries': len(files), 'files': len(files)+1, 'originalBindings': len(bindings), 'publicationSHA256': sha((out/'publication.json').read_bytes()), 'checkpoint': record(checkpoint), 'companion': record(companion), 'portableArchive': portable, 'scope': 'External-stage writes only; no repository/runtime changes'}
(tools/'publish-package-d20ebd29-result.json').write_text(json.dumps(summary, indent=2)+'\n')
print(json.dumps(summary, indent=2))
