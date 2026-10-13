# CLOUD-085 affected task prerequisites — controlled Git/API scope

A new run can select only affected tasks while referencing an unchanged,
previously integrated prerequisite. Preparation records a task input digest
bound to its linked requirements, design, global constraints and recursively to
its dependency inputs. The reference names the original run, task, operation
and integration digest; it is neither a new worker allocation nor an invented
success. Legacy tasks without that digest do not gain reuse from an old task ID.

The read-only resolver and admission verify the prior actual integration and
frozen handoff, unchanged applicable task contract, current approved context,
clean source, original integration ancestry and unchanged prerequisite write
scope. A dirty/moved source, changed dependency or handoff, stale authority,
foreign run, nonancestor or unknown integration blocks the new allocation.
Running tasks receive the attributed original report and original integration;
they do not receive a fabricated new report or check outcome. The original run
bytes remain unchanged. A two-request preparation race has one CAS winner.

Controlled Go1.26.3/race tests used real temporary Git and registered product
HTTP fixtures. `preparation.log` passes the immutable-input test; the later
`affected.log` passes all six selected top-level `TestProjectSwarmReuse*`
tests in53.617s, including positive reopen/once-only behavior and negative
cases. Plain Go output gives no independent subtest count. Five key source
hashes before/after the affected run were stable; `source-bindings.json` binds
all nine current changed product files. Initial/root/tmp/cgroup headroom and
command scope are retained in adjacent receipts. This is a focused source
result, not a full product package, installed Docker/model, semantic quality,
Windows performance, live two-person coordination or cloud microVM run.

The old production path required every dependency in the selected new run;
this change is a new capability. No original before-fix executable failure is
claimed for these tests. The future accepted-event consumer and policy-based
candidate integration must still invoke the current typed route with exact
reviewed authority and preserve real verified output quality.
