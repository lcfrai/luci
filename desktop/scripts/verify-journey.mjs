// Real local-app integration eval. Requires a disposable artifacts/eval-projects server.
// Never signs in, installs a runtime, contacts a cloud provider or creates cloud resources.
import { chromium } from "playwright";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import path from "node:path";
import assert from "node:assert/strict";

const base = process.env.LUCI_EVAL_URL || "http://127.0.0.1:8730";
const verifyCsp = process.env.LUCI_EVAL_CSP === "1";
const evidence = path.resolve(verifyCsp ? "../artifacts/csp-journey" : "../artifacts/journey-ui");
await mkdir(evidence, { recursive: true });
const initial = await fetch(`${base}/v1/setup/status`).then((r) => r.json());
assert.match(
  initial.config.projectsDir.replaceAll("\\", "/"),
  /\/artifacts\/eval-projects\/?$/,
);
const name = `luci-ui-eval-${Date.now()}`;
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({
  viewport: { width: 1440, height: 1000 },
  deviceScaleFactor: 1,
});
if (verifyCsp) {
  const config = JSON.parse(await readFile("src-tauri/tauri.conf.json", "utf8"));
  await page.route(base, async route => {
    const response = await route.fetch();
    await route.fulfill({ response, headers: {
      ...response.headers(), "content-security-policy": config.app.security.csp,
    } });
  });
}
const errors = [],
  checks = [],
  localFonts = [];
