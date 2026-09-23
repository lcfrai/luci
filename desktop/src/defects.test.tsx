// Contract §9 (task 24) and §8. Phase 2's evidence file, in two halves. The first asserts in
// the ASSEMBLED app what the unit suites can only assert in isolation: that App mounts the
// shell, loads the catalogue exactly once, polls sessions and nothing else, and destroys no
// terminal while it does. The second is the manifest over §8's ten known defects, and the
// facts later groups settled after the UI contract was written.

import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import { FakeTerm, FakeWebSocket } from "./test/fakes";
import { aBranch, aHarness, aMachine, aProject, aSession, aWorktree } from "./test/fixtures";
import {
  __resetAttachRegistry,
  acquireAttach,
  configureAttachRegistry,
  releaseAttach,
} from "./api/attachRegistry";
import { ApiError } from "./api/client";
import type { CockpitClient } from "./api/client";
import { POLL_INTERVAL_MS } from "./api/poller";
import { resetCockpit, useCockpit } from "./store/store";
import type { Session, SessionId } from "./store/types";
import App, { startCatalogue } from "./App";

// The dynamic-import form, per task 14's note at the top of fakes.ts: vi.mock is hoisted above
// this file's imports, so a factory that closes over the static FakeTerm binding can run first.
vi.mock("@xterm/xterm", async () => ({ Terminal: (await import("./test/fakes")).FakeTerm }));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class { fit() {} activate() {} dispose() {} },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

/** The control plane, as a table. `vi.hoisted` because the mock factory below is hoisted
 *  above every import and a plain `const` would not exist yet when it runs. */
const server = vi.hoisted(() => ({
  machines: [] as unknown[],
  projects: [] as unknown[],
  harnesses: [] as unknown[],
  branches: new Map<string, unknown[]>(),
  worktrees: new Map<string, unknown[]>(),
  sessions: [] as unknown[],
  /** Set to make every endpoint reject — the control plane not running. */
  fail: null as Error | null,
  /** One endpoint rejecting while the rest answer: a 500 on one list, not an absent daemon. */
  failing: new Map<string, Error>(),
  /** Endpoints that never answer: a request that is still in flight when the next tick lands. */
  hanging: new Set<string>(),
  calls: { machines: 0, projects: 0, harnesses: 0, branches: 0, worktrees: 0, sessions: 0 },
  /** Every listWorktrees call's (projectId, branchId), in order. */
  worktreeArgs: [] as [string, string][],
}));

// Every module in the tree that talks to the control plane imports `{ api }` from here — App,
// useCreateSession, EmptyDevelop, SessionPane, sessionVerbs and the attach registry's default
// client — so one stub covers the whole assembled app.
vi.mock("./api", async () => {
  const { FakeWebSocket: Socket } = await import("./test/fakes");
  /** Far above any count a test asserts; see the comment inside `answer`. */
  const RUNAWAY_CALLS = 50;
  const answer = async <T,>(key: keyof typeof server.calls, value: T): Promise<T> => {
    server.calls[key] += 1;
    // A loader that fetches on every store write feeds itself — each answer is a store write —
    // and spins in microtasks, which HANGS the worker rather than failing a test. Parking every
    // call past this cap turns that runaway into a count an assertion can read.
    if (server.calls[key] > RUNAWAY_CALLS) return new Promise<T>(() => {});
    if (server.hanging.has(key)) return new Promise<T>(() => {});
    if (server.fail !== null) throw server.fail;
    const failure = server.failing.get(key);
    if (failure !== undefined) throw failure;
    return value;
  };
  return {
    api: {
      listMachines: () => answer("machines", server.machines),
      listProjects: () => answer("projects", server.projects),
      listHarnesses: () => answer("harnesses", server.harnesses),
      listBranches: (projectId: string) =>
        answer("branches", server.branches.get(projectId) ?? []),
      // Keyed by project AND branch, as §4.4's route is: the wrong project answers nothing,
      // which is what /v1/projects/<wrong>/worktrees does. A stub that ignored the project
      // would accept a loader that fetched before it knew which project the branch is in.
      listWorktrees: (projectId: string, branchId: string) => {
        server.worktreeArgs.push([projectId, branchId]);
        return answer(
          "worktrees",
          (server.worktrees.get(branchId) ?? []).filter(
            (w) => (w as { projectId: string }).projectId === projectId,
          ),
        );
      },
      listSessions: () => answer("sessions", server.sessions),
      openAttach: (s: Session) =>
        new Socket(`ws://127.0.0.1:8730${s.attachPath}`) as unknown as WebSocket,
      getSession: async () => server.sessions[0] as Session,
    },
  };
});

/** Fake timers plus a real microtask flush. `advanceTimersByTimeAsync` drains the promise
 *  queue between timer callbacks, which `advanceTimersByTime` does not, so a fetch resolved
 *  inside a tick has actually been written to the store by the time this returns. */
async function settle(ms = 0): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
    // Three drains, not one. A catalogue load resolves, its store write wakes the
    // navigation subscription, and THAT starts a second fetch: branches, then worktrees.
    // One drain returns before the second round has begun.
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(0);
  });
}

function seedCatalogue(): void {
  server.machines = [aMachine({ id: "m-1" })];
  server.projects = [aProject({ id: "p-1" })];
  server.harnesses = [aHarness({ id: "claude" })];
  server.branches.set("p-1", [aBranch({ id: "b-1", projectId: "p-1" })]);
  server.worktrees.set("b-1", [
    aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1" }),
  ]);
}

/** The one error line the rail footer renders, or a sentence saying there is none — so a
 *  failure reads "expected 'nothing: lastError is null' to contain …", not a type complaint. */
function shownError(): string {
  return useCockpit.getState().lastError?.message ?? "nothing: lastError is null";
}

/** Task 19's trick: reach the registry entry the pane already built. The factory throws if
 *  it is ever called, so reaching one IS the assertion that one exists. */
function grabTerm(id: SessionId): FakeTerm {
  const handle = acquireAttach(useCockpit.getState().sessions[id], {
    createTerm: () => {
      throw new Error(`no terminal exists for ${id}; the pane never mounted`);
    },
    readOnly: false,
  });
  releaseAttach(id);
  return handle.term as unknown as FakeTerm;
}

/** The DOM node the registry's element sits in right now: the pane's host while it is mounted.
 *  Reached the way grabTerm reaches the terminal, so reaching one is also the assertion that the
 *  entry exists. */
