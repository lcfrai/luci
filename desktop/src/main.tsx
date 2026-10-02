// The token layer and the document ground load FIRST, before any component stylesheet.
// Vite emits CSS in module-graph order, so importing this after "./App" would put every
// component sheet ahead of the reset that is supposed to sit under it.
import "./styles/app.css";
import React from "react";
import ReactDOM from "react-dom/client";
import { invoke, isTauri } from "@tauri-apps/api/core";
import "./components/journey/designAssets";

const root = ReactDOM.createRoot(
  document.getElementById("root") as HTMLElement,
);
async function launch() {
  try {
    if (isTauri()) {
      const runtime = await invoke<{
        apiUrl: string;
        startupError: string | null;
      }>("runtime_info");
      if (runtime.startupError) throw new Error(runtime.startupError);
      window.__LUCI_API__ = runtime.apiUrl;
      root.render(
        <div style={{ padding: 40 }}>
          <h1>Luci</h1>
          <p>Starting your local service…</p>
        </div>,
      );
      let ready = false;
      for (let attempt = 0; attempt < 60; attempt += 1) {
        try {
          const response = await fetch(`${runtime.apiUrl}/v1/setup/status`, {
            signal: AbortSignal.timeout(1000),
          });
          if (response.ok) {
            ready = true;
            break;
          }
        } catch {
          /* Startup may still be opening the listener. */
        }
        await new Promise((resolve) => setTimeout(resolve, 500));
      }
      if (!ready)
        throw new Error(
          "Luci’s local service did not become ready. Check the local service or restart Luci.",
        );
    }
    // Resolve runtime configuration before either API singleton is constructed.
    const { default: App } = await import("./App");
    root.render(
      <React.StrictMode>
        <App />
      </React.StrictMode>,
    );
  } catch (error) {
    root.render(
      <div style={{ padding: 40 }}>
        <h1>Luci needs a moment.</h1>
        <p role="alert">
          {error instanceof Error ? error.message : "The app could not start."}
        </p>
        <button onClick={() => void launch()}>Retry</button>
      </div>,
    );
  }
}
void launch();
