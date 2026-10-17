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

/** Forces a drain+pull without waiting for the 60 s poll. */
export async function forceSync(page: Page): Promise<void> {
  await page.goto('/dev/sync');
  await page.getByRole('button', { name: 'Sync now' }).click();
}
