# Document assembler input-closure source check

The accepted source-only input-closure ref `42e4bdc9c4781fc43b531aa9a1fc6467604b6bba` joined the preview branch at `0498bb63a`. This check used the pre-existing Go toolchain/cache and synthetic callbacks only. No Docker engine, native image, network provider, or module output was used.

Before the first batch, cgroup charge was 33,013,936,128 of 34,359,738,368 bytes and host available memory was 2,490,707,968 bytes. The serial source command in `control-plane/` was:

```sh
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm timeout 180s /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/api -run '^(TestAssemblyClosureRef|TestAssemblyPrivatePublicationFailureCannotReleaseUnallocatedClaim$|TestAssemblySharedFenceRequiresActualACKAndExactDurableIntent$|TestAssemblyPrivateHookRecoveryPreservesPublishedCaptureAsUnavailableEvidence$|TestAssemblyLegacyMixedHistoryRefusesNewFenceIntentsAcrossPurposes$|TestAssemblyRunningPressureStopsBeforeBlockedReaderAndPublisher$|TestAssemblyUnknownCleanupAndStreamCloseNeverReleaseCapacity$|TestAssemblyEOFIsNotExitOrRegistration$)' -count=1 -parallel 1
```

It exited 1 after 5.219 seconds of test time. The sole failure was `TestAssemblyPrivateHookRecoveryPreservesPublishedCaptureAsUnavailableEvidence`, in `capture_present`, `capture_missing` and `receipt_missing`, at `document_image_assembly_capture_test.go:203`: `launch recovery ownership could not be verified`. The fixture transitions the lease through `running` before opening `CaptureSink`; the new `boundDocumentImageAssemblyHooks.CaptureSink` requires the exact owned lease in `reserved`. The production runner opens the sink while reserved, before the `creating` transition. The test needs its synthetic capture order updated by the dedicated owner; this result does not prove native capture failure. All other selected cases in this command passed.

Three exact anchored selectors in the peer's requested set were absent from the first regex. They were run once, without repeating the failed case:

```sh
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm timeout 90s /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/api -run '^(TestAssemblyRuntimeNoPartialHooksOrAuthorityBypass|TestAssemblySharedAdmissionCurrentOperatorSourceAndImmutableCAS|TestAssemblySharedFullPrivateHooksRequireDurableCaptureAndFreshAbsence)$' -count=1 -parallel 1
```

It exited 0: `ok github.com/lcfrai/luci/control-plane/internal/api 0.052s`. The coherent closure check remains failing until the owned fixture is corrected and selectively verified. Earlier 707 tests were not rerun.
