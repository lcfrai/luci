// desktop/e2e/phase2.e2e.mjs
// Task 25. The headed end-to-end harness for Phase 2. It drives the cockpit's Vite dev
// server in Playwright's BUNDLED Chromium, against a real control plane and real Docker,
// and asserts on behaviour only: DOM state laid out by a real engine, the frames on the
// attach sockets, what the PTY reports about itself, and what the control plane logs.
//
// It takes no screenshot and judges no appearance. CLAUDE.md: "A screenshot is not
// evidence that something works", and visual acceptance "is the owner's eyes, not a
// self-report". What looks right is task 25's owner checklist, not this file.
//
// Run from desktop/ with the stack already up (task 25 step 6):   npm run e2e
// Prints one JSON line per check; exits 1 if any check failed.

import { execFileSync, spawn } from "node:child_process";
import { openSync, readFileSync } from "node:fs";
import { chromium } from "playwright";

const APP = process.env.PANACEA_E2E_APP ?? "http://localhost:1420";
const API = process.env.PANACEA_E2E_API ?? "http://127.0.0.1:8730";
const CDP = process.env.PANACEA_E2E_CDP ?? "";
/** The control plane's log file, for the checks that read the daemon's own account. */
const CP_LOG = process.env.PANACEA_E2E_CP_LOG ?? "";
const ONLY = new Set((process.env.PANACEA_E2E_ONLY ?? "").split(",").filter(Boolean));

// sandboxd/internal/wire/frame.go:15-18 — one kind byte, then the payload.
const STDOUT = 0x00;
const RESIZE = 0x02;
const EXIT = 0x03;

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/** Polls `probe` until it returns something truthy, and returns that. Throws naming
 *  `what` when `ms` runs out, so a timeout says which condition never arrived. */
async function until(what, probe, ms = 10_000, every = 100) {
  const deadline = Date.now() + ms;
  for (;;) {
    const value = await probe();
    if (value) return value;
    if (Date.now() > deadline) throw new Error(`timed out after ${ms} ms waiting for ${what}`);
    await sleep(every);
  }
}

/** The control plane, from Node. Node's fetch sends no Origin header, which
 *  OriginPolicy.Allow treats as a non-browser client (control-plane/internal/httpx/origin.go:118-120). */
