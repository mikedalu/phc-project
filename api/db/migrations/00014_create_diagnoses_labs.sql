-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Sprint 5: the consultation's OUTPUT.
--
--  A diagnosis is an opinion; a lab order is a request; a lab result is a
--  measurement. All three are APPEND-ONLY: a second diagnosis is a new row, a
--  corrected result is a new result. Nothing in this migration has an update
--  path in the service layer, and the sync applier refuses updates for all
--  three (see sync_appendonly.go).
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE diagnoses (
    id           UUID PRIMARY KEY,
    patient_id   UUID NOT NULL REFERENCES patients(id),
    encounter_id UUID NOT NULL REFERENCES encounters(id),

    diagnosis_type TEXT NOT NULL DEFAULT 'primary',
    -- Free text plus a searchable reference list. Deliberately NOT an FK to an
    -- ICD table: importing and licensing the full set is out of scope, and a
    -- code that cannot be found must not block a diagnosis.
    icd10_code   TEXT NULL,
    description  TEXT NOT NULL,
    clinical_notes TEXT NULL,

    diagnosed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    diagnosed_by UUID NOT NULL REFERENCES users(id),

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_diagnoses_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_diagnoses_type
        CHECK (diagnosis_type IN ('primary', 'secondary', 'differential', 'provisional', 'final')),
    CONSTRAINT chk_diagnoses_description
        CHECK (length(btrim(description)) > 0)
);

CREATE INDEX idx_diagnoses_patient ON diagnoses (patient_id, diagnosed_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_diagnoses_encounter ON diagnoses (encounter_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_diagnoses_server_rev ON diagnoses (server_rev);

CREATE TABLE lab_orders (
    id            UUID PRIMARY KEY,
    patient_id    UUID NOT NULL REFERENCES patients(id),
    encounter_id  UUID NOT NULL REFERENCES encounters(id),
    order_number  TEXT NOT NULL,

    test_code     TEXT NOT NULL,
    -- Snapshotted at order time: a tariff rename must not rewrite history.
    test_name     TEXT NOT NULL,
    specimen_type TEXT NULL,

    clinical_indication TEXT NULL,
    priority      TEXT NOT NULL DEFAULT 'routine',
    status        TEXT NOT NULL DEFAULT 'ordered',

    ordered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ordered_by    UUID NOT NULL REFERENCES users(id),
    collected_at  TIMESTAMPTZ NULL,
    collected_by  UUID NULL REFERENCES users(id),
    completed_at  TIMESTAMPTZ NULL,
    -- invoice_id and charge_id are added by the Sprint 6 migration, when those
    -- tables exist. Adding a NOT NULL FK to a table that does not yet exist is
    -- not possible; adding the columns later is a one-line ALTER.

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_lab_orders_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_lab_orders_status
        CHECK (status IN ('ordered', 'collected', 'resulted', 'cancelled')),
    CONSTRAINT chk_lab_orders_priority
        CHECK (priority IN ('routine', 'urgent', 'stat')),
    CONSTRAINT chk_lab_orders_specimen
        CHECK (specimen_type IS NULL OR specimen_type IN ('blood', 'urine', 'stool', 'sputum', 'swab', 'other'))
);

CREATE UNIQUE INDEX uq_lab_orders_number ON lab_orders (order_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_lab_orders_encounter ON lab_orders (encounter_id);
CREATE INDEX idx_lab_orders_status ON lab_orders (status, priority, ordered_at);
CREATE INDEX idx_lab_orders_server_rev ON lab_orders (server_rev);

CREATE TABLE lab_results (
    id            UUID PRIMARY KEY,
    lab_order_id  UUID NOT NULL REFERENCES lab_orders(id),
    -- Denormalised for the patient timeline; a result without its person is
    -- never a useful query.
    patient_id    UUID NOT NULL REFERENCES patients(id),

    parameter_name TEXT NOT NULL,
    value_text     TEXT NULL,
    value_numeric  NUMERIC(12,4) NULL,
    unit           TEXT NULL,
    reference_range TEXT NULL,
    abnormal_flag  TEXT NULL,

    result_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    result_by   UUID NOT NULL REFERENCES users(id),
    verified_at TIMESTAMPTZ NULL,
    verified_by UUID NULL REFERENCES users(id),
    remarks     TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_lab_results_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    -- A result with no value at all is not a result.
    CONSTRAINT chk_lab_results_value
        CHECK (value_text IS NOT NULL OR value_numeric IS NOT NULL),
    CONSTRAINT chk_lab_results_flag
        CHECK (abnormal_flag IS NULL OR abnormal_flag IN
               ('normal', 'low', 'high', 'critical_low', 'critical_high'))
);

CREATE INDEX idx_lab_results_order ON lab_results (lab_order_id);
CREATE INDEX idx_lab_results_patient ON lab_results (patient_id, result_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_lab_results_server_rev ON lab_results (server_rev);

-- ─── sync scopes ───────────────────────────────────────────────────
INSERT INTO sync_table_scopes
    (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('diagnoses', '{doctor,nurse,admin}', '{doctor}', 'append_only',
     'Clinical opinions. A second diagnosis is a new row, never an edit.'),
    ('lab_orders', '{doctor,lab_scientist,nurse,admin}', '{doctor}', 'lww',
     'Status advances (ordered -> collected -> resulted), so LWW on the row is right.'),
    ('lab_results', '{doctor,lab_scientist,nurse,admin}', '{lab_scientist}', 'append_only',
     'Measurements. A corrected value is a NEW result, never an overwrite.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM sync_table_scopes WHERE table_name IN ('lab_results', 'lab_orders', 'diagnoses');
DROP TABLE IF EXISTS lab_results;
DROP TABLE IF EXISTS lab_orders;
DROP TABLE IF EXISTS diagnoses;
-- +goose StatementEnd
