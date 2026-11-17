// Starts the recording: about every 0.4 s it saves a screenshot of the page as <epoch-ms>-<scene>.png in @FRAMES@.
// The scene is whatever the step scripts last put in window.__scene (they set it as their first act), so the film is
// cut by scene name, not by frame number. The timer lives in the browser tool's process and runs until the browser
// is closed; run this once, first, after the editor page is open.
async (page) => {
  const dir = '@FRAMES@';
  const state = { busy: false };
  setInterval(async () => {
    if (state.busy) return;
    state.busy = true;
    try {
      const scene = (await page.evaluate(() => window.__scene || 'setup')) || 'setup';
      await page.screenshot({ path: `${dir}/${Date.now()}-${scene}.png`, type: 'png' });
    } catch (e) { /* the page is closed or navigating: skip a frame */ }
    state.busy = false;
  }, 400);
  await page.waitForTimeout(1500);
  return 'recording to ' + dir;
}
