from pathlib import Path
import argparse,subprocess,json,hashlib,os
p=argparse.ArgumentParser();p.add_argument('--preflight',required=True);p.add_argument('--archive-preservation');a=p.parse_args()
commit='edd0cb121ff6847f681ab6bf41509a497f043d41';previous='d20ebd292f3f1de04d6726bab0998c210987bcb5'
repo=Path('/workspace/luci');tools=Path('/workspace/.luci-tools');src=repo/'artifacts/quality-swarm/candidate-edd0cb12/package';release=repo/'artifacts/releases/0.5.0';stage=Path(__file__).parent/'docs/luci';out=stage/'evidence/package-edd0cb12'
sha=lambda b:hashlib.sha256(b).hexdigest();read=lambda p:json.loads(p.read_text())
names=['build-invocation.json','build-result.json','build.log','package-integrity.json','package-provenance.json','previous-release-preservation.json','compact-package-verification.json']
assert all((src/n).is_file() for n in names)
build,integrity,provenance,compact,manifest,preservation=[read(x) for x in [src/'build-result.json',src/'package-integrity.json',src/'package-provenance.json',src/'compact-package-verification.json',release/'release-manifest.json',src/'previous-release-preservation.json']]
assert build['sourceCommit']==commit and build['exitCode']==0 and not build['timedOut'] and build['cleanBefore'] and build['cleanAfter'] and build['sameHead']
assert all(x['sourceCommit']==commit and x['sourceDirty'] is False for x in [integrity,provenance,compact,manifest])
assert compact['status']=='passed' and integrity['allMembersMatched'] and integrity['missingLicenses']==0
assert compact['originalProvenanceSHA256']==sha((src/'package-provenance.json').read_bytes())
assert compact['releaseManifestSHA256']==sha((release/'release-manifest.json').read_bytes());assert compact['artifacts']==manifest['artifacts']
assert preservation['oldSource']==previous and preservation['originalProvenanceUnchanged']
preflight=Path(a.preflight);assert preflight.is_file()
assert not out.exists(),'Refuse overwrite';out.mkdir(parents=True)
bindings=[]
def put(data,target,origin):
 path=out/target;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data);bindings.append({'publishedPath':target,'originalPath':origin,'originalBytes':len(data),'originalSHA256':sha(data),'publishedBytes':len(data),'publishedSHA256':sha(data),'transformation':'none'})
def copy(source,target):put(source.read_bytes(),target,str(source))
for n in names:copy(src/n,'receipts/'+n)
copy(preflight,'receipts/preflight-refusal.json')
if a.archive_preservation:copy(Path(a.archive_preservation),'receipts/new-archive-preservation.json')
for n in ['build-cloud054-candidate.py','verify-local-package.py','verify-compact-local-package.py']:copy(tools/n,'helpers/'+n)
copy(Path(__file__),'helpers/assemble.py')
for n in ['release-manifest.json','SHA256SUMS']:copy(release/n,'release/'+n)
env={k:v for k,v in os.environ.items() if not k.startswith('GIT_')};env['GIT_NO_LAZY_FETCH']='1'
for path in ['scripts/build-release.sh','scripts/stage-release-docs.py','scripts/package.py','scripts/collect-licenses.py','scripts/launch-unix.sh']:
 put(subprocess.check_output(['git','-C',str(repo),'show',commit+':'+path],env=env),'source/'+path,'git:'+commit+':'+path)
