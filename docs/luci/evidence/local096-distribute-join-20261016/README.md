# LOCAL-096/099 and Distribute coherent source join

The source join combines `340332e3c9ae4184d1f5cc028c2694bfb12f736b` with reviewed `6795fd686d2b42e37022c82b97a7eedc484740ff`. It preserves the separate continuation and artifact histories. No Docker, model, cloud, document image or installed release was exercised.

Run serially in `/dev/shm/luci-local096` with bounded compiler settings:

```sh
cd control-plane
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=384MiB GOGC=10 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache TMPDIR=/dev/shm GOTMPDIR=/dev/shm /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/product ./internal/api ./internal/management -run 'Test(AcceptedAmendment|CoordinatorAccepted|Distribute|TeamControl|TeamStandards|ControlAssessment)' -count=1
# exit 0; exact stdout joined-go.log

cd ../desktop
TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm test -- --run src/api/distributeControls.test.ts src/api/orchestrationContracts.test.ts src/components/develop/ExecutionContractReview.test.tsx src/components/orchestrator/CoordinatorExecution.test.tsx src/components/distribute/ArtifactWorkspace.test.tsx src/components/distribute/DistributeWorkspace.test.tsx
# exit 0; 6 files, 99 tests; exact stdout joined-ui.log
TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm run typecheck
# exit 0; exact stdout joined-typecheck.log
```

The checks establish only source integration. Accepted-amendment actual work, independent review, artifacts, exports and installed Windows behavior remain subject to their separate real execution gates.

SHA-256: `joined-go.log` `1e7b74b0010ba2039494c5e4783aa3c93e66f840fd677c3bddb88fbb52d65216`; `joined-ui.log` `db7645ce5c5c05abaf1cebb777130a23871e7ee1b85390f5d9e8bc5185e4e6dd`; `joined-typecheck.log` `6d51e3ec9a9d283eee096e5e9403c5d963c9468e9756fd61f3e9e0d3df1bde71`.
