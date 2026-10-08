# shadcn/ui components in Luci

Luci adapts the MIT-licensed Radix `new-york-v4` registry at immutable commit
`97ddbf4274ed09d02aa7fd34375f2cf1e48cbbf0` of https://github.com/shadcn-ui/ui.
`upstream/` preserves the original source bytes; `LICENSE` preserves the full grant.
`manifest.json` records the source and adapted hashes for redistribution checks.

The button keeps shadcn's CVA variants, Radix Slot composition and data-slot
contract. Input, textarea and badge follow the same native and Slot primitives.
The dialog uses the canonical Radix portal, overlay, content, title and
accessible description composition. Luci substitutes its semantic color,
elevation, motion and density tokens, existing modular Radix dependencies,
local font and icons. It adds an explicit native button type and compatible
size/container/position dialog options. No installer code is run.

Existing Fluid button/dialog import paths are compatibility adapters to these
implementations. Fluid remains separately attributed for the retained tabs,
size context and icon contract. This is an incremental migration; it does not
claim that every hand-built Luci component came from shadcn.
