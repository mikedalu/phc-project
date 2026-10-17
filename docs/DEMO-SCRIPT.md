# Demo script — the 12-minute defence walkthrough

Written against the system as built (§14 of `07-API-CONTRACT.md` lists every live
route). Every step below was executed during development; the timings are
measured, not estimated.

---

## Before you start (5 minutes, once)

```bash
make api        # terminal 1 — API on :8080
cd web && pnpm dev   # terminal 2 — app on :5173
```

Open **http://localhost:5173** in two windows side by side. Window B gets a
different browser profile (or an incognito window) — **two profiles are two
devices** as far as the server is concerned, with separate keys and separate
replicas.

Sign in:

| Window | Staff number | Role |
|---|---|---|
| A | `DEMO/STF/0002` | front desk |
| B | `DEMO/STF/0003` | nurse |

Password for every demo account: `ChangeMe!2026`.

**Fallbacks are at the end.** Read them once before you present.

---

## 1. The claim, in one sentence (0:00 – 0:30)

> "This is a patient record system for a primary health centre. The browser is
> the database; the server is a replication peer. Everything works with the
> network off, and a referral can be read at another hospital that has never
> seen this system."

Then do not explain further. Demonstrate.

---

## 2. Register a patient with the network OFF (0:30 – 2:30)

In **window A**:

1. DevTools → Network tab → **Offline**.
2. Patients → *Register patient*.
3. First name `Chinwe`, surname `Okeke`, sex female, date of birth `1992-11-03`.
4. *Register patient*.

What to point at:

- The patient number is `DEMO/PAT/2026/xxx-0001` — **generated on the device**.
  It contains the facility code, the year, this device's short code and its own
  sequence. Nothing asked the server.
- The badge says **Saved on this device**.
- The age was derived from the date of birth, on the device.

Repeat once with the date-of-birth field replaced by "enter an age instead" —
the same record is produced from the other direction.

> "A clerk in a PHC with no signal can do the part of their job that matters."

---

## 3. Work the queue offline (2:30 – 4:30)

Still offline, in window A:

1. Open the patient → **Start visit**.
2. Queue → the patient is in *Waiting for vitals*.
3. **Record vitals**: 118/76, 36.9 °C, 84 bpm, 18, 98 %, 165 cm, 61 kg → *Save*.
4. The card moves to *Waiting for doctor* → **Send to doctor**.
5. **Open consultation** — it opens the consultation screen.
6. Type `malar` in the diagnosis field: the ICD-10 shortlist offers
   **B54 Unspecified malaria**. Press Enter. *Record diagnosis*.
7. *Order test* (Malaria Parasite). Note the bill is raised automatically.

What to point at:

- Every button offered came from the server's `nextStages` for that encounter.
  The UI cannot offer an illegal move.
- The vitals form is inside the card — no modal, because a nurse does this forty
  times a day.
- Going back online is not required for any of it.

---

## 4. Reconnect, and prove it was not local-only (4:30 – 6:00)

1. DevTools → **Online**.
2. Watch the badge: *1 waiting to sync* → **All records synced**.
3. Open a SQL client (or the API) and show the row exists in Postgres:
   ```sql
   SELECT patient_number, first_name, surname, age_years
     FROM patients WHERE first_name = 'Chinwe' ORDER BY created_at DESC LIMIT 1;
   ```
4. In **window B** (nurse), press *Sync now* on **Sync status**, then open the
   Queue — the patient is there, in consultation, with the diagnosis.

> "Two browsers, two device keys, two replicas. The record arrived without
> anyone pressing Save twice."

---

## 5. Money, and the double-charge that cannot happen (6:00 – 8:00)

Window A, sign out, sign in as `DEMO/STF/0007` (**cashier**).

1. **Billing**: the consultation's test appears with its balance.
2. *Take payment*, amount shown, **Pay**. A receipt number is issued.
3. Point at **Recent receipts**.

Now the interesting part:

4. Open DevTools → Application → IndexedDB → `phc_records` → `outbox`.
   Find the payment row. Change `status` from `done` to `pending` — this
   simulates a push whose acknowledgement was lost, the retry that would
   double-charge a patient in a naive system.
5. Sync status → *Sync now*.

What to show: still **one** payment, **one** receipt, the invoice still settled.
The `operation_id` is the idempotency key, and the database has a UNIQUE
constraint on it. The replay returns `duplicate: true` with the original receipt.

> "This is the one place a retry would cost real money, so it is the one place
> the constraint does the work rather than my care."

---

## 6. The referral — the second headline (8:00 – 10:30)

In window A, sign in as `DEMO/STF/0004` (**doctor**).

1. Queue → open a consultation → **Refer patient**.
2. Refer to `General Hospital, Enugu`, reason `Needs surgical review`, urgency
   *urgent* → **Issue referral**.
3. A QR code appears, drawn on this device.

Now the demonstration that matters:

4. Copy the URL printed under the QR code.
5. Open a **brand-new incognito window** — no session, no local database, a
   machine that has never seen this system — and paste the URL.

The note renders: patient name, age, sex, both facilities, reason, diagnosis.

6. Edit one character of the token in the address bar and reload.

`This referral code is not valid.` — one opaque message, because *"bad
signature"* tells an attacker they were close.

> "AES-256-GCM: unreadable without the facility key, and tamper-evident. The
> token contains no patient ID, no phone number and no address — the receiving
> clinician gets what they need to treat the person in front of them, and nothing
> that would let them look that person up later."

---

## 7. What is deliberately not here (10:30 – 12:00)

Say this plainly; it is stronger than pretending.

- **Whole-row LWW, not per-field.** Two clinicians editing different fields of
  one patient lose one edit — and the loser is stored in `sync_conflicts` with
  BOTH payloads, not dropped. Recorded as decision D64 with the trade-off.
- **No reports, no dashboards, no audit viewer.** Sprint 9's reporting scope was
  traded for hardening: token revocation and change-log retention, both of which
  a running clinic needs more.
- **A referral cannot be reprinted.** The token is returned once and never
  stored, because storing it would make the encryption pointless. That is a
  design consequence, not a missing feature.
- **Single facility.** No tenant column anywhere; the deployment *is* the
  facility.

---

## Fallbacks

**Neon is slow or cold.** Every screen reads Dexie, so the demo runs offline
anyway. If the connection is bad, put DevTools offline and do sections 2–3 — then
show a screenshot of the reconcile for section 4.

**The printer is dead.** Section 6 needs no printer: the QR is on screen and the
URL can be pasted. The print button is a convenience.

**A login fails with 403.** Check the device is not revoked (admin → devices) and
that you are on the origin in `ALLOWED_ORIGINS` — `localhost:5173` and
`127.0.0.1:5173` are different origins to a browser, and the allow-list names
both. This cost a session to learn.

**Nothing syncs after a restore.** A device whose cursor predates the retained
change log is told to re-bootstrap; that path is exercised by setting
`pullCursor` to a low value and pressing Sync. The app refills itself from
snapshots.