(out/'original-bindings.json').write_text(json.dumps({'scope':'Byte-exact copied completed receipts/helpers and five immutable packaging sources. Publication does not hash private large archives or run build/verifiers/application.','files':bindings},indent=2)+'\n')
artifacts={r['name']:r for r in compact['artifacts']};portable=artifacts['luci-linux-amd64.tar.gz'];cloud=artifacts['luci-cloud-linux-amd64.tar.gz']
(out/'README.md').write_text(f'''# Matching clean Linux package — edd0cb12

Clean package verification passed for **`{commit}`** on `codex/luci-production-runs`. This is a retrievable source checkpoint, **not LOCAL TEST HANDOFF READY**. Linux archive bytes remain private cloud artifacts; no new Windows binary or native pass is claimed.

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | {portable['bytes']:,} | `{portable['sha256']}` |
| Embedded/separate `luci-cloud-linux-amd64.tar.gz` | {cloud['bytes']:,} | `{cloud['sha256']}` |

[Provenance](receipts/package-provenance.json) binds binary `{provenance['binarySHA256']}` and web index `{provenance['webIndexSHA256']}`. Additive package identity: `{compact['packageIdentity']}`. [Release manifest](release/release-manifest.json), SHA256 `{compact['releaseManifestSHA256']}`, records the same source with `sourceDirty=false` and `nativeRequested=false`.

## Completed build and preserved refusal

[Build](receipts/build-result.json) completed exit 0 in **{build['elapsedSeconds']:.2f} seconds**, clean before/after and unchanged HEAD, with `GOMAXPROCS=2` and `GOFLAGS=-mod=readonly -p=1`; the 600-second timeout did not fire. [Raw output](receipts/build.log) retains warnings. The historical invocation remains `running`; final result supersedes it.

The [preflight recovery account](receipts/preflight-refusal.json) records a guard refusal separately from the build. This is a new factual account from retained tool outputs, **not reconstructed original stderr**. The official remote-tracking ref was stale; the assertion was before build/output-directory and release mutation. Root authenticated the exact published source with remote lookup and fetched the official ref, without synthesizing refs or weakening clean/public source checks. The unchanged [wrapper](helpers/build-cloud054-candidate.py) subsequently ran the build.

## Verification bounds

[Original verifier](receipts/package-integrity.json): **{integrity['verifiedExtractedFiles']} extracted files**, **{integrity['licenseComponents']} licensed components**, zero missing licences, all members matched. [Additive verifier](receipts/compact-package-verification.json): **{compact['verifiedPackageFiles']} portable / {compact['verifiedCloudFiles']} cloud files**, **{compact['verifiedRuntimeSourceFiles']} runtime-source files**, **{compact['verifiedGuideFiles']} staged guide/index files**, and **{compact['newHostedURLsBoundToExactGitObjects']} hosted URLs** bound to exact source objects. Staging separately reports **{compact['stagingResult']['rewrittenLinks']} rewritten links**; counts differ by design.

These recorded verifiers compare archives/extraction, modes, embedded-cloud equality, licences and source-bound compact guides. Guide regeneration uses the same committed implementation; offline object checks do not establish live URL/heading availability. Publication verifies copied receipts and selected source only, not independent retrieval of private archive bytes or full compiler dependency closure. Build duration is not app throughput, responsiveness or RAM evidence.

[Previous-release preservation](receipts/previous-release-preservation.json) records the byte-preserving rename of old d20 release outputs. Prior exact archive/provenance validation completed before preservation. No failed fixture or ledger was changed by this publication.
''' + ('''
[New-archive preservation](receipts/new-archive-preservation.json) is a separate run-owner receipt. It preserves exact bytes and compatibility paths; this publisher did not move/delete archives. Any memory-backed destination trades disk pressure for memory pressure and is not a total-resource gain.
''' if a.archive_preservation else '') + '''
## Product scope and remaining acceptance

This package includes [CLOUD-054 terminal/attach correction](../cloud054-terminal-close-20261009/README.md). Typed terminal evidence survives delayed polls and close ordering; an optional opaque generation distinguishes same-session replacement without being an authorization token or success result. Output-overload pause and explicit retry remain intact. Source verification: Go race 74 named passes (34 top + 40 sub); full frontend 2,826 pass / 1 canonical-doc mismatch across 161 files; subsequent contract-only 24/24 affected pass; TypeScript 0. These overlap and do not constitute a second full-suite run or installed result.

Prior d20 sustained attempt 8 had five natural completions and reviewed integrations with functionalPassed=true, but failed unchanged responsiveness targets. That scripted-provider fixture does not establish live-model quality or captured handoff reports. Historical failures, charged fixtures, cleanup/zombie limits and all Windows/provider/recovery/performance gates remain separate. This new package has no matching installed result at publication.

No new configuration or secret names are required. Follow [reproduction](REPRODUCE.md) and the [isolated Windows source checkpoint](../../TEST-CHECKPOINT-EDD0CB12.md), only after local resource admission. Publication launches nothing on the user's machine. Generic task/source paths remain in raw receipts; no application state or credentials are included.
''')
old=(tools/'publish-package-d20ebd29/docs/luci/evidence/package-d20ebd29/REPRODUCE.md').read_text()
old=old.replace(previous,commit).replace('d20ebd29','edd0cb12').replace('the d20 source','the edd0 source').replace('fresh-d20-verification','fresh-edd0-verification').replace('build-cloud052-candidate.py','build-cloud054-candidate.py')
start=old.index('Matching edd0cb12 installed');end=old.index('\n\nThe exact build wrapper',start)
old=old[:start]+'''Matching edd0 installed terminal/restart acceptance, Windows, live-provider quality and responsiveness remain open. Prior d20 sustained attempt 8 completed its scripted functional workflow but failed latency targets; report-capture and recovery limits are retained. CLOUD-054 adds no setup/configuration/secret names. Keep fresh isolated directories and the tester's resource gates. The source-controlled optional generation is an equality identity, not authority or semantic completion.''' + old[end:]
(out/'REPRODUCE.md').write_text(old)
checkpoint=stage/'TEST-CHECKPOINT-EDD0CB12.md'
checkpoint.write_text(f'''# Windows source checkpoint — edd0cb12

Exact retrievable source: [`{commit}`](https://github.com/lcfrai/luci/commit/{commit}), branch `codex/luci-production-runs`. [Matching clean Linux package](evidence/package-edd0cb12/README.md) passed its original/additive verification. Linux archives remain private cloud artifacts; no matching Windows binary/native acceptance is asserted. **The local milestone remains open.**

CLOUD-054 preserves typed terminal evidence across close/poll ordering and distinguishes same-ID allocation replacement with an opaque generation. Controlled source evidence includes Go 74 race outcomes, the preserved full frontend 2,826 pass / 1 documentation mismatch, a separate 24-pass affected contract correction, and TypeScript 0. It does not establish installed transport or live-provider behavior.

There are **no new configuration or secret names**. Carry the [67a5 safe setup and four-project procedures](TEST-CHECKPOINT-67A5B7C7.md) plus [d20 cancellation cases](TEST-CHECKPOINT-D20EBD29.md), substituting this exact source. Preserve existing user state/projects, previous checkouts, all evidence and shared Docker workloads. Use task-local supported Node 24.19.0/npm and Go 1.26.3; the earlier Windows tester's npm 10.9.3 is separate from cloud root's npm 11.9.0. No Windows headroom was measured by this publication. Do not launch while its existing 2 GiB source / 2.5 GiB browser-native admission hold persists.

Use the prior guide's checked fresh retrieval with:

```powershell
$candidate = '{commit}'
$checkout = Join-Path 'E:\\Luci-local-acceptance\\checkouts' ('edd0cb12-' + [guid]::NewGuid().ToString('N'))
```

Clone/fetch into that new directory, detach at `$candidate`, verify clean exact HEAD, then run locked `npm ci` in `desktop` using task-local caches. The safe source launcher remains `scripts/start-local-acceptance.ps1 -AcceptanceRoot E:\\Luci-local-acceptance\\runs -Port <free> -FrontendPort <distinct-free>`. It creates isolated state/projects. Never bypass `test-installed-windows.ps1`'s disposable-runner guard.

For native build, use `scripts/build-release.ps1 -Version0.5.0 -Native` (spell parameters as `-Version 0.5.0 -Native` in the shell) from the new checkout after prerequisites/resource admission. Require its manifest to name `{commit}` with `sourceDirty=false`. Preserve the complete native/sidecar/resource tree, use fresh nonlinked `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `WEBVIEW2_USER_DATA_FOLDER` and no inherited `LUCI_API`; take the native free API address from `runtime_info.apiUrl`. Do not install NSIS into the personal profile merely to test this checkpoint.

Prioritize four distinct saved project purposes, 1440→390→1440 navigation, disclosure/Escape/focus/drafts, lazy large context, reviewed retry and current authority. Add actual shell typed exit then socket close, delayed status read, same-ID Restart with a distinct generation, terminal DOM retention, no automatic reconnect after exit, nonzero exit remaining ended rather than successful, output pause across replacement and explicit Retry. Keep exact source/package/image/instance identities and evidence. The exclusive Linux acceptance helper must never run against the shared Windows Docker daemon.

Prior d20 sustained attempt 8 completed five scripted task results/integrations but failed latency; it is not a live-provider or report-capture result. Matching Windows/runtime, sustained responsiveness, real-model quality and unresolved recovery/report gates remain open. Send stable-ID findings and exact tested revision between batches; preserve older failed fixtures. This is **not LOCAL TEST HANDOFF READY**.
''')
# Correct shell syntax directly, without ambiguous pseudo-parameters.
s=checkpoint.read_text().replace('`scripts/build-release.ps1 -Version0.5.0 -Native` (spell parameters as `-Version 0.5.0 -Native` in the shell)','`scripts/build-release.ps1 -Version 0.5.0 -Native`');checkpoint.write_text(s)
companion=stage/'evidence/2026-10-09-package-edd0cb12.md'
companion.write_text(f'''# Clean Linux package checkpoint — edd0cb12

Exact source `{commit}` is retrievable on `codex/luci-production-runs`. Clean build and original/additive package verification passed: {compact['verifiedPackageFiles']} portable files, {integrity['licenseComponents']} licensed components, {compact['verifiedGuideFiles']} staged guides/index files and {compact['newHostedURLsBoundToExactGitObjects']} source-bound URLs. Archive bytes remain private cloud artifacts; no Windows package pass or full local acceptance is claimed.

See [identity, complete receipts and scoped preflight recovery account](package-edd0cb12/README.md), [reproduction](package-edd0cb12/REPRODUCE.md), and [safe Windows source checkpoint](../TEST-CHECKPOINT-EDD0CB12.md). C054 source-controlled terminal/restart correction is included; actual installed verification, responsiveness and remaining local gates stay open. No new configuration or secret names. No local processes were started by this publication.
''')
members=[]
for f in sorted(out.rglob('*')):
 if f.is_file():b=f.read_bytes();members.append({'path':str(f.relative_to(out)),'bytes':len(b),'sha256':sha(b)})
