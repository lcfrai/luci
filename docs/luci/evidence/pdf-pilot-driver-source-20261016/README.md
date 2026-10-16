# PDF pilot driver source checkpoint, 2026-10-16

- Source commit: `e719d4e70b79539c170405488f204cc360a68b4b`
- Source tree: `414a47fdcc710e63f7a7d866ce673e08a98be3f7`
- Scope: isolated `scripts/distribute-pdf-pilot/cmd/pilot-driver` source and the dedicated stage/interface plan. No production route, image, font, package, or executor registration.
- Static staged whitespace check: `git diff --cached --check`, exit 0 before source commit. Source read and manual review only.
- Formatter, compiler, tests, input acquisition, native tools, Docker, PDF render and reader: **not run** under the source-only hold.

New unexecuted selectors in `cmd/pilot-driver`:

```text
TestPDFPilotDriverRejectsArbitraryInputAndParserRoutes
TestPDFPilotDriverPDFInfoProvesOnePageBeforeOtherReaders
TestPDFPilotDriverRunsFixedInfoTextPageSequenceAndFramesOnlyResults
TestPDFPilotDriverFailedPageCountAndCancelCannotEmitResult
TestPDFPilotDriverDoesNotRetainNativeOutput
TestPDFPilotDriverFrameBindsExactRolesRecordsAndEOF
TestPDFPilotDriverFrameRefusesBadOrderOverrunAndIncompleteWrite
TestPDFPilotDriverFrameCancellationDoesNotWaitForBlockedCapture
TestPDFPilotDriverBoundedOriginalOpenRefusesLinksFIFOAndOversize
TestPDFPilotDriverUnsupportedPlatformRefusesOriginalAndStage
```

The fixed reader sequence checks `pdfinfo`'s unique `Pages: 1` field, then extracts text and first-page PNG. The proposed Debian Trixie `poppler-utils` `pdfinfo(1)` manual is identified in the driver README; exact package binary, recursive dependencies, licences, image, UID, bind ownership and bounded runtime remain unavailable. A future API owner must enforce the whole-role deadline and exact-CID retirement, bounded Docker capture/EOF and immutable original input binding. This checkpoint provides no runtime or PDF fidelity acceptance.
