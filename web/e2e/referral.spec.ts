import { expect, test } from '@playwright/test';
import { signIn, waitForBootSync } from './helpers';

/**
 * Sprint 8's proof: a referral issued inside the system is readable by a machine
 * that has never seen the system.
 *
 * The second half of this test is the point. The note is opened in a context with
 * NO session, NO device key and NO local database — which is exactly the position
 * of a clinician at another facility holding a printed QR code.
 */
test.describe.configure({ timeout: 300_000 });

const API = 'http://127.0.0.1:8080/api/v1';

test.describe('referral', () => {
  test('a note issued here is readable by a stranger, and cannot be forged', async ({ browser }) => {
    // ── setup: a patient with an open encounter ────────────────────
    const login = await fetch(`${API}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        staffNumber: 'DEMO/STF/0002',
        password: 'ChangeMe!2026',
        deviceId: crypto.randomUUID(),
        deviceName: 'e2e-setup',
      }),
    });
    const token = (await login.json()).data.accessToken as string;
    const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` };

    const suffix = Date.now().toString().slice(-5);
    const patientRes = await fetch(`${API}/patients`, {
      method: 'POST',
      headers: auth,
      body: JSON.stringify({
        firstName: 'Ref',
        surname: `E2E${suffix}`,
        gender: 'male',
        dateOfBirth: '1976-06-06',
      }),
    });
    const patient = (await patientRes.json()).data.patient as { id: string };

    const encounterRes = await fetch(`${API}/encounters`, {
      method: 'POST',
      headers: auth,
      body: JSON.stringify({ patientId: patient.id, encounterType: 'opd' }),
    });
    const encounter = (await encounterRes.json()).data.encounter as { id: string };

    // ── the doctor issues the referral through the UI ─────────────
    const context = await browser.newContext();
    const doctor = await signIn(context, 'DEMO/STF/0004');
    await waitForBootSync(doctor);

    // The encounter was created over REST, so it arrives by PULL (the change-log
    // trigger from D59). Sync once to be sure it is on this device.
    await doctor.evaluate(async () => {
      const engine = (await import('/src/sync/engine.ts')) as { runSync: () => Promise<void> };
      await engine.runSync();
    });

    await doctor.goto(`/referrals/new?encounterId=${encounter.id}`);
    await expect(doctor.getByText('Refer a patient')).toBeVisible({ timeout: 60_000 });

    const fields = doctor.locator('section input');
    await fields.nth(0).fill('General Hospital, Enugu');
    await fields.nth(1).fill('Needs surgical review');
    await fields.nth(2).fill('Acute abdomen');
    await doctor.getByRole('button', { name: 'Issue referral' }).click();

    await expect(doctor.getByText('Referral issued')).toBeVisible({ timeout: 60_000 });
    await expect(doctor.locator('canvas')).toBeVisible();

    const url = await doctor
      .locator('p')
      .filter({ hasText: '/r/' })
      .first()
      .textContent();

    expect(url).toContain('/r/');

    // ── a STRANGER reads it: fresh context, no session ────────────
    const strangerContext = await browser.newContext();
    const stranger = await strangerContext.newPage();
    await stranger.goto(url!.trim());

    await expect(stranger.getByText('Referral note')).toBeVisible({ timeout: 60_000 });
    await expect(stranger.getByText(`Ref E2E${suffix}`)).toBeVisible();
    await expect(stranger.getByText('General Hospital, Enugu')).toBeVisible();
    await expect(stranger.getByText('Needs surgical review')).toBeVisible();
    await expect(stranger.getByText('Acute abdomen')).toBeVisible();

    // ── a tampered code is refused, with one opaque message ───────
    const tampered = url!.trim().slice(0, -6) + 'AAAAAA';
    await stranger.goto(tampered);
    await expect(stranger.getByText(/This referral code is not valid/)).toBeVisible({
      timeout: 60_000,
    });

    await context.close();
    await strangerContext.close();
  });
});
