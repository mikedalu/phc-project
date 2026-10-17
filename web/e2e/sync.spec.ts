import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { signIn } from './helpers';

/**
 * The Sprint 2 proof: two workstations converge through the server.
 *
 * ═══ WHY TWO CONTEXTS ═══
 *
 * Two TABS would share one IndexedDB and therefore one device — which would pass
 * while proving nothing. Two browser CONTEXTS have separate storage, separate
 * device keys and separate sessions, which is two physical workstations as far
 * as the server is concerned.
 */

async function openProbe(page: Page): Promise<void> {
  await page.goto('/dev/sync');
  await expect(page.getByLabel('Probe title')).toBeVisible();
}

async function addProbe(page: Page, title: string): Promise<void> {
  await page.getByLabel('Probe title').fill(title);
  await page.getByRole('button', { name: 'Add row' }).click();
}

/** The row in the probe table whose text contains `title`. */
function probeRow(page: Page, title: string) {
  return page.locator('tbody tr').filter({ hasText: title });
}

test.describe('sync', () => {
  test('two devices converge in both directions, and survive being offline', async ({ browser }) => {
    const suffix = Date.now().toString();
    const fromA = `A says hello ${suffix}`;
    const fromB = `B replies ${suffix}`;

    // Two contexts = two workstations. Different users prove attribution too.
    const contextA = await browser.newContext();
    const contextB = await browser.newContext();

    const pageA = await signIn(contextA, 'DEMO/STF/0002'); // front desk
    const pageB = await signIn(contextB, 'DEMO/STF/0003'); // nurse

    await openProbe(pageA);
    await openProbe(pageB);

    // ── A writes, A syncs ──────────────────────────────────────────
    await addProbe(pageA, fromA);
    await expect(probeRow(pageA, fromA)).toContainText('pending');
    await expect(probeRow(pageA, fromA)).toContainText('synced');

    // ── B receives it ──────────────────────────────────────────────
    await pageB.getByRole('button', { name: 'Sync now' }).click();
    await expect(pageB.getByText(fromA)).toBeVisible();

    // ── B writes, A receives ───────────────────────────────────────
    await addProbe(pageB, fromB);
    await expect(probeRow(pageB, fromB)).toContainText('synced');

    await pageA.getByRole('button', { name: 'Sync now' }).click();
    await expect(pageA.getByText(fromB)).toBeVisible();

    // ── A goes offline, queues three, then drains ──────────────────
    await contextA.setOffline(true);
    for (const n of ['offline one', 'offline two', 'offline three']) {
      await addProbe(pageA, `${n} ${suffix}`);
    }

    // The point of the badge: the work is visibly saved, and visibly unsent.
    await expect(pageA.getByText(/3 waiting/)).toBeVisible();

    await contextA.setOffline(false);
    // Three pushes at a few seconds each; give it room.
    await expect(pageA.getByText(/0 waiting/)).toBeVisible({ timeout: 90_000 });

    for (const n of ['offline one', 'offline two', 'offline three']) {
      await expect(probeRow(pageA, `${n} ${suffix}`)).toContainText('synced');
    }

    // And B sees all three, which is the whole point of the sprint.
    await pageB.getByRole('button', { name: 'Sync now' }).click();
    await expect(pageB.getByText(`offline three ${suffix}`)).toBeVisible();

    await contextA.close();
    await contextB.close();
  });
});
