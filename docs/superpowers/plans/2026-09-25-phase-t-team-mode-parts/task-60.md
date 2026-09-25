### Task 60: Tier-M forecasts; lane bundles into the mirror

**Slice:** TM2. **Focused time:** 2.5 h. **dependsOn:** 34 (ST.6's `lastCommit`, **not yet
written; required of task 34**), 36 (`raise` with lines, the strength order `P < H < M < S`,
`Open.Strongest`, `Open.Lines`, `dropNoise`, `task-36.md:194-241`), 59 (ST.7, `mirror.Client`,
the runner's `ingest` and `pairs`); through 59, task 58's pod and relay; and task 25's
`Forecaster`, `Signal`, `Raise`, `Open` (`task-25.md:64-202`). **Seams:** consumes ST.6 and ST.7.

Tier M is the exact answer: `git merge-tree` between two lanes of different owners, in the mirror
(`05-design.md` §8.2). This task feeds the mirror and asks it. After each commit on a team lane,
the lane owner's `machined` exports the lane as a bundle (phase 3 ruling R4: lane work crosses as
bundles) and the broker carries it into the project's mirror. Fleet then asks tier M **only for the
pairs that tier P or H already flagged**, so the work grows with activity, not with the square of
the team. The answer raises or updates a `cf-…` at tier `M`, or marks an open one *mergeable*.

**Decisions where the scope is silent, made here so nobody re-derives them.**

**(a) "Flagged" means: an open conflict for the pair, or a live path overlap tier P saw.** Task 36
drops noisy paths (lockfiles, generated files, co-edited files) **at tier P only**, and says they
still raise at H and at M (`task-36.md:21-25`). A pair whose only shared paths are noisy therefore
has no open conflict, and a rule of "open conflicts only" would never ask M about it. So a pair is
flagged when it has an open conflict **or** the two agents' current observed paths intersect in
task 35's activity table, noisy or not. Both are read at the moment of the commit, from state
that exists already (the working copy and the 45-second activity table), so no new in-memory set
is kept (global constraint: only activity and presence are in memory).

**(b) Only pairs that include the committing lane are asked.** A commit changes one lane. Every
other flagged pair's answer is unchanged by it, so a commit on lane `L` asks M for at most the
flagged pairs `(L, X)`: one per teammate agent whose work overlaps `L`'s. With `k` overlapping
agents that is `k` merge-trees, never `n²`.

**(c) What an answer does.** For a pair `(a, b)` whose answer is valid (ST.7's `Validate`) and
**current** (its `basis.a` and `basis.b` are still the two lanes' latest ingested tips; otherwise
it is dropped, since a newer commit's ask follows):
- `conflicts` → `raise(…, "M", a, b, paths, lines)` with the conflicted paths and every path
  whose `lines` are non-empty, and M's lines. Noise is **not** dropped at M. On an open conflict
  this moves `Strongest` to `M` and replaces the pair's lines with M's, which are exact on the
  merge base (task 36's step (4) merge; tier M's lines win over H's for the same path).
- `clean` with paths both sides changed → if the pair has an open conflict, its working copy is
  marked `Mergeable: true` with `MBasis`, and nothing is sent to the Hub (route 58 cannot patch
  tier or paths, `task-36.md:217-219`). If it has none, the paths are filtered through
  `dropNoise`, and any left raise at tier `P` marked `Mergeable`, so a clean textual merge on
  overlapping real paths **still yields a conflict record**, and a lockfile that merges cleanly
  still raises nothing (the scope's second test, and task 36's noise rule).
- `clean` with no shared paths → an open conflict for the pair is marked `Mergeable`; nothing else.
- `unknown` → nothing changes; the pair stays at its tier. Fleet logs the code.

**(d) Mergeable is Fleet state.** Whether two lanes merge cleanly *now* is a fact about now, and
changes with the next commit, so it lives in the working copy (the placement rule, global
constraint 1), not in the Hub record. `panacea check` shows it (*"overlap on auth/session.go;
merges cleanly as of 3f2a…"*) through `verbs.Conflict`, which gains one field (gap 2). Task 30's
`proceed-both` rule reads `Strongest`: a pair at `P` that M found mergeable stays `P`, and a pair M
found conflicting is `M`, which task 30 already treats as a `decide` (`task-36.md:237-241`).

**(e) The bytes never pass through Fleet.** Fleet sends one instruction to the broker, *bundle this
owner's lane into this project's mirror*, under the lane owner's delegation; the broker reads the
bundle from `machined` and writes it into the mirror's `/control`, copying and never decoding, as
it does for attach (task 17). Fleet receives only the mirror's `IngestResult`. So no hostile
object is ever in Fleet's memory, which holds every member's delegations (`07` I11).

**(f) One send per lane per 10 seconds.** Agents commit in bursts. A commit arriving while a send
for the same lane is in flight, or within 10 s of the last, sets a *pending* mark; the sender sends
once more when the window closes, with the newest tip. The window is a constant (`SendEvery`).

**(g) The basis is what the mirror already holds.** Each lane bundle is asked `over` the lane's
last ingested tip and the mirror's base tip, the shas Fleet recorded from earlier
`IngestResult`s, which phase 3's runner drops if the lane lacks them (`task-24.md:794-796`). So the
bundle carries the lane *since what the mirror has*, which is at most *since its merge base* once
the base is in. When the mirror answers `need-base`, Fleet asks the same owner's `machined` for a
base bundle of the project's default branch (kind `base`, task 59) over the mirror's base tip, then
resends the lane bundle once. A second `need-base` for the same lane clears Fleet's record of what
the mirror holds for the project and resends with an empty basis: the mirror lost its disk.
