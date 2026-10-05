import { Button } from "../fluid/components/button";
import {
  DEFAULT_APPEARANCE,
  updateAppearance,
  useAppearance,
  type Appearance,
} from "../../preferences/appearance";

export function AppearancePanel() {
  const value = useAppearance();
  return (
    <section className="appearance-panel" aria-labelledby="appearance-title">
      <h2 id="appearance-title">Make room for your way of working.</h2>
      <p className="journey-muted">
        Changes apply immediately on this device, including your design canvas
        and terminals.
      </p>
      <div className="setup-field-grid">
        <label className="luci-field">
          Colour scheme
          <select
            value={value.theme}
            onChange={(event) =>
              updateAppearance({
                theme: event.target.value as Appearance["theme"],
              })
            }
          >
            <option value="system">Follow system</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </label>
        <label className="luci-field">
          Accent colour
          <select
            value={value.accent}
            onChange={(event) =>
              updateAppearance({
                accent: event.target.value as Appearance["accent"],
              })
            }
          >
            <option value="teal">Teal</option>
            <option value="blue">Blue</option>
            <option value="rose">Rose</option>
          </select>
          <small>Status labels keep their own colours and text.</small>
        </label>
        <label className="luci-field">
          Interface density
          <select
            value={value.density}
            onChange={(event) =>
              updateAppearance({
                density: event.target.value as Appearance["density"],
              })
            }
          >
            <option value="comfortable">Comfortable</option>
            <option value="compact">Compact</option>
          </select>
        </label>
        <label className="luci-field">
          Terminal text size
          <select
            value={value.terminalFontSize}
            onChange={(event) =>
              updateAppearance({ terminalFontSize: Number(event.target.value) })
            }
          >
            {Array.from({ length: 14 }, (_, index) => index + 11).map(
              (size) => (
                <option key={size} value={size}>
                  {size} px
                </option>
              ),
            )}
          </select>
          <small>Applies to open sessions without restarting them.</small>
        </label>
      </div>
      <p className="appearance-preview">
        <span>Preview</span> Readable text, clear focus and a little breathing
        room.
      </p>
      <Button
        variant="secondary"
        type="button"
        onClick={() => updateAppearance(DEFAULT_APPEARANCE)}
      >
        Reset appearance
      </Button>
    </section>
  );
}
