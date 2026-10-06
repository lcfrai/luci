import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Excalidraw, MainMenu, serializeAsJSON } from "@excalidraw/excalidraw";
import type {
  ExcalidrawInitialDataState,
  ExcalidrawImperativeAPI,
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
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraft, useProjectDraftScope } from "../../hooks/useProjectDraft";
import { sketchDraftValid } from "../../hooks/projectDraftValidation";
import { DraftRecovery } from "../common/DraftRecovery";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";
import { Button } from "../fluid/components/button";

type Props = { projectId: string; onDirtyChange?(dirty: boolean): void; onBusyChange?(busy: boolean): void };
type CanvasSnapshot = Parameters<NonNullable<ExcalidrawProps["onChange"]>>;
function persistedScene([elements, appState, files]: CanvasSnapshot): DesignScene {
  const serialized = JSON.parse(serializeAsJSON(elements, appState, files, "local")) as DesignScene;
  return { elements: serialized.elements, appState: serialized.appState, files: serialized.files };
}
/** Compare the document exported by the pinned adapter, excluding viewport,
 * selection, tools and theme. Cache metadata on image files is not an edit. */
function documentFingerprint(scene: DesignScene): string {
  return JSON.stringify({ ...scene, files: Object.fromEntries(Object.entries(scene.files).sort(([a], [b]) => a.localeCompare(b)).map(([key, file]) => {
    const value = file as Record<string, unknown>;
    return [key, { id: value.id, mimeType: value.mimeType, dataURL: value.dataURL }];
  })) });
}

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