function hostOf(id: SessionId): Node | null {
  const handle = acquireAttach(useCockpit.getState().sessions[id], {
    createTerm: () => {
      throw new Error(`no terminal exists for ${id}; the pane never mounted`);
    },
    readOnly: false,
  });
  releaseAttach(id);
  return handle.element.parentNode;
}

beforeEach(() => {
  vi.useFakeTimers();
  FakeWebSocket.reset();
  __resetAttachRegistry();
  resetCockpit();
  server.machines = [];
  server.projects = [];
  server.harnesses = [];
  server.sessions = [];
  server.branches.clear();
  server.worktrees.clear();
  server.fail = null;
  server.failing.clear();
  server.hanging.clear();
  server.worktreeArgs = [];
  for (const key of Object.keys(server.calls) as (keyof typeof server.calls)[]) {
    server.calls[key] = 0;
  }
  configureAttachRegistry({
    client: {
      openAttach: (s: Session) =>
        new FakeWebSocket(`ws://127.0.0.1:8730${s.attachPath}`) as unknown as WebSocket,
      getSession: async () => server.sessions[0] as Session,
    },
  });
});

afterEach(() => {
  // Unmounted HERE, while the clock is still fake, so App's cleanup clears the interval it
  // actually armed and each pane's release lands on the fake clock too. Left to setup.ts's own
  // cleanup, which runs after this hook, both would be handed to the real clock instead.
  cleanup();
  __resetAttachRegistry();
  vi.useRealTimers();
});

