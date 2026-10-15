# Distribute export and dependency boundary — 16 October 2026

The current Go API and desktop release dependencies contain no pinned DOCX,
PPTX or PDF renderer. The cloud executor has host conversion commands, but
they are not verified portable/native package dependencies and must not be
assumed present in ordinary Windows setup. The existing browser and LC18
extractor do not authorize a new document renderer or model execution.

The additive [artifact v1 interface](interfaces/distribute-artifact-v1.json)
uses the existing actor/project store and review/CAS boundary. Preparation
derives a real editable draft from saved project content and exact private
check/release evidence. It labels unknowns and assumptions. It never starts a
model, browser, Docker workload, publication or paid provider operation.

Native Go `archive/zip` and XML encoding can produce bounded DOCX, PPTX and
XLSX Open Packaging Convention bytes without a new executable dependency.
Source fixtures must reopen every member, check relationships and source
content, and confirm that workbook strings cannot become formulas. Each
export refuses if its entire section/table content cannot fit the format's
declared bounds; truncation is not fidelity. Export is a private read with a
content digest, exact project/revision and visible draft/stale notice.

Workbook admission must account for the existing 2 MiB compressed and 8 MiB
expanded XML limits, at most 2,000 rows and one decoded workbook per request.
Use one shared in-process parse/export slot with a bounded acquisition wait and
request cancellation; a busy slot returns a recoverable response before ZIP
decode or output allocation. Serial assessment reports and future artifact
exports charge their complete projected output and one active encoder to the
same slot. Never queue unbounded request bodies, retain multiple expanded
workbooks, or run conversion in the UI thread. The current XLSX route bounds
individual requests; the shared concurrency admission is required before
claiming constrained-PC export acceptance.

PDF needs a concrete font and layout renderer with original licence,
version/digest, packaging for each supported host, bounded memory/time/output,
positive owned cleanup and representative visual/text fidelity checks. Until
that exact dependency and containment path is reconciled and packaged, the
PDF route must refuse clearly; it must not fall back to an unpinned host
`soffice`, `pandoc`, Chromium, remote service or browser print dialog.

Live model quality, installed Windows exports, actual fidelity in Office
readers, and external delivery remain separate acceptance gates. No export
publishes content or attests compliance.
