# CLOUD-075 installed Linux authority witness, package 78665270

The installed Linux web/API check passed on 2026-10-09, using package source `786652704fa8bebd1ba1af41cfa1d414d7def100` and Chromium `151.0.7922.173`. The parent joined the driver with exit 0. The receipt spans 17:25:41.962Z–17:25:56.718Z. This supplements the [CLOUD-075 source regressions](../cloud075-project-authority-reads-20261009/REPORT.md); it is not CLOUD-032 latency or native-window acceptance.

The driver verified all **964 installed archive members** against the captured clean-package provenance before startup. It rechecked its frozen helper inputs, selected Playwright/lockfile inputs, binary and web index after cleanup. Exact code, source/dependency bindings, package verifier, controlled-test output, historical preparation notes and the preserved pre-correction driver are under [driver/](driver/). The historical preparation README and `preparedOnly` binding describe the freeze before this actual run; they have not been rewritten to manufacture execution evidence. [actual/result.safe.json](actual/result.safe.json) records this execution separately.

## Actual API and UI observations

The fixture used fresh isolated owner and member accounts, one real team and four real projects created through valid management/product APIs. The ordinary member had separate project grants; execute/secrets/deploy/admin flags were false. Each project had a distinct saved purpose. Login, Continue to projects, Workspace selection, project opening, reload and recovery used the real UI. No auth/application storage or authoritative response body was fabricated.

| Boundary | Observed current-project permission GETs | UI/API assertion |
| --- | ---: | --- |
| Select project 1 | 1 | Correct purpose; other project purposes absent |
| Select project 2 | 1 | Correct purpose; other project purposes absent |
| Select project 3 | 1 | Correct purpose; other project purposes absent |
| Select project 4 | 1 | Correct purpose; other project purposes absent |
| Later full browser reload on project 4 | 1 fresh read | Correct scoped purpose |
| Held grant for newly opened project 1 | 1 | Real 200 fetched before denial; target purpose still unpublished |
| Entry after fresh project 1 grant restored | 1 fresh read | Correct scoped purpose restored |

All seven responses were real `200` bodies and were ultimately fulfilled. The six ordinary entry/reload windows delayed response delivery by **300 ms** to expose overlapping consumers, then asserted exactly one current-project read. This establishes the instrumented request-count witness; it does not measure ungated request rates or end-to-end switching latency. The installed witness does not include an old-package comparison; the identical before/fixed source test body supplies the regression comparison.

For the authority boundary, an actual owner request removed team membership while a real successful project response was held. The UI reached **Workspace unavailable** before the old response was released. Its delivery succeeded; the cancellation fallback was not exercised. Membership was then restored without project grants: none of the four private purposes appeared, and a real member specification GET returned **403**. Restoring a fresh project grant produced the final new GET and correct purpose. This is a coarse transport race. The exact settled-promise-before-denial microtask race remains established by the carried source tests.

The journal records **35 operator HTTP requests**: 34 expected successes and the intentional specification 403. The browser mutation/probe observer recorded only the login POST. The guards permitted benign readiness probes, but none was observed. Actual session catalogues were empty before and after. Execution mutations were excluded by the browser/operator allowlists, and no provider was configured. The scenario's literal `agentOrCheckDispatches:0` and `providerCalls:0` fields are not presented as independently measured counters.

Management and owned user/team workspace API subprocesses were necessary for real authority setup. No agent/shell/task/check execution or Docker container was started by this check. Startup inherited only PATH/LANG/LC_ALL/TZ before applying isolated fixture settings. Both Docker endpoint variables named a fresh nonexistent private Unix socket. No global Docker query or mutation was performed. The frozen [guards](driver/guards.mjs), [launch](driver/launch.mjs), [scenario](driver/scenario.mjs) and [route boundary](driver/route-boundary.mjs) retain the exact boundaries and budgets.

## Cleanup and independent retained-state inspection

The driver completed browser, controller and final-observation cleanup phases; its route boundary was sealed with zero pending callbacks, zero pending mutations and zero closing reads. Its final observation still saw a controller zombie and two browser zombies. The later independent read-only inspection at **17:28:01.753958Z** checked only the seven exact fixture-recorded PID/start identities, including the driver:

- Driver, all three controllers and main browser: original identities absent.
- Two browser identities: same original start ticks, state **Z**; quiescent but still present.
- All known originals quiescent: true. All originals absent and strict postrun closure: **false**. The inspector deliberately exited **2** for this distinction; the successful workflow result remains separate.

The [exact closure receipt](closure/receipt.json), [inspector source](closure/inspect.py) and three [owned identity/scope manifests](closure/ownership/) are retained. The inspector read only recorded `/proc/<pid>/stat` identities; it performed no process census, environment read, signal, HTTP request, socket connection, Docker query or cleanup mutation. Managed scopes were two user runtimes and one team runtime. It did not discover unrecorded processes or claim whole-machine closure.

Selected own state and safe receipt files were stable across the independent inspection. Main/managed session registries and the execution lease ledger were absent; two worker-address metadata files existed and retained identical hashes. Session-file formats can contain tokens, so any such files are hash-only inputs, never parsed/copied. These file bookends are before/after **the inspection**, not an invented workflow baseline. The private Docker socket was absent at both bookends. The driver's process scanner retained 128 capped EACCES diagnostic samples, potentially repeated across scans; publication retains only that count and code, not unrelated PIDs or environment values.

## Carried validation and evidence limits

The source validation was completed before the installed run and is not rerun or inflated here:

- Identical ten-case regression body: old source 2 pass/8 fail; fixed source 10/10 pass.
- Initial four existing affected suites: 64/66 pass, two obsolete exact-argument assertions and one unused deferred rejection retained in raw evidence. After compatibility corrections, Provider alone passed 28/28; the other three suites had already passed 38/38 and were not repeated. This supports 76 unique final passing cases across documented runs, not one clean combined execution.
- Full desktop TypeScript check passed.
- Frozen installed-driver route-boundary tests passed 5/5 in one source-only Node run, exit 0. They cover callback/mutation drain and late failure handling; no app/browser starts occur in those tests.

No native desktop window, Windows build/runtime, installed keyboard workflow, dirty-draft workflow, or latency acceptance was added by this witness. Source tests carry cancellation, provider isolation, revision, transient/confirmed denial and dirty-draft semantics. The real installed authority check does not imply a backend authorization bypass or data disclosure in the old source.

Publication uses explicit field projections: [96 safe journal rows](actual/events.safe.json) preserve source sequence/timestamps, alias fixture project IDs and normalize authority paths; seven rail DOM metadata rows are omitted. The original 103-row journal and result hashes are bound in [publication.json](publication.json), and [publication-extract.py](publication-extract.py) records extraction. Exact own PID/start proof is retained only in the closure evidence. Credentials, raw private logs/failures, screenshots, raw API bodies, unrelated process identities, fixture state, dependencies and binary/web payload are excluded. The final manifest binds every published payload file. No repository files were changed by this worker.