describe("the assembled cockpit", () => {
  it("mounts the shell and its grain layer, and adds no chrome of its own", async () => {
    const { container } = render(<App />);
    await settle();
    const shell = container.querySelector(".shell");
    expect(shell).not.toBeNull();
    expect(shell?.classList.contains("grain")).toBe(true);
    // App renders <AppShell/> and nothing else: no wrapper div, no banner, no router.
    expect(container.firstElementChild).toBe(shell);
  });

  it("loads machines, projects and harnesses exactly once on mount", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    expect(server.calls.machines).toBe(1);
    expect(server.calls.projects).toBe(1);
    expect(server.calls.harnesses).toBe(1);
    const state = useCockpit.getState();
    expect(state.machines).toHaveLength(1);
    expect(state.projects).toHaveLength(1);
    expect(state.harnesses).toHaveLength(1);
  });

  it("polls sessions on the interval and never refetches the catalogue", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    expect(server.calls.sessions).toBe(1);   // §4.7: the first tick does not wait 2 s
    await settle(POLL_INTERVAL_MS * 3);
    expect(server.calls.sessions).toBe(4);
    // Machines, projects and harnesses change when the user acts, not when an agent does.
    expect(server.calls.machines).toBe(1);
    expect(server.calls.projects).toBe(1);
    expect(server.calls.harnesses).toBe(1);
  });

  it("stops the interval when the app unmounts", async () => {
    const { unmount } = render(<App />);
    await settle();
    const before = server.calls.sessions;
    unmount();
    await settle(POLL_INTERVAL_MS * 5);
    expect(server.calls.sessions).toBe(before);
  });

  it("loads branches when a project opens, worktrees when a branch is selected", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    expect(server.calls.branches).toBe(0);   // nothing is open yet

    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(server.calls.branches).toBe(1);
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);

    act(() => useCockpit.getState().selectBranch("p-1", "b-1"));
    await settle();
    expect(server.calls.worktrees).toBe(1);
    expect(useCockpit.getState().worktreesByBranch["b-1"]).toHaveLength(1);

    // Loaded once. The poll does not touch either, and neither does a re-render.
    await settle(POLL_INTERVAL_MS * 2);
    expect(server.calls.branches).toBe(1);
    expect(server.calls.worktrees).toBe(1);
  });

  // RV24-A and RV24-B (task 24 review, I2). An empty list is a real answer, and every other
  // fixture in this file has exactly one branch and one worktree, so nothing else can see a
  // loader that read "no rows" as "not arrived". That loader refetches after EVERY store write,
  // its own loading flag included: the self-feeding loop the failure memo exists to stop,
  // reached through success instead. The stub parks every call past its cap, so the loop reads
  // as 51 calls here rather than hanging the worker. A real control plane has no cap.
  it("takes an empty branch list as an answer and does not ask for it again (RV24-A)", async () => {
    // A project with no local branch lists []. A fresh `git init` plausibly does.
    seedCatalogue();
    server.branches.set("p-1", []);
    render(<App />);
    await settle();
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle(POLL_INTERVAL_MS * 2);
    expect(server.calls.branches).toBe(1);
    expect(useCockpit.getState().branchesByProject["p-1"]).toEqual([]);
  });

  it("takes an empty worktree list as an answer and does not ask for it again (RV24-B)", async () => {
    seedCatalogue();
    server.worktrees.set("b-1", []);
    render(<App />);
    await settle();
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    act(() => useCockpit.getState().selectBranch("p-1", "b-1"));
    await settle(POLL_INTERVAL_MS * 2);
    expect(server.calls.worktrees).toBe(1);
    expect(useCockpit.getState().worktreesByBranch["b-1"]).toEqual([]);
  });

  it("waits for a restored branch's project list before asking for its worktrees", async () => {
    // persist rehydrates `selection` before any fetch has run, so the branch id is known and
    // the branch list that says which project owns it is not. listWorktrees needs both (§4.4),
    // so the first ask must wait for that list rather than guess — and be the only ask.
    seedCatalogue();
    useCockpit.setState({
      selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    });
    render(<App />);
    await settle();
    expect(server.worktreeArgs).toEqual([["p-1", "b-1"]]);
    expect(useCockpit.getState().worktreesByBranch["b-1"]).toHaveLength(1);
  });

  it("leaves the shell standing when the control plane is not running, and names the failure", async () => {
    server.fail = new Error("fetch failed");
    const { container } = render(<App />);
    await settle();
    expect(container.querySelector(".shell")).not.toBeNull();
    const error = useCockpit.getState().lastError;
    expect(error).not.toBeNull();
    expect(error?.message).toContain("fetch failed");
  });

  it("keeps a failed list's error on screen across good ticks, then clears it when the retry lands", async () => {
    // One list fails while the poller is healthy — a 500 on /v1/projects, not an absent
    // daemon, so the test above cannot see it. The poller's good ticks must not wipe the
    // catalogue's report (they used to, 2 s later), and each good tick retries the list.
    seedCatalogue();
    server.failing.set("projects", new Error("projects answered 500"));
    const { container } = render(<App />);
    await settle();
    expect(shownError()).toContain("projects answered 500");
    const firstTicks = server.calls.sessions;

    // Every value written, not the end state (clause 7). The retry on each good tick fails again
    // and re-reports, so a poller that wrongly cleared the report would leave the same message
    // on screen at the end, having blanked the footer for the length of one retry on every
    // tick. Measured: the end-state assertion alone let that poller through.
    const written: (string | null)[] = [];
    const unsubscribe = useCockpit.subscribe((next, prev) => {
      if (next.lastError !== prev.lastError) written.push(next.lastError?.message ?? null);
    });
    await settle(POLL_INTERVAL_MS);
    await settle(POLL_INTERVAL_MS);
    unsubscribe();
    expect(server.calls.sessions).toBe(firstTicks + 2);           // two more good ticks
    expect(written).not.toContain(null);
    expect(shownError()).toContain("projects answered 500");
    // Retried on those ticks — and never more than once per tick.
    expect(server.calls.projects).toBeGreaterThan(1);
    expect(server.calls.projects).toBeLessThanOrEqual(server.calls.sessions + 1);

    server.failing.delete("projects");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().lastError).toBeNull();
    const names = [...container.querySelectorAll(".rail .rail-name")].map((el) => el.textContent);
    expect(names).toContain("panacea");

    // Arrived means arrived. Further good ticks fetch neither the recovered list nor the two
    // that never failed: the retry asks only for what failed or has not arrived.
    const recovered = server.calls.projects;
    await settle(POLL_INTERVAL_MS * 2);
    expect(server.calls.projects).toBe(recovered);
    expect(server.calls.machines).toBe(1);
    expect(server.calls.harnesses).toBe(1);
  });

  it("still lets the poller clear an error it raised itself, on its next good tick", async () => {
    // Task 19's behaviour, which the fix above must not have broken: the poller now clears
    // only its own report, and this is its own report.
    seedCatalogue();
    server.failing.set("sessions", new Error("sessions answered 500"));
    render(<App />);
    await settle();
    expect(shownError()).toContain("sessions answered 500");

    // And the catalogue's own successes leave it alone: a branch list arriving says nothing
    // about whether sessions can be read.
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);
    expect(shownError()).toContain("sessions answered 500");

    server.failing.delete("sessions");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().lastError).toBeNull();
    expect(server.calls.projects).toBe(1);   // the catalogue never failed, so nothing retried
  });

  it("clears only the error of the list that recovered, never another list's", async () => {
    // Two lists fail; one recovers. Its success must not wipe the other's report — which is
    // what a clear keyed on "did I ever fail" rather than "is that still my error" would do,
    // because the retries land in order and the still-failing list reports first.
    seedCatalogue();
    server.failing.set("machines", new Error("machines answered 500"));
    server.failing.set("projects", new Error("projects answered 500"));
    render(<App />);
    await settle();
    server.failing.delete("projects");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().projects).toHaveLength(1);
    expect(shownError()).toContain("machines answered 500");
  });

  it("does not let the poller's recovery clear a catalogue error that replaced its own", async () => {
    // Sessions fail, then a branch list fails and its report replaces the poller's. When
    // sessions recover the poller must see that lastError is no longer its report and leave
    // it. Asserted on every value written during the recovery, not on the end state: the
    // branch retry on that same tick fails again and re-reports, which would paper over a
    // poller that had wrongly cleared it a moment earlier.
    seedCatalogue();
    server.failing.set("sessions", new Error("sessions answered 500"));
    render(<App />);
    await settle();
    server.failing.set("branches", new Error("branches answered 500"));
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(shownError()).toContain("branches answered 500");

    const written: (string | null)[] = [];
    const unsubscribe = useCockpit.subscribe((next, prev) => {
      if (next.lastError !== prev.lastError) written.push(next.lastError?.message ?? null);
    });
    server.failing.delete("sessions");
    await settle(POLL_INTERVAL_MS);
    unsubscribe();
    expect(written).not.toContain(null);
    expect(shownError()).toContain("branches answered 500");
  });

  it("puts a list's report back when the poller clears the report that had replaced it (RV24-C)", async () => {
    // The order the test above does not take (task 24 review, M1). The catalogue fails first,
    // then the poller's failure takes the one slot, then the poller recovers. It clears its own
    // report, as it must, and the slot empties while the projects list is still missing. The
    // retry that same good tick starts is HELD here, and that is the case that matters: a hung
    // list lasts up to the client's 10 s timeout. Before this fix the rail was empty and the
    // footer clean for all of it. The loader now puts its outstanding report back.
    seedCatalogue();
    server.failing.set("projects", new Error("projects answered 500"));
    render(<App />);
    await settle();
    expect(shownError()).toContain("projects answered 500");

    server.failing.set("sessions", new Error("sessions answered 500"));
    await settle(POLL_INTERVAL_MS);
    expect(shownError()).toContain("sessions answered 500");

    server.hanging.add("projects");
    server.failing.delete("sessions");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().projects).toHaveLength(0);
    expect(shownError()).toContain("projects answered 500");
  });

  it("puts back the newest of two outstanding reports", async () => {
    // §3.2's slot holds "the last API failure", so a put-back picks the list that failed most
    // recently. Machines and projects both fail at mount. Then only machines is retried to a
    // fresh failure, because projects' retry is held. When the slot next empties, machines holds
    // the newer report, though projects was the one on screen first.
    seedCatalogue();
    server.failing.set("machines", new Error("machines answered 500"));
    server.failing.set("projects", new Error("projects answered 500"));
    render(<App />);
    await settle();
    server.hanging.add("projects");
    await settle(POLL_INTERVAL_MS);   // a good tick: machines fails again, projects is held
    expect(shownError()).toContain("machines answered 500");

    server.hanging.add("machines");   // from here no retry lands, so only the put-back writes
    server.failing.set("sessions", new Error("sessions answered 500"));
    await settle(POLL_INTERVAL_MS);
    expect(shownError()).toContain("sessions answered 500");
    server.failing.delete("sessions");
    await settle(POLL_INTERVAL_MS);
    expect(shownError()).toContain("machines answered 500");
  });

  it("does not put back the report of a list the user has since closed", async () => {
    // The other half of putting a report back. p-1's branch list fails and the user collapses
    // p-1, so the rail shows no branch rows because it shows none, not because a list is
    // missing. A closed project's list is not retried, so a report put back for it would sit on
    // the footer, unrefreshed, for as long as the project stays closed.
    seedCatalogue();
    server.failing.set("branches", new Error("branches answered 500"));
    render(<App />);
    await settle();
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(shownError()).toContain("branches answered 500");
    act(() => useCockpit.getState().toggleProject("p-1"));

    // Empty the slot the only way a test can without touching the catalogue: the poller's own
    // report replaces it, and the poller's recovery clears that.
    server.failing.set("sessions", new Error("sessions answered 500"));
    await settle(POLL_INTERVAL_MS);
    expect(shownError()).toContain("sessions answered 500");
    server.failing.delete("sessions");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().lastError).toBeNull();
  });

  it("does not put back the report of a worktree list whose branch the user has since closed", async () => {
    // The same for the level below: the project stays open and only the branch closes, so only
    // the worktree half of "can the user see it is missing" decides. Survived as mutant M1g.
    seedCatalogue();
    server.failing.set("worktrees", new Error("worktrees answered 500"));
    useCockpit.setState({ expandedProjects: { "p-1": true }, expandedBranches: { "b-1": true } });
    render(<App />);
    await settle();
    expect(shownError()).toContain("worktrees answered 500");
    act(() => useCockpit.getState().toggleBranch("b-1"));

    server.failing.set("sessions", new Error("sessions answered 500"));
    await settle(POLL_INTERVAL_MS);
    expect(shownError()).toContain("sessions answered 500");
    server.failing.delete("sessions");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().lastError).toBeNull();
  });

  it("fills in the rail once a control plane that was down at launch starts answering", async () => {
    // The cockpit opened before its control plane. Every list fails, the poller fails, and
    // nothing retries until a tick succeeds — so a daemon that is down costs no extra traffic.
    seedCatalogue();
    server.fail = new Error("fetch failed");
    const { container } = render(<App />);
    await settle();
    await settle(POLL_INTERVAL_MS * 2);
    expect(useCockpit.getState().projects).toHaveLength(0);
    expect(server.calls.projects).toBe(1);

    server.fail = null;
    await settle(POLL_INTERVAL_MS);
    const state = useCockpit.getState();
    expect(state.machines).toHaveLength(1);
    expect(state.projects).toHaveLength(1);
    expect(state.harnesses).toHaveLength(1);
    expect(state.lastError).toBeNull();
    const names = [...container.querySelectorAll(".rail .rail-name")].map((el) => el.textContent);
    expect(names).toContain("panacea");
  });

  it("retries a failed branch list on the next good tick", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    server.failing.set("branches", new Error("branches answered 500"));
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(useCockpit.getState().branchesByProject["p-1"]).toBeUndefined();

    server.failing.delete("branches");
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);
    expect(useCockpit.getState().lastError).toBeNull();
  });

  it("keeps at most one request in flight per list, however many good ticks pass", async () => {
    // A list that is still loading is neither failed nor arrived, so every good tick would
    // retry it without the in-flight guard.
    seedCatalogue();
    server.hanging.add("projects");
    render(<App />);
    await settle();
    await settle(POLL_INTERVAL_MS * 3);
    expect(server.calls.sessions).toBeGreaterThanOrEqual(4);
    expect(server.calls.projects).toBe(1);
  });

  it("does not retry a failed load on every store write", async () => {
    seedCatalogue();
    render(<App />);
    await settle();
    server.fail = new Error("fetch failed");
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(server.calls.branches).toBe(1);

    // Any unrelated store write re-runs the subscription. A loader with no memo of what
    // already failed turns that into a request per keystroke.
    act(() => useCockpit.getState().toggleRail());
    act(() => useCockpit.getState().toggleRail());
    await settle();
    expect(server.calls.branches).toBe(1);
  });

  it("survives StrictMode's double mount: the stopped loader writes nothing, every list lands once", async () => {
    // main.tsx renders under StrictMode, so this is the shape every dev build actually runs.
    // The effect mounts, stops and re-mounts synchronously. The stopped loader's replies are
    // discarded and the second loader fetches for itself — one duplicate request per list, in
    // dev only, which is what the poller already costs. What is pinned is the ACT (clause 7):
    // each list is WRITTEN once, because a stopped loader that still wrote would put the same
    // value in the store twice and no assertion on the value could tell.
    seedCatalogue();
    useCockpit.setState({ expandedProjects: { "p-1": true } });
    const writes = { machines: 0, projects: 0, harnesses: 0, branches: 0 };
    const unsubscribe = useCockpit.subscribe((next, prev) => {
      if (next.machines !== prev.machines) writes.machines += 1;
      if (next.projects !== prev.projects) writes.projects += 1;
      if (next.harnesses !== prev.harnesses) writes.harnesses += 1;
      if (next.branchesByProject !== prev.branchesByProject) writes.branches += 1;
    });
    render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
    await settle();
    unsubscribe();
    expect(writes).toEqual({ machines: 1, projects: 1, harnesses: 1, branches: 1 });
    expect(server.calls.machines).toBe(2);   // one per mount; the first reply is discarded
    const state = useCockpit.getState();
    expect(state.machines).toHaveLength(1);
    expect(state.projects).toHaveLength(1);
    expect(state.harnesses).toHaveLength(1);
    expect(state.branchesByProject["p-1"]).toHaveLength(1);
    expect(Object.entries(state.loading).filter(([, on]) => on)).toEqual([]);
  });

  it("under StrictMode, reports a failed list once — from the loader that survived", async () => {
    // The other direction. The stopped loader's failure is discarded like its successes; the
    // surviving loader's own attempt is what reports. Both loaders' reports reaching the store
    // would be the same message twice.
    seedCatalogue();
    server.hanging.add("sessions");   // no good tick, so no retry muddies the count
    server.failing.set("projects", new Error("projects answered 500"));
    const reported: string[] = [];
    const unsubscribe = useCockpit.subscribe((next, prev) => {
      if (next.lastError !== prev.lastError && next.lastError !== null) {
        reported.push(next.lastError.message);
      }
    });
    render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
    await settle();
    unsubscribe();
    expect(server.calls.projects).toBe(2);
    expect(reported.filter((m) => m.includes("projects answered 500"))).toHaveLength(1);
    expect(shownError()).toContain("projects answered 500");
  });

  it("asks for a branch's worktrees under the project that lists it, not the first one open", async () => {
    // Project 1 is open and its branch list has landed. Then a branch of project 2 is selected,
    // from the palette, say, which selects without expanding. A lookup that took the first
    // project it found, rather than the one whose list holds the branch, would ask
    // /v1/projects/p-1/worktrees for p-2's branch at once, get [], and keep that empty answer
    // for good, because a branch whose worktrees are defined is never fetched again.
    //
    // The order is set by hand, not left to request order, because request order is not
    // landing order here: `run` writes its loading flag BEFORE it fetches, and that write
    // re-enters `sync`, which fetches the second project's list first. With both projects open
    // at mount, "the first entry" would be the right project by accident, and the lookup would
    // survive. Measured: the first draft of this test did exactly that.
    seedCatalogue();
    server.projects = [aProject({ id: "p-1" }), aProject({ id: "p-2", name: "second" })];
    server.branches.set("p-2", [aBranch({ id: "b-2", projectId: "p-2" })]);
    server.worktrees.set("b-2", [aWorktree({ id: "w-2", projectId: "p-2", branchId: "b-2" })]);
    render(<App />);
    await settle();
    act(() => useCockpit.getState().toggleProject("p-1"));
    await settle();
    expect(Object.keys(useCockpit.getState().branchesByProject)).toEqual(["p-1"]);

    act(() => useCockpit.getState().selectBranch("p-2", "b-2"));
    await settle();
    expect(server.worktreeArgs).toEqual([["p-2", "b-2"]]);
    expect(useCockpit.getState().worktreesByBranch["b-2"]).toHaveLength(1);
  });

  it("reports the control plane's refusal by its own code and its own sentence", async () => {
    // One direction of asError (shared shape §3A.1 clause 3). Every other failure in this file
    // is a plain Error, so the ApiError branch was never taken by any test. The rail footer
    // renders `message` verbatim, so the server's sentence must arrive unprefixed, and the code
    // is what task 12's problems.ts classifies on.
    seedCatalogue();
    server.hanging.add("sessions");   // no good tick: nothing retries and nothing clears
    server.failing.set("projects", new ApiError("project_not_found", "No project has that id.", 404));
    render(<App />);
    await settle();
    expect(useCockpit.getState().lastError).toEqual({
      code: "project_not_found",
      message: "No project has that id.",
    });
  });

  it("reports anything that is not an ApiError as internal, with its class in the message", async () => {
    // The other direction. The client converts every network failure to an ApiError, so this
    // branch is for a defect in the cockpit itself, and "internal" plus the class name is what
    // tells the owner which side of the wire to look at.
    seedCatalogue();
    server.hanging.add("sessions");
    server.failing.set("projects", new TypeError("cannot read properties of undefined"));
    render(<App />);
    await settle();
    expect(useCockpit.getState().lastError).toEqual({
      code: "internal",
      message: "TypeError: cannot read properties of undefined",
    });
  });

  it("does not load the branches of a project the user collapsed, as persist restores it", async () => {
    // expandedProjects is persisted (store.ts partialize), so a project expanded and then
    // collapsed last session comes back as `false`, not absent. The key is there; the value
    // says closed. Nothing else in this file writes a `false`, so no other test could see a
    // loader that read the keys and ignored the values.
    seedCatalogue();
    useCockpit.setState({ expandedProjects: { "p-1": false } });
    render(<App />);
    await settle(POLL_INTERVAL_MS);
    expect(server.calls.branches).toBe(0);
  });

  it("does not load the worktrees of a branch the user collapsed, inside a project that is open", async () => {
    // The same for branches, with the project open so its branch list lands and the branch's
    // project is known. Only the branch's own `false` stands between it and a fetch.
    seedCatalogue();
    useCockpit.setState({ expandedProjects: { "p-1": true }, expandedBranches: { "b-1": false } });
    render(<App />);
    await settle(POLL_INTERVAL_MS);
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);
    expect(server.calls.worktrees).toBe(0);
  });
});

