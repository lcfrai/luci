# CLOUD-001 source correction evidence

[Source receipt](cloud-001-source.json) records focused product and management
regressions for commit `9a84e740ad85e4a03540f2dfb082f98cb60604c5`.
The three file hashes retained by the original checks match the corresponding Git
blobs. The checks ran before the commit; there is no complete compilation-input
manifest for these particular focused runs.

Private queue reads and priority/pause/resume/cancel controls remain available
while source allocation holds its mutex. Source-dependent routes keep their
admission guard. Separate management checks verify fresh project permissions,
revocation and Execute requirements. Commands and original-artifact hashes are in
the receipt. No additional test run was performed for this curation.

This does not establish a complete installed four-project pass, native Windows
acceptance, or coverage of the frontend correction committed at the same time.
The [e81 foundation](../e81b5860/README.md) remains a separate historical result,
including its installed failures.
