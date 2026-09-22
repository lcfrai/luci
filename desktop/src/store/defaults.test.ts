import { describe, expect, it } from "vitest";
import { aHarness } from "../test/fixtures";
import { resolveDefaults, toCreateRequest } from "./defaults";
import type { Harness, SessionDefaults } from "./types";

/* The catalogue this suite reasons about, composed HERE from task 15's `aHarness` rather than
 * added to `fixtures.ts`: shared-shape §3.2 sanctions six builders and no seventh, and a
 * catalogue is a composition of one of them rather than a new one. Every field an assertion
 * below depends on is written out here, so nothing rests on a default task 15 chose.
 *
 * A full harness, a bare one whose option arrays are all empty, and an installed-but-unverified
 * one that is NOT available. The empty arrays are `[]` and never `null` — the Go side allocates
 * unconditionally (task 8's fix round), so nothing here needs a `?? []`. */
const claude = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [
    { id: "opus-4.8", name: "Opus 4.8" },
    { id: "sonnet-4.8", name: "Sonnet 4.8" },
  ],
  defaultModel: "sonnet-4.8",
  efforts: ["", "low", "medium", "high"],
  permissionModes: ["ask", "bypass", "plan"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "",
});

const shell = aHarness({
  id: "shell",
  name: "Shell",
  command: "/bin/bash",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
  supportsOpeningPrompt: false,
  supportsResume: false,
  note: "A plain shell in the sandbox.",
});

