import { expect, test } from '@playwright/test';
import { forceSync, signIn, waitForBootSync } from './helpers';

/**
 * Sprint 6's proof: money moves between two devices, and a REPLAYED payment
 * does not charge the patient twice.
 *
 * The replay is not simulated at the HTTP level — that was proven separately.
 * Here the CLIENT's own outbox entry is reset and pushed again, which is exactly
 * what happens when a push succeeds and its acknowledgement is lost. The
 * operation_id must make the second attempt a no-op on both sides.
 */
test.describe.configure({ timeout: 480_000 });

const API = 'http://127.0.0.1:8080/api/v1';

async function token(staffNumber: string): Promise<string> {
  const response = await fetch(`${API}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      staffNumber,
      password: 'ChangeMe!2026',
      deviceId: crypto.randomUUID(),
      deviceName: 'e2e-setup',
    }),
  });
  const body = await response.json();
  return body.data.accessToken as string;
}

test.describe('billing', () => {
  test('a payment reaches the other device, and a replay does not double-charge', async ({ browser }) => {
    // ── setup: a patient with one billed invoice, through the API ──
    const setupToken = await token('DEMO/STF/0002');
    const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${setupToken}` };

    const patientRes = await fetch(`${API}/patients`, {
      method: 'POST',
      headers: auth,
      body: JSON.stringify({
        firstName: 'Billed',
        surname: `E2E${Date.now().toString().slice(-5)}`,
        gender: 'female',
        dateOfBirth: '1991-04-04',
      }),
    });
    const patient = (await patientRes.json()).data.patient as { id: string; patientNumber: string };

    const invoiceRes = await fetch(`${API}/invoices`, {
      method: 'POST',
      headers: auth,
      body: JSON.stringify({
        patientId: patient.id,
        invoiceType: 'lab',
        charges: [
          {
            chargeType: 'lab',
            description: 'Malaria parasite test',
            quantity: 1,
            unitPriceMinor: 150000,
          },
        ],
      }),
    });
    const invoice = (await invoiceRes.json()).data.invoice as {
      id: string;
      invoiceNumber: string;
      totalMinor: number;
    };

    expect(invoice.totalMinor).toBe(150000);

    // ── two workstations ───────────────────────────────────────────
    const contextA = await browser.newContext();
    const contextB = await browser.newContext();

    const cashier = await signIn(contextA, 'DEMO/STF/0007');
    const accountant = await signIn(contextB, 'DEMO/STF/0008');

    await waitForBootSync(cashier);
    await waitForBootSync(accountant);

    // ── A: pay the invoice in full ─────────────────────────────────
    await forceSync(cashier);
    await cashier.goto('/billing');

    // Identify the row by the invoice number AND the action button it must
    // contain. Filtering on text alone picks the innermost <div> holding the
    // number, which is a child of the row and has no button.
    await expect(cashier.getByText(invoice.invoiceNumber)).toBeVisible({ timeout: 120_000 });

    const row = cashier
      .locator('div')
      .filter({ has: cashier.getByRole('button', { name: 'Take payment', exact: true }) })
      .filter({ hasText: invoice.invoiceNumber })
      .last();

    // 'Pay' must be EXACT: by substring it also matches 'Take payment', which
    // every other row still shows.

    // Two elements contain the amount ("₦1,500.00" and "of ₦1,500.00"), so match
    // the exact balance line rather than a substring.
    await expect(row.getByText('₦1,500.00', { exact: true })).toBeVisible({ timeout: 60_000 });

    // The shared dev database holds payments from earlier runs, so scope every
    // assertion to what THIS test created.
    const startedAt = new Date().toISOString();

    await row.getByRole('button', { name: 'Take payment', exact: true }).click();
    await row.locator('input[inputmode=decimal]').fill('1500');
    await row.getByRole('button', { name: 'Pay', exact: true }).click();

    // The invoice leaves the outstanding list: balance is zero.
    await expect(cashier.getByText(invoice.invoiceNumber)).toHaveCount(0, { timeout: 60_000 });

    const receipt = (await cashier.evaluate(async (since) => {
      const { db } = (await import('/src/db/index.ts')) as any;
      const rows = await db.payments.toArray();
      return rows
        .filter((r: any) => r.paidAt >= since)
        .map((r: any) => r.receiptNumber) as string[];
    }, startedAt)) as string[];

    expect(receipt).toHaveLength(1);
    const receiptNumber = receipt[0];
    expect(receiptNumber).toMatch(/RCP/);

    await forceSync(cashier);

    // ── B: the accountant sees the same single payment ────────────
    await forceSync(accountant);
    await accountant.goto('/billing');
    await expect(accountant.getByText(receiptNumber)).toBeVisible({ timeout: 120_000 });
    await expect(accountant.getByText(invoice.invoiceNumber)).toHaveCount(0);

    // ── A: REPLAY the payment by resetting its outbox entry ───────
    //
    // This is the lost-acknowledgement case: the push did its work, the reply
    // never arrived, and the batch is retried with the SAME operation_id.
    await cashier.evaluate(async () => {
      const { db } = (await import('/src/db/index.ts')) as any;
      const entries = await db.outbox.where('entityTable').equals('payments').toArray();
      for (const entry of entries) {
        await db.outbox.put({ ...entry, status: 'pending', attempts: 0, nextAttemptAt: 0 });
      }
    });

    await forceSync(cashier);

    // Still exactly one payment locally, and the invoice is still settled.
    const afterReplay = (await cashier.evaluate(async (since) => {
      const { db } = (await import('/src/db/index.ts')) as any;
      const payments = (await db.payments.toArray()).filter((p: any) => p.paidAt >= since);
      const invoices = await db.invoices.toArray();
      const allocations = await db.payment_allocations.toArray();
      return {
        payments: payments.length,
        receipts: payments.map((p: any) => p.receiptNumber),
        statuses: invoices.map((i: any) => i.status),
        allocations: allocations.length,
      };
    }, startedAt)) as { payments: number; receipts: string[]; statuses: string[]; allocations: number };

    expect(afterReplay.payments).toBe(1);
    expect(afterReplay.receipts).toEqual([receiptNumber]);
    expect(afterReplay.statuses).toContain('paid');

    // ── B: still one receipt after the replay ────────────────────
    await forceSync(accountant);
    await accountant.goto('/billing');
    await expect(accountant.getByText(receiptNumber)).toHaveCount(1);

    await contextA.close();
    await contextB.close();
  });
});
