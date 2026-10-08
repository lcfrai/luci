# Correction package and source export

[Package/source receipt](package-source.json) records the clean Linux amd64 build
from `2819ce9eacf62b2d73a1522b2db25301a7b393d4`: 996 extracted members matched,
347 licence components, none missing. The source export contains 1,514 files.
Package and source ZIP hashes were independently recomputed during curation.

Source ZIP SHA-256:
`179e11de73383b74fd4550ead9a4528bc9027de400292b15eb5ec39108502e42`.

The receipt also binds the [correction checks](../native-lifecycle-correction/README.md)
to this commit where retained hashes permit: 578 full-product input hashes,
336 affected API input hashes, two catalogue files and five disk-admission files.
These scopes overlap. They do not turn the earlier CLOUD-006 full API run into
a full API pass on all later changes. No tests or builds were repeated.

Installed runtime and observer receipts are maintained separately. The
[later fixture-helper correction](../git-fixture-helper-isolation.json) and
subsequent Windows backing-storage work are outside this immutable package.
Package integrity, source tests and deterministic workflows do not establish
completion of native Windows or real-provider acceptance.