const codex = aHarness({
  id: "codex",
  name: "Codex",
  command: "codex",
  installed: true,
  flagsVerified: false,
  available: false,
  models: [{ id: "gpt-5-codex", name: "GPT-5 Codex" }],
  defaultModel: "gpt-5-codex",
  efforts: ["low", "high"],
  permissionModes: ["ask"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "Installed, but its flags have not been verified on this machine.",
});

const catalogue: Harness[] = [claude, shell, codex];

/** The literal value of `APP_DEFAULTS` (contract §3.4), written out rather than imported
 *  from `./slices/prefsSlice`. This suite is the proof that §3.6's function is pure, so it
 *  imports exactly one module of production code; step 4's store suite asserts the real
 *  constant equals this literal, which is what keeps the two from drifting. */
const APP: SessionDefaults = {
  harness: "claude-code",
  model: "",
  effort: "",
  permissionMode: "ask",
};

const full: SessionDefaults = {
  harness: "claude-code",
  model: "opus-4.8",
  effort: "high",
  permissionMode: "plan",
};

describe("resolveDefaults — the merge", () => {
  it("lets a branch override beat a project override beat the app default", () => {
    const r = resolveDefaults(full, { model: "sonnet-4.8" }, { effort: "low" }, catalogue);
    expect(r).toEqual({
      harness: "claude-code",
      model: "sonnet-4.8",
      effort: "low",
      permissionMode: "plan",
    });

    // The two overrides above touch DISJOINT keys, so they cannot tell the precedence order
    // apart — swapping project and branch produces the same answer. Make them contend for
    // one key, which is the only shape that proves branch beats project.
    const contended = resolveDefaults(
      full,
      { model: "sonnet-4.8", effort: "low" },
      { model: "opus-4.8" },
      catalogue,
    );
    expect(contended.model).toBe("opus-4.8");
    expect(contended.effort).toBe("low");
  });

  it("inherits an absent key rather than resetting it", () => {
    const r = resolveDefaults(full, {}, {}, catalogue);
    expect(r).toEqual(full);
  });

  it("treats a key present with an undefined value as absent, not as a reset", () => {
    const r = resolveDefaults(full, { effort: undefined }, undefined, catalogue);
    expect(r.effort).toBe("high");
  });

  it('honours an explicit "" effort, because "" means send no flag', () => {
    const r = resolveDefaults(full, undefined, { effort: "" }, catalogue);
    expect(r.effort).toBe("");
  });
});

describe("resolveDefaults — the clamp, in §3.6's order", () => {
  it("falls back to claude-code when the named harness is not in the catalogue", () => {
    const r = resolveDefaults({ ...full, harness: "gone" }, undefined, undefined, catalogue);
    expect(r.harness).toBe("claude-code");

    // claude-code is FIRST in `catalogue`, so the assertion above cannot tell "prefer
    // claude-code" apart from "take the first available". Put it second, behind another
    // available harness, which is the only shape that separates the two branches.
    const shellFirst: Harness[] = [shell, claude];
    expect(
      resolveDefaults({ ...full, harness: "gone" }, undefined, undefined, shellFirst).harness,
    ).toBe("claude-code");
  });

  it("falls back to claude-code when the named harness is present but unavailable", () => {
    const r = resolveDefaults({ ...full, harness: "codex" }, undefined, undefined, catalogue);
    expect(r.harness).toBe("claude-code");
  });

  it("falls back to the first available harness when claude-code is unavailable", () => {
    const degraded: Harness[] = [
      aHarness({ id: "claude-code", available: false }),
      aHarness({ id: "opencode", name: "OpenCode", available: true, defaultModel: "sonnet-4.8" }),
    ];
    const r = resolveDefaults(full, undefined, undefined, degraded);
    expect(r.harness).toBe("opencode");
  });

  it("falls back to shell when nothing in the catalogue is available", () => {
    const none = catalogue.map((h) => ({ ...h, available: false }));
    const r = resolveDefaults(full, undefined, undefined, none);
    expect(r.harness).toBe("shell");
  });

  it("falls back to shell and sends nothing optional when the catalogue is empty", () => {
    const r = resolveDefaults(full, undefined, undefined, []);
    expect(r).toEqual({ harness: "shell", model: "", effort: "", permissionMode: "ask" });
  });

  it("replaces a model the harness no longer offers with its defaultModel", () => {
    const r = resolveDefaults({ ...full, model: "opus-3" }, undefined, undefined, catalogue);
    expect(r.model).toBe(claude.defaultModel);
  });

  it("resolves an empty model to the harness defaultModel, as APP_DEFAULTS relies on", () => {
    const r = resolveDefaults(APP, undefined, undefined, catalogue);
    expect(r.model).toBe("sonnet-4.8");
  });

  it("empties the model when the harness has no models", () => {
    const r = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect(r.model).toBe("");
  });

  it("empties an effort the harness does not list", () => {
    const only = [aHarness({ id: "claude-code", available: true, efforts: ["low", "medium"] })];
    const r = resolveDefaults(full, undefined, undefined, only);
    expect(r.effort).toBe("");
  });

  it("empties the effort when the harness lists none", () => {
    const r = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect(r.effort).toBe("");
  });

  it("replaces an unlisted permission mode with the first the harness lists", () => {
    const only = [
      aHarness({ id: "claude-code", available: true, permissionModes: ["bypass", "ask"] }),
    ];
    const r = resolveDefaults(full, undefined, undefined, only);
    expect(r.permissionMode).toBe("bypass");
  });

  it('falls back to "ask" when the harness lists no permission modes', () => {
    const r = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect(r.permissionMode).toBe("ask");
  });

  it("returns a fresh object and never mutates its inputs", () => {
    const app = { ...APP };
    const project = { model: "opus-4.8" };
    const r = resolveDefaults(app, project, undefined, catalogue);
    expect(r).not.toBe(app);
    expect(app).toEqual(APP);
    expect(project).toEqual({ model: "opus-4.8" });
  });
});

describe("toCreateRequest — §3.6's inclusion table", () => {
  const resolved = resolveDefaults(full, undefined, undefined, catalogue);

  it("always carries worktreeId and harness, and never machineId", () => {
    const req = toCreateRequest("w-1", resolved, claude);
    expect(req.worktreeId).toBe("w-1");
    expect(req.harness).toBe("claude-code");
    expect("machineId" in req).toBe(false);
  });

  it("omits model and effort for a harness that offers neither", () => {
    const d = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    const req = toCreateRequest("w-1", d, shell);
    expect("model" in req).toBe(false);
    expect("effort" in req).toBe(false);
    expect("permissionMode" in req).toBe(false);
  });

  it('omits effort when the value is "" even though the harness lists efforts', () => {
    const req = toCreateRequest("w-1", { ...resolved, effort: "" }, claude);
    expect("effort" in req).toBe(false);
  });

  it("includes permissionMode whenever the harness lists any", () => {
    const req = toCreateRequest("w-1", resolved, claude);
    expect(req.permissionMode).toBe("plan");
  });

  it("trims an opening prompt and omits one that is only whitespace", () => {
    expect(toCreateRequest("w-1", resolved, claude, { openingPrompt: "  fix it  " }).openingPrompt)
      .toBe("fix it");
    expect("openingPrompt" in toCreateRequest("w-1", resolved, claude, { openingPrompt: "   " }))
      .toBe(false);
  });

  it("omits the opening prompt for a harness that does not support one", () => {
    const d = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    const req = toCreateRequest("w-1", d, shell, { openingPrompt: "hello" });
    expect("openingPrompt" in req).toBe(false);
  });

  it("omits title unless the user typed one, so the server derives it", () => {
    expect("title" in toCreateRequest("w-1", resolved, claude)).toBe(false);
    expect("title" in toCreateRequest("w-1", resolved, claude, { title: "  " })).toBe(false);
    expect(toCreateRequest("w-1", resolved, claude, { title: " Audit " }).title).toBe("Audit");
  });

  it("carries resume only for a harness that supports it", () => {
    expect(toCreateRequest("w-1", resolved, claude, { resume: "abc" }).resume).toBe("abc");
    const d = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect("resume" in toCreateRequest("w-1", d, shell, { resume: "abc" })).toBe(false);
  });

  it("carries contextRef only when the caller passes one", () => {
    expect("contextRef" in toCreateRequest("w-1", resolved, claude)).toBe(false);
    expect(toCreateRequest("w-1", resolved, claude, { contextRef: "ev-9" }).contextRef).toBe("ev-9");
  });

  it("sends the descriptor's id, so a mismatched descriptor cannot be sent", () => {
    const req = toCreateRequest("w-1", { ...resolved, harness: "codex" }, claude);
    expect(req.harness).toBe("claude-code");

    // The same rule on the option fields. "omits model and effort for a harness that offers
    // neither" passes a set already CLAMPED to shell, so its model is "" and the omission
    // cannot be attributed to the descriptor. Here the set names a real model and only the
    // descriptor says no.
    expect("model" in toCreateRequest("w-1", resolved, shell)).toBe(false);
    expect(resolved.model).toBe("opus-4.8");
  });
});
