# 02 — Product prior art: teams working with coding agents on one project

Date: 2026-09-24. Author: research agent (workflow run, product-prior-art lane).
Scope: how shipping products let a **team** work with coding agents on **one project**, from
**separate machines**, with **shared visibility** and **agent-to-agent awareness**.

## The four capabilities being tested

The owner's ask, reduced to four tests applied to every product below:

| # | Capability | Short name |
|---|---|---|
| C1 | Share a project with a team | **Share** |
| C2 | Everyone sees each member's work, running agents and each agent's focus, live | **See** |
| C3 | Each member spins up their own agents on the same repo, from their own machine | **Spawn** |
| C4 | Every agent, whoever owns it, knows what the other agents are doing, so they coordinate | **Aware** |

## Method and evidence rules

- External claims cite a URL and the date read (all read **2026-09-24** unless stated).
- Vendor docs and changelogs are primary; blog posts and reviews are secondary and marked so.
- A claim I could not load from a primary source is marked **UNVERIFIED**.
- Network: this runs on the KPMG corporate network. **cube.computer is blocked**
  (WebFetch 302s to a KPMG intranet page; recorded in the user's memory note
  `~/.claude/auto-memory/cube-computer-teardown.md`). Other blocks are named where they occurred.
- Repo claims cite `path:line`.

## Repo context that frames the comparison

- Worktrees are local clones, work fetched back under `refs/panacea/lanes/…`
  (`docs/superpowers/planning/2026-09-23-owner-decisions.md:13`, OD-2).
- Briefs, approvals and the ledger are shared server-side from day one
  (`docs/superpowers/planning/2026-09-23-owner-decisions.md:14`, OD-3).
- Users sign in as themselves via Entra PKCE; nothing pooled
  (`docs/superpowers/planning/2026-09-23-owner-decisions.md:17`, OD-6).
- A harness transcript file may be read through a pinned adapter; nothing derives meaning from
  PTY bytes (`docs/superpowers/planning/2026-09-23-owner-decisions.md:19`, OD-8).
- The Hub (Phase H) owns identity, shared stores and teams (`docs/PROGRAM.md:59`).

These matter because most products below solve C2 and C4 with **server-hosted agents plus a
vendor-side database**; PanaceaApp must solve them with **per-user subscriptions, per-user
sandboxes and a Hub the customer's regulator can inspect**.

---

## 1. Claude Code (the harness we run) — agent teams, cross-session messaging, memory

Most relevant because PanaceaApp runs Claude Code unmodified. Its native coordination features
decide what our Hub must add and what it must not fight.

Sources: https://code.claude.com/docs/en/agent-teams ;
https://code.claude.com/docs/en/cross-session-messaging ; https://code.claude.com/docs/en/memory
(all read 2026-09-24).

**What is shared.**
- *Agent teams* (experimental, `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`): one lead session spawns
  teammates; each is a full Claude Code instance with its own context. The lead's terminal shows
  an agent panel; you open any teammate's transcript and message it directly.
- The team state is **local files**: mailbox per agent at
  `~/.claude/teams/{team-name}/inboxes/{agent-name}.json`, config at
  `~/.claude/teams/{team-name}/config.json`, tasks at `~/.claude/tasks/{team-name}/`. The docs
  state the task list "persists locally and is never uploaded".
- Limits stated verbatim: "One team per session … You can't create additional named teams or
  share a team across sessions." "No nested teams." "Lead is fixed."

**How one agent learns what others do.**
- A **shared task list** with states pending / in progress / completed and dependencies.
  "Task claiming uses file locking to prevent race conditions."
- A **mailbox** (`SendMessage`), delivered automatically; idle notifications carry the final answer.
- *Cross-session messaging* (`ListAgents` + `SendMessage`) between **your own** independent
  sessions: same machine over a per-session Unix socket or Windows named pipe; other machines
  and cloud sessions "Through Anthropic servers", over Remote Control, which needs a claude.ai
  sign-in. Not available on Bedrock, Vertex, Foundry or API-key auth.
- A message "is a piece of text … never the sender's conversation history or files". It cannot
  approve prompts or change configuration.
- *Memory*: project `CLAUDE.md` is "shared with your team through version control". Auto memory
  "is machine-local … Files are not shared across machines or cloud environments."

**Conflicts.** Advice, not mechanism: "Two teammates editing the same file leads to overwrites.
Break the work so each teammate owns a different set of files." Worktrees are suggested for
manual parallel sessions. No file lease, no merge support.

**Identity and billing.** Every teammate runs under the lead's own auth and bills to it; the
docs warn teams "use significantly more tokens". Cross-machine messaging is scoped to **one
person's** account: "your sessions on other machines". There is **no cross-user team.**

**Enterprise objections.**
- Experimental, off by default; session resumption does not restore in-process teammates.
- Cross-machine messages transit Anthropic servers and need consumer-style claude.ai sign-in.
- One user only; nothing lets Alice's agent message Bob's.
- Coordination state sits in dot-files in a home directory: no audit, no RBAC, no retention.

