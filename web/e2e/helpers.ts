import { expect, type BrowserContext, type Page } from '@playwright/test';

export const PASSWORD = 'ChangeMe!2026';

/** Signs in and waits for the dashboard. */
export async function signIn(context: BrowserContext, staffNumber: string): Promise<Page> {
  const page = await context.newPage();
  await page.goto('/login');

  await page.getByLabel('Staff number').fill(staffNumber);
  await page.getByLabel('Password').fill(PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page).toHaveURL('/', { timeout: 45_000 });
  return page;
}

/**
 * Forces a drain+pull without waiting for the 60 s poll, and WAITS for it.
 *
 * Clicking the button only schedules work, so a test that asserts immediately
 * afterwards is racing the engine. Awaiting `runSync()` directly is
 * deterministic: it resolves when the run has finished, and it still works when
 * the button is disabled because a run is already in flight.
 */
export async function forceSync(page: Page): Promise<void> {
  await page.goto('/dev/sync');
  const button = page.getByRole('button', { name: 'Sync now' });
  if (await button.isEnabled()) {
    await button.click();
  }
  await page.evaluate(async () => {
    const engine = (await import('/src/sync/engine.ts')) as { runSync: () => Promise<void> };
    await engine.runSync();
  });
}


/**
 * Waits for the FIRST-RUN snapshot bootstrap to finish.
 *
 * A new device fills itself table by table from /sync/snapshot, which takes a
 * few seconds per table on a cold database. Acting before that lands means the
 * test presses Sync now while the engine is already busy, so the work it waits
 * for is queued behind a bootstrap that has not finished.
 */
export async function waitForBootSync(page: Page): Promise<void> {
  await expect(page.locator('body')).toContainText(/All records synced|Offline/, {
    timeout: 180_000,
  });
}
