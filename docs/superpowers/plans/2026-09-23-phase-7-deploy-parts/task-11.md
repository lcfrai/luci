### Task 11: Governance routes — auto-deploy, project access, branch posture, URL prefix

**Half:** 7a. **Hours:** 2. **Depends on:** 04, 08.

Four controls, drawn as the API really offers them (brief `B1`, corrected in plan §3.2): **project
access** is a list of people with a role; **branch posture** is one of five modes; **auto-deploy** and
the **URL prefix** are owner-only per-branch switches. The control plane adds **no** permission logic
of its own — Panacea enforces every guard (`routes.go:1308-1314`, `:1407-1409`, `:1430-1431`), and
its refusal is relayed verbatim. The watcher's `ownerControls` (task 07, plan 7P-9) lets the lens
draw owner controls disabled *before* a press, with the reason; it is never used to refuse here.

**Files:**
- `control-plane/internal/api/deploy_governance.go` (new) — nine routes
- `control-plane/internal/api/deploy_governance_test.go` (new)
- `control-plane/internal/api/routes_audit_test.go` (modified) — three `GET` lines
- `control-plane/internal/api/api.go` (modified) — `a.registerDeployGovernance(guarded)` in `New`; no new `Deps` field: the handlers read `Deps.DeployConnection` (task 05, whose `Current()` is the `Governor`) and `Deps.DeployBinding` (task 06, for the Panacea project, else `409 deploy_not_bound`)

#### Why the branch is a query parameter

A branch name contains `/` (`feat/evidence-upload`). As a path segment it would need `%2F`, and
whether each hop preserves that is one more thing to be wrong about. Every route below takes
**`?branch=<name>`**, URL-encoded by the desktop, decoded once by `r.URL.Query()`. Panacea's paths
still get `url.PathEscape(branch)` inside the connector (task 04).

#### Control-plane routes — complete wire

`{P}` is `/v1/projects/{projectId}/deploy`. Every route answers `409 deploy_not_bound` when the
local project has no Panacea binding, and `writeDeployError` (task 05) for every Panacea failure.

```
GET {P}/auto-deploy?branch=feat%2Fevidence-upload
  200 { "branch": "feat/evidence-upload", "explicit": true | false | null, "projectDefault": false,
        "effective": true, "productionBranch": false }
  403 deploy_forbidden   non-owner (routes.go:1308)
  409 deploy_conflict    "branch has no environment yet — deploy it once, or set the project default" (projects.go:4133)

PUT {P}/auto-deploy?branch=…
  request { "enabled": true }                         ("enabled" required, boolean)
  200 the GET body

GET {P}/posture?branch=…
  200 { "branch": "…", "environment": "…", "mode": "kpmg-sso", "modeKnown": true, "modeRaw": "kpmg-sso",
        "passwordSet": false, "bypassTokenCount": 0, "ipAllowlist": [] }

PUT {P}/posture?branch=…
  request { "mode": "panacea-sso" | "kpmg-sso" | "password" | "bypass-token" | "none",
            "password": "…",               (required for, and only sent with, "password")
            "rotateBypassToken": true }    (only with "bypass-token")
  200 { "posture": <the GET body>, "enforced": true | false, "message": "…",
        "bypassTokens": ["…"] }           (bypassTokens present only when Panacea minted one — shown ONCE)
  400 bad_request        a mode outside the five — refused here, never sent (deploy.ParsePostureMode);
                         "password" missing for "password"; "password" given for any other mode
  409 deploy_conflict    Panacea's SSO lock: "access mode cannot be relaxed: …" (project_security.go:733-735)

GET {P}/access
  200 { "grants": [ { "grantee": "a.person@kpmg.com.au", "group": "a.person@kpmg.com.au", "legacy": false,
                      "role": "viewer" | "developer" | "owner" | "unknown", "roleRaw": "editor" },
                    { "grantee": "team-old", "group": "project-engagement-portal-viewer", "legacy": true,
                      "role": "viewer", "roleRaw": "viewer" } ] }
      a person's row: group == grantee (Panacea's key for it); a legacy binding: group is the
      Keycloak group's name, grantee its panacea-group attribute (task 04; review M30)

POST {P}/access
  request { "grantee": "a.person@kpmg.com.au", "role": "viewer" | "developer" | "owner" }
  201 the GET body, re-read after the write
      A role change is this POST for a current member at the new role. Panacea's share is
      single-role and does the replace itself: it drops the grantee from the other role groups
      (clients/keycloak.go:198-214), then adds the new one (:216-218). So a role change is one
      request and Panacea's own replace, never a Revoke then a Grant. It is NOT safer than
      that pair: an add that fails after the drop is a 500 (handlers/access.go:55-58) and
      leaves the person with no role, and the drop is best-effort (:207-210). So the cockpit
      re-reads access after any failed write, draws a grantee still on two rows on both, and
      holds a change that lowers a role (task 18; review M30, re-review I-1)
  400 bad_request        grantee empty, or containing "," or whitespace (one person, never a list —
                         survey §4: Panacea's header parser splits on commas); role outside the three
  403 deploy_forbidden   non-owner (routes.go:1408)
  422 deploy_rejected    "No Panacea user matches that name or email." — Panacea answers this case
                         with a 500 (handlers/access.go:57); see below

DELETE {P}/access?group=a.person%40kpmg.com.au
  200 the GET body, re-read
  400 bad_request        "group" missing or empty
      "group" is the row's group, never its grantee: a person's row is removed from every role,
      a legacy row's group is deleted (keycloak.go:245-267). A legacy grantee would answer 200
      and remove nothing (:268) (review M30)

PUT {P}/url-prefix?branch=…
  request { "prefix": "uat-test" }
  200 { "branch": "…", "prefix": "uat-test", "host": "uat-test.panacea.kpmg" }
  409 deploy_conflict    "url prefix already taken: … is already claimed by another branch" (projects.go:4321)
  422 deploy_rejected    Panacea's label-rule sentence (handlers/branch_url_prefix.go:41 validateURLPrefix)

DELETE {P}/url-prefix?branch=…
  204
```

