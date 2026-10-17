-- +goose Up
-- +goose StatementBegin

-- Role assignments.
--
-- Roles are a separate table rather than a `users.role` column because a real
-- PHC has staff who are both a nurse and a midwife, or a doctor who also
-- administers. See docs/01-DATA-MODEL.md §5.3.
--
-- A role is REVOKED, never deleted: `revoked_at` preserves the audit trail, and
-- the partial unique index below keeps that honest.
CREATE TABLE user_roles (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,

    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by UUID NOT NULL REFERENCES users(id),
    revoked_at TIMESTAMPTZ,
    revoked_by UUID REFERENCES users(id),
    -- Why the role was withdrawn. Required alongside revoked_at: revoking
    -- someone's clinical access without recording a reason is an audit hole,
    -- and "why did Dr X lose prescribing rights?" is a question that WILL be
    -- asked.
    revoked_reason TEXT,

    -- ─── sync column contract (docs/01-DATA-MODEL.md §3) ───────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL,
    updated_by  UUID        NOT NULL,

    CONSTRAINT chk_user_roles_role CHECK (role IN (
        'front_desk', 'nurse', 'doctor', 'lab_scientist',
        'pharmacist', 'cashier', 'accountant', 'admin'
    )),
    CONSTRAINT chk_user_roles_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    -- A revocation without attribution is an untraceable change.
    CONSTRAINT chk_user_roles_revoked_by
        CHECK ((revoked_at IS NULL) = (revoked_by IS NULL)),
    -- ...and a revocation without a recorded reason is not much better.
    CONSTRAINT chk_user_roles_reason
        CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL)),
    CONSTRAINT chk_user_roles_reason_len
        CHECK (revoked_reason IS NULL OR length(btrim(revoked_reason)) >= 5)
);

-- A user may hold each role at most once, among ACTIVE assignments only.
-- Partial on revoked_at so that re-granting a role after a revoke is legal:
-- the old revoked row stays for history, the new one takes the unique slot.
CREATE UNIQUE INDEX uq_user_roles_active
    ON user_roles (user_id, role)
    WHERE revoked_at IS NULL AND deleted_at IS NULL;

CREATE INDEX idx_user_roles_user       ON user_roles (user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_user_roles_server_rev ON user_roles (server_rev);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_roles;
-- +goose StatementEnd
