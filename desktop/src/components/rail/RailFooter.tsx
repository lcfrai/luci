// Contract §6.2: the user chip, the ⌘K hint, and THE single API-error line. §7.7 makes this
// the owner of the "control plane not running" state in the rail.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import { useAuth } from "../auth/AuthProvider";
import "./rail.css";
import { TeamWorkspaceControl } from "../teams/TeamWorkspaceControl";
import { clientKeyboardPlatform, shortcutLabel } from "../../lib/keyboardPlatform";

export function RailFooter(): JSX.Element {
  const setup = useSetup();
  const auth = useAuth();
  const lastError = useCockpit((s) => s.lastError);
  const openPalette = useCockpit((s) => s.openPalette);
  const platform = clientKeyboardPlatform();

  return (
    <div className="rail-footer">
      <TeamWorkspaceControl />
      <div>
        <button
          className="rail-settings"
          type="button"
          onClick={() => setup?.openSettings()}
        >
          Settings
        </button>
        <button
          className="rail-settings rail-command"
          type="button"
          onClick={openPalette}
          aria-keyshortcuts={platform === "mac" ? "Meta+K" : "Control+K"}
        >
          Commands <kbd className="rail-kbd">{shortcutLabel("K", platform)}</kbd>
        </button>
      </div>
      {auth && (
        <div className="rail-account">
          <button className="rail-settings" onClick={auth.openAccount}>
            {auth.profile.user.displayName || auth.profile.user.username}
          </button>
          {Object.values(auth.profile.permissions).some(Boolean) &&
            (auth.profile.permissions.manageUsers ||
              auth.profile.permissions.manageBacking ||
              auth.profile.permissions.managePolicy ||
              auth.profile.permissions.inspectAudit) && (
              <button className="rail-settings" onClick={auth.openAdmin}>
                Admin
              </button>
            )}
        </div>
      )}
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
