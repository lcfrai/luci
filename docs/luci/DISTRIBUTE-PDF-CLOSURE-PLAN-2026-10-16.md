# LC21 PDF renderer: selected source closure and bounded pilot

Status: source plan only. No module, font, renderer, or image has been added to a build or release. PDF export continues to refuse with 422. This plan refines [the renderer proposal](DISTRIBUTE-PDF-RENDERER-PROPOSAL-2026-10-16.md); it does not grant dependency acquisition or runtime execution.

## Exact candidate inputs

| Purpose | Immutable input | Verified module sum or original-byte SHA-256 | Grant to retain |
| --- | --- | --- | --- |
| PDF objects/content stream | `github.com/benoitkugler/pdf v0.0.15`, commit `85f51161fb69eb2439ded4532d7bf8191f055072` | `h1:FDatswON2irQJv2+02Q2ojKqoUPbwPxIbLguX4kBExM=` | MIT, original LICENSE SHA-256 `dec6a32c1421f5f31234e6e0bcb9e148c44a75f929fe18c49f84b0e028798a5d` |
| Complex-script shaping | `github.com/go-text/typesetting v0.3.3`, commit `c3ca8673d4734bb5b16df1b08c5f27af6c9b6329` | `h1:ihGNJU9KzdK2QRDy1Bm7FT5RFQoYb+3n3EIhI/4eaQc=` | choose BSD-3-Clause from `Unlicense OR BSD-3-Clause`, original LICENSE SHA-256 `6b1bf94c77d7faa0ce94f8745595bfaf4c404f8b3873f5c2ce55ca15f8e44eca`; HarfBuzz subtree MIT SHA-256 `e45d96558ba3d406f95dffed6f22c3be07093475ef617bcfddf88444d6b66788` |
| SFNT face and subset | `github.com/tdewolff/font v0.0.0-20261010204837-88a7f6b56918`, commit `88a7f6b5691888bf23e957963fc9e49ff39dc16d` | `h1:z7YAbjYz/bmG0AUx2pqmClRxUI2aCxP0rQMJAMcdmok=` | MIT, LICENSE.md SHA-256 `375ceacf38f92020f8896427fbb3e7259c72add31218107c78c692b43ade4f2f` |
| Latin / Arabic / CJK faces | Debian original `NotoSans-Regular.ttf`, `NotoSansArabic-Regular.ttf`, `NotoSansCJK-Regular.ttc` | `89c3c497f618fdaa0b2d1e98fef93582f28c71debd2c4a8cdf41f190ced2909d`, `504d7407d86875acf7d04dfaa0fd7524d0b8797723bc4aa18022f29db25b0b6e`, `b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a` | SIL OFL-1.1 and exact Debian package notices; source packages `fonts-noto-core 20201225-2`, `fonts-noto-cjk 1:20240730+repack1-1` |

The three font candidates total 20,241,528 bytes before subsetting, below the 32 MiB **aggregate input** bound. This does not establish that a selected CJK face subsets below the 16 MiB **PDF output** bound. Debian copyright SHA-256: core `075f91382875da1c4b491f1facf2586f55448705133264d98ce15cab6211b125`, CJK `849f4ea9c214fa4ac3593b770c699f387534b11ce671264c1b10d85bdcb5997b`. Debian package metadata and installed bytes were inspected read-only; a later release must obtain and bind the original selected files through an approved source path, with no host-font fallback.

## Source-selected transitive candidates

Read-only source import traversal selected `pdf/contentstream` and `pdf/model`, 13 internal packages; `typesetting/shaping`, 11 internal packages; and the `tdewolff/font` root. Their external imports select these modules. The table records upstream sumdb `h1` values, **not an acquired or built final Go graph**.

