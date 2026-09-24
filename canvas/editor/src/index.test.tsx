import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { PackageProbe } from "./index";

// The package is tested alone (OD-11 rule 4): this runs with the package's own React, with no
// desktop anywhere on the path, so a package that only works inside the desktop fails here.
describe("@panacea-app/canvas-editor", () => {
  it("renders alone", () => {
    const errors = vi.spyOn(console, "error");
    render(<PackageProbe />);
    expect(screen.getByTestId("probe").textContent).toBe("1");
    expect(errors).not.toHaveBeenCalled();
  });
});
