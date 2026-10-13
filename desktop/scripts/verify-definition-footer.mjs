// Focused pointer/layout regression for the Discover interview scrollport.
// The buttons and footer use the shipped definition stylesheet; no force click
// or enlarged viewport hides an intercepted target.
import { chromium } from 'playwright';
import { readFile } from 'node:fs/promises';
import assert from 'node:assert/strict';

const css = (await readFile(new URL('../src/components/definition/definition.css', import.meta.url), 'utf8'))
  .replace(/^@import[^\n]*\n/, '');
const browser = await chromium.launch({ headless: true, executablePath: process.env.LUCI_CHROMIUM_EXECUTABLE || '/usr/bin/chromium' });
try {
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 700 }, { width: 760, height: 500 }]) {
    const page = await browser.newPage({ viewport });
    await page.setContent(`<style>
      * { box-sizing: border-box; }
      body { margin: 0; overflow: hidden; font: 14px sans-serif; }
      .journey-page { block-size: 100vh; overflow: auto; padding: 34px 38px; }
      .luci-field { display: block; }
      textarea { display: block; width: 100%; min-height: 136px; }
      button { min-height: 36px; }
      .journey-actions { display: flex; gap: 12px; }
    </style><main class="journey-page"><section class="definition-workspace">
      <div class="definition-layout"><div class="definition-content"><fieldset class="definition-fields">
        <div class="definition-interview">
          <label class="luci-field">Starting point<select><option>A new idea</option></select></label>
          <p>Question <span id="step">1</span> of 5</p>
          <h2>What are you building?</h2>
          <p>Describe the outcome, the people who need it, and the context.</p>
          <label class="luci-field">Answer<textarea rows="7"></textarea></label>
          <div style="block-size: 340px" aria-hidden="true"></div>
          <div class="journey-actions"><button id="back" type="button">Back</button><button id="next" type="button">Next question</button></div>
          <p>You can switch to the specification or save at any time.</p>
        </div>
      </fieldset></div></div>
      <footer class="definition-footer"><div class="journey-actions"><button id="save" type="button">Save definition</button></div><p>1 requirement · 0 definition questions</p></footer>
    </section></main>`);
    await page.addStyleTag({ content: css });
    assert.equal(await page.locator('.definition-footer').evaluate(element => getComputedStyle(element).position), 'static',
      `The action footer must stay in the scroll flow at ${viewport.width}x${viewport.height}`);
    await page.evaluate(() => {
      let step = 1;
      const answer = document.querySelector('textarea');
      const update = () => {
        document.querySelector('#step').textContent = String(step);
        document.querySelector('#next').textContent = step === 5 ? 'Review specification' : 'Next question';
      };
      document.querySelector('#next').addEventListener('click', () => { step++; update(); });
      document.querySelector('#back').addEventListener('click', () => { step--; update(); });
      document.querySelector('#save').addEventListener('click', () => { document.querySelector('#save').dataset.saved = answer.value; });
    });
    await page.getByRole('textbox', { name: 'Answer' }).fill('An answer kept while navigating');
    await page.getByRole('button', { name: 'Next question' }).click({ timeout: 1500 });
    assert.equal(await page.locator('#step').textContent(), '2');
    await page.getByRole('button', { name: 'Back' }).click({ timeout: 1500 });
    assert.equal(await page.locator('#step').textContent(), '1');
    for (let step = 1; step < 5; step++) await page.getByRole('button', { name: 'Next question' }).click({ timeout: 1500 });
    await page.getByRole('button', { name: 'Review specification' }).click({ timeout: 1500 });
    assert.equal(await page.locator('#step').textContent(), '6');
    await page.getByRole('button', { name: 'Save definition' }).click({ timeout: 1500 });
    assert.equal(await page.locator('#save').getAttribute('data-saved'), 'An answer kept while navigating');
    console.log(`PASS ${viewport.width}x${viewport.height}: pointer Next/Back/final review/save`);
    await page.close();
  }
} finally {
  await browser.close();
}
