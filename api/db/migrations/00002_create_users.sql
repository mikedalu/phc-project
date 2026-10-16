-- +goose Up
-- +goose StatementBegin

-- Case-insensitive text for email, so 'Grace@phc.ng' and 'grace@phc.ng'
-- cannot both be registered.
CREATE EXTENSION IF NOT EXISTS citext;

-- Staff accounts.
--
-- No facility_id: this deployment serves exactly one health centre, so every
-- user belongs to it implicitly. Staff numbers and emails are therefore globally
-- unique rather than unique-per-facility.
--
-- Roles are NOT a column here. A real PHC has staff who are both a nurse and a
-- midwife, so roles live in user_roles (a later migration) and a user may hold
-- several. See docs/01-DATA-MODEL.md §5.3.
CREATE TABLE users (
    id                   UUID PRIMARY KEY,
    staff_number         TEXT NOT NULL,
    first_name           TEXT NOT NULL,
    last_name            TEXT NOT NULL,
    middle_name          TEXT,
    email                CITEXT,
    phone                TEXT,

    -- Argon2id encoded string. This column is NEVER synced to a client: the
    -- sync layer strips it. See docs/02-SYNC-PROTOCOL.md §9.4.
    password_hash        TEXT NOT NULL,
    password_changed_at  TIMESTAMPTZ,
    must_change_password BOOLEAN NOT NULL DEFAULT true,

    is_active          BOOLEAN  NOT NULL DEFAULT true,
    last_login_at      TIMESTAMPTZ,
    failed_login_count SMALLINT NOT NULL DEFAULT 0,
    locked_until       TIMESTAMPTZ,

    -- ─── sync column contract ──────────────────────────────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL,
    updated_by  UUID        NOT NULL,

    CONSTRAINT chk_users_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_users_failed_logins
        CHECK (failed_login_count >= 0),
    CONSTRAINT chk_users_staff_number_not_blank
        CHECK (length(btrim(staff_number)) > 0)
);

CREATE UNIQUE INDEX uq_users_staff_number
    ON users (staff_number) WHERE deleted_at IS NULL;

-- Postgres treats NULLs as distinct in a unique index, so users with no email
-- do not collide with each other. That is the behaviour we want.
CREATE UNIQUE INDEX uq_users_email
    ON users (email) WHERE email IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_users_updated    ON users (updated_at);
CREATE INDEX idx_users_server_rev ON users (server_rev);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
