// Seam SX.1 and S3.5 (docs/SEAMS.md). Phase 3 owns this file: docs/PROGRAM.md's phase 3 is
// "diff review … merge ordering, conflict resolution". Reordering goes through S3.4's
// visible-position helpers, never an index across a filter. AppShell.tsx routes here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function MergeLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="merge"
      phase={3}
      what="Review a worktree's diff and land it on the branch."
    />
  );
}