async function api(method, path, body) {
  const res = await fetch(API + path, {
    method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status} ${text}`);
  return text === "" ? null : JSON.parse(text);
}

function cpLogLines(pattern) {
  if (CP_LOG === "") throw new Error("PANACEA_E2E_CP_LOG is not set; this check reads the daemon's log");
  return readFileSync(CP_LOG, "utf8").split("\n").filter((line) => pattern.test(line));
}

/** Every attach socket the page opens, with its traffic decoded. The page is the only
 *  client, so this list IS the set of attaches the cockpit made — which is how a check
 *  proves a socket did NOT re-open, not merely that one is open now. */
function watchSockets(page, sockets) {
  page.on("websocket", (ws) => {
    const m = /\/v1\/sessions\/([^/?]+)\/attach/.exec(ws.url());
    if (m === null) return;
    const rec = {
      sessionId: decodeURIComponent(m[1]),
      opened: Date.now(),
      closed: null,
      stdout: "",
      resizes: [],
      exited: false,
    };
    const decoder = new TextDecoder();
    const bytes = (payload) => (typeof payload === "string" ? Buffer.from(payload) : payload);
    ws.on("framereceived", ({ payload }) => {
      const b = bytes(payload);
      if (b[0] === STDOUT) rec.stdout += decoder.decode(b.subarray(1), { stream: true });
      if (b[0] === EXIT) rec.exited = true;
    });
    ws.on("framesent", ({ payload }) => {
      const b = bytes(payload);
      // ResizePayload: big-endian uint16 cols, then rows (frame.go:41-60).
      if (b[0] === RESIZE && b.length >= 5) {
        rec.resizes.push({ cols: b.readUInt16BE(1), rows: b.readUInt16BE(3), at: Date.now() });
      }
    });
    ws.on("close", () => { rec.closed = Date.now(); });
    sockets.push(rec);
  });
}

const paneOf = (id) => `[data-session-id="${id}"]`;
const socketsFor = (h, id) => h.sockets.filter((s) => s.sessionId === id);
const stdoutOf = (h, id) => socketsFor(h, id).map((s) => s.stdout).join("");

/** The rows xterm's DOM renderer has ON SCREEN for one session: the viewport, not the
 *  buffer. xterm renders a space as U+00A0, normalised so a pattern can say " ". */
async function screen(page, id) {
  return page.$$eval(`${paneOf(id)} .xterm-rows > div`, (els) =>
    els.map((el) => (el.textContent ?? "").replace(/ /g, " ").trimEnd()));
}

/** Types into one session's terminal the way a user does: click it, type, press Enter. */
async function type(page, id, command) {
  await page.click(`${paneOf(id)} .xterm`);
  await page.keyboard.type(command);
  await page.keyboard.press("Enter");
}

/** Asks the PTY its own size. The command text contains no digit after "SIZE=", so the
 *  shell's echo of the command can never satisfy the pattern — only the answer can. */
async function ptySize(h, id) {
  const before = stdoutOf(h, id).length;
  await type(h.page, id, "stty size | sed 's/^/SIZE=/'");
  const m = await until(`stty size from ${id}`, () =>
    /SIZE=(\d+) (\d+)/.exec(stdoutOf(h, id).slice(before)));
  return { rows: Number(m[1]), cols: Number(m[2]) };
}

/** Shared across checks, deliberately. Filled by the "setup" check. */
const ctx = { machineId: "", projectId: "", branchId: "", worktreeId: "", claude: "", sh: "", created: [] };

async function createShell(title) {
  const s = await api("POST", "/v1/sessions", {
    machineId: ctx.machineId,
    worktreeId: ctx.worktreeId,
    harness: "shell",
    title,
  });
  ctx.created.push(s.id);
  return s;
}

const CHECKS = [];
/** `closes` names the carry or row the check is evidence for, so step 19's table can be
 *  built from this script's output rather than from memory. `always` runs even under
 *  PANACEA_E2E_ONLY, because other checks depend on what it leaves behind. */
function check(id, closes, fn, { always = false } = {}) {
  CHECKS.push({ id, closes, fn, always });
}

async function main() {
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
  const page = await context.newPage();
  const h = { browser, context, page, sockets: [] };
  watchSockets(page, h.sockets);

  let failed = 0;
  for (const c of CHECKS) {
    if (ONLY.size > 0 && !ONLY.has(c.id) && !c.always) continue;
    const started = Date.now();
    try {
      const observed = await c.fn(h);
      console.log(JSON.stringify({ id: c.id, closes: c.closes, ok: true, ms: Date.now() - started, observed }));
    } catch (error) {
      failed += 1;
      console.log(JSON.stringify({ id: c.id, closes: c.closes, ok: false, ms: Date.now() - started, error: String(error?.message ?? error) }));
      if (c.always) break;   // nothing after a failed setup can mean anything
    }
  }

  // Kill every session this run made. DELETE destroys the sandbox and keeps the record
  // (docs/FUNCTIONAL-SPEC.md:36 — finished sessions stay listed); the registry is the
  // throwaway PANACEA_APP_STATE_DIR, so the records go with it.
  for (const id of ctx.created) await api("DELETE", `/v1/sessions/${id}`).catch(() => {});
  await browser.close();
  process.exit(failed === 0 ? 0 : 1);
}

check("setup", "task 24 step 11, re-run: the catalogue loads and a session can be created from the UI", async (h) => {
  const machines = (await api("GET", "/v1/machines")).machines;
  const project = (await api("GET", "/v1/projects")).projects.find((p) => p.name === "e2e-demo");
  if (project === undefined) throw new Error("the fixture project is not enumerated; check PANACEA_APP_PROJECTS_DIR");
  const branch = (await api("GET", `/v1/projects/${project.id}/branches`)).branches.find((b) => b.name === "feat/e2e");
  const worktree = (await api("GET", `/v1/projects/${project.id}/worktrees?branchId=${branch.id}`)).worktrees[0];
  Object.assign(ctx, { machineId: machines[0].id, projectId: project.id, branchId: branch.id, worktreeId: worktree.id });

  // The user's path: open the project, pick the branch, press the split button's left half.
  await h.page.goto(APP);
  await h.page.click(`[data-kind="project"][data-id="${project.id}"]`);
  await h.page.click(`[data-kind="branch"][data-id="${branch.id}"]`);
  const known = new Set((await api("GET", "/v1/sessions")).sessions.map((s) => s.id));
  await until("the New session button to enable", () => h.page.isEnabled(".top-split-main"));
  await h.page.click(".top-split-main");
  const made = await until("the UI-created session to reach the control plane", async () =>
    (await api("GET", "/v1/sessions")).sessions.find((s) => !known.has(s.id)));
  ctx.claude = made.id;
  ctx.created.push(made.id);
  await until("a pane for the UI-created session", () => h.page.$(paneOf(ctx.claude)));
  await until("an attach socket for it", () => socketsFor(h, ctx.claude).length > 0, 30_000);
  // A record is not a started session. Task 25's first run passed every line above for a UI
  // create whose sandbox never started: the record appeared, then "the sandbox could not be
  // created" (docker start: context canceled — the cockpit's 10 s request timeout fired mid-create).
  // p1.claude and o2.resume then failed for a reason this check had already seen. So the session
  // must leave "starting" alive before anything is built on it.
  const started = await until("the UI-created session to leave starting", async () => {
    const s = await api("GET", `/v1/sessions/${ctx.claude}`);
    return s.status === "starting" ? null : s;
  }, 60_000, 500);
  if (started.status === "exited") throw new Error(`the UI-created session exited: ${started.exitReason}`);

  // The shell every later check types into, made over REST: the poller must find it.
  ctx.sh = (await createShell("e2e-shell")).id;
  await until("the poller to render the REST-created session", () => h.page.$(paneOf(ctx.sh)), 10_000);
  await until("the shell's prompt", () => /\$ $/m.test(stdoutOf(h, ctx.sh)), 30_000);
  return { harness: made.harness, claude: ctx.claude, shell: ctx.sh };
}, { always: true });

check("p1.fidelity", "phase-1 rows: ANSI, box drawing, cursor control, exit status, 500-line burst, Ctrl-C — through the relay", async (h) => {
  const id = ctx.sh;
  const since = () => stdoutOf(h, id).length;

  // ANSI colour: xterm's DOM renderer puts palette colour 1 on a span as class xterm-fg-1.
  await type(h.page, id, "printf '\\033[31mRED%s\\033[0m\\n' OK");
  await until("a red span reading REDOK", () => h.page.$$eval(`${paneOf(id)} .xterm-rows span`, (spans) =>
    spans.some((s) => /\bxterm-fg-1\b/.test(s.className) && (s.textContent ?? "").includes("REDOK"))));

  // Box drawing, typed as octal so the echo holds no box character: U+256D U+2500 U+256E.
  await type(h.page, id, "printf '\\342\\225\\255\\342\\224\\200\\342\\225\\256\\n'");
  await until("╭─╮ on screen", async () => (await screen(h.page, id)).some((r) => r.includes("╭─╮")));

  // Cursor control: clear and home, then print. Row 0 can only read TOPMARK if both worked.
  await type(h.page, id, "printf '\\033[2J\\033[HTOPMARK\\n'");
  await until("TOPMARK on row 0", async () => (await screen(h.page, id))[0] === "TOPMARK");

  // Exit status propagates.
  let from = since();
  await type(h.page, id, "false; echo \"R\"\"C=$?\"");
  await until("RC=1", () => stdoutOf(h, id).slice(from).includes("RC=1"));

  // 500 lines, every one, in order, as the PTY emits them (onlcr: \r\n).
  from = since();
  await type(h.page, id, "seq -f 'L%g' 1 500");
  const expected = Array.from({ length: 500 }, (_, i) => `L${i + 1}`).join("\r\n");
  await until("all 500 lines in order", () => stdoutOf(h, id).slice(from).includes(expected), 15_000);

  // Ctrl-C reaches the process and the shell survives it.
  await type(h.page, id, "sleep 30");
  await sleep(500);
  await h.page.keyboard.press("Control+C");
  from = since();
  await type(h.page, id, "echo \"ALI\"\"VE\"");
  await until("ALIVE after Ctrl-C", () => stdoutOf(h, id).slice(from).includes("ALIVE"), 5_000);
  return "six rows, all through browser -> control plane -> sandboxd -> PTY";
});

check("p1.claude", "phase-1 rows: Claude Code runs through the PTY, and its help renders", async (h) => {
  const from = stdoutOf(h, ctx.sh).length;
  await type(h.page, ctx.sh, "claude --help | head -3");
  await until("claude's usage line", () => /Usage: claude/.test(stdoutOf(h, ctx.sh).slice(from)), 15_000);
  // The UI-created session runs claude itself. Whatever its first screen is — login, theme
  // choice — it is a TUI, and a TUI draws with box characters (U+2500-U+257F).
  const rows = await until("claude's first screen to draw", async () => {
    const r = await screen(h.page, ctx.claude);
    return r.some((line) => /[─-╿]/.test(line)) ? r : null;
  }, 45_000);
  return { firstRows: rows.filter((r) => r.trim() !== "").slice(0, 6) };
});

check("c2.cols", "real-browser carry 2: mount after append yields a non-zero column count", async (h) => {
  const pty = await ptySize(h, ctx.sh);
  const sent = socketsFor(h, ctx.sh).flatMap((s) => s.resizes);
  const last = sent.at(-1);
  if (last === undefined) throw new Error("no resize frame was ever sent for the shell");
  if (pty.cols < 20 || pty.rows < 5) throw new Error(`the PTY reports ${pty.rows}x${pty.cols}`);
  if (last.cols !== pty.cols || last.rows !== pty.rows) {
    throw new Error(`last resize frame ${last.rows}x${last.cols} but the PTY reports ${pty.rows}x${pty.cols}`);
  }
  const box = await h.page.$eval(`${paneOf(ctx.sh)} .pane-body`, (el) => {
    const r = el.getBoundingClientRect();
    return { width: Math.round(r.width), height: Math.round(r.height) };
  });
  return { pty, resizeFrames: sent.length, paneBody: box };
});

check("c3.theme", "real-browser carry 3: themeFromTokens() produces real colours", async (h) =>
  h.page.evaluate((sel) => {
    // Normalise any CSS colour to RGBA bytes by painting it. An unparseable value leaves
    // the sentinel in place, so "the token did not parse" is distinguishable from "black".
    const g = Object.assign(document.createElement("canvas"), { width: 1, height: 1 }).getContext("2d");
    const rgba = (css) => {
      g.clearRect(0, 0, 1, 1);
      g.fillStyle = "rgb(1, 2, 3)";
      g.fillStyle = css;
      g.fillRect(0, 0, 1, 1);
      return Array.from(g.getImageData(0, 0, 1, 1).data).join(",");
    };
    const SENTINEL = "1,2,3,255";
    const root = getComputedStyle(document.documentElement);
    const page = rgba(root.getPropertyValue("--page").trim());
    const ink = rgba(root.getPropertyValue("--ink").trim());
    if (page === SENTINEL || ink === SENTINEL) throw new Error(`a token did not parse: --page ${page}, --ink ${ink}`);

    const term = document.querySelector(`${sel} .xterm`);
    if (term === null) throw new Error("no .xterm in the pane");
    const painted = [term, ...term.querySelectorAll("*")]
      .map((el) => rgba(getComputedStyle(el).backgroundColor))
      .filter((c) => !c.endsWith(",0"));
    const rows = term.querySelector(".xterm-rows");
    const fg = rows === null ? "" : rgba(getComputedStyle(rows).color);
    // xterm's own defaults are black on white-ish; either one here means the theme was dropped.
    if (!painted.includes(page)) throw new Error(`no terminal surface is painted --page (${page}); saw ${[...new Set(painted)]}`);
    if (fg !== ink) throw new Error(`.xterm-rows is ${fg}, --ink is ${ink}`);
    return { page, ink, surfacesPaintedPage: painted.filter((c) => c === page).length };
  }, paneOf(ctx.sh)));

check("c7.popover", "real-browser carry 7: Popover's viewport clamp, including the flip-above branch", async (h) => {
  const results = [];
  for (const height of [1000, 560]) {
    await h.page.setViewportSize({ width: 1600, height });
    await h.page.click(".rail-machine");
    const pop = await until("the machine popover", () => h.page.$(".pop"));
    const r = await pop.evaluate((el) => {
      const b = el.getBoundingClientRect();
      const chip = document.querySelector(".rail-machine").getBoundingClientRect();
      return { top: b.top, bottom: b.bottom, left: b.left, right: b.right,
               vw: innerWidth, vh: innerHeight, above: b.bottom <= chip.top + 1 };
    });
    await h.page.keyboard.press("Escape");   // hooks/useDismissable owns Escape (AppShell.tsx:78-79)
    await until("the popover to close", async () => (await h.page.$(".pop")) === null);
    if (r.top < 0 || r.left < 0 || r.bottom > r.vh || r.right > r.vw) {
      throw new Error(`at ${height}px the popover spills the viewport: ${JSON.stringify(r)}`);
    }
    results.push({ height, ...r });
  }
  await h.page.setViewportSize({ width: 1600, height: 1000 });

  // The flip-above branch, which is what carry 7 is about. Task 25's first run found the plan's
  // premise wrong: `.rail-machine` is the rail's FIRST row (top 0, bottom 40.8 at both heights),
  // not its footer, so the chip can never flip — there is no room above it. The context menu is
  // the Popover that anchors to a POINT (ContextMenu.tsx: a zero-size DOMRect at the pointer), so
  // a right-click low on the rail's lowest row, in a window too short for the menu to fit below
  // that point, is a gesture that must take the flip. The height is computed from the menu's
  // measured size, not guessed; if no row sits low enough for the flip to be possible at all,
  // that is said, and the check fails rather than passing on the no-op path.
  const row = await h.page.$$eval("[data-kind]", (els) => {
    const b = els.at(-1).getBoundingClientRect();
    return { kind: els.at(-1).getAttribute("data-kind"), id: els.at(-1).getAttribute("data-id"), x: b.x + b.width / 2, y: b.bottom - 2 };
  });
  const openMenu = async () => {
    await h.page.mouse.click(row.x, row.y, { button: "right" });
    const pop = await until("the context menu", () => h.page.$(".pop"));
    const r = await pop.evaluate((el) => { const b = el.getBoundingClientRect(); return { top: b.top, bottom: b.bottom, height: b.height, vh: innerHeight }; });
    await h.page.keyboard.press("Escape");
    await until("the context menu to close", async () => (await h.page.$(".pop")) === null);
    return r;
  };
  const tall = await openMenu();
  if (row.y - tall.height - 6 <= 8) throw new Error(`no rail row sits low enough to flip: lowest anchor y ${row.y}, menu ${tall.height}px`);
  const short = Math.floor(row.y + 6 + tall.height);   // the menu cannot fit below the point
  await h.page.setViewportSize({ width: 1600, height: short });
  const flipped = await openMenu();
  await h.page.setViewportSize({ width: 1600, height: 1000 });
  if (flipped.bottom > row.y + 0.5) throw new Error(`at ${short}px the menu did not open above the point: ${JSON.stringify({ row, flipped })}`);
  if (flipped.top < 0 || flipped.bottom > flipped.vh) throw new Error(`at ${short}px the flipped menu spills the viewport: ${JSON.stringify(flipped)}`);
  results.push({ contextMenu: row.kind, anchorY: row.y, height: short, ...flipped, above: true });
  return results;
});

check("c9.exit-row", "real-browser carry 9: the exit row is zero-height until exit, and the pane does not move", async (h) => {
  const s = await createShell("e2e-exit");
  await until("the exit session's pane", () => h.page.$(paneOf(s.id)), 10_000);
  await until("its prompt", () => /\$ $/m.test(stdoutOf(h, s.id)), 30_000);
  // The exit bar is measured as its own element, not as "the third track". Task 25's first run
  // found the plan's premise wrong twice over: `.pane-exit` is rendered only once the session
  // has exited (SessionPane.tsx), so a running pane's third track holds whatever child is third,
  // and on a linked worktree that is the git note (p.pane-gitnote, 48px) — the resolved tracks
  // read "38px 809.76px 48px 25.3958px" with header, body, note and footer as the children.
  // What carry 9 asks is unchanged: no space is kept for the bar while running, and its
  // arrival moves nothing. So: no track is sized that no rendered child fills, then a bar with
  // a real height, then the pane's own box where it was.
  const measure = () => h.page.$eval(paneOf(s.id), (el) => {
    const b = el.getBoundingClientRect();
    const bar = el.querySelector(".pane-exit");
    const rows = getComputedStyle(el).gridTemplateRows;
    return { rows, top: b.top, height: b.height, children: el.children.length,
             sizedTracks: rows.split(" ").filter((t) => t !== "0px").length,
             exitBar: bar !== null, exitBarPx: bar === null ? 0 : bar.getBoundingClientRect().height };
  });
  const before = await measure();
  if (before.exitBar) throw new Error("a running session already shows the exit bar");
  if (before.sizedTracks > before.children) throw new Error(`a running pane sizes a track no child fills: ${before.rows}, ${before.children} children`);

  await type(h.page, s.id, "exit");
  await until("the exit bar", async () => (await measure()).exitBar, 10_000);
  const after = await measure();
  if (after.exitBarPx <= 0) throw new Error(`exit bar present but ${after.exitBarPx}px tall (${after.rows})`);
  if (Math.abs(after.top - before.top) > 0.5 || Math.abs(after.height - before.height) > 0.5) {
    throw new Error(`the pane moved: ${JSON.stringify({ before, after })}`);
  }
  return { before: before.rows, after: after.rows, exitBarPx: after.exitBarPx };
});

const GRACE_MS = 5_000;   // src/api/attachRegistry.ts:20, guarded in step 1 (b)

async function flipTo(h, label) {
  await h.page.click(`[role="group"][aria-label="Layout"] button:has-text("${label}")`);
  await until(`the ${label} layout`, () =>
    h.page.$(`[role="group"][aria-label="Layout"] button[aria-pressed="true"]:has-text("${label}")`));
}
const tagTerm = (h, id) => h.page.$eval(`${paneOf(id)} .xterm`, (el, v) => { el.__e2eTag = v; }, id);
const tagOf = (h, id) => h.page.$eval(`${paneOf(id)} .xterm`, (el) => el.__e2eTag ?? null);
const relayEnds = () => cpLogLines(/attach relay ended/).length;
const liveIds = async () =>
  (await api("GET", "/v1/sessions")).sessions
    .filter((s) => ctx.created.includes(s.id) && s.status !== "exited").map((s) => s.id);

/** Every row the terminal can show, from the top of its scrollback to the bottom, read by
 *  scrolling the way a user does. Leaves the terminal scrolled to the bottom.
 *
 *  One wheel event moves xterm a few rows WHATEVER its delta: measured by task 25 in both
 *  layouts, a single -1,000,000 px event and a -300 px one each moved exactly five rows
 *  (Columns W349 -> W344 -> W339, Canvas W391 -> W386 -> W381). So the top is reached by
 *  repeating the gesture until the first row stops changing, and the way down reads the
 *  viewport after rows/ROWS_PER_WHEEL_MAX events: half a viewport at the measured five rows
 *  an event, a whole one even at twice that, so no row can be scrolled past unread. */
const ROWS_PER_WHEEL_MAX = 10;   // twice the five measured, as margin
async function readBuffer(h, id) {
  const box = await h.page.$eval(`${paneOf(id)} .xterm`, (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  await h.page.mouse.move(box.x, box.y);
  let top = null;
  for (let i = 0; i < 1_000; i += 1) {
    for (let k = 0; k < 20; k += 1) await h.page.mouse.wheel(0, -300);
    await sleep(50);
    const first = (await screen(h.page, id))[0] ?? "";
    if (first === top) break;   // twenty more events moved nothing: this is the top
    top = first;
  }
  const seen = new Set();
  let previous = "";
  for (let i = 0; i < 5_000; i += 1) {
    const rows = await screen(h.page, id);
    for (const r of rows) seen.add(r);
    const key = rows.join("\n");
    if (key === previous) break;   // the viewport stopped moving: this is the bottom
    previous = key;
    const steps = Math.max(1, Math.floor(rows.length / ROWS_PER_WHEEL_MAX));
    for (let k = 0; k < steps; k += 1) await h.page.mouse.wheel(0, 300);
    await sleep(25);
  }
  return seen;
}

function missingMarks(seen, prefix, n) {
  const missing = [];
  for (let i = 1; i <= n; i += 1) if (!seen.has(`${prefix}${i}`)) missing.push(i);
  return missing;
}

check("c1.flip", "real-browser carry 1: scrollback survives Columns <-> Canvas for a session both render", async (h) => {
  await flipTo(h, "Columns");
  const from = stdoutOf(h, ctx.sh).length;
  await type(h.page, ctx.sh, "seq -f 'FLIP%g' 1 3000");
  await until("FLIP3000", () => stdoutOf(h, ctx.sh).slice(from).includes("FLIP3000\r\n"), 15_000);

  const live = await liveIds();
  for (const id of live) await tagTerm(h, id);
  const socketsBefore = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));
  const endsBefore = relayEnds();
  const log = [];

  for (const label of ["Canvas", "Columns"]) {
    await flipTo(h, label);
    await sleep(2 * GRACE_MS);
    for (const id of live) {
      const tag = await tagOf(h, id);
      if (tag !== id) throw new Error(`${label}: ${id}'s terminal is a different DOM node (tag ${tag})`);
      const opened = socketsFor(h, id).length - socketsBefore[id];
      if (opened !== 0) throw new Error(`${label}: ${id} opened ${opened} new attach socket(s)`);
      if (socketsFor(h, id).at(-1).closed !== null) throw new Error(`${label}: ${id}'s socket closed`);
    }
    const ends = relayEnds() - endsBefore;
    if (ends !== 0) throw new Error(`${label}: the control plane logged ${ends} relay end(s)`);
    const missing = missingMarks(await readBuffer(h, ctx.sh), "FLIP", 3000);
    if (missing.length > 0) throw new Error(`${label}: ${missing.length} FLIP lines not rendered, first ${missing[0]}`);
    log.push({ layout: label, liveSessions: live.length, newSockets: 0, relayEnds: 0, flipLinesRendered: 3000 });
  }
  return log;
});

