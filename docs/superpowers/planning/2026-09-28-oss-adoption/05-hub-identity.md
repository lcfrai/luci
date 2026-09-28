# Phase H — the Hub: what we can take from open source

**Research only. Nothing was built, nothing was run, no source file was changed.** Written
2026-09-28. Every licence below was read from the project's own `LICENSE` file or package
metadata at the URL given, on **2026-09-28**, not from a badge or from memory.

**Scope.** Phase H is the shared server: identity (Keycloak brokering Entra, `OD-30`), tenancy,
the brief store, approvals, the **provenance ledger**, audit and releases. 44 planned tasks, 6
approved. Postgres underneath. Audit is append-only; brief revisions and the ledger are immutable
(`provenance-contract-draft.md` §1.2, §2.1).

## The headline

**The Hub is the phase with the best open-source yield in the programme, and almost none of it is
a framework.** Five findings, in order of how much work they remove:

1. **Identity is a solved library problem.** `coreos/go-oidc` (Apache-2.0) plus
   `golang.org/x/oauth2` (BSD-3-Clause, *already an indirect dependency of the control plane*)
   plus `zalando/go-keyring` (MIT) covers the whole OD-30 flow — discovery, PKCE, loopback
   redirect, JWKS ID-token verification, refresh, Windows Credential Manager. That is roughly
   6–8 of the 44 Hub tasks reduced to wiring and tests.
2. **The provenance ledger should adopt a FORMAT, not a library.** **in-toto Attestation
   Framework** (Apache-2.0) gives us a `subject` + `predicateType` + `predicate` envelope that
   already means "this thing was produced from that thing" and is already understood by every
   supply-chain tool a regulated customer runs. Our brief key `brief:b-42@3` + digest is a
   `subject` and a custom `predicateType`. This is the single highest-value item in this
   document, because it makes the product spine **interoperable** instead of proprietary.
3. **Do not adopt an authorization engine as a service. Consider Cedar as a library, later.**
   OpenFGA and SpiceDB are both Apache-2.0 and both excellent, and both are a *server* we would
   have to run, back, secure and upgrade inside a customer's cloud. At 44 tasks and one tenancy
   model, they are a wall of configuration — exactly what `CLAUDE.md` says has failed our bar.
4. **immudb is out.** It **relicensed to BUSL 1.1** and is now forbidden by the licence rule.
   This is recorded below with the licence text URL, because a blog-level search still describes
   immudb as Apache-2.0 and someone will propose it again. Same story, same trap: **HashiCorp
   Vault is BUSL from 1.15**, and **Redis is RSAL/SSPL/AGPL from 8.0**.
5. **A bug we have not hit yet, found by reading the Win32 docs rather than a repo.** Windows
   Credential Manager caps a secret at **`CRED_MAX_CREDENTIAL_BLOB_SIZE` = 5×512 = 2560 bytes**.
   `OD-6` puts the **offline refresh token** there, and a Keycloak-brokered offline JWT routinely
   exceeds that. §1.4 has the evidence and three fixes. **This is a Phase H task that does not
   exist yet**, and it is worth more than any library in this document.

---

## Candidate table

Legend: **adopt** = take as a dependency · **vendor** = copy a permissive piece in with
attribution · **study-only** = read the approach, write our own · **reject**.