describe("startCatalogue on its own", () => {
  /** A client whose every list waits until the test answers it, so a test can stop the loader
   *  with its requests still out — StrictMode's first mount, frozen at the moment it matters. */
  function heldClient(): {
    client: CockpitClient;
    held: { list: string; resolve: (value: unknown) => void; reject: (error: unknown) => void }[];
  } {
    const held: { list: string; resolve: (value: unknown) => void; reject: (error: unknown) => void }[] = [];
    const hold = (list: string) => (): Promise<never> =>
      new Promise<never>((resolve, reject) =>
        held.push({ list, resolve: resolve as (value: unknown) => void, reject }),
      );
    const client = {
      listMachines: hold("machines"),
      listProjects: hold("projects"),
      listHarnesses: hold("harnesses"),
      listBranches: hold("branches"),
      listWorktrees: hold("worktrees"),
    } as unknown as CockpitClient;
    return { client, held };
  }

  it("writes nothing once stopped — no list, no error, not even a loading flag — and starts nothing", async () => {
    // The StrictMode test above counts the LISTS a stopped loader writes. This counts every
    // store write (clause 7), because the one a list count cannot see is the loading flag: a
    // stopped loader that still cleared it would switch off the surviving loader's spinner
    // while that loader's own request is in flight. And `retry` is called after `stop`, as a
    // caller other than App's cleanup might, to pin "starts nothing after stop" directly.
    const { client, held } = heldClient();
    useCockpit.setState({ expandedProjects: { "p-1": true } });   // so a branch list is out too
    const catalogue = startCatalogue(client, useCockpit);
    expect(held.map((h) => h.list)).toEqual(["machines", "projects", "harnesses", "branches"]);

    let writes = 0;
    const unsubscribe = useCockpit.subscribe(() => {
      writes += 1;
    });
    catalogue.stop();
    held[0].resolve([aMachine()]);
    held[1].reject(new Error("projects answered 500"));
    held[2].resolve([]);
    held[3].resolve([]);
    await settle();
    catalogue.retry();
    await settle();
    unsubscribe();

    expect(writes).toBe(0);
    expect(held).toHaveLength(4);
  });

  it("puts no report back once stopped, though a list is still outstanding", async () => {
    // The put-back is the one store write that does not pass through `run`, so the test above
    // cannot see it: there, the only failure lands after stop and is discarded. Here the list
    // fails while the loader runs, the slot is emptied after stop, where no subscription can
    // answer, and `retry` then reaches `sync` directly.
    const { client, held } = heldClient();
    const catalogue = startCatalogue(client, useCockpit);
    held[1].reject(new Error("projects answered 500"));
    await settle();
    expect(shownError()).toContain("projects answered 500");

    catalogue.stop();
    useCockpit.getState().setError(null);
    let writes = 0;
    const unsubscribe = useCockpit.subscribe(() => {
      writes += 1;
    });
    catalogue.retry();
    await settle();
    unsubscribe();
    expect(writes).toBe(0);
  });

  it("asks for a restored selection's branches at once, without waiting for another reply", async () => {
    // A rehydrated selection is navigation that already happened, so its branch list goes out
    // with the three root lists rather than after the first of them lands. Held here, no
    // reply ever lands, so nothing but startup itself can have asked. In the assembled app the
    // poller's first tick writes the store within one round trip and would ask anyway, which
    // is why no test there can see this.
    const { client, held } = heldClient();
    useCockpit.setState({
      selection: { projectId: "p-1", branchId: null, worktreeId: null, sessionId: null },
    });
    const catalogue = startCatalogue(client, useCockpit);
    expect(held.map((h) => h.list)).toEqual(["machines", "projects", "harnesses", "branches"]);
    catalogue.stop();
  });
});

