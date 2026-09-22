import { describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { StatusDot } from "./StatusDot";

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
});