| # | Candidate | Licence (read at source, 2026-09-28) | Replaces | Verdict |
|---|---|---|---|---|
| 1 | `coreos/go-oidc/v3` | Apache-2.0 | H identity: discovery, JWKS, ID-token verification | **adopt** |
| 2 | `golang.org/x/oauth2` | BSD-3-Clause | H identity: auth-code + PKCE + refresh | **adopt** |
| 3 | `zalando/go-keyring` | MIT | H identity: Windows Credential Manager store | **adopt** |
| 4 | `danieljoos/wincred` | MIT | the Windows half of #3, if we want it direct | **adopt (transitively)** |
| 5 | in-toto **Attestation Framework** (format) | Apache-2.0 | the provenance ledger's *record format* | **adopt as format** |
| 6 | `in-toto/in-toto-golang` | Apache-2.0 | DSSE envelope + attestation structs in Go | **vendor (the DSSE + statement types)** |
| 7 | SLSA Provenance v1 predicate | Apache-2.0 (spec repo) | the release-link predicate in Deploy | **adopt as format** |
| 8 | `transparency-dev/merkle` | Apache-2.0 | tamper-evident audit: inclusion + consistency proofs | **adopt** |
| 9 | `pressly/goose` | MIT | Postgres migrations for the Hub | **adopt** |
| 10 | `golang-migrate/migrate` | MIT | same as #9, alternative | adopt (either) |
| 11 | `jackc/pgx/v5` | MIT | Postgres driver + pool | **adopt** |
| 12 | `cedar-policy/cedar-go` | Apache-2.0 | tenancy/authorization **as a library** | **study-only now, adopt later** |
| 13 | `openfga/openfga` | Apache-2.0 | relationship authorization **as a server** | **study-only** |
| 14 | `authzed/spicedb` | Apache-2.0 | same as #13 | **study-only** |
| 15 | `casbin/casbin` | Apache-2.0 | RBAC/ABAC policy in-process | **reject** (model file is the config wall) |
| 16 | `cerbos/cerbos` | Apache-2.0 | policy decision point as a sidecar | **reject** for H |
| 17 | `codenotary/immudb` | **BUSL 1.1** (was Apache-2.0) | immutable ledger storage | **REJECT — licence** |
| 18 | `google/trillian` | Apache-2.0 | transparency log server | **study-only** (too much operationally) |
| 19 | `transparency-dev/tessera` | Apache-2.0 | tile-based transparency log library | **study-only** (no Postgres driver) |
| 20 | `sigstore/rekor` | Apache-2.0 | public transparency log service | **reject** for H |
| 21 | `zitadel/oidc` | Apache-2.0 | a full OIDC RP **and OP** in Go | **study-only** (we have Keycloak) |
| 22 | `Nerzal/gocloak` | Apache-2.0 | Keycloak **admin** API client | **adopt if** we automate realm config |
| 23 | `CycloneDX/cyclonedx-go` | Apache-2.0 | SBOM attached to a release | **adopt** (phase 7/9 join) |
| 24 | `spdx/tools-golang` | **`Apache-2.0 OR GPL-2.0-or-later`** (dual) | SBOM, alternative to #23 | **adopt under Apache-2.0** — §4.5 |
| 25 | `ory/keto` | Apache-2.0 | Zanzibar-style server | **reject** (superset of #13's cost) |
| 26 | `openbao/openbao` | MPL-2.0 | server-side secrets for the Hub | **CONDITIONAL (MPL)** — §4.4 |
| 27 | `sqlc-dev/sqlc` | MIT | hand-written `rows.Scan` for the ledger + revision log | **adopt, optional** |
| 28 | `riverqueue/river` | **MPL-2.0** | Postgres job queue for the deploy poller | **CONDITIONAL (MPL)**, not v1 |
| 29 | `99designs/keyring` | MIT | alternative to #3 | **reject** (more config than we need) |
| 30 | `secure-systems-lab/dsse` (format) | Apache-2.0 | signing envelope for ledger records | **adopt as format**, phase 9 |
| 31 | `valkey-io/valkey` | BSD-3-Clause | the Redis slot, **if** we ever need one | **adopt if needed** (Redis ≥8 is forbidden) |

Detail, evidence and the argument for each verdict follow.

---

## 1. Identity — the OD-30 sign-in flow

**What Phase H has to build:** the desktop opens the system browser at a Keycloak realm, auth-code
+ PKCE, a one-shot `127.0.0.1:0` loopback listener catches the redirect, the code is exchanged for
tokens, the ID token is verified against the realm's JWKS, and the **offline refresh token** goes
into Windows Credential Manager so later launches are silent
(`2026-09-25-identity-keycloak-broker.md` §2, §7; `OD-6`, `OD-30`).

### 1.1 `golang.org/x/oauth2` — PKCE is already in the tree

| | |
|---|---|
| Repo | `https://github.com/golang/oauth2` · module `golang.org/x/oauth2` |
| Licence | **BSD-3-Clause**, read at `https://pkg.go.dev/golang.org/x/oauth2?tab=licenses`, 2026-09-28 |
| Maturity | Go project official. Latest seen **v0.37.0, 2026-08-25**. |
| Already present | **Yes — `control-plane/go.mod` pins `golang.org/x/oauth2 v0.21.0` as an *indirect* dependency** (pulled in by `k8s.io/client-go`). |

**The finding that matters:** PKCE is built in. `oauth2.GenerateVerifier()`,
`oauth2.S256ChallengeOption(verifier)` and `oauth2.VerifierOption(verifier)` were **added in
v0.13.0**, and the tree already has v0.21.0. So the PKCE half of OD-30 costs **one line in
`require` moving from indirect to direct** — no new supply chain, no new licence question, nothing
for a licence gate to argue about.

Do **not** hand-roll the verifier/challenge. A hand-rolled S256 challenge is the classic place to
get base64url padding wrong and produce an error that only shows up against a real IdP.

**Verdict: adopt.** Replaces: Phase H identity tasks covering the authorization request, the code
exchange, and refresh-token redemption.

### 1.2 `coreos/go-oidc/v3` — discovery and ID-token verification

| | |
|---|---|
| Repo | `https://github.com/coreos/go-oidc` |
| Licence | **Apache-2.0**, read at `https://raw.githubusercontent.com/coreos/go-oidc/master/LICENSE`, 2026-09-28 |
| Maturity | **2.5k stars**; the de-facto Go OIDC relying-party library (Dex, kubelogin and much of the Kubernetes ecosystem use it). **Flag: the releases page read on 2026-09-28 lists the latest tag as v3.21.0, dated 2023-09-01 — no release in ~3 years.** See §6, item 1, before pinning. |
| What it does | `oidc.NewProvider(ctx, issuerURL)` does the `/.well-known/openid-configuration` fetch; `provider.Verifier(&oidc.Config{ClientID: …})` returns a verifier that fetches and **caches/rotates the JWKS** and checks signature, `iss`, `aud`, `exp`, `nbf`; `idToken.Claims(&v)` unmarshals. |

**Why this specifically.** The identity ruling's validation table (§3–§5 of
`2026-09-25-identity-keycloak-broker.md`) is a list of checks — issuer, audience, signature,
expiry, key rotation. Every one of those is a place to write a subtle security bug. `go-oidc` is
the library those checks have already been debugged in, against Keycloak among others, for a
decade.

**Integration cost: low, and it does not force an architecture.** It is a client library; it holds
no state beyond a key cache and an `http.Client` you supply. That last point matters on this
machine: the KPMG TLS-intercepting proxy means we must pass our own `http.Client` with the
corporate root, and `go-oidc` supports exactly that through `oidc.ClientContext(ctx, client)`.

**Risks.** (a) Release cadence is slow — it is a small, stable library, and "no release in 12
months" would not be alarming here, but **I could not read a release date from the repo page and
have not verified the latest tag**; check before pinning. (b) It deliberately does **not** do the
OAuth2 dance — that is `x/oauth2`'s job — so the two are used together, which is the normal
pattern, not a gap.

**Verdict: adopt.** Replaces: Phase H identity tasks for discovery, JWKS handling, and ID-token
validation — the whole of the ruling's validation table except the `kc_idp_hint`/`login_hint`
parameters, which are ours.

### 1.3 `zalando/go-keyring` and `danieljoos/wincred` — the token store