| Module/version | h1 | Original grant choice / source note |
| --- | --- | --- |
| `github.com/benoitkugler/pstokenizer v1.0.1` | `3+18uif4Dg4+w84AmkWPKOujhPKbLnkgxP1eb/KtiGg=` | MIT; inspected licence bytes match pdf MIT grant above |
| `github.com/hhrutter/lzw v1.0.0` | `laL89Llp86W3rRs83LvKbwYRx6INE8gDn0XNb1oXtm0=` | BSD-3-Clause; inspected licence SHA-256 `2d36597f7117c38b006835ae7f537487207d8ec407aa9d9980794b2030cbc067` |
| `github.com/andybalholm/brotli v1.2.2` | `HzTuoo2ErYQqf5qvcJInB8uvqSVxRttzkFexPWtnceM=` | MIT LICENSE SHA-256 `3d180008e36922a4e8daec11c34c7af264fed5962d07924aea928c38e8663c94`; retain nested flate BSD grant SHA-256 `2d36597f7117c38b006835ae7f537487207d8ec407aa9d9980794b2030cbc067` if selected |
| `github.com/golang/freetype` pseudo `v0.0.0-20170609003504-e2365dfdc4a0` | `DACJavvAHhabrF08vX0COfcOBJRhZ8lUbR+ZWIs0Y5g=` | choose FTL from FTL/GPL; original LICENSE SHA-256 `d3ba056adc2b7909e95681deaae397fb37c97ed491a920f491214f07b62c41d0`, `licenses/ftl.txt` SHA-256 `f4b133e25df1f86ad3ffea453aa0e613f0474f34778dbbb3e437e7b2724937d8` |
| `github.com/tdewolff/parse/v2 v2.8.16` | `bLk5svUOQRkW/Y2SJ+DeENSIkZBcTIkq+Atyv5D8feI=` | MIT LICENSE.md SHA-256 `0ebcdb0e336d600038ecdb7b8d339d937a66b62cb6d1128896fdc20cfe01c188` |
| `golang.org/x/image v0.45.0` | `FMb1nTbH5H9vF55SriQHgFw5GnNL9Jg6L25BwXKzhB0=` | BSD-3-Clause; acquire exact original grant during approved pilot |
| `golang.org/x/text v0.41.0` | `vz/seA0lnX87Othu2f/0L24RcgrXD9/YFTSuGjj3rH8=` | BSD-3-Clause; selected-version cached original LICENSE SHA-256 `911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad`, PATENTS SHA-256 `96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc`; release inclusion remains subject to closure |
| `golang.org/x/sys v0.47.0` | `o7XGOvZQCADBQQ4Y7VNq2dRWQR7JmOUW8Kxx4ZsNgWs=` | BSD-3-Clause; selected-version cached original LICENSE and PATENTS have the same respective SHA-256 values as `x/text` above; release inclusion remains subject to closure |

`typesetting-utils` appears in the shaping module's `go.mod` but not its inspected shaping source imports; `tdewolff/font` declares a broad indirect Canvas graph but its inspected root imports do not select Canvas. The approved pilot must compute the **actual** package/linker closure with the pinned toolchain, lock all transitive checksums and grants, and refuse any unreviewed extra package. A `go.mod` declaration alone is neither release inclusion nor evidence of exclusion.

The `x/text` and `x/sys` grant hashes above were calculated from already-present selected-version module-cache files without adding dependencies or invoking a renderer. The exact `x/image v0.45.0` source grant is still unverified; this is a release closure gap, not permission to substitute another version's notice.

## Fixed shaping/subset interface to prove before production export

The pilot consumes the exact saved artifact bytes and pinned font manifest. It segments original Unicode into clusters, shapes each run with script/direction/language and the selected face, and records every shaped glyph's original cluster, face index, glyph ID, advance and offset. Assign a distinct bounded PDF CID for each `(face, glyph ID, original Unicode cluster)` mapping; repeated glyph IDs with different clusters **must not** share a ToUnicode mapping. Refuse more than 65,535 CIDs. Group glyph IDs by face, call pinned `SFNT.Subset(glyphIDs, KeepPDFTables)`, then map requested old glyph IDs to the returned order and account for appended composite dependencies before writing CIDToGIDMap/embedded streams. A CFF2 face refuses. Generate ToUnicode from original clusters, not from a reverse glyph lookup. Measure line breaks using shaped advances and preserve original logical text order in extraction; a visibly correct page with reordered or missing extracted text fails.

The pilot must prove exact round-trip extraction and page layout for Latin ligatures, Arabic joining/RTL, combining marks, CJK, table wraps, multi-page source references and unsupported-glyph refusal. It must show selected CJK TTC face subsetting as a standalone font rather than embedding the 19 MiB collection. Missing glyph/script, invalid cluster map, oversized subset, clipped row, cancellation or any unknown writer behavior returns an explicit refusal; there is no raster fallback or silent glyph replacement.

## Acquisition and resource pilot plan (not executed)

1. After separate source/dependency approval, fetch only pinned original module zips and the three original Debian font/package inputs into a task-owned bounded scratch location. Verify the listed module sums and font SHA-256, original licence/notice bytes, file count/path safety and package graph before changing `go.mod` or the release inventory. Do not use host fonts at export time.
2. Build a **test-only** shaped-glyph/subset writer under one capped process. Charge at least 512 MiB shared host admission, hold `ExecutionStateDir/distribute-work.lock` across any concurrent managed API worker, cap input font bytes at 32 MiB, intermediate at 32 MiB, final PDF at 16 MiB, artifact at 256 KiB, pages at 512 and whole request at 30 seconds. Check cancellation in shape/subset/layout/write and retain no success on cancel. No external renderer process or remote service.
3. Compare independent text extraction with each original cluster and visually inspect page breaks/tables on the supported desktop. Record peak host/cgroup memory, elapsed time, retained allocations and positive cleanup. Only then choose production integration and release-package/licence closure. This pilot does not establish installed Windows/Office fidelity, model quality, public delivery or full LC21 acceptance.
