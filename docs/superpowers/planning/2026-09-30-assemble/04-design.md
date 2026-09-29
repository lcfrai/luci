# 04 — Design: an agent-editable canvas. Adopt, integrate or buy instead of phase 5

Status: IN PROGRESS (skeleton written 2026-09-30; sections fill in as research lands).

Replaces, if adopted: phase 5, the canvas (31 tasks, 5 built).

## 1. The answer in one screen

_Pending._

## 2. What the capability has to do

The bar comes from the phase 5 plan's own goal
(`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md:28-62`)
and the owner's words in `OD-11`: *"pretty much exactly like paper.design with the MCP"*. Eight
tests, used for every candidate below:

| # | Test | Why it matters here |
|---|---|---|
| T1 | **Agents write through MCP, headless and in parallel** — an unmodified Claude Code session in a sandbox can create and change frames with no human tab open | Plan goal 1: agents design "while the Design lens is open, or while it is closed". A fleet is parallel by definition |
| T2 | **Humans finish the work by direct manipulation, multiplayer** — select, move, restyle, undo, with several people and agents in one file | Plan goals 2 and 3 |
| T3 | **Fidelity to code** — frames are real HTML/CSS or real components, not a drawing that someone re-types | `OD-11`: "real-HTML frames". It is why paper.design felt powerful |
| T4 | **Export into the codebase** — HTML, JSX, tokens (DTCG), PNG per screen into the worktree | Plan goal 4: the handoff bundle a Develop session starts from |
| T5 | **Runs on Azure in our environment, later in a customer's cloud, with an AU option** | `OD-5`, `OD-18`, `OD-19`, `OD-24` |
| T6 | **Signs in through Keycloak → Entra, with an audit trail** | `OD-30` |
| T7 | **Licence we can ship into a customer tenancy** | MIT/Apache/BSD/ISC/MPL-2.0 fine; AGPL/GPL/SSPL/BUSL/fair-code/Elastic restricted; commercial fine if KPMG can buy it |
| T8 | **Provenance hooks** — an API to bind a file to a brief, snapshot it at approval, and pull the approved revision | The product spine: brief → design → develop → release. Nothing off the shelf does this; the question is only how cheap the glue is |

**The untrusted-agent rule applies here too.** A design MCP that needs a *user's* credential inside
the sandbox (a Figma or Penpot token) hands an untrusted agent everything that user can see. The
answer in every "adopt" or "buy" case is the same: the credential stays in the control plane and
the sandbox reaches the design tool only through our MCP relay (phase 5 tasks 18–19, already
planned), which pins the call to the project's own file.

## 3. Candidates

_Pending._

## 4. Comparison table

_Pending._

## 5. Recommendation

_Pending._

## 6. What we still build (glue)

_Pending._

## 7. Owner calls (money or customer-visible)

_Pending._

## 8. What this study could not verify

_Pending._

## 9. Sources

_Pending._
