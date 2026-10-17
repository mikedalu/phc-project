-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  The clinical workflow: encounters and vitals.
--
--  An encounter IS the queue entry (§2.4). There is no separate queue
--  table, deliberately: two rows for one visit invite them to disagree,
--  and "move the patient forward" becomes two writes that can half-fail.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE encounters (
    id           UUID PRIMARY KEY,
    visit_number TEXT NOT NULL,
    patient_id   UUID NOT NULL REFERENCES patients(id),

    -- Snapshotted from the patient at creation: the billing party for THIS
    -- visit. A later change of household must not rewrite history.
    household_id UUID NULL REFERENCES households(id),

    encounter_type TEXT     NOT NULL DEFAULT 'opd',
    workflow_stage TEXT     NOT NULL DEFAULT 'registered',
    status         TEXT     NOT NULL DEFAULT 'active',
    priority       SMALLINT NOT NULL DEFAULT 0,

    chief_complaint TEXT NULL,
    queued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ NULL,
    closed_at       TIMESTAMPTZ NULL,
    closed_by       UUID NULL REFERENCES users(id),

    attending_doctor_id       UUID NULL REFERENCES users(id),
    attending_nurse_id        UUID NULL REFERENCES users(id),
    referred_from_encounter_id UUID NULL REFERENCES encounters(id),
    notes TEXT NULL,

    -- ─── the nine sync columns ─────────────────────────────────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_encounters_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_encounters_stage CHECK (workflow_stage IN (
        'registered','vitals_done','awaiting_doctor','in_consultation',
        'awaiting_lab','lab_done','awaiting_lab_payment',
        'awaiting_pharmacy','awaiting_drug_payment','pharmacy_done',
        'admitted','closed')),
    CONSTRAINT chk_encounters_status
        CHECK (status IN ('active','closed','cancelled')),
    CONSTRAINT chk_encounters_type
        CHECK (encounter_type IN ('opd','anc','immunisation','emergency','follow_up','inpatient')),
    CONSTRAINT chk_encounters_priority CHECK (priority BETWEEN 0 AND 2)
);

CREATE UNIQUE INDEX uq_encounters_visit ON encounters (visit_number) WHERE deleted_at IS NULL;

-- The queue board query: active encounters by stage, emergencies first.
CREATE INDEX idx_encounters_queue ON encounters (workflow_stage, priority DESC, queued_at)
    WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX idx_encounters_patient ON encounters (patient_id, queued_at DESC);
-- Deliberately NOT unique: a concurrent double-registration on two offline
-- devices would then fail at sync. The service enforces one active encounter
-- today; a unique index is a Sprint 9 hardening step once real conflicts have
-- been observed (01-DATA-MODEL.md §7.1).
CREATE INDEX idx_encounters_open ON encounters (patient_id)
    WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX idx_encounters_server_rev ON encounters (server_rev);

CREATE TABLE vitals (
    id           UUID PRIMARY KEY,
    patient_id   UUID NOT NULL REFERENCES patients(id),
    encounter_id UUID NULL REFERENCES encounters(id),
    -- admission_id arrives with Sprint 7; NULL means "not a ward observation".
    admission_id UUID NULL,

    -- The CLINICAL timestamp, distinct from created_at. A reading taken at
    -- 08:40 and synced at 14:00 is an 08:40 observation.
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    bp_systolic      SMALLINT NULL,
    bp_diastolic     SMALLINT NULL,
    temperature_c    NUMERIC(4,1) NULL,
    pulse_rate       SMALLINT NULL,
    respiratory_rate SMALLINT NULL,
    height_cm        NUMERIC(5,1) NULL,
    weight_kg        NUMERIC(5,2) NULL,
    spo2             SMALLINT NULL,
    bmi              NUMERIC(4,1) NULL,
    pain_score       SMALLINT NULL,
    notes            TEXT NULL,

    recorded_by UUID NOT NULL REFERENCES users(id),

    -- ─── the nine sync columns ─────────────────────────────────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_vitals_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_vitals_systolic
        CHECK (bp_systolic IS NULL OR bp_systolic BETWEEN 40 AND 300),
    CONSTRAINT chk_vitals_diastolic
        CHECK (bp_diastolic IS NULL OR bp_diastolic BETWEEN 20 AND 200),
    CONSTRAINT chk_vitals_temp
        CHECK (temperature_c IS NULL OR temperature_c BETWEEN 25.0 AND 45.0),
    CONSTRAINT chk_vitals_pulse
        CHECK (pulse_rate IS NULL OR pulse_rate BETWEEN 20 AND 300),
    CONSTRAINT chk_vitals_resp
        CHECK (respiratory_rate IS NULL OR respiratory_rate BETWEEN 5 AND 80),
    CONSTRAINT chk_vitals_height
        CHECK (height_cm IS NULL OR height_cm BETWEEN 20 AND 260),
    CONSTRAINT chk_vitals_weight
        CHECK (weight_kg IS NULL OR weight_kg BETWEEN 0.5 AND 400),
    CONSTRAINT chk_vitals_pain CHECK (pain_score IS NULL OR pain_score BETWEEN 0 AND 10)
);

CREATE INDEX idx_vitals_patient_recorded ON vitals (patient_id, recorded_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_vitals_encounter ON vitals (encounter_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_vitals_server_rev ON vitals (server_rev);

-- ─── sync scopes ───────────────────────────────────────────────────
INSERT INTO sync_table_scopes
    (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('encounters', '{*}', '{front_desk,nurse,doctor}', 'lww',
     'The queue lives here. Every station needs to see it.'),
    ('vitals', '{nurse,doctor,admin}', '{nurse,doctor}', 'append_only',
     'Immutable facts. A second reading is a NEW row, never an edit.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM sync_table_scopes WHERE table_name IN ('vitals', 'encounters');
DROP TABLE IF EXISTS vitals;
DROP TABLE IF EXISTS encounters;
-- +goose StatementEnd