check("c1.cap", "real-browser carry 1, the boundary: a session dropped by the Canvas cap is re-attached, not preserved", async (h) => {
  await flipTo(h, "Columns");
  for (let i = 1; i <= 9; i += 1) await createShell(`e2e-cap-${i}`);
  const live = await until("twelve sessions on the branch, all live but one", async () => {
    const ids = await liveIds();
    return ids.length >= 11 && ids.every((id) => socketsFor(h, id).length > 0) ? ids : null;
  }, 60_000);
  for (const id of live) await tagTerm(h, id);
  const socketsBefore = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));

  const flippedAt = Date.now();
  await flipTo(h, "Canvas");
  const tiles = await h.page.$$eval("[data-cv-tile]", (els) => els.map((el) => el.getAttribute("data-cv-tile")));
  if (tiles.length > 10) throw new Error(`Canvas rendered ${tiles.length} tiles; the cap is 10`);
  const dropped = live.filter((id) => !tiles.includes(id));
  const kept = live.filter((id) => tiles.includes(id));
  if (dropped.length === 0) throw new Error("nothing was dropped; the boundary was not exercised");

  await sleep(GRACE_MS + 3_000);
  for (const id of dropped) {
    const closed = socketsFor(h, id).at(-1).closed;
    if (closed === null) throw new Error(`dropped ${id} still has an open socket after the grace`);
    if (closed - flippedAt < GRACE_MS - 250) throw new Error(`dropped ${id} closed ${closed - flippedAt} ms after the flip, inside the grace`);
  }
  for (const id of kept) {
    if (socketsFor(h, id).length !== socketsBefore[id]) throw new Error(`kept ${id} re-attached`);
    if ((await tagOf(h, id)) !== id) throw new Error(`kept ${id} is a different DOM node`);
  }

  await flipTo(h, "Columns");
  for (const id of dropped) {
    await until(`a fresh socket for dropped ${id}`, () => socketsFor(h, id).length === socketsBefore[id] + 1, 15_000);
    if ((await tagOf(h, id)) !== null) throw new Error(`dropped ${id} kept its old DOM node; it should be a fresh terminal`);
  }
  return { live: live.length, tiles: tiles.length, dropped: dropped.length, droppedGraceMs: GRACE_MS };
});