**One Panacea answer needs a deliberate exception, and it lives in the connector.**
`AccessHandler.Add` writes **500** for every `AddProjectAccess` error (`handlers/access.go:57`),
including *"no Keycloak user found for grantee …"* (`clients/keycloak.go:189`) and *"a grantee
(username or email) is required"* (`:182`) — a user's typo, not an outage. (`:182` is reached only by
a whitespace-only grantee: an empty one is Panacea's `400` first, `access.go:50-52`, and this route
refuses whitespace before sending; re-review M-2.) Task 02's rule hides every
5xx sentence. So `panacea.Client.Grant` (task 04), the only code that may know Panacea's wording,
maps a 500 whose text after `"failed to add access binding: "` **begins with** `no Keycloak user
found for grantee ` or **equals** `a grantee (username or email) is required` to
`KindRejected` with **our** sentence, *"No Panacea user matches that name or email."* — never the rest
of the body. This is reading an error string, which the plan otherwise refuses; plan 7P-12 names it
as the **single exception** to "no Panacea sentence is ever parsed" (review M8). It is kept because
it fails safe: a reworded phrase falls back to `KindUnavailable`, never to a wrong refusal. It is
confined to one connector method, pinned by a test that quotes both source lines, and any other 500
stays `KindUnavailable` — `:186`'s *"resolve grantee … user search returned 503 …"*, a Keycloak
outage behind the same prefix, included (re-review M-1). This route then answers `422 deploy_rejected` through `writeDeployError` like any
other refusal.

**Never logged:** `password`, any `bypassTokens` value. The request struct's `Password` field has a
redacting `String()`; the handler test captures the log and greps for the password.

#### Tests to write first (`deploy_governance_test.go`, against `deploytest.Fake`)

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestBranchIsAQueryParam` | `?branch=feat%2Fx` reaches the fake as `"feat/x"` | a path-segment branch that splits at the `/` |
| `TestUnknownModeNeverSent` | `{"mode":"public"}` → 400; the fake saw no `SetPosture` | forwarding a word Panacea would fail closed on, or worse, a future synonym |
| `TestPasswordOnlyWithPasswordMode` | `{"mode":"kpmg-sso","password":"x"}` → 400 | a stray password persisted against the wrong mode |
| `TestNotEnforcedCarried` | the fake returns `Enforced:false` + message → 200 with `"enforced": false` and the message. The message is the connector's own sentence (task 04 writes it; Panacea's embeds an `err.Error()`, review M7), so this handler passes it through and adds nothing | the lens drawing a saved-but-unapplied posture as done |
| `TestMintedTokenOnceNotLogged` | the fake mints `bt_123` → it is in the 200 body, not in the captured log, and a second `GET` does not carry it | a token in a log or a cache |
| `TestGranteeIsOnePerson` | `"a@x, b@y"` → 400 with *"One person at a time."* | a comma-joined list that Panacea would read as two groups (survey §4) |
| `TestGrantRereads` | `POST` → the response is the fake's `Access` after the write | a UI that shows what it asked for instead of what Panacea holds |
| `TestRoleChangeIsOneGrant` | a member at `viewer`, `POST {grantee, role:"owner"}` → the fake saw exactly one `Grant` and no `Revoke` | a role change as Revoke then Grant: two requests where Panacea's one replace does the job, and a second place to fail half-way (review M30, re-review I-1) |
| `TestRevokeSendsGroup` | `DELETE ?group=project-engagement-portal-viewer` → the fake's `Revoke` got that group; `DELETE ?grantee=team-old` → 400 | revoking a legacy row by its grantee, a 200 that removes nothing (review M30) |
| `TestNoUserFoundIs422` | the fake returns `KindRejected` *"No Panacea user matches that name or email."* → 422 with that sentence | a Keycloak typo shown as "Panacea is down" |
| `TestOwnerRefusalRelayed` | the fake answers `KindForbidden` *"insufficient role"* → 403 `deploy_forbidden` with that sentence | the control plane inventing its own permission check |
| `TestNotBound` | an unbound project → 409 `deploy_not_bound` on all nine routes | a governance write to a guessed project name |

`catalogReadOnlyRoutes` lines, each: *"Relays one Panacea read through the connector; writes
nothing locally."* — for `GET {P}/auto-deploy`, `GET {P}/posture`, `GET {P}/access`.

#### Steps

- [ ] **Read first:** `handlers/access.go:38-62` and `clients/keycloak.go:174-192` again (task 04 pins
  the two sentences in `TestGrantUnknownUserIsRejected` from the file, not from this part).
- [ ] Tests first; then the nine handlers; register through `routes`.
- [ ] Run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/api/ -race -count=1 -run 'Governance|Branch|Mode|Password|Enforced|Minted|Grantee|Grant|Revoke|RoleChange|NoUser|Owner|NotBound'
→ ok
```

#### What this task does NOT claim

- Per-branch Entra group access. The API has none (brief `B1`, survey §4); building it is a
  Panacea change the owner has not asked for.
- That `developer` is accepted on Panacea's wire — the connector sends `editor` (task 04).
- Anything the lens shows. Task 18.
