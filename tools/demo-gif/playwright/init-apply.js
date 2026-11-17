// Scene "init": the status bar offers Kanbanzai: Init; choose Claude Code only, read the plan, click Apply, open the Board.
async (page) => {
  await page.evaluate(() => { window.__scene = 'init'; });
  await page.waitForTimeout(2500);
  await page.locator('.statusbar').getByText('Kanbanzai: Init').click();
  await page.waitForTimeout(2500);
  await page.getByText('OpenCode').first().click();
  await page.waitForTimeout(1500);
  await page.getByRole('button', { name: 'OK' }).click();
  await page.waitForTimeout(5000);
  await page.locator('.statusbar').getByText('Kanbanzai: Apply').click();
  await page.waitForTimeout(8000);
  await page.keyboard.press('Control+b');
  await page.waitForTimeout(1500);
  await page.locator('.statusbar').getByText('Board').click();
  await page.waitForTimeout(3500);
  return await page.locator('.statusbar').innerText();
}
