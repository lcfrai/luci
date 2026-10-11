# CLOUD-080 controlled adapter validation

Underlying exit: 0. Wall time: 21.94 seconds. Go 1.26.3, GOMAXPROCS=2, package parallelism=1, default vet enabled. Bounded to 240 seconds wall / 180 seconds Go test timeout. No dependencies installed or fetched. No real Docker, application, browser or cloud run.

Selected product tests: 31 top-level + 54 subtests. Selected platform tests: 16 top-level + 35 subtests. Total: 47 top-level + 89 subtests, no failures or skips. This is not a full-suite result or cmd/api build claim. See exact invocation and unabridged compressed JSON event log.

All 877 captured Go source, module/lock and embedded asset inputs matched before and after. Git HEAD and the full tracked diff hash were unchanged. Each overlay Go file was copied before execution and is retained in tested-overlay.tar.gz with declared member hashes. Effective main.go contains the shared coordinator's combined option/authority wiring; adapter-to-test-bindings.json distinguishes it from the adapter-only patch hunk.

The external tool go.work, tool go.work.sum when present, repository workspace files and sandbox module locks were supplemented after the run only. That file explicitly does not claim before/after hashes for those configuration files. The unchanged clean Git revision independently binds tracked repository files. Actual GOWORK used the control-plane and sandboxd modules in this checkout; no dependency fetch occurred.

The selected tests use explicit controlled typed-runtime fixtures, retaining reviewed source, safe committed archives, private configuration, health, ownership, lifecycle persistence and stale-write assertions. They do not prove shared host admission, real Docker hard limits, native default-builder availability or installed application acceptance. The API coordinator has separate evidence; merely including its cold-demand source in this overlay does not mean this run executed those API tests.

Actual acceptance remains open for supported builder execution, base-image provisioning, recoverable uncertain-build outcomes and the real application journey. Any retained unknown build must remain charged until exact settlement; this receipt is not permission to release uncertain capacity.
