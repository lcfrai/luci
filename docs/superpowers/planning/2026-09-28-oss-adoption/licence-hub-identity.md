# Licence audit — Phase H (the Hub): identity, tenancy, briefs, provenance ledger, audit, releases

**Adversarial verification of `05-hub-identity.md`. Audited 2026-09-28. Read-only: no code changed,
nothing built, nothing run.**

My job was to **disprove** the researcher's licence claims, not confirm them. Every licence below was
read as a **raw file at the repository**, over `curl --ssl-no-revoke`, on **2026-09-28**. Where the
researcher's URL was a badge, a rendering (`pkg.go.dev?tab=licenses`) or the wrong branch, I say so
and give the file I actually read. Maturity figures (stars, last push, latest release) come from the
GitHub REST API via authenticated `gh api` on the same date.

**Bottom line: no forbidden licence found in anything marked adopt or vendor.** The researcher's
licence calls survive. But **nine defects** in the licence record need fixing before any of this
reaches a CI licence gate — a wrong branch, two repositories that ship only the Apache-2.0 *short
notice*, three repositories GitHub itself classifies as `NOASSERTION`, an MPL-2.0 driver hiding in a
"MIT" tool's `go.mod`, an undisclosed commercial private module beside an MPL core, three NOTICE
files that create a mandatory attribution obligation, and one summary-table row that states the
wrong licence family outright.

---

## 1. Verdict table

`yes` = permissive, ship it · `conditional` = usable with a stated obligation · `no` = forbidden.
"Agrees" = my verdict matches the researcher's licence claim.

| # | Candidate (module path we would import) | Claimed | **Verified at source** | Agrees | Verdict | Obligation we take on |
|---|---|---|---|---|---|---|
| 1 | in-toto Attestation Framework — `github.com/in-toto/attestation` (`/go/v1`) | Apache-2.0 | **Apache-2.0** (short notice only) | yes | **conditional** | Ship full Apache-2.0 text ourselves — upstream `LICENSE` is only the 13-line notice; preserve "Copyright 2021 in-toto Developers" |
| 2 | `golang.org/x/oauth2` | BSD-3-Clause | **BSD-3-Clause** | yes | **yes** | Reproduce copyright + 3-clause text in NOTICES; no endorsement use of "Google"/"The Go Authors" |
| 3 | `github.com/coreos/go-oidc/v3` | Apache-2.0 | **Apache-2.0 + NOTICE** | yes (URL wrong) | **conditional** | Apache-2.0 §4(d): reproduce the repo's `NOTICE` ("CoreOS Project / Copyright 2014 CoreOS, Inc") in our attribution file |
| 4 | `github.com/zalando/go-keyring` | MIT | **MIT** (`master`, not `main`) | yes | **yes** | MIT copyright notice ("Copyright (c) 2016 Zalando SE") |
| 4b | `github.com/danieljoos/wincred` (underneath) | MIT | **MIT** | yes | **yes** | MIT notice ("Copyright (c) 2014 Daniel Joos") |
| 5 | `github.com/transparency-dev/merkle` | Apache-2.0 | **Apache-2.0**, full text, no NOTICE | yes | **yes** | Apache-2.0 §4(a)/(b) only. **Maturity caveat: last tagged release v0.0.2, 2023-05-05** |
| 6 | `github.com/pressly/goose/v3` | MIT | **MIT** — *but* `go.mod` pulls **MPL-2.0** `go-sql-driver/mysql` | **no — incomplete** | **conditional** | MIT notice + MPL-2.0 obligation inherited from a direct require; GitHub classifies goose `NOASSERTION` |
| 7 | `github.com/jackc/pgx/v5` | MIT | **MIT** | yes | **yes** | MIT notice ("Copyright (c) 2013-2021 Jack Christensen"); all 4 `jackc/*` deps MIT |
| 8 | SLSA Provenance v1 (format) | "Apache-2.0 (spec repo)" in the summary row | **Community Specification License 1.0** — the *only* licence in `LICENSE.md` | **no — summary row wrong** | **conditional** | CSL-1.0 is a *specification* licence with patent ("Necessary Claims") and trademark terms; fine to implement, **not** fine to paste spec prose unreviewed |
| 9 | `github.com/CycloneDX/cyclonedx-go` | Apache-2.0 | **Apache-2.0 + NOTICE** | yes | **conditional** | Apache-2.0 §4(d): reproduce "CycloneDX Go / Copyright (c) OWASP Foundation" |
| 10 | `github.com/in-toto/in-toto-golang` | Apache-2.0 | **Apache-2.0** (short notice only) | yes | **conditional** | Ship full Apache-2.0 text; preserve "Copyright 2018 New York University"; GitHub says `NOASSERTION` |
| 11 | `github.com/Nerzal/gocloak/v14` | Apache-2.0 | **Apache-2.0**, full text, no NOTICE | yes | **yes** | Apache-2.0 §4(a)/(b) |
| 12 | `github.com/sqlc-dev/sqlc` | MIT | **MIT**, "Copyright (c) 2024 Riza, Inc." | yes | **yes** | MIT notice. Build-time tool: generated code carries no upstream obligation |
| 13 | `github.com/openfga/openfga` | Apache-2.0 | **Apache-2.0**, no NOTICE | yes | **yes** (study-only on fit) | Apache-2.0 §4(a)/(b) |
| 14 | `github.com/cedar-policy/cedar-go` | Apache-2.0 | **Apache-2.0 + NOTICE** | yes | **conditional** | Apache-2.0 §4(d): reproduce "Copyright Cedar Contributors" |
| 15 | `github.com/casbin/casbin` | Apache-2.0 | **Apache-2.0** (appendix present) | yes | **yes** | Apache-2.0 §4(a)/(b) |
| 16 | `github.com/riverqueue/river` | MPL-2.0 | **MPL-2.0** — **relicensed 2023-11-22**; **River Pro is a paid private module** | **no — incomplete** | **conditional** | MPL-2.0 §3: publish source of any river *file* we modify; ship the MPL text. Never import River Pro |
| 17 | `google/trillian` + `transparency-dev/tessera` | Apache-2.0 both | **Apache-2.0 both**, full text | yes | **yes** (study-only on fit) | Apache-2.0 §4(a)/(b) |
| 18 | `github.com/zitadel/oidc/v3` | Apache-2.0 | **Apache-2.0**, full text | yes | **yes** (study-only on fit) | Apache-2.0 §4(a)/(b) |