**What this means for us.** Claude Code gives us a good **intra-user** protocol (task list with
claims, mailbox, idle notice) and explicitly refuses the **inter-user** case. That gap is our
product. We should copy the *shape* (tasks with claim + dependency, plain-text messages that
carry no authority) into the Hub, and inject it into each harness through MCP, rather than
depending on the experimental team files. Reading `~/.claude/teams/*` would fall under OD-8's
pinned-adapter rule (`docs/superpowers/planning/2026-09-23-owner-decisions.md:19`) and is fragile.

---

## 2. Devin (Cognition) — org sessions, subscribers, managed Devins, knowledge

Sources (read 2026-09-24): https://docs.devin.ai/work-with-devin/advanced-capabilities ;
https://docs.devin.ai/product-guides/knowledge ; https://docs.devin.ai/release-notes/2026 ;
https://devin.ai/pricing ; https://docs.devin.ai/integrations/slack .

**What is shared.**
- Sessions live in an **organization** in Devin's cloud. Release note of 3 June 2026:
  "Session Subscribers" lets users "Follow and view others' sessions", and the sidebar groups
  by subscription. This is the closest shipped match to our C2 ("see what my teammate's agents
  are doing").
- Sessions can be shared to Slack threads; Slack can start and steer sessions.
- 21 Sept 2026: "Nested Sub-Devin Sessions" render as a tree in the sidebar.

**How one agent learns what others do.**
- **Coordinator pattern** ("managed Devins"): a coordinator session can "spin up managed Devins",
  "message child sessions", monitor resource use, and "put child sessions to sleep or terminate
  them". Each child runs "in its own isolated VM". The coordinator "scopes the work, monitors
  progress, resolves conflicts, and compiles results."
- **Organization knowledge**: items scoped to the org are "visible to all members"; Devin
  "retrieves Knowledge when relevant, not all at once"; it "will automatically suggest Knowledge
  to remember based on your feedback". Pinnable per repo. Since 18 Sept 2026 knowledge is
  "Moving to Skills" inside org/enterprise/personal plugins.
- **Peer sessions started by different people do not know about each other.** Awareness flows
  only down the coordinator tree. (No doc describes cross-tree awareness; absence is not proof,
  so: UNVERIFIED that none exists.)

**Conflicts.** Delegated to the coordinator's judgement; the docs name no lock, claim or merge
mechanism. Each session opens its own PR.

**Identity and billing.** Seats plus metered **ACUs** (Agent Compute Units). Team plan is
"$80/month + $40/month per full developer seat", "Up to 200 users"; Enterprise adds
"SAML/OIDC SSO" and a "Dedicated deployment option" (pricing page). ACUs are pooled at org
level and budgeted per org/session. Since June 2026 "personal automations" run "under your own
identity". Aug 2026: "Devin Local On by Default for Enterprise".

**Enterprise objections.**
- Agent loop runs in Cognition's cloud even when execution is on customer machines.
- Pooled org credit: one heavy user drains the team's ACUs; cost attribution is per session.
- Coordination is top-down only; two engineers' independent Devins can still collide.
- Knowledge auto-suggested from chat is a governance risk (who approved this rule?).

**What this means for us.** Copy "subscribe to a teammate's session" as a first-class verb, and
org/repo-scoped knowledge with a suggestion queue that a human approves. Do not copy pooled
compute: it contradicts our never-pool rule.

---

## 3. Cursor — cloud agents (formerly background agents), team rules, worktrees

Sources (read 2026-09-24): https://cursor.com/docs/cloud-agent ; https://cursor.com/pricing ;
https://cursor.com/changelog/2-0 (via search summary) ; https://cursor.com/docs/configuration/worktrees
(via search summary) ; https://forum.cursor.com/t/collaborate-with-the-team-on-agents/165248 .

**What is shared.**
- A cloud agent run has a URL. "Send an agent's URL to a teammate and they can open the run to
  see the conversation, the code changes, and the artifacts the agent produced." "Viewing is
  read-only." An admin can enable **team follow-ups** so teammates can message the agent.
- The teammate must be in the same Cursor team **and** have connected their own source-control
  account with access to the repo: visibility is gated by git permissions, a good rule.
- **Team Rules / commands / hooks** are set in the web dashboard and "automatically applied to all
  members of your team", with an admin audit log of Team Rule edits (changelog 2.0, search summary).
- Forum thread of 9–13 July 2026: users still found teammates could not reliably open each
  other's agents, even with follow-ups on. Sharing is per link, not a live team roster.

**How one agent learns what others do.** Nothing in the docs. Up to eight agents run in parallel
on one prompt, each "in its own isolated copy of your codebase" (git worktree or remote VM). They
share rules, not state. (Absence in docs: UNVERIFIED that no mechanism exists.)

**Conflicts.** Avoided by isolation: separate worktree or VM, separate branch, a PR at the end.
Merging is the human's PR review.

**Identity and billing.** Seats: Teams "$40/user/month" with "SAML/OIDC SSO" and "Team-wide
privacy mode"; Enterprise adds "Pooled usage", "SCIM", "Audit logs and service accounts".
"Cloud Agents are charged at API pricing for the selected model." Agents are started from IDE,
web, iOS, Slack `@cursor`, GitHub/Bitbucket comments, Linear, or API. A "Self-Hosted Machines"
option runs tool execution on customer hardware.

**Enterprise objections.**
- Code is cloned into Cursor-hosted VMs by default; the model loop is Cursor's.
- Pooled usage at Enterprise means one budget, weak per-person attribution.
- Sharing is a link, so there is no "who on my team is running what, right now" roster.

**What this means for us.** Copy two things: the **git-permission gate** on viewing a teammate's
agent, and **admin-set team rules** that every agent loads. The per-link share is too weak for C2.

---

## 4. GitHub — Copilot cloud (coding) agent, Agent HQ / mission control, Copilot Memory

Sources (read 2026-09-24):
https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-coding-agent ;
https://github.blog/ai-and-ml/github-copilot/how-to-orchestrate-agents-using-mission-control/
(posted 1 Dec 2025) ; https://github.blog/changelog/2026-02-04-claude-and-codex-are-now-available-in-public-preview-on-github/
(search summary) ; https://docs.github.com/en/copilot/concepts/agents/copilot-memory (search summary).

**What is shared.**
- The **issue and the pull request** are the shared surface. Assign an issue to Copilot, Claude,
  Codex "or all three" from the Assignees dropdown; each opens a draft PR (Feb 2026 changelog).
- Session logs are viewable on GitHub.com; "every step happening in a commit and being viewable
  in logs". Anyone who can read the repo sees the issue assignee, the PR and its commits, so
  **"who is working on what" is the issue board**. This is repo-wide by construction.
- **Mission control** is one view to assign tasks "across repos", "watch real-time session logs",
  "steer mid-run (pause, refine, or restart)" (blog). It is framed as *your* fleet; a team-wide
  roster is not described (UNVERIFIED either way).

**How one agent learns what others do.**
- **Copilot Memory**, the strongest shipped C4 mechanism found. Repository facts are "stored with
  citations pointing to the code that supports them"; before use the agent "checks those
  citations against the current branch"; "Only validated facts are used". Memories are "shared
  across agents" (cloud agent, code review, CLI). They can be created only by actions of
  "contributors with write permissions" and used only in tasks "initiated by users with read
  permissions". Unused entries are "automatically deleted after 28 days".
- Live peer awareness: none. The mission-control post says: "Agents working in parallel can
  create merge conflicts if they touch the same files. Be thoughtful about partitioning work."

**Conflicts.** Human partitioning plus hard limits: the agent "can only make changes in the
repository specified", "can only work on one branch at a time and can open exactly one pull
request" per task, sessions max "59 minutes". Branch protection applies; Copilot may need to be
a bypass actor.

**Identity and billing.** Runs in GitHub Actions; consumes "GitHub Actions minutes and AI
credits" under the org's Copilot plan. The session is attributed to the human who assigned it.
Third-party agents need Copilot Pro+ or Enterprise (search summary of Feb 2026 changelog).

**Enterprise objections.**
- GitHub only; our OD-4 needs Azure DevOps too (`docs/superpowers/planning/2026-09-23-owner-decisions.md:15`).
- Code runs in GitHub-hosted Actions runners unless self-hosted runners are configured.
- Awareness is asynchronous (issue, PR, memory); two agents can still edit the same file for 59
  minutes before anyone knows.

**What this means for us.** Copy the **issue-as-claim** idea (an agent's task is a visible,
assigned work item) and, above all, **citation-validated, permission-scoped, expiring memory**.
It solves "agents understand each other" without trusting an agent's prose.

---

## 5. OpenAI Codex — cloud tasks, workspace environments

Sources (read 2026-09-24): https://learn.chatgpt.com/docs/cloud (308 redirect from
developers.openai.com/codex/cloud) ; https://learn.chatgpt.com/docs/enterprise/admin-setup ;
https://help.openai.com/en/articles/11369540-using-codex-with-your-chatgpt-plan (search summary).

