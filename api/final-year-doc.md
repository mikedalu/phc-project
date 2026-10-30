# Software Requirements Specification

## Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres

**Document type:** Software Requirements Specification (SRS)  
**Project:** Final Year Project  
**Version:** 1.1  
**Date:** October 2026  
**Status:** MVP implemented and verified; post-MVP items are explicitly identified.

<!-- pagebreak -->

## Document Control

| Item | Value |
|---|---|
| Document title | Software Requirements Specification |
| Project topic | Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres |
| Version | 1.1 |
| Status | Final-year project submission; MVP verified |
| Prepared for | Final year project assessment and project stakeholders |
| Classification | Internal / Academic |

### Revision History

| Version | Date | Description |
|---|---|---|
| 0.1 | Sprint 0 | Initial scope and architecture notes |
| 0.5 | Sprint 5 | Clinical workflow requirements added |
| 1.0 | October 2026 | Consolidated SRS after MVP audit, ward billing, password change and referral verification |
| 1.1 | October 2026 | Restructured to the final-year SRS outline: background, purpose, scope and objectives; functional and non-functional requirements; external interfaces; appendices and glossary |

### Reading Guide

This document describes the intended purpose, functionality, requirements, interfaces and constraints of the system. Requirements are written so that they can be tested. Implementation status is shown against each requirement so that the document does not confuse what the MVP delivers with what is planned for future work.

<!-- pagebreak -->

## Table of Contents

1. Introduction
   1.1 Background of the Project
   1.2 Purpose of the Proposed System
   1.3 Scope of the Project
   1.4 Objectives of the System
2. Overall Description
   2.1 Product Perspective
   2.2 Product Functions
   2.3 User Characteristics
   2.4 Constraints
   2.5 Assumptions and Dependencies
3. Functional and Non-Functional Requirements
   3.1 Functional Requirements
   3.2 Non-Functional Requirements
4. External Interface Requirements and Dependencies
   4.1 User Interface Requirements
   4.2 Hardware Requirements
   4.3 Software Requirements
   4.4 Database Requirements
   4.5 External Systems, APIs and Internet Connectivity
   4.6 External Dependencies
   4.7 Deployment and Operating Environment
   4.8 Degraded-Mode Behaviour
5. Appendices and Glossary
   5.1 Appendix A: References
   5.2 Appendix B: Implementation Status and Verification
   5.3 Appendix C: Referral QR System — Built and Deferred
   5.4 Appendix D: Requirement Traceability Summary
   5.5 Appendix E: Assumptions, Limitations and Future Work
   5.6 Glossary

<!-- pagebreak -->

# 1. Introduction

## 1.1 Background of the Project

A primary healthcare centre (PHC) is the first point of contact for most patients in the communities it serves. Its staff must register patients, record vital signs, triage, consult, order laboratory tests, prescribe, dispense medicines, collect payments and refer patients to higher-level facilities. These activities are continuous and time-sensitive, but the infrastructure available to many PHCs is not continuous: mobile data and fixed broadband can disappear for hours, power can fail, and workstations are often shared and modest.

Paper records remain common because they are available when the network is not. Their weaknesses are equally well known. Paper is difficult to search, easy to misplace, hard to read, expensive to duplicate and unable to warn two staff members that they are editing the same record. A purely cloud-based electronic record solves searchability and continuity of data, but it fails at the point of care when the link is down. A clinic cannot tell a patient to return tomorrow because the server is unreachable.

The project therefore starts from a different premise: **the browser is the primary database during a shift, and the server is a replication peer that is also the authority for identity, security and referral links.** A workstation keeps a local clinical database, writes to it immediately, queues the write for later transmission, and continues to work through an outage. When connectivity returns, the queued changes are pushed to a central PostgreSQL database and changes from other workstations are pulled back. Conflicts are preserved rather than silently discarded.

Referral is the second half of the problem. A handwritten referral letter can be lost, altered, illegible or presented at a facility that cannot verify who wrote it. The project implements a secure, QR-code referral note: the issuing facility seals the referral into an encrypted token, prints it as a scannable code, and the receiving clinician opens the note through a public web page without needing an account. The token is time-limited and tamper-evident, and it deliberately carries only the clinical information needed to continue care.

The project is therefore both a clinical records system and a referral-management system. Its context is a single PHC deployment with several shared workstations, eight staff roles, intermittent connectivity and a thermal printer at the front desk.

## 1.2 Purpose of the Proposed System

This Software Requirements Specification (SRS) defines the intended purpose, functions, users, constraints, interfaces and requirements of the **Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres**. It is written so that the project supervisor, examining panel, developers, testers and clinical staff can share one understanding of what the system must do and what the MVP actually delivers.

The proposed system exists to give a PHC a dependable electronic clinical record that does not stop working when the internet stops working. It replaces fragmented paper registers and ad-hoc spreadsheets with a single local-first application that supports the complete patient journey, preserves an audit trail, enforces role boundaries and produces a secure referral note that another facility can verify.

The document also serves as a communication contract. Functional requirements are numbered so that testers can trace a test to a requirement, and implementation status is shown so that the difference between "required for the MVP" and "planned for a future release" is explicit.

**Document overview:** Section 2 describes the product context, functions, users and constraints. Section 3 states functional and non-functional requirements with identifiers, priority and MVP status. Section 4 describes user, hardware, software, database and communication interfaces. Section 5 contains references, implementation evidence, the referral QR status, traceability, limitations and the glossary.

## 1.3 Scope of the Project

### 1.3.1 Project Topic

**Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres.**

The topic is interpreted as a single-facility clinical information system that continues to function when the network is unavailable, then converges with a central database when connectivity returns. The system also manages referrals so that a patient can be sent to another facility with a secure, scannable QR note rather than a handwritten letter that can be lost or altered.

### 1.3.2 In Scope

The MVP covers the following clinical and administrative workflow:

- Individual and family patient registration, including hospital numbers, household links and duplicate warnings.
- Encounter creation, queue management, triage, vitals capture and stage transitions.
- Doctor consultation, diagnosis with ICD-10 support, laboratory ordering, prescriptions and referrals.
- Laboratory specimen collection and result entry.
- Pharmacy dispensing of paid prescriptions.
- Cashier billing, invoices, charge lines, payment capture, payment allocation and derived balances.
- Inpatient admission, manual bed-day charging, ward board and discharge with a balance gate.
- Secure QR-code referral issue, public read, 48-hour expiry and tamper detection.
- Eight staff roles with role-based navigation and server-side authorisation.
- Device registration, device revocation, offline sign-in with a grace period, session refresh and password change.
- Offline-first local storage, background synchronisation, idempotent push, pull cursors, conflict preservation and audit logging.
- Operational dashboard, command palette, audit activity view and staff/device administration screens.
- Responsive, keyboard-first, light-mode clinical interface suitable for low-power workstations.

### 1.3.3 Explicitly Out of Scope

The following items are outside the MVP and must not be inferred from the project topic:

- Multi-facility tenancy in one deployment. One deployment serves one health centre; a second facility deploys its own copy.
- Pharmacy inventory, stock levels, purchase orders and supplier management.
- Human resources, payroll, rota and staff scheduling.
- Insurance claims, NHIS integration and third-party payer adjudication.
- SMS, email or WhatsApp reminders to patients.
- Native Android or iOS applications.
- HL7, FHIR or other external health-information-exchange standards.
- Full accounting ledger, tax reporting and statutory financial statements.
- Automatic offline issuance of referral tokens. Referral creation is online-only in the MVP because the token is encrypted and issued by the server.

### 1.3.4 Current Implementation Boundary

The project is best described as an **MVP with a verified clinical demo path and a documented post-MVP roadmap**. The complete core workflow and the QR referral read path are implemented. Referral PIN verification, referral access logging, a referral register, revocation UI, formal reports and exports, ward/bed master data, nursing notes, transfers and an automatic nightly accrual runner are post-MVP items. Appendix B and Appendix C record this honestly.

## 1.4 Objectives of the System

The system is designed to achieve the following objectives:

1. **Continue the clinical workflow without connectivity.** Registration, triage, consultation, laboratory ordering, payment, dispensing and ward work must remain usable when the server is unreachable, with writes committed locally before any network call.
2. **Provide one coherent patient record.** A patient's demographics, visits, vitals, diagnoses, laboratory results, prescriptions, payments and admissions must be available from one record rather than scattered across paper books and spreadsheets.
3. **Make referral safe and portable.** Issue a referral as an encrypted, tamper-evident QR note that a receiving clinician can open without an account, with a fixed expiry and a payload limited to information needed for care.
4. **Protect data integrity during synchronisation.** Preserve local writes in an outbox, make push idempotent, pull changes in order, and record conflicts rather than silently discarding a losing edit.
5. **Enforce role-based access and device trust.** Restrict clinical and financial actions by role on the server, require registered devices for sync, support offline grace and make a lost device revocable.
6. **Support a real clinic desk.** Deliver a fast, keyboard-friendly, high-contrast interface that works on low-power workstations, prints on 58–80 mm thermal paper and shows network state honestly.
7. **Leave an auditable and reproducible MVP.** Record state changes, provide role-specific operating instructions, and verify the core workflow with automated tests and a documented demonstration path.

<!-- pagebreak -->

# 2. Overall Description

## 2.1 Product Perspective

### 2.1.1 System Context

The system is a browser-based clinical records application with a local database and a central replication service. It is not a thin client. During a shift, the workstation can register a patient, record vitals, complete a consultation, order a test, take a payment, dispense a drug, admit a patient and print a referral without a network connection. The server is contacted when available, and every local write is replayed to it.