check("c5.resize", "real-browser carry 5: a tile resize reflows the terminal, and the PTY is told", async (h) => {
  await flipTo(h, "Canvas");
  const tile = `[data-cv-tile="${ctx.sh}"]`;
  if ((await h.page.$(tile)) === null) throw new Error("the shell has no Canvas tile; c1.flip should have pinned it");
  const before = await ptySize(h, ctx.sh);
  const framesBefore = socketsFor(h, ctx.sh).flatMap((s) => s.resizes).length;
  const boxBefore = await h.page.$eval(tile, (el) => el.getBoundingClientRect().toJSON());

  const grip = await h.page.$eval(`${tile} .cv-resize`, (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  await h.page.mouse.move(grip.x, grip.y);
  await h.page.mouse.down();
  await h.page.mouse.move(grip.x + 240, grip.y + 160, { steps: 10 });
  await h.page.mouse.up();

  const boxAfter = await h.page.$eval(tile, (el) => el.getBoundingClientRect().toJSON());
  if (boxAfter.width <= boxBefore.width || boxAfter.height <= boxBefore.height) {
    throw new Error(`the tile did not grow: ${JSON.stringify({ boxBefore, boxAfter })}`);
  }
  const frame = await until("a resize frame after the drag", () => {
    const all = socketsFor(h, ctx.sh).flatMap((s) => s.resizes);
    return all.length > framesBefore ? all.at(-1) : null;
  }, 5_000);
  const after = await ptySize(h, ctx.sh);
  if (after.cols <= before.cols || after.rows <= before.rows) {
    throw new Error(`the PTY did not grow: ${before.rows}x${before.cols} -> ${after.rows}x${after.cols}`);
  }
  if (frame.cols !== after.cols || frame.rows !== after.rows) {
    throw new Error(`frame ${frame.rows}x${frame.cols} disagrees with the PTY's ${after.rows}x${after.cols}`);
  }
  ctx.resizedTile = { id: ctx.sh, width: boxAfter.width, height: boxAfter.height };
  return { before, after, tilePx: { from: [boxBefore.width, boxBefore.height], to: [boxAfter.width, boxAfter.height] } };
});

check("c4.pinned", "real-browser carry 4: the pinned-scroll rule holds against real xterm's asynchronous parse", async (h) => {
  await flipTo(h, "Columns");
  const id = ctx.sh;
  await type(h.page, id, "while :; do echo \"TI\"\"CK $RANDOM\"; sleep 0.05; done");
  await until("ticks arriving", () => /TICK \d+/.test(stdoutOf(h, id)), 5_000);
  const centre = await h.page.$eval(`${paneOf(id)} .xterm`, (el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  });
  await h.page.mouse.move(centre.x, centre.y);

  // Scrolled up: sample the viewport for three seconds of arriving output. It must not move.
  await h.page.mouse.wheel(0, -900);
  await sleep(300);
  const grown = stdoutOf(h, id).length;
  const pinned = await screen(h.page, id);
  for (let i = 0; i < 30; i += 1) {
    await sleep(100);
    const now = await screen(h.page, id);
    if (now.join("\n") !== pinned.join("\n")) throw new Error(`scrolled up, the viewport moved at sample ${i}`);
  }
  if (stdoutOf(h, id).length <= grown) throw new Error("no output arrived while pinned; the check proved nothing");

  // At the bottom: the last tick on screen must keep changing.
  await h.page.mouse.wheel(0, 1_000_000);
  await sleep(300);
  const lastTick = async () => (await screen(h.page, id)).filter((r) => /TICK \d+/.test(r)).at(-1);
  const first = await lastTick();
  await until("the viewport to follow new output", async () => (await lastTick()) !== first, 3_000);

  await h.page.keyboard.press("Control+C");
  return { pinnedSamples: 30, followed: true };
});

check("c6.load", "real-browser carry 6: 4,000 lines/s for 10 s — usability asserted, render rate measured", async (h) => {
  const id = ctx.sh;
  const other = ctx.created.find((x) => x !== id && x !== ctx.claude && socketsFor(h, x).at(-1)?.closed === null);
  if (other === undefined) throw new Error("no second live shell to echo into");

  await h.page.evaluate((sel) => {
    const rows = document.querySelector(`${sel} .xterm-rows`);
    window.__e2eLoad = { mutations: 0, longest: 0, over200: 0 };
    new MutationObserver(() => { window.__e2eLoad.mutations += 1; })
      .observe(rows, { childList: true, subtree: true, characterData: true });
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        window.__e2eLoad.longest = Math.max(window.__e2eLoad.longest, e.duration);
        if (e.duration > 200) window.__e2eLoad.over200 += 1;
      }
    }).observe({ type: "longtask" });
  }, paneOf(id));

  const started = Date.now();
  await type(h.page, id, "awk 'BEGIN{for(i=1;i<=40000;i++){print \"RATE\" i; if(i%400==0) system(\"sleep 0.1\")}}'");
  await sleep(4_000);   // mid-burst

  const echoFrom = stdoutOf(h, other).length;
  const typed = Date.now();
  await type(h.page, other, "echo \"PI\"\"NG\"");
  await until("the second pane's echo during the burst", () => stdoutOf(h, other).slice(echoFrom).includes("PING"), 2_000);
  const echoMs = Date.now() - typed;

  await until("RATE40000 on screen", async () => (await screen(h.page, id)).some((r) => r === "RATE40000"), 60_000);
  const seconds = (Date.now() - started) / 1_000;
  const load = await h.page.evaluate(() => window.__e2eLoad);
  return {
    burstSeconds: Number(seconds.toFixed(1)),
    rowMutationsPerSecond: Number((load.mutations / seconds).toFixed(1)),
    longestTaskMs: Math.round(load.longest),
    tasksOver200ms: load.over200,
    secondPaneEchoMs: echoMs,
  };
});