**What is shared.** A ChatGPT Business/Enterprise **workspace**. Cloud environments (dependencies,
setup steps, env vars, internet access) are configured per repo; the fetched summaries disagree
on whether every member sees every environment ("shared settings" vs "environments aren't
automatically shared") — **UNVERIFIED**. Tasks can be started "from the web, GitHub, GitLab,
Linear, or Slack". Scheduled cloud tasks can be shared "when scheduled task sharing is available".
A per-task live view for teammates is not documented (UNVERIFIED).

**How one agent learns what others do.** `AGENTS.md` in the repo, which is shared through git.
Nothing live. "Run tasks in parallel" and "compare several attempts" is best-of-N for one person,
not coordination.

**Conflicts.** Each task yields a diff; you "inspect the summary and diff" before opening a PR.
Merge is human.

**Identity and billing.** Seats in a ChatGPT workspace; "Codex activity consumes eligible ChatGPT
workspace credits" — pooled vs per-seat not stated (UNVERIFIED). Separate admin boundaries for
local runtime, cloud, API and plugins; a "Compliance API for audit and investigation records";
source-system permissions are respected ("workspace access doesn't bypass those controls").

**Enterprise objections.** Code and agent loop in OpenAI's cloud; GitHub/GitLab only (no Azure
DevOps in the list); no team-level live view.