Nothing here is AGPL, GPL, LGPL, SSPL, BUSL, Elastic, Commons Clause, source-available, or
field-of-use restricted. **The one GPL string anywhere in the set is an *option* we decline**
(`spdx/tools-golang`, §4.7).

---

## 2. Where I disagree with the researcher

Nine items. Two change a verdict to **conditional**; the rest correct the record so the CI licence
gate does not trip or, worse, wave something through.

### D1 — `coreos/go-oidc`: the researcher read the **wrong branch**, and missed a NOTICE file

The researcher's URL of record is `https://raw.githubusercontent.com/coreos/go-oidc/master/LICENSE`.
**The default branch of that repository is `v3`, not `master`** (`gh api repos/coreos/go-oidc --jq
.default_branch` → `v3`, 2026-09-28). `master` is the legacy v2 line; the module we would import is
`github.com/coreos/go-oidc/v3`, whose `go.mod` lives on the `v3` branch
(`https://raw.githubusercontent.com/coreos/go-oidc/v3/go.mod`, HTTP 200, `module
github.com/coreos/go-oidc/v3`). Reading a licence off a branch we do not consume is the badge-level
mistake the licence rule exists to stop, even when the answer happens to be the same.

I read both. **Both are Apache-2.0, full text** (`.../v3/LICENSE`, HTTP 200, 2026-09-28). No
disagreement on the licence — disagreement on the evidence.

**The substantive miss:** the repository ships a **`NOTICE` file** on both branches
(`https://raw.githubusercontent.com/coreos/go-oidc/v3/NOTICE`, HTTP 200, 2026-09-28):

> `CoreOS Project` / `Copyright 2014 CoreOS, Inc` / `This product includes software developed at
> CoreOS, Inc. (http://www.coreos.com/).`

Apache-2.0 **§4(d)** is not optional: if a work we distribute includes a NOTICE file, our
distribution "must include a readable copy of the attribution notices contained within such NOTICE
file". PanaceaApp ships to customers to run on their own cloud, so this is a real, recurring
obligation — a third-party notices file in the desktop cockpit and in the Hub image, not a to-do.
The researcher's §1.2 does not mention it. **Verdict: adopt, but as conditional, not clean.**

### D2 — `pressly/goose` is MIT, and its `go.mod` drags in an **MPL-2.0** driver and a lot else

The licence itself is fine. I read the whole file
(`https://raw.githubusercontent.com/pressly/goose/main/LICENSE`, HTTP 200, 1197 bytes, 2026-09-28):
standard MIT body under three copyright lines, "Original work Copyright (c) 2012 Liam Staskawicz /
Modified work Copyright (c) 2016 Vojtech Vitek / Modified work Copyright (c) 2021 Michael Fridman,
Vojtech Vitek". Licence-file history shows no relicence, only "Improve LICENSE readability" (2016)
and "Standardise the license (#648)" (2023-11-22).

Two things the researcher did not report.

