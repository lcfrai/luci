// The footer owns API notices outside Home. On Home it links to the single
// main notice instead of announcing the same failure twice.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import { useAuth } from "../auth/AuthProvider";
import "./rail.css";
import { TeamWorkspaceControl } from "../teams/TeamWorkspaceControl";
import { clientKeyboardPlatform, shortcutLabel } from "../../lib/keyboardPlatform";
import { ApiProblemNotice } from '../common/ApiProblemNotice';
import { Button } from '../fluid/components/button';

export function RailFooter({ errorPresentedInMain = false }: { errorPresentedInMain?: boolean } = {}): JSX.Element {
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
      {lastError && (errorPresentedInMain ? <Button variant="ghost" onClick={() => {
        const notice = document.getElementById('home-api-notice');
        notice?.scrollIntoView?.({ block: 'nearest' }); notice?.focus({ preventScroll: true });
      }}>{lastError.sessionCreate ? 'View session notice' : 'View project notice'}</Button> : <ApiProblemNotice problem={lastError} />)}
    </div>
  );
}
