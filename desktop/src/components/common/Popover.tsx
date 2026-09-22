// Positioning, outside-click, Escape and focus return. Contract §6.7. Used by the session
// picker, the machine chip and (task 22) the session menu.
import { useLayoutEffect, useState } from "react";
import type { JSX, ReactNode } from "react";
import { useDismissable } from "../../hooks/useDismissable";
import "./common.css";

const GUTTER = 8;

export function Popover(props: {
  anchor: DOMRect | null;
  onClose: () => void;
  children: ReactNode;
}): JSX.Element | null {
  const { anchor, onClose, children } = props;
  const ref = useDismissable<HTMLDivElement>(anchor !== null, onClose);
  const [placement, setPlacement] = useState<{ top: number; left: number } | null>(null);

  // Measure after mount and clamp into the viewport, so a popover anchored near the right edge
  // opens leftwards instead of off screen. jsdom reports every rect as zero, which is fine:
  // the clamp is a no-op there and the tests assert content, not geometry.
  useLayoutEffect(() => {
    const el = ref.current;
    if (anchor === null || el === null) {
      setPlacement(null);
      return;
    }
    const self = el.getBoundingClientRect();
    const left = Math.max(
      GUTTER,
      Math.min(anchor.left, window.innerWidth - self.width - GUTTER),
    );
    const below = anchor.bottom + 6;
    const top =
      below + self.height + GUTTER > window.innerHeight && anchor.top - self.height - 6 > GUTTER
        ? anchor.top - self.height - 6
        : below;
    setPlacement({ top, left });
    el.focus();
  }, [anchor, ref]);

  if (anchor === null) return null;

  return (
    <div
      ref={ref}
      role="dialog"
      tabIndex={-1}
      className="pop"
      style={{ top: placement?.top ?? anchor.bottom + 6, left: placement?.left ?? anchor.left }}
    >
      {children}
    </div>
  );
}
