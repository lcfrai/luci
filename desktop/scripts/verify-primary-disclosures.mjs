// Focused shipped-CSS guard for the primary conversation disclosures.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { chromium } from 'playwright';

const css = (await readFile(new URL('../src/components/orchestrator/orchestrator.css', import.meta.url), 'utf8'))
  .replace(/^@import[^\n]*\n/, '');
const browser = await chromium.launch({ headless: true, executablePath: process.env.LUCI_CHROMIUM_EXECUTABLE || '/usr/bin/chromium' });
try {
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    const page = await browser.newPage({ viewport });
    await page.setContent(`<main class="orchestrator orchestrator-primary orchestrator-open">
      <div class="orchestrator-body">
        <details class="orchestrator-agent"><summary>Agent and model</summary><p>Agent settings</p></details>
        <details class="orchestrator-status-details"><summary>Project context and refresh</summary><p>Context settings</p></details>
      </div>
    </main>`);
    await page.addStyleTag({ content: css });
    for (const name of ['Agent and model', 'Project context and refresh']) {
      const summary = page.getByText(name, { exact: true });
      const details = summary.locator('xpath=..');
      const closed = await summary.evaluate(element => ({
        display: getComputedStyle(element).display,
        marker: getComputedStyle(element).listStyleType,
        height: element.getBoundingClientRect().height,
      }));
      assert.equal(closed.display, 'list-item', `${name} must retain its native disclosure marker`);
      assert.equal(closed.marker, 'disclosure-closed', `${name} must show a closed disclosure marker`);
      assert.ok(closed.height >= 44, `${name} must have a 44px hit target`);
      await summary.click();
      assert.equal(await details.evaluate(element => element.open), true);
      assert.equal(await summary.evaluate(element => getComputedStyle(element).listStyleType), 'disclosure-open');
      await summary.press('Enter');
      assert.equal(await details.evaluate(element => element.open), false);
      await summary.press('Space');
      assert.equal(await details.evaluate(element => element.open), true);
      await summary.press('Space');
      assert.equal(await details.evaluate(element => element.open), false);
    }
    console.log(`PASS ${viewport.width}x${viewport.height}: markers, 44px targets, pointer and keyboard toggle`);
    await page.close();
  }
} finally {
  await browser.close();
}
