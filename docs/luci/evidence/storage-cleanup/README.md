# Windows storage and stopped-task cleanup source checkpoint

The combined source passes **2,835 Go race-test outcomes in 33 packages**, with
27 explicit test skips and two packages without tests. The complete run took
817.987 seconds with `GOMAXPROCS=2` and `-p 1`. All 1,786 tracked/nonignored file
hashes, HEAD and working-tree status stayed unchanged during execution. The
working tree was deliberately dirty over `2819ce9e`; these manifests identify
the tested bytes rather than attributing new behavior to that old commit.

The full Windows amd64 API cross-compiled in 165.579 seconds against the same
file hashes. This includes management private-file handling and startup storage
and cleanup wiring, beyond the earlier backend-only compilation. It has not
been executed on Windows. Frontend evidence is separate: [2,394 passing tests,
TypeScript and desktop/phone checks](../2026-10-08-windows-storage-ui.md).

CLOUD-005 adds explicit host-operator storage confirmation, independent host and
Docker guest capacity checks, and gateway observations shared privately across
workspace processes. [The multi-process receipt](../2026-10-08-windows-storage-multiprocess.json)
records 18 reader processes and 360 reads with zero worker probe attempts;
actual Windows invocation counts still require native measurement.

CLOUD-008 compares the observed session generation before exit updates and
removes only an exact immutable owned stopped task container without force.
Its settled launch receipt remains until backend absence, so removal failures
can retry after reconstruction. Completed manual tasks also release capacity
on subsequent admission without a Stop or Forget action. Focused checks passed
138 outcomes, including actual HTTP restart and second-project admission.
These source checks do not establish installed queue-turnover latency.

CLOUD-009 remains open: a controller interruption during backend-dispatching,
before a durable acknowledgment, has no authoritative retirement fence. Stop
can remove a child without releasing its unsettled reservation; a late
created-only container can remain held. Never delete the ledger or infer
settlement from a timeout, controller death or empty inventory. This checkpoint
is available for focused correction testing, not the finished local milestone.

- [Complete Go result and skip list](full-go-report.json)
- [Individual completed outcomes](full-go-outcomes.json)
- [Source before](full-go-source-before.json) and [after](full-go-source-after.json)
- [Focused cleanup race checks](cleanup-race-report.json)
- [Windows API compilation](windows-api-compile.json)
- [Independent guest/host runtime preparation](guest-preparation-report.json)
- [Original/curated artifact hashes](artifact-manifest.json)

Raw logs remain private; original hashes retain lineage. Curated completed
outcomes omit raw test output. The directory name is not a release assertion.
