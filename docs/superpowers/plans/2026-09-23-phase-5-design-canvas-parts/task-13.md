# Task 13 — The render bridge, and the human's selection

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 11 · **Hours:** 2

## Why

Go has no layout engine (spec §7.2). Screenshots, computed styles, a node's size and font
availability come from a **renderer**: a desktop webview that has the canvas open — the Design lens,
or the hidden renderer the desktop mounts while agents work (P5-Q, spec OQ-7). Paper's agents verify
their own work by screenshot (spec §3.4 step 5); ours do the same through this bridge. When no
renderer answers, the tool fails with `renderer-unavailable` and a sentence saying why — never a
blank image.

The human's selection is agent context (Paper's `get_selection`, spec §3.2 row 6). It is held here,
in memory, set by the editor.

## Files

- `canvas/authority/render.go`, `selection.go` (new); tests beside them
- `canvas/canvastest/renderer.go` (new) — a fake renderer that answers from fixtures

## Contracts

```go
// canvas/authority/render.go

type RenderKind string

const (
	RenderScreenshot RenderKind = "screenshot" // PNG per node
	RenderComputed   RenderKind = "computed"   // resolved CSS per node
	RenderLayout     RenderKind = "layout"     // box per node
	RenderFont       RenderKind = "font"       // one family's availability
)

// RenderRequest reaches a renderer as an SSE event on topic renderer:<canvasId> (task 20a).
type RenderRequest struct {
	ID       string     `json:"id"`       // rq-<12 hex>
	CanvasID string     `json:"canvasId"`
	Revision int        `json:"revision"` // render the document AT this revision (the head, or approved)
	Kind     RenderKind `json:"kind"`
	NodeIDs  []string   `json:"nodeIds"`  // 1–50
	Scale    int        `json:"scale"`    // 1 or 2, screenshots only
	Family   string     `json:"family"`   // font only
	Deadline string     `json:"deadline"` // RFC 3339; the renderer drops a request past it
	// Progress is set only by the approval's capture (task 21 step 4, review I5): one
	// artboard per request, so the renderer that takes it can show "3 of 12 · Billing"
	// in the approval toast (task 27's approvalProgress). nil on every agent request.
	Progress *RenderProgress `json:"progress,omitempty"`
}

type RenderProgress struct {
	Purpose string `json:"purpose"` // "approval"
	Index   int    `json:"index"`   // 1-based
	Total   int    `json:"total"`
	Name    string `json:"name"`    // the artboard's name, for the toast and the failure sentence
}

type Box struct {
	NodeID string  `json:"nodeId"`
	X      float64 `json:"x"` // CSS px, relative to the artboard's top-left
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type PNG struct {
	NodeID string `json:"nodeId"`
	Width  int    `json:"width"`  // device px, after clamping
	Height int    `json:"height"`
	Scale  int    `json:"scale"`  // the scale actually used: a 4096 px longest edge clamps it (spec §8.4)
	Data   []byte `json:"data"`   // PNG bytes; base64 in JSON
}

type Computed struct {
	NodeID string            `json:"nodeId"`
	Styles map[string]string `json:"styles"` // getComputedStyle for the dialect's properties only
}

type FontInfo struct {
	Family    string   `json:"family"`
	Available bool     `json:"available"`
	Weights   []int    `json:"weights"` // from document.fonts.check at 100…900, plus the manifest (P5-T)
	Styles    []string `json:"styles"`  // "normal", "italic"
	Variable  bool     `json:"variable"`
	Axes      []string `json:"axes"`    // from the bundled-font manifest; [] when unknown
	Source    string   `json:"source"`  // "bundled" | "system" | "missing"
}

type RenderResult struct {
	ID       string     `json:"id"`
	Error    string     `json:"error"` // "" | "not-found" | "not-rendered" | "too-large" | "failed"
	Message  string     `json:"message"`
	PNGs     []PNG      `json:"pngs"`
	Computed []Computed `json:"computed"`
	Boxes    []Box      `json:"boxes"`
	Font     *FontInfo  `json:"font"`
}

const (
	RenderTimeout      = 15 * time.Second
	MaxScreenshotEdge  = 4096
	MaxRenderNodes     = 50
	// ApprovalRenderTimeout bounds ONE artboard's 2× capture during an approval
	// (review I5). The agents' RenderTimeout serves a one-node screenshot and
	// does not grow. 30 s is provisional: task 01's S8 records the per-artboard
	// capture time at 1× and 2×, and task 13's implementer sets this to
	// max(30 s, 3 × the slowest 2× capture S8 measured), citing the S8 row.
	ApprovalRenderTimeout = 30 * time.Second
)

// The three causes of ErrRendererUnavailable (task 11). Each wraps it, so a
// caller asking only errors.Is(err, ErrRendererUnavailable) is unchanged; task
// 21's approval tells them apart, re-sends on ErrRendererDetached, and names
// the cause in its failure sentence (re-review RR-I5-1).
var (
	ErrNoRenderer       = fmt.Errorf("%w: none attached", ErrRendererUnavailable)                   // nothing attached for the canvas when the request was made
	ErrRendererDetached = fmt.Errorf("%w: the renderer detached", ErrRendererUnavailable)           // the renderer it was sent to detached with it outstanding
	ErrRenderTimeout    = fmt.Errorf("%w: unanswered at its deadline", ErrRendererUnavailable)      // no CompleteRender before r.Deadline
)

// AttachRenderer registers one renderer for one canvas. visible is true for
// the lens's renderer, false for the hidden one; Render prefers visible. The
// channel carries requests; detach removes the renderer and fails its
// outstanding requests with ErrRendererDetached. rendererID is opaque here;
// task 20a mints one per stream instance and topic
// (<window>/<es>/<canvasId>:<visible|hidden>), so no two live registrations
// share an id and one stream's detach never removes another's (review M17).
//
// Strikes (task 20's phantom rule): the Service itself detaches a renderer,
// closing its channel so task 20 sends `gone`, at its SECOND strike. A strike
// is an AGENT request (Progress nil) that reaches its deadline unanswered AND
// whose renderer held no approval request (Progress set) outstanding at any
// moment of that request's life. So an approval request's own timeout is
// never a strike (it has its own bound, and the approval stops at it), and an
// agent request that waited behind an approval capture on the renderer's one
// main thread is not one either. Answering any request resets the count to 0.
func (s *Service) AttachRenderer(canvasID, rendererID string, visible bool) (requests <-chan RenderRequest, detach func())

// Render sends a request to the preferred attached renderer and waits for
// its result, the deadline, or ctx. No renderer is ErrNoRenderer, a timeout
// ErrRenderTimeout, a detach ErrRendererDetached (all three are
// ErrRendererUnavailable). A result with Error set is returned as
// *RenderError{Code, Message}.
func (s *Service) Render(ctx context.Context, canvasID string, r RenderRequest) (RenderResult, error)

// RenderWithin is Render with timeout in place of RenderTimeout: it sets
// r.Deadline to now + timeout and waits that long. Render is
// RenderWithin(ctx, canvasID, r, RenderTimeout). A timeout outside
// (0, ApprovalRenderTimeout] is refused before anything is sent (an error,
// not a clamp), so no caller can hold a renderer longer than an approval
// does. Only task 21's approval passes ApprovalRenderTimeout (review I5).
func (s *Service) RenderWithin(ctx context.Context, canvasID string, r RenderRequest, timeout time.Duration) (RenderResult, error)

// CompleteRender delivers a renderer's result. The request must be
// outstanding AND have been sent to rendererID; anything else is ErrNotFound.
func (s *Service) CompleteRender(rendererID string, res RenderResult) error

type RenderError struct{ Code, Message string }

func (e *RenderError) Error() string

// HasRenderer is get_canvas_info's "rendererAttached".
func (s *Service) HasRenderer(canvasID string) bool
```

