# Luci managed service privacy notice — draft for operator review

Draft date: 6 October 2026. **Not published, not an active privacy notice.** This
inventory describes the reviewed application and its operator procedures. It
does not establish an operator's legal identity, hosting region, retention policy
or compliance certification. Complete the publication decisions below before
using this notice for a paid service.

## Scope and responsible operator

This notice concerns information processed by the managed Luci service. The
published version must name the operating entity, privacy contact and address and
explain its responsibilities for account/service information and customer content.
Depending on the arrangement, a customer may control project/application data and
the operator may process it on that customer's instructions. A suitable
data-processing agreement, lawful bases and transfer terms require review for
the actual service and jurisdictions.

Self-hosted Luci stores and processes information on the infrastructure its
operator configures. Installing the MIT-licensed app does not, by itself, send a
copy of all project content to lcfr.ai or create a hosted subscription. Connected
agent, Git, model, payment and cloud providers receive information according to
the actions and configuration used.

## Information used to provide the service

| Category | Current application records and purpose |
| --- | --- |
| Identity and access | Account IDs, usernames, display names, password hashes, installation roles, team membership, project grants and account status authenticate people and enforce access. The core account-entry flow does not require an email address. |
| Sessions and invitations | Hashed authentication tokens with expiry, invitation-code hashes, issuing/accepting actors, team scope, permissions, expiry and revocation support sign-in and invitation use. New invitation secrets are shown once and should be shared privately. |
| Project work | Source and Git history, definitions, specifications, sketches/designs, plans, discussions, agent conversations, transcripts and selected provider/model configuration support collaborative development. Tool output or customer files can contain personal data and secrets. |
| Execution and delivery | Session/check status, bounded output/history, selected successful check commands, source digests, private approvals, deployment/release records, health observations, database-backup records and resource identifiers support execution, diagnosis and release evidence. |
| Credentials | Connected-provider tokens, API keys, agent sign-in state, Git credentials and SSH material are kept in the configured private workspace/host locations so requested actions can use them. File access restrictions are not an encryption-at-rest guarantee. |
| Attribution and administration | Authenticated actor IDs/names, action, time, affected project/team, context revision/digest, confirmed or unconfirmed mutation status, membership/service changes and deletion-request receipts support collaboration, access administration and diagnosis. |
| Billing | Stripe merchant/configuration/price identifiers, customer and subscription IDs, provider status/periods, checkout/session and idempotency references, an owner's price/currency/interval/tax/terms/privacy acknowledgement, and bounded webhook event IDs/digests/status receipts reconcile paid-service access. Luci does not intentionally retain card details, billing addresses, payer email, payment-method details or complete Stripe webhook payloads in this billing record. |

Stripe receives the information entered into its checkout and billing portal and
handles it under its own terms and privacy notice. The fact that Luci's billing
record omits card details does not imply Stripe omits them. Operator support
systems, reverse-proxy/access logs, infrastructure monitoring and backups may
collect additional information; their actual use and retention must be added to
the published notice. This draft does not claim an unexamined host collects no IP
addresses, logs or telemetry.

## Sharing and access

People with project access can read shared content. Editors can change context;
separate capabilities govern execution, credentials, deployment and team access.
Team administrators can access all projects in their team. Current owners can
submit or cancel data-deletion requests. Installation/host administrators can
administer private service state. Logical team isolation does not prevent
host-operator access and is not end-to-end encryption.

When an authorized person runs an agent or connects a provider, the configured
tool may transmit prompts, files, context, tool results or credentials to that
provider. Git pushes and deployment actions transmit source or artifacts to
selected destinations. Public publication can make information available to
anyone. Provider retention, training choices, subprocessors and regions depend on
the actual account and contract; Luci cannot promise those choices on a
provider's behalf. Publish the operator's actual hosting and supplied-provider
list and evaluate customer-selected providers before sensitive work.

This implementation adds no right for the operator to sell customer content or
train a general model on it. The published notice must accurately state the
operator's actual uses rather than infer a business policy from missing code.

## Browser storage and private host storage

The browser stores the current bearer token in session storage. Local browser
storage keeps interface preferences, selected workspace and scoped unsaved
context drafts. Drafts are scoped by account, workspace and project but remain on
that browser until its applicable discard/cleanup behavior or the user clears
site data. Shared-device users should sign out and clear local site data when
appropriate. Exported/downloaded files are under the recipient's control.

The managed gateway keeps identity and membership separate from a team's private
worker state and project folders. A project export is intentionally portable and
does not carry authentication, credentials or trusted private approvals. Private
host archives include account/session state, credentials and ignored project
files. The archive tool creates restricted files but does **not** encrypt them;
operators must secure and encrypt off-host copies through their backup system.

## Retention and deletion

Authentication sessions expire after twelve hours; invitation codes expire after
48 hours. Expiry stops their authorization; it is not a promise that all related
records, backups or audit events are immediately erased. Management activity is
bounded to the installation's most recent 1,000 events, with a team admin viewing
at most 200 matching team events. Private context attribution retains at most
2,000 events per project subject to its storage-size bound. Old context revisions
can be archived and explicitly pruned; the current context remains separately
available. A bounded record count is not a time-based retention guarantee.

Project data, conversations, connected credentials, private evidence, database
backups and billing records have no universal timed deletion promise in the
current application. Operators must set and publish actual retention periods,
legal/accounting exceptions, backup expiry and deletion procedures. A teammate's
removal, revoked login, cancelled subscription, archived context or stopped
application does not automatically erase those records or external copies.

Owners can submit a deletion request and cancel it while pending. The receipt
identifies the authenticated actor and request; it does not claim deletion.
After review, the operator can remove the backed-up, verified files of a stopped
bound managed host using the separate procedure. That operation unlinks files;
it is not secure-media sanitization. A minimal startup-blocking tombstone, lock
file and external processing receipt remain. The proof archive, earlier backups,
cloud snapshots, Docker volumes, deployed applications, published Git content,
Stripe and other provider records require separate handling. The operator must
record what was actually completed and what remains, including justified legal
retention. Interrupted erasure requires operator inspection and is not reported
as complete.

## Requests and security

The published notice must give an accessible contact channel for privacy,
correction, access/export and deletion requests, explain verification and any
applicable statutory rights or complaint channels, and describe customer versus
operator responsibilities. The owner-only team deletion workflow does not replace
individual data-subject rights or an operator's obligation to handle them.

Current safeguards include authenticated gateway routing, scoped permissions,
private records, bounded inputs, ownership checks, immutable evidence binding and
stopped-host recovery checks. These reduce particular risks; they are not a claim
of independent certification, guaranteed confidentiality or production penetration
testing. Deployment operators remain responsible for HTTPS, host/volume security,
access controls, patching, monitoring, protected backups and incident handling.

## Publication decisions

Identify the controller/processor roles, legal operator and privacy contact;
actual hosting countries and subprocessors; lawful bases; data-transfer measures;
time-based retention and deletion deadlines; financial/legal retention; log and
monitoring inventory; support access and incident notification; applicable rights;
and the method/date for notifying changes. Have the notice and any DPA reviewed
for the actual offering. Preserve dated published versions and configure the
resulting HTTPS URL alongside the terms URL in checkout. This source draft alone
has not been presented to or accepted by a paying customer.

See [terms draft](MANAGED-SERVICE-TERMS-DRAFT.md),
[data lifecycle operations](MANAGED-DATA-LIFECYCLE.md), and
[host recovery](MANAGED-HOST-RECOVERY.md).
