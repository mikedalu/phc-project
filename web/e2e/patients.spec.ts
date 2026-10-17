import { expect, test } from '@playwright/test';
import { forceSync, signIn } from './helpers';

/**
 * Sprint 3's proof: a patient registered with the network OFF reaches another
 * workstation.
 *
 * Two browser CONTEXTS, not two tabs — separate storage means separate device
 * keys and separate sessions, which is two workstations as far as the server is
 * concerned.
 */
test.describe('patient registration', () => {
  test('registers offline and reaches another workstation', async ({ browser }) => {
    const suffix = Date.now().toString().slice(-6);
    const firstName = `Chinwe${suffix}`;
    const surname = 'Okonkwo';

    const contextA = await browser.newContext();
    const contextB = await browser.newContext();

    const pageA = await signIn(contextA, 'DEMO/STF/0002'); // front desk
    const pageB = await signIn(contextB, 'DEMO/STF/0003'); // nurse

    // ── A fills the form, then loses the network BEFORE submitting ──
    await pageA.goto('/patients/new');
    await pageA.getByLabel('First name').fill(firstName);
    await pageA.getByLabel('Surname').fill(surname);
    await pageA.getByLabel('Sex').selectOption('female');
    await pageA.locator('input[type=date]').fill('1990-01-15');

    await contextA.setOffline(true);
    await pageA.getByRole('button', { name: 'Register patient' }).click();

    // Registered locally and attributed to nobody's network connection.
    await expect(pageA).toHaveURL(/\/patients\/[0-9a-f-]+$/);
    // exact, because the sync indicator and the toast also contain the phrase.
    await expect(pageA.getByText('Saved on this device', { exact: true })).toBeVisible();
    await expect(pageA.getByText(firstName)).toBeVisible();

    // ── Reconnect: the engine drains it ────────────────────────────
    await contextA.setOffline(false);
    await expect(pageA.getByText('Synced to server')).toBeVisible({ timeout: 120_000 });

    // ── B pulls and sees the same patient ──────────────────────────
    await forceSync(pageB);
    await pageB.goto('/patients');
    await pageB.getByLabel('Search patients').fill(firstName);

    await expect(pageB.getByText(`${firstName} ${surname}`)).toBeVisible({ timeout: 60_000 });

    await contextA.close();
    await contextB.close();
  });
});
