import { useEffect, useRef, useState } from "react";
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

export default function DesignEditor({ projectId }: { projectId: string }) {
  const [design, setDesign] = useState<Design | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const current = useRef<Parameters<
    NonNullable<ExcalidrawProps["onChange"]>
  > | null>(null);
  const saving = useRef(false);
  useEffect(() => {
    let cancelled = false;
    productApi
      .design(projectId)
      .then((value) => {
        if (!cancelled) setDesign(value);
      })
      .catch((cause) => {
        if (!cancelled) setError(errorText(cause));
      });
    return () => {
      cancelled = true;
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
      setDesign(next);
      setSaved(current.current === snapshot);
    } catch (cause) {
      setError(errorText(cause));
    } finally {
      saving.current = false;
      setBusy(false);
    }
  }
  return (
    <section className="design-page">
      <JourneyHeader
        title="Give the idea a shape."
        subtitle="Sketch the screens and connections. Your design stays with this project."
      >
        <span className="journey-revision">
          {design
            ? `Design ${design.revision} · Brief ${design.briefRevision}`
            : "Loading…"}
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
          </p>
        )}
        {saved && (
          <span role="status" className="journey-muted">
            Design saved locally.
          </span>
        )}
      </div>
      <div className="design-canvas">
        {design && (
          <Excalidraw
            initialData={
              (design.scene ?? {
                elements: [],
                appState: {
                  viewBackgroundColor: getComputedStyle(
                    document.documentElement,
                  )
                    .getPropertyValue("--ink")
                    .trim(),
                },
                files: {},
              }) as ExcalidrawInitialDataState
            }
            theme="dark"
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