**What this means for us.** Copy the **split admin boundaries** (local runtime vs cloud vs API) and
the rule that platform access never bypasses the git host's own permissions.

---

## 6. Amp (Sourcegraph / Amp Inc.) — workspace threads, read/find threads, multiplayer orbs

The closest shipped answer to C2 + C4 together.

Sources (read 2026-09-24): https://ampcode.com/docs/orbs/multiplayer ; search summaries of
https://ampcode.com/news/read-threads , https://ampcode.com/news/find-threads and
https://ampcode.com/news/end-of-public-threads (both news pages failed to load in WebFetch with
"Parse Error: Header overflow", so their wording here is from search snippets: treat as
secondary).

**What is shared.**
- **Threads** (an agent conversation) belong to a **workspace**. Visibility: workspace, unlisted
  (unguessable URL) or private; public threads were removed. Workspace admins set the default;
  the post argues "Shared by default is the right default for workspaces" because it
  "makes collaboration feel more like sharing Google Docs edit history or Git branches".
- **Multiplayer** on a thread: "workspace teammates can write prompts in your thread and access
  the thread's orb or shared runner, including portals, the terminal, and the file browser." An
  **orb** is a hosted agent environment. Multiplayer lasts "7 days by default".
- Multiplayer is refused on personal runners because that "would give your teammates access to a
  machine you never shared" — the right instinct.

**How one agent learns what others do.** Two agent tools:
- `find_thread`: search "your past threads and your workspace members' threads … by keyword,
  file path, repository, author, date, or task".
- `read_thread`: fetches a thread as Markdown and "uses another model to extract the relevant
  context based on your instructions" (a subagent).
This is **pull-based awareness**: an agent can ask "has anyone on my team touched `auth/`
this week, and what did they decide?" It is not live and not push: a running agent is not told
that a colleague's agent just started on the same file.

**Conflicts.** None documented for multiplayer (two humans prompting one thread). Across
threads: normal git.

**Identity and billing.** "All agent and orb costs are billed to the thread owner", whoever
prompts. That keeps cost with one person, but a teammate spends the owner's budget by typing.
Workspace SSO / retention terms: UNVERIFIED (not loaded).

**Enterprise objections.**
- "While multiplayer is active, every workspace member can access the thread, its orb or shared
  runner, secrets, files, and terminal" — all-or-nothing, no per-member grant.
- Threads (full transcripts, possibly with secrets) are searchable workspace-wide by default.
- A teammate prompting on the owner's bill breaks per-user accountability, and would breach our
  never-pool rule if the owner's capacity were a Claude subscription.

**What this means for us.** Copy **find/read over teammates' transcripts as agent tools** (MCP),
with a model-side extractor so the caller's context is not flooded. Reject owner-pays
multiplayer: a teammate who wants to steer should fork into their own lane on their own
subscription.

---

## 7. Factory (Droids) — org memory, droid computers

Sources (read 2026-09-24): https://factory.com/news/droid-computers (307 from factory.ai,
published 22 Apr 2026) ; https://docs.factory.ai/guides/power-user/memory-management ; search
summary of https://factory.ai/news/context-window-problem .

**What is shared.** Project instructions in git ("Project instructions belong in git so every
teammate and automation session gets the same operating context"), layered with personal
`~/.factory/` files. **Droid Computers** are "persistent machines for remotely orchestrating
Droids"; the post names no team sharing or session visibility.

**How one agent learns what others do.** Factory describes "hierarchical memory" with **User
Memory** and **Org Memory** that "encodes the shared institutional knowledge" (search summary of
the context-window post; the page itself was not loaded: UNVERIFIED wording). Multi-agent
decomposition ("racing different solution approaches, and cross-verifying critical changes") is
described as something Factory is "exploring", not shipped.

**Conflicts.** Not documented.

**Identity and billing.** Bring-your-own-machine is "Free for all users"; managed computers on
Max, Growth and Enterprise plans are "charged while they're awake".

**Enterprise objections.** Team visibility is not a shipped feature in anything read; org memory
governance (who approves an org fact) is not described in the docs loaded.

**Adjacent research worth reading.** Dhanyamraju and Raghav, *Shared Organizational Memory for
Enterprise Coding Agents: System Design and Deployment Snapshot*, arXiv 2608.00122, submitted
31 Jul 2026 (https://arxiv.org/abs/2608.00122). Not a Factory paper (a search engine conflated
them). Its lifecycle — capture "with contributor approval", curate into Q&A memories, gate for
privacy/security risk, retrieve for future agents — is the governance shape we want. It reports
effects "remain under evaluation": no results yet.

---

## 8. Replit Agent 4 — one shared project, shared Kanban, agent-assisted merge

Sources (read 2026-09-24): https://replit.com/blog/whats-changed-agent3-to-agent4 (posted
19 Mar 2026) ; https://docs.replit.com/build/invite-teammates ; search summary of
https://replit.com/blog/live-from-hq-agent4-launch-pt2 .

