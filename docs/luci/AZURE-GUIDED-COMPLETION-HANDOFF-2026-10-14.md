# Guided Azure setup implementation checkpoints

This continues published `afbdbc80baf5c69f3bee890dd1adaaae6fff1844` and incorporates
management reconciliation `0dd0bff783f571ce4bfcb449b942552078a4c29d`. LC-01–16,
per-task microVMs, private mTLS, durable disk/fencing and the live authority gates
are unchanged. No account query, provisioning, spend or cloud acceptance occurred.

The first checkpoint adds `scripts/azure-setup/guided.py`: ordinary account
questions reuse the existing selection validator; status aggregates retained
credential/image/gallery/infrastructure/connection/retirement journals; gallery
publication uses an exact existing generalized managed-image source, bounded
absence preflight, one journaled PUT and exact source/identity GET reconciliation.
An interrupted write stays pending and cannot be retried by deleting/replacing a
selection. Budget estimates are neither enforced Azure billing caps nor invoices.
The exact bundle inventories and API command list include the guided module.

Six focused Python adapter tests pass on Windows (fixtures perform no Azure calls),
covering private CIDR refusal, exact publication readback, lost ACK/restart, stale
selection, incomplete absence inventory and truthful progress. Python syntax checks
pass. These adapter fixtures deliberately use a portable private-store test double;
they do not replace the existing Linux POSIX private-file/installer checks. No Go,
TypeScript, Vite, Docker or browser campaign has run for this checkpoint.

Still being implemented: deferred operator/template configuration and finalization,
generalized VHD image preparation/publication handoff, exact private management
host identity/connection descriptor and desktop join, owned retirement and retained
charge handoff. Existing image-root preparation alone is not a publishable disk or
a signed guest runtime. The management network rule remains gated on the root's
final owner ACK record. No entry in this file constitutes product acceptance.