| | |
|---|---|
| `zalando/go-keyring` | **MIT**, declared on `https://github.com/zalando/go-keyring`, 2026-09-28. **1.3k stars.** 28 open issues, professional tone, active into September 2026. |
| `danieljoos/wincred` | **MIT**, `https://raw.githubusercontent.com/danieljoos/wincred/master/LICENSE`, "Copyright (c) 2014 Daniel Joos", 2026-09-28. This is the Windows backend go-keyring calls. |
| `99designs/keyring` | **MIT**, `https://raw.githubusercontent.com/99designs/keyring/master/LICENSE`, "Copyright (c) 2015 99designs", 2026-09-28. The heavier alternative (adds file/pass/vault backends). |

**Caution on the `zalando/go-keyring` LICENSE URL:** `.../main/LICENSE` returned **HTTP 404**. The
MIT declaration above is GitHub's own repo-page licence detection, which reads the file whatever
it is named. **Before adopting, open the file itself and record its path** — this is exactly the
badge-not-source gap the licence rule warns about, and I am flagging it rather than papering over
it.

**Recommendation: `zalando/go-keyring`, not `99designs/keyring`.** We need one platform (Windows)
and one operation pair (set/get/delete). `99designs/keyring` brings backend selection, prompt
functions and a config struct — the "wall of configuration" `CLAUDE.md` rules against, for
capability we do not use.

### 1.4 **The 2560-byte wall** — a design constraint, not a library choice

Verified at Microsoft Learn, `wincred.h` `CREDENTIALA`, read 2026-09-28
(`https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentiala`):

> `CredentialBlobSize` … "This member cannot be larger than **CRED_MAX_CREDENTIAL_BLOB_SIZE
> (5*512) bytes**."

**5 × 512 = 2560 bytes.** OD-6 and the identity ruling both say the **offline refresh token** goes
into Windows Credential Manager. A Keycloak refresh token is a JWT; with Entra-brokered identity,
group claims and an offline session id it is routinely **1–3 KB**, and an Entra-issued token
carried through the broker can be larger still. **This can silently exceed the limit**, and the
failure mode is a write error at sign-in on some users' accounts and not others — the worst kind
of bug to find in the owner's hands.

**This is not a reason to reject the library. It is a Phase H design task nobody has written
yet.** Three options, ranked:
1. **Keep the token out of the blob.** Store a short opaque handle in Credential Manager, and the
   token itself in a DPAPI-protected file under the app's local state. `golang.org/x/sys/windows`
   — **already a direct dependency at v0.24.0** — exposes `CryptProtectData`/`CryptUnprotectData`,
   which is the same protection Credential Manager itself uses. No new dependency.
2. **Chunk across multiple credential entries.** Works, and is what several CLIs do, but now
   every read and delete has to be transactional across N entries. More code than option 1.