```
Clinical workstation (browser)
  React 19 + Vite + TanStack Router
  Dexie / IndexedDB  ── local clinical database + outbox + sync cursor
  Service worker      ── offline application shell
  Web Crypto          ── device keys and local credentials
          │
          │  HTTPS / JSON, signed sync batches
          ▼
Go API (single facility deployment)
  Authentication, RBAC, device trust
  Clinical and billing services
  Sync push / pull / snapshot
  Referral token sealing and public read
          │
          ▼
PostgreSQL (Neon)
  Source of truth for identity, security and audit
  Replication peer for clinical rows
  Change log, conflicts, sync operations
```

### 2.1.2 Architectural Principles

1. **Local-first writes.** A clinical write is committed to IndexedDB and to the outbox in the same transaction. The user is not blocked by the network.
2. **The server is the authority for identity and security.** Staff accounts, roles, device trust, password changes, referral tokens and access decisions are enforced on the server.
3. **Append-only facts.** Vitals, results, payments, dispenses and ward nights are facts. They are inserted, not silently rewritten. Corrections are new rows or controlled reversals.
4. **Mutable headers use whole-row last-write-wins.** Patient and encounter header edits converge, and the losing payload is preserved in `sync_conflicts` rather than discarded.
5. **No hard deletes.** Rows are soft-deleted with a tombstone so deletion replicates like any other change.
6. **One facility per deployment.** The facility configuration row supplies the name, address, document prefix, currency and timezone. No tenant column exists.
7. **Security is not a client preference.** Hiding a navigation item is usability; the API still rejects an unauthorised action.

### 2.1.3 Core Information Objects

| Object | Purpose |
|---|---|
| Patient | Demographic and identifying record, hospital number and household link |
| Household | Family grouping used by family registration and household billing views |
| Encounter | One visit, its workflow stage, priority and closure |
| Vitals | Append-only observations: blood pressure, temperature, pulse, respiratory rate, height, weight, SpO2, BMI and pain score |
| Diagnosis | Consultation diagnosis with ICD-10 code and notes |
| Lab order | Requested test, specimen state and result state |
| Lab result | Append-only result values for a collected specimen |
| Prescription | Drug order header linked to an invoice |
| Prescription item | Drug, dose, frequency, duration, quantity and price |
| Dispense | Pharmacy fact recording what was handed over |
| Invoice | Bill header for drugs, laboratory work or ward care |
| Charge | One billable line with a snapshotted price |
| Payment | Money received, with an idempotency key |
| Payment allocation | Which charge a payment settled |
| Admission | Inpatient stay, ward, bed label and discharge state |
| Ward charge | One billed night, unique per admission and date |
| Referral | Issued referral record and its expiry state |
| User and role | Staff identity and the roles that authorise actions |
| Device | A registered workstation with a public key and revocation state |
| Sync change log | Ordered record of applied server changes for pull |
| Sync conflict | Preserved losing payload for review |

### 2.1.4 Product Position

The product is not a replacement for a national health information exchange. It is a **facility-level operating system for a small clinical team**: it prioritises availability, clarity at the point of care, and the security of a referral that leaves the building.

## 2.2 Product Functions

The product functions are grouped into twelve functional areas.

### 2.2.1 Identity, Session and Device Trust

- Staff sign in with staff number and password.
- The server issues short-lived access tokens and rotating refresh tokens.
- A workstation is registered with a device key; sync batches are signed.
- Offline sign-in is possible while the device is within its configured grace window.
- Revoked devices and disabled accounts are refused at the next server check.
- Staff can change their own password; other sessions are revoked while the current workstation remains signed in.
- Temporary passwords are flagged in the interface so staff can replace them.

### 2.2.2 Staff Administration and RBAC

- Administrators list staff and their roles.
- Administrators create staff accounts with a temporary password.
- Administrators change a role, deactivate an account and revoke a device.
- Eight roles are recognised: front desk, nurse, doctor, laboratory scientist, pharmacist, cashier, accountant and administrator.
- Navigation is role-aware; server-side checks remain authoritative.

### 2.2.3 Patient Registration and Households

- Register an individual with demographics, contact details, age or date of birth, and clinical flags such as allergies and chronic conditions.
- Register a family and link members to a household.
- Generate a facility-prefixed hospital number.
- Warn about a possible duplicate without blocking legitimate same-name patients.
- Search by name, hospital number, phone, or patient identifier.
- Show a patient profile with visits, clinical history, household and balance context.

### 2.2.4 Encounters, Queue, Vitals and Triage

- Start a visit from a patient record.
- Enforce one open visit per patient.
- Move a visit through controlled workflow stages: registered, vitals done, awaiting doctor, in consultation, awaiting laboratory, awaiting drug payment, awaiting pharmacy, admitted and closed.
- Record vitals as append-only observations and calculate BMI.
- Move a patient to the doctor queue after vitals.
- Display a live queue board by stage and priority.
- Record a second reading without overwriting the first.

### 2.2.5 Consultation, Diagnosis, Orders and Prescriptions

- Open a consultation from the queue.
- Record diagnoses with ICD-10 code suggestions and notes.
- Order laboratory tests with priority and clinical indication.
- Prescribe multiple drugs with dose, frequency, duration, quantity and unit price.
- Raise the relevant invoice and charge at the point of ordering.
- Show the clinical timeline and prior results on the consultation screen.
- Issue a referral from the consultation.

### 2.2.6 Laboratory

- List specimens to collect and orders awaiting results.
- Mark a specimen collected.
- Enter one or more result parameters for an order.
- Refuse a result for a specimen that was never collected.
- Keep results append-only and visible to the ordering clinician.

### 2.2.7 Pharmacy

- List prescriptions that are paid and ready to dispense.
- Refuse dispensing when payment is still outstanding.
- Record a dispense fact and show the prescription as dispensed.
- Preserve the medication and quantity that were handed over.

### 2.2.8 Billing and Cashier

- Create invoices for drugs, laboratory work and ward care.
- Add charge lines with snapshotted prices.
- Record payments by cash, transfer or POS card.
- Allocate a payment using automatic oldest-first allocation or explicit lines.
- Derive balances from charges minus allocations; never store a separate balance column.
- Make payment idempotent so a retry cannot double-charge.
- Show unpaid and partially paid invoices to the cashier and accountant.

### 2.2.9 Inpatient Admission and Ward

- Admit a patient from a consultation with a ward and optional bed label.
- Move the encounter to the admitted stage.
- Charge one bed-day per admission per date.
- Create a ward invoice and charge line locally and on the server.
- Display admitted patients, billed nights and charges on the ward board.
- Block discharge while the ward balance is outstanding, in both the client and the API.
- Close the encounter when discharge succeeds.

### 2.2.10 Referral Management and Public QR Access

- Issue a referral for a patient and encounter with receiving facility, reason, diagnosis summary, treatment given and urgency.
- Generate a cryptographically protected token and a QR code on the issuing device.
- Return the token once; do not store the token in the database.
- Allow a receiving clinician with no account to open the public referral page and read the note.
- Enforce a 48-hour expiry on the server.
- Reject forged, malformed and expired tokens without revealing which check failed.
- Keep the payload minimal: no patient identifier, phone number or address.
- **Post-MVP:** PIN challenge, access log, referral register, revocation UI and reprint.

### 2.2.11 Offline Synchronisation and Conflict Handling

- Queue every local domain write in an outbox in the same transaction.
- Push batches with operation identifiers for idempotency.
- Sign sync batches with the registered device key.
- Pull changes using a monotonic server cursor.
- Bootstrap a new device from table snapshots before adopting a cursor.
- Apply mutable rows with last-write-wins and preserve the losing payload.
- Apply append-only facts without merging them.
- Retry with backoff on network failure.
- Show pending, syncing, offline and error states without blocking clinical work.

### 2.2.12 Dashboard, Audit and Administration

- Show a role-aware shift dashboard with live local counts.
- Provide a global command palette for patient search, department navigation and emergency actions.
- Provide a facility-wide activity and audit view for administrators.
- Provide a sync status view for support and demonstration.
- Provide a change-password page and a temporary-password prompt.
- Accept a referral note on a public route without an account.

## 2.3 User Characteristics

### 2.3.1 Staff Roles

| Role | Primary work | Typical technical skill | Key system expectations |
|---|---|---|---|
| Front desk | Registration, visit start, queue entry, payment cover | Basic computer and browser use | Fast keyboard entry, duplicate warnings, clear patient search |
| Nurse | Vitals, triage, ward work, referral support | Basic clinical application use | Dense queue, keyboard-friendly vitals, append-only readings |
| Doctor | Consultation, diagnosis, orders, prescriptions, admission, referral | Moderate clinical application use | Timeline at a glance, one-screen consultation, safe order entry |
| Laboratory scientist | Specimen collection and result entry | Basic laboratory application use | Clear worklists, no result without specimen, result history |
| Pharmacist | Dispensing paid prescriptions | Basic dispensing application use | Clear paid/unpaid distinction, no dispensing without payment |
| Cashier | Invoices, payments and receipts | Basic cashier application use | Fast payment entry, automatic allocation, idempotent retry |
| Accountant | Billing oversight and read-only financial views | Moderate reporting use | Read-only oversight; cannot take payments |
| Administrator | Staff, roles, devices, audit and configuration | Moderate administration use | Account creation, role changes, device revocation, audit trail |

### 2.3.2 Non-Staff Users

