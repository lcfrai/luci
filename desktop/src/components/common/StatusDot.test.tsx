import { describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { StatusDot } from "./StatusDot";
// Read as text, not as a stylesheet: jsdom computes no animations and applies no media query,
// so the only way to assert a reduced-motion rule EXISTS is to read the source. Same mechanism
// as heartbeat.test.ts's --beat pin and task 14's tokens.test.ts.
import commonCss from "./common.css?raw";

function dot(): HTMLElement {
  return screen.getByRole("img");
}

describe("StatusDot", () => {
  it("carries shape as well as colour, for every status the server can send", () => {
    const cases: Array<[string, string]> = [
      ["starting", "dot-ring"],
      ["running", "dot-disc"],
      ["idle", "dot-disc"],
      ["exited", "dot-square"],
      ["provisioning", "dot-ring"],
      ["sleeping", "dot-ring"],
      ["stopped", "dot-square"],
      ["error", "dot-square"],
    ];
    for (const [status, shape] of cases) {
      const { unmount } = render(
        <StatusDot status={status as "running"} live={false} label={status} />,
      );
      expect(dot().className).toContain(shape);
      expect(dot().className).toContain(`dot-tone-${status}`);
      unmount();
    }
  });

  it("names the state for a screen reader and for a hover", () => {
    render(<StatusDot status="running" live label="running · claude" />);
    expect(dot().getAttribute("aria-label")).toBe("running · claude");
    expect(dot().getAttribute("title")).toBe("running · claude");
  });

  it("joins the shared beat only when live, and never spins per row", () => {
    const { unmount } = render(<StatusDot status="running" live label="running" />);
    expect(dot().className).toContain("dot-live");
    expect(dot().style.animationDelay).toMatch(/^-\d+ms$/);
    unmount();
    render(<StatusDot status="idle" live={false} label="idle" />);
    expect(dot().className).not.toContain("dot-live");
    expect(dot().style.animationDelay).toBe("");
  });

  it("flashes for 600 ms when the status changes, then stops", () => {
    vi.useFakeTimers();
    const { rerender } = render(<StatusDot status="starting" live={false} label="starting" />);
    expect(dot().className).not.toContain("dot-flash");
    rerender(<StatusDot status="running" live label="running" />);
    expect(dot().className).toContain("dot-flash");
    act(() => void vi.advanceTimersByTime(600));
    expect(dot().className).not.toContain("dot-flash");
    vi.useRealTimers();
  });

  // `running` and `idle` are both dot-disc, separated in normal motion by colour AND by the
  // breath. tokens.css zeroes animation-duration globally under prefers-reduced-motion, so for
  // that user the breath stops and the two would differ by COLOUR ALONE — which is what the
  // shape vocabulary exists to prevent, and a WCAG 1.4.1 failure for a sighted colour-blind
  // user that aria-label does not reach. common.css restores a static shape difference.
  it("still separates running from idle by shape when motion is reduced", () => {
    // The premise, MEASURED off the component rather than restated as a literal: the two
    // share a shape class, so the breath is the only other thing telling them apart.
    const shapeOf = (el: HTMLElement): string | undefined =>
      /dot-(disc|ring|square)/.exec(el.className)?.[0];
    const { unmount } = render(<StatusDot status="running" live label="running" />);
    const runningShape = shapeOf(dot());
    const runningIsLive = dot().className.includes("dot-live");
    unmount();
    render(<StatusDot status="idle" live={false} label="idle" />);
    expect(runningShape).toBe(shapeOf(dot()));
    expect(runningIsLive).toBe(true);
    expect(dot().className).not.toContain("dot-live");

    // So .dot-live is the differentiator, and it must survive motion being switched off.
    const block = /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]*?)\n\}/.exec(
      commonCss,
    );
    expect(block).not.toBeNull();
    expect(block![1]).toContain(".dot-live");
    // NON-inset, so it is visibly distinct from .dot-ring's inset ring.
    expect(block![1]).toMatch(/box-shadow:\s*0 0 0 1\.5px currentColor/);
  });
});
