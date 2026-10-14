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

Follow-up SETUP-02-R3 requires explicit `vcs.modified=false` as well as the
exact revision; absent/dirty build information is refused by a focused helper
test. The ARM list reader now reuses the bounded authenticated JSON transport
without passing list data to the single-resource interface (Linux identified
the original compile failure). SETUP-02-R4 adds explicit same-controller reboot
resume: systemd mounts the already initialized UUID, verifies the original
signature plus retained installation receipt, and reads unchanged generation/
disk receipts without formatting, writing or signing. Active restore selection
is retained for subsequent restarts. The API still enforces the off-disk fence;
the desktop's old boot pin remains invalid. Eight operator fixtures and the
controller package test pass locally. Replacement-controller signed successor
staging remains unfinished; these fixtures are not live recovery acceptance.

The authorized `34f7bab2` foundation implementation adds `foundation.py`, its
installer commands and read-only guided discovery. Account/tenant/operator,
existing VNet/subnets/DNS and an explicit private listener probe precede any
proposal. It derives two collision-free /24s, reviews exact privately configured
state storage/container, vault, endpoints, DNS and minimal operator RBAC, then
journals each operation locally before a PUT. Unknown responses reconcile by
GET without replay, including an absent read. Verified existing private
state/vault/DNS resources remain externally owned. Existing linked DNS zones
are dependencies rather than recreated resources. Private DNS plus real
authenticated Blob/vault data reads precede backend handoff. Terraform reuses
the selected existing VNet and foundation DNS, with moved blocks for older
owned resource addresses. Read-only data actions are separated from fresh
managed creates in the installer plan guard; managed adoption/update is still
refused. Budget and all final bills remain unverified.

Eight new foundation fixtures and ten affected guided fixtures pass locally,
covering success/backend handoff, reuse, existing DNS, collisions, missing
route, cost/absence refusal, private drift and interrupted response/restart.
Python syntax and diff checks pass. POSIX installer/package and Terraform
validation are not run locally; no account/provider operation or compiler was
started. No real Azure/KVM/recovery acceptance is claimed. Next source remains
actual image staging, signed replacement-controller successor and retirement.

Under exact image-producer reconciliation `308507c5`, normal guided controller
images now omit the selected deferred operator, public CA and service environment.
These bounded nonsecret files instead enter the existing authenticated recovery-
to-controller initial RunCommand after concrete resource readback. The same
off-disk bootstrap intent precedes staging/mount; retained files must match exact
bytes and reconciliation only inspects them. Private-key content is refused.
The account gateway/loopback API and managed-identity references remain intact.
Nine operator fixtures pass, including post-create staging/refusal, and Python
syntax passes. The two variables.tf formatting lines reported by independent
Terraform checks are corrected. Legacy advanced complete-operator image input
retains its compatibility behavior; generated normal images are generic.
Private builder implementation proceeds under the now-aligned `308507c5` gate;
no builder or provider was started by this worker.

Retirement readback groundwork adds the controller-only `retirement-observe`
action. It requires a stopped account unit and empty service cgroup, zero ready
node sandbox/starting counts, authoritative capacity-ledger decoding and exact
owner/scope task-journal validation across personal/team workers. It returns
only state-file digests and settlement flags, never releases capacity or marks
provider billing settled. The new read-only helpers reuse existing decoders;
fixtures allocate actual mock-provider attempts/shared reservations and refuse
retirement until exact settlement, plus foreign/schema refusal. New Go checks
are written and formatted but not run locally: `go test -p 1 ./cmd/api
./internal/api ./internal/backend/e2bbe -run 'TestAzureSetupRetirement|TestAzureRetirement' -count=1`
with `GOMAXPROCS=1 GOMEMLIMIT=512MiB` on independent Linux. This checkpoint is
groundwork only: durable retirement fence, backup and owned compute lifecycle
adapter are still being implemented, with all real Azure outcomes untested.

The image producer batch follows exact `308507c5` and `f32a6929` ACK/gates.
`builder.py` prepares source archives without local compilation, plans exact
temporary resources/cost/lifetime, and journals each private staging/allocation/
reader grant/guest install/deprovision/deallocate/generalize/VM-source gallery
capture/owned retirement operation. It enforces sequential fresh role builders,
private NICs, tagged private OS disks and no active generation configuration.
The temporary system MI reader is container-scoped; source bytes are downloaded
and fully hashed rather than trusted from metadata. `builder_guest.py` verifies
original commit/tree bytes (without shipping Git history), compiles exact clean
source on the temporary VM, resolves immutable upstream service image digests,
installs the existing source package and preserves original licence/NOTICE
inventory. No service/task or microVM is started as preparation evidence.
Generalized VMs are never reused. Published versions retain source VM/package
bindings separately from temporary resources, grants and staging cleanup.
The installer accepts only exact bound producer receipts as the remote package
path, retaining the local-package compatibility path.

