### Task 02: `hubapi`: Work id prefixes, the `issue:` key, `DisplayAlias`, the `issue-revision` canonical kind and golden vectors

**Slice:** W1. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 01 (SW.0's grammar line); Hub tasks H01-H03 (`ids.Wide`, `hubapi.Canonical`,
`DigestOf`, the wire types), sync-merged from `feat/hub` into `feat/work-layer` by the orchestrator
first (rule H1). **Seams:** defines **SW.1**, first half (the id prefixes, the `issue:` key,
`DisplayAlias`, the `issue-revision` canonical form and its golden vectors). Task 03 adds the
second half; both freeze at WS0 with task 05.

An issue is on the provenance thread only if a key can name one revision of it, and that key can
be checked. This task gives `hubapi` the three things every later Work task leans on: **the ids**
the work layer mints, **the key** `issue:is-…@r` in the Hub's grammar, and **the digest** of an
issue revision, computed by the same one function the Hub uses for briefs (hub-contract §3.4: *"No
TypeScript implementation exists or may be written"*). Nothing here stores or serves an issue.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`** (paths under
`control-plane/internal/hub/hubapi/`; the scope read `48351f8`, and every anchor still holds):
`keys.go:15-27` (the `Prefix*` const block), `:31-35` (`ValidID`), `:47-52` (`KeyKind`),
`:54` (`ErrInvalidKey`), `:56-59` (`briefKeyRe`, `designKeyRe`), `:61-77` (`BriefKey`,
`ParseBriefKey`), `:96-105` (`ParseKey`), `:114` (`DigestOf`); `canonical.go:39`
(`Canonical(v any) ([]byte, error)`); `envelope.go:29-32` (`MaxEnvelopeBytes = 64 << 10`),
`:34-46` (`ErrBadEnvelope`), `:49-52` (`kindBrief`, `kindAudit`), `:54` (`bad`), `:57-66`
(`under`), `:127-191` (`BriefEnvelope`, `Canonical`, `Digest`), `:234-262` (`readEnvelope`);
`keys_test.go:683` (`constValues`), `:719` (`prefixEntity`), `:737-775`
(`TestPrefixesAndReasonsAreTheContracts`); `ids/ids.go:61` (`Wide`). Contract: §3.1
(`hub-contract:91`), §3.2 (`:114`), §3.4 (`:139`), §3.5 (`:176`). 04 §2.4 is
`04-internal-fit.md:159-190`.

Six decisions the scope left open, made here.

**(a) The Work prefixes live in `workids.go`, never in `keys.go`'s const block.**
`TestPrefixesAndReasonsAreTheContracts` (`keys_test.go:756-758`) reads every `Prefix*` constant
**of `keys.go`** and fails unless there are exactly ten, matching §3.1's ten rows. A Work prefix in
`keys.go` breaks a Hub test; one in `workids.go` does not, and task 02 edits neither §3.1 nor that
test (rule H1, additive only). Phase W's own disjointness test (below) holds the twelve.

**(b) `KeyIssue`, `issueKeyRe`, `IssueKey` and `ParseIssueKey` do go in `keys.go`**, because the
grammar is one table (hub-contract §3.2) and `ParseKey` is one `switch`. These are appended lines;
no existing line of `keys.go` changes. The `feat/hub` owner has the proposal (task 01) and the
orchestrator's merge carries it; on a conflict the Hub's file wins whole and these lines are
re-applied (rule H1).

**(c) `DisplayAlias` has no inverse.** It formats `"EP-142"`, and a test proves `ParseKey` refuses
it. No `ParseDisplayAlias` exists; `IssueByNumber(projectID, number)` (task 04) is how a person's
`EP-142` becomes an issue, with the project already known from context (05 §3.6).

**(d) The envelope is `{v, kind, issueId, revision, projectId, content}`**, six members. It carries
no `scope` or `branchRef` (an issue is project-scoped; a feature's branch is W2's `featureId` on
the head, never in a digest) and no state, priority or assignee (the head, never digested: 04
§2.4, *"reprioritising all afternoon writes no revision"*).

**(e) Title length counts code points; description length counts UTF-8 bytes.** A title is a
name a person reads (≤ 200 characters, `utf8.RuneCountInString`, the same count Postgres's
`char_length` gives, so task 06's `CHECK` refuses exactly what this does). A description is a
document (≤ 16 KiB = 16,384 bytes, `len`, Postgres's `octet_length`). A title is also 1 or more
characters after trimming spaces; a description may be empty.

**(f) Character hygiene is not this task's.** `ValidateIssueRevisionContent` checks shape and
limits. The Unicode checks (the tag block, bidi controls) run where the Hub accepts a write
(task 09) with the same `teamtext` rules Phase T's task 02 builds, if that package has landed on
the sync base, else as task 09 names. This task makes no claim about them.
