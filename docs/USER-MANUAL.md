# User manual

For PHC staff. One page per role. You do not need to know how the system works —
only what to press and what to do when the connection drops.

**Signing in:** your staff number (for example `DEMO/STF/0002`) and your password.
If the screen says *"This device has been revoked"*, an administrator must
re-enable the device — clearing your browser data is the only other way out.

---

## Everyone: the one thing to remember

**The system works without the internet.** Everything you type is saved on this
computer first and sent to the server when there is a connection.

The badge at the top right tells you where you stand:

| Badge | Meaning | What to do |
|---|---|---|
| **All records synced** | Everything is on the server | Nothing |
| **3 waiting to sync** | Saved here, not yet sent | Nothing — it sends itself |
| **Offline** | No connection; work continues | Carry on working |
| **Syncing…** | Sending now | Wait a few seconds |
| **Offline — writes blocked** | Too long offline (grace period passed) | Sign in again when you have signal |

Never close the browser to "make it save". It has already saved.

---

## Front desk

You see: **Patients**, **Households**, **Billing**, **Queue**.

**What you do most**

1. **Register a patient** — Patients → *Register patient*. First name, surname
   and sex are required; the date of birth can be replaced by an age if the
   patient does not know it (press *No birth date? Enter an age instead*).
   The patient number appears automatically — it is created on this computer,
   so registration works with no signal.
2. **Register a family** — Patients → *Register family*. Add each member, choose
   the head of household, submit. All members are saved together; you cannot end
   up with half a family.
3. **Start a visit** — open the patient → *Start visit*. This puts them in the
   queue. A patient can only have one open visit.

If you see a duplicate warning, read it and continue if it is genuinely a
different person. Two patients with the same name is normal; the system warns,
it does not block.

**Take payments** if you cover the cashier's desk — see *Cashier* below.

---

## Nurse

You see: **Queue**, **Patients**, **Ward**, **Laboratory** (view), **Pharmacy** (view).

**What you do most**

1. **Record vitals** — Queue → find the patient in *Waiting for vitals* →
   *Record vitals* → type the numbers → *Save vitals*. Press **Enter** to save;
   you do not need the mouse. The patient moves to *Waiting for doctor*.
2. **Send to doctor** — on the card, *Send to doctor*.
3. **Record vitals again** — later readings are added as new rows. Nothing is
   overwritten; the doctor sees the trend.

**Important:** a second reading never replaces the first. If you make a mistake,
record a corrected reading rather than trying to edit — editing is not possible,
by design.

---

## Doctor

You see: **Queue**, **Patients**, **Ward**, **Laboratory**, **Pharmacy**.

**What you do most**

1. **Open a consultation** — Queue → *Open consultation* on a patient in
   *Waiting for doctor*. Type into the consultation screen directly.
2. **Record a diagnosis** — type two or more letters of the condition; the list
   offers matching ICD-10 codes. Press **Enter** to take the first, or click one.
   Then *Record diagnosis*. Add notes if useful.
3. **Order a test** — choose the test, set priority, *Order test*. The bill is
   raised at the same time; the cashier will see it immediately.
4. **Prescribe** — at the bottom of the consultation screen, add each drug with
   its dose, frequency, days and quantity, then *Prescribe*. The bill is raised at
   the same moment and the cashier sees it immediately. The patient takes the
   prescription to the cashier, then to the pharmacy.
5. **Refer a patient** — *Refer patient* at the top of the consultation. Give the
   receiving facility, the reason, and how urgent it is. Print the note or show
   the QR code on screen. It is valid for 48 hours.

---

## Laboratory

You see: **Laboratory**.

**What you do most**

1. **Collect a specimen** — the left column lists tests to collect. Press
   *Collect specimen*. The card moves to *Awaiting result*.
2. **Enter a result** — *Enter result*, type the value and unit, *Save result*.
   The first result closes the order; you can add further parameters to the same
   order (a full blood count has several).

**You cannot record a result for a specimen that has not been collected.** That
is deliberate: a result against a specimen nobody took is a fiction.

---

## Pharmacist

You see: **Pharmacy**.

**What you do most**

1. **Dispense** — prescriptions that have been paid appear under *Ready to
   dispense*. Press **Dispense**. That is the whole job.

**If a prescription says "Waiting for the cashier"**, the patient has not paid.
Send them to the cashier; do not hand over the drugs. The system will refuse, and
it is right to.

---

## Cashier

You see: **Billing**, **Patients**.

**What you do most**

1. **Take a payment** — Billing lists unpaid invoices with what is still owed.
   Press *Take payment*, type the amount (or leave it blank for the full
   balance), choose cash/transfer/POS, press **Pay**. A receipt number is issued.
2. **Give a receipt** — recent receipts are listed below the invoices.

**You cannot take more than is owed** — the amount is refused and the balance is
shown. That is not an error; it protects the patient's money.

**If the same payment seems to appear twice**, it will not have been charged
twice. The system recognises a repeated payment and keeps the original receipt.

---

## Accountant

You see: **Billing**, **Patients**.

You can see every invoice and its balance, and every payment taken. You cannot
take a payment — that is the cashier's role, so that the person who records money
can be checked by someone who did not.

---

## Administrator

You see: **Staff accounts**, **Sync status**, and everything else.

**What you do most**

1. **Add a member of staff** — Staff accounts. Give them a role; without one they
   cannot sign in.
2. **Reset a password** — Staff accounts → the person → set a new password. Tell
   them to change it at first sign-in.
3. **Revoke a lost device** — Staff accounts shows devices. Revoking one stops it
   syncing and stops it signing in. **Note:** a revoked device cannot be recovered
   from its own browser; the person must clear the site data or use a new
   browser profile.

**Roles matter.** The system decides what a person may do from their role, not
from which screen they found. Giving someone `admin` gives them everything.

---

## When something goes wrong

| What you see | What it means | What to do |
|---|---|---|
| *"This patient already has an open visit"* | A visit is still open from earlier | Close it, or continue the existing one |
| *"That is more than the outstanding balance"* | The amount is higher than what is owed | Check the balance shown on the card |
| *"This prescription has not been paid for"* | The bill is unsettled | Send the patient to the cashier |
| *"Another station moved this patient"* | A colleague acted first | Reload; the screen shows the current state |
| *"The server took too long to respond"* | The connection is poor | Keep working; it will sync. The badge tells you the truth |
| *"This device has been revoked"* | An administrator disabled it | Contact the administrator |

**Nothing you type is lost when the connection drops.** If you are ever unsure,
look at the badge: *waiting to sync* means saved.