Eight new lightweight fixtures pass: complete two-role source/capture/cleanup,
cost/private/single-active refusal, source byte drift, exact reader identity/
role/scope, lost create/install/generalize ACK with read-only restart, traversal
refusal and real local Git source reconstruction. Python syntax/diff checks pass;
no remote build, Go compile, Docker or provider operation ran locally. New Go
changes are bundle inventory only; independent Linux should run the new builder
suite plus affected installer/bundle tests. Real guest apt/Go/registry commands,
private egress, provider property normalization, gallery timing/permissions and
all live outcomes remain untested. Material source work remains: normal guided
progression/source-prerequisite simplification, incomplete-builder abort,
retirement fence/backup/compute adapter. Signed successor recovery ownership is
exclusively transferred to root's cloud Sol (operator_setup.py, mount.py,
recover.py, test_operator.py, test_recovery.py); installer joins await its schema.


Producer abort follow-on (14 October): `builder-step --stage abort` now durably
records explicit cancellation, prevents new production/image binding and advances
at most one provider mutation per invocation. It preserves unknown prior intents
and requires their exact read-only reconciliation; observed deallocation precedes
VM deletion. Temporary container-scoped grant principal/role/scope is exact. An OS
disk that precedes the tagging PATCH is bound by provider immutable uniqueId while
the owned VM still proves attachment, then deleted only after owned VM deletion
and exact unattached identity readback. Planned resources with no completed create
are never adopted; unrelated or replaced resources remain preserved. Both aborted
roles allow only owned staging cleanup. Gallery/foundation/runtime retention and
unverified final charges remain explicit. Three new offline regressions cover
incomplete untagged-disk cleanup, unknown create/delete acknowledgments with no
mutation replay, and replaced-disk/retained-gallery refusal. Full builder file:
11 tests passed; Python syntax/CLI and diff checks passed. No live Azure, compiler,
Docker, KVM, cloud task or billing acceptance. Remaining normal guided progression,
source guest/loopback registry, signed replacement successor (transferred owner),
and final whole-installation retirement are still tracked independently.


Loopback guest source checkpoint follows exact fa7cc890 reconciliation. New
`azure-embed-host/guest_registry.py` uses Distribution source commit
9ed95e7365224025ee89365e12cf128e1f1bf965 and the official 3.0.0 linux-amd64 archive
SHA256 61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d;
original LICENSE SHA256 cb5e8e7e5f4a3988e1063c142c60dc2df75605f4c46515e776e3aca6df976e14.
The existing temporary runtime builder resolves guest FROM digests, builds the exact
microVM source, preserves original application/vendor/upstream grants plus an
installed-grant inventory, probes actual harness versions in positively owned
network-isolated containers and removes only observed owned probes. It preloads
actual pinned Embed service images, seeds actual OCI bytes into the local registry,
then tests GET/HEAD and exact write refusal before sealing the root-owned source.
No source preparation occurs on runtime boot. The installed host package binds the
full source receipt, includes the source verifier, and profiles the upstream default
base-template/ready path out of the Azure default. Native systemd ordering, immutable
configuration and full filesystem/manifest/config/layer verification cover initial
boot/reboot. A fixed read-only ARM RunCommand inspector gates actual controller
`template-create`/`template-start` on the exact source and signed guest digest; its
unknown result cannot permit a template mutation. It adds no cloud resource/RBAC
or public/private-interface listener. The normal signed guest is derived from
producer readback rather than a user-authored digest. Registry source restoration
explicitly does not restore task/template metadata, projects or reservations.

Local evidence: new registry tests 8/8 pass, new normal guest derivation/substitution
test 1/1 passes, affected builder tests 11/11 pass before the later bytecode-only
guard (unchanged behavior), AST checks and Go formatting pass. Host package tests
were attempted once but local Python lacks PyYAML; no dependency was installed.
Go compilation/test, actual released native registry behavior, Docker build/probes,
Azure readbacks, KVM/task execution and provider billing remain unrun here.
Independent Linux commands (sequential):
`python3 -m unittest discover -s scripts/azure-embed-host -p 'test_*.py' -v`;
`python3 -m unittest discover -s scripts/azure-setup -p test_operator.py -v`;
`GOMAXPROCS=1 GOMEMLIMIT=512MiB go test -p 1 ./cmd/api -run TestAzureGuestRegistry -count=1`
from control-plane. Source-bundle helper/inventory ownership is separately with
root's cloud Sol; join requires the packaged sparse source helper and its inventory
(including the new registry module), not ordinary user private-GitHub access.
Remaining owned implementation: normal continue progression and source-helper join,
verified builder Envoy extraction, signed successor installer join (returned five
files), and whole-installation retirement fence/backup/compute handoff. These are
unfinished capabilities, not provider/environment-only acceptance gaps.