**First, GitHub's own licence detector returns `NOASSERTION` for `pressly/goose`** (`gh api
repos/pressly/goose --jq .license.spdx_id`, 2026-09-28). The multi-copyright preamble defeats the
matcher. Any licence gate or SBOM tool that keys on GitHub metadata or `licensee` will report goose
as *unknown licence* — which in a KPMG review reads as worse than a known copyleft. We need a
manual allow-list entry with the file quoted, or the gate fails on a dependency that is actually
clean.

**Second, and materially: `github.com/pressly/goose/v3`'s `go.mod` lists
`github.com/go-sql-driver/mysql` as a *direct* require, and that module is MPL-2.0**
(`https://raw.githubusercontent.com/go-sql-driver/mysql/master/LICENSE`, HTTP 200, 2026-09-28 —
"Mozilla Public License Version 2.0"). The researcher presents goose as unambiguously MIT and puts
MPL-2.0 only against River in §4.4. MPL-2.0 is permitted by our rule, so this does not block
adoption — but it is a **conditional** we did not know we were taking.

The rest of goose's direct requires, all verified permissive or already known:
`ClickHouse/clickhouse-go/v2`, `microsoft/go-mssqldb`, `vertica/vertica-sql-go`,
`ydb-platform/ydb-go-sdk/v3`, `tursodatabase/libsql-client-go`, `modernc.org/sqlite`,
`ziutek/mymysql` (**BSD-3-Clause**, `https://raw.githubusercontent.com/ziutek/mymysql/master/LICENSE`,
"Copyright (c) 2010, Michal Derkacz", 2026-09-28), `moby/moby/api`, `moby/moby/client`,
`Azure/azure-sdk-for-go/sdk/azcore` + `azidentity` (indirect),
`AzureAD/microsoft-authentication-library-for-go` (indirect), `jackc/pgx/v5`.

That is the real finding on goose, and it is a supply-chain point rather than a licence one: **a
schema-migration tool for one database drags in eight database drivers, the Docker API client, and
the Azure SDK.** For a product that ships to regulated customers and gets scanned, that is a large
transitive surface for a small job. `golang-migrate/migrate` (MIT) has the same shape. If we want a
narrow dependency, the honest options are (a) accept the graph, (b) run goose only as a **CLI in
CI** so it never links into the shipped Hub binary, or (c) write the 150-line embedded-SQL migrator
ourselves. Option (b) costs nothing and removes the whole question from the shipped artifact.

### D3 — in-toto ships only the Apache-2.0 **short notice**, and GitHub calls both repos `NOASSERTION`

`in-toto/attestation`'s `LICENSE` is **559 bytes** and `in-toto/in-toto-golang`'s is **560 bytes**
(both HTTP 200, 2026-09-28). They are not the Apache-2.0 licence — they are the Apache-2.0
*boilerplate notice* that normally goes in a file header:

> `Copyright 2021 in-toto Developers` … `Licensed under the Apache License, Version 2.0 (the
> "License"); you may not use this file except in compliance with the License.`

Consequences, both real:

1. **GitHub reports `NOASSERTION` for both repositories** (`gh api`, 2026-09-28) — same gate problem
   as goose, on the two components the researcher calls "the single highest-value item in this
   document".
2. **If we vendor** (the researcher's verdict on `in-toto-golang` is *vendor the DSSE + statement
   types*), Apache-2.0 **§4(a)** requires us to give recipients "a copy of this License". Upstream
   does not ship one, so **we must add the full Apache-2.0 text ourselves** next to the vendored
   files, together with the upstream copyright line. Vendoring a file that says "see the License"
   without shipping the License is a defect a customer's counsel can find in ten seconds.

Neither repo has a `NOTICE` file (both HTTP 404, checked 2026-09-28), so §4(d) does not bite here.

Dependency check on the two, since they would ship:
- `github.com/in-toto/attestation` requires only `google.golang.org/protobuf` (**BSD-3-Clause**,
  `https://raw.githubusercontent.com/protocolbuffers/protobuf-go/master/LICENSE`, 2026-09-28) and
  `stretchr/testify` (MIT, test-only). Clean — but note that adopting the Go types package means
  **taking the protobuf runtime into the Hub**, which the researcher does not mention. If we only
  want the JSON shape, the researcher's own fallback (a 40-line hand-written struct with an
  attribution header) avoids protobuf entirely and is the better call.
- `github.com/in-toto/in-toto-golang` requires `secure-systems-lab/go-securesystemslib` (**MIT**,
  2026-09-28), `shibumi/go-pathspec` (**Apache-2.0**, 2026-09-28), `spf13/cobra` (Apache-2.0),
  `spiffe/go-spiffe/v2` (Apache-2.0) and **`google.golang.org/grpc`** (Apache-2.0). Clean on licence;
  **gRPC and SPIFFE are a heavy graph for "a DSSE envelope"**, which supports vendoring narrowly or
  not at all.

### D4 — `riverqueue/river`: a **relicence** the researcher did not date, and a **commercial private module** beside it

