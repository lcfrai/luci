import "./develop.css";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { ColumnsLayout } from "./ColumnsLayout";
import { EmptyDevelop } from "./EmptyDevelop";
import { selectPaneSessionIds, selectSelectedBranch } from "../../store/selectors";
import { useCockpit } from "../../store/store";

export function DevelopLens(): JSX.Element {
  // useShallow is mandatory here: this selector builds a new array every call (§3.7). Without
  // it, zustand 5's Object.is sees a fresh array on every 2 s poll and re-renders the whole
  // lens thirty times a minute — invisible until ten panes are open. selectSelectedBranch
  // needs none: it returns a stored object.
  const sessionIds = useCockpit(useShallow(selectPaneSessionIds));
  const branch = useCockpit(selectSelectedBranch);

  if (sessionIds.length === 0) {
    return (
      <div className="cols-empty">
        <EmptyDevelop branch={branch} />
      </div>
    );
  }

  /* TASK 21 — the Canvas branch goes HERE, and this comment is its reservation.
     In ONE step (Group D shared-shape rule 2), task 21 adds the `layout` and
     `selectCanvasSessionIds` reads above and replaces the return below with a ternary whose
     canvas arm is <CanvasLayout sessionIds={canvasIds}/>. Neither read exists yet on
     purpose: an unused local is either deleted by the compiler's check or kept alive by a
     `void`, and both are worse than adding it with its consumer.
     Until task 21 lands, Canvas renders Columns rather than an empty plane — task 18's
     layout toggle already ships, and a main area that goes blank when a user flips a toggle
     reads as a crash. The component name above is deliberately mid-line so task 21's
     anchored guard reads 0 here and 1 after its change (shared-shape rule 3). */
  return <ColumnsLayout sessionIds={sessionIds} />;
}
