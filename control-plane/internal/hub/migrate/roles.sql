-- control-plane/internal/hub/migrate/roles.sql — contract §10.1, verbatim.
-- Run ONCE per environment by a database administrator, in database `hub`,
-- before the first `hub migrate`. Phase 8's deploy pipeline runs it (phase 8 task 29/30).
CREATE ROLE hub_owner NOLOGIN;   -- owns every object; used only by `hub migrate`
CREATE ROLE hub_app   NOLOGIN;   -- the running Hub; DML only, and not all of it
-- Deployed (phase 8, P8-D10, Entra-only auth): grant the roles to the workload identities.
--   GRANT hub_owner TO "id-hub-migrate";
--   GRANT hub_app   TO "id-hub";
-- Local (compose, trust auth on 127.0.0.1): the roles log in directly.
--   ALTER ROLE hub_owner LOGIN;  ALTER ROLE hub_app LOGIN;
GRANT CREATE ON DATABASE hub TO hub_owner;