const CP_BIN = process.env.PANACEA_E2E_CP_BIN ?? "";
let cpPid = Number(process.env.PANACEA_E2E_CP_PID ?? "0");

const healthy = async () => { try { await api("GET", "/v1/healthz"); return true; } catch { return false; } };

async function stopControlPlane() {
  if (CP_BIN === "" || cpPid === 0) throw new Error("PANACEA_E2E_CP_BIN and PANACEA_E2E_CP_PID are required");
  execFileSync("taskkill", ["/PID", String(cpPid), "/F"]);
  await until("the control plane to stop answering", async () => !(await healthy()), 10_000);
}

async function startControlPlane() {
  const log = openSync(CP_LOG, "a");
  const child = spawn(CP_BIN, [], { detached: true, stdio: ["ignore", log, log], env: process.env });
  child.unref();
  cpPid = child.pid;
  await until("the control plane to answer again", healthy, 20_000);
  return cpPid;
}

const containerOf = (sessionId) =>
  execFileSync("docker", ["ps", "-q", "--filter", `label=panacea-app.session-id=${sessionId}`], { encoding: "utf8" }).trim();

check("d.detach", "durability: a pane unmounted past the grace re-attaches, and the replay carries the transcript", async (h) => {
  await flipTo(h, "Columns");
  const id = ctx.sh;
  await type(h.page, id, "echo \"DETACH\"\"MARK\"");
  await until("DETACHMARK", () => stdoutOf(h, id).includes("DETACHMARK"));
  const socketsBefore = socketsFor(h, id).length;

  const main = (await api("GET", `/v1/projects/${ctx.projectId}/branches`)).branches.find((b) => b.name === "main");
  await h.page.click(`[data-kind="branch"][data-id="${main.id}"]`);
  await sleep(GRACE_MS + 2_000);
  if (socketsFor(h, id).at(-1).closed === null) throw new Error("the socket outlived the grace with no pane to show it");
  const status = (await api("GET", `/v1/sessions/${id}`)).status;
  if (status === "exited") throw new Error("detaching ended the session; disconnect must not destroy (master plan :60)");

  await h.page.click(`[data-kind="branch"][data-id="${ctx.branchId}"]`);
  const fresh = await until("a new socket on return", () =>
    socketsFor(h, id).length > socketsBefore ? socketsFor(h, id).at(-1) : null, 15_000);
  await until("the replay to carry DETACHMARK", () => fresh.stdout.includes("DETACHMARK"), 10_000);
  if (!(await readBuffer(h, id)).has("DETACHMARK")) throw new Error("replayed, but DETACHMARK is not rendered");
  return { statusWhileDetached: status, replayBytes: fresh.stdout.length };
});

