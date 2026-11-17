// Opens the editor on the demo project, trusts the folder and clears the Welcome tab.
async (page) => {
  await page.goto('@URL@');
  await page.waitForTimeout(4000);
  await page.evaluate(() => { window.__scene = 'setup'; });
  const trust = page.getByRole('button', { name: 'Yes, I trust the authors' });
  if (await trust.count()) await trust.click();
  await page.waitForTimeout(2500);
  await page.keyboard.press('Control+w');
  await page.waitForTimeout(500);
  return await page.locator('.statusbar').innerText();
}