export default function DesignEditor(props: Props) {
  const scope = useProjectDraftScope(props.projectId);
  return <ProjectDesignEditor key={`${scope}:${props.projectId}`} {...props} scope={scope} />;
}
function ProjectDesignEditor({ projectId, onDirtyChange, onBusyChange, scope }: Props & { scope: string | null }) {
  const access = useProjectAccess(projectId);
  const { canRead, canEdit } = access;
  const appearance = useAppearance();
  const [design, setDesign] = useState<Design | null>(null);
  const savedDesign = useRef<Design | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [sceneDraft, setSceneDraft] = useState<DesignScene | null>(null);
  const [canvasVersion, setCanvasVersion] = useState(0);
  const draftBase = design ? JSON.stringify({ revision: design.revision, briefRevision: design.briefRevision }) : null;
  const draft = useProjectDraft({ scope, kind: "sketch", base: draftBase, value: sceneDraft, dirty, valid: sketchDraftValid });
  useUnsavedProjectDraft(projectId, dirty, "Sketch", "design", scope ? { retained: draft.retained, discard: discardEdits } : undefined);
  const baseline = useRef<string | null>(null), latestDocument = useRef<string | null>(null);
  const current = useRef<Parameters<
    NonNullable<ExcalidrawProps["onChange"]>
  > | null>(null);
  const saving = useRef(false);
  const alive = useRef(true);
  const [loading, setLoading] = useState(true);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => () => { onDirtyChange?.(false); }, [onDirtyChange]);
  useEffect(() => { onBusyChange?.(busy); }, [busy, onBusyChange]);
  useEffect(() => () => { onBusyChange?.(false); }, [onBusyChange]);
  const canvas = useRef<HTMLDivElement>(null);
  const nativeAPI = useRef<ExcalidrawImperativeAPI | null>(null);
  // The pinned editor's resize observer can update its device breakpoint after
  // the window resize already wrote the same bounds, without another render.
  // Refresh its supported layout API after those observers finish; this does
  // not change tools, elements, the viewport or the saved document.
  useLayoutEffect(() => {
    const root = canvas.current;
    if (!root || typeof ResizeObserver === "undefined") return;
    let disposed = false, frame: number | null = null;
    let { width, height } = root.getBoundingClientRect();
    const observer = new ResizeObserver(() => {
      if (disposed) return;
      const next = root.getBoundingClientRect();
      if (next.width === width && next.height === height) return;
      width = next.width; height = next.height;
      if (frame !== null) cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        frame = null;
        if (!disposed && width > 0 && height > 0) nativeAPI.current?.refresh();
      });
    });
    observer.observe(root);
    return () => {
      disposed = true;
      observer.disconnect();
      if (frame !== null) cancelAnimationFrame(frame);
    };
  }, []);
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
  function discardEdits() {
    draft.clear(); baseline.current = null; latestDocument.current = null; current.current = null;
    setDirty(false); setSaved(false); setDesign(savedDesign.current); setSceneDraft(null); setCanvasVersion(version => version + 1);
  }
  async function load() {
    if (!canRead) return;
    setLoading(true);
    setError("");
    try {
      const value = await productApi.design(projectId);
      if (alive.current) {
        baseline.current = null; latestDocument.current = null; current.current = null;
        setDirty(false); setSaved(false); savedDesign.current = value; setDesign(value); setSceneDraft(null); setCanvasVersion(version => version + 1);
      }
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
  }, [projectId, canRead || !!design]);
  async function save() {
    if (!canEdit || !design || !current.current || saving.current || draft.pending) return;
    saving.current = true;
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      const snapshot = current.current;
      const scene = persistedScene(snapshot);
      const fingerprint = documentFingerprint(scene);
      const next = await productApi.saveDesign(projectId, {
        revision: design.revision,
        frames: design.frames,
        scene,
      });
      if (alive.current) {
        baseline.current = fingerprint;
        const remaining = latestDocument.current !== fingerprint;
        if (!remaining) draft.clear();
        savedDesign.current = next; setDesign(next);
        setDirty(remaining);
        setSaved(!remaining);
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
        <Button type="button" variant="primary" loading={busy}
          disabled={!canEdit || !design || busy || !!draft.pending || !dirty}
          onClick={() => void save()}
        >
          {busy ? "Saving…" : "Save design"}
        </Button>
      </JourneyHeader>
      <div>
        {access.message && <p role="status" className="journey-muted">{access.message}</p>}
        {error && (
          <p className="journey-error" role="alert">
            {error}
            {design && <Button type="button" variant="secondary" disabled={busy || loading} onClick={() => { draft.offerRecovery(); void load(); }}>Reload saved sketch</Button>}
            {!design && (
              <Button type="button" variant="secondary" loading={loading}
                disabled={loading}
                onClick={() => void load()}
              >
                Retry loading design
              </Button>
            )}
          </p>
        )}
        {saved && (
          <span role="status" className="journey-muted">
            Design saved.
          </span>
        )}
        {dirty && <span role="status" className="journey-muted">Unsaved sketch changes.</span>}
      </div>
      <DraftRecovery draft={draft} label="Sketch" base={draftBase} saved={savedDesign.current?.scene ?? null} busy={busy || loading} readOnly={!canEdit} onDiscard={discardEdits} onRestore={value => {
        if (!design) return;
        baseline.current = "restored draft"; latestDocument.current = null;
        setDesign({ ...design, scene: value }); setSceneDraft(value); setDirty(true); setCanvasVersion(version => version + 1);
      }} />
      <div className="design-canvas" ref={canvas} aria-busy={loading}>
        {design && canRead && (
          <Excalidraw key={canvasVersion}
            viewModeEnabled={!canEdit || !!draft.pending}
            excalidrawAPI={(api) => { nativeAPI.current = api; }}
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
              try {
                const document = persistedScene(args);
                const fingerprint = documentFingerprint(document);
                // Excalidraw normalizes the initial document before its first
                // change callback. Hydration itself is not a user edit.
                if (baseline.current === null) baseline.current = fingerprint;
                const documentChanged = latestDocument.current !== fingerprint;
                latestDocument.current = fingerprint;
                const changed = fingerprint !== baseline.current;
                setDirty(changed);
                // The pinned editor emits onChange after host renders too.
                // Publishing a fresh scene object for that same document would
                // feed another host render back into the native editor forever.
                if (changed && documentChanged) { setSceneDraft(document); setSaved(false); }
                else if (!changed && draft.retained && !draft.pending) draft.clear();
              } catch (cause) {
                setDirty(true); setSaved(false); setError(errorText(cause));
              }
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
