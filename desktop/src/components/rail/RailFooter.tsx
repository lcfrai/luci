// Contract §6.2: the user chip, the ⌘K hint, and THE single API-error line. §7.7 makes this
// the owner of the "control plane not running" state in the rail.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import "./rail.css";

export function RailFooter(): JSX.Element {
  const lastError = useCockpit((s) => s.lastError);

  return (
    <div className="rail-footer">
      <div>
        <span className="rail-name">This machine</span> <span className="rail-kbd">⌘K</span>
      </div>
      {lastError === null ? null : (
        // The server's own sentence, verbatim. There are sixteen error codes and the cockpit
        // does not paraphrase any of them — non_loopback_publish in particular describes a
        // sandbox that IS running, so any locally invented "not running" phrasing is a lie.
        <p className="rail-footer-error" role="status">
          {lastError.message}
        </p>
      )}
    </div>
  );
}
