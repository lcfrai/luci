# Two corrected source fixtures

The previously reviewed source branch `84602034c` contains coordinator reviewed-source kind readback, the `e415b89a` release join and assembler input closure `42e4bdc9`. The two dedicated fixture-only corrections were merged into its descendant without changing production code:

- `39efa51222bbc4254642dc838d1a5880a29f7257`: open the assembler capture sink while the exact lease is still reserved, matching production Create ordering.
- `1c374e31955a31beac9a37d9e57946f661ac6a1a`: prove that first-admission refusal never creates the ledger or lock, while preserving the intentionally unavailable/non-numeric read observation.

The coherent merge head before this receipt was `049bbe30eadabb16ee73aacd999ea7756b1d8f4e`. Before the bounded check, cgroup charge was 33,030,766,592 of 34,359,738,368 bytes; host available memory was 2,480,979,968 bytes. The exact command in `control-plane/` was:

```sh
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm timeout 120s /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/api -run '^(TestSourceFirstAdmissionRefusalAfterPrepareDoesNotRetainPhantomAllocation|TestAssemblyPrivateHookRecoveryPreservesPublishedCaptureAsUnavailableEvidence)$' -count=1 -parallel 1
```

Result: exit 0, `ok github.com/lcfrai/luci/control-plane/internal/api 0.072s`. No backend, product, UI, prior 707, native module, Docker, model or cloud test was rerun. This is source fixture evidence, not installed preview or assembler acceptance.
