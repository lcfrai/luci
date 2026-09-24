### Task 16a: invites and their binding — routes 10–12

**Lane A. Depends on: 15. About 2.5 hours. Contract task H2.**

Teammates join by an **email invite that binds at their first sign-in** (ADR-H1 Q4): no Microsoft
Graph call, so no Graph consent (contract §7.3). This task builds the three invite routes and the
binding inside `/v1/me`. Review M4 split task 16 in two: task 16b builds the authorisation matrix
harness, and its rows for these three routes.

**Files:**
- `control-plane/internal/hub/server/invites.go` + `invites_integration_test.go` (new)
- `control-plane/internal/hub/store/invites.go` (new)
- `control-plane/internal/hub/server/me.go` — binding (modify)

#### Contract — the routes, complete

| # | Route | Role | Success | Refusals | Events |
|---|---|---|---|---|---|
| 10 | `GET /v1/teams/{teamId}/invites?after=&limit=` | V | `200 Page[Invite]`, newest first | 404 `team_not_found` | — |
| 11 | `POST /v1/teams/{teamId}/invites` `CreateInviteRequest` | A | `201 Invite`, `state: pending` | 404; 403 `forbidden`; 400 `field: email` (lowercased, 3–254 characters, exactly one `@`) or `field: role`; 409 `conflict` (a pending invite for that email in this team); 409 `conflict` `field: email` (that email already belongs to a member of this team) | `team.invite_created {"email","role"}` |
| 12 | `DELETE /v1/teams/{teamId}/invites/{inviteId}` | A | `204`; the row stays, `state: revoked` | 404 `team_not_found`; 403; 404 `invite_not_found`; 409 `conflict` (already `bound`) | `team.invite_revoked {}` |
| 3 | `GET /v1/me` — **binding** | any | `invitesBound`: the number bound by this call | — | per bound invite: `team.invite_bound {"userId","role"}`; then, if membership changed, `team.member_set {"role"}` |

**Binding** (contract §7.2, as correction A15 amends it): every `pending` invite **in the caller's
org** whose `email` equals the caller's **`Claims.BindEmail`** (task 13) becomes `bound`, with
`bound_user_id` and `bound_at`. `BindEmail` is the `email` claim alone, lowercased, and only when
this token shows a member account (`acct == 0`) or a verified email domain owner
(`xms_edov == true`); otherwise it is `""` and **nothing binds** — `invitesBound: 0`, the invites
stay `pending`, and no event is written. It is never `preferred_username` (mutable, and not
verified by the tenant for a B2B guest; Microsoft says not to authorise by it or by `email`), and
never the `users` row's stored `email`, which stays display text (review I12). When a binding
happens, the caller becomes a member with the invite's role, **or keeps a higher role already
held**. `/v1/me` runs under `Write` (task 14) so binding takes the org's audit lock; it sends no
`Idempotency-Key` because it is a `GET`, and binding twice is impossible because a bound invite
is no longer pending.

**A limitation, stated (plan re-review b05 m2).** A caller whose token cannot bind — a member
with no `email` claim, or a B2B guest without `xms_edov: true` — has **no in-product remedy in
v1**. Task 15's `PUT …/members/{userId}` needs their `u-…` id, and no v1 route lists an org's
users, and no v1 surface shows a user their own id: `GET /v1/me` returns it only to the control
plane that calls it, and `GET /v1/auth/status` carries no Hub user id (task 32). So the fallback is
out-of-band and needs an operator: someone with access to the Hub's database reads the id from
`users` and gives it to a team admin. Their invite stays `pending` with no reason shown to anyone. In v1's single KPMG
tenant this reaches only an account with no mail attribute or an unverified guest. The in-product
remedy is owner question Q12's second part; this task builds none.

#### Tests to write first

`invites_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestInviteCreateAndList` | Grace invites `Vera@Example.test` → stored and answered as `vera@example.test`; two invites list newest first | a case-sensitive email that never binds |
| `TestInviteEmailRules` | `nobody`, `a@b@c`, `x@`, 255 characters → 400 `field: email`; role `owner` → 400 `field: role` | — |
| `TestASecondPendingInviteIs409` | the same email twice → 409 `conflict` | two pending rows racing to bind |
| `TestInvitingAMemberIs409` | inviting Linus's email while he is a member → 409 `conflict` `field: email` | — |
| `TestRevokeKeepsTheRow` | revoke → 204; the list shows `state: revoked`; revoking a bound invite → 409 `conflict`; an unknown id → 404 `invite_not_found` | history deleted with the invite |
| `TestBindingOnFirstSignIn` | Grace invites Vera as `viewer`; Vera's `GET /v1/me` → `invitesBound: 1` and T with role `viewer`; a second call → 0; events `team.invite_bound` then `team.member_set` | an invite that never binds, or binds twice |
| `TestBindingKeepsAHigherRole` | Linus is `member`; an invite for him as `viewer` binds and leaves him `member` | a demotion by invite |
| `TestBindingStaysInTheOrg` | an invite in org 1 for `xavier@other.test` does not bind when Xavier (org 2) signs in | a cross-tenant join by email (T3) |
| `TestAGuestCannotBindAMembersInvite` | the test's own issuer, from the committed config with one user flipped (task 11: `Options.Config` is a value). (1) Out Sider as `guest: true`, `emailVerified: false`; Grace invites `out@example.test` as `member`; Out Sider's `GET /v1/me` → `invitesBound: 0`, the invite still `pending`, Out Sider not in T, no `team.invite_bound` event; (2) the same world with `emailVerified: true` → `invitesBound: 1`; (3) Vera a member with `email` set to `""` (her `preferred_username` still `vera@example.test`) and an invite for `vera@example.test` → `invitesBound: 0` | a guest's unverified `email` granting a member's invite (review I12); a `preferred_username` fallback for binding; binding that reads the stored `users.email` instead of this token |

#### Steps

- [ ] **Step 1:** Re-read contract §5.3, §7.2, §7.3 and §8.
- [ ] **Step 2:** The tests first. Run them and see them fail, since no invite route exists. Then
  `store/invites.go`, `invites.go`, and the binding in `me.go`.
- [ ] **Step 3:** Commit by pathspec, the four files.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestInvite|TestASecondPending|TestInvitingAMember|TestRevokeKeeps|TestBinding|TestAGuestCannotBind' \
  2>&1 | grep -c "^--- PASS"                                                                     # 9
```

#### What this task does not claim

- A people picker: v1 invites by email (ADR-H1 §5.9 item 7).
- Invitation email delivery: none is sent in v1 (contract §7.3).
- That these routes refuse the right callers in every column: the matrix rows for routes 10–12
  are task 16b's. Until then only this task's own tests hold them.
