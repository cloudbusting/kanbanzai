// Scene "layout": the terminal beside the board (panel on the right, widened), and the setup committed.
async (page) => {
  await page.evaluate(() => { window.__scene = 'layout'; });
  await page.keyboard.press('Control+Backquote');
  await page.waitForTimeout(3000);
  await page.keyboard.press('F1');
  await page.waitForTimeout(800);
  await page.keyboard.type('View: Move Panel Right', { delay: 30 });
  await page.waitForTimeout(1200);
  await page.keyboard.press('Enter');
  await page.waitForTimeout(2000);
  const plan = page.getByRole('tab', { name: /plan 1\.txt/ });
  if (await plan.count()) {
    await plan.hover();
    await plan.locator('.codicon-close, .action-label').first().click();
    await page.waitForTimeout(1500);
  }
  await page.mouse.move(980, 400);
  await page.mouse.down();
  await page.mouse.move(800, 400, { steps: 8 });
  await page.mouse.move(600, 400, { steps: 8 });
  await page.mouse.up();
  await page.waitForTimeout(1000);
  await page.locator('.xterm').first().click();
  await page.keyboard.type('git add -A && git commit -q -m "Set up Kanbanzai" && git log --oneline', { delay: 25 });
  await page.keyboard.press('Enter');
  await page.waitForTimeout(2500);
  return 'ok';
}
