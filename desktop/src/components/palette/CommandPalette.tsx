// Contract §6.7 (the ⌘K surface) and §6.8 (it owns DOM focus WHILE OPEN and returns it on
// close, and owns no other focus and no scroll).
//
// Selection is by ID, never by array index — §6.7 and known defect 10. Every position that
// enters this file is converted back to an id through idAtVisiblePosition (§3.9) before it is
// used for anything.
import { useLayoutEffect, useMemo, useRef, useState } from "react";
import type { JSX, KeyboardEvent as ReactKeyboardEvent } from "react";
import { buildCommands, filterCommands } from "./commands";
import type { Command } from "./commands";
import { useDismissable } from "../../hooks/useDismissable";
import { idAtVisiblePosition } from "../../lib/immutable";
import { useCockpit } from "../../store/store";
import "./palette.css";

/** A toggle row shows its state where other rows show their context, so the state is read in
 *  the row itself and in the same place on every toggle (OD-9). */
function metaText(command: Command): string {
  if (command.checked === undefined) return command.meta ?? "";
  return command.checked ? "On" : "Off";
}

export function CommandPalette(): JSX.Element | null {
  const open = useCockpit((s) => s.paletteOpen);
  // Every hook lives in PaletteBody, so a mount IS the open event and the command list can be
  // memoised with no dependencies. Putting them here would need [open] in every dep array and
  // would keep a stale list alive between opens.
  return open ? <PaletteBody /> : null;
}

function PaletteBody(): JSX.Element {
  const closePalette = useCockpit((s) => s.closePalette);

  // Called FIRST: its layout effect records the previously focused element before ours steals
  // focus for the input (§6.8). It also owns Escape and outside-click for this overlay, which
  // is why neither appears below and neither appears in useKeyboardMap (§6.9).
  const shellRef = useDismissable<HTMLDivElement>(true, closePalette);
  const inputRef = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => {
    inputRef.current?.focus();
  }, []);

  const commands = useMemo(() => buildCommands(useCockpit.getState()), []);
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const visible = filterCommands(commands, query);
  const visibleIds = visible.map((command) => command.id);
  // Defect 10, contained: the cursor follows the ID and only falls back to the first row when
  // that id is no longer visible. An index here would silently re-point at a different command
  // on every keystroke that shortens the list.
  const activeId =
    selectedId !== null && visibleIds.includes(selectedId) ? selectedId : (visibleIds[0] ?? null);

  function move(delta: number): void {
    if (visibleIds.length === 0) return;
    // 1-based, and 0 when the cursor is not in the visible list at all.
    const position = activeId === null ? 0 : visibleIds.indexOf(activeId) + 1;
    let next = position + delta;
    if (next < 1) next = visibleIds.length;
    if (next > visibleIds.length) next = 1;
    setSelectedId(idAtVisiblePosition(visibleIds, next));
  }

  function runId(id: string | null): void {
    if (id === null) return;
    // Looked up in the VISIBLE list, by id, at the moment Enter fires. Not commands[index].
    const command = visible.find((candidate) => candidate.id === id);
    if (command === undefined) return;
    // Close first: the overlay is gone before the command's writes land, so the user never
    // sees a palette rendered over an app that has already changed underneath it.
    closePalette();
    command.run();
  }

  function onKeyDown(event: ReactKeyboardEvent<HTMLInputElement>): void {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      move(1);
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      move(-1);
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      runId(activeId);
    }
    // Escape is deliberately absent. useDismissable owns it for every overlay (§6.9).
  }

  return (
    <div className="pal-scrim" role="presentation">
      <div className="pal" role="dialog" aria-modal="true" aria-label="Command palette" ref={shellRef}>
        <input
          ref={inputRef}
          className="pal-input"
          type="text"
          role="combobox"
          aria-expanded="true"
          aria-controls="pal-list"
          aria-activedescendant={activeId === null ? undefined : `pal-row-${activeId}`}
          placeholder="Search commands, sessions, worktrees, branches and projects"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={onKeyDown}
        />
        {visible.length === 0 ? (
          <p className="pal-empty">
            Nothing matches <b>{query}</b>.
          </p>
        ) : (
          <ul className="pal-list" id="pal-list" role="listbox" aria-label="Commands">
            {visible.map((command) => (
              <li
                key={command.id}
                id={`pal-row-${command.id}`}
                role="option"
                aria-selected={command.id === activeId}
                // OD-9's toggle row. Undefined on every other row, so React omits the attribute.
                aria-checked={command.checked}
                className={command.id === activeId ? "pal-row pal-row-on" : "pal-row"}
                // Keeps DOM focus in the input: a pointerdown that blurred it would fire
                // useDismissable's focus return mid-click.
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => runId(command.id)}
              >
                <span className="pal-glyph" aria-hidden="true">
                  {command.glyph}
                </span>
                <span className="pal-label">{command.label}</span>
                <span className="pal-meta">{metaText(command)}</span>
                <kbd className="pal-hint" hidden={command.hint === undefined}>
                  {command.hint ?? ""}
                </kbd>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