```go
// canvas/authority/selection.go

type Selection struct {
	NodeIDs []string `json:"nodeIds"` // in the order the human selected them
	At      string   `json:"at"`
}

// SetSelection is the editor's (human only; a session caller is refused).
// Ids that do not exist are dropped. Publishes a selection event.
func (s *Service) SetSelection(ctx context.Context, canvasID string, c Caller, nodeIDs []string) (Selection, error)

// GetSelection is get_selection. Empty when no editor has set one, or when
// the lens that set it has detached (the selection is cleared on detach of
// the visible renderer).
func (s *Service) GetSelection(canvasID string) Selection
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestRender_noRenderer` | `Render` with none attached → `ErrNoRenderer` (and `errors.Is(…, ErrRendererUnavailable)`) at once, not after the timeout | an agent waiting 15 s to learn the desktop is closed |
| `TestRender_prefersVisible` | with a hidden and a visible renderer attached, the request goes to the visible one | screenshots from a renderer without the lens's loaded fonts |
| `TestRender_fallsBackToHidden` | with only the hidden one, it answers | layout tools dead while the lens is closed (P5-Q) |
| `TestRender_timeout` (injected clock) | an unanswered request fails `ErrRenderTimeout` (and `ErrRendererUnavailable`) after 15 s; a late `CompleteRender` is `ErrNotFound` | a leaked waiter |
| `TestRender_wrongRenderer` | renderer B completing A's request → `ErrNotFound` | one window answering for another |
| `TestRender_detachFailsOutstanding` | detaching with a request in flight fails it immediately with `ErrRendererDetached` (and `ErrRendererUnavailable`), neither `ErrNoRenderer` nor `ErrRenderTimeout` | a request orphaned by a closed lens; a detach task 21 cannot tell from a timeout, so it never re-sends (re-review RR-I5-1) |
| `TestRender_strikesSkipApproval` (injected clock, one renderer) | two agent requests left unanswered to their deadlines → detached at the second, channel closed; on a fresh renderer, two approval requests (`Progress` set) left unanswered → still attached; on a third, an approval request outstanding while two agent requests expire behind it → still attached, and an agent request answered between two expiries resets the count | an approval capture, or the agent requests queued behind it, detaching the lens mid-approval (re-review RR-I5-1) |
| `TestRender_errorPassthrough` | a result with `Error: "not-rendered"` → `*RenderError` | a failed render reported as an empty success |
| `TestRender_limits` | 51 node ids, scale 3 → `ErrTooLarge` before any renderer sees it | an unbounded request |
| `TestRenderWithin_ownDeadline` (injected clock) | `RenderWithin(…, ApprovalRenderTimeout)` answered at `RenderTimeout + 5 s` → the result, and the request's `Deadline` is now + `ApprovalRenderTimeout`; unanswered → `ErrRenderTimeout` at `ApprovalRenderTimeout`, not at `RenderTimeout`; `Render` still fails at `RenderTimeout`. Every time is written from the two constants, never as a literal, so S8 resetting `ApprovalRenderTimeout` leaves the test true (re-review RR-I5-3) | an approval capture held to the agents' 15 s budget, or the agents' budget grown (review I5) |
| `TestRenderWithin_bounds` | a timeout of 0, a negative one, and `ApprovalRenderTimeout + 1ns` → an error, and no renderer saw a request | a caller holding a renderer indefinitely |
| `TestRender_progressPassesThrough` | a request with `Progress{approval, 3, 12, "Billing"}` reaches the renderer channel unchanged; an agent's request (`Render`) carries `nil` | a toast with no count, or an agent request dressed as an approval |
| `TestSelection_humanOnly` | a session caller → refused; a human sets `[a, b, missing]` → `[a, b]` | an agent writing the human's selection |
| `TestSelection_clearedOnDetach` | detaching the visible renderer clears the selection | `get_selection` reporting a selection from a closed lens |

## Steps

1. The fake renderer in `canvastest`. 2. Tests, then code. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/authority/... -race -count=1 -run 'Render|Selection'
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/authority`.

## Not claimed

- No renderer exists yet: the desktop side is task 24, the transport task 20a.
- The fidelity of a screenshot is the renderer's (task 01's S8), not this bridge's.
