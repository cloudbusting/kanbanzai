// Scene "do": run the card. /do asks permission for each command and, at the end, how to publish the board; the first
// choice (Enter) is the recommended one each time.
async (page) => {
  await page.evaluate(() => { window.__scene = 'do'; });
  await page.keyboard.type(@COMMAND@, { delay: 60 });
  await page.waitForTimeout(800);
  await page.keyboard.type(' ' + @ARGS@, { delay: 40 });
  await page.waitForTimeout(1200);
  await page.keyboard.press('Enter');
  await page.waitForTimeout(40000);
  for (let i = 0; i < 9; i++) {
    await page.keyboard.press('Enter');
    await page.waitForTimeout(18000);
  }
  return 'ok';
}