| User | Relationship to the system |
|---|---|
| Patient or caregiver | Data subject; may see a printed referral slip or receipt. Does not operate the system. |
| Receiving clinician at another facility | Opens the public QR referral page without an account. Must be able to read the note on a phone. |
| Support engineer or developer | Uses the sync status view, audit view and API logs to diagnose a device. |
| Examiner or assessor | Uses the demo script and this document to evaluate scope, evidence and limitations. |

### 2.3.3 User Assumptions

- Staff can use a modern browser and a keyboard.
- Staff receive role-specific training before live use.
- A shared workstation is normal; the signed-in staff member is visible in the header.
- The facility has at least one workstation, one thermal printer and an intermittent internet link.
- A receiving facility has a smartphone camera or QR scanner and a browser.
- Staff understand that hiding a navigation item is not the same as a security boundary.

### 2.3.4 Intended Audience

| Audience | How this document is used |
|---|---|
| Project supervisor and examiners | Assess whether the implementation and documentation match the approved topic and scope |
| Clinical staff (front desk, nurses, doctors, laboratory, pharmacy, cashier, accountant) | Understand expected behaviour, role boundaries and offline operation |
| Facility administrator | Understand staff, device, audit and security responsibilities |
| Developers | Implement, maintain and extend the system without breaking offline or security invariants |
| Testers | Derive test cases from numbered functional and non-functional requirements |
| Future researchers | Identify the boundaries of the MVP and the recommended next increments |

## 2.4 Constraints

### 2.4.1 Technical Constraints

| Constraint | Effect |
|---|---|
| Browser as the primary database | Requires IndexedDB, service worker, Web Crypto and a modern evergreen browser |
| One facility per deployment | No tenant column; multi-facility use requires separate deployments |
| Referral token issued by the server | Referral issue and public read require connectivity; clinical records do not |
| Referral token is returned once and not stored | A lost referral slip cannot be reprinted from the system; a new referral must be issued |
| Whole-row LWW for mutable headers | Two clinicians editing different fields of one row can lose one edit; the loss is preserved in `sync_conflicts` |
| Thermal printer support | Layout must remain legible on 58–80 mm paper; QR must be large enough for a phone camera |
| Browser storage quota | Local data is bounded by the device's IndexedDB quota; the MVP does not implement archival export |
| Server-side encryption key | Loss of the referral encryption key makes outstanding referral links unreadable |
| Single timezone per facility | Day boundaries and bed-night dates must use the facility timezone; the server stores UTC |

### 2.4.2 Operational Constraints

- Connectivity may be absent for hours, slow, metered or unstable; the system must remain clinically usable and sync in bounded batches when a link exists.
- A device may be lost or stolen; it must be revocable.
- Staff may share a workstation; the active user must be obvious.
- The facility may have low-power workstations; the interface must avoid heavy graphics and animation.
- The receiving facility may have no account and a poor camera; the public page must be simple and mobile-friendly.
- Clinical data is sensitive; staff must not share screens or printed notes unnecessarily.

### 2.4.3 Regulatory and Ethical Constraints

- Patient confidentiality and least-privilege access are mandatory.
- Only the minimum data needed for care and referral should be collected and transmitted.
- Referral QR payloads must not contain patient identifiers that allow later lookup.
- Audit records must not be silently edited or deleted by ordinary users.
- The deployment must follow the applicable data-protection regime for the facility's jurisdiction; NDPR is the reference used in this project.
- Clinical decisions remain the responsibility of qualified staff; the system records and supports, but does not replace, clinical judgement.

### 2.4.4 Project Constraints

- **Time:** the academic calendar fixes the delivery window; the team prioritised the core clinical workflow and referral path over reporting and long-term operations.
- **Cost:** the project must run on low-cost or free-tier hosting, managed PostgreSQL and open-source libraries; it does not assume paid SMS, payment-gateway or interoperability services.
- **Technology:** the chosen stack is a modern browser, React, Go, PostgreSQL and a service worker; a native application or a specialised clinical device integration would exceed the project constraints.
- **Internet availability:** referral issue and public referral read require connectivity; the rest of the clinical workflow is designed to operate without it.
- The implementation is a final-year MVP, not a fully certified medical device.
- Reporting, formal exports, multi-facility tenancy and interoperability standards are post-MVP.
- The demo uses seeded staff accounts with a shared temporary password; a real installation must change it.
- The project is assessed on a reproducible demo path, not on every item in the long-term roadmap.

## 2.5 Assumptions and Dependencies

### 2.5.1 Assumptions

1. The facility uses one deployment and one facility configuration row.
2. A workstation is a modern browser on a desktop or laptop with local storage.
3. Staff are trained on the role-specific workflow and on offline status indicators.
4. The server and database are administered by a competent operator.
5. The facility has a process for replacing a revoked device.
6. The receiving facility can open a URL and scan a QR code.
7. The project assessment environment can run the Go API, the web application and a PostgreSQL database.

### 2.5.2 Dependencies

| Dependency | Why it matters |
|---|---|
| Neon PostgreSQL | Central database, change log, identity, audit and sync authority |
| Go API runtime | Authentication, RBAC, clinical services, sync and referral sealing |
| Modern browser | IndexedDB, service worker, Web Crypto, Canvas QR rendering |
| HTTPS | Protects credentials, tokens and clinical payloads in transit |
| Referral encryption key | Required to issue and read referral tokens |
| Device key pair | Required to sign sync batches |
| Thermal printer | Required for the printed referral and receipt path |
| Network, when available | Required for login, sync, referral issue and public referral read |
| Cloud hosting (Cloud Run and Cloudflare Pages or equivalent) | Production deployment |
| Backup and point-in-time recovery | Recovery from operator error or data corruption |

<!-- pagebreak -->

# 3. Functional and Non-Functional Requirements

## 3.1 Functional Requirements

This section states what the system must do. Each requirement has an identifier, a priority and an MVP status. The functional areas follow the patient journey from sign-in and registration through consultation, laboratory, pharmacy, billing, ward care, referral, synchronisation and administration. They cover the expected categories directly: user registration and login, data entry and storage, searching and retrieving records, updating and soft-deleting records where appropriate, and report generation. Report generation is explicitly marked as post-MVP rather than being claimed as built.

### 3.1.1 Identifiers

- Functional requirements use `FR-<AREA>-<NN>`.
- Non-functional requirements use `NFR-<AREA>-<NN>`.
- Areas include AUTH, ADM, PAT, QUE, CON, LAB, PHA, BIL, WAR, REF, SYN, AUD, UI, SEC, PERF, REL, PRIV, MNT and OPS.

### 3.1.2 Priorities

| Priority | Meaning |
|---|---|
| Must | Required for the MVP or for safe clinical operation |
| Should | Important, but a documented workaround exists |
| Could | Useful enhancement, not required for the MVP |
| Won't | Explicitly excluded from this release |

### 3.1.3 MVP Status

| Status | Meaning |
|---|---|
| Implemented | Present and verified in the MVP |
| Partial | Present with a documented limitation |
| Deferred | Designed or acknowledged, but not built in the MVP |
| Out of scope | Excluded by the project scope |
| Online-only | Implemented, but intentionally requires connectivity |

### 3.1.4 Authentication, Session and Device Trust

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-AUTH-01 | A staff member shall sign in with staff number and password and receive an access token, a rotating refresh token and a session identifier. | Must | Implemented |
| FR-AUTH-02 | The system shall display the signed-in staff member and their role on every authenticated screen. | Must | Implemented |
| FR-AUTH-03 | The system shall refresh an expired access token without forcing the user to re-enter credentials while the refresh token is valid. | Must | Implemented |
| FR-AUTH-04 | The system shall allow a previously verified staff member to sign in while offline, inside the configured offline grace window, using a locally cached credential verifier and device key. | Must | Implemented |
| FR-AUTH-05 | The system shall refuse offline sign-in after the grace window and shall explain how to restore connectivity. | Must | Implemented |
| FR-AUTH-06 | The system shall register a workstation device key and shall sign sync batches with that key. | Must | Implemented |
| FR-AUTH-07 | A revoked device shall be unable to sync or sign in after the next server check. | Must | Implemented |
| FR-AUTH-08 | A disabled staff account shall be refused at the next server check. | Must | Implemented |
| FR-AUTH-09 | A signed-in staff member shall be able to change their own password by giving the current password and a new password of at least 12 characters. | Must | Implemented |
| FR-AUTH-10 | A successful password change shall revoke every other session for that account while keeping the current workstation signed in. | Must | Implemented |
| FR-AUTH-11 | The interface shall prompt a staff member with a temporary password to change it. | Should | Partial — prompt only; it does not block the demo accounts |
| FR-AUTH-12 | An administrator shall be able to reset another staff member's password. | Should | Deferred |
| FR-AUTH-13 | The system shall log out locally and revoke the refresh token on the server when the user signs out. | Must | Implemented |
| FR-AUTH-14 | The system shall show an expired or revoked session screen instead of a broken application shell. | Must | Implemented |

### 3.1.5 Staff Administration and RBAC

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-ADM-01 | The system shall recognise the roles front desk, nurse, doctor, laboratory scientist, pharmacist, cashier, accountant and administrator. | Must | Implemented |
| FR-ADM-02 | The system shall show only navigation destinations permitted for the signed-in roles. | Must | Implemented |
| FR-ADM-03 | The API shall enforce every role restriction independently of the interface. | Must | Implemented |
| FR-ADM-04 | An administrator shall list staff, their roles and their active state. | Must | Implemented |
| FR-ADM-05 | An administrator shall create a staff account with a temporary password and one role. | Must | Implemented |
| FR-ADM-06 | An administrator shall change a staff member's role and active state with a reason. | Must | Implemented |
| FR-ADM-07 | An administrator shall view registered devices and revoke a lost or compromised device with a reason. | Must | Implemented |
| FR-ADM-08 | An administrator shall view the audit activity trail. | Must | Implemented |
| FR-ADM-09 | The system shall prevent a staff member from granting themselves a role through the client. | Must | Implemented |
| FR-ADM-10 | The system shall record staff and device administration changes in the audit log. | Must | Implemented |

