# CLOUD-084/088 — trusted guest output producer (controlled source scope)

This slice connects bootstrap to a root-owned task cgroup and adds the fixed
Linux collector. It preserves source/context/operation/VM/PID/boot/inode bindings,
waits for actual freezer observations, retains original source changes/deletions,
and emits a bounded verified archive plus exact metadata. It implements neither
a coding engine nor a running cloud product backend. Native Exit, artifact
integrity, independent checks/integration and VM/billing settlement remain
separate facts. No live cloud/model/Windows/performance claim is made.

## Changes and meaningful checks

- Root bootstrap records its exact identity and moves the exec process and all
  future descendants into a fresh undelegated cgroup. The separate collector
  validates the original protected input and launch markers, refuses running
  within that group, checks boot/inode and observes frozen=1. Failure/cancellation
  never unfreezes the task or releases its reservation.
- Capture retains original paths regardless of modified ignore files, records
  additions/modifications/deletions, and excludes only fixed new cache/dependency/
  credential/Git paths. It refuses links, special/unowned/cross-device files,
  changed identity, oversized input and lost freeze. Exclusive capture intent
  and immutable output prevent blind retries after partial publication.
- The first real fixture found a loader bug: decoding `{}` into a reused marker
  structure inherited fields from the prior prepared receipt. Original failure
  and source are retained; decoding each marker into a fresh value fixes it.
- Independent source review found a second policy bug: an original file under
  `.cache` forced directory traversal, which included a new generated sibling.
  The same regression fails before and passes after applying fixed exclusions
  to ancestor segments while preserving the original modified file.

## Evidence, without cumulative test inflation

`runs/first` failed overall: guest14 top-level/43 subtests passed,1 top/1 sub
failed,1 existing root-permission test skipped. E2B transport59 top/116 sub passed;
both command packages compiled (no tests). All882 captured repository inputs
were stable. `runs/marker-fixed` then passed15 guest top/44 sub,1 root skip,
882 stable inputs. These runs overlap.

`runs/exclusion-before` retains the single new policy counterexample failure.
`runs/exclusion-fixed` passes16 guest top/44 sub,1 root skip,884 stable repository
inputs. That is the final guest result, not a sum of prior counts. The separate
[native observation capsule](../cloud084-native-observation/README.md) records its
own later transport65 top/127 sub passes and overlapping guest15 top/44 sub before
the final exclusion correction. No unchanged controller suite was repeated
solely for this guest policy correction.

The fixtures use real files, Git/archive behavior where relevant and controlled
kernel observations. This non-root environment **does not verify real freezer,
mount/UID permissions, KVM isolation, trusted template binaries, live provider
calls, whole-machine responsiveness or per-VM teardown**. The earlier root-Docker
attempt failed before Start and remains separately retained in the bootstrap
capsule; it is not a successful prerequisite here.

`source-bindings.json` contains17 current source copies checked against the final
884-input capture. Full before/after repository Go/module inventories are gzipped;
external Go SDK/modules and exact executed binary bytes are outside that scope.
Two original implementations and the original pre-follow-up test file are kept
under `before/`. `representation-bindings.json` binds decompressed original
receipts/logs to publication bytes. `run-controlled.py` is the final reusable
wrapper, not a claim that earlier executions used its later-added case branches;
original invocation command/environment/time/floors and outputs are retained
separately for every run. Race tests used Go1.26.3, GOMAXPROCS2/-p1 and bounded
memory-backed cache. Test-only space guards remain512MiB root/256MiB tmp/4GiB
cgroup remaining; product admission limits were not changed.

Use the template requirements in [images/microvm/README.md](../../../../images/microvm/README.md).
Controller collector invocation, durable output import/review, product backend
selection, authenticated model egress, host-loss recovery and real12-task/
4-project/4-concurrent-VM acceptance remain open.