**What is shared.** "Everyone works in the same project. There's no forking into separate
environments." Each person starts their own agent threads. Everyone sees "cursor presence and
which files teammates are viewing", and **all tasks appear on a shared Kanban board** with
columns "Drafts, Active, Ready, and Done". This is the cleanest shipped answer to "what is my
teammate's agent focused on": the task card *is* the focus.

**How one agent learns what others do.** Through the board and through merge: each task runs in
"isolated environments—exact copies of your current project" and is applied back. Whether a
running agent reads other cards before starting is not documented (UNVERIFIED).

**Conflicts.** "When a task is ready to merge, the Agent assists with the process, with no manual
conflict resolution required"; the system "flags the conflict" when two tasks touch the same
files. A launch write-up claims AI resolves merge conflicts "automatically 90% of the time"
(search summary; UNVERIFIED, vendor claim, no method given).

**Identity and billing.** Seats per plan (Core 5, Pro 15 collaborators); parallel tasks Core 1,
Pro 10. Whose credits a teammate's agent spends is not documented (UNVERIFIED).

**Enterprise objections.** Everything runs in Replit's cloud on Replit's own project model, not
the customer's git host; agent-auto-merge of conflicts without human review is a control gap in
regulated change management.

**What this means for us.** Copy the **shared board of agent tasks with a fixed small set of
states** as the team's C2 surface, and **flag file overlap at plan time, not at merge time**.
Do not copy silent auto-resolution of conflicts.

---

## 9. Zed — channels, shared agent threads, and Delta (DeltaDB)

Network note: **zed.dev/blog is blocked on the KPMG network** (WebFetch 302 to
`kpmgaust.sharepoint.com/.../acceptable-use-policy.aspx`); zed.dev/docs returned **403**.
Evidence below is from https://delta.dev/ (loaded 2026-09-24) and search snippets of
https://zed.dev/blog/introducing-delta , https://zed.dev/blog/delta-public-beta and
https://zed.dev/docs/collaboration/channels (secondary; UNVERIFIED wording).

**What is shared.**
- Zed editor: **channels** are "persistent rooms" with shared projects, voice and notes; click an
  avatar to **follow** a teammate's cursor (docs snippet).
- **Delta** (private beta Aug 2026 per a third-party report, public beta since; dates
  UNVERIFIED): the unit is a **thread** — "a conversation with an agent paired with a repository
  checkout, kept together as one shareable artifact", synced across desktop, web and cloud
  runners. Access levels: Owner, invited only, everyone in the org, or anyone with the link.
  "When someone joins your thread, they see the same worktrees you do and can work with them on
  their own machine" (blog snippet).
- "Delta also connects to third-party agent harnesses, starting with Claude Code … your session
  syncs live into a Delta thread" (blog snippet). This is **exactly our architecture's shape**:
  an unmodified harness, mirrored into a shared, multi-machine view.

**How one agent learns what others do.** DeltaDB "records work as a stream of fine-grained
deltas. Each delta has a stable identity, so a reference anchors to a change rather than a line
number"; comments on the conversation or any line "stay anchored as the work evolves"
(delta.dev). Agent-to-agent awareness across threads: not documented (UNVERIFIED).

**Conflicts.** DeltaDB replicates "the conversation and the worktree together, in real time" and
"works with the git repository you already have … teammates who never open Delta see a normal git
repo" (blog snippet). Merge semantics are not documented on the page loaded (UNVERIFIED).

**Identity and billing.** Not found. **Enterprise objections.** Beta; hosting, residency, pricing
and self-host all undocumented; a second version-control system beside git is a hard sell to a
change-control board.

**What this means for us.** Zed is the best signal that our "mirror unmodified Claude Code into a
shared thread" bet is where the market is going. Copy **stable-identity change anchors** for
comments and provenance (fits the product spine), and **"join a thread, work on your own
machine"**. Do not replace git; stay on OD-2's clone-per-lane.

---

## 10. Windsurf, now Devin Desktop (Cognition) — Agent Command Center and Spaces

Sources (read 2026-09-24): https://devin.ai/blog/windsurf-is-now-devin-desktop (the fetch
returned "June 2, 2024", which contradicts the 2026 rebrand in search results; date
UNVERIFIED, June 2026 likely) ; search summaries of https://docs.windsurf.com/windsurf/spaces
(direct fetch timed out after 60 s) and https://devin.ai/blog/windsurf-2-0 .

**What is shared.** The **Agent Command Center** lets you "manage every local and cloud agent from
a single Kanban view", grouped by status. **Spaces** "group all of the agent sessions, PRs, files,
and context for a task or project into a single view". Whether a Space is shared with teammates
or only with one user's agents is **not clear** in what loaded; Teams setup adds members by email
or invite link (search summary) — treat team-shared Spaces as UNVERIFIED.

**How one agent learns what others do.** Space **context inheritance**: "When you create a new
session in a Space, it inherits everything the Space already knows about the project" — "the
project conventions, relevant files, decisions that were made". Third-party agents via the open
**Agent Client Protocol (ACP)** "show up in the Kanban view, run inside Spaces, and share context
with other agents" (Devin Desktop post).

**Conflicts.** Not documented in what loaded.

**Identity and billing.** Folded into Devin plans (§2): seats plus ACUs.

