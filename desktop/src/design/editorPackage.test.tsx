import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { PackageProbe } from "@panacea-app/canvas-editor";

// The canvas editor is its own npm package (OD-11 rule 4), linked in with file:../canvas/editor.
// Its source imports "react" from canvas/editor, which has its own node_modules. Two things
// must hold across that boundary, and each test below breaks when its config line goes.

describe("canvas editor package, consumed by the desktop", () => {
  // resolve.dedupe in vitest.config.ts (and vite.config.ts): without it the package's hook runs
  // against canvas/editor's React while the desktop's react-dom renders, and throws
  // "Invalid hook call".
  it("one React across the boundary", () => {
    const errors = vi.spyOn(console, "error");
    render(
      <PackageProbe>
        <b>x</b>
      </PackageProbe>,
    );
    expect(screen.getByTestId("probe").textContent).toBe("1x");
    expect(errors).not.toHaveBeenCalled();
  });

  // compilerOptions.paths in tsconfig.json: a ReactNode typed by the desktop's @types/react goes
  // into a prop typed by the package. The compile-time half is `npx tsc --noEmit` in desktop/.
  it("one @types/react", () => {
    const child: ReactNode = <b>x</b>;
    render(<PackageProbe>{child}</PackageProbe>);
    expect(screen.getByTestId("probe").textContent).toBe("1x");
  });
});
