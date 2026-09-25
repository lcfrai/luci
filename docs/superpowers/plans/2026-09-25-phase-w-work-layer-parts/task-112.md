### Task 112: the same verbs in solo mode through phase 3's runner (UNVERIFIED that it can host them)

Stream **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 101 (the three work verbs, their dispatch rows and their ST.1 fixtures). Uses TM-09's control
wire v1 and its fixture set (`testdata/panacea-control/v1/*.json`, Phase T plan `:506-514`, read at
`64ab2c4`), TM-16's stamping rule (*"every request on it is stamped with the pod's session id and
class before it is forwarded"*, `:584-589`), TM-18's `panacea` binary (`:605-614`), SW.10's Start
binding (task 24) and `Session.IssueRef` (task 19); all Phase T paths are **UNVERIFIED until
merge**. Phase 3's runner is `lanegit.Runner` and `lanegit.Lanes` (P3 task 24, phase 3 plan
`:120-123` and `:598`, read at `8fce56a`): the `sandboxd` git runner's `/git/v1/` RPC on the
attach token.

04 §4.5 says why this task exists: *"Solo mode has no `panacea` channel yet … Until the control
plane serves the same verbs through phase 3's `sandboxd` runner, solo issues get observed facts and
human text only."* This task serves them, or records that it cannot.

**Decisions made here, so nobody re-derives them:**

1. **A spike comes first and can stop the task.** A solo session runs either on the host (a PTY
   the control plane starts on the person's machine) or in a local sandbox under phase 3's runner.
   A loopback endpoint on the host is reachable from a host session; from inside a container,
   `127.0.0.1` is the container's own loopback, so the same endpoint is **not** reachable unless
   the sandbox forwards it. The spike answers one question: *can a session under phase 3's runner
   reach the control plane's verb endpoint, through `sandboxd`'s `/control` (TM-16) dialled by the
   control plane, with no second runner?* It records **yes** or **no** with the command and its
   output. On **no**, the task serves host sessions only, says so in the results, and writes no
   second runner.
2. **Three verbs and nothing else.** `issue.show`, `issue.note` and `evidence.propose`, byte for
   byte as task 101 serves them in Fleet, tested against the **same** fixture files. `team` is
   answered with `solo_project`, whose message is *"solo project: no team view"*. Every other verb
   in TM-09's table is answered `unknown_verb`: solo mode has no leases, peers or workers.
3. **The credential is the channel** (TM-16's rule). Each host session gets its own listener on
   `127.0.0.1` with an ephemeral port and a 32-byte random bearer token, passed in the session's
   environment only (`PANACEA_CONTROL_URL`, `PANACEA_CONTROL_TOKEN`). The stamp is the session
   whose token authenticated; any session id in a request body is ignored. The token is never
   written to disk, logged or returned by `sessionJSON` (phase 2's rule).
4. **Loopback, twice.** The listener binds a loopback address only, and every connection whose
   remote address is not loopback is closed before a byte is read. `PANACEA_APP_WORK_VERBS_SOCKET`
   names the bind address (default `127.0.0.1:0`); a non-loopback value fails startup.
5. **The Start-bound session is the issue's one voice.** With no Orchestrator in solo mode, a
   session created by Start (SW.10) with `Session.IssueRef` naming issue X may `note` and `propose`
   on X only. Any other session, including a plain session the person opened by hand, gets
   `class_forbidden`. `issue.show` is allowed from any Start-bound session, for any issue the
   person may read.
6. **Written as the person's own agent.** The control plane writes through task 99's
   `AddAgentComment` with the **person's** Hub token and the attribution `{ownerId: the person,
   agentId: the session id, model, sessionId}`, class `text·A`, `via: "local"`. Task 100's guard
   requires a non-null `via` on an agent-kind comment and was written for Fleet; whether it
   accepts `via: "local"` from a person's token is **UNVERIFIED** (gap 2), and this part never
   writes the comment as the person's own `text·H`.
7. **The binary is TM-18's, unchanged in meaning.** The control plane puts the directory holding
   `panacea` first on the session's `PATH`. TM-18's binary dials a Unix socket
   (`/run/panacea/control.sock`, TM-16); on a host session it must dial `PANACEA_CONTROL_URL`
   instead, one added transport in `sandboxd/internal/panaceacli` (a delta, gap 3), and a Windows
   build, because TM-18 builds a static Linux binary (`CGO_ENABLED=0`, P8-D8).
8. **No CLI is parsed or modified** (`OD-8`). The harness calls `panacea` because SW.7's managed
   block asks it to; nothing reads the harness's terminal to find out whether it did.

**Files:**
- `control-plane/internal/workverbs/serve.go` (+ `_test.go`) — the listener per session and the
  three-verb dispatcher (new; not in the file map)
- `control-plane/internal/workverbs/stamp.go` (+ `_test.go`) — tokens, the stamp, the loopback
  check (new; not in the file map)
- `control-plane/internal/api/workverbs.go` (+ `_test.go`) — the session hook: allocate on a
  Start-bound session, inject the environment, release on exit (new; not in the file map)
- `control-plane/internal/api/api.go` ⚑, `routes_audit_test.go` ⚑ — `Deps.WorkVerbs`; one audit
  row (modified)
- `control-plane/cmd/api/config.go` ⚑ — `PANACEA_APP_WORK_VERBS_SOCKET` (modified)
- `sandboxd/internal/panaceacli/transport.go` ⚑ (TM-18's package) — the URL transport (modified;
  **UNVERIFIED until merge**)
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the spike's record, under W3 (modified;
  task 113 owns the section, this task writes its one paragraph)
