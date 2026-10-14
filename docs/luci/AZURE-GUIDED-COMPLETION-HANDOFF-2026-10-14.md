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

Root's independent Linux run of checkpoint `ef50d02d` passed both new management
tests and all 21 then-current setup Python tests; root integrated that checkpoint
in `563ac8af`. Those unchanged checks were not repeated locally.

The controller staging batch follows the authorized `34f7bab2` reconciliation.
Its signature tests do not establish live gateway access. It adds
`azure-setup/operator_setup.py`: generated deferred operator
and exact private template-tool configuration, signed guest selection bound to a
chosen immutable OCI digest and Luci/runtime revisions, existing journaled Embed
template create/build/status adapters, then provider/gateway/node-bound signed
operator finalization. The mount adapter verifies the source-pinned signature
before any disk formatting and installs exact durable template journals. A
pending remote operator installation can only be reconciled through a readback
branch; it cannot repeat its write. Private template-tool roots now exclude
public system CAs. Controller image preparation generates its previously missing
service environment, and binds measured package hashes automatically; both exact
gallery receipts can be bound with `bind-images`, without editing Terraform JSON.

Seven focused portable Python tests pass with actual ECDSA/OpenSSL verification
(Git-bundled OpenSSL on Windows), including tampered signatures, multiple/stale
runtime identities and lost installation ACK. The nine prior portable adapter
tests pass. Full Linux installer/controller checks and compilation remain pending
for this newer source. No provider call or product acceptance is claimed.

The independent identity API checkpoint adds authenticated
`GET /v1/setup/cloud/azure/identity` with version/state/sourceCommit/scope/generation,
controllerResourceID/dataDiskID/bootID and `microVMExecutionVerified:false`.
Optional `SSHRequest.azureIdentity` carries that exact pin plus its fingerprint;
the server requires RFC1918/luci/22 and only accepts the matching inspected key.
Activation, restart reconnect and status compare authenticated current controller
source, disk and boot/generation against the imported pin. A changed controller
cannot replace the chosen workspace. A pinned Azure connection refuses the old
compatibility Docker installer. `desktop/src/api/product.ts` exposes the same
optional type; AzureSetup UI/client ownership is with root's cloud UI worker.
Descriptor timestamps use `observedAtUTC`; `dataDiskID` is explicit. Three focused
new product tests are written but not run locally while the host reserve is low:
`GOMAXPROCS=1 GOMEMLIMIT=512MiB go test -p 1 ./internal/product -run
'^TestAzure(ConnectionPin|SSHInspect|WorkspaceActivation)' -count=1` from
`control-plane`. No full compile, Vite or browser work was run.

SETUP-02-R2 adds an actual request boundary check, because a disconnected status
alone did not stop forwarding through an existing SSH tunnel. Project/harness/
deployment proxy requests now compare the current authenticated Azure identity
before forwarding or requesting a websocket ticket. Release preview preparation,
existing release preview bridges, platform browser/data joins and backing admin
operations use the same check. Failure marks the selected workspace unready and
keeps every remote mutation behind the refusal; generic SSH retains its existing
behavior. New mocked remote-counter tests cover stale project/harness/deployment
POSTs, websocket tickets, new and already opened preview joins, plus matching-pin
and generic-SSH controls. These checks are not a runtime reproduction or acceptance.
Run `GOMAXPROCS=1 GOMEMLIMIT=512MiB go test -p 1 ./internal/product -run
'^TestAzureWorkspace(RequestGate|PreviewGate)' -count=1` for the affected new cases;
they have not been run locally while the host reserve is low.

The earlier handoff called this proxy issue R1; the canonical issue is R2.
R1 remains the bounded archive/identity-read issue fixed in `596a4925`.

The controller-only `luci-api azure-setup` command uses managed identity and the
existing private Embed template adapters. The setup host sends only nonsecret
source/resource/template choices through authenticated ARM RunCommand and reads
bounded receipts. Template create/build intents remain on the durable disk;
unknown responses cannot replay mutations. Initial disk initialization starts
account management with an explicitly deferred, unrunnable task operator.
Finalization requires the exact controller boot, ARM ownership, generation,
gateway, ready node and immutable template journals. The image-pinned signing
key verifies the final operator before installation on the existing filesystem;
disk and generation receipts are preserved. The service tmpfs is explicitly
mode 0700. A guest selection signature binds an OCI digest; it is not a guest
execution or content measurement receipt.

Current lightweight checks: seven operator tests, nine guided adapter tests,
one controller package test, Python syntax and `git diff --check`. New Go checks
remain unrun locally. From `control-plane`, the independent Linux check is
`GOMAXPROCS=1 GOMEMLIMIT=512MiB go test -p 1 ./cmd/api ./internal/management -run
'TestAzureSetup|TestAzureWorkerSelection' -count=1`; also run the setup and
controller-disk Python suites on POSIX. No Windows-specific Go expectation is
introduced. New-account private foundation, actual base-disk image staging and
owned retirement remain unfinished source, alongside all live Azure outcomes.
