// Controlled native browser selector regression; no Luci service, Docker or provider.
import test from 'node:test';
import assert from 'node:assert/strict';
import { join } from 'node:path';
const repo=process.env.LUCI_ACCEPTANCE_REPO;
assert(repo,'Set LUCI_ACCEPTANCE_REPO to the checkout with installed Playwright');
const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
test('nested populated textarea has exact accessible role name while label text includes its initial content',async()=>{
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-proxy-server']});
 try{
  const page=await browser.newPage();
  await page.setContent('<label class="luci-field">The idea<textarea rows="4">Build Escaped UI renderer with explicit verified behavior.</textarea></label>');
  assert.equal(await page.getByLabel('The idea',{exact:true}).count(),0);
  const field=page.getByRole('textbox',{name:'The idea',exact:true});
  assert.equal(await field.count(),1);
  assert.equal(await field.inputValue(),'Build Escaped UI renderer with explicit verified behavior.');
  await field.fill('Unsent fixture draft');
  assert.equal(await field.inputValue(),'Unsent fixture draft');
 }finally{await browser.close();}
});
