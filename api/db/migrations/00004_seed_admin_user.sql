-- +goose Up
-- +goose StatementBegin

-- The first administrator: the bootstrap identity for a fresh installation.
--
-- ⚠️  DEV / FIRST-INSTALL CREDENTIAL — READ THIS
--
--   Staff no:  DEMO/STF/0001
--   Email:     admin@demo.phc
--   Password:  ChangeMe!2026
--
--   The password is committed here on purpose so a fresh environment is usable.
--   must_change_password = true forces a change at first login.
--
--   BEFORE ANY PUBLIC DEPLOYMENT: change this password, or delete this user.
--   A committed credential is a known credential, and this one is in your git
--   history permanently. For a graded project that is acceptable *if you say so
--   in your report* — name it as a risk you identified and deliberately bounded.
--
-- The hash is real Argon2id, not a placeholder, and was round-trip verified to
-- accept the password above and reject an incorrect one:
--   $argon2id$v=19$m=65536,t=3,p=4$<16-byte salt>$<32-byte key>
-- It must match internal/platform/password.Verify exactly (Sprint 1).
--
-- There is no facility_id column: this deployment serves one health centre.
--
-- created_by / updated_by use the reserved system UUID, because at bootstrap
-- there is no other actor to name. Nothing else may use that UUID.
INSERT INTO users (
    id, staff_number, first_name, last_name, email,
    password_hash, must_change_password,
    created_by, updated_by
) VALUES (
    '018f0000-0000-7000-8000-0000000000aa',
    'DEMO/STF/0001',
    'System',
    'Administrator',
    'admin@demo.phc',
    '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo',
    true,
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
)
ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE id = '018f0000-0000-7000-8000-0000000000aa';
-- +goose StatementEnd
