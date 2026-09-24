### Task 07b: `sql/0001_init.sql`, part 2 — the guards and the owner functions, and a database test for each

**Lane A. Depends on: 07a. About 3.5 hours. Contract task H0 (and the database half of G4, G5).**

Review M4 split task 07 in two. Task 07a wrote the tables, row-level security and the grants, and
proved tenancy. This task **appends** to the same file the guards and the five functions that
`hub_app` calls and that run as the owner. It proves, **in the database and without any Go
handler**, the rest of §10.7's "Database" column: a revision never changes, a head moves one step
onto a real revision, a pin never moves, append-only tables are append-only, and the audit chain is
gapless and hashed.

**One file, one migration.** `0001_init.sql` stays the only migration, so `Latest()` stays 1.
Task 06's runner refuses an applied migration whose text has changed (`ErrEdited`), so nothing
applies 0001 to a persistent database between 07a and 07b. The tests run on fresh databases
(`hubtest.Migrated`); a local `hub-pg-dev` migrated at 07a is recreated (task 06's
`docker rm -f hub-pg-dev`). **The orchestrator merges 07a and 07b into `feat/hub` together.**

**Two corrections that change behaviour**, folded into the contract by task 44a before this task
runs:
- **A18 (review M10):** `hub.org_ids()`, `SECURITY DEFINER`, returns the org ids and nothing
  else. Task 07a put `orgs` under row-level security, so `hub_app` sees no org before a
  `set_config`. The janitor (task 24) and `hub verify` (tasks 20, 27) list orgs through this
  function.
