### Task 61: The team land queue over the mirror

In TM1 a team member lands the phase 3 way, in her own project repository (`05-design.md` §8.3).
From TM2 a team project has **one land queue for every member's lanes**, held by Fleet and run
over the lane mirror (task 58's pod, task 59's runner): one integration line that every member's
agents converge on, each change attributed to the person whose agent made it. The queue is
ordered by task 29's `sequence` hints. Each land is still **its owner's reviewed act** (task 49's
review). The integration merge is built in the mirror, handed as a bundle to the lane owner's
`machined`, and pushed by that owner with **that owner's own token** (phase 3 task 13). Every
member's repository then takes the new head from the mirror, as a bundle (phase 3 ruling R4).

Four properties, each pinned by a test.

1. **Order is the hints, then arrival.** An entry whose lane was the *second* side of a live
   `sequence` hint waits behind the *first* side; otherwise entries land in the order their
   owners asked.
2. **Nothing lands that its review has not passed.** At the head of the queue, and again just
   before the handoff, task 49's `Review.Ready` must say yes: the owner's approval, and a second
   member's approval where the lane changes an agent-config path.
3. **Fleet never holds a push credential.** The handoff to `machined` carries a bundle and a
   ref, and no credential field exists in its type. The push is the owner's own act, on the
   owner's own path to her machine, with her own token in memory.
4. **Fleet believes a push only when the remote shows it.** The mirror re-reads the customer's
   remote with the project's read-only fetch credential (task 48) and the entry is `landed` only
   when the branch names the merge the mirror built.

Five decisions the scope left open, made here.

**(a) The merge is phase 3's, moved.** Phase 3 task 09's `Land` builds a two-parent merge with
`merge-tree --write-tree`, `commit-tree -p old -p laneTip` and task 09's trailers, then
`update-ref` with the old tip as a compare-and-swap (phase 3
`parts/task-09.md:98-132`, **UNVERIFIED until merge**: the part exists, the code does not). The
team land runs the same steps 6 to 11 **in the mirror**, over two refs the mirror already holds:
`refs/panacea/integration/<branch>` (the last remote head it fetched with the fetch credential)
and the lane's `refs/panacea/lanes/<owner-hash>/<wt-id>` (task 59). The mirror never runs step 12's
`update-ref` on a branch anyone pushes from; it writes the merge to
`refs/panacea/landq/<entry>` and exports `old..merge` as a bundle. A textual conflict refuses the
entry with `merge_conflict` and the paths, never content, exactly as task 09 refuses one.

**(b) The commit is the lane owner's.** `commit-tree` takes `GIT_AUTHOR_*` and `GIT_COMMITTER_*`
from the identity the owner's `machined` reported with her lane's bundle (task 60's export;
phase 3 task 09 step 8's `Identity`, the user's own). Name and email are bounded (`teamtext`,
task 02, 200 and 254 characters) and hygiene-checked. The trailers add one line to task 09's:
`Panacea-Owner: <Hub principal id>`, so release attribution (task 68) reads the person from the
commit, never from the git author (`04` §7.3).

**(c) The push is a click, not a queue step.** OD-22 and the global constraint 8 keep push a human
action with the owner's token, per act, in memory only. So the queue stops at `ready`, and the
owner's Desk shows *"Your lane `auth-refresh` is next to land into `main`"* (task 54's NEEDS YOU
row). Her **Land** click goes from the desktop through her own local control plane, which reads
the token from phase 3 task 13's credential store, to her own machine through phase 8's broker
(the S8.1 path she already uses), and pushes the merge with `--force-with-lease=<branch>:<old>`.
Fleet is not on that path. A `ready` entry nobody clicks for 24 hours goes back to `waiting` and
yields the head; its owner is told once.

**(d) Every member's repository takes the new head from the mirror.** Once the push is verified,
Fleet asks the mirror for one bundle per member, `<that member's last told head>..merge`, and
hands each to that member's `machined` through the broker. `machined` applies it as phase 3 task
09 step 14's `RefreshClones` does: a bundle into each lane's runner (R4.5), never a host git in
a lane. A member whose machine is asleep is told on her next start; a failure is logged per
member and never fails the land.

**(e) One land at a time per project.** The queue has at most one entry in `building`, `ready`
or `pushing`. A push rejected because the remote moved (someone pushed outside the queue) sends
the entry back to `waiting` at the head; the mirror fetches the remote and rebuilds. Two rejects
in a row stop the queue with `remote_contended`, one inform to every member, and no retry loop.
