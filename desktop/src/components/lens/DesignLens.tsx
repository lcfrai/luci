// Seam SX.1 and S5.2 (docs/SEAMS.md). Phase 5 owns this file: the Design canvas replaces the
// body below. It is handed a fixed box — .shell-lens is min-block-size 0, overflow hidden —
// and owns its own scroll. AppShell.tsx routes the "design" lens here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function DesignLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="design"
      phase={5}
      what="A canvas of real HTML frames an agent can edit directly."
    />
  );
}
