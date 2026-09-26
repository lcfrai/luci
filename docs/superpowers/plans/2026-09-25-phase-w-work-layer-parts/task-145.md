### Task 145: the W5 migration, its guards and matrix rows: no sync principal or app writes a fact state or `done`

**Slice:** W5. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 07 (SW.3: `guard_issue_state`, the settings, the matrix file) and 143 (the seven
`ActorKind`s, the W5 wire). **Seam:** **SW.3**, extended: after this task, a W5 principal's reach
into issue state is fixed in the database, and each W5 route task (146-167) appends only its own
tables' columns and its route rows. Consumed by 146-167 and by task 168's end-to-end run.

W5 is the first slice whose writers are not all people. A tracker's integration principal, an
approved app, an API key and an outside host each write to the Hub. The design's rule is one
sentence (05 §5.4 W5, `05-design.md:1273`): *"Sync never writes a state, a fact or `done`"*, and
§1 L-250 adds that *"an app acts as a person, never as a free agent"* (`05-design.md:437`). Go
enforces both in each route; this task makes the database refuse them too, so a Go bug in any of
twenty routes cannot close an issue.

**Anchors, re-read on 2026-09-26** in the Hub worktree at `feat/hub` `3df9fab8`
(`docs/superpowers/contracts/2026-09-23-hub-contract.md`): §10.3 row-level security
(`:1904-1932`, the `org_isolation` policy on `current_setting('hub.org_id', true)`); §10.5 grants
(`:2001-2027`); §5.3's matrix columns `V M A O N X` (`:337-341`); `forbid_change()` (`:1799`); and
the process's reach, *"No Microsoft Graph, no client secret, no certificate, no storage account, no
Key Vault access in v1"* (`:2083-2085`).

**Read in this branch's parts, UNVERIFIED until merge:** task 07's guards (`task-07.md:101-245`),
its settings (`:16-31`) and matrix file (`:317-391`); task 10's replacement of
`guard_issue_state()` (`task-10.md:166-204`), the body this task replaces again; task 143's
`ActorKinds` (`task-143.md:152-166`); task 144's outside causes (`task-144.md`, *`hubapi/work.go`
⚑*); and the eleven route parts that wrote their tables for this migration (decision 1).

#### Decisions the scope left open, made here

1. **The route owner's SQL wins, and is taken as written** (task 143's decision 1, for SQL). Eleven
   parts already hold complete SQL for this file, each headed *"in task 145's NNNN_work_w5.sql"*.
   This part places each block in the file, in task order, and writes nothing over it; it writes
   only the tables no part has written, the guards and the matrix:

   | Block, in file order | Owner | Source in this plan |
   |---|---|---|
   | `work_requests`, its freeze and `erase_request` | 146 | `task-146.md:97-172` |
   | `work_intake_rule_sets`, the rule-version key on requests | 147 | `task-147.md:79-103` |
   | `work_intake_forms`, the form key on requests | 148 | `task-148.md:77-109` |
   | `work_intake_settings`, `work_release_issues`, `work_request_notices`, `work_mail_cursor` | 149 | `task-149.md:98-170` |
   | `work_tracker_connections`, `work_external_links`, `work_sync_runs`, `guard_sync_principal` | 150 | `task-150.md:90-187` |
   | `issue_comments`' external columns | 153 | `task-153.md:140-160` |
   | `work_hook_deliveries`, `project_key_history` | 154 | `task-154.md:110-139` |
   | **`work_host_hooks`, `work_outside_pr_links`, `work_outside_commit_links`** | **145** | below |
   | the linkback and autolink columns, `work_outside_pr_filings` | 155 | `task-155.md:145-178` |
   | **`work_webhooks`, `work_webhook_deliveries`** | **145** (156's columns) | below |
   | **`work_teams_bindings`, `work_teams_pairings`, `work_request_messages`** | **145** (157's columns) | below |
   | **`work_apps`, `hub.app_for_client`, `hub.find_principal`** | **145** (159's columns) | below |
   | `work_api_keys`, `hub.key_by_id` | 160 | `task-160.md:165-235` |
   | `work_mcp_connectors` | 161 | `task-161.md:439-463` |
   | `work_loops`, `work_loop_versions`, `work_loop_runs`, `work_loop_cursors` | 162 | `task-162.md:212-250` |
   | **the run-outcome columns and their freeze** | **145** (163's columns) | below |
   | **`work_team_agent_policies`** | **145** (143 decision 5) | below |
   | **the actor guards** (decisions 3-6) | **145** | below |

2. **The file is `NNNN_work_w5.sql`.** The scope says `NNNN_work_intake.sql`; nine of the eleven
   owners wrote `NNNN_work_w5.sql` and two (161, 162) wrote `_work_intake`. The majority wins, and
   `_intake` would be wrong anyway: most of W5 is not intake. `NNNN` is the next free number at
   sync time, renumbered on a clash (rule H1).
