### Task 22: Cockpit inference panel

A person should be able to see, at a glance and without opening anything, where the selected project's inference goes: **Global** or **AU**. One click shows why (the data class), how much has been spent against each budget, and the only action this surface offers: *Keep inference in Australia*. Nothing on it lowers a route — **and since fix batch b05 that is this surface's own rule, not a restatement of the server's.** Task 21's decision (j) lets a **solo** project's owner lower the route to the class's floor, so `PATCH {"route":"global"}` on a local project now returns `200`, not `403`. Whether a person should be offered that is owner question Q5, and while Q5 is open this panel does not draw a control the owner has not ruled on; task 23 step 6 exercises the API path by `curl` instead. If the owner answers Q5 with the design's rule, this task gains a *Move back to Global* action gated on a new server field (gap 5), never on a guess about who is asking.

**The two things this task must not get wrong.** First, the chip changes **only on the server's answer**: an optimistic flip to AU that the server then refuses would tell the person their data is staying in Australia when it is not. Second, a budget is **words and figures in `tabular-nums`**, never a bar, gauge or badge (`docs/DESIGN-SYSTEM.md`, "Designing for many live processes"; the skeleton's global constraint 8). A bar invites the eye to judge "nearly full"; the sentence says what happens when it is.

Read `docs/DESIGN-SYSTEM.md` before writing any of this. The rules that bite here: the accent (`--accent`) is for interactive and current state only, so the chip is neutral and only its focus ring and the action button use the accent; elevation is a ring plus a layered shadow (`--e-btn`, `--e-overlay`), never a flat border; motion is `transform`/`opacity` only; `prefers-reduced-motion` is honoured.

**Files:**
- `desktop/src/components/inference/RouteChip.tsx`: the top-bar chip (new)
- `desktop/src/components/inference/InferencePanel.tsx`: fetch, the chip, the popover's contents, the raise with its confirmation (new)
- `desktop/src/components/inference/format.ts`: `formatUsd`, `spendSentence`, `resetDay`, `isExhausted` (new; decision (e))
- `desktop/src/components/inference/inference.css`: tokens only, no hex, no `transition: all` (new)
- `desktop/src/components/inference/InferencePanel.test.tsx`: eleven tests, one mutant each (new)
- `desktop/src/api/client.ts`: `getInference`, `patchInference` on `CockpitClient`; `"needs_compliance"` added to `ProblemCode` (modified)
- `desktop/src/store/types.ts`: `Route`, `DataClass`, `UsageRow`, `ProjectInference`, `InferencePatch` (modified)
- `desktop/src/components/topbar/TopBar.tsx`: one element, `<InferencePanel />`, after `<LayoutToggle />` (modified)

**Interfaces:**
- Produces: `InferencePanel` (no props; reads `selection.projectId` from the store), `RouteChip`, the two client methods, the five types
- Consumes: task 21's `GET` and `PATCH /v1/projects/{projectId}/inference` and its `inferenceJSON`, field for field; phase 2's `Popover` (`desktop/src/components/common/Popover.tsx:10-55`), `useCockpit` and `Selection.projectId` (`desktop/src/store/types.ts:196-201`), `ApiError` (`desktop/src/api/client.ts:58-74`), the `api` singleton (`desktop/src/api/index.ts`), the tokens in `desktop/src/styles/tokens.css`
- Required by: task 23 (step 3 raises the project through this panel)

**Decisions where the skeleton is silent:**

**(a) The chip sits after the layout toggle, and names the project in its label.** The skeleton says "beside the project name in the top bar"; the top bar has no project name today (`desktop/src/components/topbar/TopBar.tsx:21-45` renders the layout toggle, a spacer, Authority, Run locally, New session and the picker). So the chip is the first element after `<LayoutToggle />`, its visible text is the route alone (**Global**, **AU**, or **Not classified**), and its accessible name is *"Inference for engagement-portal: Global"*. With no project selected it renders nothing.

**(b) Unclassified is its own state, with no action.** Task 21's decision (a) makes a local project unclassified until it has a data class, and its sessions then fail closed (`503 route_unknown`). The chip reads **Not classified**; the popover says *"No data class yet. Sessions on this project can't use org inference until one is set."* The raise is not offered: task 21 answers it `409`. Setting the class belongs to the create flow (ask P4-1, seam S6.8), and task 23 sets it by `curl`. Whether local mode needs a class picker here instead is gap 2.