### 3.1.6 Patient Registration and Households

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-PAT-01 | A front-desk user shall register a new individual patient with the required demographics and contact fields. | Must | Implemented |
| FR-PAT-02 | The system shall generate a unique facility-prefixed hospital number for each patient. | Must | Implemented |
| FR-PAT-03 | The system shall support age in years or months, or a date of birth, and shall derive the other representation consistently. | Must | Implemented |
| FR-PAT-04 | The system shall warn when a likely duplicate patient is found and shall allow the user to continue after reviewing the warning. | Must | Implemented |
| FR-PAT-05 | A user shall register a family and link members to a household. | Should | Implemented |
| FR-PAT-06 | A user shall search patients by name, hospital number, phone or identifier. | Must | Implemented |
| FR-PAT-07 | A user shall open a patient profile showing demographics, visits, clinical history and balance context. | Must | Implemented |
| FR-PAT-08 | The system shall store allergies, chronic conditions, blood group and genotype where supplied. | Should | Implemented |
| FR-PAT-09 | The system shall support soft deletion and shall not hard-delete a patient record through the application. | Must | Implemented |
| FR-PAT-10 | Patient registration shall work while offline and shall synchronise later. | Must | Implemented |
| FR-PAT-11 | The system shall record who created or last updated a patient row and when. | Must | Implemented |
| FR-PAT-12 | An authorised user shall update patient demographics and clinical flags; the update shall be revisioned and shall synchronise like any other local write. | Must | Implemented |

### 3.1.7 Encounters, Queue, Vitals and Triage

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-QUE-01 | A user shall start a visit from a patient record. | Must | Implemented |
| FR-QUE-02 | The system shall prevent more than one open visit for the same patient. | Must | Implemented |
| FR-QUE-03 | The system shall enforce a controlled workflow from registration through vitals, consultation, laboratory, pharmacy, admission and closure. | Must | Implemented |
| FR-QUE-04 | The queue board shall show live counts and patient cards by stage. | Must | Implemented |
| FR-QUE-05 | A nurse shall record blood pressure, temperature, pulse, respiratory rate, height, weight, SpO2, pain score and notes. | Must | Implemented |
| FR-QUE-06 | The system shall compute BMI from height and weight when both are present. | Should | Implemented |
| FR-QUE-07 | Vitals shall be append-only; a later reading shall not overwrite an earlier reading. | Must | Implemented |
| FR-QUE-08 | Recording vitals for a newly registered visit shall move it to the vitals-done stage. | Must | Implemented |
| FR-QUE-09 | A nurse or doctor shall be able to send a patient to the doctor queue. | Must | Implemented |
| FR-QUE-10 | The system shall display the patient's stage, priority and waiting context without requiring a page refresh. | Must | Implemented |
| FR-QUE-11 | Queue changes shall work offline and synchronise later. | Must | Implemented |
| FR-QUE-12 | The system shall show a clear conflict or refusal if the server rejects an invalid stage transition. | Should | Implemented |

### 3.1.8 Consultation, Diagnosis, Orders and Prescriptions

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-CON-01 | A doctor shall open a consultation from a patient in the doctor queue. | Must | Implemented |
| FR-CON-02 | The consultation shall show the patient header, vitals trend, prior diagnoses, prior results and current orders. | Must | Implemented |
| FR-CON-03 | A doctor shall record a diagnosis with free text and an optional ICD-10 code. | Must | Implemented |
| FR-CON-04 | The system shall suggest ICD-10 codes as the doctor types a condition. | Should | Implemented |
| FR-CON-05 | A doctor shall order one or more laboratory tests with priority and clinical indication. | Must | Implemented |
| FR-CON-06 | A doctor shall prescribe one or more drugs with dose, frequency, duration, quantity and unit price. | Must | Implemented |
| FR-CON-07 | Ordering a test or prescription shall create the corresponding invoice and charge in the same local transaction. | Must | Implemented |
| FR-CON-08 | The consultation screen shall provide a link to issue a referral for the encounter. | Must | Implemented |
| FR-CON-09 | Clinical writes shall work offline and synchronise later. | Must | Implemented |
| FR-CON-10 | The system shall not allow a result to be recorded against an uncollected specimen. | Must | Implemented |
| FR-CON-11 | The system shall show prior results and diagnoses while the doctor is consulting. | Must | Implemented |

### 3.1.9 Laboratory

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-LAB-01 | The laboratory screen shall list orders awaiting specimen collection. | Must | Implemented |
| FR-LAB-02 | A laboratory scientist shall mark a specimen collected with the collecting user and time. | Must | Implemented |
| FR-LAB-03 | A laboratory scientist shall enter one or more result parameters for a collected order. | Must | Implemented |
| FR-LAB-04 | The first result shall close the order or move it to a completed state according to the test configuration. | Must | Implemented |
| FR-LAB-05 | Results shall be append-only and visible to the ordering clinician. | Must | Implemented |
| FR-LAB-06 | The system shall show an error and refuse a result when the specimen has not been collected. | Must | Implemented |
| FR-LAB-07 | Laboratory work shall remain available offline and synchronise later. | Must | Implemented |

### 3.1.10 Pharmacy

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-PHA-01 | The pharmacy screen shall list paid prescriptions ready for dispensing. | Must | Implemented |
| FR-PHA-02 | The system shall refuse to dispense a prescription whose invoice is not paid. | Must | Implemented |
| FR-PHA-03 | A pharmacist shall record a dispense fact and the dispensing user and time. | Must | Implemented |
| FR-PHA-04 | The prescription shall show whether it is waiting for payment, ready or dispensed. | Must | Implemented |
| FR-PHA-05 | Dispensing shall work offline and synchronise later. | Must | Implemented |
| FR-PHA-06 | Pharmacy inventory and stock deduction shall not be part of the MVP. | Won't | Out of scope |

### 3.1.11 Billing and Cashier

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-BIL-01 | The system shall create invoices for drug, laboratory and ward charges. | Must | Implemented |
| FR-BIL-02 | Each charge line shall snapshot the unit price and amount at the time of posting. | Must | Implemented |
| FR-BIL-03 | A cashier shall record a payment by cash, transfer or POS card. | Must | Implemented |
| FR-BIL-04 | A payment shall be idempotent; replaying the same operation shall not create a second payment. | Must | Implemented |
| FR-BIL-05 | The system shall support automatic oldest-first allocation across outstanding charges. | Must | Implemented |
| FR-BIL-06 | The system shall support explicit allocation for the exception case. | Should | Implemented |
| FR-BIL-07 | The balance shall be derived from active charges minus allocations and shall not be stored as an independent editable field. | Must | Implemented |
| FR-BIL-08 | The system shall prevent over-allocation and over-settlement of a charge. | Must | Implemented |
| FR-BIL-09 | The cashier screen shall show unpaid and partially paid invoices and their derived balances. | Must | Implemented |
| FR-BIL-10 | Billing shall work offline, and payments shall synchronise with idempotency. | Must | Implemented |
| FR-BIL-11 | The system shall record a receipt number and the staff member who received the money. | Must | Implemented |
| FR-BIL-12 | Formal financial reports and statutory exports shall be post-MVP. | Could | Deferred |

### 3.1.12 Inpatient Admission and Ward

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-WAR-01 | A nurse, doctor or administrator shall admit a patient from a consultation with a ward and optional bed label. | Must | Implemented |
| FR-WAR-02 | Admission shall move the encounter to the admitted stage and remove it from the active consultation queue. | Must | Implemented |
| FR-WAR-03 | The system shall prevent a second open admission for the same patient. | Must | Implemented |
| FR-WAR-04 | The ward board shall list admitted patients, billed nights and charges. | Must | Implemented |
| FR-WAR-05 | A ward user shall charge one night per admission per date; a duplicate night shall be a no-op. | Must | Implemented |
| FR-WAR-06 | A ward charge shall create or update a ward invoice and an active charge line. | Must | Implemented |
| FR-WAR-07 | A ward user shall discharge a patient, and discharge shall close the encounter. | Must | Implemented |
| FR-WAR-08 | The client shall refuse discharge while the known ward balance is outstanding. | Must | Implemented |
| FR-WAR-09 | The server shall refuse discharge while the derived ward balance is outstanding, returning a conflict with the amount. | Must | Implemented |
| FR-WAR-10 | Ward work shall remain available offline and synchronise later. | Must | Implemented |
| FR-WAR-11 | A ward/bed master table, bed grid and bed occupancy display shall be post-MVP. | Should | Deferred |
| FR-WAR-12 | Nursing notes, transfer between wards and discharge summary templates shall be post-MVP. | Should | Deferred |
| FR-WAR-13 | An automatic nightly accrual runner shall be post-MVP; the MVP charges a night on demand. | Should | Deferred |