describe("defect 1, in the assembled app", () => {
  it("three poll ticks do not touch the terminal the user is typing into", async () => {
    seedCatalogue();
    const base = aSession({
      id: "s-1",
      projectId: "p-1",
      branchId: "b-1",
      worktreeId: "w-1",
      status: "running",
      attachPath: "/v1/sessions/s-1/attach",
    });
    server.sessions = [base];
    useCockpit.setState({
      selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
      lens: "develop",
    });

    render(<App />);
    await settle();

    // A zero here is not a defect-1 failure: it means the pane never rendered. Check that
    // the worktree load resolved, that `lens` is still "develop", and that AppShell's
    // Develop arm returns <DevelopLens/> rather than task 18's reserved LensPlaceholder.
    expect(FakeWebSocket.instances).toHaveLength(1);
    const term = grabTerm("s-1");
    term.write("the user is typing");
    const host = hostOf("s-1");
    expect(host?.isConnected).toBe(true);

    for (const status of ["running", "idle", "running"] as const) {
      server.sessions = [{ ...base, status, updatedAt: new Date().toISOString() }];
      await settle(POLL_INTERVAL_MS);
      // Asserted per tick, because the last status equals the first: a check made only at
      // the end could not tell three real store writes from none.
      expect(useCockpit.getState().sessions["s-1"].status).toBe(status);
    }

    // The store moved — so the poll really ran — and the terminal did not.
    expect(server.calls.sessions).toBe(4);
    const after = grabTerm("s-1");
    expect(after).toBe(term);
    // Identity AND content, both read off the handle re-acquired AFTER the three ticks.
    // Asserting on `term` here would prove nothing the line above has not already proved;
    // asserting on `after` is what catches a refactor that put a FRESH Terminal under the
    // same key, because a fresh one has an empty `written`. FakeTerm.written is a string[]
    // (task 14), one entry per write, so join("") is everything the pane has ever shown.
    expect(after.written.join("")).toContain("the user is typing");
    expect(after.disposed).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);
    // And the pane never remounted. A <Terminal> keyed on anything a tick changes (its status,
    // say) survives every assertion above, because the registry hands the new mount the same
    // terminal. But the remount lifts the element out of the DOM and puts it into a NEW host, and
    // a detached element loses focus and its scroll offset. That is argued, not observed here,
    // since jsdom lays nothing out. In a real window it is this defect by another route, and the
    // old host staying the host is what rules it out.
    expect(hostOf("s-1")).toBe(host);
  });
});