**Enterprise objections.** Two products mid-merge (Windsurf → Devin Desktop, Cascade → Devin
Local, Knowledge → Skills): a moving target for a procurement review.

**What this means for us.** **"A Space per piece of work that new agents inherit"** is the right
unit for C4, and ACP shows harness-neutral registration is expected. It maps onto our project +
brief + provenance thread.

---

## 11. Warp — Oz cloud agent platform (multi-harness)

Network note: **warp.dev/blog and docs.warp.dev are blocked on the KPMG network** (302 to the
KPMG acceptable-use policy page). Everything here is from search snippets of
https://www.warp.dev/oz , https://www.warp.dev/blog/multi-harness-cloud-agent-orchestration ,
https://docs.warp.dev/agent-platform/cloud-agents/managing-cloud-agents and
https://docs.warp.dev/platform/harnesses/claude-code/ (secondary; UNVERIFIED wording).

**What is shared.** "See all agents running across your team in one interface" and "join sessions
with one click to audit agents"; the management view can "sort by task originator". **Warp Drive**
holds team Workflows, Notebooks, Prompts, Rules and environment variables, split Team vs Personal.

**How one agent learns what others do.** "Persistent memories that are shared across your team's
agent fleet", described as cross-harness.

**Conflicts.** Not found.

**Identity and billing.** Oz is **multi-harness**: it spins up cloud agents that "directly use
Claude Code, Codex, and Warp Agent", with "governance, access control, and audit logs", and
workloads can run "on your own infrastructure". Anthropic credentials can be stored "as
Warp-managed secrets for managing team credentials" — if that is one shared key it is fine for
API billing, but a **shared Claude subscription** would be the pooling our rules forbid
(UNVERIFIED which Warp permits).

**Enterprise objections.** Blocked at KPMG itself; team-credential storage invites pooling.

**What this means for us.** Warp Oz is the nearest **architectural twin**: unmodified harnesses,
team-wide roster, join-to-audit, self-hosted execution. Differentiate on per-user subscription
rotation, Kata isolation and the provenance thread; copy **roster sorted by originator** and
**join-to-audit (read-only by default)**.

---

## 12. Google Jules — async cloud tasks, per-repo memory, critic

Sources (read 2026-09-24): https://jules.google/docs/changelog/ ; search summaries of
https://developers.google.com/jules/api and https://jules.google/docs/usage-limits/ .

**What is shared.** Nothing team-level found: the changelog "contains no entries regarding
team/workspace sharing, multiple users, or enterprise accounts" (as extracted). Tasks run in a
cloud VM; `jules remote new --parallel` runs up to 5 attempts of one prompt (search summary).

**How one agent learns what others do.** `AGENTS.md` in the repo root; **memory** since
30 Sep 2025: "During a task, Jules will save your preferences, nudges, and corrections", reused
in later tasks "within the same repository" — per user, not team (as worded). A **critic
agent** (8 Aug 2025) reviews changes, and a planning critic (26 Jan 2026) reviews auto-approved
plans, claimed "a 9.5% reduction in task failure rates" (vendor claim).

**Conflicts.** Each task yields a PR. GitHub issue label `jules` starts a task (26 Jun 2025).

**Identity and billing.** Per user Google account; limits in tasks/day and concurrent tasks
(enterprise tiers 15 or 60 concurrent, per search summary; UNVERIFIED).

**Enterprise objections.** No team model; GitHub only.

**What this means for us.** Copy the **critic on a plan before execution**: it is where a
cross-agent overlap check should run ("another agent already owns `auth/`").

---

## 13. Linear (and GitHub issues) as the agent task board

Sources (read 2026-09-24): https://linear.app/developers/aig ; https://linear.app/docs/coding-sessions ;
search summaries of https://linear.app/developers/agent-interaction and
https://linear.app/changelog/2025-07-30-agent-interaction-guidelines-and-sdk .

**What is shared.** The issue tracker. Assigning an issue to an agent "sets it as the
**delegate**, not the assignee—so humans maintain ownership while agents act on their behalf". An
**AgentSession** has six states: "pending, active, error, awaitingInput, complete, stale"; agents
post **Agent Activities** (reasoning steps, tool use, clarification prompts, final response).
**Coding sessions** (launched 11 Jun 2026) run Claude Code or Codex in "a managed development
sandbox"; progress "appears within the issue itself"; diffs land in a Reviews tab.

**How one agent learns what others do.** The board itself: who is delegated what, in which
state. Any agent with API access can read it. No push between agents.

**Conflicts.** Human partitioning by issue; PR review.

**Identity and billing.** Coding sessions: "Usage draws from your workspace's AI credits"
(pooled). Third-party agents (Cursor, Devin, Codex, Factory integrate here) bill on their own
vendor.

**The Agent Interaction Guidelines** are the best short statement of team etiquette found:
disclose agent identity; native platform integration; instant feedback; "Transparent Internal
State" (thinking, waiting, executing, complete); respect disengagement; "the final
responsibility should always remain with a human".

**Enterprise objections.** Linear is SaaS-only; pooled workspace credits.

