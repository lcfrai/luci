# Installed check Create recovery preparation

This helper runs a bounded installed check Create recovery acceptance. It requires an explicit runtime window, fresh output and fixture, an exact installed product package, and an exact committed helper source binding. Original failed fixtures are never an input. Controlled test DTOs are not installed acceptance evidence.

Default reproduction product: `4df94ab9dad0e9fedd28a873bbadc56720e1402e`, installed at `/tmp/luci-retained-tools/package-0.5.0-4df94ab9`. Package provenance: `/workspace/luci/artifacts/quality-swarm/candidate-4df94ab9/package/package-provenance.json`. Root verified 2,021 members; the adapter independently verifies archive and installed members before startup. Image: `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. Do not substitute earlier d6 product, which predates the second-copy admission correction. A later explicitly reviewed package is selected with `--expected-source FULL_40_HEX_PRODUCT_COMMIT`; package provenance must name that exact source, every installed archive member/binary/web hash remains verified, and runtime image/source validation is unchanged. Helper commit/source binding remain independent and exact.

Baseline observer origin: `5d306b6dae3cc9d0a4d9cbf95f8142c37958964f`. Actual executing helper selection is supplied by `--helper-commit` and independently checked against every executed source blob in that commit. Lifecycle, startup, journal, proxy baseline, wrapper baseline, and guard extraction originate from `81f2ba5b194a81a4f2ad167e466793f59162b387`; each origin/current byte hash is recorded separately in `source-binding.json`. New adapter/scenario code is externally staged work, not code already contained in either commit.

## Real installed flow

1. Before startup, verify all11 driver/wrapper/shared-helper/sampler files against both the frozen binding and exact selected Git commit. Start a fresh private fixture and installed package with zero providers/workers. Create a fresh team, two projects, source, actual specification/design approvals, and reviewed commits through supported APIs. Supported management policy permits one 1 GiB/1,000 mCPU check per actor; preserve 25%/minimum 2 GiB memory headroom and 1 GiB working disk reserve.
2. Run a real UI-consented check on the unrelated project. Briefly hold its actual daemon Create ACK through the existing bounded proxy gate while capturing product-effective resources before any product Start; then deliver that same ACK. Terminal/refusal resource receipts provide another timestamped observation for correlation with the independent cgroup sampler. Require passed status, nonempty `RECOVERY_CHECK_OK` output, confirmed cleanup and no held charge. Preserve that history and both source trees.
3. Consent to one target check in the installed UI. A proxy drains actual daemon Create201 and captures its immutable ID, then deliberately loses that response. The product must naturally reach `unknown / check_create_unconfirmed`, retain its exact creating allocation and charge, and send no target Start. Daemon acknowledgement is not product acknowledgement.
4. Inspect the exact unstarted container's identity, image, command, environment, source mount, resources and security shape. Read/dismiss actual recovery review without effects. Revoke an editor's project Edit grant after the editor read the valid review: the same authenticated reviewed POST must return403. The owner's stale positive revision must return409. Each probe must preserve full ledger, receipts, source hashes and Docker mutation intents.
5. Re-read current owner review. Require actual dialog acknowledgement and exactly one successful recovery POST `{expectedRevision,recoveryDigest,acknowledgeStop:true}`. Correlate original Delete204, original absence, acknowledged fence Create201, exact ID/name inspected inert shape and durable `retired` fence proof before counting the exact target charge released.
6. Target remains cancelled/unverified, with no invented output or command replay. Preserve unrelated completed history and both source trees. Full-ledger charge falls from one check to zero; recovery GET becomes `complete`. Retain the permanent image-free marker.
7. Close owned controllers, positively drain proxy, then close/reap browser under bounded cleanup. Independently require exact remaining marker, no volumes/active check charges and retained retired proof. Scenario assertions alone deliberately cannot set `passed:true`.

Default exact totals: **2 ordinary check Creates, 1 fence Create, 1 Start, 0 workers, 0 external model calls**, at most **1 GiB/1,000 mCPU** ordinary charge. The inert, image-free marker has configured bounds 6 MiB/100 mCPU/one PID and is never started; it requires no second image copy.

Explicit `--charged-witness` adds one real unrelated reservation, held before Create forwarding. Admission must cover the actual full ledger plus prospective image disk demand and RAM/CPU, without weakening policy or fabricating a lease. After target retirement, release that original held request once and require normal completion. Optional totals: 3 ordinary Creates, 1 fence Create, 2 Starts, at most 2 GiB/2,000 mCPU. Root's measured VFS capacity supports one heavy allocation, so default evidence explicitly leaves concurrent charged-witness isolation **unverified**.

## Boundaries and retained marker

`installed-recovery.mjs` implements concrete lifecycle, API and actual UI flows. `recovery-scenario.mjs` has inert imports and scoped proofs. `check-create-guard.mjs` generalizes the existing DAG guard while retaining complete ledger/security checks; every check is single-command/index0. The shared proxy preserves defaults and adds explicit opt-in recovery/name-inspection/budget support. Foreign mutation identities, marker Start and opted-in permanent marker Delete are refused.

Workflow budget is 600 seconds, cleanup 180 seconds. Wrapper adds 120 seconds preparation and enforces one 900-second driver budget. Forced or uncertain termination fails. The source-bound shared cgroup observer follows only the driver's exact team-instance pointer. Measurement provenance is separate from recovery success. An image-free retained marker may yield explicit identity/label diagnostics in the frozen observer; do not present those as runnable-check samples or silently discard them.

Exact charge/release proof comes from the complete private shared ledger. The baseline 4df owner's public scheduler summary is a process-local cache and can miss a team child's changes. Default mode preserves it only as diagnostic evidence. For a matching product containing the CLOUD-034 correction, add `--require-public-reservations`: each quiescent charged/released checkpoint requires the current owner's public `ready` observation to be at most 5 seconds old and match the complete ledger's count/memory/CPU. Explicit `unavailable` responses retry only that GET within a 10-second checkpoint budget, then fail; absent/legacy/malformed/stale/incorrect ready totals fail. This visibility assertion never replaces exact allocation ownership, absence/fence or release proof. No installed CLOUD-034 visibility result is claimed by controlled tests.

Credentials remain in memory/fresh private fixture. Bounded phase/final JSON rejects known credential literals; no raw Create body or auth header is exported. Screenshots and proposed public artifacts require later secret review. Publication is not part of preparation.

The marker must remain while the fixture could resume or late original transports could act. Marker removal is never product lease proof. No removal/terminal retirement is implemented. Any separately authorized terminal fixture retirement must preserve an archive, positively close controllers/transports, rule out fixture resumption, freshly verify immutable ownership/shape and report marker removal separately from product recovery/capacity release.

## Source binding and invocation

The integrated controlled suite passed 388 Node tests and 37 mocked-wrapper tests, with 13 syntax checks. It includes all 29 original default proxy cases, 12 opt-in proxy cases, exact source/package-selection boundaries, complete synthetic scenario flows, and late/never-settling cleanup cases. These are helper checks, not installed recovery evidence. Preserve actual run commands, exact source hashes and raw outcomes; changed bytes need appropriate fresh validation.

After the new helper is committed, choose its reviewed full40-hex commit and generate a fresh binding. The output's parent must already exist. This generator reads Git/source and writes only the one binding JSON; it starts no runtime:

```sh
python3 scripts/acceptance/check-create-recovery/freeze-source-binding.py \
  --repo /workspace/luci --helper-commit REVIEWED_40_HEX_HELPER_COMMIT \
  --output /workspace/luci/artifacts/quality-swarm/check-recovery-4df94ab9-attempt1/source-binding.json