/* ── the ten known defects — contract §8 ────────────────────────────────────────────── */

// The source tree, read as text. `import.meta.glob` rather than node:fs, for the measured
// reason in styles/designsystem.test.ts's header: `@types/node` is not a dependency, so a
// node:fs import fails `npm run typecheck` with TS2591. The glob is expanded from disk on every
// run, so a regression test file that is deleted or renamed drops out of this map, which is
// exactly what the manifest checks for.
const SOURCE = import.meta.glob<string>("/src/**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
});
function text(relPath: string): string {
  const source = SOURCE[`/${relPath}`];
  if (source === undefined) throw new Error(`missing: ${relPath}`);
  return source;
}

/** The same source with block and line comments blanked, newlines kept. Shared shape §3A, and
 *  defect 8 is the shape it describes: an `absent` anchor bans a construct the file's own prose
 *  can legitimately DISCUSS. `useKeyboardMap.ts` exists partly to explain why it does NOT
 *  handle shift+tab, and the first draft of its header named both the key and the field while
 *  explaining it — which made a raw /permissionMode/ red against a correct file, permanently.
 *  Task 23 has since reworded that header to name neither, so the raw form passes today too.
 *  This strips anyway: a guard that only holds while another task keeps a particular sentence
 *  out of a particular comment is a guard that breaks on a rewrite nobody thought was risky.
 *
 *  Task 14 exports `stripCssComments` and the audit uses it; TypeScript's `//` form has no
 *  equivalent there, and this is the only place in Group D that needs one, so it lives here
 *  rather than growing another task's file. It is deliberately naive — it blanks a `//` inside
 *  a string literal too. For a `present` anchor that errs LOUD: code after such a `//` would
 *  vanish and the anchor would go red on a correct file. For an `absent` anchor it errs SILENT,
 *  hiding code that should have failed, which is why the one `absent` row sits on a file
 *  measured to have no `//` inside a string, and why a second `absent` row needs the same
 *  measurement before it is added. */
function codeOnly(source: string): string {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
    .replace(/\/\/[^\n]*/g, "");
}

interface Anchor {
  file: string;
  /** Code the fix cannot exist without, matched against `codeOnly(...)` like `absent` is. It
   *  used to be matched raw, on the argument that every anchor is a real declaration or call;
   *  measured, five of them are ALSO satisfied by a comment in their own file, so deleting the
   *  code left the anchor green. Against codeOnly a comment cannot stand in for the code. */
  present?: RegExp;
  /** Code whose PRESENCE would be the defect. Defect 8 is the only one shaped this way. */
  absent?: RegExp;
}

interface Defect {
  n: number;
  /** docs/FUNCTIONAL-SPEC.md's wording, as contract §8 quotes it. */
  spec: string;
  /** §8's "Fixed by" column. */
  fixedBy: number[];
  /** §8's "Regression test" column: the files, not the titles. Titles are prose that authors
   *  legitimately reword, and seven of §8's eighteen already have (counted by task 24's review,
   *  M6); a manifest keyed on them would be red for a reason that is not a regression, and a
   *  gate that cries wolf is a gate somebody deletes. */
  tests: string[];
  anchors: Anchor[];
}

