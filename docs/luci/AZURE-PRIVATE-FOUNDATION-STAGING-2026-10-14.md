# Azure private foundation and controller staging reconciliation

This completes the guided Bring your own cloud flow authorized by Dylan on 14 October. It extends the audited Azure architecture and private management join without changing LC-01–16 or authorizing live cloud operations. Root has reviewed the following ownership and ordering. Implementation begins after the affected workers acknowledge this committed record; actual account queries, resources, spending and deployment still await a supplied environment and concrete authority.

## Private prerequisites and foundation

The user's setup command first performs authenticated read-only inventory of the selected account, tenant and an existing privately reachable VNet, address space, subnets and DNS. It verifies an existing private route with an explicit user-run probe. It may propose two unused /24 subnets only after validating available address space; insufficient space or a missing route produces a precise prerequisite. Never create implicit peering, a public address, Bastion, VPN or another connectivity gateway.

Reuse suitable existing private Blob state and Key Vault resources as the fast path. Otherwise show an exact resource, ownership and cost plan for explicit acknowledgment before creating a separately owned foundation: state resource group/storage/private container, private Key Vault and only the necessary private endpoints and DNS links in the selected existing VNet. Public access stays disabled. Every effect requires exact absence preflight and a durable local pending journal before the call. Lost acknowledgments reconcile through authenticated reads without replaying a possibly completed create.

The local journal precedes remote backend availability. After exact private backend readback, normal infrastructure state lives there and references the foundation IDs. Foundation ownership remains separately journaled and tagged, never silently imported or discarded. The main module may reuse the selected existing VNet and create only owned controller/runtime subnets and NSGs. Publish exact versioned credentials through the private Key Vault route before dependent runtime/controller application.

## Controller-side initialization

Runtime gateway TCP 443 remains reachable only from the controller. The ordinary setup host must not call that private runtime gateway directly or broaden its firewall. Initial fenced disk initialization starts protected account management with an explicitly deferred, unrunnable operator selection. A bundled controller setup command, reached through authenticated management, fetches exact Key Vault versions using controller managed identity and reuses the existing Embed template adapters. Durable effect journals live on the mounted controller disk.

Return only nonsecret template/node receipts over the authenticated ARM/SSH management path. Bind source, template, runtime node, disk, controller boot and fenced generation to exact readbacks. Install the signed final operator selection with a pending/no-replay journal and validate it before activation. Finalization must not format the already initialized disk. Missing or uncertain effects stay visible and resumable; setup receipts do not establish real microVM execution.

## Ownership and evidence

| Requirement | Owner | Evidence before source handoff |
| --- | --- | --- |
| SETUP-02 guided private foundation and resumable commands; LC-09/11/12 | Local Sol `/root/main_branch_reconciliation` | Existing-resource reuse; private-only plan/refusal; address collision; local-to-remote state transition; exact create/lost-ack/readback and interrupted resume fixtures |
| SETUP-02 controller initialization; LC-06/15/16 | Same local Sol | Deferred operator cannot run tasks; controller-only gateway access; exact managed identity and versioned secrets; signed selection and generation mismatch refusal; no second format; durable template journals |
| Protected connection UI | Cloud Sol after the API contract is committed | Import bounded nonsecret receipts, verify host fingerprint and exact authenticated generation before Ready; concise progress and actionable prerequisites |
| Retirement and retained charges | Local Sol, root acceptance review | Settle tasks and verify backup before compute retirement; keep foundation, DNS/endpoints, gallery, Key Vault and disk retention/charges explicit; unknown deletion or billing outcomes remain unresolved |
| Contract alignment and main delivery | Root | Exact worker ACKs, ledger audit, integrated revision and clear distinction between offline evidence and later live Azure acceptance |

Local runtime distribution has separate cloud/Windows ownership under its own reconciliation. Do not edit its importer, producer or package paths without root coordination. No nested delegation, live cloud calls, public connectivity shortcut or silent product scope reduction is authorized here.
