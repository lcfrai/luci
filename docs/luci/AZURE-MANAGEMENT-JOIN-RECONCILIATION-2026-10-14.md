# Azure private management connection reconciliation

This reconciles the protected connection handoff required by the 14 October setup amendment with audited architecture `6bf391dc956a451f9c0ce9af578650658ab269df`. The controller's current deny-all inbound rule prevents the existing desktop SSH connection from reaching it. LC-01–16 and their live acceptance gates remain unchanged. This record authorizes offline implementation after root review and worker acknowledgment; it authorizes no cloud operation.

## Connection and initialization

Use the existing authenticated, host-key-pinned SSH transport only between the desktop and the separate Luci controller. The controller product API stays on loopback with its existing identity and authorization controls. This management connection does not provide Docker Develop: cloud tasks continue through the Azure adapter into independently owned microVMs, with the existing controller-to-Embed private mTLS gateway and exact runtime identities.

Add an optional, explicitly selected RFC1918 management source CIDR and a controller-only TCP 22 allow rule ahead of the existing inbound deny. Reject public, wildcard and unrestricted sources. Keep all other inbound denial and runtime gateway rules unchanged. The setup machine must already have a private route to that network; expose the exact missing route prerequisite rather than claiming connectivity. Do not silently create a public address, Bastion, VPN or another billable gateway.

Generate a deferred, source-bound operator selection before resources exist. Finalize managed identity client IDs, VM generations and template build IDs only from journaled authenticated readback. Obtain the SSH host key through authenticated readback tied to the exact controller resource and generation, not unauthenticated key scanning or trust on first use. Issue a nonsecret connection descriptor containing the private endpoint and pinned public host identity. Credential selection stays in the existing protected connection flow.

## Ownership and acceptance map

| Requirement | Implementation and owner | Required evidence |
| --- | --- | --- |
| SETUP-02 protected desktop connection; LC-09/11 | Local Sol `/root/main_branch_reconciliation`: guided installer, descriptor/API/UI join, controller network variable/rule and package inventory | Offline private-CIDR refusal, exact resource/generation/host-key binding, stale descriptor and failed-connect cases; later real private connection and account-authority checks |
| SETUP-02 generated configuration; LC-06/15/16 | Same Sol: deferred selection, finalized operator/template configuration after authenticated readback | Offline missing/mismatched identity, interrupted creation/readback, duplicate-safe resume; later actual per-task microVM execution and retirement |
| Existing runtime protection | Existing Azure provider/host code; any necessary narrow join stays with the same Sol | Preserve separate identities, private mTLS, durable disk/fence, template provenance, uncertain outcomes and retained charges; no claim of live acceptance from fixtures |

Root owns this reconciliation, the central acceptance ledger, integration and review of any additional scope change. Cloud Sol retains exclusive LOCAL-033/034 local Docker runtime recovery work and does not edit these Azure paths after its clean `596a4925` handoff. No nested delegation.
