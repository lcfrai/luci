// Render every marketing HTML document to a sibling A4 PDF.
//
// Uses the Playwright already installed for the phase 2 e2e harness
// (desktop/node_modules), so this adds no dependency of its own.
//
//   node docs/marketing/_assets/render.mjs               # render all
//   node docs/marketing/_assets/render.mjs <name>.html   # render one
//
// Files whose name starts with "_" are templates and are skipped, as is
// anything under _assets/. A first-page PNG is written beside the PDF so the
// cover can be eyeballed without opening a viewer.

import { chromium } from "../../../desktop/node_modules/playwright/index.mjs";
import { readdirSync, statSync } from "node:fs";
import { pathToFileURL } from "node:url";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..");

function targets() {
  const only = process.argv[2];
  if (only) return [path.resolve(ROOT, only)];
  return readdirSync(ROOT)
    .filter((n) => n.endsWith(".html") && !n.startsWith("_"))
    .map((n) => path.join(ROOT, n))
    .filter((p) => statSync(p).isFile());
}

const files = targets();
if (files.length === 0) {
  console.log("no HTML targets found under", ROOT);
  process.exit(0);
}

const browser = await chromium.launch();
let ok = 0;
for (const html of files) {
  const pdf = html.replace(/\.html$/, ".pdf");
  const page = await browser.newPage({
    viewport: { width: 840, height: 1188 },
    deviceScaleFactor: 2,
  });
  await page.goto(pathToFileURL(html).href, { waitUntil: "load" });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(500); // let the self-hosted webfonts settle
  await page.pdf({
    path: pdf,
    format: "A4",
    printBackground: true,
    margin: { top: "0", right: "0", bottom: "0", left: "0" },
  });
  const cover = await page.$("section.page");
  if (cover) await cover.screenshot({ path: pdf.replace(/\.pdf$/, "-cover.png") });
  const pages = await page.$$eval("section.page", (n) => n.length);
  await page.close();
  ok += 1;
  const kb = Math.round(statSync(pdf).size / 1024);
  console.log(`  PDF  ${path.basename(pdf)}  ${pages} pages, ${kb} KB`);
}
await browser.close();
console.log(`rendered ${ok}/${files.length} document(s)`);
