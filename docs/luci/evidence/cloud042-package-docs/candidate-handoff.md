# CLOUD-042 candidate handoff

Outside-checkout candidate based on `e2f426bb50267a7cceac1f53eb1e00a0a4cc278e`; no repository edits or package/runtime changes were made by this task. `candidate.json` binds before/after copies for eight changed paths and the unchanged build.rs test input; `candidate.patch` is the complete proposed patch. Original metadata audit remains unchanged under the sibling `package-payload-audit-2da/` directory. The earlier nine-file candidate is preserved under `intermediate-publication-unchecked/`.

## Proposed behavior

Both packagers use one dependency-free Python staging helper. On a clean exact Git revision reachable from a cached official remote-tracking commit, it retains the existing guides, relative assets, root licence/NOTICE, UAT and runbook, and omits only `docs/luci/evidence/**`. The offline association uses a validated canonical repository URL and a bounded tracking-ref containment query; it does not establish current GitHub URL availability or cryptographic publisher proof. No network/auth flow is added and remote URL values are not logged. Every staged source file must exist as an ordinary file at the named commit and match its Git object identity. Normal Git clean semantics handle legitimate CRLF Windows checkouts; staged bytes remain exact committed bytes. Ignored/untracked files and assume-unchanged modifications cannot enter a supposedly clean stage. A small generated validation index links to the full source revision and keeps release-gate limits explicit.

Recognized Markdown link destinations for omitted repository files become immutable GitHub links, or raw immutable content for images. Local shipped links and external URLs remain unchanged. Query/fragment meaning is preserved, including percent escapes and CommonMark entity delimiters; **heading anchors are not validated**. Code spans, fenced/indented code and HTML comments remain literal. A nested-list/indented-code ambiguity retains the full docs tree instead of omitting evidence and leaving a possible real link broken. No such ambiguous block was found in the current guides by the final read-only syntax probe. The helper is a purpose-built link-destination scanner, not a general Markdown renderer; the current guide audit and syntax fixtures define its verified scope. No application startup scan is added.

Dirty, unknown, unpublished or fork-only source keeps the full documentation tree without asserting official hosted revision links. The source exporter includes the new helper and excludes generated native guide resources.

Native resources use the same staged guide copies, including root README/DESIGN, while retaining runtime/licences/cloud resources. The existing documented native workflow already stages sidecar/runtime/licences through the release script before direct Tauri/Cargo operations. The candidate adds guides to that workflow and generated-directory ownership/ignore checks. Updated README guidance identifies the supported Windows release route and keeps Linux/macOS native packaging unverified. **build.rs remains unchanged**: no additional unconditional Cargo guard is introduced; normal Tauri resource validation remains. There is no new application runtime or Python startup dependency; both release scripts already require Python.

## Validation performed

Final `tests-attempt6.log`: **22 tests passed, zero skips**, using only disposable synthetic Git repositories outside the user checkout. Earlier 11-, 13-, 14-, 18- and 19-test runs are retained as overlapping development checkpoints, not additional unique outcomes. Cases cover exact-source retention, licences/assets, dirty/unknown/fork/unpublished/offline-probe fallback, controlled cached tracking ancestry, root/source-only links, nested labels, whitespace/percent encoding, balanced/escaped parentheses, titles, images/reference definitions, directory destinations, CommonMark entities and escaped ampersands, nested-list ambiguity fallback, literal code/comments, actual Git autocrlf normalization, unsafe/missing targets, symlinks, existing-output preservation and release/native/source-export wiring. Fake remote refs in disposable tests are controlled fixtures, not evidence of network publication.

`bash -n after/scripts/build-release.sh` passed. Python helper/test syntax is exercised by the final fixture run. PowerShell is not installed in this environment, so no PowerShell parse/native Windows build is claimed. No archive/native package build, actual size improvement, startup regression check or product suite was run for this candidate.

Reproduce the pure fixtures:

```sh
python3 -m unittest discover -s <candidate-stage>/after/scripts -p test_stage_release_docs.py -v
bash -n <candidate-stage>/after/scripts/build-release.sh
```

## Integration prerequisites and remaining checks

The original actual-source guide staging attempt at e2f failed conservatively before output. `stage-current-attempt1.log` and `current-missing-links.json` retain it. Two worklist links then resolved to absent files, both at `docs/luci/QUALITY-WORKLIST.md:37`:

- `evidence/check6-terminal-retirement-20261009/README.md` should refer to the existing narrative `evidence/2026-10-09-check6-terminal-retirement.md` (or explicitly to the capsule's existing `publication.json`).
- `evidence/check8-terminal-retirement-20261009/README.md` has the analogous existing narrative `evidence/2026-10-09-check8-terminal-retirement.md`.

Root corrected those worklist links and published `765fcaadeee7de1cf2ba3a3fb7b295d0af445b70`. The earlier helper's actual compact staging then passed: 43 unchanged inputs, 44 output files, 1,083,178 logical bytes, 196 rewritten links, clean source before/after; `actual-staging-765fcaad/` retains that intermediate receipt. **It predates the final publication-association, CRLF and parser refinements and is not a final-helper pass.** Initial final-helper association preflight returned no cached containing ref; root subsequently performed an authenticated fetch of the genuine published branch. No synthetic ref was added to the real checkout. Current source now includes other active work, so final actual staging is deferred until integration, a clean commit/push/fetch, and source binding to that exact revision. Do not weaken missing-file refusal or claim final actual staging from fixture tests or the intermediate receipt.

After integration, run relevant source-export/packaging checks and make a matching Linux build to measure compressed/member changes. Native resource/PowerShell/Cargo behavior needs Windows validation separately. Preserve the already-running sustained package's original identity and receipts; this candidate is not a modification to that measured build.