check("d.reload", "durability: the cockpit restarts and every live session re-attaches; known defect 6 in a real window", async (h) => {
  const live = await liveIds();
  const before = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));
  const layoutBefore = await h.page.$eval(`[role="group"][aria-label="Layout"] button[aria-pressed="true"]`, (b) => b.textContent);
  await h.page.reload();
  // No click: selection, layout, tiles and pinned are persisted (store.ts:29-42).
  for (const id of live) {
    await until(`${id} to re-attach after the reload`, () => socketsFor(h, id).length > before[id], 20_000);
  }
  const layoutAfter = await h.page.$eval(`[role="group"][aria-label="Layout"] button[aria-pressed="true"]`, (b) => b.textContent);
  if (layoutAfter !== layoutBefore) throw new Error(`layout ${layoutBefore} came back as ${layoutAfter}`);
  let tile = null;
  if (ctx.resizedTile !== undefined) {
    await flipTo(h, "Canvas");
    tile = await h.page.$eval(`[data-cv-tile="${ctx.resizedTile.id}"]`, (el) => el.getBoundingClientRect().toJSON());
    if (Math.abs(tile.width - ctx.resizedTile.width) > 1 || Math.abs(tile.height - ctx.resizedTile.height) > 1) {
      throw new Error(`defect 6: the resized tile came back ${tile.width}x${tile.height}, was ${ctx.resizedTile.width}x${ctx.resizedTile.height}`);
    }
    await flipTo(h, layoutBefore);
  }
  return { reattached: live.length, layout: layoutAfter, tileKeptSize: tile !== null };
});

check("d.cp-restart", "durability: the control plane restarts; sessions and containers survive; the cockpit re-attaches unaided", async (h) => {
  const live = await liveIds();
  const containers = Object.fromEntries(live.map((id) => [id, containerOf(id)]));
  const before = Object.fromEntries(live.map((id) => [id, socketsFor(h, id).length]));
  await stopControlPlane();
  await sleep(10_000);   // well inside the ladder's 60 s budget
  const pid = await startControlPlane();

  const listed = (await api("GET", "/v1/sessions")).sessions;
  for (const id of live) {
    const s = listed.find((x) => x.id === id);
    if (s === undefined) throw new Error(`${id} is missing after the restart`);
    if (s.status === "exited") throw new Error(`${id} came back exited`);
    if (containerOf(id) !== containers[id]) throw new Error(`${id}'s container changed across the restart`);
  }
  for (const id of live) {
    await until(`${id} to re-attach unaided`, () => socketsFor(h, id).length > before[id], 60_000);
  }
  return { sessions: live.length, containersUnchanged: true, newControlPlanePid: pid };
});

check("c10.retry", "real-browser carry 10 + carry 9's overlay: Retry attach after a genuine failure; the strip lets the pointer through", async (h) => {
  await flipTo(h, "Columns");
  const id = ctx.sh;
  await type(h.page, id, "echo \"RETRY\"\"MARK\"");
  await until("RETRYMARK", () => stdoutOf(h, id).includes("RETRYMARK"));
  await tagTerm(h, id);
  const socketsBefore = socketsFor(h, id).length;

  await stopControlPlane();
  const failedAt = await until("the pane to report failed", () =>
    h.page.$(`${paneOf(id)} [data-attach-phase="failed"]`), 120_000, 500);

  const hit = await failedAt.evaluate((strip) => {
    const r = strip.getBoundingClientRect();
    const under = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
    return { corner: strip.classList.contains("pane-attach--corner"),
             inTerminal: under?.closest(".xterm") !== null, onStrip: under?.closest(".pane-attach") !== null };
  });
  if (!hit.corner) throw new Error("a failed pane is not showing the corner presentation");
  if (!hit.inTerminal || hit.onStrip) throw new Error(`the strip takes the pointer: ${JSON.stringify(hit)}`);

  await startControlPlane();
  await sleep(3_000);   // failed is terminal: nothing reconnects on its own (open-carries :30-31)
  if (socketsFor(h, id).length !== socketsBefore) throw new Error("the pane reconnected without Retry; failed was not terminal");

  await h.page.click(`[data-session-menu-anchor="${id}"]`);
  await h.page.getByRole("menuitem", { name: "Retry attach" }).click();
  await until("a new socket after Retry", () => socketsFor(h, id).length > socketsBefore, 15_000);
  await until("the overlay to clear", async () => (await h.page.$(`${paneOf(id)} [data-attach-phase="failed"]`)) === null, 15_000);
  if ((await tagOf(h, id)) !== id) throw new Error("Retry rebuilt the terminal; the transcript did not survive it");
  if (!(await readBuffer(h, id)).has("RETRYMARK")) throw new Error("RETRYMARK is gone after Retry");
  const from = stdoutOf(h, id).length;
  await type(h.page, id, "echo \"AFTER\"\"RETRY\"");
  await until("stdin working on the new socket", () => stdoutOf(h, id).slice(from).includes("AFTERRETRY"));
  return { overlay: hit, sameTerminal: true };
});

