import { expect, test, type Page } from '@playwright/test';
import { forceSync, signIn } from './helpers';

/**
 * Sprint 5's proof: a doctor's conclusion and order reach the bench, and the
 * result reaches the doctor — across two devices.
 *
 * The interesting part is that NOTHING here waits for a network round trip on
 * the clinical path: the diagnosis and the order are written to the device, and
 * replication carries them. The test drives sync explicitly so it stays fast.
 */
test.describe.configure({ timeout: 420_000 });

/** Waits for the first-run snapshot bootstrap to finish on a fresh device. */
async function waitForBootSync(page: Page): Promise<void> {
  await expect(page.locator('body')).toContainText(/All records synced|Offline/, {
    timeout: 180_000,
  });
}

test.describe('consultation', () => {
  test('diagnosis and lab order reach the bench, and the result comes back', async ({ browser }) => {
    const suffix = Date.now().toString().slice(-6);
    const firstName = `Cons${suffix}`;
    const surname = 'Workflow';

    const contextA = await browser.newContext();
    const contextB = await browser.newContext();

    const doctor = await signIn(contextA, 'DEMO/STF/0004');
    const lab = await signIn(contextB, 'DEMO/STF/0005');

    await waitForBootSync(doctor);
    await waitForBootSync(lab);

    // ── A: register, start a visit, run it to the doctor ───────────
    await doctor.goto('/patients/new');
    await doctor.getByLabel('First name').fill(firstName);
    await doctor.getByLabel('Surname').fill(surname);
    await doctor.getByLabel('Sex').selectOption('female');
    await doctor.locator('input[type=date]').fill('1979-02-11');
    await doctor.getByRole('button', { name: 'Register patient' }).click();
    await expect(doctor).toHaveURL(/\/patients\/[0-9a-f-]+$/);

    await doctor.getByRole('button', { name: 'Start visit' }).click();
    await expect(doctor).toHaveURL('/queue');

    const card = doctor.locator('article').filter({ hasText: `${firstName} ${surname}` });
    await card.getByRole('button', { name: 'Record vitals' }).click();
    const vitals = card.locator('input');
    await vitals.nth(0).fill('122');
    await vitals.nth(1).fill('80');
    await vitals.nth(2).fill('37.0');
    await vitals.nth(3).fill('88');
    await card.getByRole('button', { name: 'Save vitals' }).click();

    await card.getByRole('button', { name: 'Send to doctor' }).click();
    await card.getByRole('button', { name: 'Open consultation' }).click();

    await expect(doctor).toHaveURL(/\/consultations\/[0-9a-f-]+$/);

    // ── A: a diagnosis, chosen from the ICD-10 shortlist ───────────
    const diagnosisInput = doctor.locator('section').filter({ hasText: 'Diagnosis' }).locator('input').first();
    await diagnosisInput.fill('malar');
    await expect(doctor.getByRole('button', { name: /Unspecified malaria/ })).toBeVisible();
    await diagnosisInput.press('Enter');
    await doctor.getByRole('button', { name: 'Record diagnosis' }).click();
    await expect(doctor.getByText('Unspecified malaria')).toBeVisible();

    // ── A: order a test ────────────────────────────────────────────
    await doctor.getByRole('button', { name: 'Order test' }).click();
    // Assert on the COUNT in the panel heading: 'Malaria Parasite' also appears
    // as an <option> in the test selector, so a plain text match is ambiguous.
    await expect(doctor.getByRole('heading', { name: /Lab orders \(1\)/ })).toBeVisible();

    // ── B: the bench sees the order ────────────────────────────────
    await forceSync(lab);
    await lab.goto('/lab');
    const order = lab.locator('article').filter({ hasText: firstName });
    await expect(order).toBeVisible({ timeout: 120_000 });

    await order.getByRole('button', { name: 'Collect specimen' }).click();

    // It moves to the bench column.
    const awaiting = lab.locator('article').filter({ hasText: firstName });
    await awaiting.getByRole('button', { name: 'Enter result' }).click();

    const resultInputs = awaiting.locator('input');
    await resultInputs.nth(1).fill('Positive');
    await awaiting.getByRole('button', { name: 'Save result' }).click();

    await expect(lab.getByText(/Malaria Parasite: Positive/).first()).toBeVisible({ timeout: 30_000 });

    // Push the result BEFORE the doctor pulls. The lab's own screen updates from
    // its local database the instant the row is written, so asserting on that
    // screen says nothing about whether the server has it yet — an earlier
    // version of this test raced exactly here.
    await forceSync(lab);

    // ── A: the doctor sees the result ──────────────────────────────
    await forceSync(doctor);
    await doctor.goto('/lab');
    await expect(doctor.getByText(/Malaria Parasite: Positive/).first()).toBeVisible({ timeout: 180_000 });

    await contextA.close();
    await contextB.close();
  });
});
