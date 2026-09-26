-- control-plane/internal/hub/migrate/roles_local.sql — the compose stack's init script
-- (task 09 mounts it into /docker-entrypoint-initdb.d/). Local only: trust auth on
-- 127.0.0.1, so the roles log in directly and hold no password.
CREATE ROLE hub_owner LOGIN;
CREATE ROLE hub_app   LOGIN;
CREATE DATABASE hub;
GRANT CREATE ON DATABASE hub TO hub_owner;
-- The integration harness creates scratch databases as the admin and grants
-- CREATE on each to hub_owner itself (hubtest.New).
