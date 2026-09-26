// Seam SX.1 and S3.5 (docs/SEAMS.md). Phase 3 owns this file by the phase number it has always
// rendered; docs/PROGRAM.md does not yet name an owner for Files, and S3.5 records that as the
// product owner's decision. AppShell.tsx routes the "files" lens here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function FilesLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="files"
      phase={3}
      what="The worktree's files, with what each session changed."
    />
  );
}
