# Setup and delivery amendment — 14 October 2026

Authority: Dylan directly requested that completed work be merged onto `main`, that the README describe the application, and that first setup offer **Local**, **Bring your own cloud**, and **Luci Cloud**. Bring your own cloud must provide easy setup commands. Dylan will develop the hosted Luci Cloud service before going live. This amendment supplements LC-01–LC-16; it does not renumber or weaken them.

## Product and architecture

First setup presents the three choices together, explains where work and data run, and saves the selected mode durably. Returning users retain their projects, accounts and configured resources. Changing mode must not silently migrate, erase or expose existing project data or provision resources. Model/harness connections remain a separate, clearly explained requirement.

**Local** runs the controller on the user's computer and upstream coding harnesses in isolated local Docker. It includes dependency checks, understandable recovery, and the supported responsive resource profile. The interface remains usable while work waits for capacity; actual worker/build allocations and queue reasons stay truthful.

**Bring your own cloud** connects Luci to infrastructure controlled by the user. Azure is the first implementation target under the architecture audited at `6bf391dc956a451f9c0ce9af578650658ab269df`; AWS remains deferred. A short guided flow collects the minimum nonsecret account/region/deployment choices and produces version-pinned, copyable setup commands for a supported shell, with prerequisites, progress, safe retry, teardown and a protected connection handoff. Commands delegate to the shipped installer/configuration tooling rather than requiring users to assemble an undocumented Terraform deployment. Where an exact private image or runtime package is required, the tool prepares it or states the concrete missing input; a successful render must not be reported as a working environment.

The cloud setup plan identifies resource ownership, isolation, lifecycle, persistence and cost implications before the user runs a command that creates resources. Existing budget acknowledgement and deployment authority remain required. Secrets use protected local files, managed identity and Key Vault as appropriate; never embed them in displayed commands, URLs, public logs, Terraform state or custom data. A recovered deployment must reconcile exact identities and pending effects before retrying. Each upstream task still needs its own newly allocated microVM, with durable context and explicit retirement/charge reconciliation.

**Luci Cloud** is visible as the future Luci-hosted service, clearly marked **not available yet**. It must not accept payment, claim a connected service, provision a substitute provider, or imply that a hosted platform has been delivered. Users can choose Local or Bring your own cloud instead. Hosted accounts, tenancy, operations, billing and a live service need their own implementation and acceptance before launch; they are not silently included in the current Azure offline preparation claim.

## Requirement, implementation and acceptance map

| ID / related LC | Change and source owner | Acceptance |
| --- | --- | --- |
| SETUP-01 / LC-01, 05, 06, 12 | Local Sol: existing first-run/setup components and persisted configuration; three visible choices, explanation, durable selection and safe return/change behavior. | New and returning user flows, keyboard/narrow-screen usability, reload persistence and project-preserving transitions. No misleading connected state. |
| SETUP-02 / LC-06, 11, 14, 15, 16 | Cloud Sol: Azure setup command/plan generator and shipped installer/configuration path; Local Sol joins the UI to that exact interface after the atomic low-memory fix. | Targeted offline render/validation/refusal/resume tests on exact source; commands match packaged files and support the declared shell. Actual Azure deployment, microVM operation and teardown remain unverified until Dylan supplies an environment. |
| SETUP-03 / LC-11, 12 | Local Sol: truthful unavailable Luci Cloud option; Dylan's future hosted-service work remains explicit. | The choice cannot manufacture a hosted account, service, payment or runtime. Local/BYO alternatives remain usable. |
| DELIVERY-01 / LC-01–16 | Root: consolidate repository branch work onto `main`, resolve conflicts against the newest contract and fixes, retain exact source/merge evidence. | Branch inventory and ancestry/semantic reconciliation; no independent required feature silently discarded. Coherent main revision builds, affected regressions pass, and GitHub receives that exact revision. Historical branches need not be deleted. |
| DELIVERY-02 / LC-01–16 | Root: rewrite README around what Luci does, supported setup modes, getting started and current availability; link technical/development evidence separately. | README agrees with actual UI and shipped package. Local evidence is separated from offline Azure preparation and the future hosted service. Another-machine instructions name exact prerequisites and entry point. |

## Reconciliation and current state

The design is authorized by the direct user instruction; no new scope confirmation is required. Root records local review of the exact committed amendment and both implementation-worker acknowledgements in the central acceptance ledger before new setup functionality starts. Already aligned LOCAL-032 resource changes and the audited Azure host/fencing implementation do not change direction. Source preparation is not product acceptance. Cloud account queries, provisioning, spending and live provider tests remain deferred until an environment and concrete authority are supplied.

As of this reconciliation, SETUP-01–03 and the final `main`/README handoff are **pending implementation**. The existing local UI checks and Azure fixture/source checks do not satisfy these new acceptance items. A mapped follow-up must record the exact shipped command interface and files, rather than retaining this planning row as a feature-completeness claim.