### 3.1.13 Referral Management and Public QR Access

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-REF-01 | A doctor, nurse or administrator shall issue a referral for a patient and encounter with receiving facility, reason, optional diagnosis summary, optional treatment given and urgency. | Must | Implemented — online-only |
| FR-REF-02 | The server shall validate that the encounter belongs to the patient and that the urgency is routine, urgent or emergency. | Must | Implemented |
| FR-REF-03 | The server shall generate a referral number and a cryptographically sealed token with a 48-hour expiry. | Must | Implemented |
| FR-REF-04 | The client shall render the token as a QR code on the issuing device. | Must | Implemented |
| FR-REF-05 | The client shall provide a printable referral slip containing the QR code and the public URL. | Must | Implemented |
| FR-REF-06 | A receiving clinician without an account shall open the public referral page and read the referral payload. | Must | Implemented — online-only |
| FR-REF-07 | The server shall reject a forged, malformed, expired or revoked token without disclosing which check failed. | Must | Implemented |
| FR-REF-08 | The referral payload shall not contain a patient UUID, phone number, address or other lookup identifier. | Must | Implemented |
| FR-REF-09 | The token shall be returned once and shall not be stored in the database. | Must | Implemented |
| FR-REF-10 | The public page shall render on a mobile phone without an account and without local clinical data. | Must | Implemented |
| FR-REF-11 | A referral PIN challenge shall be required before the note is displayed. | Should | Deferred |
| FR-REF-12 | Every referral read attempt shall be written to an access log. | Should | Deferred |
| FR-REF-13 | The issuing clinic shall be able to list issued referrals and revoke a live referral. | Should | Deferred — the server recognises a revoked status, but no register or revocation UI is built |
| FR-REF-14 | A referral shall be reprintable from the system. | Could | Deferred — deliberately not planned because the token is not stored |
| FR-REF-15 | Referral creation shall be available while offline. | Could | Deferred — the MVP requires connectivity to issue a server-sealed token |

### 3.1.14 Offline Synchronisation and Conflict Handling

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-SYN-01 | Every local domain write shall be committed to IndexedDB and the outbox in the same transaction. | Must | Implemented |
| FR-SYN-02 | The client shall push queued operations in order with a batch identifier and per-operation identifiers. | Must | Implemented |
| FR-SYN-03 | The server shall make push idempotent: retrying an applied batch shall not duplicate rows or money. | Must | Implemented |
| FR-SYN-04 | Sync push shall be signed with the registered device key. | Must | Implemented |
| FR-SYN-05 | The client shall pull changes after a monotonic cursor and shall not skip writes made during bootstrap. | Must | Implemented |
| FR-SYN-06 | A new device shall bootstrap from table snapshots before adopting a cursor. | Must | Implemented |
| FR-SYN-07 | Mutable rows shall use whole-row last-write-wins and preserve the losing payload in `sync_conflicts`. | Must | Implemented |
| FR-SYN-08 | Append-only facts shall reject update and merge operations. | Must | Implemented |
| FR-SYN-09 | Deletes shall be soft tombstones and shall replicate. | Must | Implemented |
| FR-SYN-10 | Failed sync shall retry with backoff and shall not block clinical writes. | Must | Implemented |
| FR-SYN-11 | The interface shall show pending, syncing, offline and error states. | Must | Implemented |
| FR-SYN-12 | Table read and write roles shall be enforced during sync, matching the REST roles. | Must | Implemented |
| FR-SYN-13 | Per-field merge or CRDT resolution shall be post-MVP. | Could | Deferred |
| FR-SYN-14 | A sync-health dashboard with per-device lag and error rates shall be post-MVP. | Should | Deferred |
| FR-SYN-15 | A full offline soak test and documented performance evidence shall be post-MVP. | Should | Deferred |

### 3.1.15 Dashboard, Audit and Administration

| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| FR-AUD-01 | The system shall record a change-log entry for every applied state change. | Must | Implemented |
| FR-AUD-02 | An administrator shall view recent activity with actor, entity, action and time. | Must | Implemented |
| FR-AUD-03 | Ordinary users shall not be able to edit or delete audit entries. | Must | Implemented |
| FR-AUD-04 | The dashboard shall show live local counts for triage, doctor queue, pending laboratories and inpatients. | Must | Implemented |
| FR-AUD-05 | The dashboard shall show needs-attention counts and the outstanding balance. | Should | Implemented |
| FR-AUD-06 | The dashboard shall provide role-appropriate quick actions. | Should | Implemented |
| FR-AUD-07 | A global command palette shall search patients by name, number, phone or identifier and navigate to departments. | Should | Implemented |
| FR-AUD-08 | The command palette shall provide an emergency check-in shortcut and department jump mode. | Should | Implemented |
| FR-AUD-09 | The system shall provide a sync status screen for support and demonstration. | Should | Implemented |
| FR-AUD-10 | Formal management reports, charts and CSV/PDF exports shall be post-MVP. | Could | Deferred |

## 3.2 Non-Functional Requirements

Non-functional requirements describe the quality attributes the system must exhibit. They are grouped under the categories requested for this project: security, reliability, usability, performance, maintainability and availability, followed by compliance and record retention.

### 3.2.1 Security and Privacy
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-SEC-01 | All client-server traffic shall use HTTPS. | Must | Implemented |
| NFR-SEC-02 | Passwords shall be stored with a memory-hard password hash; plaintext passwords shall never be stored or logged. | Must | Implemented — Argon2id |
| NFR-SEC-03 | Access tokens shall be short-lived; refresh tokens shall rotate and shall be revocable. | Must | Implemented |
| NFR-SEC-04 | The API shall enforce role-based authorisation on every protected action. | Must | Implemented |
| NFR-SEC-05 | Sync pushes shall be signed by a registered device key and checked by the server. | Must | Implemented |
| NFR-SEC-06 | Referral payloads shall be encrypted with AES-256-GCM and shall be tamper-evident. | Must | Implemented |
| NFR-SEC-07 | The referral payload shall be data-minimised and shall not contain patient UUID, phone or address. | Must | Implemented |
| NFR-SEC-08 | Referral tokens shall expire after 48 hours and shall be rejected after expiry. | Must | Implemented |
| NFR-SEC-09 | A revoked device shall be unable to refresh or sync after revocation. | Must | Implemented |
| NFR-SEC-10 | The system shall rate-limit authentication and referral reads to reduce brute-force and enumeration risk. | Should | Implemented |
| NFR-SEC-11 | The public referral page shall not cache a note across patients or persist it in the local clinical database. | Must | Implemented |
| NFR-SEC-12 | Sensitive values shall not be written to client logs, console output or analytics. | Must | Implemented by design |
| NFR-SEC-13 | A content security policy shall restrict script, style, font and connection sources. | Should | Implemented |
| NFR-SEC-14 | A formal third-party penetration test and independent security certification shall be post-MVP. | Should | Deferred |

### 3.2.2 Reliability and Data Integrity
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-REL-01 | A local write and its outbox entry shall be atomic. | Must | Implemented |
| NFR-REL-02 | A retried sync batch shall not duplicate a row, a payment or a dispense. | Must | Implemented |
| NFR-REL-03 | Conflicts shall be preserved for review rather than silently discarded. | Must | Implemented |
| NFR-REL-04 | The system shall not hard-delete clinical or financial rows through the application. | Must | Implemented |
| NFR-REL-05 | The server shall use database transactions for multi-row clinical and financial operations. | Must | Implemented |
| NFR-REL-06 | Discharge shall be refused while a ward balance is outstanding. | Must | Implemented |
| NFR-REL-07 | The database shall support point-in-time recovery or scheduled backups. | Must | Deployment responsibility — Neon supports PITR |
| NFR-REL-08 | A device shall be able to re-bootstrap from a snapshot after cursor loss or retention expiry. | Must | Implemented |
| NFR-REL-09 | Failed sync operations shall surface an actionable status rather than a silent spinner. | Must | Implemented |
| NFR-REL-10 | A complete restore drill shall be documented post-MVP. | Should | Deferred |

### 3.2.3 Usability and Accessibility
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-USE-01 | The interface shall use a calm, high-contrast, light clinical theme with tabular figures for counts and money. | Must | Implemented |
| NFR-USE-02 | Numeric displays shall use tabular figures so counts do not shift as values change. | Must | Implemented |
| NFR-USE-03 | Colour shall not be the only carrier of clinical meaning; text and icons shall accompany status colours. | Must | Implemented |
| NFR-USE-04 | Primary text and controls shall meet WCAG 2.1 AA contrast. | Must | Implemented by token design |
| NFR-USE-05 | Forms shall have visible labels, keyboard focus and inline error messages. | Must | Implemented |
| NFR-USE-06 | Registration and triage shall be operable by keyboard, including Enter, Tab and documented shortcuts. | Should | Implemented |
| NFR-USE-07 | The layout shall remain usable from 360 px mobile width to a large desktop. | Must | Implemented |
| NFR-USE-08 | The interface shall avoid animation that delays the first clinical action. | Must | Implemented |
| NFR-USE-09 | Offline status shall be explained in plain language, not as a red error screen. | Must | Implemented |
| NFR-USE-10 | Printed referral slips shall be legible on 58–80 mm thermal paper. | Must | Implemented via print styles |
| NFR-USE-11 | The public referral page shall be readable on a phone by a non-account holder. | Must | Implemented |
| NFR-USE-12 | Full screen-reader auditing of every screen shall be post-MVP. | Should | Deferred |

