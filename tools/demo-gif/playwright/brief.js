// Scene "brief": click a card on the board to open its brief in an editor tab, then go back to the Board tab. The card
// is found inside the board's webview; if that fails, Quick Open opens the same file.
async (page) => {
  await page.evaluate(() => { window.__scene = 'brief'; });
  const board = page.frameLocator('iframe.webview').frameLocator('iframe#active-frame');
  const card = board.getByText('Weekly plan').first();
  try {
    await card.click({ timeout: 5000 });
  } catch (e) {
    await page.keyboard.press('Control+p');
    await page.waitForTimeout(800);
    await page.keyboard.type('TASK-03', { delay: 40 });
    await page.waitForTimeout(1200);
    await page.keyboard.press('Enter');
  }
  await page.waitForTimeout(6000);
  await page.getByRole('tab', { name: 'Board' }).click();
  await page.waitForTimeout(4000);
  return 'ok';
}
