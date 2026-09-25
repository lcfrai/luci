### Task 38: The template-only PTY writer and the delivery ladder

**Slice:** TM1. **Focused time:** 2.5 h. **dependsOn:** 19 (the control stream and dispatch), 31
(directives, piggyback as rung 1); 17 (the broker's attach relay under a delegation) and 20 (the
session's stop and resume) through them.

A directive is a `dv-…` Hub record before it is delivered (§4.3, task 31). Task 31 built rung 1:
every `panacea` call a worker makes returns its pending directives first. That reaches a **busy**
worker, which calls `panacea check` before every edit. It does not reach an **idle** one, which is
sitting at its prompt and will call nothing until something types into it. This task is the rest
of §4.3's ladder, and it owns the most dangerous line of code in the phase: the only Fleet code
that writes bytes into an agent's terminal.

The Claude Code prompt is a command line as well as a chat box: `!` runs a shell command, `/` a
slash command, `#` writes a memory, `Shift+Tab` (`ESC [ Z`) cycles permission modes, and text
typed while the agent works queues as the next user turn (§4.3, citing the interactive-mode docs
read 2026-09-25, `07` C3). A PTY write is a user keystroke with the human's authority. So:

1. **The writer's API takes a template id, never bytes.** There is no function in the package
   that accepts a `[]byte` or a `string` to write. The bytes live in one unexported table.
2. **A unit test proves it refuses every other byte sequence**, including `!`, `/`, `#`, `ESC [ Z`
   and every control byte (`07` C3), by construction and by fuzzing.
3. **No directive text, no peer text and no agent text ever crosses a PTY.** A nudge says only
   *"go and look"*; the facts arrive through the pull (§5.4). Task 31's piggyback remains the
   **only** way directive text reaches a worker (`task-31.md:18-21`).

Six decisions the scope left open, made here so nobody re-derives them.

**(a) The writer is in Fleet and writes one `Stdin` frame through the broker's attach relay.**
The skeleton puts it in `cp/internal/fleet/ptywriter`; task 09's `nudge` event says instead that
"the pod's delivery rung decides" (`task-09.md:402-406`). Task 16 queues pushed events for
`events.wait` only, and answers that verb only to the PTY's own pid (`task-16.md:137-143`), which
for a worker is the harness itself, never a caller; so a worker pod has no consumer for `nudge`.
This task follows the skeleton: Fleet dials the worker's `/attach` through the broker (task 17's
attach relay under the owner's delegation), sends **one** phase 2 `Stdin` frame (kind byte
`0x01`, `sandboxd/internal/wire/frame.go:16`) whose payload is a template, and closes. Task 09's
`nudge` stays the Orchestrator runner's signal. Listed in the gaps for task 09 to reword.

**(b) The writer never reads what the terminal says.** Attaching makes `sandboxd` send the
scrollback replay first (`sandboxd/internal/server/server.go:191`). `coder/websocket`'s
`CloseRead` would close the socket on that first data message, so the writer runs one goroutine
that reads each message into `io.Discard` and never decodes it, not even the kind byte, then
closes after its write. Nothing is logged but the byte count. OD-8 holds in the strongest form:
the bytes carry no meaning to Fleet because Fleet never looks at them.

**(c) A template ends in one carriage return, and that is the only control byte allowed.**
Typing without Enter leaves the text in the input box and the idle agent sees nothing. So each
text template is printable ASCII followed by exactly one `\r` (0x0D), and the interrupt template
is the single byte `0x03` (Ctrl+C). The lint test pins the shape: printable `0x20`–`0x7E` only,
no leading `!`, `/` or `#`, no `!` anywhere, one trailing `\r`. The design's directive template
begins with `[`, which after an `ESC` would read as a CSI sequence; no template contains `ESC`, and
decision (e) keeps the writer away from a terminal someone else is typing into, so nothing can
precede it. **UNVERIFIED:** that `\r` submits and
`0x03` interrupts in the pinned Claude Code, mid-generation and mid-tool-call; task 47's probes
settle both, and a failure changes a constant in this file, not the design.

**(d) Idle is a byte count the pod reports, never content.** Phase 2's idle is derived from bytes
crossing the relay (`control-plane/internal/registry/session.go:22-23`, and the reaper's
`LastOutputAt`, `control-plane/internal/reaper/reaper.go:650-658`). In the cloud nobody relays a
worker's terminal unless its owner is attached, so `sandboxd` itself reports
`{"event":"pty.stats","bytesOut":…,"lastOutputAt":…}` on its control connection every 10 s while
the count moves and every 60 s when it does not. Fleet keeps the latest per agent **in memory**
(activity-class state; global constraint 1). A worker is idle when `now - lastOutputAt >=
IdleAfter` (15 s). A worker with no report yet is **not** idle: the ladder fails quiet.

**(e) No nudge while the owner is attached.** A template typed into a terminal where a person is
half-way through a sentence would corrupt their input and submit it. Task 39's relay knows every
live attach; the ladder asks `Attached(agent)` and skips rung 3 while it is true. The owner sees
the directive on their Desk anyway.

**(f) One nudge per worker per 3 minutes, and a nudge is owed, not queued.** §5.4's cap
(each nudge costs the worker's owner a turn). The ladder keeps, per worker, `lastNudge` and an
`owed` flag. A directive sets `owed`; the next time the worker is idle, not attached, and 3
minutes have passed since `lastNudge`, the ladder writes **one** template and clears `owed`. Two
directives in one minute therefore cause one nudge; the worker's single `panacea check` then
drains both through rung 1. A worker that calls `panacea` for any reason clears `owed` too,
because piggyback has just delivered everything.