### 3.2.4 Performance and Scalability
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-PERF-01 | Local list and dashboard queries should return within 100 ms for a facility-scale dataset. | Should | Implemented by design — Dexie local queries |
| NFR-PERF-02 | Dashboard counters shall update without a page reload when local data changes. | Must | Implemented |
| NFR-PERF-03 | The application should render its authenticated shell within 2 seconds on a low-power workstation after the shell is cached. | Should | Implemented by design |
| NFR-PERF-04 | The API should respond to normal single-row operations within 500 ms at p95 on a healthy link. | Should | Supported |
| NFR-PERF-05 | Sync batches shall be bounded to protect the server and the device on a slow link. | Must | Implemented |
| NFR-PERF-06 | The system shall support a single facility with at least 8 roles, 10 workstations and tens of thousands of patient and encounter rows without redesign. | Should | Supported by design |
| NFR-PERF-07 | The public referral page should load on a mobile connection in under 3 seconds when the API is reachable. | Should | Supported |
| NFR-PERF-08 | Offline soak tests with a large outbox and prolonged disconnection shall be documented post-MVP. | Should | Deferred |

### 3.2.5 Maintainability and Portability
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-MNT-01 | The API shall use a layered structure with HTTP, service, repository and domain concerns separated. | Must | Implemented |
| NFR-MNT-02 | The web client shall use strict TypeScript and a shared design-token layer. | Must | Implemented |
| NFR-MNT-03 | Database changes shall be versioned migrations with working down migrations. | Must | Implemented |
| NFR-MNT-04 | The sync protocol shall be table-extensible through a registry rather than ad-hoc branches. | Must | Implemented |
| NFR-MNT-05 | The system shall include automated unit, integration and end-to-end tests for critical paths. | Must | Partial — Go unit tests, Playwright E2E, frontend unit suite requires the declared Node version |
| NFR-MNT-06 | The application shall run in modern Chromium, Firefox and Safari-based browsers with the required web APIs. | Should | Supported |
| NFR-MNT-07 | The application shall be installable as a PWA where the browser supports it. | Should | Partial — manifest and service worker exist; install prompt is post-MVP |
| NFR-MNT-08 | Configuration and secrets shall come from environment variables, not source code. | Must | Implemented |
| NFR-MNT-09 | The system shall expose health and readiness endpoints for deployment checks. | Must | Implemented |

### 3.2.6 Availability and Offline Operation
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-AVA-01 | The clinical workflow shall remain available for registration, vitals, consultation, orders, payment, dispensing and ward work while the network is unavailable. | Must | Implemented |
| NFR-AVA-02 | Offline writes shall be durable across a page reload and shall survive closing the browser tab. | Must | Implemented |
| NFR-AVA-03 | The application shell shall be cached by a service worker so a reload does not produce a blank page while offline. | Must | Implemented |
| NFR-AVA-04 | The interface shall distinguish online, syncing, pending, offline and offline-writes-blocked states. | Must | Implemented |
| NFR-AVA-05 | No clinical screen shall depend on a live server request to render its already-synchronised data. | Must | Implemented |

### 3.2.7 Compliance and Record Retention
| ID | Requirement | Priority | MVP status |
|---|---|---|---|
| NFR-PRIV-01 | The system shall collect only data needed for care, billing and referral. | Must | Implemented |
| NFR-PRIV-02 | Access to clinical and financial data shall follow least privilege. | Must | Implemented |
| NFR-PRIV-03 | Audit records shall be retained and protected from ordinary-user alteration. | Must | Implemented |
| NFR-PRIV-04 | Change-log retention shall be configurable and pruned by a dedicated process. | Must | Implemented |
| NFR-PRIV-05 | Patient data shall not be sent to third-party analytics or advertising services. | Must | Implemented by design |
| NFR-PRIV-06 | A formal data-protection impact assessment and retention schedule shall be completed before live deployment. | Should | Deferred |

<!-- pagebreak -->

# 4. External Interface Requirements and Dependencies

## 4.1 User Interface Requirements

### 4.1.1 Interface Principles

- The browser is the workspace, not a thin form over a server.
- The active staff member and role are always visible.
- The current sync state is always visible, but never blocks clinical work.
- Data density is preferred over decorative whitespace on clinical screens.
- Every list has an empty state that explains the next action.
- Colour is supported by text and icons.
- The primary clinical theme is light, high-contrast and glare-resistant.

### 4.1.2 Main Screens

| Screen | Route | Main users | Purpose |
|---|---|---|---|
| Sign in | `/login` | All staff | Staff number and password; offline sign-in where permitted |
| Dashboard | `/` | All staff | Role-focused shift cockpit, live counts, quick actions and department tiles |
| Patients | `/patients` | All staff | Search and browse the local patient register |
| Register patient | `/patients/new` | Front desk, doctor, nurse, admin | Individual and family registration |
| Patient profile | `/patients/$id` | Clinical staff | Demographics, visits, history and actions |
| Queue | `/queue` | Front desk, nurse, doctor, admin | Triage and consultation board |
| Consultation | `/consultations/$encounterId` | Doctor, nurse | Diagnosis, orders, prescriptions, referral and admission |
| Laboratory | `/lab` | Laboratory, doctor, nurse | Specimen and result worklists |
| Pharmacy | `/pharmacy` | Pharmacist, doctor, nurse | Paid prescriptions and dispensing |
| Billing | `/billing` | Cashier, front desk, accountant, admin | Invoices, balances and payments |
| Ward | `/ward` | Nurse, doctor, admin | Inpatients, bed-days and discharge |
| Refer a patient | `/referrals/new` | Doctor, nurse, admin | Issue a referral and render its QR |
| Public referral | `/r/$token` | Receiving clinician | Read a referral without an account |
| Staff accounts | `/admin/users` | Administrator | Staff, roles and devices |
| Activity | `/admin/activity` | Administrator | Audit trail |
| Change password | `/change-password` | All staff | Self-service password change |
| Session expired | `/session-expired` | All staff | Recover from a revoked or expired session |
| Sync status | `/dev/sync` | Support, developer | Inspect sync state and conflicts |

### 4.1.3 Navigation and Keyboard Behaviour

- The sidebar is role-aware and marks the active destination.
- A global command palette opens with `Ctrl+K` or `Cmd+K`.
- The palette searches patients by name, hospital number, phone and identifier.
- Typing `>` in the palette switches to department navigation.
- Typing `!` switches to emergency check-in.
- `Alt+N` opens registration; `Alt+T` opens the queue; `Alt+L` opens the laboratory; `Alt+D` opens pharmacy; `Alt+P` opens billing; `Alt+A` opens administration activity where permitted.
- Arrow keys move the palette highlight; Enter opens the selection; Escape closes the palette.
- Forms use visible labels and logical tab order.

### 4.1.4 Offline Feedback

| State | User-facing meaning |
|---|---|
| All records synced | Local changes have reached the server |
| Syncing | A push or pull is in progress |
| N waiting to sync | Saved locally, not yet sent |
| Offline — working locally | No uplink; clinical work continues |
| Offline — writes blocked | Offline grace has expired; sign in when connectivity returns |
| Reconnect required | Session or device needs re-authentication |
| Sync problem | An actionable error is available in the sync view |

## 4.2 Hardware Requirements

| Interface | Requirement |
|---|---|
| Workstation | Desktop or laptop with a modern browser, keyboard, pointing device and local storage |
| Display | Minimum practical width 360 px; clinical screens are optimised for 1280 px and above |
| Thermal printer | 58 mm or 80 mm POS printer accessible through the browser print dialog |
| QR scanner | Any phone camera or handheld scanner capable of opening a URL from a QR code |
| Network interface | Facility LAN or mobile uplink; the browser may be offline for long periods |
| Power | The system itself does not control power; an uninterruptible supply is recommended for the workstation and printer |
| No specialised clinical hardware | The MVP does not integrate with analysers, ECG machines or bedside monitors |

## 4.3 Software Requirements

### 4.3.1 Browser Platform Interfaces

| Interface | Use |
|---|---|
| IndexedDB through Dexie | Local clinical database, outbox, sync cursor and cached metadata |
| Service Worker and Cache Storage | Offline application shell |
| Web Crypto | Device key generation, signing and local credential verification |
| Canvas | QR code rendering on the issuing device |
| Clipboard and print dialog | Copy referral URL and print a slip or receipt |
| Web Storage | Small non-clinical preferences where used |
| TanStack Query | Public referral read and online-only queries |
| TanStack Router | Typed client routing and route search parameters |

### 4.3.2 Server Interfaces

| Interface | Use |
|---|---|
| Go HTTP API | Authentication, RBAC, clinical services, billing, ward, referrals and sync |
| PostgreSQL through pgx | Transactions, row storage, change log, conflicts and audit |
| Neon | Managed PostgreSQL, connection pooling and point-in-time recovery |
| Cloud Run or equivalent | API container hosting |
| Cloudflare Pages or equivalent | Static web hosting, TLS and CDN |
| Referral encryption key | AES-256-GCM sealing and opening of referral tokens |
| JWT signing key | Access-token signing and verification |
| Device public keys | Signature verification for sync pushes |

### 4.3.3 Third-Party Libraries

The MVP uses open-source libraries including Go, Gin, pgx, React, Vite, TanStack Router, TanStack Query, Dexie, Dexie React Hooks, Tailwind CSS, qrcode, Zustand, Sonner, React Hook Form, Zod and Lucide React. These are dependencies, not external services; no patient data is sent to them.

## 4.4 Database Requirements

### 4.4.1 Central Database

- The central store shall be PostgreSQL, hosted on Neon or an equivalent managed service.
- The API shall connect through a pooled connection for normal traffic and a direct connection for migrations.
- Multi-row clinical and financial operations shall use database transactions so that an admission, invoice, charge or payment cannot be half-written.
- The database shall keep an ordered change log for sync pull, sync operations for idempotency and conflict rows for losing writes.
- The database shall support point-in-time recovery or scheduled backups appropriate to the facility's risk assessment.
- Schema changes shall be applied through versioned migrations, not manual production edits.

### 4.4.2 Local Database