MPL-2.0 is right (`https://raw.githubusercontent.com/riverqueue/river/master/LICENSE`, HTTP 200,
16726 bytes, "Mozilla Public License Version 2.0", 2026-09-28; `gh api` SPDX `MPL-2.0`). Two
additions.

**The relicence.** `gh api repos/riverqueue/river/commits?path=LICENSE` returns exactly two commits:
`2023-11-14  move license to LICENSE to fix subpackage licensing (#25)` and **`2023-11-22  change
license to MPL-2.0 (#60)`**. River was **not** MPL-2.0 before 2023-11-22. The rule says a project
that relicensed must be reported with both licences and the version boundary; the researcher
reported neither. The boundary is harmless for us (we would take v0.47.0, 2026-08-31) but it must be
in the record, because it establishes that **this project's maintainers have changed the licence
once already**.

**The commercial add-on — the trap the licence rule warns about.** `https://riverqueue.com/pro`
(fetched 2026-09-28) describes **River Pro**, a paid subscription from $125/month whose features are
workflows, concurrency control, sequences, batching, dead-letter queues and encrypted jobs, and
which is, in the page's own words, **"distributed as a private Go module and is available through a
paid subscription."** The open-source core "is open-source software and always will be" (same page).
So River is precisely the **permissive-core / restricted-add-on** shape the researcher flagged for
`goauthentik/authentik` but did not flag for River. Nothing forbidden happens as long as we import
only `github.com/riverqueue/river` and its public sub-modules — but a developer reaching for
"workflows" lands in a paid private module, and the failure mode is a procurement conversation, not
a compile error.

**Sub-module licensing.** River's root `go.mod` requires four *separate Go modules that live in the
same repository*: `riverdriver`, `riverdriver/riverpgxv5`, `rivershared`, `rivertype` (all v0.47.0).
**None of them has its own `LICENSE` file** — `.../master/riverdriver/LICENSE` and
`.../master/rivershared/LICENSE` both return **HTTP 404** (2026-09-28). They rely on the repository
root `LICENSE`, which is what commit #25 was for. That is normal and it is fine, but a licence gate
that resolves licences per Go module will show four modules with **no licence file**. Record it, or
the gate will find it for us.

**Verdict: conditional, as the researcher said — with the obligation stated precisely.** MPL-2.0
§3.2: if we modify a river file, we publish that file's source. §3.1: ship the MPL text. We may keep
our own code proprietary and link freely. Add one more: **never `riverpro`.**

### D5 — SLSA: the summary row says Apache-2.0; the licence file says **Community Specification License 1.0**

Row 7 of the researcher's table reads "SLSA Provenance v1 predicate | Apache-2.0 (spec repo)". I
fetched `https://raw.githubusercontent.com/slsa-framework/slsa/main/LICENSE.md` (HTTP 200, 16129
bytes, 2026-09-28). Its first line is:

> `# Community Specification License 1.0`

and its purpose clause says it governs how contributors participate in the working group and "how
the materials developed under this License may be used", adding explicitly: **"It is not intended
for source code."** There is no Apache-2.0 text in that file. The researcher's body §2.2 is closer
("dual-licensed … Community Specification License 1.0 … and Apache-2.0 for pre-existing portions"),
but **the summary row a reviewer will actually read states the wrong licence family**, and the
summary row is what a licence gate gets pasted from.

Practically this is low risk and I still say **adopt the format**: a `predicateType` URI and a JSON
field layout are not copyrightable expression, and implementing a published specification is what it
is published for. CSL-1.0 additionally carries a **patent commitment on "Necessary Claims"** and a
**trademark clause**, both of which are favourable to an implementer. The obligation is narrow and
specific: **do not paste SLSA specification prose into PanaceaApp documentation** without recording
CSL-1.0/CC-BY attribution, and **do not claim SLSA conformance** in product material without meeting
the levels (a trademark/marketing question, not an engineering one).

### D6 — `transparency-dev/merkle` is Apache-2.0 and effectively unreleased

Licence confirmed (`.../merkle/main/LICENSE`, HTTP 200, 11358 bytes, full Apache-2.0 text,
2026-09-28; `gh api` SPDX `Apache-2.0`; no NOTICE file — 404). Its only require is
`github.com/google/go-cmp` (BSD-3-Clause, verified 2026-09-28). As clean as a dependency gets.

What the researcher omits: **41 stars, and the latest GitHub release is `v0.0.2`, published
2023-05-05** — over three years ago, against a brief that says prefer a release in the last twelve
months. The repository itself is alive (last push 2026-09-28) and it is maintained by the
transparency.dev team behind Trillian and Tessera, so this is not abandonware; it is a **stable,
finished, pre-1.0 library**. That is a defensible adoption, but "adopt" without the release date
beside it is an incomplete recommendation for a component that computes our audit log's inclusion
proofs. Either pin the pseudo-version and record that we accept a pre-1.0 dependency, or copy the
~300 lines of RFC 6962 hashing under Apache-2.0 attribution.

