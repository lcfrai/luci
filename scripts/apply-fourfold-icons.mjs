// Mechanical packaging of the approved Fourfold artwork, never a redesign.
// Requires Sharp as a developer tool; LUCI_SHARP_MODULE may name an installed copy.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const sharp = require(process.env.LUCI_SHARP_MODULE || "sharp");
const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const artwork = "desktop/src/assets/brand/handoff-2026-10-07";
const hash = data => createHash("sha256").update(data).digest("hex");
const read = file => readFile(path.join(root, file));
const canonical = await read(`${artwork}/canonical/luci-four-ds-fourfold.svg`);
assert.equal(hash(canonical), "e69afb839bc6384841c53ebcc1a72953ae1c18b1a1bf672f989d38c42ca9e155");
const checksums = JSON.parse(await read(`${artwork}/checksums.json`));
for (const record of checksums) {
  const bytes = await read(`${artwork}/${record.file}`);
  assert.equal(bytes.length, record.bytes, `Approved source length changed: ${record.file}`);
  assert.equal(hash(bytes), record.sha256, `Approved source hash changed: ${record.file}`);
}

const outputs = [], images = new Map();
async function raster(size) {
  if (images.has(size)) return images.get(size);
  const supplied = checksums.find(record => record.file === `png/light-surface/luci-fourfold-${size}.png`);
  const source = supplied ? `${artwork}/${supplied.file}` : `${artwork}/canonical/luci-four-ds-fourfold.svg`;
  // Large outputs rasterize the original vector at their actual dimensions.
  // A small supplied PNG is never enlarged to make a larger icon.
  const bytes = supplied ? await read(source) : await sharp(canonical, { density: Math.max(72, 72 * size / 256) }).resize(size, size).png().toBuffer();
  const meta = await sharp(bytes).metadata();
  assert.equal(meta.width, size); assert.equal(meta.height, size);
  assert.ok(meta.hasAlpha, "Preserve the approved transparent canvas");
  const image = { bytes, source, method: supplied ? "exact-approved-png" : "direct-canonical-svg-rasterization" };
  images.set(size, image);
  return image;
}
async function output(file, bytes, source, method, extra = {}) {
  await writeFile(path.join(root, file), bytes);
  outputs.push({ file, source, method, ...extra, bytes: bytes.length, sha256: hash(bytes) });
}
async function pngFiles(directory) {
  const result = [];
  for (const entry of await readdir(path.join(root, directory), { withFileTypes: true })) {
    const file = `${directory}/${entry.name}`;
    if (entry.isDirectory()) result.push(...await pngFiles(file));
    else if (entry.name.endsWith(".png")) result.push(file);
  }
  return result.sort();
}
// Preserve all existing native platform filenames and dimensions. Only replace ink.
for (const file of await pngFiles("desktop/src-tauri/icons")) {
  const meta = await sharp(await read(file)).metadata();
  assert.equal(meta.width, meta.height, `Expected an existing square icon: ${file}`);
  const image = await raster(meta.width);
  await output(file, image.bytes, image.source, image.method, { width: meta.width, height: meta.height });
}

const icoSizes = [16, 24, 32, 48, 64, 128, 256];
const icoImages = await Promise.all(icoSizes.map(raster));
const icoHeader = Buffer.alloc(6 + 16 * icoSizes.length);
icoHeader.writeUInt16LE(1, 2); icoHeader.writeUInt16LE(icoSizes.length, 4);
let offset = icoHeader.length;
icoImages.forEach((image, index) => {
  const at = 6 + 16 * index, size = icoSizes[index];
  icoHeader[at] = size === 256 ? 0 : size; icoHeader[at + 1] = icoHeader[at];
  icoHeader.writeUInt16LE(1, at + 4); icoHeader.writeUInt16LE(32, at + 6);
  icoHeader.writeUInt32LE(image.bytes.length, at + 8); icoHeader.writeUInt32LE(offset, at + 12);
  offset += image.bytes.length;
});
await output("desktop/src-tauri/icons/icon.ico", Buffer.concat([icoHeader, ...icoImages.map(image => image.bytes)]), artwork, "ico-png-container", { sizes: icoSizes });

const icnsTypes = [["icp5", 32], ["icp6", 64], ["ic07", 128], ["ic08", 256], ["ic09", 512], ["ic10", 1024]];
const chunks = [];
for (const [type, size] of icnsTypes) {
  const image = await raster(size), chunk = Buffer.alloc(8);
  chunk.write(type, 0, "ascii"); chunk.writeUInt32BE(8 + image.bytes.length, 4);
  chunks.push(Buffer.concat([chunk, image.bytes]));
}
const icnsHeader = Buffer.alloc(8);
icnsHeader.write("icns", 0, "ascii"); icnsHeader.writeUInt32BE(8 + chunks.reduce((sum, chunk) => sum + chunk.length, 0), 4);
await output("desktop/src-tauri/icons/icon.icns", Buffer.concat([icnsHeader, ...chunks]), artwork, "icns-png-container", { sizes: icnsTypes.map(([, size]) => size) });
await output("desktop/public/luci.svg", canonical, `${artwork}/canonical/luci-four-ds-fourfold.svg`, "exact-approved-svg");
const lightSource = `${artwork}/derivatives/luci-fourfold-light-ink.svg`;
await output("desktop/public/luci-light.svg", await read(lightSource), lightSource, "exact-approved-svg");

const receipt = { schemaVersion: 1, approvalDate: "2026-10-07", applicationDate: "2026-10-14", canonicalSha256: hash(canonical),
  geometry: "Unchanged 256-square viewBox, silhouette, stroke proportions, aperture and built-in clear space; no background tile or animation.",
  renderer: { sharp: sharp.versions.sharp, vips: sharp.versions.vips },
  sourceChecksumsVerified: checksums.length, outputs };
await writeFile(path.join(root, "desktop/src/assets/brand/application-2026-10-14.json"), JSON.stringify(receipt, null, 2) + "\n");
console.log(JSON.stringify({ outputs: outputs.length, approvedSourceChecksums: checksums.length, rasterSizes: [...images.keys()].sort((a, b) => a - b), sharp: sharp.versions.sharp }));
