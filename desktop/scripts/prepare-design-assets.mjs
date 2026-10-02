import { cp, mkdir, realpath, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
const source = fileURLToPath(
  new URL(
    "../node_modules/@excalidraw/excalidraw/dist/prod/fonts",
    import.meta.url,
  ),
);
const desktopRoot = await realpath(fileURLToPath(new URL("../", import.meta.url)));
const target = path.resolve(desktopRoot, "public/design-assets/fonts");
const existing = await realpath(target).catch(error=>{if(error.code === "ENOENT")return null;throw error;});
// Only replace our fixed generated font folder, never a linked path outside this checkout.
if (!target.startsWith(desktopRoot + path.sep) || (existing !== null && existing !== target)) {
  throw new Error("The generated design-font destination must stay inside desktop/public.");
}
await rm(target, {recursive:true,force:true});
await mkdir(target, { recursive: true });
// Excalidraw 0.18.1 marks legacy Liberation as serverSide-only. Browser fonts ship separately.
await cp(source, target, { recursive: true, filter: entry => path.basename(entry) !== "Liberation" });
