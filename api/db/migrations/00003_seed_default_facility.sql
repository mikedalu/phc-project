-- +goose Up
-- +goose StatementBegin

-- Seed the ONE configuration row for this installation.
--
-- Idempotent: safe to run repeatedly on a fresh or existing environment.
--
-- created_by / updated_by are NULL because this row is inserted at install time,
-- before any user exists to name as the actor. They are filled in later, by the
-- admin who first edits the facility settings through the application.
--
-- ⚠️  Edit `name`, `address`, `phone`, `lga`, `state` and `currency_code` to
--     match the real health centre before going live. They appear on every
--     printed receipt and referral note.
--
-- ⚠️  `code` MUST also be set before any document is issued, and must never
--     change afterwards — it prefixes every patient number, invoice, receipt
--     and referral number. See the note in 00001. Must match ^[A-Z0-9]{2,10}$.
INSERT INTO facilities (
    id, name, code, lga, state, address, phone,
    currency_code, currency_minor_units, timezone
) VALUES (
    '018f0000-0000-7000-8000-000000000001',
    'Demo Primary Health Centre',
    'DEMO',
    'Demo LGA',
    'Demo State',
    '1 Health Centre Road',
    '08000000000',
    'NGN', 2, 'Africa/Lagos'
)
ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM facilities WHERE id = '018f0000-0000-7000-8000-000000000001';
-- +goose StatementEnd