**(c) One confirmation, inline, because the act is one-way.** The design system prefers optimistic mutation with an Undo window over confirm dialogs (`docs/DESIGN-SYSTEM.md`, "Interaction standards"). This is the case that rule does not cover: there is no undo **on this surface**, by design (OD-24). So the action expands in place into one sentence and two buttons: *"Keep this project's inference in Australia? Nothing on this panel moves it back."* with **Keep in Australia** and **Cancel**. No modal, no scrim. The sentence stops short of *"needs your organisation's compliance role"*, which fix batch b05 made untrue for a solo project (task 21 decision (j)): it states what this panel does, which is true whichever way the owner answers Q5. The wording is the owner's to accept (OD-12).

**(d) The panel owns its state; the store gains types only.** The route is fetched when the selected project changes and again each time the popover opens, and replaced by the PATCH's answer. There is no poll: a route changes only when someone acts, and the spend figure is at most 60 s old at source anyway (task 09). Nothing here adds a timer, so the one-heartbeat rule has nothing to share.

**(e) Formatting lives in one module, tested on its own terms.** US dollars as LiteLLM records them (P6-D8): spend with two decimals, a budget with none when it is whole (`US$212.40 of US$1,000 this month`). The reset day is the first of the month after `periodStart`, **in UTC** (task 06's decision (e)), formatted `1 Oct` with `en-AU` and `timeZone: "UTC"`, so a Sydney evening on 30 September still says `1 Oct`.

**(f) A row missing is said, not guessed.** Task 21 omits a scope with no current-month row (its decision (e)). The panel then says *"No spend recorded this month."* for that dimension rather than inventing `US$0.00 of` an unknown budget.

- [ ] **Step 1: Preflight**

```bash
cd desktop
test -d src/components/inference && echo "STOP: inference components exist" || echo "ok"   # expect ok
grep -c 'registerInference' ../control-plane/internal/api/inference.go                      # expect 1 (task 21 merged)
grep -n 'json:"' ../control-plane/internal/inference/wire/wire.go | grep -i -A6 'scope'     # read UsageRow's keys
grep -c '"non_loopback_publish"' src/api/client.ts                                          # expect 1
npx vitest run src/components/topbar --reporter=dot 2>&1 | tail -2                          # expect every test passing
```

The third line prints task 03's `UsageRow` JSON keys. This part mirrors them as `scope`, `id`, `periodStart`, `spendUsd`, `budgetUsd`, `requests`. If task 03 chose other names, `UsageRow` below follows task 03, and so do the fixtures in step 3.

- [ ] **Step 2: Write the contract types**

Append to `desktop/src/store/types.ts`:

```ts
/* ── inference (phase 6 task 21's inferenceJSON, field for field) ───────────────────── */

/** "au" is the tighter route. The desktop never computes one from the other: the server's
 *  effectiveRoute is the only route this app shows. */
export type Route = "global" | "au";

export interface DataClass {
  /** Proposed levels, owner question Q1; rendered verbatim, never switched on. */
  level: string;
  residency: "any" | "au-only";
}

export interface UsageRow {
  scope: "person" | "project" | "team";
  id: string;
  /** RFC 3339, 00:00 UTC on the first of the month. */
  periodStart: string;
  spendUsd: number;
  budgetUsd: number;
  requests: number;
}

export interface InferenceUser {
  id: string;
  displayName: string;
  email: string;
}

export interface ProjectInference {
  projectId: ProjectId;
  /** null: unclassified. dataClass, route and effectiveRoute are null together. */
  dataClass: DataClass | null;
  route: Route | null;
  effectiveRoute: Route | null;
  routeChangedBy: InferenceUser | null;
  routeChangedAt: string | null;
  gateway: "on" | "off";
  usageState: "ok" | "off" | "unavailable";
  /** Current UTC month only; at most one row per scope; never null. */
  usage: UsageRow[];
}

/** Exactly one key, and the route is only ever the raise. */
export type InferencePatch = { route: "au" } | { dataClass: DataClass };
```

`InferencePatch` types a route of `"au"` only. The server would accept `{"route":"global"}` on a solo project (task 21 decision (j)); the type is this surface's rule, that the desktop does not ask while owner question Q5 is open, expressed where a reviewer cannot miss it.

In `desktop/src/api/client.ts`, add `| "needs_compliance"` to `ProblemCode` after `"non_loopback_publish"` (`client.ts:45`), with the comment `// 403 phase 6: loosening a data class, or a team project's route, needs the org compliance role`. `classify` (`desktop/src/api/problems.ts:37`) needs no change: it is "server-said", and its comment's "EIGHTEEN members" becomes "NINETEEN". Add to `CockpitClient` (`client.ts:132-168`):

```ts
  /** Phase 6 task 21. */
  getInference(projectId: ProjectId): Promise<ProjectInference>;
  /** Answers the state after the write. A refusal is an ApiError, never a partial object. */
  patchInference(projectId: ProjectId, patch: InferencePatch): Promise<ProjectInference>;
```

and to the returned object (`client.ts:304-319`, beside `setWorktreeReady`), using the existing `get`, `send`, `parseJson` and `project` helpers (`client.ts:224`, `:294-295`):

```ts
    getInference: (projectId) => get<ProjectInference>(`${project(projectId)}/inference`),
    patchInference: async (projectId, patch) =>
      parseJson<ProjectInference>(await send("PATCH", `${project(projectId)}/inference`, patch)),
```

Import `ProjectInference` and `InferencePatch` from `../store/types` beside the file's existing type imports.

- [ ] **Step 3: Write the failing tests**

Eleven tests, Vitest and React Testing Library, in the shape of phase 2's `NewSessionButton.test.tsx` (`vi.hoisted` fakes, `resetCockpit`, `useCockpit.setState`). Each names the mutant it kills. The desktop has no `@testing-library/jest-dom` (`desktop/src/test/setup.ts` registers none), so every assertion is plain Vitest: a `getBy…` query that throws is the presence check.

Create `desktop/src/components/inference/InferencePanel.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import css from "./inference.css?raw";
import { InferencePanel } from "./InferencePanel";
import { formatUsd, resetDay } from "./format";
import { ApiError } from "../../api/client";
import { resetCockpit, useCockpit } from "../../store/store";
import { aProject } from "../../test/fixtures";
import type { ProjectInference, UsageRow } from "../../store/types";

const { getInference, patchInference } = vi.hoisted(() => ({
  getInference: vi.fn(),
  patchInference: vi.fn(),
}));
vi.mock("../../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../api")>()),
  api: { getInference, patchInference },
}));

const SEP = "2026-09-01T00:00:00Z";

function row(scope: UsageRow["scope"], spendUsd: number, budgetUsd: number): UsageRow {
  return { scope, id: scope + "-id", periodStart: SEP, spendUsd, budgetUsd, requests: 10 };
}

function inference(over: Partial<ProjectInference> = {}): ProjectInference {
  return {
    projectId: "p-1",
    dataClass: { level: "internal", residency: "any" },
    route: "global",
    effectiveRoute: "global",
    routeChangedBy: null,
    routeChangedAt: null,
    gateway: "on",
    usageState: "ok",
    usage: [row("person", 12.5, 200), row("project", 212.4, 1000), row("team", 400, 2000)],
    ...over,
  };
}

async function openPanel(): Promise<void> {
  await userEvent.click(await screen.findByRole("button", { name: /^Inference for/ }));
}

describe("InferencePanel", () => {
  beforeEach(() => {
    getInference.mockReset();
    patchInference.mockReset();
    resetCockpit();
    useCockpit.setState({
      projects: [aProject({ id: "p-1", name: "engagement-portal" })],
      selection: { projectId: "p-1", branchId: null, worktreeId: null, sessionId: null },
    });
  });

  // Mutant: the chip reads `route`, so an au-only project stored as global shows Global.
  it("shows the effective route, never the stored one", async () => {
    getInference.mockResolvedValue(
      inference({ dataClass: { level: "restricted", residency: "au-only" }, route: "global", effectiveRoute: "au" }),
    );
    render(<InferencePanel />);
    const chip = await screen.findByRole("button", { name: "Inference for engagement-portal: AU" });
    expect(chip.textContent).toBe("AU");
  });

  // Mutant: a fetch for projectId null, or a chip for no project.
  it("renders nothing and fetches nothing with no project selected", () => {
    useCockpit.setState({ selection: { projectId: null, branchId: null, worktreeId: null, sessionId: null } });
    const { container } = render(<InferencePanel />);
    expect(container.innerHTML).toBe("");
    expect(getInference).not.toHaveBeenCalled();
  });

  // Mutants: a one-click raise with no confirmation; a PATCH body other than {route:"au"}.
  it("raises only after the confirmation, and the chip changes on the server's answer", async () => {
    getInference.mockResolvedValue(inference());
    patchInference.mockResolvedValue(inference({ route: "au", effectiveRoute: "au" }));
    render(<InferencePanel />);
    await openPanel();
    await userEvent.click(screen.getByRole("button", { name: "Keep inference in Australia" }));
    expect(patchInference).not.toHaveBeenCalled();
    expect(screen.getByText(/Nothing on this panel moves it back/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Keep in Australia" }));
    await waitFor(() => expect(patchInference).toHaveBeenCalledWith("p-1", { route: "au" }));
    await screen.findByRole("button", { name: "Inference for engagement-portal: AU" });
  });

  // Mutant: an optimistic flip to AU that stays when the server refuses.
  it("keeps the chip on Global when the PATCH fails, and shows the server's sentence", async () => {
    getInference.mockResolvedValue(inference());
    patchInference.mockRejectedValue(
      new ApiError("needs_compliance", "Only your organisation's compliance role can loosen this project's data class.", 403),
    );
    render(<InferencePanel />);
    await openPanel();
    await userEvent.click(screen.getByRole("button", { name: "Keep inference in Australia" }));
    await userEvent.click(screen.getByRole("button", { name: "Keep in Australia" }));
    expect((await screen.findByRole("alert")).textContent).toMatch(/compliance role/);
    expect(screen.getByRole("button", { name: "Inference for engagement-portal: Global" })).toBeTruthy();
  });

  // Mutant: a two-way toggle, or any control that names Global as a destination.
  it("offers no lowering control on an AU project", async () => {
    getInference.mockResolvedValue(inference({ route: "au", effectiveRoute: "au" }));
    render(<InferencePanel />);
    await openPanel();
    const buttons = screen.getAllByRole("button").map((b) => b.textContent ?? "");
    expect(buttons.filter((t) => /global|lower|move back|australia/i.test(t))).toEqual([]);
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.getByText(/stays in Australia/)).toBeTruthy();
  });

  // Mutant: an unclassified project shown as Global, or offered a raise the server 409s.
  it("says Not classified, explains it, and offers nothing", async () => {
    getInference.mockResolvedValue(
      inference({ dataClass: null, route: null, effectiveRoute: null, usage: [] }),
    );
    render(<InferencePanel />);
    const chip = await screen.findByRole("button", { name: "Inference for engagement-portal: Not classified" });
    await userEvent.click(chip);
    expect(screen.getByText(/can't use org inference until one is set/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Keep inference in Australia" })).toBeNull();
  });

  // Mutants: figures without tabular numerals; a budget printed with cents; spend without them.
  it("writes spend as words and tabular figures", async () => {
    getInference.mockResolvedValue(inference());
    render(<InferencePanel />);
    await openPanel();
    const line = screen.getByText("US$212.40 of US$1,000 this month");
    expect(line.classList.contains("inf-figure")).toBe(true);
    expect(css).toMatch(/\.inf-figure\s*\{[^}]*font-variant-numeric:\s*var\(--numeric\)/);
    expect(formatUsd(1000)).toBe("US$1,000");
    expect(formatUsd(212.4)).toBe("US$212.40");
    expect(formatUsd(0.5)).toBe("US$0.50");
  });

  // Mutants: no sentence at the limit; the reset day computed in local time, so a
  // UTC+10 machine says 2 Oct or 30 Sep; a "reached" test that uses > instead of >=.
  it("says when a budget is reached and when it resets, in UTC", async () => {
    getInference.mockResolvedValue(inference({ usage: [row("person", 12.5, 200), row("project", 1000, 1000)] }));
    render(<InferencePanel />);
    await openPanel();
    expect(
      screen.getByText("Budget reached. New sessions on this project can't start until 1 Oct, or an admin raises it."),
    ).toBeTruthy();
    expect(resetDay("2026-09-01T00:00:00Z")).toBe("1 Oct");
    expect(resetDay("2026-12-01T00:00:00Z")).toBe("1 Jan");
  });

  // Mutant: spend drawn as US$0.00 when the gateway is off, which reads as "free".
  it("hides spend when the gateway is off, and says so", async () => {
    getInference.mockResolvedValue(inference({ gateway: "off", usageState: "off", usage: [] }));
    render(<InferencePanel />);
    await openPanel();
    expect(screen.getByText(/The inference gateway is off on this machine/)).toBeTruthy();
    expect(screen.queryByText(/US\$/)).toBeNull();
  });

  // Mutants: an unavailable figure rendered as nothing at all; a missing scope rendered as US$0.00.
  it("says when spend is unavailable, and when a scope has no row this month", async () => {
    getInference.mockResolvedValueOnce(inference({ usageState: "unavailable", usage: [] }));
    const { unmount } = render(<InferencePanel />);
    await openPanel();
    expect(screen.getByText(/Spend is unavailable right now/)).toBeTruthy();
    unmount();

    getInference.mockResolvedValue(inference({ usage: [row("project", 212.4, 1000)] }));
    render(<InferencePanel />);
    await openPanel();
    expect(screen.getAllByText("No spend recorded this month.")).toHaveLength(2); // person and team
  });

  // Mutant: a progress bar, meter or gauge for a budget (global constraint 8).
  it("draws no bar, meter or badge", async () => {
    getInference.mockResolvedValue(inference());
    const { container } = render(<InferencePanel />);
    await openPanel();
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(screen.queryByRole("meter")).toBeNull();
    expect(container.ownerDocument.querySelector("progress, meter, svg")).toBeNull();
    expect(css).not.toMatch(/transition:\s*all|#[0-9a-fA-F]{3,8}\b/);
  });
});
```

Eleven tests. The design-system checks phase 2 already runs over every stylesheet (`desktop/src/styles/designsystem.test.ts`) cover `inference.css` too if they glob `src/**/*.css`; the last assertion above repeats the two rules that matter here so this file does not depend on how that suite finds its inputs.

- [ ] **Step 4: Run the tests to verify they fail**

Run: `npx vitest run src/components/inference`

Expected: `Failed to resolve import "./InferencePanel"` (and `./format`, `./inference.css?raw`). That is the red state.

- [ ] **Step 5: Write the formatting module**

Create `desktop/src/components/inference/format.ts`:

```ts
// Words and figures for spend (decision (e)). Pure; no clock. US dollars, as LiteLLM
// records them (P6-D8); the month is the UTC calendar month (task 06, decision (e)).
import type { Route, UsageRow } from "../../store/types";

const cents = new Intl.NumberFormat("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const whole = new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 });

/** "US$212.40", "US$1,000": two decimals unless the amount is whole. */
export function formatUsd(amount: number): string {
  return "US$" + (Number.isInteger(amount) ? whole : cents).format(amount);
}

/** At or over: a key cannot be minted with nothing left (task 06's Slice). */
export function isExhausted(r: UsageRow): boolean {
  return r.spendUsd >= r.budgetUsd;
}

export function spendSentence(r: UsageRow): string {
  return `${formatUsd(r.spendUsd)} of ${formatUsd(r.budgetUsd)} this month`;
}

const day = new Intl.DateTimeFormat("en-AU", { day: "numeric", month: "short", timeZone: "UTC" });

/** The first of the month after periodStart, in UTC: "1 Oct". */
export function resetDay(periodStart: string): string {
  const s = new Date(periodStart);
  return day.format(new Date(Date.UTC(s.getUTCFullYear(), s.getUTCMonth() + 1, 1)));
}

export const SCOPE_LABEL: Record<UsageRow["scope"], string> = {
  person: "You",
  project: "This project",
  team: "Your team",
};

export const ROUTE_LABEL: Record<Route, string> = { global: "Global", au: "AU" };
```

`Intl.DateTimeFormat("en-AU", { month: "short" })` gives `Oct` in current ICU builds, and Node's full-ICU default (Node 13 onwards) is what Vitest runs on; a build that answered `Oct.` would fail the reset-day test loudly rather than ship a different string.

- [ ] **Step 6: Write the chip and the panel**

Create `desktop/src/components/inference/RouteChip.tsx`:

```tsx
// The top-bar chip (decision (a)). Neutral: the accent is for interactive state only, so
// it appears on the focus ring and nowhere else here. AU gets a shape as well as a word,
// never a colour alone (docs/DESIGN-SYSTEM.md, "Locked palette").
import type { JSX } from "react";
import type { Route } from "../../store/types";
import { ROUTE_LABEL } from "./format";

export function RouteChip(props: {
  projectName: string;
  route: Route | null;
  expanded: boolean;
  onOpen: (anchor: DOMRect) => void;
}): JSX.Element {
  const { projectName, route, expanded, onOpen } = props;
  const label = route === null ? "Not classified" : ROUTE_LABEL[route];
  return (
    <button
      type="button"
      className="top-btn inf-chip"
      data-route={route ?? "none"}
      aria-label={`Inference for ${projectName}: ${label}`}
      aria-haspopup="dialog"
      aria-expanded={expanded}
      onClick={(e) => onOpen(e.currentTarget.getBoundingClientRect())}
    >
      {label}
    </button>
  );
}
```

Create `desktop/src/components/inference/InferencePanel.tsx`:

```tsx
// Phase 6 task 22. The chip, and one popover: class, route, spend, and the one action.
import { useCallback, useEffect, useState } from "react";
import type { JSX } from "react";
import { api, ApiError } from "../../api";
import { Popover } from "../common/Popover";
import { useCockpit } from "../../store/store";
import type { ProjectInference, UsageRow } from "../../store/types";
import { ROUTE_LABEL, SCOPE_LABEL, isExhausted, resetDay, spendSentence } from "./format";
import { RouteChip } from "./RouteChip";
import "./inference.css";

const SCOPES: UsageRow["scope"][] = ["person", "project", "team"];

export function InferencePanel(): JSX.Element | null {
  const projectId = useCockpit((s) => s.selection.projectId);
  const projectName = useCockpit(
    (s) => s.projects.find((p) => p.id === s.selection.projectId)?.name ?? "this project",
  );
  const [inf, setInf] = useState<ProjectInference | null>(null);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (id: string) => {
    try {
      const next = await api.getInference(id);
      setInf((cur) => (next.projectId === id ? next : cur));
    } catch {
      // A failed read leaves what was there (nothing, on first load). The chip appears only
      // with a server answer, so it never shows a route nobody told it (P6-D5, applied to UI).
    }
  }, []);

  useEffect(() => {
    setInf(null);
    setAnchor(null);
    setConfirming(false);
    setError(null);
    if (projectId !== null) void load(projectId);
  }, [projectId, load]);

  if (projectId === null || inf === null || inf.projectId !== projectId) return null;

  const open = (a: DOMRect): void => {
    setAnchor(a);
    setConfirming(false);
    setError(null);
    void load(projectId); // fresh spend each time it opens (decision (d))
  };

  const raise = async (): Promise<void> => {
    setBusy(true);
    setError(null);
    try {
      // The chip changes only here, on the server's answer. Never before it.
      setInf(await api.patchInference(projectId, { route: "au" }));
      setConfirming(false);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "The control plane did not answer, so nothing changed.");
    } finally {
      setBusy(false);
    }
  };

  const eff = inf.effectiveRoute;
  return (
    <>
      <RouteChip projectName={projectName} route={eff} expanded={anchor !== null} onOpen={open} />
      <Popover anchor={anchor} onClose={() => setAnchor(null)}>
        <div className="inf-panel">
          <dl className="inf-facts">
            <dt>Data class</dt>
            <dd>{inf.dataClass === null ? "Not set" : `${inf.dataClass.level}, ${inf.dataClass.residency}`}</dd>
            <dt>Inference</dt>
            <dd>{eff === null ? "Not classified" : ROUTE_LABEL[eff]}</dd>
          </dl>
          {eff === null && (
            <p className="inf-note">No data class yet. Sessions on this project can't use org inference until one is set.</p>
          )}
          {eff === "au" && <p className="inf-note">This project's inference stays in Australia.</p>}
          {eff === "global" && !confirming && (
            <button type="button" className="inf-action" onClick={() => setConfirming(true)}>
              Keep inference in Australia
            </button>
          )}
          {eff === "global" && confirming && (
            <div className="inf-confirm">
              <p>Keep this project's inference in Australia? Nothing on this panel moves it back.</p>
              <button type="button" className="inf-action" disabled={busy} onClick={() => void raise()}>
                Keep in Australia
              </button>
              <button type="button" className="top-btn" disabled={busy} onClick={() => setConfirming(false)}>
                Cancel
              </button>
            </div>
          )}
          {error !== null && <p role="alert" className="inf-error">{error}</p>}
          <Spend inf={inf} />
        </div>
      </Popover>
    </>
  );
}

/** Words and figures, one line per dimension. No bar, no gauge, no badge. */
function Spend(props: { inf: ProjectInference }): JSX.Element {
  const { inf } = props;
  if (inf.gateway === "off") {
    return <p className="inf-note">The inference gateway is off on this machine, so there is no spend to show.</p>;
  }
  if (inf.usageState === "unavailable") {
    return <p className="inf-note">Spend is unavailable right now. The route above is still current.</p>;
  }
  return (
    <dl className="inf-spend">
      {SCOPES.map((scope) => {
        const r = inf.usage.find((u) => u.scope === scope);
        return (
          <div key={scope} className="inf-spend-row">
            <dt>{SCOPE_LABEL[scope]}</dt>
            {r === undefined ? (
              <dd className="inf-note">No spend recorded this month.</dd>
            ) : (
              <dd>
                <span className="inf-figure">{spendSentence(r)}</span>
                {isExhausted(r) && (
                  <span className="inf-reached">
                    Budget reached. New sessions on this project can't start until {resetDay(r.periodStart)}, or an
                    admin raises it.
                  </span>
                )}
              </dd>
            )}
          </div>
        );
      })}
    </dl>
  );
}
```

The exhausted sentence is plain text in `--ink`, not `--fail`: nothing has failed, a limit has done its job. It is set apart by position (its own line under the figure) and weight, not by colour.

The JSX in the reached sentence wraps across two source lines; the rendered text is the single sentence the test pins, because JSX collapses the line break and indentation between `an` and `admin` to one space.

Create `desktop/src/components/inference/inference.css`:

```css
/* Phase 6 task 22. Tokens only (desktop/src/styles/tokens.css); no hex, no transition: all. */

.inf-chip {
  /* Neutral chip on the top-btn base. The AU state carries a shape, a leading ring glyph
     drawn in CSS, so it never rests on colour alone. */
  border-radius: var(--r-chip);
}
.inf-chip[data-route="au"]::before {
  content: "";
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-inline-end: 6px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1.5px var(--ink-2);
  vertical-align: 1px;
}
.inf-chip:focus-visible {
  outline: none;
  box-shadow: var(--e-btn), 0 0 0 2px var(--accent-ring);
}

.inf-panel {
  display: grid;
  gap: 10px;
  width: 300px;
  padding: 12px;
}
.inf-facts,
.inf-spend {
  display: grid;
  gap: 4px;
  margin: 0;
}
.inf-facts {
  grid-template-columns: max-content 1fr;
  column-gap: 12px;
}
.inf-facts dt,
.inf-spend dt {
  color: var(--ink-2);
}
.inf-facts dd,
.inf-spend dd {
  margin: 0;
}
.inf-spend-row {
  display: grid;
  gap: 2px;
}
.inf-figure {
  font-variant-numeric: var(--numeric);
}
.inf-reached {
  display: block;
  font-weight: 500;
}
.inf-note {
  margin: 0;
  color: var(--ink-2);
}
.inf-error {
  margin: 0;
  color: var(--ink);
}
.inf-action {
  padding: 4px 10px;
  background: var(--accent);
  color: var(--accent-ink);
  border: 0;
  border-radius: var(--r-ctl);
  box-shadow: var(--e-btn);
  font: inherit;
  cursor: pointer;
  transition: opacity var(--dur-fast) var(--ease);
}
.inf-action:disabled {
  opacity: 0.6;
  cursor: default;
}
.inf-confirm {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.inf-confirm p {
  flex-basis: 100%;
  margin: 0;
}
@media (prefers-reduced-motion: reduce) {
  .inf-action {
    transition: none;
  }
}
```

The popover's own surface, ring and shadow are phase 2's `.pop` (`desktop/src/components/common/common.css`); this file adds none, so the panel's elevation is the one every popover has.

- [ ] **Step 7: Put it in the top bar**

In `desktop/src/components/topbar/TopBar.tsx`, import `InferencePanel` from `../inference/InferencePanel` and add one element after `<LayoutToggle layout={layout} onChange={setLayout} />` (`TopBar.tsx:23`):

```tsx
      <InferencePanel />
```

Nothing else in the top bar moves. The spacer after it keeps Authority, Run locally and New session where they are.

- [ ] **Step 8: Run the tests to verify they pass**

```bash
npx vitest run src/components/inference --reporter=verbose 2>&1 | grep -c '✓'   # expect 11
npx vitest run                                                                 # expect every suite passing, phase 2's included
npx tsc --noEmit -p .                                                          # expect nothing
```

The second line matters for two phase 2 suites in particular: `src/api/client.test.ts` (the client gained two methods and a code) and the top-bar tests (the bar gained an element that renders nothing without a selected project).

- [ ] **Step 9: Kill four mutants by hand**

Each edit, made alone and reverted, must turn the named test red:

1. In `raise`, call `setInf({ ...inf, route: "au", effectiveRoute: "au" })` before the `await` → "keeps the chip on Global when the PATCH fails".
2. In `RouteChip`, pass `route={inf.route}` instead of `eff` → "shows the effective route, never the stored one".
3. In `resetDay`, use `getMonth`/`getFullYear` and `new Date(y, m + 1, 1)` → "says when a budget is reached and when it resets, in UTC", under `TZ=Pacific/Kiritimati npx vitest run src/components/inference` (UTC+14, where local and UTC months differ at 00:00 UTC on the first).
4. In `isExhausted`, `>` for `>=` → the same test.

- [ ] **Step 10: Commit**

```bash
git add desktop/src/components/inference/RouteChip.tsx desktop/src/components/inference/InferencePanel.tsx \
  desktop/src/components/inference/format.ts desktop/src/components/inference/inference.css \
  desktop/src/components/inference/InferencePanel.test.tsx
git commit -F <message file> -- desktop/src/components/inference desktop/src/api/client.ts desktop/src/api/problems.ts \
  desktop/src/store/types.ts desktop/src/components/topbar/TopBar.tsx
```

Message: `feat(desktop): inference chip and panel, raise-only, spend in words (phase 6 task 22)`, a body naming decisions (a) to (f), and the attribution line.

**Acceptance:**

1. Step 8 prints `11`, then every suite passing, then nothing from `tsc`.
2. Step 9's four edits each fail the named test.
3. By hand, with task 21's control plane on `file:` and the desktop in dev mode: select a classified Global project; the chip reads **Global**; open it, choose *Keep inference in Australia*, confirm; the chip reads **AU** and the popover says the inference stays in Australia. Whether it *looks* right is the owner's call, not this task's (`CLAUDE.md`, working agreements).

**What this task does not claim:**

- **Visual acceptance.** jsdom renders no layout, and the corporate network blocks headless screenshots. The owner's eyes decide.
- **That the spend figure is current.** It is what task 21 returned when the popover opened, which is keysmith's figure at most 60 s old (task 09).
- **A cloud project.** The desktop calls the local control plane. For a Hub project the same shape comes from the Hub: the `GET`, spend included, through ask H-5 (seam S6.11, review I-9), the `PATCH` through ask H-1's new member-role route (seam S6.1, review I-3). Which base URL serves them is phase H's.
- **Team-project announcements** of a raise (`05-design.md:219-222`), which are Fleet's.
- **That `level` strings read well to a person.** They are rendered verbatim (`internal, any`) until owner question Q1 names KPMG's scheme and its wording.

**Gaps against the skeleton, for the orchestrator:**

1. **The top bar has no project name** to sit beside (`TopBar.tsx:21-45`). The chip carries the name in its accessible label instead (decision (a)); whether the bar should show the project name at all is a phase 2 design question for the owner.
2. **Local projects cannot be classified from the cockpit.** The skeleton offers one action, and classification is ask P4-1's create flow. Local projects have no create flow (task 21's decision (a)), so until phase 4 exists they are classified by `curl` (task 23). A class picker shown only for unclassified projects would be one more control here; it is an owner call, not built.
3. **`UsageRow`'s JSON keys are assumed** (`spendUsd`, `budgetUsd`, `periodStart`, …) because task 03's part is not written yet. Step 1 reads the real ones; this file follows task 03.
4. **The exhausted sentence names the project** whichever dimension is exhausted. For the person or team dimension the true effect is wider (every project of that person or team). The skeleton's wording is kept; the owner may want one sentence per dimension.
5. **Task 21's `GET` does not say whether this caller may lower the route** (fix batch b05, review I-2, owner question Q5). Decision (j) lets a solo project's owner lower to the floor, and `localProjectsAreSolo` is a server-side constant no wire field carries, so the desktop cannot tell a project it may lower from one it may not. While Q5 is open that costs nothing, because this panel draws no lowering control either way. If the owner answers Q5 with the design's rule, task 21's `inferenceJSON` should gain one boolean — `canLowerRoute`, computed as `route != dataclass.Floor(class) && dataclass.CanChange(class, route, Floor(class), actor) == nil`, and served `false` by the Hub for a team project (ask H-5) — and this task gains a low-emphasis *Move back to Global* action shown only when it is true. **Both halves are load-bearing.** `CanChange` returns `nil` at its raise-or-no-change branch (`task-01.md:606-608`, `if rt >= rf`), so the second half alone is true where there is nothing to lower: a project already on Global, and an `au-only` project (floor `au`, route `au`), whose rendered *Move back to Global* would send `PATCH {"route": "global"}` and earn `409 conflict` from task 01's floor rule, the refusal task 21's tests already assert. Read the field as *a lowering to the floor would be accepted **and** there is one to make*. The desktop must never infer the right from the route or the class itself: that would be the cockpit re-deriving an authorisation rule task 01 owns.
