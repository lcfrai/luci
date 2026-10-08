# Windows storage settings: frontend evidence

The current frontend passed **2,394 tests in 145 files**, with no failed,
skipped, pending or todo tests. The run used at most two Vitest workers and took
274.35 seconds. All frontend files and the shared management HTTP fixture had
identical hashes before and after the run. TypeScript also passed on that exact
source manifest and was reused rather than rerun without a change.

This checks source changes based on `2819ce9eacf62b2d73a1522b2db25301a7b393d4`
with an explicitly dirty working tree. It does **not** establish that the
installed `2819ce9e` package contains the new storage flow. The source manifests
identify the code checked; a later package must have its own provenance.

## What the checks cover

The frontend client validates the same complete JSON fixtures that the Go
management HTTP test compares against actual handler responses. It preserves a
measured zero, distinguishes Docker guest free space from the conservative
minimum across mounted local host volumes, and rejects unexpected response
shapes. The source fixture is
[`execution-storage-http.json`](../../../control-plane/internal/management/testdata/execution-storage-http.json).

The component and setup tests cover explicit review before confirmation,
read-only polling, uncertain writes without an automatic retry, private-data
clearing on a fresh permission error, and stale-request fencing across account,
team and permission changes. A layout-effect regression checks that switching
between two authorized accounts cannot paint the previous account's private
storage path. Runtime preparation respects the backend storage check without
making a Windows desktop block a selected cloud workspace on local storage.

The portable browser helper rendered the actual storage component, controls and
CSS in Linux Chromium at **1440×1080** and **390×844**. It checked full-path
review, keyboard focus and Tab order, content geometry, zero confirmation
callbacks before consent, one explicit confirmation followed by read-only
polling, and zero automated axe findings. The phone review screenshot was also
visually inspected. Authentication, capacity values and storage callbacks in
this browser witness are deliberately controlled fixtures.

## Reproduce and inspect

- [Portable helper instructions](../../../scripts/acceptance/windows-storage-ui/README.md)
- [Summary JSON](2026-10-08-windows-storage-ui.json)
- [Full test receipt](windows-storage-ui/full-frontend/result.json),
  [all individual test outcomes](windows-storage-ui/full-frontend/vitest-raw.json),
  and [test log](windows-storage-ui/full-frontend/tests.log)
- [Source hashes before](windows-storage-ui/full-frontend/source-before.json)
  and [after](windows-storage-ui/full-frontend/source-after.json)
- [TypeScript and restored-test receipt](windows-storage-ui/final/restored-coverage-followup.json)
  and [TypeScript log](windows-storage-ui/final/typecheck-restored.log)
- [Browser receipt](windows-storage-ui/browser/result.json),
  [phone review](windows-storage-ui/browser/storage-review-390.png),
  [desktop review](windows-storage-ui/browser/storage-review-1440.png),
  and [phone initial state](windows-storage-ui/browser/storage-initial-390.png)
- [Original and curated artifact hashes](windows-storage-ui/artifact-manifest.json)

The full test reporter counts 252 nested suites within 145 files; these are not
252 separate files. Its raw outcome list is retained. Text evidence replaces
only the absolute checkout prefix with `<repo>`; the artifact manifest records
both original and curated hashes.

Earlier unsuccessful fixture attempts remain in the history directory. The
first focused run exposed an older setup test fixture without the newly required
storage check; the fixture was corrected without weakening its assertion. The
first browser scaffold omitted Vite's React preamble and was corrected before
the passing witness. A final diff review also caught accidentally removed
pre-existing `LocalTools` tests: all three were restored byte-for-byte, the new
regression was appended, and the restored file plus TypeScript passed before
this complete frontend baseline. These intermediate checks are not counted as
additional independent product acceptance.

## Limits

This batch runs no Docker containers, worker processes, model calls or cloud
operations. The browser helper makes no real management HTTP request and does
not measure physical storage. Automated axe checks supplement interaction and
visual review; they are not an accessibility certification. No production build
was run in this frontend batch.

Native Windows still needs its own installed acceptance: fresh Super Admin
authority, the actual Docker Desktop active/pending layout, VHD/settings/engine
identity invalidation, complete host-volume and guest observations, and the
resulting new-work admission behavior. A UI fixture cannot satisfy those gates.
