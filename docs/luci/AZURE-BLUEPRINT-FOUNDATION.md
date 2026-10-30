# Blueprint-bound Azure foundation

This slice connects a generated deployment blueprint to the existing Azure
single-VM setup lifecycle. It does not implement Kubernetes apply or launch an
application/inference workload. No new route, SDK, provider login, package or
credential format is introduced.

In Deploy, generate the blueprint, expand the Azure foundation section, and
explicitly choose a resource-group name, region, domain, existing foundation
tier, planning budget and public SSH key. Review creates a saved CloudPlan only.
The existing CloudManager then retains its separate cost and unverified-price
acknowledgements before Create. Its foundation-only view shows just this plan's
job, including retained progress and owned-resource removal.
Existing account/cloud-setup privileges remain required in addition to current
project deploy rights; this integration does not widen either route policy.

The existing Small/Standard/Full Azure tiers are nominal 2 CPU/4 GiB,
2 CPU/8 GiB, and 4 CPU/8 GiB respectively. Aggregate replica requests must fit
after an explicit 1 GiB system memory reserve. Missing requests, oversize totals
and GPU requests block this foundation choice. This arithmetic does not attest
to subscription quota, available host capacity, performance or a provider price.
Kubernetes, model/PVC preparation, application image delivery and telemetry
remain explicit unresolved workload steps.

The server compiles the exact blueprint again and resolves the current project
and context. It captures a trusted original deployment actor, current journey
and application configuration identity, selected tier and complete CloudPlan
digest. A browser-supplied digest is not permission. Create reauthorizes the
same actor's project Read/Edit/Deploy rights and current provisioning,
public-exposure and budget policy before reserving the job. Context is checked
under the existing project lock immediately before reservation. Existing plan
expiry, explicit acknowledgements and saved-job idempotency are retained.

The background provider path rehydrates the original actor through the existing
DeploymentAuthorizer before commands and rechecks the bound current context.
It only admits the original creating job. A retained unknown operation cannot
be replayed through this binding; existing resource ID and luci-job ownership
tags still control observation and removal. Retained observation/cleanup requires
current deploy access but does not require a new allocation budget. Retained provider responses are checked again immediately before publication;
late grant loss withholds the response without rolling back a saved observation.
Removal rechecks the original deployer grant after the ownership read and before
the destructive command, using cleanup authority rather than a new-allocation
budget. It never
adopts an unrelated Azure group or substitutes another actor. The existing Azure
CLI account/subscription remains the operator-selected provider context; this
slice does not add a new account attestation or credential store.

Blueprint commit/image references remain intent here. VM creation does not
verify those images, build a commit or certify app readiness. After connection,
the existing Git/Dockerfile release flow has its own source, configuration,
target-fingerprint, ownership and health checks. It does not apply these
Kubernetes manifests. Azure foundation provisioning and observed VM status must
not be reported as observed application/inference delivery.

Absent optional bindings preserve exact legacy CloudRequest/CloudPlan JSON.
Existing unbound setup paths retain their behavior. This source patch has not
been formatted, compiled, tested or exercised against Azure.
