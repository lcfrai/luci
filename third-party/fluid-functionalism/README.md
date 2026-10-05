# Fluid Functionalism in Luci

Luci reuses the official Radix Button, Tabs Subtle, Dialog and their shared helpers from
[Fluid Functionalism](https://github.com/mickadesign/fluid-functionalism), pinned
to commit `97183f8018e619722edffd2f9fccefbea0effe8d`. The original MIT grant and
copyright are retained in LICENSE. Glasses discovery/inspection identifiers and
source fingerprints are recorded in manifest.json.

The retained source is in desktop/src/components/fluid. Imports are local,
navigation keeps Luci's existing icons, native buttons default to type=button,
and loading announces aria-busy. Tabs support reduced motion and clean up their
label observer. The utility compiler is scoped to these components and omits
Preflight; styles use Luci's light/dark/accent tokens and density. Inter's
variable font is served from the local bundle; its original SIL Open Font
License is collected from @fontsource-variable/inter with release notices.

Registry metadata is discovery evidence. Vendored code and licence evidence come
from the immutable repository revision. The selected components use React19,
Radix and Motion; no documentation-site code, remote fonts, telemetry or new
hosted service is included. Updating retained source requires updating the
normalized source hashes after the intended adaptations; release collection
checks them. Normalization changes CRLF to LF for Windows checkouts.
