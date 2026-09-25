### Task 52: Team read models and actions on the local control plane; the desktop client

**Slice:** TM1. **Size:** 3 focused hours. **dependsOn:** 21 (`fleetbe`, `TeamLauncher`), 43 (the
Hub's role-to-verb rows and team policy), 51 (the prototype's sample keys). **Seam:** defines
**ST.8**, the team read model the cockpit renders and its author-class labels, consumed by 53–56,
63 and 67.

The cockpit never talks to Fleet or the Hub directly. Phase 2's rule is that the desktop has one
server, the local control plane on loopback (`desktop/src/api/client.ts:30`,
`DEFAULT_BASE_URL = "http://127.0.0.1:8730"`), and that `desktop/src/store/types.ts` mirrors its JSON
field for field (`types.ts:3-5`: *"Renaming a field here breaks at runtime, not at compile time, so
the names are binding."*). This task keeps both rules. It adds:

1. **One Fleet route for live facts**, `GET /v1/projects/{p}/team` on `fleetd`, under task 12's
   `VerbTeamRead` (`task-12.md:272`, *"the strip and task cards"*; viewer and up). Its body,
   `verbs.TeamHuman`, is the third sibling of task 22's `TeamWorker` and `TeamOrchestrator`, built
   from the **same** `verbs.Sources` (`task-22.md:77-88`; task 22 lists 52 under "Required by",
   `task-22.md:60`), so the three views cannot disagree about who holds what.
2. **Two Fleet routes for the caller's own threads**, under `VerbThreadRead` (own only,
   `task-12.md:299`): the open-thread list as typed state, and one thread's messages. Fleet reads
   them from the Hub's route 63 as Fleet·O under the caller's own Orchestrator delegation, the
   only principal route 63 admits for a participant's owner (`task-01.md:268`), and relays them.
3. **The local routes** in `control-plane/internal/api/team.go`, which read those three Fleet
   routes through `fleetbe` and the Hub's own-only routes (66, 68, 71, 74) through task 08's
   `hubclient.Team` as the signed-in user, and **assemble** ST.8: the strip, a person's detail,
   the Desk's four row kinds, the Orchestrator conversation, a thread, and the project's team
   settings. Six actions write, each to exactly one Hub route, as the user.
4. **The desktop client**: `desktop/src/api/team.ts`, `store/slices/teamSlice.ts`, and the team
   block of `store/types.ts`, field for field, with every `text·A` field typed as such.

Four rules hold everywhere in this task:

- **No teammate's terminal, transcript, prompt, directive text, reply or conversation reaches the
  read model** (§9.2, `05-design.md:982-983`). The only agent words a teammate's row carries are
  the task title and a `blocked --on human` report, both quoted with their class (§9.1). A key
  allowlist test pins every ST.8 type (Step 1).
- **Every free-text value is a `TeamText`**: the body, its author class, the authoring agent, the
  owner, the time, and a **label assembled from typed fields** (*"Alice's Orchestrator (agent)"*,
  *"Priya's export worker (agent)"*, *"You"*). No model writes a label and no label quotes a body.
- **The owner comes from the token** (global constraint 3). No local route takes a user id in a
  body; the one path parameter that names a person (`…/people/{userId}`) selects a row to read.
- **Nothing is decided here that Fleet or the Hub decides.** The local control plane assembles and
  labels; it never filters by role (Fleet and the Hub already have), never infers a status, and
  never writes a record in anyone's name but the user's.

#### Decisions this part makes

**(a) One live read, assembled locally.** Fleet serves one `TeamHuman` per project; the strip and
every person's detail are cut from it on the local side. A second Fleet route per person would
double the Entra checks for no new fact. The strip polls it every 5 s, which matches `machined`'s
5–10 s sweep (skeleton diagram, `2026-09-25-phase-t-team-mode.md:57`); polling faster shows
nothing new.

**(b) The five-row cap is the UI's.** ST.8 carries every person with a live agent, ordered; task
53 renders five and *"+N more"*. A cap in the model would hide the sixth person from click-in.

**(c) OD-27 removes HORIZON's time.** §9.3's row reads *"…converge on `auth/session.go`, ~20
min"* (`05-design.md:992`). OD-27 (`owner-decisions.md:164`) bans ETAs, so ST.8's `HorizonItem`
has no time-to-collision field at all, only the tier, which says **how** it was predicted.

**(d) DECIDED WITHOUT YOU is found by typed fields, and Undo is a message to your own
Orchestrator.** Task 29 writes it as an `inform` escalation with `fromAgentId` absent and
`threadId` set (`task-29.md:54-58`). The local side classifies by exactly that, never by the
question's text prefix. Undo cannot touch the other side (OD-16), so it writes one `hm-…` to the
user's own Orchestrator from a fixed template naming the conflict and the resolution; that starts
a **human-origin** turn, in which the Orchestrator may renegotiate (§7.2). `undo.available` is true
only while the conflict is `accepted` and the resolution is `yield` or `sequence`; once it is
`resolved` (landed) there is nothing to reverse.