/** The persisted part of Claude Code's store inside a sandbox, and ONLY that part: the resume
 *  fix mounts <state>/conversations/local/<worktree>/ here and nothing wider in ~/.claude
 *  (sessionConversationDir, control-plane/internal/api/sessions.go; the Dockerfile's
 *  `RUN mkdir -p /home/dev/.claude/projects`). A transcript is <this>/<cwd-slug>/<uuid>.jsonl,
 *  and the harness runs in /home/dev/work, so the slug is -home-dev-work. */
const CLAUDE_STORE = "/home/dev/.claude/projects";
/** Where the harness runs, so a seeded transcript lands in the same slug as the session's. */
const WORK = "/home/dev/work";
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function argvOf(cid) {
  const env = execFileSync("docker", ["inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", cid], { encoding: "utf8" });
  const m = /^PANACEA_APP_ARGS_JSON=(.*)$/m.exec(env);
  if (m === null) throw new Error(`container ${cid} carries no PANACEA_APP_ARGS_JSON`);
  return JSON.parse(m[1]);
}
const flagValue = (argv, flag) => { const i = argv.indexOf(flag); return i === -1 ? null : argv[i + 1]; };
const inSandbox = (cid, script) => execFileSync("docker", ["exec", "-w", WORK, cid, "sh", "-c", script], { encoding: "utf8" }).trim();
/** Runs `script` in the sandbox and returns its REAL exit code and output. The code goes
 *  through a file, never a pipe: `cmd | tail` reports tail's status (the master plan's :52). */
function runInSandbox(cid, script) {
  const tag = `/tmp/e2e-${Date.now()}`;
  inSandbox(cid, `( ${script} ) > ${tag}.out 2>&1; echo $? > ${tag}.rc; true`);
  return { code: Number(inSandbox(cid, `cat ${tag}.rc`)), out: inSandbox(cid, `cat ${tag}.out; rm -f ${tag}.out ${tag}.rc`) };
}
/** `file` may be a glob: sh (dash) does not expand one in a redirection, so it is an argument. */
const lineCount = (cid, file) => Number(inSandbox(cid, `cat ${file} | wc -l`));

check("o2.resume", "open carry 2: Resume forks the same Claude conversation, from its persisted transcript, in a new sandbox", async (h) => {
  const source = ctx.claude;
  const sourceCid = containerOf(source);
  if (sourceCid === "") throw new Error("the source session has no running sandbox to resume from");
  const minted = flagValue(argvOf(sourceCid), "--session-id");
  if (minted === null || !UUID.test(minted)) {
    throw new Error(`the source was not started with --session-id <uuid> (got ${minted}); the resume fix has not landed — step 1 (h)`);
  }

  // Seed a real conversation. A session nobody spoke in has no transcript, and the control plane
  // refuses to resume it (409 "Nothing has been said…", sessionResumableConversation). `-p`
  // writes the user turn before the API call, even when not signed in (resume-fix-report §3), so
  // its exit code is recorded, not asserted: unsigned, it exits 1 "Not logged in". THE MARKER IS
  // THE TRANSCRIPT <U>.jsonl — a real file in the real persisted store, standing in for nothing.
  const nonce = `e2e-${Date.now()}`;
  const seed = runInSandbox(sourceCid, `claude -p ${nonce} --session-id ${minted}`);
  const transcript = `${CLAUDE_STORE}/*/${minted}.jsonl`;
  const seeded = Number(inSandbox(sourceCid, `ls ${transcript} 2>/dev/null | wc -l`));
  if (seeded !== 1) {
    throw new Error(`claude -p did not write ${transcript} (found ${seeded}; exit ${seed.code}: ${seed.out.slice(0, 200)})`);
  }
  if (!inSandbox(sourceCid, `cat ${transcript}`).includes(nonce)) throw new Error(`${minted}.jsonl does not hold ${nonce}`);
  const sourceLines = lineCount(sourceCid, transcript);

  const before = await api("GET", `/v1/sessions/${source}`);
  await api("DELETE", `/v1/sessions/${source}`);
  await until("the source to read exited", async () => (await api("GET", `/v1/sessions/${source}`)).status === "exited");
  await until("the source sandbox to be destroyed", () => containerOf(source) === "", 30_000);
  const known = new Set((await api("GET", "/v1/sessions")).sessions.map((s) => s.id));

  // The user's path: the exited pane's own menu.
  await h.page.click(`[data-session-menu-anchor="${source}"]`);
  await h.page.getByRole("menuitem", { name: "Resume" }).click();
  const resumed = await until("the resumed session", async () =>
    (await api("GET", "/v1/sessions")).sessions.find((s) => !known.has(s.id)), 15_000);
  ctx.created.push(resumed.id);
  const cid = await until("the resumed sandbox", () => containerOf(resumed.id), 30_000);

  // A resume FORKS: --resume <U> --fork-session --session-id <V> (argv.go, claudeArgv). The CLI
  // accepts that pairing and no other, and V names the fork so nothing reads the CLI's output.
  const argv = argvOf(cid);
  const resumedWith = flagValue(argv, "--resume");
  if (resumedWith !== minted) {
    throw new Error(`the resumed sandbox runs --resume ${resumedWith}; the source's Claude session id is ${minted}`);
  }
  if (!argv.includes("--fork-session")) throw new Error(`the resume does not fork: ${JSON.stringify(argv)}`);
  const fork = flagValue(argv, "--session-id");
  if (fork === null || !UUID.test(fork)) throw new Error(`the fork is not named by a UUID --session-id (got ${fork})`);
  if (fork === minted) throw new Error(`the fork reuses the source's id ${minted}; the CLI refuses a --session-id already in use`);
  for (const k of ["harness", "model", "effort", "permissionMode", "worktreeId"]) {
    if (resumed[k] !== before[k]) throw new Error(`resumed ${k} is ${JSON.stringify(resumed[k])}; the source's was ${JSON.stringify(before[k])}`);
  }
  // The store survived: the new sandbox reads <U>.jsonl, and the source is untouched by the fork.
  const readable = runInSandbox(cid, `cat ${transcript} > /dev/null`);
  if (readable.code !== 0) throw new Error(`the resumed sandbox cannot read ${transcript}: exit ${readable.code} ${readable.out}`);
  const resumedLines = lineCount(cid, transcript);
  if (resumedLines !== sourceLines) throw new Error(`${minted}.jsonl has ${resumedLines} lines after the resume; it had ${sourceLines}`);
  // Recorded, not asserted: whether the fork's own transcript exists yet. The interactive CLI
  // writes <V>.jsonl only once something is said, and nobody signed in to say it.
  const forkFiles = Number(inSandbox(cid, `ls ${CLAUDE_STORE}/*/${fork}.jsonl 2>/dev/null | wc -l`));
  return { claudeSessionId: minted, resumedWith, forkedAs: fork, sourceLines, resumedLines,
           seedExit: seed.code, forkTranscriptFiles: forkFiles };
});

