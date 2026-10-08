# Evidence for e81b5860

These are compact, allowlisted receipts for commit
`e81b586077f8af97e5e59472e34eafd161f16b8a`. They describe a historical local-product
foundation, **not a completed local release**. Later fixes require their own
verification. Raw logs, screenshots, fixture state, credentials, local paths and
process identities remain outside Git.

| Receipt | Observed result | Boundary |
| --- | --- | --- |
| [Frontend](frontend.json) | 2,361 tests in 143 files; typecheck and web build passed | Source tests, not native GUI or full browser acceptance |
| [Go services](go-services.json) | 2,600 passing test outcomes, 25 skips; 32 packages passed, 2 skipped | Linux; skipped cases remain unverified |
| [Controlled sustained run](controlled-sustained.json) | Requested 600 seconds, 628.11 seconds total; four projects, seven controlled workers, six successes and one deliberate failure | Actual local processes with synthetic admission observations; no Docker or models |
| [Package](package.json) | Clean Linux build; 947 extracted files matched; 347 licence components, zero missing licences | Archive integrity and inventory, not legal clearance or cloud deployment |
| [Source export](source-export.json) | 1,429 source files checked against Git; ZIP hash verified | Allowlisted export; no fresh exported-tree build, publication or legal-clearance claim |
| [Portable Linux](portable-linux.json) | Six portable groups and seven shipped-launcher outcomes passed | Real packaged service/launcher; no native GUI, Windows, Docker or model calls |
| [Failed and limited scopes](failed-and-limited-scopes.json) | Earlier sustained failure, slower tiny-workload parallel result and five failed installed attempts preserved | Full installed four-project acceptance did not pass in this evidence set |

The sustained trial observed three live workers in each of 600 workload samples
and zero in its initial/final samples. It verified the expected controlled output,
a deliberate process failure, fair fourth-project admission, cancellation before
allocation, frozen running context through amendment, and service/ledger reopen.
Reopen occurred inside the test process; it was not an OS-process crash or host
reboot. Sampled controller-plus-worker RSS peaked at 158,851,072 bytes. That sum
excludes browsers, Docker and models, may double-count shared pages, and is not a
whole-app memory budget. Host CPU/memory figures include other work on the shared
Linux host and do not establish Windows responsiveness or application quality.

## Source and artifact identity

[source-binding.json](source-binding.json) independently compares every retained
input hash with its Git blob at this commit: 394 frontend, 583 Go and 670
controlled-process inputs. The frontend and Go reports recorded no changed inputs
during their checks. The sustained binary's original compilation metadata retains
its earlier dirty checkout identity, `0f70e067…`; all 670 retained compilation
inputs were subsequently proven byte-identical to `e81b5860`. This does not rewrite
the history as a clean-commit compilation.

The controlled test executable SHA-256 is
`8b728fd85c104d4121beab2d8e9382c363acdf1f616e2045e0dd8e4daa8d9c9a`.
The separate clean packaged service SHA-256 is
`ed64cff8a7021a580e9980bb3b6c3a0c244f203d7fba4b7e690f099f2ff93173`.
They are different executables for different checks.

[lineage.json](lineage.json) records SHA-256 values of the original ignored
artifacts. These allow exact comparisons if those originals are available; hashes
are not signed attestations, and private originals are not required to pull this
source. Recheck the source bindings from a clone containing this commit:

```sh
python3 docs/luci/evidence/e81b5860/verify-source.py
```

## Reproduce the checks

Use an isolated checkout at the exact revision. These commands create fresh
outputs; they do not promise identical timings or byte-identical build outputs.
The recorded toolchain was Go 1.26.3 on Linux amd64 and Node 24.19.0. Install the
locked frontend dependencies first. Keep the evidence files available outside
that historical checkout if needed; they were committed later.

```sh
cd desktop
npm ci --no-fund --no-audit
npm run test -- --reporter=json --outputFile=vitest.json
npm run typecheck
npm run build
cd ..
go test -p 2 -json ./control-plane/... ./sandboxd/...
```

The exact controlled sustained test is opt-in and takes about ten minutes. Run it
with a fresh report path and adequate host capacity. The recorded run compiled a
retained test executable, hashed it and compared source inputs before/after build:

```sh
mkdir -p artifacts/reproduction/e81-sustained
cd control-plane
go test -c -o ../artifacts/reproduction/e81-sustained/capacity.test ./internal/api
cd ..
LUCI_SWARM_MULTI_PROJECT=1 \
LUCI_SWARM_MULTI_PROJECT_SECONDS=600 \
LUCI_SWARM_MULTI_PROJECT_REPORT="$PWD/artifacts/reproduction/e81-sustained/result.json" \
  artifacts/reproduction/e81-sustained/capacity.test \
  -test.run='^TestProjectSwarmFourProjectControlledCapacity$' \
  -test.timeout=15m -test.v
```

To rebuild Linux packages and repeat portable acceptance, use the clean historical
checkout. Choose a new extraction/evidence directory. The portable evaluator uses
an isolated disposable profile and starts/stops only its owned service:

```sh
bash scripts/build-release.sh 0.5.0 linux-amd64
mkdir -p artifacts/reproduction/e81-package
tar -xzf artifacts/releases/0.5.0/luci-linux-amd64.tar.gz \
  -C artifacts/reproduction/e81-package
node scripts/verify-portable.mjs \
  artifacts/reproduction/e81-package \
  artifacts/reproduction/e81-portable
```

The source-export command requires a clean checkout and a new output path:

```sh
python3 scripts/export-public-source.py \
  --revision e81b586077f8af97e5e59472e34eafd161f16b8a \
  --version 0.5.0 --output artifacts/reproduction/e81-source
```

Native Windows, live-provider output quality, complete installed four-project
execution and sustained full-stack resource/interaction acceptance remain separate
release gates. A passing controlled witness or portable startup does not close
them.