```

Use the same reviewed commit and binding in the wrapper invocation after the explicit runtime window opens:

```sh
python3 scripts/acceptance/check-create-recovery/run-with-observer.py \
  --observation-dir /workspace/luci/artifacts/quality-swarm/check-recovery-4df94ab9-attempt1/observation \
  --fixture-base /workspace \
  -- --window-granted --repo /workspace/luci \
  --helper-commit REVIEWED_40_HEX_HELPER_COMMIT \
  --expected-source 4df94ab9dad0e9fedd28a873bbadc56720e1402e \
  --source-binding /workspace/luci/artifacts/quality-swarm/check-recovery-4df94ab9-attempt1/source-binding.json \
  --binary /tmp/luci-retained-tools/package-0.5.0-4df94ab9/luci \
  --web /tmp/luci-retained-tools/package-0.5.0-4df94ab9/web \
  --provenance /workspace/luci/artifacts/quality-swarm/candidate-4df94ab9/package/package-provenance.json \
  --image-provenance /workspace/luci/artifacts/quality-swarm/dependency-4df94ab9-helper-4df94ab9-attempt9/result/image-provenance.json \
  --expected-image-id sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98 \
  --report /workspace/luci/artifacts/quality-swarm/check-recovery-4df94ab9-attempt1/result/report.json
```

For a newly pinned package containing CLOUD-034, replace the product/package/provenance paths consistently and add `--require-public-reservations` to the driver arguments. The 13 added visibility/CLI cases and affected guard/source-binding/scenario suite passed 161/161 controlled Node tests after that opt-in change. The original 388-case integration run remains historical evidence of the earlier helper bytes; neither batch is an installed recovery result.

Keep original wrapper stdout/stderr, driver result/incomplete checkpoint, durable journal, observer/self-cost, frozen committed sources, allocation/fence/recovery receipts, screenshots and independent closure. A failed original result is immutable evidence. Terminal fixture retirement and publication are separate reviewed operations.
