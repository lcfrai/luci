// Seam SX.1 and S5.2 (docs/SEAMS.md). Phase 5 owns this file: the Design canvas replaces the
// body below. It is handed a fixed box — .shell-lens is min-block-size 0, overflow hidden —
// and owns its own scroll. AppShell.tsx routes the "design" lens here and is closed.
import { lazy, Suspense } from "react";
import { ProjectRequired } from "../journey/shared";
const DesignWorkspace = lazy(() => import("../design/DesignWorkspace"));

export function DesignLens() {
  return (
    <ProjectRequired>
      {(id) => (
        <Suspense
          fallback={
            <div className="journey-page">
              <p role="status">Opening your design tools…</p>
            </div>
          }
        >
          <DesignWorkspace key={id} projectId={id} />
        </Suspense>
      )}
    </ProjectRequired>
  );
}