- **A19 (review M11):** `audit_append` is `SECURITY DEFINER` and checks the org **before** it
  locks anything. `hub_app` has no `UPDATE` on `audit_heads` (task 07a's grants). §10.6's write
  lock is `hub.lock_audit_head()`, since `SELECT … FOR UPDATE` needs `UPDATE` privilege.

**A17 (review I6) leaves `erase_brief` as the contract has it.** Route 26's stored response kept
the text for 24 hours. The fix is that it no longer stores the text at all: tasks 14 and 19 store
it with every `content` null and rebuild the content on replay from `brief_revisions`. So
`erase_brief` deletes nothing from `idempotency_keys`, and `TestEraseBriefLeavesEveryStoredResponse`
pins that (re-review b06 N1).

**Files:**
- `control-plane/internal/hub/migrate/sql/0001_init.sql`: append the guards and the owner
  functions after task 07a's grants (modify)
- `control-plane/internal/hub/migrate/guards_integration_test.go` (new), using task 07a's seed
  helper from `schema_integration_test.go`

#### Contract — complete (appended to task 07a's file, in this order)

```sql
-- 0001_init.sql, continued (task 07b): the guards and the owner functions. Appended after
-- task 07a's grants; the EXECUTE grants for the functions are with the functions.
-- ── append-only tables ───────────────────────────────────────────────────────
CREATE FUNCTION forbid_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'hub: % is append-only', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END $$;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['design_approvals', 'design_approval_blobs', 'session_links',
                           'release_deployments', 'evidence', 'audit_events'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON hub.%I
                    FOR EACH ROW EXECUTE FUNCTION hub.forbid_change()', t || '_append_only', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON hub.%I
                    FOR EACH STATEMENT EXECUTE FUNCTION hub.forbid_change()', t || '_no_truncate', t);
  END LOOP;
END $$;

-- ── a brief revision changes once, by erasure, and only inside hub.erase_brief ─
CREATE FUNCTION guard_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: brief revisions are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF current_user <> 'hub_owner'
     OR OLD.canonical IS NULL OR NEW.canonical IS NOT NULL OR NEW.erased_at IS NULL
     OR (NEW.org_id, NEW.brief_id, NEW.revision, NEW.project_id, NEW.scope, NEW.branch_ref,
         NEW.status, NEW.digest, NEW.saved_at, NEW.saved_by)
        IS DISTINCT FROM
        (OLD.org_id, OLD.brief_id, OLD.revision, OLD.project_id, OLD.scope, OLD.branch_ref,
         OLD.status, OLD.digest, OLD.saved_at, OLD.saved_by)
  THEN
    RAISE EXCEPTION 'hub: a brief revision is immutable' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER brief_revisions_guard BEFORE UPDATE OR DELETE ON brief_revisions
  FOR EACH ROW EXECUTE FUNCTION guard_revision();
CREATE TRIGGER brief_revisions_no_truncate BEFORE TRUNCATE ON brief_revisions
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();

-- ── a head moves forward one revision at a time, onto a revision that exists ──
CREATE FUNCTION guard_brief() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: briefs are archived, never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.scope, NEW.created_at, NEW.created_by)
       IS DISTINCT FROM (OLD.id, OLD.org_id, OLD.project_id, OLD.scope, OLD.created_at, OLD.created_by)
     OR NEW.head_revision NOT IN (OLD.head_revision, OLD.head_revision + 1)
     OR NEW.latest_complete_revision < OLD.latest_complete_revision
     OR (OLD.archived_at IS NOT NULL AND (NEW.archived_at IS DISTINCT FROM OLD.archived_at
                                          OR NEW.head_revision <> OLD.head_revision
                                          OR NEW.branch_ref <> OLD.branch_ref))
     OR (OLD.erased_at IS NOT NULL AND NEW.erased_at IS DISTINCT FROM OLD.erased_at)
     OR (NEW.head_revision <> OLD.head_revision AND NOT EXISTS
           (SELECT 1 FROM hub.brief_revisions r WHERE r.brief_id = NEW.id AND r.revision = NEW.head_revision))
  THEN
    RAISE EXCEPTION 'hub: illegal change to brief %', OLD.id USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER briefs_guard BEFORE UPDATE OR DELETE ON briefs
  FOR EACH ROW EXECUTE FUNCTION guard_brief();

-- ── a release's pin and identity never change; only a reserved release moves ─
CREATE FUNCTION guard_release() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: releases are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.kind, NEW.panacea_project,
      NEW.branch_ref, NEW.commit_sha, NEW.source_deployment_id, NEW.source_release_id,
      NEW.pin_reason, NEW.pin_brief_id, NEW.pin_brief_revision, NEW.pin_brief_digest,
      NEW.pin_head_revision_at_ship, NEW.design_key, NEW.reserved_at, NEW.reserved_by, NEW.expires_at)
     IS DISTINCT FROM
     (OLD.id, OLD.org_id, OLD.project_id, OLD.kind, OLD.panacea_project,
      OLD.branch_ref, OLD.commit_sha, OLD.source_deployment_id, OLD.source_release_id,
      OLD.pin_reason, OLD.pin_brief_id, OLD.pin_brief_revision, OLD.pin_brief_digest,
      OLD.pin_head_revision_at_ship, OLD.design_key, OLD.reserved_at, OLD.reserved_by, OLD.expires_at)
  THEN
    RAISE EXCEPTION 'hub: a release''s pin never changes' USING ERRCODE = 'insufficient_privilege';
  END IF;
  -- the environment is written once (HA7-1, task 00): a value never changes or returns
  -- to NULL; NULL -> a value only while reserved or confirmed (the join is confirmed-only)
  IF NEW.environment IS DISTINCT FROM OLD.environment AND
     (OLD.environment IS NOT NULL OR OLD.state NOT IN ('reserved', 'confirmed')) THEN
    RAISE EXCEPTION 'hub: release %''s environment is set once', OLD.id USING ERRCODE = 'insufficient_privilege';
  END IF;
  -- a non-reserved release takes no change but that one
  IF OLD.state <> 'reserved' AND
     (NEW.state, NEW.pipeline_run, NEW.confirmed_at, NEW.abandoned_at, NEW.abandon_reason,
      NEW.abandon_message, NEW.expired_at)
     IS DISTINCT FROM
     (OLD.state, OLD.pipeline_run, OLD.confirmed_at, OLD.abandoned_at, OLD.abandon_reason,
      OLD.abandon_message, OLD.expired_at)
  THEN
    RAISE EXCEPTION 'hub: release % is %, not reserved', OLD.id, OLD.state USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER releases_guard BEFORE UPDATE OR DELETE ON releases
  FOR EACH ROW EXECUTE FUNCTION guard_release();

-- ── the audit chain: gapless, in order, and it advances its own head ─────────
-- M11 (review), A19: SECURITY DEFINER, so the head moves only here. hub_app holds no UPDATE
-- on audit_heads (grants below), so a compromised Hub cannot move a head by a direct
-- UPDATE. As the owner this function is not bound by row-level security, so it checks the
-- org itself, FIRST, before it locks anything: without that check an org-A session could
-- lock, and for a moment move, org B's head. WITH CHECK would still refuse the row, but only
-- after BEFORE triggers have run (from knowledge). TestAuditAppendChecksTheOrgFirst pins the
-- check: with B's head held by another session, the check returns 42501 at once, where a
-- trigger without it blocks on that lock.
CREATE FUNCTION audit_append() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE h hub.audit_heads%ROWTYPE;
BEGIN
  IF NEW.org_id IS DISTINCT FROM current_setting('hub.org_id', true) THEN
    RAISE EXCEPTION 'hub: audit event for org %, not this session''s', NEW.org_id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  SELECT * INTO h FROM hub.audit_heads WHERE org_id = NEW.org_id FOR UPDATE;
  IF NOT FOUND OR NEW.seq <> h.seq + 1 OR NEW.prev_hash <> h.hash THEN
    RAISE EXCEPTION 'hub: audit event % out of order', NEW.seq USING ERRCODE = 'serialization_failure';
  END IF;
  UPDATE hub.audit_heads SET seq = NEW.seq, hash = NEW.hash WHERE org_id = NEW.org_id;
  RETURN NEW;
END $$;
CREATE TRIGGER audit_events_append BEFORE INSERT ON audit_events
  FOR EACH ROW EXECUTE FUNCTION audit_append();

-- ── the five functions hub_app calls that run as the owner (contract §10.4; ─────
-- ── the contract's three, and A18's org_ids and A19's lock_audit_head) ──────────
CREATE FUNCTION ensure_org(p_tenant uuid, p_name text, p_id text) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_id text;
BEGIN
  INSERT INTO orgs (id, entra_tenant_id, display_name) VALUES (p_id, p_tenant, p_name)
  ON CONFLICT (entra_tenant_id) DO UPDATE SET display_name = EXCLUDED.display_name
  RETURNING id INTO v_id;
  INSERT INTO audit_heads (org_id) VALUES (v_id) ON CONFLICT (org_id) DO NOTHING;
  RETURN v_id;
END $$;

CREATE FUNCTION resolve_principal(p_tenant uuid, p_oid uuid, p_name text, p_email text,
                                  p_new_user_id text)
RETURNS TABLE (org_id text, user_id text, created boolean)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text; v_user text; v_name text := left(p_name, 256); v_email text := lower(left(p_email, 254));
BEGIN
  SELECT o.id INTO v_org FROM orgs o WHERE o.entra_tenant_id = p_tenant;
  IF v_org IS NULL THEN RETURN; END IF;
  INSERT INTO users (id, org_id, entra_oid, display_name, email)
  VALUES (p_new_user_id, v_org, p_oid, v_name, v_email)
  ON CONFLICT ON CONSTRAINT users_org_id_entra_oid_key DO NOTHING
  RETURNING users.id INTO v_user;
  IF v_user IS NOT NULL THEN
    RETURN QUERY SELECT v_org, v_user, true; RETURN;
  END IF;
  UPDATE users u SET display_name = v_name, email = v_email, last_seen_at = now()
   WHERE u.org_id = v_org AND u.entra_oid = p_oid
     AND (u.display_name IS DISTINCT FROM v_name OR u.email IS DISTINCT FROM v_email
          OR u.last_seen_at < now() - interval '1 hour');
  SELECT u.id INTO v_user FROM users u WHERE u.org_id = v_org AND u.entra_oid = p_oid;
  RETURN QUERY SELECT v_org, v_user, false;
END $$;

CREATE FUNCTION erase_brief(p_brief text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true); v_n integer;
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM briefs b WHERE b.id = p_brief AND b.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: no brief % in this org', p_brief USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE brief_revisions SET canonical = NULL, erased_at = now()
   WHERE brief_id = p_brief AND canonical IS NOT NULL;
  GET DIAGNOSTICS v_n = ROW_COUNT;
  UPDATE briefs SET archived_at = coalesce(archived_at, now()),
                    erased_at   = coalesce(erased_at, now()),
                    updated_at  = now()
   WHERE id = p_brief;
  -- I6 (review), A17: idempotency_keys is NOT touched. No stored response holds brief text:
  -- route 26 stores its reply with every content null and rebuilds content on replay from
  -- these rows (tasks 14, 19), so a replay after this call answers content null, as /v1/refs
  -- does. A purge here would delete the replays of approvals, evidence, releases, co-batched
  -- briefs and this erase itself (re-review b06 N1), and turn their retries into second writes.
  RETURN v_n;
END $$;

-- M11 (review), A19: the org's write lock (contract §10.6's first statement after set_config).
-- SELECT … FOR UPDATE needs UPDATE privilege on at least one column of the table (from
-- knowledge; TestTheHeadLockNeedsTheFunction pins it), so with hub_app's UPDATE on
-- audit_heads revoked the lock is taken here, as the owner, and only on the session's org:
-- there is no parameter to name another. The row lock belongs to the calling transaction.
CREATE FUNCTION lock_audit_head() RETURNS TABLE (seq bigint, hash bytea)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true);
BEGIN
  RETURN QUERY SELECT h.seq, h.hash FROM audit_heads h WHERE h.org_id = v_org FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'hub: no audit head for org %', v_org USING ERRCODE = 'no_data_found';
  END IF;
END $$;

-- M10 (review), A18: the org ids, and nothing else, for the two callers that must visit
-- every org before any hub.org_id is set: the janitor (task 24) and hub verify (tasks 20,
-- 27), both on hub_app's connection. With orgs under row-level security, a plain SELECT
-- there sees no org at all, and the janitor would sweep nothing and hub verify would pass
-- on an empty walk. No name and no tenant id leaves this function.
CREATE FUNCTION org_ids() RETURNS SETOF text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT id FROM orgs ORDER BY id
$$;

REVOKE ALL ON FUNCTION ensure_org(uuid, text, text), resolve_principal(uuid, uuid, text, text, text),
                       erase_brief(text), lock_audit_head(), org_ids() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ensure_org(uuid, text, text), resolve_principal(uuid, uuid, text, text, text),
                          erase_brief(text), lock_audit_head(), org_ids() TO hub_app;

```

**Five Postgres facts the tests pin rather than trust** (all from knowledge, unverified here):
- inside a `SECURITY DEFINER` function `current_user` is the owner, which is what lets
  `erase_brief` pass `guard_revision` (`TestOnlyEraseBriefCanEraseARevision`);
- `SELECT … FOR UPDATE` needs `UPDATE` privilege on at least one column of the table, which is why
  the lock is `lock_audit_head()` (`TestTheHeadLockNeedsTheFunction`);
- a row lock taken inside a `SECURITY DEFINER` function belongs to the calling transaction and is
  held until it ends (the same test's second connection);
- row-level security's `WITH CHECK` is enforced **after** `BEFORE` triggers fire, which is why
  `audit_append` checks the org itself (`TestAuditAppendChecksTheOrgFirst`). The mutant run tells
  which order holds: `55P03` means the trigger ran first and reached the lock; a `42501` with the
  row-level-security message means `WITH CHECK` ran first. The test goes red on both (it asserts
  the message), but in the second case the check is only defence in depth. Record which one was
  seen in the commit message;
- `lock_timeout` bounds a row-lock wait inside a trigger, so the mutant fails `55P03` after 2 s
  rather than hanging (the same test; give it a 10 s context deadline so a wrong fact fails
  rather than stalls). An error inside a transaction block releases that transaction's locks at
  once, not at `ROLLBACK` (re-review b07-1 N1), which is why the test holds the lock in a
  **live** transaction on connection 2, not in connection 1's failed one.

#### Tests to write first — `guards_integration_test.go` (`//go:build integration`)

Each runs on `hubtest.Migrated(t)`, seeded with task 07a's helper. The helper writes rows in the
order the Hub will, so these guards accept its seed. `ensure_org` and `resolve_principal` are
tested by calling them directly. "Refused" means the statement returns a `*pgconn.PgError` whose
`Code` is the one named.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAppendOnlyTriggersHoldEvenForTheOwner` | as `hub_owner`, `UPDATE`, `DELETE`, `TRUNCATE` on each of the six append-only tables → `42501` from `forbid_change` | append-only enforced by grants alone |
| `TestOnlyEraseBriefCanEraseARevision` | as the owner outside the function, an erasure-shaped `UPDATE` passes `guard_revision` only because `current_user` is `hub_owner`; any other column change → `42501`; `erase_brief` as `hub_app` erases and returns the count | the revision guard trusting the caller |
| `TestAHeadMovesOneStepOntoARealRevision` | `head_revision` +2 → `42501`; +1 with no revision row → `42501`; +1 after inserting that revision → ok; `latest_complete_revision` backwards → `42501` | a head pointing at nothing (P3) |
| `TestAnArchivedBriefIsFrozen` | after `archived_at` is set: head, ref and `archived_at` changes → `42501`; a new live brief on the same ref → ok | branch deletion that frees the ref but not the old brief |
| `TestAPinNeverMoves` | as `hub_app`, `UPDATE releases SET pin_brief_revision = …` → `42501` (column grant); as the owner → `42501` (trigger) | P7 held by one layer only |
| `TestOnlyAReservedReleaseMoves` | confirm a release; any further `UPDATE` of `state` → `42501` | a confirmed release re-confirmed with another run |
| `TestANullEnvironmentReleaseConfirmsAbandonsAndExpires` | three NULL-environment runs: `UPDATE` to `confirmed` (with `pipeline_run`), to `abandoned`, to `expired` → each ok, `environment` still NULL | the review's `state = 'reserved'` check, which refuses all three (task 00) |
| `TestTheEnvironmentIsSetOnce` | a NULL-environment run: `SET environment = 'feat-x'` while `confirmed` → ok; then `= 'feat-y'` → `42501`; then `= NULL` → `42501`; a NULL one `abandoned` then `SET environment` → `42501`; a non-NULL reserved one `SET environment` → `42501` | a join that re-points a release at another environment, or a deleted one |
| `TestReleaseGuardCoversEveryColumn` | reads **the installed function**, not a copy: `SELECT pg_get_functiondef('hub.guard_release'::regproc)`. From that text it takes every `( NEW.a, NEW.b, … ) IS DISTINCT FROM ( OLD.a, OLD.b, … )` pair and expects exactly two. The first is the immutable tuple and the second the mutable one. In each pair the `NEW` list equals the `OLD` list, name for name and in order. `NEW.environment IS DISTINCT FROM OLD.environment` appears exactly once, outside both. Then, against `information_schema.columns` for `hub.releases`, every column appears **exactly once** across immutable, mutable and `{environment}`, and no name appears there that is not a column. Last, mutable ∪ `{environment}` = `hub_app`'s `UPDATE` columns (`information_schema.column_privileges`) | a column `x` added to `releases`, and to any Go list, but not to the trigger's tuple: the test goes red on the missing column, where a copied list would stay green. `x` would otherwise stay mutable after confirmation for the owner role, which bypasses the grant. The old `NEW IS DISTINCT FROM OLD` caught this; the tuples do not. It also kills a `NEW`/`OLD` mismatch (`NEW.pin_brief_digest` beside `OLD.pin_brief_id`) |
| `TestTheAuditChainIsGaplessAndHashed` | insert seq 1 then 2 correctly → ok and `audit_heads` advanced; seq 3 with the wrong `prev_hash`, or seq 4 skipping 3 → `40001`; a `hash` not equal to `sha256(prev_hash ‖ canonical)` → `23514` | a chain the Go side can fork (G5) |
| `TestResolvePrincipal` | an unknown tenant → no row; a new oid → `created = true`; again → `created = false`, same id; a changed name → refreshed | provisioning twice, or never refreshing the display name |
| `TestEnsureOrgIsIdempotent` | twice with two fresh ids → the first id both times; one `audit_heads` row | a second org row per restart |
| `TestEraseBriefChecksTheOrg` | with `hub.org_id` = B, erasing A's brief → `P0002` (`no_data_found`), and A's revisions keep their content | the owner function bypassing RLS without its own check (T3's residual) |
| `TestEraseBriefLeavesEveryStoredResponse` | org A, one project, briefs A1 and A2. Four `idempotency_keys` rows, each byte-copied before the call: r1 is route 26's stored form of a batch that wrote A1 **and** A2 together (every `content` null, as task 19 stores it); r2 is a stored route-30 `DesignApproval` body naming `brief:<A1>@1` (text-free); r3's `response` is `NULL` (in flight); r4 is **in org B**, and its response contains A1's id. `erase_brief(A1)` with `hub.org_id` = A → all four rows kept, byte for byte (review I6, re-review b06 N1) | a purge by the brief id, which deletes r1 and r2: A2's co-batched retry becomes 409 `brief_archived` and the approval's retry re-runs into 422 `key_unresolvable`; a purge by `path`, which deletes r1; a purge without `org_id = v_org`, which also deletes r4 |
| `TestAuditAppendChecksTheOrgFirst` | orgs A and B; B's head read beforehand as the owner. Connection 2 goes **first**: as `hub_app`, `BEGIN`, `set_config('hub.org_id', B, true)`, `SELECT * FROM hub.lock_audit_head()`, and it holds that lock. Connection 1, as `hub_app`: `BEGIN`, `set_config('hub.org_id', A, true)`, `SET LOCAL lock_timeout = '2s'`, `INSERT` an event with `org_id` = B, correctly chained on B's head → `42501` **and** a message containing `not this session's`, returned at once. Both roll back; B's head is unchanged (review M11, A19; re-review b07-1 N1) | an `audit_append` without its org check: running as the owner, it bypasses row-level security, reaches `FOR UPDATE` on B's head, blocks behind connection 2, and returns `55P03` (`lock_not_available`) after the timeout. The message assertion matters: a `WITH CHECK` refusal is also `42501`, but its message is `new row violates row-level security policy`, so `Code` alone cannot tell the check from the policy |
| `TestTheHeadLockNeedsTheFunction` | as `hub_app` with `hub.org_id` = A, three transactions on one connection. First, in its own `BEGIN … ROLLBACK`: `SELECT seq, hash FROM hub.audit_heads WHERE org_id = A FOR UPDATE` → `42501` (no `UPDATE` privilege). Then a fresh `BEGIN`: `SELECT seq, hash FROM hub.lock_audit_head()` → A's head; while that transaction is open, a second connection as the owner runs the same row `FOR UPDATE NOWAIT` → `55P03`; the first `COMMIT`s, and the second's `NOWAIT` → ok. Last, a fresh transaction with `hub.org_id` unset: `lock_audit_head()` → `P0002`. One transaction for all three would fail the second statement `25P02` (aborted), not return A's head (review M11, A19; re-review b07-1 m1) | a lock that no longer locks: `lock_audit_head` without `FOR UPDATE`, so two writers read one head and one gets `40001` from the trigger instead of waiting its turn |
| `TestOrgIdsListsEveryOrgAndOnlyIds` | orgs A and B; as `hub_app` with `hub.org_id` unset: `SELECT id FROM hub.orgs` → zero rows; `SELECT * FROM hub.org_ids()` → exactly A's and B's ids, in id order, and the result has one column (review M10, A18) | an `org_ids` that returns `display_name` or `entra_tenant_id`; the janitor or `hub verify` reading `orgs` directly and walking no org at all |
| `TestHubAppExecutesExactlyTheOwnerFunctions` | `information_schema.role_routine_grants` for `hub_app` in schema `hub` = `ensure_org`, `resolve_principal`, `erase_brief`, `lock_audit_head`, `org_ids`, and no other; `PUBLIC` holds `EXECUTE` on none of them | an owner function left executable by `PUBLIC` (Postgres's default on `CREATE FUNCTION`), or a sixth one nobody reviewed |

#### Steps

- [ ] **Step 1:** Re-read contract §10.2's guard functions, §10.4 and §10.6. A17, A18 and A19 are
  already in the contract (task 44a). A contract whose `audit_append` is not `SECURITY DEFINER`,
  whose §10.5 still grants `UPDATE` on `audit_heads`, or whose `erase_brief` deletes from
  `idempotency_keys`, is stale, and this SQL wins.
- [ ] **Step 2:** The tests first. Run them and see them fail: the functions do not exist
  (`42883`), and a guard-less `UPDATE` succeeds. Then append the SQL above to `0001_init.sql`.
- [ ] **Step 3:** Run against the task-06 container. Fix the SQL until every test passes. **Any
  change to the SQL is recorded in the commit message**, and in the contract by task 44b (a build
  departure).
- [ ] **Step 4:** Commit by pathspec, the two files.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/migrate/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 38 (10 from task 06 + 10 from 07a + 18)
go build ./control-plane/...                                                                               # ok
grep -cE "^LANGUAGE .*SECURITY DEFINER" control-plane/internal/hub/migrate/sql/0001_init.sql               # 6 (five owner functions and audit_append)
```

#### What this task does not claim

- That the owner role cannot rewrite history. It can, by dropping a constraint or disabling a
  trigger. `hub verify` (tasks 20, 27) catches that afterwards (contract T5).
- Performance of the per-org audit lock under load (ADR-H1 R10): measured later, not here.
- That `audit_append`'s org check fires for a correct Hub. Task 14's `Write` sets `hub.org_id`
  before any append. The check is there for a query path that does not.
- That the owner or the administrator may append without `hub.org_id`. The check reads the
  session's setting, not the role, so it binds every role: **every role, the owner included,
  sets `hub.org_id` before it appends**, and an append without it is refused `42501`. No task
  appends as the owner today (07a's seed appends as `hub_app` with `hub.org_id` set); a repair
  script or a future seed must set it too (re-review b07-1 m7).
