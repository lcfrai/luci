// Seam SX.1 (docs/SEAMS.md). Phase 4 owns this file: Discover replaces the body below and
// nothing else in the tree has to change. AppShell.tsx routes the "discover" lens here and
// is closed for the programme. Until Phase 4 it renders what AppShell rendered inline
// before task 26: the placeholder, naming the phase that brings the lens.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function DiscoverLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="discover"
      phase={4}
      what="Turn a conversation into a spec this branch can be measured against."
    />
  );
}