**What this means for us.** Copy **delegate ≠ owner** (a human owns each agent's task) and the
**six-state session model** verbatim as our C2 status vocabulary. Integrate with Azure Boards and
GitHub Issues rather than building a tracker.

---

## 14. cube.computer — the product we cloned

Network note: **cube.computer is blocked on the KPMG network** (recorded in
`~/.claude/auto-memory/cube-computer-teardown.md`, which also names the public changelog as the
working mirror). Source used: https://github.com/collabs-inc/cube-releases/releases (read
2026-09-24; latest shown "Cube 0.14.4", 17 Sept).

**Finding.** Across all visible release notes there are **no mentions of teams, sharing,
collaboration, multiplayer, invites, shared machines, org/workspace, or agent-to-agent
messaging**. Per the teardown note, cube is one user's machine tree (cloud machine, local
machine → repo → worktree → agent sessions), billed via Stripe. cube proves the single-user
cockpit; it offers **no prior art for C1–C4**. The team layer is white space for us.

---

## Comparison table

Legend: **Y** shipped and documented · **P** partial · **N** not found · **?** unverified.
"Aware" means one agent can learn, unprompted by its human, what another agent is doing.

| Product | C1 Share | C2 See (live, team-wide) | C3 Spawn own agents, own machine | C4 Aware — mechanism | Conflicts | Identity / billing | Top enterprise objection |
|---|---|---|---|---|---|---|---|
| Claude Code teams + messaging | N (one user) | P (lead's panel, own sessions) | Y (own sessions, any machine) | Y intra-user: task list with file-locked claims + mailbox | Advice only ("own different files") | Per user; teammates bill the lead | Experimental; single-user; state in dot-files |
| Devin | Y org | Y "Session Subscribers" | P (cloud VM; Devin Local) | P coordinator → children; org knowledge/skills | Coordinator judgement | Seats + pooled ACUs; SSO | Loop in vendor cloud; pooled ACUs |
| Cursor | Y team | P per-link, read-only | Y (IDE/web/Slack/Linear) | N (shared rules only) | Worktree/VM isolation + PR | $40/seat; API-priced agents | Vendor VMs; no live roster |
| GitHub Copilot / Agent HQ | Y repo | Y via issues/PRs; mission control per user | Y (any assigner) | Y async: **Copilot Memory**, cited + validated | Human partitioning; 1 PR per task | Org Copilot plan; Actions minutes | GitHub-only; GitHub-hosted runners |
| OpenAI Codex | Y workspace | ? | Y | P `AGENTS.md` only | Diff review | Workspace credits | Vendor cloud; no ADO |
| Amp | Y workspace | Y threads shared by default; multiplayer | Y | Y pull: `find_thread` / `read_thread` over teammates' threads | None documented | **Owner pays** for all prompts | Multiplayer exposes secrets to all |
| Factory | P | N | Y (BYOM free) | P org memory (claimed) | ? | Plan-based | No team visibility found |
| Replit Agent 4 | Y project | Y shared Kanban + presence | P (Replit cloud only) | P board + merge | Agent auto-merge ("90%", unverified) | Seats; credits ? | Vendor cloud; auto-merge unreviewed |
| Zed / Delta | Y thread invite | Y thread synced live | Y "work on your own machine" | ? (anchored deltas; cross-thread ?) | DeltaDB beside git | ? | Beta; second VCS |
| Devin Desktop (Windsurf) | ? team Spaces | Y Kanban Command Center | Y local + cloud | Y Space context inheritance; ACP | ? | Devin plans | Product mid-merge |
| Warp Oz | Y team | Y team roster, join-to-audit | Y multi-harness, self-host | Y? shared fleet memory | ? | Team-stored credentials | Credential pooling risk; blocked at KPMG |
| Google Jules | N | N | Y | P repo memory (per user), critic | PR | Per Google account | No team model |
| Linear coding sessions | Y workspace | Y status on the issue | P (Linear sandbox) | P the board | Human partitioning | Pooled workspace credits | SaaS only |
| cube.computer | N | N | N | N | Worktree per agent | Stripe, per user | Single-user |

**Reading the table.** Nobody ships all four capabilities for a *cross-user* team with
*per-user* billing. The closest partial fits are:
- Amp (C2 + C4 pull);
- Warp Oz (C2 + C3 multi-harness);
- GitHub (C4 durable memory);
- Replit (C2 board).

Every product that does C4 at all does it **asynchronously**: memory, threads, a board. None
pushes "your colleague's agent just claimed this file" into a running agent. Claude Code
does that, but for one user only.

---

## The 5 ideas most worth copying (ranked)

**1. A team-wide live agent roster with a fixed, small state vocabulary.** (C2)
- *From:* Linear's six AgentSession states ("pending, active, error, awaitingInput, complete,
  stale"); Replit's shared Kanban; Warp's "sort by task originator"; Devin's "Session
  Subscribers".
- *For us:* one Hub row per agent: owner (human), machine, lane/branch, **focus** (task title +
  declared paths), state, last heartbeat.
- *Gate:* visible only to members with **read access on the git host** (Cursor's rule).
- *Follow:* a teammate's agent can be followed, and joined **read-only by default**.
- *Fit:* the "one global heartbeat clock" rule (`CLAUDE.md:46`) already fits a roster of many
  rows.

**2. Claims with leases on work and paths, checked at plan time.** (C4, push)
- *From:* Claude Code's shared task list, where "Task claiming uses file locking" and dependencies
  block claims; Jules's plan critic; Replit flagging file overlap.
- *For us:* lift this from one user's `~/.claude/tasks/` into the Hub so it works **across
  users**. An agent declares its scope when it starts. The Hub finds overlaps with live claims and
  **tells both agents**, as plain-text messages that "can't approve anything" (Claude Code's
  cross-session rule).
- *Leases:* a lease lapses when heartbeats stop.
- *Advisory, not blocking:* OD-2 clones keep every lane safe, and the claim only prevents
  wasted work.

**3. Teammates' work as a searchable agent tool.** (C4, pull)
- *From:* Amp's `find_thread` (by "keyword, file path, repository, author, date, or task") and
  `read_thread` (an extractor subagent).
- *For us:* expose them through the Hub's MCP server to every harness, e.g. `find_work` and
  `read_work(id, question)`.
- *Source text:* structured summaries produced by OD-8's pinned transcript adapter, never PTY
  bytes. Use the extractor so the caller's context is not flooded.
- *Scope:* the same permissions as the roster.

**4. Project memory that proves itself.** (C4, durable)
- *From:* Copilot Memory, where facts are "stored with citations pointing to the code", checked
  "against the current branch", created only by write-permission contributors, used only by
  read-permission users, and deleted when unused for 28 days.
- *Governance:* add Devin's suggestion queue and arXiv 2608.00122's "contributor approval" gate,
  so an agent *proposes* a memory and a human accepts it.
- *Provenance:* accepted memories become entries on the provenance thread, with the citation and
  the approver.

**5. The project brief as an inherited "space".** (C1 + C4)
- *From:* Windsurf/Devin Desktop Spaces ("a new session in a Space … inherits everything the
  Space already knows"); Cursor Team Rules "automatically applied to all members"; Linear's
  "delegate, not the assignee".
- *For us:* every agent spawned into a shared project, by any member, starts with:
  - the Define brief;
  - team rules;
  - the live roster;
  - the active claims;
  - a pointer to (3) and (4).
- *Ownership:* a named human always owns the agent's task. That is the product spine
  (provenance) doing C4's job.

## The 3 anti-patterns

**1. Someone else pays for my prompt.**
- *Seen in:* Amp bills "all agent and orb costs … to the thread owner" whoever types; Warp stores
  Anthropic credentials "as Warp-managed secrets for managing team credentials"; Devin, Cursor
  Enterprise and Linear pool workspace credit.
- *Why not:* on Claude subscriptions this is the cross-user pooling we must never do, and it
  destroys per-person attribution.
- *Do instead:* to steer a teammate's agent, **fork into your own lane on your own
  subscription**, or send its owner's agent a message it may decline.

**2. Coordination by advice, collisions found at merge, then resolved silently.**
- *Seen in:* GitHub ("Be thoughtful about partitioning work"); Claude Code ("Break the work so
  each teammate owns a different set of files"); Replit's agent merge with "no manual conflict
  resolution required".
- *Why not:* advice does not scale past one person's head. Silent AI resolution of a conflict is
  an unreviewed change, which regulated change control will reject.
- *Do instead:* detect overlap at claim time (idea 2), and route every real conflict to a named
  human in convergence (phase 3).

**3. Sharing the live machine instead of the state.**
- *Seen in:* Amp multiplayer, where "every workspace member can access the thread, its orb or
  shared runner, secrets, files, and terminal"; link-based sharing ("Anyone with the link" in
  Delta; unlisted URLs in Amp).
- *Why not:* it breaks Kata's per-user blast radius and leaks secrets.
- *Do instead:* share **state**: the roster, claims, summaries and memory. Terminal viewing stays
  read-only, per member, audited, and never by link.

---

## Gaps and what is unverified

- **Network-blocked, secondary sources only:** cube.computer (used its public GitHub changelog);
  zed.dev/blog (302 to KPMG policy) and zed.dev/docs (403); warp.dev/blog and docs.warp.dev
  (302 to KPMG policy); ampcode.com news pages (WebFetch "Header overflow"). docs.windsurf.com
  timed out.
- **Unclear:** whether Windsurf/Devin Desktop Spaces are shared across users; whether Codex cloud
  environments are visible to all workspace members; Delta's merge semantics and pricing;
  Replit's and Amp's per-user billing detail; Warp's credential model for Claude subscriptions.
- **Absence of evidence:** where the table says "N" for agent awareness, that means *not found in
  the docs loaded*, not proven absent.
- **Vendor claims not tested:** Replit's "90%" auto-resolved conflicts; Jules's "9.5%" fewer
  failures.
- **Not covered:** Claude Code "agent view" and Remote Control pages (only referenced from the
  messaging doc); Sourcegraph enterprise terms for Amp; Azure DevOps / Azure Boards agent
  integrations (no vendor found shipping an ADO-native equivalent of Linear's delegate model).