### D7 — `CycloneDX/cyclonedx-go` and `cedar-policy/cedar-go` also carry NOTICE files

Same §4(d) obligation as D1, unmentioned in the research:
- `https://raw.githubusercontent.com/CycloneDX/cyclonedx-go/master/NOTICE` (HTTP 200, 2026-09-28):
  "CycloneDX Go / Copyright (c) OWASP Foundation / This product includes software developed by the
  CycloneDX community (https://cyclonedx.org/)."
- `https://raw.githubusercontent.com/cedar-policy/cedar-go/main/NOTICE` (HTTP 200, 2026-09-28):
  "Copyright Cedar Contributors".

Checked and **absent** (HTTP 404, 2026-09-28): `Nerzal/gocloak`, `transparency-dev/merkle`,
`openfga/openfga`, `in-toto/attestation`, `in-toto/in-toto-golang`.

One practical caveat on `cyclonedx-go` that is not a licence issue but decides whether we adopt it:
its `go.mod` lists **`github.com/terminalstatic/go-xsd-validate` as a direct require**. That package
is **MIT** (`.../go-xsd-validate/master/LICENSE`, HTTP 200, "Copyright (c) 2017 Wolfgang Grimm",
2026-09-28) but it is a **cgo binding to libxml2**, i.e. a C toolchain and libxml2 headers at build
time. On a Windows developer machine behind the KPMG proxy that is a real cost. It is only needed
for XML BOM validation; if we emit JSON SBOMs we should confirm the import graph avoids it before
adopting. **Unverified: whether importing only the JSON encoder avoids the cgo package.** Check with
`go list -deps` at adoption time.

### D8 — `zalando/go-keyring`: the researcher's 404 was right, and the fix is right

Recorded because the researcher flagged it honestly and I can now close it. `.../go-keyring/main/LICENSE`
is **HTTP 404** because the default branch is `master`. I read `.../go-keyring/master/LICENSE`
(HTTP 200, 1077 bytes, 2026-09-28): "The MIT License (MIT) / Copyright (c) 2016 Zalando SE", full MIT
body. `gh api` SPDX `MIT`. Dependencies: `danieljoos/wincred` v1.2.3 (**MIT**, verified, the Windows
backend we actually use), `godbus/dbus/v5` (**BSD-3-Clause**, verified — Linux path only),
`golang.org/x/sys` (BSD-3-Clause). **Clean. I agree with adopt.**

### D9 — `golang.org/x/oauth2`: licence read from a rendering, not a file; and the "already a dependency" claim checks out

The researcher's URL is `https://pkg.go.dev/golang.org/x/oauth2?tab=licenses` — a rendering of the
file, not the file. I read `https://raw.githubusercontent.com/golang/oauth2/master/LICENSE` (HTTP
200, 1453 bytes, 2026-09-28): "Copyright 2009 The Go Authors." followed by the three-clause BSD text
including the no-endorsement clause. **BSD-3-Clause confirmed**; `gh api` SPDX `BSD-3-Clause`.

I also verified the claim that it is already in our tree: `control-plane/go.mod` line 54 reads
`golang.org/x/oauth2 v0.21.0 // indirect`, and `go.sum` has two entries for it. **The researcher's
"no new supply chain" argument is true.** Note `golang/oauth2` publishes **no GitHub releases**
(`gh api .../releases/latest` → 404); it is versioned by Go module tags, currently v0.36.0 as
required by `coreos/go-oidc/v3`. That is not a maturity concern; it is how `golang.org/x/*` works.

---

## 3. Where the researcher was right, and I tried to break it

I attacked these and failed to find a defect. Each licence read as a raw file on 2026-09-28, each
SPDX id cross-checked against GitHub's detector via authenticated `gh api`.

| Candidate | File read | Identifying line quoted from the file | GitHub SPDX | Relicence history |
|---|---|---|---|---|
| `jackc/pgx/v5` | `.../jackc/pgx/master/LICENSE` (1078 B) | "Copyright (c) 2013-2021 Jack Christensen" + "MIT License" | `MIT` | 2 commits ever: "Add license" 2013-07-31, "Update copyright date" 2021-03-25. **No relicence.** |
| `sqlc-dev/sqlc` | `.../sqlc-dev/sqlc/main/LICENSE` (1067 B) | "MIT License / Copyright (c) 2024 Riza, Inc." | `MIT` | 3 commits: "Initial open source commit" 2019-07-09, LICENSE/README cleanup 2023-11-06, "Set current year in docs and license" 2024-07-16. **Copyright holder changed (Kyle Conroy → Riza, Inc.); licence did not.** |
| `casbin/casbin` | `.../casbin/casbin/master/LICENSE` (12602 B, appendix present) | Apache-2.0 full text | `Apache-2.0` | "Initial commit" 2017-04-08; "chore: complete ASF license cleanup (#1744)" 2026-07-24 — still Apache-2.0 in the file I read today. |
| `Nerzal/gocloak` | `.../Nerzal/gocloak/main/LICENSE` (11346 B) | Apache-2.0 full text; "Copyright [2021]" style appendix | `Apache-2.0` | 2 commits: "Initial commit" 2018-11-14, "fix license" 2021-05-17. No relicence. |
| `openfga/openfga` | `.../openfga/openfga/main/LICENSE` (10176 B) | Apache-2.0, terms only (no appendix) | `Apache-2.0` | not examined — study-only verdict |
| `cedar-policy/cedar-go` | `.../cedar-policy/cedar-go/main/LICENSE` (10142 B) | Apache-2.0, terms only | `Apache-2.0` | not examined — study-only verdict |
| `google/trillian` | `.../google/trillian/master/LICENSE` (11358 B) | Apache-2.0 full text | `Apache-2.0` | not examined |
| `transparency-dev/tessera` | `.../transparency-dev/tessera/main/LICENSE` (11357 B) | Apache-2.0 full text | `Apache-2.0` | not examined |
| `zitadel/oidc` | `.../zitadel/oidc/main/LICENSE` (11357 B) | Apache-2.0 full text | `Apache-2.0` | not examined |
| `secure-systems-lab/dsse` (format) | `.../dsse/master/LICENSE` | Apache-2.0 full text | not queried | — |

**I also checked every Apache-2.0 file's tail for an appended clause** — a Commons Clause,
"additional use grant" or field-of-use rider bolted onto the end is the cheapest way to smuggle a
restriction past a reviewer who reads only the first line. All ten Apache files end at the standard
"END OF TERMS AND CONDITIONS" / boilerplate-notice point with nothing appended. **No Commons Clause
anywhere in this set.**

### Direct dependencies that would ship in our artifact

| Dependency | Pulled in by | Licence, read at source 2026-09-28 |
|---|---|---|
| `github.com/go-jose/go-jose/v4` | `coreos/go-oidc/v3`, `zitadel/oidc` | **Apache-2.0** |
| `golang.org/x/oauth2` | `coreos/go-oidc/v3` | **BSD-3-Clause** |
| `github.com/danieljoos/wincred` | `zalando/go-keyring` (Windows) | **MIT** |
| `github.com/godbus/dbus/v5` | `zalando/go-keyring` (Linux) | **BSD-3-Clause** |
| `github.com/jackc/puddle/v2` | `pgx/v5`, `river` | **MIT** |
| `github.com/jackc/pgpassfile` | `pgx/v5` | **MIT** |
| `github.com/jackc/pgservicefile` | `pgx/v5` | **MIT** |
| `github.com/google/go-cmp` | `transparency-dev/merkle` | **BSD-3-Clause** |
| `google.golang.org/protobuf` | `in-toto/attestation` | **BSD-3-Clause** |
| `github.com/secure-systems-lab/go-securesystemslib` | `in-toto-golang` | **MIT** |
| `github.com/shibumi/go-pathspec` | `in-toto-golang` | **Apache-2.0** |
| `github.com/terminalstatic/go-xsd-validate` | `cyclonedx-go` | **MIT** (cgo → libxml2) |
| `github.com/go-sql-driver/mysql` | **`pressly/goose/v3`** | **MPL-2.0** — see D2 |
| `github.com/ziutek/mymysql` | `pressly/goose/v3` | **BSD-3-Clause** |

**No copyleft dependency infects anything we would ship**, with the single MPL-2.0 exception in
goose's graph, and MPL-2.0 is permitted-with-obligation under our rule.

### Rejected projects: I re-read the licences rather than trust the rejection

A wrong rejection costs us a good component, so the rejects were verified too.

| Project | Read at | What the file actually says |
|---|---|---|
| `codenotary/immudb` | `.../codenotary/immudb/master/LICENSE`, HTTP 200 | **BUSL 1.1. Confirmed, and worse than reported.** "Licensor: Codenotary, Inc. / Licensed Work: immudb Version 1.9DOM.2 or later." The *Additional Use Grant* is an explicit **competitor field-of-use restriction**: production use is allowed "provided Your use does not include offering the Licensed Work to third parties on a hosted or embedded basis in order to compete with Codenotary's paid version(s)". PanaceaApp embeds and hosts. **Forbidden twice over.** Researcher correct. |
| `redis/redis` | `.../redis/redis/unstable/LICENSE.txt`, HTTP 200 | **Confirmed tri-licence from Redis 8**: "your choice of: (a) the Redis Source Available License v2 (RSALv2); or (b) the Server Side Public License v1 (SSPLv1); or (c) the GNU Affero General Public License v3 (AGPLv3)", and "Redis Open Source 7.2 and prior releases remain subject to the BSDv3 clause license". All three current options are forbidden. Researcher correct. |
| `valkey-io/valkey` (the substitute) | `.../valkey-io/valkey/unstable/COPYING`, HTTP 200 | **BSD 3-Clause License. Confirmed — and this is the file the researcher said they did NOT read.** "Copyright (c) 2024-present, Valkey contributors / Copyright (c) 2006-2020, Redis Ltd." Gap closed: valkey is usable. |
| `minio/minio`, `grafana/grafana`, `hashicorp/vault`, Elastic, `goauthentik/authentik` | not re-read this pass | Researcher's quotes are specific and internally consistent. **Marked UNVERIFIED by me** — see §5. None is a candidate for adoption, so the cost of leaving them unverified is nil. |

---

## 4. The obligations register — what adopting this set actually commits us to

This is the part a licence gate cannot infer. Written so it can be lifted straight into the
third-party notices work.

**4.1 A THIRD-PARTY-NOTICES file, shipped with both the Hub image and the desktop cockpit.** It must
contain, verbatim:
1. The **full Apache-2.0 licence text**, once.
2. The **NOTICE contents** of `coreos/go-oidc` ("CoreOS Project / Copyright 2014 CoreOS, Inc …"),
   `CycloneDX/cyclonedx-go` ("CycloneDX Go / Copyright (c) OWASP Foundation …") and
   `cedar-policy/cedar-go` ("Copyright Cedar Contributors") **if we adopt those**. Apache-2.0 §4(d).
3. The **MIT text with each copyright line** for `zalando/go-keyring` (Zalando SE 2016),
   `danieljoos/wincred` (Daniel Joos 2014), `jackc/pgx` (Jack Christensen 2013-2021), `jackc/puddle`,
   `jackc/pgpassfile`, `jackc/pgservicefile`, `secure-systems-lab/go-securesystemslib` (NYU 2021),
   and `pressly/goose`'s three lines if goose ships.
4. The **BSD-3-Clause text** for `golang.org/x/oauth2`, `golang.org/x/sys`, `golang.org/x/text`,
   `google.golang.org/protobuf`, `google/go-cmp`, `godbus/dbus`.
5. The **MPL-2.0 text** if `riverqueue/river` or `go-sql-driver/mysql` ship.

**4.2 MPL-2.0, stated exactly** (river; go-sql-driver/mysql via goose). We may link MPL code into a
proprietary binary and keep our own source closed — MPL is **file-level**, not project-level. Two
duties: §3.1, distribute the MPL text with the binary; §3.2, **if we modify an MPL-covered file, we
publish that file's source**. Practical rule for the programme: **never patch a vendored MPL file.**
Fork upstream, or wrap it in our own file. `OD-11`'s CI licence gate should fail the build if any
file under `vendor/` carrying an MPL header differs from its upstream copy — that is a byte
comparison, cheap to implement, and it converts a legal obligation into a test.

**4.3 Apache-2.0 §4(b): state changes.** If we modify any Apache-2.0 file we carry, the modified
files must "carry prominent notices stating that You changed the files". Same rule as above: do not
modify vendored files.

**4.4 The `spdx/tools-golang` election, if we ever take it.** `LICENSE.code` (read 2026-09-28) says
"The tools-golang source code is provided and may be used, at your option, under either: Apache
License, version 2.0 (Apache-2.0), OR GNU General Public License, version 2.0 or later
(GPL-2.0-or-later)." We would elect **Apache-2.0**, and that election must be **written down in the
notices file** — otherwise every SPDX-expression scanner we meet for the next decade reports GPL in
a KPMG product. The researcher flagged this correctly; I confirm the file and recommend
`CycloneDX/cyclonedx-go` instead, which has one unambiguous licence.

**4.5 Three dependencies GitHub cannot classify.** `pressly/goose`, `in-toto/attestation` and
`in-toto/in-toto-golang` all return **`NOASSERTION`**. The licence gate needs a **manual override
list** with, for each entry: the raw URL, the date read, the quoted identifying line, and the
approver. Without it these three fail the gate, someone overrides them under deadline pressure, and
the override habit is how an AGPL component gets in.

**4.6 River Pro is out of bounds.** Add `github.com/riverqueue/riverpro` (and any `riverpro/*` path)
to the gate's **deny** list now, before anyone reaches for workflows. Paid private Go module, per
riverqueue.com/pro, 2026-09-28.

**4.7 Clean-room discipline is unaffected by anything here.** Nothing in this candidate set is a
canvas, an editor or a design tool. No AGPL or GPL source was opened during this audit. `Paperdesign`,
`kgoedecke/doop` and every AGPL canvas source remain unopened and unread.

---

## 5. UNVERIFIED — say it plainly

1. **`minio/minio` (AGPL-3.0), `grafana/grafana` (AGPL-3.0), `hashicorp/vault` (BUSL 1.1, ≥1.15),
   Elastic/Kibana (Elastic 2.0 / SSPL), `goauthentik/authentik` (MIT core + `authentik/enterprise/`
   carve-out), `openbao/openbao` (MPL-2.0), `keycloak/keycloak` (Apache-2.0),
   `goharbor/harbor` (Apache-2.0), `golang-migrate/migrate` (MIT), `99designs/keyring` (MIT),
   `authzed/spicedb`, `cerbos/cerbos`, `ory/keto`, `open-policy-agent/opa`** — **not re-read by me
   this pass.** The researcher's quotes are specific and plausible; I simply did not spend the
   fetches, because none of them is on the adopt or vendor list. **Do not cite these as verified.**
   `keycloak/keycloak` is the one worth closing before Phase H ships, since `OD-30` depends on it.
2. **Whether importing `cyclonedx-go`'s JSON encoder avoids the cgo `go-xsd-validate` package.**
   Licence is MIT either way; the build cost on Windows is the open question. Settle with
   `go list -deps` at adoption time (D7).
3. **Contributor Licence Agreements.** I did not check CLA/DCO terms for any project. CLAs bind
   *contributors*, not *users*, so this does not affect consumption — but if PanaceaApp ever
   upstreams a patch to `goose`, `river`, `sqlc` or `gocloak`, check the CLA first. (`coreos/go-oidc`
   ships a `DCO` file in its repo root; that is a sign-off requirement for contributors, not a
   restriction on use.)
4. **Sub-package sweeps beyond the modules named.** I verified licences for every module we would
   import and every direct dependency that would ship. I did **not** walk the full transitive
   closure of `goose`'s driver graph (ClickHouse, YDB, Vertica, libsql, mssql, moby, Azure SDK) —
   see D2, and prefer running goose as a CI-only CLI so the question never applies to the artifact.
5. **`transparency-dev/merkle` beyond `go-cmp`** — there is nothing beyond `go-cmp`. Stated so the
   absence is not read as an omission.

---

## 6. What I would change in `05-hub-identity.md`

1. Row 1: change the `coreos/go-oidc` URL of record to the **`v3` branch**, and add the **NOTICE**
   obligation (D1).
2. Row 7: change "SLSA Provenance v1 predicate | Apache-2.0 (spec repo)" to **"Community
   Specification License 1.0 (spec text CC-BY-4.0)"** — the body is nearly right, the row a reviewer
   reads is not (D5).
3. Row 9 (`goose`): change **MIT** to **"MIT; `go.mod` requires MPL-2.0 `go-sql-driver/mysql`"**, and
   add the recommendation to run goose as a **CI-only CLI** (D2).
4. Rows 5, 6 (in-toto): note that upstream ships only the Apache-2.0 **short notice**, so vendoring
   obliges us to add the full licence text; and that both are **`NOASSERTION`** on GitHub (D3).
5. Row 8 (`merkle`): add **"latest release v0.0.2, 2023-05-05"** beside the adopt verdict (D6).
6. §4.4 (`river`): add the **2023-11-22 relicence** and a one-line **"River Pro is a paid private Go
   module — never import"** (D4).
7. §7.3 (licence-gate obligations): add the **NOASSERTION override list** (4.5), the **`riverpro`
   deny rule** (4.6), and the **vendored-MPL byte comparison** (4.2).

---

## 7. Audit method, so this can be re-run

- `curl -sS --ssl-no-revoke -L` against `raw.githubusercontent.com` for every `LICENSE`, `NOTICE`,
  `COPYING`, `LICENSE.md`, `LICENSE.code` and `go.mod`; HTTP status recorded for every fetch,
  including the 404s.
- `gh api repos/<r>` for `license.spdx_id`, stars, `pushed_at`, `archived`; `gh api
  repos/<r>/releases/latest` for the release date; `gh api repos/<r>/commits?path=LICENSE` for
  relicence history. **Unauthenticated `api.github.com` was rate-limited by the shared KPMG egress
  IP** ("API rate limit exceeded for 24.239.128.117"); authenticated `gh` worked.
- `riverqueue.com/pro` fetched through the web tool (the raw-file route does not apply to a
  marketing page).
- Every Apache-2.0 file's **tail** inspected for appended riders (Commons Clause, additional use
  grants). None found.
- Raw responses kept under the session scratchpad at
  `…\56a311e8-629e-4437-9e39-3cb2f503917f\scratchpad\lic\` for the life of the session; they are not
  committed.

**Nothing in this audit was built, run, installed or changed. No source file in the repository was
modified. The only file written is this one.**
