// Scene "triage": start Claude Code, type the triage prompt (the command first, so its menu shows), approve what it
// asks, and wait for the card. The permission prompts are answered blind with Enter (the first choice, "Yes"); the
// demo project's state, not the screen, says whether it worked (demo.sh verify).
async (page) => {
  await page.evaluate(() => { window.__scene = 'triage'; });
  await page.keyboard.type('claude', { delay: 60 });
  await page.keyboard.press('Enter');
  await page.waitForTimeout(12000);
  await page.keyboard.type(@COMMAND@, { delay: 70 });
  await page.waitForTimeout(2000);
  await page.keyboard.type(' ' + @ARGS@, { delay: 35 });
  await page.waitForTimeout(1500);
  await page.keyboard.press('Enter');
  for (let i = 0; i < 4; i++) {
    await page.waitForTimeout(20000);
    await page.keyboard.press('Enter');
  }
  await page.waitForTimeout(15000);
  return 'ok';
}
