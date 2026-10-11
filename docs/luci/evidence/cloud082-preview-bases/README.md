# C082 cold preview bases — evidence capsule

C082 lets an admitted project preview obtain missing images declared by its reviewed Dockerfile, then build from verified immutable image IDs. Preparation remains read-only. Each pull and the final build require a fresh check of the exact build lease, current Deploy authority and source. The runtime installer’s separate administrator authority is unchanged.

The capsule preserves distinct observations:

| Observation | Result | Scope |
|---|---|---|
| Original cold-base regression | Expected failure, exit 1 | One added test against the original backend; 52 source inputs unchanged |
| Initial integrated validation | 186 passing test outcomes: 43 top-level, 143 subtests; exit 0 | Race-enabled backend/API selection, before the quoted-whitespace COPY correction |
| Initial compile check | Exit 0 | `go test -run '^$' ./...`; no application tests executed |
| COPY regression before correction | Exit 1; four failures, five passes | Nine focused leaf cases; 55 inputs unchanged |
| COPY regression after correction | Exit 0; nine passes, zero skipped | Same focused cases; 55 inputs unchanged |

The initial integrated and compile runs had no complete before/after source manifest. Their outputs are preserved exactly; root observed both exit codes and observes that the captured API source has remained unchanged. These are not fully bound final-source runs. A later review found that quoted whitespace in preceding COPY flags could conceal an external `--from` image. The correction passed its dedicated nine-case, source-bound follow-up. Do not report the initial 186 outcomes as a rerun against the corrected source; those initial checks and the focused correction run provide distinct coverage.

`evidence/claims.json` holds the commands and these limits. `evidence/before-cold` retains the original invocation, overlay, source manifests and outputs. Those original files still refer to their execution-time absolute paths; they were preserved without rewriting. The overlay’s added regression and original `newPreviewProbe`/`checkProbe` helper files are included under `sources/original-before`, recovered from Git commit `64829123a5799858b57616b711c84c61b945871a` and checked against the original manifest. The remaining original input paths are retrievable from that commit and bound by the same manifest.

`sources/initial` contains the unchanged staged backend patch, source bindings and nine source/test files. Their original binding document describes staging time and correctly says fixed tests had not yet run. `sources/api-original` and `sources/api-captured` distinguish root’s API originals from the captured C082 changes. `sources/copy-correction` preserves the validator that the initial run exercised. `evidence/copy-correction` contains the dedicated original/fixed regression receipts, exact runner, patch and pinned Moby contract evidence. `sources/final` and `sources/final-selected-source-bindings.json` capture all 13 changed/new C082 Go source/test files, including corrected validator and regression. This selected manifest is not a full compiled input manifest: ten backend files match the focused run's unchanged input hashes; stability of the three API files is reported by root and their final bytes are captured. `sources/final-selected-source.patch` reconstructs the selected changes over the pinned baseline. The disposable-cache cleanup receipt is kept separately under `environment`; it is not a product outcome.

The supported cold-image policy is a 2 GiB virtual-size cap per missing external image, charged at twice that amount per FROM stage, plus twice the context size including bounded tar growth when pinning, plus 1 GiB working reserve. Cold plans above 32 GiB total demand are refused. There are at most 16 FROM stages; external references are deduplicated for pulls. Warm-only and scratch builds keep the prior demand calculation. These are conservative admission estimates, not hard Docker download, unpacking, disk or cgroup quotas.

Successful cold pulls require the matching canonical/familiar Docker terminal status, a valid manifest Digest, bounded complete JSON transport and successful closure. The immutable repository digest must match image inspection before any build. Tags are not followed after acknowledgement, and no ImageTag operation is introduced. Interrupted or unacknowledged pulls retain capacity and must not be replayed automatically. A fully acknowledged pull followed by a read-only policy/inspection failure can settle without starting a build.

No actual cold Docker pull, application image build or Windows idea-to-preview journey was validated by this capsule. See `WINDOWS-HANDOFF.md` for the remaining checks. No module cache, Docker state, credentials or unrelated execution artifacts are included.