- The browser shall keep a local IndexedDB database through Dexie, including clinical rows, an outbox, a sync cursor and cached metadata.
- A domain write and its outbox entry shall be committed in the same local transaction.
- The local schema shall be versioned and upgraded without discarding unsynced clinical work.
- Local data shall be scoped to the browser profile and the device; clearing site data is a destructive recovery step.
- IndexedDB quota shall be monitored and the facility shall have an archival or device-replacement procedure.

### 4.4.3 Data Integrity Rules

| Rule | Purpose |
|---|---|
| Unique patient hospital number | Prevent duplicate registration identifiers |
| Unique invoice number | Prevent duplicate bills |
| Unique ward charge per admission and date | Prevent double-billing a night |
| Unique sync operation identifier | Make push idempotent |
| Foreign keys for clinical relationships | Prevent orphaned records |
| Soft-delete tombstones | Replicate deletion without losing audit history |
| Sync columns on every synced table | Revision, server revision, sync state and timestamps |
| Change-log triggers | Produce a pullable change for every applied state change |

### 4.4.4 Retention and Recovery

- Clinical and financial rows shall not be hard-deleted through the application.
- Change-log retention shall be configurable and pruned by a dedicated process.
- The facility shall retain a documented backup and restore procedure.
- A full restore drill is recommended before live deployment; the MVP records this as a post-MVP operational task.

### 4.4.5 Database Access

- The API shall use a least-privilege database role for runtime operations.
- Migration credentials shall not be available to the browser or the normal API process.
- Database traffic shall use TLS.
- Patient data shall not be written to application logs, analytics or error-tracking services.

## 4.5 External Systems, APIs and Internet Connectivity

### 4.5.1 Protocol

- HTTPS with JSON request and response bodies.
- Base path `/api/v1`.
- Bearer access token for authenticated requests.
- Refresh token rotation for session continuation.
- Device signature headers on sync push.
- Standard response envelope with `data` or `error`, including a request identifier.

### 4.5.2 Main API Groups

| Group | Examples | Purpose |
|---|---|---|
| Health | `GET /health`, `GET /ready`, `GET /api/v1/meta` | Deployment checks and facility metadata |
| Authentication | `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `POST /auth/offline-verify`, `POST /auth/change-password` | Identity and session |
| Devices | `POST /devices`, admin revocation | Workstation trust |
| Patients | Search, read, create, update, family registration | Registration and records |
| Queue and encounters | Queue read, start visit, transition, vitals | Clinical workflow |
| Consultation | Diagnoses, laboratory orders, prescriptions | Doctor work |
| Laboratory | Collect, result list, result entry | Laboratory work |
| Pharmacy | Dispense | Pharmacy work |
| Billing | Create invoice, balance, record payment | Cashier work |
| Ward | Admit, accrue bed-day, discharge | Inpatient work |
| Referrals | Create referral, public read | QR referral |
| Administration | Staff list/create/update, activity, device revoke | Administration |
| Sync | Push, pull, snapshot | Replication |

### 4.5.3 Referral URL Interface

- The QR code encodes a URL of the form `https://<facility-web-host>/r/<token>`.
- The token is opaque, URL-safe and contains the encrypted referral payload.
- The public page calls `GET /api/v1/referrals/<token>` and renders the note if the token is valid and unexpired.
- The token is not stored in the database; the referral record stores the referral number and lifecycle, not the token.
- The public page does not require an account, a cookie or local clinical storage.

### 4.5.4 Sync Interface

- `POST /api/v1/sync/push` sends a bounded batch of outbox operations.
- `GET /api/v1/sync/pull` returns ordered changes after a cursor.
- `GET /api/v1/sync/snapshot` bootstraps a table for a device with no cursor.
- The client is single-flight: one sync run at a time, with a queued re-run if a request arrives during a run.
- A push applies operations transactionally and records idempotency outcomes.
- A pull applies rows and advances the cursor only after a successful batch.

### 4.5.5 CORS, CSP and Hosting

- The API allows the configured web origin only.
- The web application sets a content security policy that permits the application origin and the configured API origin.
- Fonts are local or system fonts; no third-party font or analytics request is required.
- Service-worker precaching excludes the public referral read from cross-patient caching.

## 4.6 External Dependencies

| Dependency | Type | Failure effect | Mitigation |
|---|---|---|---|
| Neon PostgreSQL | Managed database | No sync, login or referral issue | Local clinical work continues; Neon PITR; retry when restored |
| Go API host | Application server | No sync, login or referral | Local clinical work continues; cached shell; clear status |
| Web host and CDN | Static application | New page loads fail; installed cache may still work | Service-worker shell; retry |
| DNS and TLS | Network infrastructure | Browser cannot open the app or API | Use reliable providers; monitor certificates |
| Referral encryption key | Secret | Referral issue/read fails | Secret management; documented rotation plan |
| JWT signing key | Secret | Sessions cannot be verified | Secret management; revocation on compromise |
| Thermal printer | Hardware | No printed slip or receipt | Reprint through browser print where the screen is available; referral reissue |
| Smartphone camera | External hardware | Receiving facility cannot scan QR | Open the printed URL manually |
| Electricity | Facility infrastructure | Workstation stops | UPS recommendation; local data survives restart |

## 4.7 Deployment and Operating Environment

### 4.7.1 Production Topology

```
Browser (PWA)  ──HTTPS──▶  Static web host / CDN
      │                         │
      └────────HTTPS───────────▶ Go API on Cloud Run
                                  │
                                  ▼
                            Neon PostgreSQL
```

### 4.7.2 Environment Configuration

Required configuration includes the database URL, JWT signing key, referral encryption key, allowed web origin, session lifetimes, offline grace policy, device signature settings and server port. Secrets are supplied through environment variables and are never committed.

### 4.7.3 Supported Clients

- Current Chromium-based browsers.
- Current Firefox.
- Current Safari on macOS and iOS, subject to IndexedDB and service-worker support.
- The public referral page is designed to work on low-end mobile browsers.

## 4.8 Degraded-Mode Behaviour

| Condition | Expected behaviour |
|---|---|
| No network at start of shift | Cached session and app shell load; local data renders; sync badge shows offline |
| Network drops during a consultation | Consultation and orders continue locally; sync resumes automatically |
| Sync fails repeatedly | Clinical work continues; the badge shows a sync problem; the sync screen explains the error |
| Offline grace expires | Clinical writes are blocked; the UI explains that a reconnect is required |
| Referral issue attempted offline | The action is refused with a clear online-only message; no local token is fabricated |
| Public referral page opened without network | The page cannot read the note; it must be opened with connectivity |
| Ward discharge attempted with outstanding balance | The client and server refuse and show the amount |
| Device revoked | Sync and refresh fail; the user is sent to the session-recovery screen |
| Server unreachable during a payment | The payment is saved locally with an idempotency key and retried safely |

<!-- pagebreak -->

# 5. Appendices and Glossary

## 5.1 Appendix A: References

1. ISO/IEC/IEEE 29148, *Systems and software engineering — Life cycle processes — Requirements engineering*.
2. IEEE 830, *Recommended Practice for Software Requirements Specifications* (structure reference).
3. Project README, *Offline-capable patient records and referral management*.
4. `docs/00-ARCHITECTURE.md`, system architecture and deployment decisions.
5. `docs/02-SYNC-PROTOCOL.md`, sync protocol, outbox, cursors and conflicts.
6. `docs/03-UI-PATTERNS.md`, interface patterns and accessibility rules.
7. `docs/04-CODING-STANDARDS.md`, engineering conventions.
8. `docs/05-SECURITY.md`, threat model, authentication, RBAC and referral security.
9. `docs/07-API-CONTRACT.md`, REST endpoints and error semantics.
10. `docs/08-ROADMAP.md`, sprint scope and planned deliverables.
11. `docs/USER-MANUAL.md`, role-based operating instructions.
12. `docs/DEMO-SCRIPT.md`, assessed demonstration path and deliberate limitations.
13. Nigeria Data Protection Regulation (NDPR) 2019, used as the privacy reference for patient data handling.

## 5.2 Appendix B: Implementation Status and Verification

### 5.2.1 Verified Demo Path

The following automated checks were run against the audited codebase:

| Check | Result |
|---|---|
| Patient registration and cross-device visibility | Passed |
| Queue, vitals and doctor handoff across two devices | Passed |
| Consultation, diagnosis, laboratory order and result | Passed |
| Billing, payment allocation and replay protection | Passed |
| Referral issue, public read and forgery rejection | Passed |
| Two-device sync convergence and offline recovery | Passed |
| Go unit tests and `go vet` | Passed |
| TypeScript typecheck and production build | Passed |
| Manual admission, bed-day invoice, unpaid discharge gate and paid discharge | Passed |

### 5.2.2 Implementation Status by Module

| Module | Status | Notes |
|---|---|---|
| Authentication and offline sign-in | Implemented | Cached credential verifier and device key |
| Staff administration and RBAC | Implemented | Staff create, role change, device revocation and activity view |
| Patient registration and households | Implemented | Individual and family registration; duplicate warning |
| Queue and vitals | Implemented | Append-only observations; stage transitions |
| Consultation and diagnosis | Implemented | ICD-10 suggestions and notes |
| Laboratory | Implemented | Collection and result entry |
| Pharmacy | Implemented | Dispensing only after payment |
| Billing | Implemented | Derived balances, automatic allocation, idempotent payments |
| Ward | Partial | Admission, manual bed-day charge and discharge gate; ward/bed master data and notes deferred |
| Referral QR | Partial | Encrypted token, QR, public read and expiry; PIN, access log, register and revocation deferred |
| Offline sync | Implemented | Outbox, signatures, push/pull/snapshot, LWW and conflict preservation |
| Dashboard and audit | Implemented | Live counts, command palette, activity view |
| Reports and exports | Deferred | Post-MVP |
| Password change | Implemented | Self-service; current session preserved |
| Forced first-login password change | Partial | Banner prompt only |
| Administrator password reset | Deferred | Not built |
| PWA install prompt | Deferred | Manifest and service worker exist |