3. **Ask Keycloak for a smaller token.** Trim group/role claims from the offline token in the
   realm's client scope. Real, but it is a change to *Panacea's* realm (`OD-30` notes the realm is
   Panacea's until phase 8), so it is an ask, not a decision we can take.

**Verdict on the store: adopt `zalando/go-keyring`, and add a Phase H task for the size limit.**
Also note the go-keyring issue tracker carries **#149 "Support Windows Credential Manager when
running in WSL"** and **#125 "Hanging on WSL/Windows"** — irrelevant to a Tauri desktop app on
Windows, relevant if a developer ever runs the control plane inside WSL, which on this programme
is plausible.

### 1.5 `zitadel/oidc` — the one we should *not* take

**Apache-2.0** (`https://raw.githubusercontent.com/zitadel/oidc/main/LICENSE`, 2026-09-28). It is a
complete OIDC **relying party and OpenID Provider** in Go, with a good RP package including PKCE
and a loopback-callback helper.

**Verdict: study-only.** Its RP half genuinely overlaps §1.1 + §1.2, and its `rp` package's
callback-handler pattern is worth reading before we write ours. But it is a larger dependency that
exists mostly to *be* an IdP, and **we already decided to be a Keycloak client, not an IdP**
(`OD-30`). Taking it means taking the OP surface too, and a licence gate then has to clear a much
bigger transitive tree for no added capability.

### 1.6 `Nerzal/gocloak` — only if we automate the realm

**Apache-2.0** (`https://raw.githubusercontent.com/Nerzal/gocloak/main/LICENSE`, "Copyright [2021]
[Tobias Theel]", 2026-09-28). A typed Go client for **Keycloak's Admin REST API** — realms,
clients, identity providers, client scopes, users, groups.

**Verdict: adopt *if*.** It replaces nothing in the sign-in path. It replaces work in **phase 8**
and in the Hub's dev setup: `deploy/portal/setup-keycloak.sh`-style shell scripting becomes typed,
testable Go. If Phase H's local-first story is "`docker compose up` gives you a working realm", this
is the library that makes that idempotent instead of a shell script. If the realm stays hand-configured
by Panacea's team, skip it.

**Risk to state plainly:** it tracks Keycloak's admin API, which moves between Keycloak majors.
Pin it, and expect a bump per Keycloak upgrade.

---

## 2. The provenance ledger — adopt a FORMAT, and one small library

This is the part of Phase H where open source is worth the most, and where **the valuable thing is
not code**.

### 2.1 The argument

`provenance-contract-draft.md` §1.2 already invented, independently, the exact shape the
supply-chain world standardised on:

| Our contract (§1.2, §2.2) | in-toto Attestation v1 |
|---|---|
| brief key `brief:b-42@3` + `digest: sha256:…` | `subject: [{ name, digest: {sha256: …} }]` |
| link *kind* — design link, merge link, release link | `predicateType: <URI>` |
| the link's payload (canvasId, pipelineRun, commit…) | `predicate: { … }` |
| "one record per event, never edited, never deleted" | an attestation is an immutable signed statement |

**We are one rename away from a published standard.** The in-toto Statement v1, read at
`https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md` on 2026-09-28:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [ { "name": "<NAME>", "digest": { "<ALG>": "<HEX>" } } ],
  "predicateType": "<URI>",
  "predicate": { }
}
```

A release link becomes:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [ { "name": "panacea-app/my-project", "digest": { "sha256": "<release commit>" } } ],
  "predicateType": "https://panacea.kpmg.au/attestations/ReleaseLink/v1",
  "predicate": {
    "briefKey": "brief:b-42@3",
    "briefDigest": "sha256:…",
    "designKey": "design:c-9@4",
    "pipelineRun": "…",
    "deploymentId": "…"
  }
}
```

**Why this is worth more than any library we could adopt.**

1. **It is the spine, made interoperable.** `CLAUDE.md` says the thread is the product. A thread
   expressed in a format that a regulated customer's existing supply-chain tooling already parses
   is a *sellable* thread. A proprietary JSON blob is a thing we have to explain in every
   assurance conversation.
2. **It answers the ISM/evidence question in Phase 9 for free.** Phase 9's scope is "audit trail …
   ISM evidence". An in-toto attestation chain, optionally DSSE-signed, is a recognised evidence
   artifact. Inventing our own means arguing our own is good enough.
3. **It costs almost nothing now.** We are designing the record shape this quarter. Naming three
   fields `_type`, `subject`, `predicateType` instead of three names of our own is a zero-cost
   decision **today** and an expensive migration in a year.
4. **It does not constrain storage.** The attestation is the record *format*. Postgres, the
   append-only table and the ledger's read API are still entirely ours.

**Licences, verified 2026-09-28:**
- in-toto Attestation Framework: **Apache-2.0**,
  `https://raw.githubusercontent.com/in-toto/attestation/main/LICENSE` ("Copyright 2021 in-toto
  Developers"). 376 stars, CNCF, Go bindings described by maintainers as the most mature of the
  four languages.
- Go package `github.com/in-toto/attestation/go/v1`: **Apache-2.0**, **v1.2.0, published
  2026-03-18** (read at `https://pkg.go.dev/github.com/in-toto/attestation/go/v1`). Provides
  `Statement`, `ResourceDescriptor` and a 17-member `HashAlgorithm` enum, generated from the
  protobuf spec.
- DSSE (the signing envelope): **Apache-2.0**,
  `https://raw.githubusercontent.com/secure-systems-lab/dsse/master/LICENSE`.

**Verdict: adopt as format (the spec), adopt as dependency (the Go types package).** The Go
package is generated protobuf structs — small, stable, and it drags in `google.golang.org/protobuf`,
**already an indirect dependency at v1.34.2**. If even that is unwanted, the Statement is four
fields; **vendor** a 40-line Go struct with an Apache-2.0 attribution header instead. Either is
defensible; the *format* decision is the one that must not be deferred.

**Replaces:** not tasks, but a design decision inside the Phase H ledger tasks — the record schema,
the digest discipline, and the "what does a link look like on the wire" question that phases 3, 5
and 7 all have to agree on.

### 2.2 SLSA Provenance v1 — the predicate for the release link

`https://github.com/slsa-framework/slsa`, LICENSE.md read 2026-09-28. **Dual-licensed:
Community Specification License 1.0 for spec content contributed under that governance, and
Apache-2.0 for pre-existing portions.** The licence text itself is CC-BY-4.0; source code in the
repo defaults to MIT.

**The obligation, quoted from what I read:** *"Attribution is not required for implementations of
the Specification."* Implementers get a royalty-free patent grant. **So emitting SLSA Provenance
from PanaceaApp carries no attribution or copyleft obligation.** Copying large blocks of the spec
*text* into our docs would need attribution; implementing it does not.

**Use:** where the Deploy phase attaches a release to a brief, emit a SLSA Provenance v1 predicate
alongside our own `ReleaseLink` predicate, so the release says both *"this came from this brief"*
(ours) and *"this was built by this builder from this source"* (theirs).

**Verdict: adopt as format, phase 7/9 join.** Caveat recorded honestly: the repo's `spec/` on
`main` is a **draft**; released versions live on `releases/v1.0` and `releases/v1.2` branches. Pin
a release branch, not `main`.

### 2.3 `in-toto/in-toto-golang` — vendor the envelope, not the framework

**Apache-2.0**, `https://raw.githubusercontent.com/in-toto/in-toto-golang/master/LICENSE`,
"Copyright 2018 New York University", read 2026-09-28. **151 stars, 1,088 commits, not archived.**

It provides link metadata, **layout policy definition and enforcement**, X.509 constraint checking,
SPIFFE/SPIRE integration. It states GPG key support is unimplemented.

**Verdict: vendor, narrowly — or skip.** The *layout* machinery is a supply-chain policy engine we
do not want and would never configure. What is useful is the **DSSE envelope** handling: canonical
`PAE` encoding, signature over the payload, verification. That is a few hundred lines, Apache-2.0,
and copying it with attribution is cleaner than depending on a framework whose main feature we
reject. **If Phase H does not sign ledger records in v1** — and nothing in the contract draft says
it must — **skip this entirely and revisit at Phase 9.**

### 2.4 Tamper-evidence: `transparency-dev/merkle`, and what to reject

The contract says the ledger is append-only and immutable. **Append-only in Postgres is a
permission and a trigger; *tamper-evident* is a Merkle tree.** They are different properties and
Phase H should decide which one it is buying.

| Project | Licence (read 2026-09-28) | Verdict |
|---|---|---|
| `transparency-dev/merkle` | **Apache-2.0** (`.../transparency-dev/merkle/main/LICENSE`) | **adopt** |
| `google/trillian` | **Apache-2.0** (`.../google/trillian/master/LICENSE`) | study-only |
| `transparency-dev/tessera` | **Apache-2.0** (repo page) | study-only |
| `sigstore/rekor` | **Apache-2.0** (`.../sigstore/rekor/main/LICENSE`) | reject for H |
| `codenotary/immudb` | **BUSL 1.1** — see §5 | **REJECT, licence** |

**`transparency-dev/merkle` is the right size.** It is the RFC 6962 Merkle logic split out of
Trillian: build a log tree, produce **inclusion proofs** and **consistency proofs**, verify them.
It is a pure computation library — **we keep the log in our own Postgres table** and hand it
hashes. That is precisely the seam Phase H needs: `ledger_entries` stays ours, and a nightly (or
per-append) tree head gives a signed root that proves nobody edited row 4,102 after the fact.

**Honest maturity note: 41 stars.** That number is meaningless here and I am saying why rather
than hiding it — this is a library extracted from Google's Certificate Transparency stack and it
underpins the public CT logs the entire web PKI depends on. Low stars, extreme pedigree. But it
*is* small and it *is* low-traffic, so pin a version and read the code you use.

**Trillian and Tessera are rejected as *services*, not as ideas.** Trillian is a multi-process
system with its own database and signers; Tessera (v0.2.0 beta, 242 stars) is a library but its
storage drivers are **GCP, AWS S3 and POSIX — there is no Postgres driver**, and the AWS path uses
MySQL. Running either inside a customer's Azure tenancy to make an audit log tamper-evident is a
wall of operational configuration for a property we can get from one library plus one table.

**Rekor is rejected** because it is a *public* transparency log service; our ledger is
tenant-private. The interesting half of Rekor is the format, which is in-toto/DSSE — §2.1 already
took that.

### 2.5 What open source does NOT give us here

Say this plainly so nobody plans against it:

- **No library gives us the brief-revision store.** The revision log
  (`provenance-contract-draft.md` §2.1) — write revision, then head, atomically, so a crash never
  orphans a head — is ours to write. It is a couple of hundred lines and a test.
- **No library gives us the resolver or the criterion evaluator** (§2.3). Those are product logic.
- **No library gives us the `contextRef` restamp rule** (§2.4).
- **Postgres gives us append-only for free** and better than any dependency: a table with no
  `UPDATE`/`DELETE` grant to the app role, plus a `BEFORE UPDATE OR DELETE` trigger that raises.
  Do that before reaching for anything else.

---

## 3. Tenancy and authorization — the place to say no, for now

Phase H needs: a person belongs to an org; an org has projects; a project has a data
classification (`OD-24`); a teammate can see a project's briefs and the ledger; nobody sees another
tenant's anything. That is a handful of tables and a `WHERE org_id = $1` discipline, plus Postgres
row-level security if we want belt and braces.

The temptation is to reach for a Zanzibar engine because it is the "proper" answer. **Resist it for
Phase H, and revisit it for Phase T (team mode).**

| Project | Licence (read at source, 2026-09-28) | Stars | Shape |
|---|---|---|---|
| `openfga/openfga` | **Apache-2.0** (`.../openfga/openfga/main/LICENSE`) | 5.9k | server **or** embeddable Go library; Postgres 14+, MySQL 8, SQLite (beta) |
| `authzed/spicedb` | **Apache-2.0** (`.../authzed/spicedb/main/LICENSE`, "Copyright 2025 Authzed, Inc.") | 7.1k | **service only, no embedded mode**; Postgres, MySQL, CockroachDB, Spanner |
| `cedar-policy/cedar-go` | **Apache-2.0** (`.../cedar-policy/cedar-go/main/LICENSE`) | 235 | pure-Go **library**, no server, no datastore |
| `casbin/casbin` | **Apache-2.0** (`.../casbin/casbin/master/LICENSE`) | 20.4k | in-process library + `.conf` PERM model file |
| `cerbos/cerbos` | **Apache-2.0** (`.../cerbos/cerbos/main/LICENSE`; no field-of-use clause) | — | sidecar/PDP, YAML policies |
| `ory/keto` | **Apache-2.0** (`.../ory/keto/master/LICENSE`) | — | Zanzibar service |
| `open-policy-agent/opa` | **Apache-2.0** (`.../open-policy-agent/opa/main/LICENSE`) | — | Rego engine, library or sidecar |

**All seven are licence-clean.** This is a fit argument, not a licence argument.

### 3.1 Why not now

1. **`OD-19`/`OD-24`'s real authorization questions are data-classification questions**, not
   relationship-graph questions: does this project's classification force AU inference; which scans
   does this classification require. Those are enum comparisons, not `Check(user, relation, object)`.
2. **`OD-16` says Orchestrators are the only cross-user channel.** The interesting permission
   surface in this product is an *agent negotiation protocol*, not an ACL. No engine models that.
3. **Every one of these adds a schema we must migrate, a model/policy DSL we must version, and a
   second source of truth.** `OD-31` explicitly cut the spend; 44 Hub tasks do not have room for a
   second database.
4. **`CLAUDE.md`'s bar:** *"If a feature is powerful but takes a wall of configuration to reach, it
   has failed our bar."* Casbin is the clearest example — a `.conf` PERM metamodel plus a policy
   CSV plus an adapter, to express "users in this org can read this org's briefs". 20.4k stars do
   not make that the right shape for us. **Reject Casbin.**

### 3.2 Which one, when the time comes

**`openfga/openfga`, and the reason is the embedded mode.** Verified at
`https://pkg.go.dev/github.com/openfga/openfga/pkg/server` on 2026-09-28: `NewServerWithOpts(...)`
returns a `*Server` implementing the gRPC and HTTP APIs **in-process**, with a datastore supplied
via `WithDatastore()` — our Postgres. No second deployment unit, no second thing for a customer's
platform team to run in their own cloud. That is the difference that matters for a product shipped
into somebody else's tenancy.

**SpiceDB is the better-known engine and is out on architecture, not quality:** it is a service
only, and `OD-5`/phase 8 already commit us to standing up AKS, Kata, Keycloak, Postgres and a
LiteLLM gateway in a customer's cloud. Adding a sixth stateful component to that list is a
deployment cost we pay on every customer forever.

**`cedar-policy/cedar-go` is the dark horse and the honest maturity warning goes with it.** It is a
pure library with no datastore — policies are strings you evaluate — which fits "a bit of policy
inside our own service" far better than a graph engine. **But: 235 stars, and the most recent
release I could read is v1.8.0 dated 2025-06-01 — more than twelve months ago.** The Rust
implementation is the reference and the Go port trails it (v1.8.0's notes flag a behaviour change
made to *align with the Rust implementation*, which tells you the port can diverge). Do not bet
Phase H on it.

**Verdicts:** OpenFGA — **study-only for Phase H, revisit at Phase T**. SpiceDB — **study-only**.
Cedar-go — **study-only**. Casbin, Cerbos, Keto, OPA — **reject for Phase H**.

**What to do instead, in Phase H:** org/project/membership tables, one `authz` package with
explicit functions (`CanReadBrief(ctx, user, briefID)`), and **Postgres row-level security** as the
backstop so a missed `WHERE` clause cannot leak across tenants. That is a known, auditable,
zero-dependency answer that a KPMG assurance reviewer can read in an afternoon.

---

## 4. The data layer — three small, boring, correct choices

### 4.1 `jackc/pgx/v5` — the Postgres driver

**MIT**, `https://raw.githubusercontent.com/jackc/pgx/master/LICENSE`, "Copyright (c) 2013-2021
Jack Christensen", read 2026-09-28. **Latest release v5.11.0, 2026-09-07.** Actively released.

The default choice for Go and Postgres: native protocol, connection pooling (`pgxpool`), proper
support for `jsonb`, arrays, `numeric` and `LISTEN`/`NOTIFY`. `LISTEN`/`NOTIFY` matters for Phase H
specifically — team mode (`OD-15`) needs "a teammate sees the same thread" without every desktop
polling the Hub, and pgx gives us change notification without adding a message broker.

**Verdict: adopt.** Replaces: nothing task-shaped — it is the floor under every Hub store task.

### 4.2 Migrations: `pressly/goose` (or `golang-migrate/migrate`)

| | `pressly/goose` | `golang-migrate/migrate` |
|---|---|---|
| Licence | **MIT** (`.../pressly/goose/main/LICENSE`) | **MIT** (`.../golang-migrate/migrate/master/LICENSE`) |
| Stars | 11.5k | 18.9k |
| `embed.FS` | yes | yes (`io/fs` source driver) |
| Library API | yes, plus CLI | yes, "stable and frozen for v3 & v4" |
| Go migrations | **yes**, plain Go functions | SQL-focused |
| Open issues | — | **313 open, 183 PRs** |

**Recommend `goose`**, on two grounds. First, **Go migrations as plain functions**: the ledger's
append-only trigger and the RLS policies are the kind of thing that eventually needs a data
backfill, and doing that in Go with the same types the app uses beats writing it twice. Second,
`golang-migrate`'s 313 open issues / 183 open PRs is a backlog, and while the API being "frozen"
makes that less dangerous than it looks, goose's tracker is the calmer one.

Either is a fine answer. **Do not build our own.** Migration ordering, dirty-state recovery and
advisory locking against concurrent app instances are solved and easy to get subtly wrong.

**Verdict: adopt goose.** Replaces: the Hub's schema-management tasks outright.

### 4.3 `sqlc-dev/sqlc` — optional, and it earns its place here

**MIT**, `https://raw.githubusercontent.com/sqlc-dev/sqlc/main/LICENSE`, "Copyright (c) 2024 Riza,
Inc.", read 2026-09-28. Generates type-safe Go from SQL at build time. No runtime dependency at all
— it is a code generator, so it does not appear in the shipped binary's dependency tree, which
makes the licence question about as small as it gets.

**Why it fits Phase H's shape:** the ledger and the revision log are **append-only with a fixed
query set**. That is exactly where generated code beats an ORM and beats hand-written
`rows.Scan`. It also removes a whole class of bug — a column added to `ledger_entries` that a hand
written scan forgets.

**Verdict: adopt, optional.** The one honest cost is a codegen step in CI that a developer can
forget to run; a `make generate && git diff --exit-code` check in CI closes it.

### 4.4 `riverqueue/river` — **CONDITIONAL, MPL-2.0**

**MPL-2.0**, `https://raw.githubusercontent.com/riverqueue/river/master/LICENSE`, read 2026-09-28.
A Postgres-backed background job queue for Go. Relevant because the release link's second record —
the `deploymentId` join written by the deploy poller (`provenance-contract-draft.md` §2.2) — is a
background job, and Phase H will grow more of them.

**The MPL-2.0 obligation, stated plainly:** MPL is **file-level** copyleft. Using it as an
unmodified dependency in proprietary software is fine and imposes nothing on our code. **If we
modify a River source file, that file's source must be made available.** The practical rule: use
it, never patch it in-tree; if we need a change, upstream it or wrap it.

**Verdict: CONDITIONAL — allowed by the licence rule, flagged.** Phase H probably does not need it
in v1 (a `SELECT … FOR UPDATE SKIP LOCKED` loop over a `jobs` table is ~150 lines and we already
have Postgres). Revisit when the job count justifies it. The same MPL-2.0 note applies to
**`openbao/openbao`** (verified MPL-2.0 at `.../openbao/openbao/main/LICENSE`) if the Hub ever
needs server-side secret storage — and note that **HashiCorp Vault itself is BUSL** since 1.15, so
OpenBao is the fork to reach for, not Vault.

### 4.5 SBOM on a release — and a dual-licence trap worth naming

`OD-24` says the data classification drives **which security scans a deployment requires**. That
makes an SBOM part of the release record, which makes it part of the provenance thread — the same
`subject` an in-toto release link points at (§2.1).

| | Licence, read at source 2026-09-28 | Note |
|---|---|---|
| `CycloneDX/cyclonedx-go` | **Apache-2.0**, `.../CycloneDX/cyclonedx-go/master/LICENSE`, "Copyright OWASP Foundation" | clean, single licence |
| `spdx/tools-golang` | **`Apache-2.0 OR GPL-2.0-or-later`** in `LICENSE.code`; docs are **CC-BY-4.0** in `LICENSE.docs`. 171 stars. | **dual, our choice** |

**The trap, stated so the licence gate does not get it wrong.** `spdx/tools-golang` is dual-licensed
with **OR**, not AND. We may elect **Apache-2.0** and the GPL option never applies to us. That is
legal and normal — but a naive licence gate that pattern-matches "GPL" in an SPDX expression will
fail the build, and a naive reviewer might read the repo page and wrongly reject the library. **The
gate must parse SPDX expressions, not substrings**, and our election of Apache-2.0 must be recorded
in the `NOTICE` file (§7.3).

**Recommendation: `cyclonedx-go`.** Not because SPDX is worse, but because one unambiguous licence
is one fewer thing to explain in a KPMG assurance conversation, and CycloneDX is the format Azure
DevOps and most scanners emit natively.

---

## 5. The excluded list — copyleft and source-available, by name only

Recorded so that nobody proposes these again, and so the record shows we looked. **No source from
any project in this table was opened.** Every licence was read at the URL given, on 2026-09-28.

| Project | Licence, read at source | Relevance to Phase H | Why excluded |
|---|---|---|---|
| **`codenotary/immudb`** | **BUSL 1.1** — `https://raw.githubusercontent.com/codenotary/immudb/master/LICENSE`. "Licensed Work: immudb Version 1.9DOM.2 or later … (c) 2024 Codenotary, Inc." Change Date: four years per version. Change Licence: Apache-2.0. | The obvious "immutable append-only database" for the provenance ledger | **BUSL is forbidden.** Note the **relicence boundary**: older immudb was Apache-2.0, and search results still describe it that way. Versions from **1.9DOM.2** onward are BUSL. Anything we could legally use is years stale. |
| **`hashicorp/vault`** | **BUSL 1.1** — `https://raw.githubusercontent.com/hashicorp/vault/main/LICENSE`. Licensor: IBM. Licensed Work: **Vault 1.15.0 or later**. Change Licence: MPL-2.0. | Server-side secrets for the Hub | **BUSL is forbidden.** Version boundary: **≤1.14 was MPL-2.0; ≥1.15 is BUSL.** Use **`openbao/openbao` (MPL-2.0)** instead — verified, and listed as CONDITIONAL in §4.4. |
| **`redis/redis`** | **Tri-licensed RSALv2 / SSPLv1 / AGPLv3** from **Redis 8.0** — `https://raw.githubusercontent.com/redis/redis/unstable/LICENSE.txt`. **7.2 and earlier remain BSD-3-Clause.** | Session cache, rate limiting, `LISTEN`-free pub/sub for the Hub | **All three options are forbidden.** Use **`valkey-io/valkey` (BSD-3-Clause, Linux Foundation)** — verified at `https://github.com/valkey-io/valkey`, licence in `COPYING`. Or, better for Phase H, use Postgres and add nothing. |
| **`minio/minio`** | **AGPL-3.0-or-later** — `https://raw.githubusercontent.com/minio/minio/master/LICENSE` | Object storage for brief attachments and release artifacts | **AGPL is forbidden, no exceptions** (`OD-11`, and the §13 reading in `PROGRAM.md`). Use Azure Blob Storage — we are on Azure (`OD-18`) and it costs us nothing to adopt. |
| **`grafana/grafana`** | **AGPL-3.0** — `https://raw.githubusercontent.com/grafana/grafana/main/LICENSE` | Audit / usage dashboards over the Hub's data | **AGPL is forbidden.** Also a relicence: Grafana was Apache-2.0 before v9. Unmodified, self-hosted, side-by-side use is a different legal question from embedding, but the programme's rule is "no AGPL", and embedding is what a cockpit would want. |
| **`goauthentik/authentik`** | **MIT core with an enterprise carve-out** — `https://raw.githubusercontent.com/goauthentik/authentik/main/LICENSE`: *"All content that resides under the `authentik/enterprise/` directory … is licensed under the license defined in `authentik/enterprise/LICENSE`."* | Alternative IdP to Keycloak | **Not excluded on licence — excluded on `OD-30`** (we broker through Keycloak). Listed here because it is the textbook **permissive-core / restricted-add-on trap** the licence rule warns about: the badge says MIT and a directory is not MIT. |
| **Elastic / Elasticsearch, Kibana** | Elastic License 2.0 / SSPL (AGPL added for 8.16+) | Audit log search | **Elastic License and SSPL are both forbidden.** `opensearch-project/OpenSearch` (Apache-2.0) is the substitute if search is ever needed. |
| **`Projects/Paperdesign`, `kgoedecke/doop`** | AGPL-3.0-only (already on file, `PROGRAM.md`) | not Phase H | **Not opened, not read, not searched.** Recorded only because the clean-room rule binds every agent on this programme, including this one. |

**Clean-licence projects I checked and did not need:** `keycloak/keycloak` — **Apache-2.0**
(`https://raw.githubusercontent.com/keycloak/keycloak/main/LICENSE.txt`), confirming `OD-30`'s
broker carries no licence problem. `goharbor/harbor` — **Apache-2.0**, if a registry is ever needed.

---

## 6. What I could not verify, and what the proxy blocked

**Blocked or failed fetches (all on 2026-09-28):**

| URL | Result | What I did instead |
|---|---|---|
| `https://raw.githubusercontent.com/zalando/go-keyring/main/LICENSE` | **HTTP 404** | Refetched `https://github.com/zalando/go-keyring/blob/master/LICENSE` — **MIT, "Copyright (c) 2016 Zalando SE"**. The 404 was a wrong default branch (`master`, not `main`), not the proxy. |
| `https://raw.githubusercontent.com/slsa-framework/slsa/main/LICENSE` | **HTTP 404** | The file is `LICENSE.md`; refetched and read it. |
| `https://raw.githubusercontent.com/valkey-io/valkey/unstable/LICENSE` | **HTTP 404** | The file is `COPYING`; read the licence from the repository page instead — **BSD-3-Clause**. I did **not** read the `COPYING` text itself. |
| `https://github.com/zalando/go-keyring/blob/main/LICENSE.md` | **HTTP 404** | Same wrong-branch cause as above. |

**No fetch was blocked by the Netskope proxy in this session.** `raw.githubusercontent.com`,
`github.com`, `pkg.go.dev` and `learn.microsoft.com` all resolved and returned content. Every 404
above was a wrong path on my part, and each is recorded with the correction.

**What I could NOT verify, stated plainly:**

1. **`coreos/go-oidc`'s release cadence is a genuine open question.** The releases page I read
   lists the latest tag as **v3.21.0 dated 2023-09-01** — **no release in roughly three years**. I
   am reporting what the page said; it is possible the page's dates were mis-summarised. The
   library is stable and embedded in Dex, kubelogin and much of the Kubernetes ecosystem, and OIDC
   relying-party logic does not change much. **But "no release in 12 months" is a real flag under
   this research's own rule, and Phase H must check it before pinning.** If it is genuinely
   dormant, **`zitadel/oidc` (Apache-2.0) is the actively-released alternative** and its `rp`
   package covers the same ground.
2. **`transparency-dev/merkle` is pre-1.0.** The latest tag I could read is **v0.0.2**, and the
   release page's dates were ambiguous. The code is used in production Certificate Transparency
   logs, but the module has never declared API stability. Pin an exact version and treat an upgrade
   as a code review, not a bump.
3. **`cedar-policy/cedar-go`'s latest release read as v1.8.0, 2025-06-01** — over twelve months old
   at the time of writing. Verify before any adoption.
4. **I did not read any dependency's dependency tree.** For the four libraries recommended for
   adoption, the transitive licence check — `go-licenses` or equivalent, in the CI licence gate
   `OD-11` already requires — has **not** been run. That gate is the right place for it, and it
   must run before any of these land.
5. **I did not verify OpenFGA's embedded mode by running it.** The claim comes from the package
   documentation at `pkg.go.dev`, read 2026-09-28. It is a documentation reading, not a
   demonstration.
6. **Nothing in this document was built, compiled or executed.** No repository was cloned.
7. **Task numbering:** I mapped recommendations to Phase H *areas* (identity, ledger, storage),
   not to numbered tasks, because Phase H's plan with its 44 numbered tasks is not in the tree —
   the phase is described in `PROGRAM.md` and `OD-3`/`OD-5` but has no plan file under
   `docs/superpowers/plans/`. Where I say "6–8 tasks", that is an estimate from the described
   scope, not a count of a written list.

---

## 7. What this actually saves, and what to do first

### 7.1 The honest saving

| Area | Hand-written | With these | Saving |
|---|---|---|---|
| Sign-in flow (PKCE, loopback, JWKS, refresh) | ~6–8 tasks | wiring + tests, ~3 | **~4 tasks** |
| Credential storage | ~2 tasks | ~1, **plus a new one for the 2560-byte limit** | **~0 — it is a wash, but we avoid a production bug** |
| Ledger record format | design debate + bespoke schema | adopt in-toto Statement v1 | **not tasks — interoperability, and a decision closed** |
| Tamper-evidence | ~3–4 tasks of Merkle code | `transparency-dev/merkle` + one table | **~3 tasks** |
| Migrations | ~2 tasks | `goose` | **~2 tasks** |
| Driver / query layer | ~2 tasks of scan boilerplate | `pgx` + optional `sqlc` | **~1–2 tasks** |
| Tenancy / authz | ~4 tasks | **nothing — write it ourselves** | **0** |

**Roughly 10–11 of 44 Phase H tasks, or about a quarter of the phase.** That is a real saving and
it is smaller than the word "open source" makes people hope. The reason is the one worth reporting
to the owner: **Phase H's hard parts are product logic — the brief-revision store, the resolver,
the criterion evaluator, the provenance thread itself — and nobody has open-sourced our product.**

### 7.2 The five decisions to take, ranked

1. **Adopt in-toto Attestation v1 as the ledger's record format.** Zero cost today, expensive
   later, and it is the one choice that makes the product spine interoperable rather than
   proprietary. **This is the recommendation to put in front of the owner.**
2. **Move `golang.org/x/oauth2` from indirect to direct and use its built-in PKCE.** No new
   dependency at all.
3. **Add a Phase H task for the 2560-byte Windows Credential Manager limit**, before the sign-in
   task is written. Preferred fix: opaque handle in Credential Manager, token in a DPAPI-protected
   file via `golang.org/x/sys/windows`, which is **already a direct dependency**.
4. **Verify `coreos/go-oidc`'s release cadence.** If it is dormant, switch to `zitadel/oidc`.
   Either way, do not hand-roll ID-token verification.
5. **Do not adopt an authorization engine in Phase H.** Org/project/membership tables plus Postgres
   row-level security. Revisit OpenFGA — embedded, not as a service — when Phase T's cross-user
   Orchestrator model (`OD-16`) is real.

### 7.3 Licence gate obligations these create

`OD-11` already requires a licence gate in CI. These recommendations give it four jobs:
- **Allow-list** MIT, Apache-2.0, BSD-2/3-Clause, ISC, Unlicense, 0BSD, Zlib.
- **Flag, do not fail,** MPL-2.0 — and fail the build if an MPL-2.0 file in `vendor/` differs from
  upstream, which is the only way MPL's obligation bites us.
- **Fail** on AGPL, GPL, LGPL, SSPL, BUSL/BSL, Elastic, Commons Clause, and any unrecognised
  licence string.
- **Run over the transitive tree, not the direct requires** — `codenotary/immudb` and
  `hashicorp/vault` both prove that a licence can change under a version bump.

For Apache-2.0 dependencies, the Hub's distribution needs a `NOTICE` file. That is a small task
nobody has written yet, and it is the only obligation any recommendation here places on us.
