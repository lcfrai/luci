# LC-21 PDF renderer and font reconciliation proposal — 16 October 2026

**Status: proposed, no dependency acquired or PDF route enabled.** The native
DOCX/PPTX/XLSX candidate is `ad116af86`; PDF still returns a specific 422.
The current desktop/API release has no portable PDF layout/shaping/font bundle.
An installed `soffice`, browser print command, remote converter, or the LC-18
document extractor would introduce an unowned dependency or execution path.

## Exact candidate inputs for review

The Go-only candidate is `github.com/go-pdf/fpdf` **v0.9.0**, upstream commit
`504c6dd8cc916cd7f2097877efd52cae5f1d8b18`, Go checksum
`h1:PPvSaUuo1iMi9KkaAn90NuKi+P4gwMedWPHhj8YlJQw=`, MIT (upstream pinned
LICENSE inspected). Complex-script shaping candidate is
`github.com/go-text/typesetting` **v0.3.3**, upstream commit
`c3ca8673d4734bb5b16df1b08c5f27af6c9b6329`, Go checksum
`h1:ihGNJU9KzdK2QRDy1Bm7FT5RFQoYb+3n3EIhI/4eaQc=`, selectable
BSD-3-Clause under its `Unlicense OR BSD-3-Clause` upstream LICENSE. Versions,
commits and module checksums came from read-only Go proxy/sumdb metadata;
neither module was added to `go.mod` or fetched into this repository. The
critical compatibility question remains **open**: fpdf must accept positioned
glyph IDs/advances from typesetting without dropping shaping or ToUnicode
mapping. A standalone working module name is not evidence of that join. If it
cannot, reject this pairing and select a pinned native Go PDF writer exposing
those primitives before any production dependency change.

The inspected font candidates are original Debian-installed Noto bytes; they
are **not** bundled release inputs yet. They must be obtained later through a
reviewed original-source/package path, verified byte for byte, and accompanied
by the original OFL grant and package notice in each release artifact:

| Candidate and inspected package version | Bytes | SHA-256 | Licence |
| --- | ---: | --- | --- |
| NotoSans-Regular.ttf, `fonts-noto-core 20201225-2` | 512,672 | `89c3c497f618fdaa0b2d1e98fef93582f28c71debd2c4a8cdf41f190ced2909d` | SIL OFL 1.1 |
| NotoSansArabic-Regular.ttf, `fonts-noto-core 20201225-2` | 244,072 | `504d7407d86875acf7d04dfaa0fd7524d0b8797723bc4aa18022f29db25b0b6e` | SIL OFL 1.1 |
| NotoSansCJK-Regular.ttc, `fonts-noto-cjk 1:20240730+repack1-1` | 19,484,784 | `b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a` | SIL OFL 1.1 |

These three files are a starting coverage set, **not all Unicode**. Font
fallback must use an exact packaged manifest, reject a missing glyph or
unsupported script/shaping feature before returning a PDF, and never replace
it with a blank box or host font. Other scripts and emoji need separately
pinned original fonts or an explicit recoverable refusal. Full Unicode
acceptance stays open until representative coverage and text extraction pass.

## Proposed effect and resource boundary

Reuse the existing exact saved artifact/revision/digest source and private
`GET .../exports/pdf?digest=...` route. Perform an effects-free glyph/layout
preflight before encoding. Map section headings, paragraphs, bullets, source
references, evidence gaps, assumptions, review attribution and current/stale
warning to bounded table/list pages. Split on measured shaped advance widths,
keep line and paragraph continuation marks, and reject a row/page that cannot
fit rather than clipping, rasterizing text or silently eliding it. Include
embedded subset font streams and ToUnicode maps; selection/copy must recover
the source Unicode text. No hyperlinks, active PDF actions, external references
or mutable font lookup.

The PDF path must hold `ExecutionStateDir/distribute-work.lock` across the full
render/response, shared by managed API workers and workbook work. Use the
existing 30-second request deadline and fresh host sample, but charge **at
least 512 MiB for PDF** in addition to the configured host reserve until
measured worst-case font/layout memory supports a tighter value. Limit source
artifact to 256 KiB, installed font inputs to an exact manifest and 32 MiB
aggregate, at most 512 pages, at most 32 MiB intermediate page data, and a
16 MiB final PDF. The current binary output cap remains; do not stage an
unbounded second copy or spin up a renderer process. Cancellation is checked
during glyph mapping, each line/page and final write; canceled or over-limit
renders produce no success response. Saved artifact bytes are never mutated.

## Review gates before implementation or acceptance

1. Confirm a pinned Go renderer can consume the pinned shaping output and
   embed/subset these fonts with exact ToUnicode text. A simple Latin demo is
   insufficient; include Arabic shaping, CJK, combining marks, page breaks,
   multi-column source/evidence tables and an unsupported-glyph refusal.
2. Obtain and verify original font/module licences, transitive module closure,
   package bytes and release-bundle inventory for Windows and Linux. No host
   font search, dependency download or package mutation happens during export.
3. Prove exact saved content can be extracted from every page, with no clipped
   row or omitted reference. Inspect representative pages visually on the
   supported desktop; check the configured memory reserve and cancellation on
   the constrained host. Reader and source fixtures alone are not acceptance.

Until these gates pass, the existing PDF 422 is the truthful user outcome.
