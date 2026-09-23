// Contract §6.5: the ⋯ menu. Props are exactly { sessionId, anchor, onClose } — the anchor is
// resolved by the caller, because uiSlice.sessionMenuFor carries only an id and task 20's
// header exposes the button through a data attribute rather than a ref.
//
// This file renders its own rows rather than importing a shared list component from
// ContextMenu.tsx, because ContextMenu.tsx imports SessionMenu to mount it — a `MenuList`
// exported from there and imported back here would be a genuine ES module cycle. Vite tolerates
// cycles; they fail later, in a bundler or a test runner, in a way that names neither file. The
// duplicated markup is the cheaper of the two, and menu.css keeps both honest.
import { Fragment, useState } from "react";
import type { FormEvent, JSX } from "react";
import { Popover } from "../common/Popover";
import { useDismissable } from "../../hooks/useDismissable";
import { useCreateSession } from "../../hooks/useCreateSession";
import { selectSession } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionId } from "../../store/types";
import { renameSessionTo, sessionVerbs } from "./sessionVerbs";
import "../menu/menu.css";

/** The pane's ⋯ button, then the rail row for the same session, then a point near the top
 *  centre of the viewport. Never null: a null anchor closes `Popover`, and a menu the user
 *  opened from a rail row for a session no pane is showing would then open and never appear. */
export function anchorRectFor(sessionId: SessionId): DOMRect {
  const pane = document.querySelector(`[data-session-menu-anchor="${sessionId}"]`);
  if (pane !== null) return pane.getBoundingClientRect();
  const row = document.querySelector(`[data-kind="session"][data-id="${sessionId}"]`);
  if (row !== null) return row.getBoundingClientRect();
  return new DOMRect(Math.round(window.innerWidth / 2), Math.round(window.innerHeight / 3), 0, 0);
}

export function SessionMenu(props: {
  sessionId: SessionId;
  anchor: DOMRect | null;
  onClose: () => void;
}): JSX.Element {
  const { sessionId, anchor, onClose } = props;
  const title = useCockpit((s) => selectSession(sessionId)(s)?.title ?? "");
  const createSession = useCreateSession();
  const [renaming, setRenaming] = useState(false);
  const [draft, setDraft] = useState("");
  // Task 18's overlay stack, not a second Escape listener. Pushed ABOVE the Popover's entry
  // while renaming, so §6.9's "close the topmost overlay" returns to the verb list rather than
  // closing the menu, and a pointerdown outside the form abandons the rename by the same route.
  // Unconditional, above the early return: hook order never varies.
  const renameRef = useDismissable<HTMLFormElement>(renaming, () => setRenaming(false));

  if (anchor === null) return <></>;

  const items = sessionVerbs(sessionId, {
    createSession,
    close: onClose,
    beginRename: () => {
      setDraft(title);
      setRenaming(true);
    },
  });

  const submit = (event: FormEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setRenaming(false);
    renameSessionTo(sessionId, draft);
    onClose();
  };

  const close = (): void => {
    setRenaming(false);
    onClose();
  };

  return (
    <Popover anchor={anchor} onClose={close}>
      {renaming ? (
        <form ref={renameRef} className="menu menu-rename" onSubmit={submit}>
          <label className="menu-rename-label" htmlFor={`rename-${sessionId}`}>
            Session title
          </label>
          <input
            id={`rename-${sessionId}`}
            className="menu-rename-input"
            value={draft}
            autoFocus
            onChange={(event) => setDraft(event.target.value)}
          />
        </form>
      ) : (
        <div className="menu" role="menu">
          {items.map((item) => (
            <Fragment key={item.id}>
              {item.separatorBefore === true ? <div className="menu-sep" /> : null}
              <button
                type="button"
                role="menuitem"
                className={item.danger === true ? "menu-item menu-item-danger" : "menu-item"}
                disabled={item.run === undefined}
                title={item.disabledReason}
                onClick={item.run}
              >
                {item.label}
              </button>
            </Fragment>
          ))}
        </div>
      )}
    </Popover>
  );
}
