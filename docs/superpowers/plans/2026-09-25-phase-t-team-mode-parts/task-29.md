### Task 29: Resolutions `yield`, `sequence`, `proceed-both`; DECIDED WITHOUT YOU

A negotiation can decide exactly five things (§6.2's resolutions table), and TM1-α builds three of
them: `yield`, `sequence` and `proceed-both`. `split`, `counter` and `build-on` are task 62's
(TM2). When every party has accepted a proposal, **Fleet applies its typed parameters and nothing
else**. The Orchestrators' prose never has an effect (§6.1, "only typed fields have effects").
Then each Orchestrator directs its **own** workers to match (task 31); Fleet moves no worker, and
neither side can move the other's. Finally, and whether or not either Orchestrator says a word to
its human, **Fleet itself writes a DECIDED WITHOUT YOU item to both humans** from the thread's
typed state, so a conflict cannot be closed in silence by an agent that simply does not inform
(§6.2, `07-critique-security.md` I1).

Four properties, each pinned by a test.

1. **A resolution changes only what its parameters name.** `yield` releases the proposer's own
   leases on exactly the named globs; `sequence` demotes the second side's overlapping leases to
   `intent` and records one land-order hint; `proceed-both` marks the conflict `accepted` on
   exactly the named paths. Nothing else in the project moves.
2. **No resolution touches the other side.** `yield` is a promise the proposer makes about its
   own side, so it can only name globs its own side holds; an accept can never yield the
   acceptor's leases. `sequence` changes the second side's lease **mode**, which both sides
   agreed to by accepting; it issues no directive to anyone.
3. **An accept that §6.5 says needs a human does not apply.** It becomes a pending decision (task
   30 writes the `decide`, and task 30's rules decide which accepts those are). Here that is one
   hook, `Rules.Classify`, and until task 30 lands its stub already sends `proceed-both` on tier H
   or M to a decide, which is the one always-decide row TM1-α could reach.
4. **Both humans hear of every applied resolution**, from Fleet, with the typed state, in the same
   transaction that applies it.

Four decisions §6.2 left open, made here.

**(a) "Every party has accepted" is the other side's `accept`.** A thread has two participants
(task 07's route 60: "exactly 2 Orchestrators, different owners"), and proposing is accepting
one's own proposal. So the one `accept` that task 28 stores, on a live proposal by the other side,
is the moment of resolution. `Resolve` is called from task 28's `Service.Send` step 9, after the
accept commits and with the thread already updated.

**(b) A yield names whole globs its side holds.** Leases are glob lists, and "`auth/**` minus
`auth/session.go`" is not a glob. So every yielded glob must equal, as a string, a glob of a live
lease held by the proposer's side (its own workers, and its own task intents). This is checked
**at propose time** (`CheckProposal`, called from task 28's step 4) with `resolution_not_held`,
and **again at accept time**, since leases move between the two; a proposal whose globs are no
longer held when accepted is `resolution_stale` and applies nothing. A lease left with some globs
is re-issued: the old lease goes to `yielded`, a new one with the remaining globs (same holder,
mode, task) is `granted` or `contested` by task 23's own rules.

**(c) "Until the first lands" in TM1-α is "until the first side's task is done".** Phase 3's
land queue is TM1's (task 34 onward) and task 61's in TM2. Until then the only land-shaped fact
Fleet has is the first side's task reaching `done` (task 24's `task update`). So the hint is a
Fleet row `(project, conflict, first, second)`; when the first side's task is `done` or
`abandoned`, the second side's demoted leases return to their original mode. In TM1 the trigger
becomes phase 3's land event; the row and the restore are unchanged.

**(d) DECIDED WITHOUT YOU is an `inform` escalation written by Fleet.** It is task 07's route
64 with `fromAgentId: ""` (Fleet for itself, `task-07.md:123-125`), `kind: "inform"`, under each
owner's own delegation, one per human. Its `question` is a product template filled from typed
fields only (the resolution kind, the other owner's display name, globs or paths, the conflict
id), never from a message's text. §6.5's "act; Fleet informs" rows are this same item.
