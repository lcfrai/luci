# Ordered check recovery preserves a charged workspace witness

**One new controlled regression passed; installed recovery gates remain open.**
This capsule retains the exact external overlay run against clean
`8c7bf16aa035b27dbea185b565052495eb60777f`; its tracked Go/module product sources
are unchanged from `edd0cb121ff6847f681ab6bf41509a497f043d41`. No production code was
changed by this experiment. Root will integrate the test separately.

## Executed scope

`TestProjectCheckOriginalReviewPreservesSeparatePendingWorkspace` passed once
with Go 1.26.3 linux/amd64, `-race -p 1 -count=1`, `GOMAXPROCS=2`. The test body
took **0.96 seconds**, package **2.055 seconds**, and complete command **18.486
seconds**. One top-level pass; zero failed/skipped tests. Go timeout was 170
seconds; the wrapper's outer hard deadline was 180 seconds. Neither fired. The
command returned zero and stderr is empty. There were no original compilation or
fixture failures in this run, and no extra batch or original-test rerun.

The test drives production HTTP handlers, the product's durable receipts, shared
execution ledger and lifecycle locks, the actual observation method and exact
reviewed recovery. Backend responses/fence proofs are **controlled fixtures**;
there is no application process, browser, Docker allocation or model/provider
execution. The test uses two independent workspace scopes and user identities.
Its actual Git fixture setup is isolated in test-owned temporary directories.

It establishes these properties together:

- Capture the unknown-Create target's exact recovery view before the first
  observer. No hidden automatic observer exists in the fixture; reopen preserves
  that explicit scheduling seam.
- Create a second workspace's real production check through HTTP; hold its
  controlled Create boundary. Both real product-created ledger entries retain
  1,024 MiB/1,000 mCPU and nonzero pending disk charges. No lease is seeded.
- Four explicit observer passes, including a product/API reopen, leave the
  original view/revision/digest, entire ledger and both history files unchanged.
- Current Edit revocation refuses recovery without a ledger mutation. Restored
  authority and the original view settle only the original target, cancelled and
  unverified, without command replay or fabricated exit/output.
- The other workspace's exact lease/receipt/history survives recovery unchanged.
  One barrier release then yields one witness Create/Start/output/removal and
  exact completion acknowledgement. Its charge releases and target fence remains.

This adds concurrent-charge conservation to the existing CLOUD-038 deterministic
ordered-review proof. The existing single-target regression was already covered
and was **not rerun**. This is neither an observed shipping-package
review-before-first-observer interleaving nor a real concurrent Docker allocation.
Those installed gates, native Windows, CLOUD-039's installed terminal-refusal
branch, live-provider quality and full local acceptance stay open. Original failed
installed fixtures and their charges were not touched.

## Exact sources and provenance

`run-1/original-draft.go` is the original unformatted external draft.
`run-1/formatted-draft.go` is the exact source tested via `run-1/overlay.json`. Its
initial UNRUN comment is retained as original bytes; the raw successful result
supersedes that comment for this source only. The repository-relative candidate
is `candidate/control-plane/internal/api/project_check_order_witness_test.go`.
`tested-to-integration-comment-only.patch` removes that obsolete commentary;
all bytes after the first three comment lines match the tested source. No
untested executable-code change is proposed, so a duplicate run solely for this
comment-only change would add no coverage.

The before/after snapshots bind **995 selected inputs**: 992 tracked repository
`.go` and `go.mod`/`go.sum` files plus exact external `go.work`, test and overlay.
All input arrays match, HEAD is unchanged, and the working tree is clean before
and after. This is intentionally **selected binding**, not complete compiled
provenance: it includes repository fixtures/evidence not compiled by this test,
and does not hash all installed external modules, standard-library/generated
inputs or compiler binary. `external-inputs/go.work` preserves the exact recorded
external workspace. `source-binding-summary.json` states these limits.

`run-1/run.py`, `command.json`, raw `test.jsonl`, empty `stderr.log` and
`result.json` retain the command, timings and outcome. The wrapper uses exact
historical absolute paths; do not execute it blindly on another machine.
Original result and source files are byte-for-byte copies. No source or output
redaction/transformation was necessary for these fixture logs.

## Reproduction

Use a fresh isolated checkout of the checked-out revision above and compatible
Go 1.26.3. Create an external Go workspace pointing to that checkout's
`control-plane` and `sandboxd` modules, following the included `go.work` structure.
Create an external overlay mapping the checkout's nonexistent
`control-plane/internal/api/project_check_order_witness_draft_test.go` to the
included exact `run-1/formatted-draft.go`. Update paths; preserve file bytes. From
`control-plane`, with isolated caches/temp and that external `GOWORK`, run:

```sh
GOMAXPROCS=2 GOFLAGS=-mod=readonly go test -race -p 1 -count=1 -timeout=170s \
  -json -overlay=/absolute/path/to/overlay.json \
  -run '^TestProjectCheckOriginalReviewPreservesSeparatePendingWorkspace$' ./internal/api
```

Once root integrates the repository-relative candidate, omit the overlay and run
that same exact test name if a later affected change warrants it. Reproduction
on Windows remains subject to its memory gates and is not established by this
Linux result. The test itself performs controlled local Go/Git fixture work; it
needs no Docker daemon, provider credential or external project.

## Publication

`publication-manifest.json` binds every payload member. `SHA256SUMS` binds those
members and the manifest. Root must force-add any Git-ignored raw log and verify
the staged and final immutable Git tree with `scripts/acceptance/verify-publication.py`.
Filesystem checksum agreement alone does not prove publication.