### 5.2.3 MVP Completion Summary

| Area | MVP position |
|---|---|
| Authentication, offline sign-in and device trust | Complete |
| Staff administration and RBAC | Complete |
| Patient and household registration | Complete |
| Queue, vitals and triage | Complete |
| Consultation, diagnosis, laboratory order and prescription | Complete |
| Laboratory collection and results | Complete |
| Pharmacy dispensing | Complete |
| Billing and cashiering | Complete |
| Inpatient admission, bed-day billing and discharge gate | Complete MVP slice |
| QR referral issue and public read | Complete core; PIN/access log/register/revocation deferred |
| Offline sync and conflicts | Complete |
| Dashboard, audit and command palette | Complete |
| Reports, exports and management analytics | Deferred |
| Ward/bed master data, nursing notes, transfers, automatic accrual | Deferred |
| Forced first-login password change and admin reset | Partial and deferred |

## 5.3 Appendix C: Referral QR System — Built and Deferred

### 5.3.1 Direct Answer

**Yes — the core QR-code patient referral system is implemented.**

The implemented path is:

1. A doctor, nurse or administrator issues a referral from a consultation.
2. The API validates the patient, encounter, receiving facility, reason and urgency.
3. The API generates a referral number and a 48-hour expiry.
4. The API encrypts the referral payload with AES-256-GCM and returns an opaque token once.
5. The issuing browser renders the token as a QR code on the device.
6. The slip can be printed on a 58–80 mm thermal printer and includes the public URL.
7. A receiving clinician opens `/r/<token>` on any phone or browser; no account is required.
8. The public page calls `GET /api/v1/referrals/<token>`.
9. The API decrypts the token, checks the expiry and rejects forged, malformed, expired or revoked tokens.
10. The note is rendered with patient name, age, sex, referring and receiving facilities, reason, diagnosis summary, treatment given and urgency.

The end-to-end test **“a note issued here is readable by a stranger, and cannot be forged”** passes.

### 5.3.2 Security Properties of the Implemented Referral

- The token is encrypted, not merely encoded; changing one byte invalidates it.
- The payload deliberately excludes the patient UUID, phone number, address and other lookup identifiers.
- The token is returned once and is not stored in the database; a database compromise does not reveal outstanding QR secrets.
- The 48-hour expiry is checked by the service on every read.
- A revoked referral status, if set, is honoured on read.
- Error messages do not explain whether the failure was signature, payload or expiry; this avoids giving an attacker a useful oracle.
- The public page does not cache the note across patients and does not write it to the clinical local database.

### 5.3.3 Referral Features Not Implemented

| Feature | Status | Consequence |
|---|---|---|
| PIN challenge before showing the note | Deferred | Anyone who possesses the QR link can read the note during its 48-hour life |
| Referral access log | Deferred | The issuing clinic cannot see when or where the note was opened |
| Referral register | Deferred | There is no screen listing all issued referrals |
| Revocation UI | Deferred | The server recognises a revoked status, but a user cannot revoke from the interface |
| Reprint from the system | Not planned | The token is not stored; a lost slip requires a new referral |
| Offline referral issue | Deferred | Referral creation requires connectivity because the token is server-sealed |

These are genuine limitations, not hidden failures. They are listed in the project tracker and in the demonstration script.

### 5.3.4 Why Some Items Are Deliberately Deferred

- A PIN adds a second factor but also adds a keypad interaction on the receiving side; the MVP prioritised a working, tamper-evident, no-account read path.
- An access log requires a read event table and a privacy decision about what the receiving facility may log; it is a post-MVP increment.
- A register and revocation UI require a referral list endpoint and a lifecycle screen; the database already accommodates a revoked status.
- Reprinting would require storing the token, which would undermine the core encryption design.

## 5.4 Appendix D: Requirement Traceability Summary

| Requirement area | Primary implementation evidence | Verification evidence |
|---|---|---|
| Authentication and RBAC | Auth service, middleware, role navigation, session store | Login and role E2E; session refresh; revocation checks |
| Patient registration | Patient service, Dexie patient tables, registration forms | Patients E2E across two devices |
| Queue and vitals | Encounter state machine, queue board, vitals write path | Queue E2E across two devices |
| Consultation and orders | Consultation service, diagnosis form, lab order and prescription writes | Consultation E2E |
| Laboratory | Collection and result endpoints and worklists | Consultation/lab E2E |
| Pharmacy | Dispense endpoint and paid-prescription worklist | Billing and pharmacy flow |
| Billing | Invoice, charge, payment and allocation services | Billing E2E including replay protection |
| Ward | Admission service, ward board, local invoice creation, discharge gate | Manual admission, invoice, 409 gate and 200 discharge evidence |
| Referral QR | Referral service, QR renderer, public page, print styles | Referral E2E including forgery rejection |
| Offline sync | Outbox, signed push, pull cursor, snapshot, conflict store | Sync E2E and recovery test |
| Audit | Change log, activity endpoint and activity screen | Administration and audit checks |
| Password change | Change-password endpoint, page and session-preserving revocation | API change-password and refresh evidence |

## 5.5 Appendix E: Assumptions, Limitations and Future Work

### 5.5.1 Limitations That Must Be Stated in the Defence

1. **Referral creation is online-only.** The core clinical workflow is offline-capable; a referral token is server-sealed and therefore needs connectivity.
2. **Referral PIN and access logging are not built.** The QR note is encrypted and expires, but possession of the link is sufficient to read it.
3. **No referral register or revocation UI.** The server model can represent revocation, but the user interface cannot yet issue one.
4. **No reports or exports.** Dashboards and the audit view exist; formal management reporting does not.
5. **Ward scope is an MVP slice.** Wards and beds are labels, not a master-data model; there are no nursing notes, transfers or automatic nightly accrual.
6. **Whole-row LWW.** Two clinicians editing different fields of the same row can lose one edit; the losing payload is preserved for review.
7. **Temporary passwords are prompted, not forced.** The demo accounts remain usable with the shared seed password.
8. **No administrator password reset.** Staff can change their own password; an administrator cannot reset another account in the MVP.
9. **No formal offline soak, performance or restore drill evidence yet.** The architecture supports these tests, but the evidence is post-MVP.
10. **Not a certified medical device or a national interoperability product.** It is a facility-level MVP.

### 5.5.2 Recommended Future Work

1. Add referral PIN challenge, access log, referral register and revocation UI.
2. Add an administrator password reset and enforce first-login password change outside the demo accounts.
3. Add reports and export for daily attendance, revenue, laboratory turnaround and ward occupancy.
4. Add ward/bed master data, a bed grid, nursing notes, transfers and an automatic nightly accrual runner.
5. Add a sync-health dashboard with per-device lag, conflict counts and error rates.
6. Run and document offline soak tests, large-outbox tests, restore drills and low-power workstation performance tests.
7. Add a PWA install prompt and an update-available flow.
8. Evaluate per-field merge or a CRDT for high-contention clinical fields.
9. Conduct a formal security review and penetration test before live patient use.
10. Complete a data-protection impact assessment and facility retention schedule.

### 5.5.3 Conclusion

The project delivers an offline-capable clinical records MVP with a complete core patient journey, a working secure QR referral path, and a verifiable offline synchronisation model. Its strongest claim is not that it implements every item in the long-term roadmap, but that the implemented workflow is coherent, testable and honest about its boundaries. The QR referral feature is real and demonstrable; the remaining referral-security features are clearly identified as the next increment.

## 5.6 Glossary

| Term | Definition |
|---|---|
| Admission | An inpatient stay linked to a patient and encounter |
| Append-only | A data policy in which facts are inserted and not silently overwritten |
| Bed-day | One billable night of inpatient care |
| Charge | A billable line attached to an invoice |
| Clinical record | The patient, encounter, vitals, diagnosis, order, result, prescription and ward data |
| Conflict | A server-side record of a losing write payload that must be reviewed |
| Device key | A cryptographic key pair that identifies a workstation to the sync service |
| Discharge gate | A rule that prevents discharge while a ward balance is outstanding |
| Encounter | One visit to the facility |
| Facility code | The prefix used in document numbers and receipts |
| Household | A family grouping of patients |
| Idempotency key | A value that makes a retried operation produce the same result once |
| Invoice | A bill header that groups charge lines |
| Last-write-wins | A conflict policy in which the latest accepted row update wins |
| Offline grace | The period for which a previously verified device may sign in without the server |
| Outbox | The local queue of writes waiting to be pushed |
| Patient identifier | The internal UUID; never placed in a referral QR token |
| Referral | A clinical note sent to another facility for continuing care |
| Referral token | The encrypted, URL-safe payload encoded in the referral QR code |
| Role | A named authorisation profile such as doctor or cashier |
| Soft delete | Marking a row deleted with a timestamp while retaining it for audit and sync |
| Sync cursor | The monotonic position after which a device requests changes |
| Sync snapshot | A table-by-table bootstrap for a device that has no usable cursor |
| Vitals | Observations such as blood pressure, temperature, pulse and oxygen saturation |
| Ward | The inpatient area to which a patient is admitted |
