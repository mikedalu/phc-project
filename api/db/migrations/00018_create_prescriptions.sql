-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Sprint 7: the prescription, and the drugs that leave the shelf.
--
--  A prescription is a REQUEST; a dispense is the FACT that stock moved. They
--  are separate rows because the gap between them is the pharmacy's queue: a
--  patient can be prescribed something the shelf does not have.
--
--  Pricing follows the lab-order pattern: the item snapshots the price at
--  prescribing time, and the bill is raised in the same transaction.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE prescriptions (
    id                  UUID PRIMARY KEY,
    prescription_number TEXT NOT NULL,
    patient_id          UUID NOT NULL REFERENCES patients(id),
    encounter_id        UUID NOT NULL REFERENCES encounters(id),
    prescription_type   TEXT NOT NULL DEFAULT 'opd',
    status              TEXT NOT NULL DEFAULT 'pending_payment',
    prescribed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    prescribed_by       UUID NOT NULL REFERENCES users(id),
    invoice_id          UUID NULL REFERENCES invoices(id),
    notes               TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_prescriptions_type CHECK (prescription_type IN
        ('opd', 'inpatient', 'discharge', 'refill')),
    CONSTRAINT chk_prescriptions_status CHECK (status IN
        ('pending_payment', 'paid', 'partially_dispensed', 'dispensed', 'cancelled'))
);

CREATE UNIQUE INDEX uq_prescriptions_number ON prescriptions (prescription_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_prescriptions_patient ON prescriptions (patient_id, prescribed_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_prescriptions_status ON prescriptions (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_prescriptions_server_rev ON prescriptions (server_rev);

CREATE TABLE prescription_items (
    id              UUID PRIMARY KEY,
    prescription_id UUID NOT NULL REFERENCES prescriptions(id),
    -- Snapshotted so the history stays readable if the catalogue is renamed.
    drug_name       TEXT NOT NULL,
    dose            TEXT NOT NULL,
    frequency       TEXT NOT NULL,
    duration_days   SMALLINT NULL,
    quantity_prescribed NUMERIC(10,2) NOT NULL,
    route           TEXT NULL,
    instructions    TEXT NULL,
    -- The priced line, created when the item is prescribed.
    charge_id       UUID NULL REFERENCES charges(id),
    unit_price_minor BIGINT NOT NULL DEFAULT 0,
    amount_minor     BIGINT NOT NULL DEFAULT 0,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_items_qty CHECK (quantity_prescribed > 0)
);

CREATE INDEX idx_prescription_items_prescription ON prescription_items (prescription_id);
CREATE INDEX idx_prescription_items_server_rev ON prescription_items (server_rev);

-- The fact that stock moved. APPEND-ONLY: a correction is a reversal, not an edit.
CREATE TABLE dispenses (
    id              UUID PRIMARY KEY,
    prescription_id UUID NOT NULL REFERENCES prescriptions(id),
    patient_id      UUID NOT NULL REFERENCES patients(id),
    dispensed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispensed_by    UUID NOT NULL REFERENCES users(id),
    notes           TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id)
);

CREATE INDEX idx_dispenses_prescription ON dispenses (prescription_id);
CREATE INDEX idx_dispenses_server_rev ON dispenses (server_rev);

-- ─── sync scopes ───────────────────────────────────────────────────
INSERT INTO sync_table_scopes (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('prescriptions', '{doctor,nurse,pharmacist,cashier,admin}', '{doctor}', 'lww',
     'Status advances after payment, so LWW on the header is right.'),
    ('prescription_items', '{doctor,nurse,pharmacist,cashier,admin}', '{doctor}', 'lww',
     'Price snapshotted at prescribing time.'),
    ('dispenses', '{doctor,nurse,pharmacist,cashier,admin}', '{pharmacist}', 'append_only',
     'Stock left the shelf. A correction is a reversal, never an edit.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM sync_table_scopes WHERE table_name IN ('dispenses', 'prescription_items', 'prescriptions');
DROP TABLE IF EXISTS dispenses;
DROP TABLE IF EXISTS prescription_items;
DROP TABLE IF EXISTS prescriptions;
-- +goose StatementEnd