page.on("pageerror", (e) => errors.push(e.message));
if (verifyCsp) page.on("console", message => {
  if (/Content Security Policy|Refused to|violates/i.test(message.text())) errors.push(message.text());
});
page.on("response", response => {
  if (response.ok() && response.url().startsWith(`${base}/design-assets/fonts/`)) localFonts.push(response.url());
});
const screenshot = async (label) => {
  await page.screenshot({
    path: path.join(evidence, `${label}.png`),
    fullPage: true,
  });
};
try {
  await page.goto(base);
  if (initial.configured)
    await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("dialog").waitFor();
  await page.getByLabel(/Projects folder/).fill(initial.config.projectsDir);
  await page
    .getByRole("button", { name: "Save settings", exact: true })
    .click();
  await page
    .getByText("Settings saved. Your projects folder is ready.")
    .waitFor();
  await screenshot("01-setup");
  checks.push("Saved local setup against the real API");
  await page.getByRole("button", { name: "New project", exact: true }).click();
  await page.getByLabel("Project name", { exact: true }).fill(name);
  await page.getByRole("dialog")
    .getByRole("button", { name: "Create project", exact: true })
    .click();
  await page.getByRole("heading", { name: "What are we making?" }).waitFor();
  await page
    .getByLabel(/The idea/)
    .fill(
      "A local garden journal with saved notes and clear privacy boundaries.",
    );
  await page
    .getByLabel("Who it is for", { exact: true })
    .fill(
      "Home gardeners who prefer to keep their notes on their own machine.",
    );
  await page
    .getByLabel(/What good looks like/)
    .fill("Notes persist after restart\nThe app works without a cloud account");
  await page.getByRole("button", { name: "Save brief", exact: true }).click();
  await page.getByText(/Saved as revision 1/).waitFor();
  await screenshot("02-define");
  await page.reload();
  await page.getByLabel(/The idea/).waitFor();
  assert.match(
    await page.getByLabel(/The idea/).inputValue(),
    /garden journal/,
  );
  checks.push(
    "Created a real project, saved brief revision 1 and confirmed reload persistence",
  );
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.locator(".excalidraw canvas").first().waitFor();
  const canvas = await page.locator(".excalidraw canvas").first().boundingBox();
  assert.ok(canvas && canvas.width > 350 && canvas.height > 250);
  await page.mouse.click(canvas.x + canvas.width / 2, canvas.y + canvas.height / 2);
  await page.keyboard.press("2");
  await page.mouse.move(canvas.x + 240, canvas.y + 160);
  await page.mouse.down();
  await page.mouse.move(canvas.x + 440, canvas.y + 300, {steps: 8});
  await page.mouse.up();
  const savedDesign = page.waitForResponse(response=>response.request().method()==="POST" && response.url().endsWith("/design"));
  await page.getByRole("button", { name: "Save design", exact: true }).click();
  const designRecord = await (await savedDesign).json();
  assert.ok(designRecord.scene.elements.some(element=>element.type==="rectangle"));
  await page.getByText(/Design 1 · Brief 1/).waitFor();
  if (verifyCsp) {
    const fontLoaded = await page.evaluate(async () => {
      const loaded = await document.fonts.load('16px "Excalifont"', "Luci");
      return loaded.length > 0 && loaded.every(font => font.status === "loaded");
    });
    assert.ok(fontLoaded && localFonts.length > 0, "Bundled fonts load under the production desktop CSP");
    checks.push("Loaded bundled design fonts with the production desktop CSP and no policy violations");
  }
  await screenshot("03-design");
  await page.getByRole("button", { name: "Develop", exact: true }).click();
  await page.getByRole("button", { name: "Files", exact: true }).click();
  await page.getByRole("button", { name: /LUCI.md/ }).click();
  await page.getByText(/A local garden journal/, { exact: false }).waitFor();
  await screenshot("04-files");
  checks.push("Opened saved LUCI.md through the bounded file browser");
  const projectPath = path.join(initial.config.projectsDir, name);
  execFileSync("git", [
    "-C",
    projectPath,
    "config",
    "user.name",
    "Luci integration eval",
  ]);
  execFileSync("git", [
    "-C",
    projectPath,
    "config",
    "user.email",
    "eval@example.invalid",
  ]);
  await page.getByRole("button", { name: "Git", exact: true }).click();
  await page.getByLabel("Commit message").fill("Save the garden journal brief");
  await page
    .getByRole("button", { name: "Commit changes", exact: true })
    .click();
  await page.getByText("Changes committed locally.").waitFor();
  assert.match(
    execFileSync("git", ["-C", projectPath, "log", "-1", "--format=%s"], {
      encoding: "utf8",
    }),
    /garden journal brief/,
  );
  await screenshot("05-git");
  checks.push(
    "Committed the brief through the real Git endpoint using fixture-local identity",
  );
  await page.getByRole("button", { name: "Design", exact: true }).click();
  await page.getByText(/Design 1 · Brief 1/).waitFor();
  checks.push("Reopened the persisted design linked to brief revision 1");
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await page
    .getByRole("button", { name: "Build local preview", exact: true })
    .waitFor();
  await page.getByRole("button", { name: "Your cloud", exact: true }).click();
  await page.getByRole("heading", { name: "Ship to your cloud" }).waitFor();
  await screenshot("06-deploy");
  checks.push(
    "Rendered deployment inputs from live project and connection state without submitting cloud work",
  );
  await page
    .getByRole("button", { name: "Cloud settings", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "A small home for what you build." })
    .waitFor();
  await page
    .getByRole("heading", { name: "Finish your deployment connection" })
    .waitFor();
  await screenshot("07-cloud");
  checks.push(
    "Rendered guided cloud planning, server verification and connection controls",
  );
  assert.deepEqual(errors, []);
  await writeFile(
    path.join(evidence, "results.json"),
    JSON.stringify(
      {
        status: "passed",
        name,
        checks,
        errors,
        screenshots: 7,
        cloudActions: 0,
        productionCsp: verifyCsp,
        localFonts: [...new Set(localFonts)],
      },
      null,
      2,
    ),
  );
  console.log(JSON.stringify({ status: "passed", name, checks, errors }));
} catch (error) {
  await screenshot("failure");
  await writeFile(
    path.join(evidence, "failure-dom.txt"),
    await page.locator("body").innerText(),
  );
  await writeFile(
    path.join(evidence, "results.json"),
    JSON.stringify(
      { status: "failed", name, checks, errors, error: String(error) },
      null,
      2,
    ),
  );
  throw error;
} finally {
  await browser.close();
}
