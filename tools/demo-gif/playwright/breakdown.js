// Scenes "breakdown" and "asking": commit the triaged card (the breakdown wants a clean tree), run the breakdown prompt,
// show the board while the session asks to approve the split ("asking"), approve it and every prompt after it.
async (page) => {
  await page.evaluate(() => { window.__scene = 'breakdown'; });
  await page.keyboard.type('! git add -A && git commit -q -m "triaged" && git log --oneline | head -1', { delay: 30 });
  await page.keyboard.press('Enter');
  await page.waitForTimeout(4000);
  await page.keyboard.type(@COMMAND@, { delay: 60 });
  await page.waitForTimeout(800);
  await page.keyboard.type(' ' + @ARGS@, { delay: 35 });
  await page.waitForTimeout(1200);
  await page.keyboard.press('Enter');
  await page.waitForTimeout(40000);
  await page.evaluate(() => { window.__scene = 'asking'; });
  await page.waitForTimeout(9000);
  await page.evaluate(() => { window.__scene = 'breakdown'; });
  for (let i = 0; i < 6; i++) {
    await page.keyboard.press('Enter');
    await page.waitForTimeout(20000);
  }
  return 'ok';
}
