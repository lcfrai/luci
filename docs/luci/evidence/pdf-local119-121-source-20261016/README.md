# Held PDF pilot source corrections — 2026-10-16

Parent source: `ce58ef668507d1c5887577d4311e5cacd5490748` (the PDF + Debian-domain join). This source successor changes only `scripts/distribute-pdf-pilot` and does not acquire inputs, compile the pilot, render a PDF, run a container, or enable product export.

- LOCAL119: `bounded_regular` refuses unsafe stages and each linked/nondirectory ancestor before a nonblocking, no-follow ordinary-file open; it holds the directory chain and checks original file identity, size and timestamps after a bounded read. `regular`, `module_zip_h1` and `module_mod_h1` all use it. The opener is a POSIX source gate; a platform without these primitives refuses. It does not establish the still-required owned-stage or hard process containment.
- LOCAL120: ZIP names containing Unicode control characters, including LF, refuse before Go h1 canonicalization. The synthetic single-member newline alias has the exact canonical hash text of a different two-member ZIP and is rejected.
- LOCAL121: `makePDF` checks the complete font + subset + PDF aggregate after writing xref and trailer. `TestMakePDFCountsCompletedTrailerInAggregate` exercises equality and one-byte overflow at the 32 MiB boundary; the Go test is **unrun** while the selected offline modules and bounded compiler capacity remain unavailable.

Focused checks on this changed source, using `LUCI_PDF_PILOT_TMPDIR=/dev/shm` and `timeout 15s python3 -B -m unittest -v`:

| Selectors under `test_source.PilotSourceTests` | Result |
| --- | --- |
| `test_all_staged_reads_refuse_fifo_before_open`, `test_staged_read_refuses_changed_leaf_and_linked_ancestor`, `test_module_h1_rejects_newline_alias_and_control_names` | 3 passed, 0.054 s after final opener change |
| `test_module_h1_uses_exact_zip_members_and_rejects_unsafe_names`, `test_go_mod_h1_is_separate_from_raw_sha` | 2 passed, 0.004 s |
| `test_bounded_original_read_rejects_oversize_and_link` | 1 passed, 0.002 s after restoring linked-file refusal wording |

The first attempt failed before any fixture ran because `/var/tmp` had no free space. The fixed run used the existing task-owned `/dev/shm` temp setting; no unrelated files were removed. The pinned `/dev/shm/luci-toolchain/go/bin/gofmt` formatted the changed Go sources; `git diff --check` passed. No Go compiler or PDF runtime was started.

Fresh source-check capacity remained below the existing Go profile: cgroup `memory.current=33,653,739,520`, `memory.max=34,359,738,368` bytes, leaving **705,998,848 bytes**. Host MemAvailable was `1,594,952 kB`. The five joined `TestAssemblyDebian*` API cases and the new Go PDF boundary test remain unrun; earlier receipts must not be interpreted as results for them.
