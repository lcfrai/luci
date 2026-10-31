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
        Choose your colours, spacing and contrast. Changes apply immediately.
      </p>
      <p className="journey-muted" role="status">
        {value.persistence === "persisted"
          ? "Appearance saved on this device."
          : "Changes apply in this window only. Device storage is unavailable; they may be lost when you close it."}
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
          <span id="appearance-contrast-label">Surface contrast</span>
          <select
            aria-labelledby="appearance-contrast-label"
            aria-describedby="appearance-contrast-help"
            value={value.contrast}
            onChange={(event) =>
              updateAppearance({
                contrast: event.target.value as Appearance["contrast"],
              })
            }
          >
            <option value="standard">Standard</option>
            <option value="strong">Stronger boundaries</option>
          </select>
          <small id="appearance-contrast-help">Make panel edges and secondary text easier to distinguish.</small>
        </label>
        <label className="luci-field">
          <span id="appearance-density-label">Interface density</span>
          <select
            aria-labelledby="appearance-density-label"
            aria-describedby="appearance-density-help"
            value={value.density}
            onChange={(event) =>
              updateAppearance({
                density: event.target.value as Appearance["density"],
              })
            }
          >
            <option value="comfortable">Comfortable</option>
            <option value="compact">Compact</option>
            <option value="spacious">Spacious</option>
          </select>
          <small id="appearance-density-help">
            {value.density === "compact"
              ? "Tighter rows and controls, with more work in view."
              : value.density === "spacious"
                ? "Larger controls and more space between rows and panels."
                : "Balanced spacing for everyday work."}
            {" "}Touch controls keep a 44 px minimum target.
          </small>
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
      <section className="appearance-preview" aria-labelledby="appearance-preview-title">
        <h3 id="appearance-preview-title">Appearance preview</h3>
        <p>Sample project cards use your current colours, contrast and spacing.</p>
        <div className="appearance-preview-cards">
          <article><h4>First project</h4><p>A separate surface for each project.</p></article>
          <article><h4>Second project</h4><p>Readable details and clear boundaries.</p></article>
        </div>
      </section>
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
