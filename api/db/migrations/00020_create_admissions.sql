-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Sprint 7: the ward.
--
--  A bed-day is the first thing in this system that a CLOCK can double-count.
--  Two devices, one night, and without a guard the patient is billed twice for
--  the same sleep.
--
--  The guard is a unique index on (admission_id, charge_date): a bed-day is
--  identified by the NIGHT, not by when the clerk typed it. Retry the sync, and
--  the second insert is a duplicate rather than a second charge.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE admissions (
    id           UUID PRIMARY KEY,
    patient_id   UUID NOT NULL REFERENCES patients(id),
    encounter_id UUID NOT NULL REFERENCES encounters(id),
    ward         TEXT NOT NULL,
    bed_number   TEXT NULL,
    status       TEXT NOT NULL DEFAULT 'admitted',
    admitted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    admitted_by  UUID NOT NULL REFERENCES users(id),
    discharged_at TIMESTAMPTZ NULL,
    discharged_by UUID NULL REFERENCES users(id),
    discharge_notes TEXT NULL,
    invoice_id   UUID NULL REFERENCES invoices(id),

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_admissions_status CHECK (status IN ('admitted', 'discharged', 'transferred'))
);

CREATE INDEX idx_admissions_patient ON admissions (patient_id, admitted_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_admissions_open ON admissions (patient_id) WHERE status = 'admitted' AND deleted_at IS NULL;
CREATE INDEX idx_admissions_server_rev ON admissions (server_rev);

CREATE TABLE ward_charges (
    id           UUID PRIMARY KEY,
    admission_id UUID NOT NULL REFERENCES admissions(id),
    patient_id   UUID NOT NULL REFERENCES patients(id),
    -- The NIGHT being billed. A DATE, not a timestamp: two clock readings a
    -- second apart belong to the same bed-day.
    charge_date  DATE NOT NULL,
    ward         TEXT NOT NULL,
    unit_price_minor BIGINT NOT NULL,
    amount_minor     BIGINT NOT NULL,
    invoice_id   UUID NULL REFERENCES invoices(id),
    charge_id    UUID NULL REFERENCES charges(id),

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_ward_amount CHECK (amount_minor >= 0)
);

-- THE guard. One billable night per admission, whatever the device count.
CREATE UNIQUE INDEX uq_ward_charges_night ON ward_charges (admission_id, charge_date);
CREATE INDEX idx_ward_charges_admission ON ward_charges (admission_id);
CREATE INDEX idx_ward_charges_server_rev ON ward_charges (server_rev);

-- ─── sync scopes ───────────────────────────────────────────────────
INSERT INTO sync_table_scopes (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('admissions', '{doctor,nurse,cashier,admin}', '{doctor,nurse}', 'lww',
     'Status advances on discharge, so LWW on the header is right.'),
    ('ward_charges', '{doctor,nurse,cashier,accountant,admin}', '{nurse,cashier,admin}', 'append_only',
     'A night billed. The unique index on (admission_id, charge_date) is the double-billing guard.')
ON CONFLICT (table_name) DO NOTHING;

-- The triggers from 00019 were installed before these tables existed; attach
-- them here so ward writes reach the change log like everything else.
DROP TRIGGER IF EXISTS trg_sync_log ON admissions;
CREATE TRIGGER trg_sync_log AFTER INSERT OR UPDATE OR DELETE ON admissions
    FOR EACH ROW EXECUTE FUNCTION sync_log_change();
DROP TRIGGER IF EXISTS trg_sync_log ON ward_charges;
CREATE TRIGGER trg_sync_log AFTER INSERT OR UPDATE OR DELETE ON ward_charges
    FOR EACH ROW EXECUTE FUNCTION sync_log_change();

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_sync_log ON ward_charges;
DROP TRIGGER IF EXISTS trg_sync_log ON admissions;
DELETE FROM sync_table_scopes WHERE table_name IN ('ward_charges', 'admissions');
DROP TABLE IF EXISTS ward_charges;
DROP TABLE IF EXISTS admissions;
-- +goose StatementEnd