const DEFECTS: Defect[] = [
  {
    n: 1,
    spec: "Live tick destroys in-progress typing",
    fixedBy: [19],
    tests: ["src/api/attachRegistry.test.ts", "src/components/Terminal.test.tsx"],
    anchors: [
      // Ref-counted entries keyed by session id, at module scope, outside React.
      { file: "src/api/attachRegistry.ts", present: /\brefs\b/ },
      { file: "src/components/Terminal.tsx", present: /\bmemo\s*\(/ },
    ],
  },
  {
    n: 2,
    spec: "`uid()` collides on fast successive calls",
    fixedBy: [15],
    tests: ["src/lib/ids.test.ts"],
    anchors: [{ file: "src/lib/ids.ts", present: /randomUUID\s*\(/ }],
  },
  {
    n: 3,
    spec: "Denying a permission dead-ends the session — composer must always render",
    fixedBy: [20],
    tests: ["src/components/develop/SessionPane.test.tsx"],
    anchors: [
      // Rendered once, unconditionally. readOnly gates stdin and nothing else.
      { file: "src/components/develop/SessionPane.tsx", present: /^\s*<Terminal\b/m },
      { file: "src/components/develop/SessionPane.tsx", present: /readOnly/ },
    ],
  },
  {
    n: 4,
    spec: "New session created off-screen with no focus",
    fixedBy: [20],
    tests: ["src/components/develop/ColumnsLayout.test.tsx"],
    anchors: [
      { file: "src/components/develop/ColumnsLayout.tsx", present: /scrollIntoView\s*\(/ },
      { file: "src/components/develop/ColumnsLayout.tsx", present: /focusSession/ },
    ],
  },
  {
    n: 5,
    spec: "Auto-scroll steals the scrollback",
    fixedBy: [19],
    tests: ["src/api/attachRegistry.test.ts"],
    anchors: [
      { file: "src/api/attachRegistry.ts", present: /pinned/ },
      { file: "src/api/attachRegistry.ts", present: /scrollToBottom\s*\(/ },
    ],
  },
  // Defects 7, 9 and 10 live in surfaces Phase 2 does not build — Discover is Phase 4, Merge
  // is Phase 3 — and each is fixed as a CLASS in the surface where the same mistake would
  // otherwise be made, which is why their anchors sit in lib/immutable.ts, store/types.ts,
  // sessionVerbs.ts and CommandPalette.tsx rather than anywhere named Merge.
  {
    n: 6,
    spec: "Canvas tile positions not persisted",
    fixedBy: [21],
    tests: [
      "src/components/develop/CanvasLayout.test.tsx",
      "src/store/store.test.ts",
    ],
    anchors: [
      { file: "src/store/slices/prefsSlice.ts", present: /\btiles\b/ },
      { file: "src/store/store.ts", present: /partialize/ },
    ],
  },
  {
    n: 7,
    spec: "`editAnswer` splices all later answers",
    fixedBy: [15, 22],
    tests: [
      "src/lib/immutable.test.ts",
      "src/components/develop/sessionVerbs.test.ts",
    ],
    anchors: [
      { file: "src/lib/immutable.ts", present: /export function replaceAt\b/ },
      { file: "src/lib/immutable.ts", present: /export function replaceById\b/ },
    ],
  },
  {
    n: 8,
    spec: "\"Don't ask again for X\" flips the whole session to bypass",
    fixedBy: [23],
    tests: ["src/hooks/useKeyboardMap.test.ts"],
    anchors: [
      // The only inverted row. The cockpit must not own a permission model at all: shift+tab
      // belongs to the agent's own TUI, so the keyboard map may not name Tab, and nothing
      // outside the create path may write permissionMode. Both run against codeOnly(...), so
      // the row asserts that no BRANCH here reads or writes either one — which is the thing
      // that matters — and stays green if a future header sentence names them while
      // explaining the rule. Shared shape §3A.
      { file: "src/hooks/useKeyboardMap.ts", absent: /["']Tab["']/ },
      { file: "src/hooks/useKeyboardMap.ts", absent: /permissionMode/ },
    ],
  },
  {
    n: 9,
    spec: "Nothing ever sets `ready`, so user-created worktrees can never merge",
    fixedBy: [22],
    tests: ["src/components/develop/sessionVerbs.test.ts"],
    anchors: [
      // §8's row is no longer contingent: §10.2 landed in task 10, so the "Mark ready" verb
      // PATCHes the flag and is always rendered. The type still carries it as optional — the
      // shape the contract kept — and the verb has to name it to write it.
      { file: "src/store/types.ts", present: /\bready\?:/ },
      { file: "src/components/develop/sessionVerbs.ts", present: /\bready\b/ },
    ],
  },
  {
    n: 10,
    spec: "Merge reorder indexes the unfiltered array",
    fixedBy: [15, 23],
    tests: [
      "src/lib/immutable.test.ts",
      "src/components/palette/CommandPalette.test.tsx",
    ],
    anchors: [
      { file: "src/lib/immutable.ts", present: /export function moveWithinVisible\b/ },
      { file: "src/lib/immutable.ts", present: /export function idAtVisiblePosition\b/ },
      // The rule the helpers encode: never pass an index across a filter boundary. The CALL,
      // not the name: the import line names it too, and survives the call being deleted.
      { file: "src/components/palette/CommandPalette.tsx", present: /\bidAtVisiblePosition\(/ },
    ],
  },
];

/** A file's text, or undefined when it does not exist. The two manifest checks take one of these
 *  rather than reading SOURCE, so that the canaries below can hand them a planted tree: against
 *  the real tree every weakening of a check passes, because the real tree has no fault to find. */
type Source = (relPath: string) => string | undefined;
const REAL_SOURCE: Source = (relPath) => SOURCE[`/${relPath}`];

function testFileProblems(defects: readonly Defect[], source: Source): string[] {
  const missing: string[] = [];
  for (const defect of defects) {
    for (const file of defect.tests) {
      const body = source(file);
      if (body === undefined) {
        missing.push(`defect ${defect.n}: ${file} does not exist`);
        continue;
      }
      if (!/\bit\s*\(/.test(body)) {
        missing.push(`defect ${defect.n}: ${file} contains no tests`);
      }
    }
  }
  return missing;
}

function anchorRegressions(defects: readonly Defect[], source: Source): string[] {
  const regressions: string[] = [];
  for (const defect of defects) {
    for (const anchor of defect.anchors) {
      const body = source(anchor.file);
      if (body === undefined) {
        regressions.push(`defect ${defect.n}: ${anchor.file} does not exist`);
        continue;
      }
      const code = codeOnly(body);
      if (anchor.present !== undefined && !anchor.present.test(code)) {
        regressions.push(`defect ${defect.n} (${defect.spec}): ${anchor.file} lost ${anchor.present}`);
      }
      if (anchor.absent !== undefined && anchor.absent.test(code)) {
        regressions.push(`defect ${defect.n} (${defect.spec}): ${anchor.file} gained ${anchor.absent}`);
      }
    }
  }
  return regressions;
}

describe("the ten known defects", () => {
  it("covers all ten, numbered 1 to 10, with no gap and no duplicate", () => {
    expect(DEFECTS.map((d) => d.n)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
  });

  it("names a regression test file for each, and every one of them exists and holds tests", () => {
    expect(testFileProblems(DEFECTS, REAL_SOURCE)).toEqual([]);
  });

  it("still has every fix in the implementation", () => {
    expect(anchorRegressions(DEFECTS, REAL_SOURCE)).toEqual([]);
  });
});

describe("the manifest's own checks, fed a planted tree", () => {
  // Shared shape §3A.1 clause 1 applied to the checker itself. Each case plants the one fault a
  // check exists to catch, where that check is the only thing that can catch it.
  const planted =
    (files: Record<string, string>): Source =>
    (relPath) =>
      files[relPath];
  const scroll: Anchor = { file: "src/fix.ts", present: /scrollIntoView\s*\(/ };
  const tab: Anchor = { file: "src/fix.ts", absent: /["']Tab["']/ };
  const canary: Defect = {
    n: 0,
    spec: "canary",
    fixedBy: [],
    tests: ["src/empty.test.ts", "src/gone.test.ts"],
    anchors: [scroll, tab],
  };

  it("blanks line and block comments, and keeps the code and the line count", () => {
    const source = 'run(); // "Tab" permissionMode\n/* refs\n   memo( */ keep();\n';
    const code = codeOnly(source);
    expect(code).not.toMatch(/Tab|permissionMode|refs|memo\(/);
    expect(code).toMatch(/run\(\);/);
    expect(code).toMatch(/keep\(\);/);
    expect(code.split("\n")).toHaveLength(source.split("\n").length);
  });

  it("does not let a comment stand in for a present anchor, and reports an absent one gained", () => {
    const source = planted({ "src/fix.ts": '// scrollIntoView() used to live here\nconst key = "Tab";\n' });
    expect(anchorRegressions([canary], source)).toEqual([
      `defect 0 (canary): src/fix.ts lost ${scroll.present}`,
      `defect 0 (canary): src/fix.ts gained ${tab.absent}`,
    ]);
  });

  it("lets a comment name what an absent anchor bans, as useKeyboardMap.ts's header may", () => {
    const source = planted({ "src/fix.ts": 'pane.scrollIntoView();\n// "Tab" belongs to the agent\n' });
    expect(anchorRegressions([canary], source)).toEqual([]);
  });

  it("reports a regression file that is missing, and one that holds no test", () => {
    const source = planted({ "src/empty.test.ts": 'describe("nothing yet", () => {});\n' });
    expect(testFileProblems([canary], source)).toEqual([
      "defect 0: src/empty.test.ts contains no tests",
      "defect 0: src/gone.test.ts does not exist",
    ]);
  });
});

/* ── the facts the UI contract predates ─────────────────────────────────────────────── */

/** Non-test source, `src/test/` included: a fixture that wrote `?? []` would be the same
 *  hidden regression one step removed. */
const SOURCES = Object.keys(SOURCE)
  .map((key) => key.replace(/^\//, ""))
  .filter((file) => !/\.test\.tsx?$/.test(file))
  .sort();

/** Task 8's fix round made the Go side allocate unconditionally and pinned it with a
 *  marshalled-bytes test, precisely because null.map() throws where [].map() returns [].
 *  A defensive `?? []` on this side is dead code that would hide a regression on the other side
 *  of the wire instead of reporting it. */
const DEFENSIVE = /\b(?:harnesses|models|efforts|permissionModes)\s*\?\?\s*\[\s*\]/g;
const defensiveFallbacks = (source: string): string[] =>
  [...source.matchAll(DEFENSIVE)].map((m) => m[0]);

describe("what Groups B and C settled after the UI contract was written", () => {
  it("treats the harness arrays as always present, because /v1/harnesses returns [] not null", () => {
    const offenders: string[] = [];
    for (const file of SOURCES) {
      for (const found of defensiveFallbacks(text(file))) {
        offenders.push(`${file}  ${found}`);
      }
    }
    expect(SOURCES.length).toBeGreaterThanOrEqual(20);
    expect(offenders).toEqual([]);
  });

  it("recognises that fallback on each of the four arrays, which the real tree never shows it", () => {
    // The canary for the test above: against a clean tree its regex could lose any name and stay
    // green, so each name is planted once here.
    for (const name of ["harnesses", "models", "efforts", "permissionModes"]) {
      expect(defensiveFallbacks(`const xs = h.${name} ?? [];`)).toEqual([`${name} ?? []`]);
    }
    expect(defensiveFallbacks("const xs = h.harnesses;")).toEqual([]);
  });

  it("declares eighteen error codes: contract-go §3.2's fifteen, ruling 7's sixteenth, and the client's two", () => {
    const union = /export type ProblemCode\s*=([\s\S]*?);/.exec(text("src/api/client.ts"));
    expect(union).not.toBeNull();
    const codes = [...(union?.[1] ?? "").matchAll(/"([a-z_]+)"/g)].map((m) => m[1]);
    expect(codes).toContain("non_loopback_publish");   // Group C's ruling 7
    expect(codes).toContain("network_unavailable");    // client-raised: nothing answered
    expect(codes).toContain("timeout");                // client-raised: timeoutMs elapsed
    expect(codes).toHaveLength(18);
    expect(new Set(codes).size).toBe(18);
  });

  it("can render only the four statuses the server can send", () => {
    // docs/AGENTIC-UX.md §4 rule 8 lists seven; contract §0 keeps only the mechanically
    // derived ones, because the rest need the agent-runtime signal that document itself calls
    // a hard runtime dependency. §0.1 rules `need` and `drift` out of the prototype's own
    // vocabulary for the same reason: neither can be derived without reading PTY bytes. A
    // fifth member here would be a status the cockpit can display and the control plane can
    // never produce.
    const union = /export type SessionStatus\s*=([\s\S]*?);/.exec(text("src/store/types.ts"));
    const members = [...(union?.[1] ?? "").matchAll(/"([a-z]+)"/g)].map((m) => m[1]).sort();
    expect(members).toEqual(["exited", "idle", "running", "starting"]);
  });
});
