import { expect, test } from '@playwright/test';
import { forceSync, signIn } from './helpers';

/**
 * Sprint 4's proof: a patient moves through the workflow, and the move is
 * visible on a SECOND workstation.
 *
 * The nurse registers, records vitals and sends the patient to the doctor on
 * device A. The doctor opens the consultation on device B — a separate browser
 * context, so a separate device key, session and replica.
 */
// This spec drives two devices through several Neon round-trips; the default
// 120 s is not enough on a cold database.
test.describe.configure({ timeout: 300_000 });

test.describe('queue', () => {
  test('vitals move a patient to the doctor, on another device', async ({ browser }) => {
    const suffix = Date.now().toString().slice(-6);
    const firstName = `Q${suffix}`;
    const surname = 'Workflow';

    const contextA = await browser.newContext();
    const contextB = await browser.newContext();

    const nurse = await signIn(contextA, 'DEMO/STF/0003');
    const doctor = await signIn(contextB, 'DEMO/STF/0004');

    // ── A: register a patient ──────────────────────────────────────
    await nurse.goto('/patients/new');
    await nurse.getByLabel('First name').fill(firstName);
    await nurse.getByLabel('Surname').fill(surname);
    await nurse.getByLabel('Sex').selectOption('female');
    await nurse.locator('input[type=date]').fill('1988-07-09');
    await nurse.getByRole('button', { name: 'Register patient' }).click();
    await expect(nurse).toHaveURL(/\/patients\/[0-9a-f-]+$/);
    await expect(nurse.getByText('Synced to server')).toBeVisible({ timeout: 90_000 });

    // ── A: start the visit and record vitals ──────────────────────
    await nurse.getByRole('button', { name: 'Start visit' }).click();
    await expect(nurse).toHaveURL('/queue');
    await expect(nurse.getByText(`${firstName} ${surname}`)).toBeVisible();

    // Scope every interaction to THIS patient's card: the shared dev database
    // already holds other patients, so a page-wide locator matches several.
    const nurseCard = nurse.locator('article').filter({ hasText: `${firstName} ${surname}` });
    await nurseCard.getByRole('button', { name: 'Record vitals' }).click();
    const vitals = nurseCard.locator('input');
    await vitals.nth(0).fill('118');
    await vitals.nth(1).fill('76');
    await vitals.nth(2).fill('36.9');
    await vitals.nth(3).fill('84');
    await vitals.nth(4).fill('18');
    await vitals.nth(5).fill('98');
    await vitals.nth(6).fill('165');
    await vitals.nth(7).fill('61');
    await nurse.getByRole('button', { name: 'Save vitals' }).click();

    // The card moves to the doctor's column.
    await expect(
      nurse.locator('section').filter({ hasText: 'Waiting for doctor' }).getByText(`${firstName} ${surname}`),
    ).toBeVisible();

    await nurseCard.getByRole('button', { name: 'Send to doctor' }).click();

    await expect(nurseCard.getByRole('button', { name: 'Open consultation' })).toBeVisible({ timeout: 30_000 });

    // ── B: the doctor pulls and opens the consultation ────────────
    await forceSync(doctor);
    await doctor.goto('/queue');
    const doctorCard = doctor.locator('article').filter({ hasText: `${firstName} ${surname}` });
    await expect(doctorCard).toBeVisible({ timeout: 90_000 });

    await doctorCard.getByRole('button', { name: 'Open consultation' }).click();

    // The patient leaves the waiting column and enters consultation.
    await expect(
      doctor.locator('section').filter({ hasText: 'In consultation' }).getByText(`${firstName} ${surname}`),
    ).toBeVisible({ timeout: 30_000 });

    // ── A sees the doctor's move after a sync ────────────────────
    await forceSync(nurse);
    await nurse.goto('/queue');
    await expect(
      nurse.locator('section').filter({ hasText: 'In consultation' }).getByText(`${firstName} ${surname}`),
    ).toBeVisible({ timeout: 90_000 });

    await contextA.close();
    await contextB.close();
  });
});