pub={'sourceCommit':commit,'sourceDirty':False,'scope':'Recorded clean Linux packaging/source checkpoint; actual installed/native/performance/provider acceptance remains open.','portableArchive':portable,'cloudArchive':cloud,'packageIdentity':compact['packageIdentity'],'members':members,'companionReport':{'path':str(companion.relative_to(stage)), 'sha256':sha(companion.read_bytes()),'bytes':companion.stat().st_size},'checkpoint':{'path':str(checkpoint.relative_to(stage)),'sha256':sha(checkpoint.read_bytes()),'bytes':checkpoint.stat().st_size}}
(out/'publication.json').write_text(json.dumps(pub,indent=2)+'\n');checks=members+[{'path':'publication.json','sha256':sha((out/'publication.json').read_bytes())}]
(out/'SHA256SUMS').write_text(''.join(r['sha256']+'  '+r['path']+'\n' for r in checks))
for r in checks:assert sha((out/r['path']).read_bytes())==r['sha256']
result={'stage':str(stage),'capsule':str(out),'members':len(members),'files':len(members)+2,'publicationSHA256':sha((out/'publication.json').read_bytes()),'sumsSHA256':sha((out/'SHA256SUMS').read_bytes()),'companion':pub['companionReport'],'checkpoint':pub['checkpoint']}
(Path(__file__).parent/'freeze.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
