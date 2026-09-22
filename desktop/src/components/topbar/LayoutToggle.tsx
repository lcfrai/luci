// Contract §6.4: the Columns/Canvas segmented control. Rendered in every lens, because §6.4
// lists it unconditionally and the choice is persisted — flipping it in Files and finding
// Develop already in Canvas when you return is the behaviour the persisted preference implies.
import type { JSX } from "react";
import type { Layout } from "../../store/types";
import "./topbar.css";

const OPTIONS: ReadonlyArray<{ id: Layout; label: string }> = [
  { id: "columns", label: "Columns" },
  { id: "canvas", label: "Canvas" },
];

export function LayoutToggle(props: {
  layout: Layout;
  onChange: (l: Layout) => void;
}): JSX.Element {
  const { layout, onChange } = props;
  return (
    <div className="top-seg" role="group" aria-label="Layout">
      {OPTIONS.map((option) => (
        <button
          key={option.id}
          type="button"
          className="top-seg-btn"
          aria-pressed={layout === option.id}
          onClick={() => onChange(option.id)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
