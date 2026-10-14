# Luci — approved Fourfold brand handoff

On 7 October 2026, the user explicitly selected **A / Fourfold** in the chat and asked for the logo to be sent to the Luci project for use in the app and its animations. This records the direct human choice; no review-board decision was manufactured.

## Identity and wording

Four equal, connected parts form one continuous loop around shared context. The four activities are **Discover, Design, Develop and Deploy**. Use these exact names; the user corrected the earlier word “Describe” to **Discover**. These are concurrent, interconnected activities that can be revisited, not four sequential gates.

## Approved static artwork

Use `canonical/luci-four-ds-fourfold.svg` as the source of truth. Its exact bytes are preserved from the Glasses Visual Foundry export:

```text
SHA-256 e69afb839bc6384841c53ebcc1a72953ae1c18b1a1bf672f989d38c42ca9e155
Size    3,801 bytes
ViewBox 0 0 256 256
Ink     #18181b
Canvas  transparent
```

Keep the silhouette, stroke proportions and central aperture intact. Scale uniformly; retain the 256-square viewBox and its built-in clear space (approximately 42 units on each edge). Do not stretch, crop tightly, redraw, add a background tile or turn the lobes into separate symbols. Use a 24px or larger SVG box for small interface marks; 32–48px suits navigation and wordmark lockups. Allow optical spacing between the mark and “Luci”.

Use dark ink on light surfaces and light ink on dark surfaces. The approved silhouette is monochrome; avoid gradients, glows and extra colours. `derivatives/luci-fourfold-currentcolor.svg` changes only the ink to `currentColor` for inline SVG use. CSS colour inheritance does not cross an external `<img>` boundary. `derivatives/luci-fourfold-light-ink.svg` supplies #fafafa ink for dark surfaces. These are explicitly identified delivery derivatives with their own hashes; neither replaces the canonical Foundry output hash.

Transparent PNGs are supplied in `png/light-surface/` (dark ink) and `png/dark-surface/` (light ink), at 32, 64, 128, 256 and 512 pixels. All are rasterized directly from SVG. Prefer SVG in the app. Never enlarge a small PNG to make a larger asset.

For an isolated brand mark, use an accessible name such as “Luci”. When adjacent to visible “Luci” text, make the repeated image decorative (`alt=""` or `aria-hidden="true"`). If explaining the system, label it “Discover, Design, Develop and Deploy share one project context.”

## Motion direction for the Luci project

These are implementation suggestions, not generated or approved motion assets:

- Keep the approved static silhouette visible and stable. Convey calm, continuous activity through a restrained travelling highlight or subtle shared emphasis.
- Let activity reflect real project state. Several activities may be active together; do not imply a mandatory start or finish point.
- Do not animate four rigid sequential gates, a progress percentage without real data, autonomous coordination that is not implemented, or a spinning/loading state when nothing is loading.
- Preserve the silhouette and proportion. Avoid morphing, elastic wobble, pulsing scale, flash, or a permanent attention-grabbing loop.
- Honour `prefers-reduced-motion: reduce` with the unchanged static logo. The canonical SVG is the intentional static fallback. Decorative activity should not carry information that is available only through motion.

## Foundry provenance and regeneration

`foundry/` contains the exact normalized recipe, reproducibility manifest, successful regeneration receipt, installed profile snapshot, relevant operator schemas and source provenance. All five operators are pinned at version 1.0.0; the canonical renderer is `vf.render-svg` 1.0.1. The recipe pins the exact installed `microscopy-signal` profile hash and uses explicit monochrome graph parameters. The current Foundry service cannot install arbitrary custom profiles; no custom-profile installation is claimed.

Regenerate through Glasses `foundry.export_svg` using the preserved recipe, then run `foundry.verify_regeneration` with the expected recipe hash, render key and output hash from the manifest. `checksums.json` records the file hashes of this package. `approval.json` records the scope of the direct user selection and current wording.

This handoff does not edit the Luci app, deploy anything, or claim an animation has already been implemented.
