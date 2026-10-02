// This side effect must run before Excalidraw's module initializes its font faces.
// Import this from the bootstrap before dynamically importing App. Putting it
// beside the editor's static imports is too late for shared dependency chunks.
window.EXCALIDRAW_ASSET_PATH = "/design-assets/";
