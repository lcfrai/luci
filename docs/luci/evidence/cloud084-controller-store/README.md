# CLOUD-084/088 controller-host durable store

This follow-up adds a concrete private filesystem store and fenced journal-writer contract to the allocation boundary at1a4dda196c8de95a2a16f2fac82c00252eebb50d. It adds no dependency, service, cloud resource, model call or guest process.

**32 top-level +43 subtests passed**, zero failures/skips, on Go1.26.3 Linux with `-race`, `GOMAXPROCS=2`, `-p1`. These75 outcomes include the65 prior allocation/transfer outcomes; they are not140 unique tests. Windows amd64 cross-compilation passed, with artifact hash/size recorded before removal. Native Windows execution is pending.

Run from `control-plane` with the task Go toolchain active:

```sh
GOMAXPROCS=2 go test -json -race -p 1 -count=1 -timeout=90s ./internal/backend/e2bbe
```

Raw final output/exit, selected before/after source hashes and exact count/scope summary are in this directory. The20 selected inputs are all new boundary files, statelease source and module/workspace files; they are unchanged across validation. This is not complete compiler/standard-library/transitive dependency provenance. Prior successful iterative results overlap this final run; no failed assertion was discarded.

Actual temporary filesystem tests cover content-addressed journal and source/returned artifacts; exclusive process-held store ownership; a fenced writer that cannot publish or release its successor; exact current-head restoration into a fresh destination; no replay of the fixture provider Create; rejection of active second writer, stale/implicit/overwriting restore and foreign ownership; corruption refusal before copying; retained charge on runtime generation replacement; byte/object budgets; cancellation/malformed archive without publication; and safe cleanup of an owned unacknowledged temporary file.

The store is local to the Luci controller. Declared distinct controller/runtime host IDs and a separation receipt are required and a declared same-host setup is rejected. These test declarations **do not prove actual physical host separation, remote-provider durability or power-loss behavior**. Production wiring must establish the disk is outside the replaceable Embed host and verify restore, capacity and retention policy. No universal backup guarantee, live microVM, real provider, API integration, task execution or deployment gate passes here.

`Config.Checkpoint` now claims a fenced `CheckpointWriter` before publication; `DirectoryStore.RestoreController` uses the exact reviewed current head and never overwrites a destination journal. Stored artifacts are data, not authority or successful-check claims. Returned data must still pass project authorization, source/context drift, independent verification, review and deployment gates. Partial output on any copy error must be discarded.

CLOUD-084/088 remain open for actual provider/KVM/private5008/cost preflight, guest grants/traffic/files/process/attach wiring, shared API/scheduler integration, real fan-out and host-loss acceptance. The earlier capsule remains historical source/fixture evidence, unchanged.
