// Contract §6.4-adjacent (§6.3's table): the honest empty state. What the lens will do, and
// which phase brings it. §7.7: every state is designed, and a skeleton that shifts layout is
// worse than no skeleton.
import type { JSX } from "react";
import type { Lens } from "../../store/types";

export function LensPlaceholder(props: {
  lens: Lens;
  phase: number;
  what: string;
}): JSX.Element {
  const { lens, phase, what } = props;
  return (
    <section className="lens-placeholder" aria-labelledby={`lens-${lens}-title`}>
      <h2 id={`lens-${lens}-title`}>{lens}</h2>
      <p>{what}</p>
      <p>Arrives in phase {phase}.</p>
    </section>
  );
}