/** The identity step 6 exported. The explicit pair wins over the host's git (sandbox fix §4). */
const GIT_NAME = process.env.PANACEA_APP_GIT_NAME ?? "";
const GIT_EMAIL = process.env.PANACEA_APP_GIT_EMAIL ?? "";
const CA_SET = (process.env.PANACEA_APP_CA_BUNDLE ?? "") !== "";

check("g.primary", "sandbox git + CA fix: git works in a session on a primary worktree, as the configured identity", async (h) => {
  if (GIT_NAME === "" || GIT_EMAIL === "") throw new Error("PANACEA_APP_GIT_NAME/EMAIL are not exported in this shell — step 6");
  // ctx.worktreeId is feat/e2e's LINKED worktree; this check needs main's, the clone itself.
  const main = (await api("GET", `/v1/projects/${ctx.projectId}/branches`)).branches.find((b) => b.name === "main");
  const wt = (await api("GET", `/v1/projects/${ctx.projectId}/worktrees?branchId=${main.id}`)).worktrees[0];
  if (wt?.isPrimary !== true) throw new Error(`main's worktree is not primary: ${JSON.stringify(wt)}`);
  const s = await api("POST", "/v1/sessions", { machineId: ctx.machineId, worktreeId: wt.id, harness: "shell", title: "e2e-git" });
  ctx.created.push(s.id);
  const cid = await until("the git session's sandbox", () => containerOf(s.id), 30_000);

  // Preflight (h)'s mechanical signal, in a created sandbox: the fix's names in docker inspect.
  const env = execFileSync("docker", ["inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", cid], { encoding: "utf8" });
  const want = [/^GIT_CONFIG_COUNT=\d+$/m, new RegExp(`^GIT_AUTHOR_NAME=${GIT_NAME}$`, "m"), new RegExp(`^GIT_COMMITTER_EMAIL=${GIT_EMAIL.replace(/\./g, "\\.")}$`, "m")];
  for (const re of want) if (!re.test(env)) throw new Error(`the sandbox's env lacks ${re}`);
  if (/^NODE_EXTRA_CA_CERTS=\/etc\/panacea-app\/ca-bundle\.pem$/m.test(env) !== CA_SET) throw new Error(`NODE_EXTRA_CA_CERTS does not follow PANACEA_APP_CA_BUNDLE (set: ${CA_SET})`);
  if (/^(SSL_CERT_FILE|NODE_TLS_REJECT_UNAUTHORIZED)=/m.test(env)) throw new Error("the sandbox replaces or disables TLS verification");

  // Real exit codes, through a file (runInSandbox), never a pipe.
  const status = runInSandbox(cid, `git -C ${WORK} status`);
  if (status.code !== 0) throw new Error(`git status on the primary worktree exited ${status.code}: ${status.out}`);
  const scope = runInSandbox(cid, `git -C ${WORK} config --show-scope --get-all safe.directory`);
  if (scope.code !== 0 || !/^command\s.*\/home\/dev\/work$/m.test(scope.out)) throw new Error(`safe.directory is not command-scope ${WORK}: ${scope.out}`);
  const commit = runInSandbox(cid, `git -C ${WORK} commit --allow-empty -q -m "e2e: g.primary"`);
  if (commit.code !== 0) throw new Error(`a commit on the primary worktree exited ${commit.code}: ${commit.out}`);
  const who = inSandbox(cid, `git -C ${WORK} log -1 --format='%an <%ae> / %cn <%ce>'`);
  const expect = `${GIT_NAME} <${GIT_EMAIL}> / ${GIT_NAME} <${GIT_EMAIL}>`;
  if (who !== expect) throw new Error(`the commit's author/committer is ${who}; want ${expect}`);
  return { worktree: wt.id, statusExit: status.code, commitExit: commit.code, identity: who, caMounted: CA_SET };
});

check("w.webview2", "phase-1 unproven item 5, agent half: the native WebView2 window mounts a live terminal", async () => {
  if (CDP === "") return "skipped: PANACEA_E2E_CDP is unset";
  const native = await chromium.connectOverCDP(CDP);
  try {
    const pages = native.contexts().flatMap((c) => c.pages());
    const page = pages.find((p) => p.url().startsWith(APP));
    if (page === undefined) throw new Error(`no WebView2 page at ${APP}; saw ${pages.map((p) => p.url())}`);
    const engine = await page.evaluate(() => navigator.userAgent);
    await until("the project in the native rail", () => page.$(`[data-kind="project"][data-id="${ctx.projectId}"]`), 15_000);
    if ((await page.$(paneOf(ctx.sh))) === null) {
      await page.click(`[data-kind="project"][data-id="${ctx.projectId}"]`);
      await page.click(`[data-kind="branch"][data-id="${ctx.branchId}"]`);
    }
    await until("the shell's pane in the native window", () => page.$(paneOf(ctx.sh)), 15_000);
    await page.click(`${paneOf(ctx.sh)} .xterm`);
    await page.keyboard.type("stty size | sed 's/^/SIZE=/'");
    await page.keyboard.press("Enter");
    // Read off the native window's own screen: its sockets opened before this connection,
    // so watchSockets cannot see them. The PTY is shared, so the size is whichever client
    // resized last — only "non-zero" is asserted, which is carry 2's claim.
    const m = await until("stty size in the native window", async () => {
      for (const row of await screen(page, ctx.sh)) {
        const hit = /SIZE=(\d+) (\d+)/.exec(row);
        if (hit !== null) return hit;
      }
      return null;
    }, 10_000);
    if (Number(m[2]) === 0) throw new Error("the native window's PTY reports zero columns");
    return { engine, rows: Number(m[1]), cols: Number(m[2]) };
  } finally {
    await native.close();   // disconnects; does not close the app's window
  }
});

await main();

