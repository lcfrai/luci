// Seam SX.1 (docs/SEAMS.md). Phase 7 owns this file: the Deploy connector replaces the body
// below. Task 26 names seams for phases 3-5 and 8, but closing AppShell.tsx needs every arm
// to be a file, so this one exists too. AppShell.tsx routes the "deploy" lens here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function DeployLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="deploy"
      phase={7}
      what="Ship the branch, with the spec attached to the release."
    />
  );
}
