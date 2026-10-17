-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Sprint 8: the referral note.
--
--  This is the first data that LEAVES the building. The token carried by the QR
--  code is AES-GCM encrypted with the facility key and contains a clinical
--  summary — so it is unreadable without the key, and tamper-evident because GCM
--  authenticates as well as encrypts.
--
--  The row here is the FACILITY's record of what it issued: it exists so a
--  referral can be revoked before its 48 hours are up, and so the clinic knows
--  what it sent, to whom, and why.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE referrals (
    id                UUID PRIMARY KEY,
    referral_number   TEXT NOT NULL,
    patient_id        UUID NOT NULL REFERENCES patients(id),
    encounter_id      UUID NOT NULL REFERENCES encounters(id),

    -- Denormalised so a referral stays readable if the other tables move on.
    patient_name      TEXT NOT NULL,
    patient_age_years SMALLINT NULL,
    patient_sex       TEXT NULL,

    from_facility     TEXT NOT NULL,
    to_facility       TEXT NOT NULL,
    reason            TEXT NOT NULL,
    diagnosis_summary TEXT NULL,
    treatment_given   TEXT NULL,
    urgency           TEXT NOT NULL DEFAULT 'routine',

    status            TEXT NOT NULL DEFAULT 'issued',
    issued_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL,
    issued_by         UUID NOT NULL REFERENCES users(id),
    revoked_at        TIMESTAMPTZ NULL,
    revoked_by        UUID NULL REFERENCES users(id),
    revoke_reason     TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_referrals_status CHECK (status IN ('issued', 'viewed', 'revoked', 'expired')),
    CONSTRAINT chk_referrals_urgency CHECK (urgency IN ('routine', 'urgent', 'emergency'))
);

CREATE UNIQUE INDEX uq_referrals_number ON referrals (referral_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_referrals_patient ON referrals (patient_id, issued_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_referrals_server_rev ON referrals (server_rev);

INSERT INTO sync_table_scopes (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('referrals', '{doctor,nurse,admin}', '{doctor}', 'lww',
     'The issued note. The TOKEN is never stored.')
ON CONFLICT (table_name) DO NOTHING;

DROP TRIGGER IF EXISTS trg_sync_log ON referrals;
CREATE TRIGGER trg_sync_log AFTER INSERT OR UPDATE OR DELETE ON referrals
    FOR EACH ROW EXECUTE FUNCTION sync_log_change();

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_sync_log ON referrals;
DELETE FROM sync_table_scopes WHERE table_name = 'referrals';
DROP TABLE IF EXISTS referrals;
-- +goose StatementEnd
