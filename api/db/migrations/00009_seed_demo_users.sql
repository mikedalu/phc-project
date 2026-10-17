-- +goose Up
-- +goose StatementBegin

-- Demo accounts: one per role, so every role-gated screen and every role × route
-- assertion can be exercised without hand-writing SQL.
--
-- ═══ WHY THESE EXIST ═══
--
-- A system whose RBAC has only ever been tested with one account has not been
-- tested. The nurse must not see staff administration; the cashier must not see
-- the triage board. Proving that requires logging in as each of them, which
-- requires eight accounts that share a known password.
--
-- The password is the same Argon2id hash as the bootstrap admin
-- (`ChangeMe!2026`) and `must_change_password` is true for all of them, so a
-- fresh install cannot be left running on a published credential.
--
-- ═══ THE UUIDs ARE DELIBERATELY STABLE ═══
--
-- Fixed ids make the Down migration exact and make test fixtures able to name a
-- user without a lookup. They are in the v7 shape but hand-assigned: this is seed
-- data, not a client-generated key.
--
-- Production deployments should delete these rows. The README says so, and the
-- `Down` migration is how.
INSERT INTO users (
    id, staff_number, first_name, last_name, email,
    password_hash, must_change_password, is_active,
    created_by, updated_by
) VALUES
    ('018f0000-0000-7000-8000-0000000000b1', 'DEMO/STF/0002', 'Funke',  'Adeyemi',  'frontdesk@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000'),
    ('018f0000-0000-7000-8000-0000000000b2', 'DEMO/STF/0003', 'Ngozi',  'Okafor',   'nurse@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000'),
    ('018f0000-0000-7000-8000-0000000000b3', 'DEMO/STF/0004', 'Emeka',  'Nwosu',    'doctor@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000'),
    ('018f0000-0000-7000-8000-0000000000b4', 'DEMO/STF/0005', 'Halima', 'Bello',    'lab@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000'),
    ('018f0000-0000-7000-8000-0000000000b5', 'DEMO/STF/0006', 'Tunde',  'Balogun',  'pharmacy@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000'),
    ('018f0000-0000-7000-8000-0000000000b6', 'DEMO/STF/0007', 'Aisha',  'Yusuf',    'cashier@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000'),
    ('018f0000-0000-7000-8000-0000000000b7', 'DEMO/STF/0008', 'Chidi',  'Eze',      'accounts@demo.phc',
     '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo', true, true,
     '00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000')
ON CONFLICT (id) DO NOTHING;

-- One role each, granted by the bootstrap administrator.
INSERT INTO user_roles (id, user_id, role, granted_at, granted_by, created_by, updated_by)
SELECT v.role_id, v.user_id, v.role, now(), v.granted_by, v.granted_by, v.granted_by
FROM (VALUES
    ('018f0000-0000-7000-8000-0000000000c1'::uuid, '018f0000-0000-7000-8000-0000000000b1'::uuid, 'front_desk',    '018f0000-0000-7000-8000-0000000000aa'::uuid),
    ('018f0000-0000-7000-8000-0000000000c2'::uuid, '018f0000-0000-7000-8000-0000000000b2'::uuid, 'nurse',         '018f0000-0000-7000-8000-0000000000aa'::uuid),
    ('018f0000-0000-7000-8000-0000000000c3'::uuid, '018f0000-0000-7000-8000-0000000000b3'::uuid, 'doctor',        '018f0000-0000-7000-8000-0000000000aa'::uuid),
    ('018f0000-0000-7000-8000-0000000000c4'::uuid, '018f0000-0000-7000-8000-0000000000b4'::uuid, 'lab_scientist', '018f0000-0000-7000-8000-0000000000aa'::uuid),
    ('018f0000-0000-7000-8000-0000000000c5'::uuid, '018f0000-0000-7000-8000-0000000000b5'::uuid, 'pharmacist',    '018f0000-0000-7000-8000-0000000000aa'::uuid),
    ('018f0000-0000-7000-8000-0000000000c6'::uuid, '018f0000-0000-7000-8000-0000000000b6'::uuid, 'cashier',       '018f0000-0000-7000-8000-0000000000aa'::uuid),
    ('018f0000-0000-7000-8000-0000000000c7'::uuid, '018f0000-0000-7000-8000-0000000000b7'::uuid, 'accountant',    '018f0000-0000-7000-8000-0000000000aa'::uuid)
) AS v(role_id, user_id, role, granted_by)
WHERE NOT EXISTS (
    SELECT 1 FROM user_roles ur
    WHERE ur.user_id = v.user_id
      AND ur.role = v.role
      AND ur.revoked_at IS NULL
      AND ur.deleted_at IS NULL
);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Roles first: user_roles.user_id references users(id).
DELETE FROM user_roles WHERE id IN (
    '018f0000-0000-7000-8000-0000000000c1',
    '018f0000-0000-7000-8000-0000000000c2',
    '018f0000-0000-7000-8000-0000000000c3',
    '018f0000-0000-7000-8000-0000000000c4',
    '018f0000-0000-7000-8000-0000000000c5',
    '018f0000-0000-7000-8000-0000000000c6',
    '018f0000-0000-7000-8000-0000000000c7'
);

DELETE FROM users WHERE id IN (
    '018f0000-0000-7000-8000-0000000000b1',
    '018f0000-0000-7000-8000-0000000000b2',
    '018f0000-0000-7000-8000-0000000000b3',
    '018f0000-0000-7000-8000-0000000000b4',
    '018f0000-0000-7000-8000-0000000000b5',
    '018f0000-0000-7000-8000-0000000000b6',
    '018f0000-0000-7000-8000-0000000000b7'
);

-- +goose StatementEnd
