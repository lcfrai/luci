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
a signed guest runtime. No entry in this file constitutes product acceptance.

The second checkpoint follows both owner ACKs of `0dd0bff7`. Terraform adds an
optional explicit RFC1918 management source, port 22 to the controller's exact
private IP only, ahead of inbound deny; runtime 443 rules remain unchanged.
`configure-private` derives an unused private /16 selection and records that an
existing private route is a prerequisite. `connection-descriptor` performs
authenticated ARM VM and guest reads, checks VM UUID, boot UUID, exact fenced
generation/disk/scope, then exports a pinned Ed25519 public host identity. It never
uses keyscan, creates connectivity services or claims private reachability.

`management/azure_worker_config.go` is the narrow trusted launcher join: the
management service selects `LUCI_AZURE_OPERATOR_TEMPLATE`, keeps its account
gateway, and derives separate per-user durable controller/staging/transfer paths
for each worker. Disk owner/scope, managed identity, Key Vault references and
mount/fence checks remain intact. Parent model credentials and foreign E2B config
remain outside the worker environment. Only the trusted launcher supplies the
derived `LUCI_E2B_CONFIG`; the controller itself is not a naked worker service.
The existing secret adapter already gives each worker its own private temporary
material beneath the configured tmpfs and removes it on close.

Credential generation now matches the pinned Embed host's `e2b_` plus 32 hex
format; other independent credential material remains 256-bit. Nine portable
Python adapter tests pass, including exact host identity and stale generation
refusal. Azure CLI stdout is bounded while the process runs. New Go tests are
**not yet executed** pending the resource slot: from `control-plane`, run
`GOMAXPROCS=1 GOMEMLIMIT=512MiB go test -p 1 ./internal/management -run
'^TestAzureWorkerSelection' -count=1`. This covers separate user state and exact
identity, excluded inherited secrets, out-of-mount paths, inline credential and
duplicate selection refusal. No Windows-specific behavior is expected; existing
private-file permission guards intentionally differ by operating system.
