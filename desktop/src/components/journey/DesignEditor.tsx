import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Excalidraw, MainMenu, serializeAsJSON } from "@excalidraw/excalidraw";
import type {
  ExcalidrawInitialDataState,
  ExcalidrawProps,
} from "@excalidraw/excalidraw/types";
import "@excalidraw/excalidraw/index.css";
import {
  productApi,
  errorText,
  type Design,
  type DesignScene,
} from "../../api/product";
import { JourneyHeader } from "./shared";
import { useAppearance } from "../../preferences/appearance";

/** Excalidraw stores a concrete paper colour. Resolve the shared token through
 * the browser so persisted scene data does not depend on CSS variable support. */
function initialPaperColour(): string | undefined {
  const colour = getComputedStyle(document.documentElement)
    .getPropertyValue("--design-paper")
    .trim();
  if (!colour) return undefined;
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 1;
  const context = canvas.getContext("2d");
  if (!context) return undefined;
  context.fillStyle = colour;
  context.fillRect(0, 0, 1, 1);
  return "#" + Array.from(context.getImageData(0, 0, 1, 1).data)
    .slice(0, 3)
    .map((channel) => channel.toString(16).padStart(2, "0"))
    .join("");
}

export default function DesignEditor({ projectId }: { projectId: string }) {
  const appearance = useAppearance();
  const [design, setDesign] = useState<Design | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const current = useRef<Parameters<
    NonNullable<ExcalidrawProps["onChange"]>
  > | null>(null);
  const saving = useRef(false);
  const alive = useRef(true);
  const [loading, setLoading] = useState(true);
  const canvas = useRef<HTMLDivElement>(null);
  // Excalidraw 0.18.1 omits a name on its menu and gives its nested canvas
  // controls a page-footer landmark. Keep its controls, supplying semantics
  // only within our embedded canvas, including when its mobile layout changes.
  useLayoutEffect(() => {
    const root = canvas.current;
    if (!root) return;
    const describe = () => {
      const menu = root.querySelector(".main-menu-trigger");
      if (menu && !menu.getAttribute("aria-label"))
        menu.setAttribute("aria-label", "Design menu");
      const toolbar = root.querySelector("footer.layer-ui__wrapper__footer");
      if (toolbar && toolbar.getAttribute("role") !== "group") {
        toolbar.setAttribute("role", "group");
        toolbar.setAttribute("aria-label", "Design canvas controls");
      }
    };
    describe();
    const observer = new MutationObserver(describe);
    observer.observe(root, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, []);
  async function load() {
    setLoading(true);
    setError("");
    try {
      const value = await productApi.design(projectId);
      if (alive.current) setDesign(value);
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      if (alive.current) setLoading(false);
    }
  }
  useEffect(() => {
    alive.current = true;
    void load();
    return () => {
      alive.current = false;
    };
  }, [projectId]);
  async function save() {
    if (!design || !current.current || saving.current) return;
    saving.current = true;
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      const snapshot = current.current;
      const [elements, appState, files] = snapshot;
      const serialized = JSON.parse(
        serializeAsJSON(elements, appState, files, "local"),
      ) as DesignScene;
      const scene = {
        elements: serialized.elements,
        appState: serialized.appState,
        files: serialized.files,
      };
      const next = await productApi.saveDesign(projectId, {
        revision: design.revision,
        frames: design.frames,
        scene,
      });
      if (alive.current) {
        setDesign(next);
        setSaved(current.current === snapshot);
      }
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      saving.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="design-page">
      <JourneyHeader
        title="Give the idea a shape."
        subtitle="Sketch the screens and connections. Your design stays with this project."
      >
        <span className="journey-revision" role="status">
          {design
            ? `Design ${design.revision} · Brief ${design.briefRevision}`
            : loading
              ? "Loading design…"
              : "Design unavailable"}
        </span>
        <button
          className="luci-button luci-primary"
          disabled={!design || busy}
          onClick={() => void save()}
        >
          {busy ? "Saving…" : "Save design"}
        </button>
      </JourneyHeader>
      <div>
        {error && (
          <p className="journey-error" role="alert">
            {error}
            {!design && (
              <button
                className="luci-button"
                disabled={loading}
                onClick={() => void load()}
              >
                Retry loading design
              </button>
            )}
          </p>
        )}
        {saved && (
          <span role="status" className="journey-muted">
            Design saved.
          </span>
        )}
      </div>
      <div className="design-canvas" ref={canvas} aria-busy={loading}>
        {design && (
          <Excalidraw
            initialData={
              (design.scene ?? {
                elements: [],
                appState: {
                  // Excalidraw applies its own dark-theme canvas transformation.
                  // A document's paper colour is independent of the editor theme.
                  viewBackgroundColor: initialPaperColour(),
                },
                files: {},
              }) as ExcalidrawInitialDataState
            }
            theme={appearance.resolvedTheme}
            name="Luci design"
            onChange={(...args) => {
              current.current = args;
              setSaved(false);
            }}
            validateEmbeddable={false}
            onLinkOpen={(_element, event) => event.preventDefault()}
            UIOptions={{
              canvasActions: {
                loadScene: false,
                saveToActiveFile: false,
                toggleTheme: false,
              },
              tools: { image: true },
            }}
          >
            <MainMenu>
              <MainMenu.DefaultItems.Export />
              <MainMenu.DefaultItems.SaveAsImage />
              <MainMenu.DefaultItems.ClearCanvas />
              <MainMenu.DefaultItems.ChangeCanvasBackground />
            </MainMenu>
          </Excalidraw>
        )}
      </div>
    </section>
  );
}
