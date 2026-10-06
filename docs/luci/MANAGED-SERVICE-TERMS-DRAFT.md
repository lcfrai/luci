# Luci managed service terms — draft for operator review

Draft date: 6 October 2026. **Not published, not an active customer agreement.**
This draft describes the implementation available for review. The operator must
complete the publication decisions below, obtain appropriate legal review and
publish the resulting agreement before configuring its HTTPS URL for checkout.
Checking this file into Git does not activate terms or create a paid subscription.

## Service and operator

The managed service provides an authenticated Luci team workspace for shared
project context, source, coding-agent sessions, checks, deployment and application
management. The operator provides the commissioned host and administers the
service. Each first-offering managed host is bound to one team. A membership or
subscription is not ownership of the underlying host or a promise of unlimited
compute, storage, model usage or availability.

The published agreement must identify the operating legal entity, business
address, contact channel and service region. Those details have not been selected
in this draft. The name lcfr.ai identifies the product's maintainer; it does not
substitute for identifying the contracting party.

Luci's own open-source code remains available under the repository's MIT license.
The managed-service agreement governs hosted operations and payment; it does
not replace MIT or the licenses of bundled dependencies. A customer may choose
self-hosted Luci without buying this managed service.

## Accounts and team authority

Use accurate account details, keep credentials and invitation codes private and
only invite people who should have access. Team owners and administrators control
membership, project sharing and additional capabilities. Running agents, using
credentials, deploying applications and administering access have separate grants.
An editor can change shared context; a viewer can read the projects shared with
them. Removing a member prevents subsequent authorized access, but cannot recall
files, transcripts, exports or information that person already obtained.

A team owner may start billing, access the billing portal, and submit or cancel a
data-deletion request. Installation operators retain host-level administration.
Team access controls do not make the workspace inaccessible to its operator.

## Customer content and agent work

Customers retain their existing rights in the source, designs, specifications,
data and other content they provide. They authorize processing and storage needed
to operate the workspace, run requested tools, provide recovery, and support
authorized operations. The published agreement must define any additional
support access and confidentiality obligations; no broad content-training license
is proposed by this draft.

Only upload content and connect repositories or services you are permitted to
use. Customers are responsible for reviewing generated code and designs,
third-party licenses, application security, deployed content and the handling of
their own users' data. Agent output, a successful check, a review receipt and an
HTTP health response each provide limited evidence. They do not guarantee that an
application is correct, secure, legally compliant or suitable for a particular use.

Do not use the service to access systems without authorization, distribute
malware, abuse other users or providers, or violate applicable law. Service
operators may restrict actions or suspend execution to address misuse, security
incidents or service operation. The published agreement must specify the notice,
appeal and emergency-suspension process and its contact channel.

## Third-party providers and deployment

Customers choose configured coding-agent, model, repository and deployment
providers. Requested actions may send project files, context, prompts, tool
results, credentials or deployment artifacts to those providers. Their own terms,
data handling, availability and account limits apply. Review provider settings
before including confidential or regulated data. The operator must publish which
providers it supplies directly and which accounts customers supply themselves.

Git publication, public applications and external resources can outlive a session
or subscription. A service cancellation, team-access removal or deletion request
does not automatically stop or erase them. Provider charges, model usage,
domains, taxes and other excluded costs must be disclosed in the actual offer.
An app budget estimate is not a billing cap imposed on an external provider.

## Price, billing and cancellation

The operator's published offer and Stripe checkout must state the actual price,
currency, billing interval, tax treatment, included resources and exclusions.
This draft sets no price or allowance. The implementation records an owner's
acknowledgement of the displayed offer and the configured terms and privacy URLs
before creating a checkout session. A return from checkout alone is not evidence
that a subscription is active; Luci reconciles provider-confirmed status.

Stripe processes payment and provides the billing portal. A scheduled cancellation
may leave a subscription active until the end of its paid period. Luci displays
the provider's current status. When billing no longer permits work, or an operator
suspends service, new execution and deployment can be restricted while permitted
reading, context access, export and resource cleanup remain available. These
permissions do not guarantee indefinite hosting of retained data.

Billing cancellation and data deletion are separate. The published agreement must
state renewal, refunds, failed-payment handling, cancellation effects, any grace
period and how price or agreement changes are communicated, subject to applicable
law. None of those commercial policies is silently established by this draft.

## Export, recovery and leaving

Use the project export and context files to keep portable copies of your work.
Project exports intentionally omit private identity, permissions, credentials and
trusted approval records; they are not complete application or host backups.
Live databases, storage volumes and external applications require their own
backups. Operators have a separate private host-backup procedure whose archives
contain credentials and other sensitive data.

An owner can request operator review of permanent team-data deletion and cancel a
pending request. A saved request is not confirmation of erasure and does not
cancel billing. The supported operator tool verifies the exact pending request,
stopped writers and a matching private backup before removing the host's verified
state files. It retains a startup-blocking tombstone and a minimal external
receipt. It does not securely wipe storage media or remove backup archives,
Docker volumes, cloud snapshots, published repositories or provider records.

The operator must account for those separate resources, publish retention periods
and confirm the actual scope completed. The published agreement must explain
export access after cancellation, backup expiry, legal retention and the request
contact channel. The current software does not promise an automatic erasure
deadline or perform a scheduled purge.

## Availability and legal terms still to settle

No uptime, support response time, recovery time, recovery point, security
certification or production-provider acceptance is promised by this draft.
Operators must establish any such commitments from their actual tested service.
Security updates, maintenance windows and incident communications need an
operational process before paid launch.

Qualified review must finalize warranties, liability limits and exclusions,
indemnities if appropriate, consumer rights, termination rights, dispute resolution,
governing law, jurisdiction, age requirements, notices and agreement changes for
the actual operator and customers. These are publication blockers, not permission
to omit mandatory rights or substitute the code license for a service agreement.

## Publication decisions

Before this becomes customer-facing terms, record the actual legal operator and
contact; service location and audience; quoted offer and refund/renewal policies;
retention and cancellation/export windows; security/support commitments;
subprocessor and transfer terms; applicable data-processing agreement; and the
legally reviewed agreement provisions above. Publish a dated, versioned HTTPS
page and a matching privacy notice, then configure those exact URLs for billing.
The billing receipt records URL/configuration acknowledgement; it does not archive
the full policy text. Preserve the published versions in the operator's records.

See [privacy draft](MANAGED-SERVICE-PRIVACY-DRAFT.md),
[data lifecycle operations](MANAGED-DATA-LIFECYCLE.md), and
[host recovery](MANAGED-HOST-RECOVERY.md).
