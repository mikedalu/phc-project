-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  The patient registry: households and patients.
--
--  These are the first CLINICAL tables. Everything before this was
--  authentication or plumbing; from here on, a bug loses a person's record.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE households (
    id            UUID PRIMARY KEY,
    family_number TEXT NOT NULL,
    family_name   TEXT NOT NULL,

    -- Circular with patients.household_id. NULLABLE on purpose: insert the
    -- household first (head unknown), then the head patient, then UPDATE.
    -- Making both NOT NULL creates an insert deadlock with no way out.
    -- The FK itself is added AFTER patients exists, below.
    head_patient_id UUID NULL,

    phone   TEXT NULL,
    address TEXT NULL,
    notes   TEXT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,

    -- ─── the nine sync columns ─────────────────────────────────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_households_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_households_name
        CHECK (length(btrim(family_name)) > 0)
);

-- Partial: a soft-deleted family must not block its number being reused by a
-- correction. Unique among live rows only.
CREATE UNIQUE INDEX uq_households_number ON households (family_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_households_server_rev ON households (server_rev);
CREATE INDEX idx_households_updated    ON households (updated_at);

CREATE TABLE patients (
    id             UUID PRIMARY KEY,
    patient_number TEXT NOT NULL,
    household_id   UUID NULL REFERENCES households(id),

    first_name  TEXT NOT NULL,
    middle_name TEXT NULL,
    surname     TEXT NOT NULL,

    -- Age and DOB are both stored because a PHC genuinely does not always know
    -- one of them. §6.2: the service derives whichever is missing and NEVER
    -- leaves both NULL — dosing needs an age.
    date_of_birth   DATE NULL,
    age_years       SMALLINT NULL,
    age_months      SMALLINT NULL,
    age_recorded_at DATE NULL,

    gender         TEXT NOT NULL,
    marital_status TEXT NULL,

    phone           TEXT NULL,
    alt_phone       TEXT NULL,
    tribe           TEXT NULL,
    occupation      TEXT NULL,
    address         TEXT NULL,
    address_landmark TEXT NULL,

    blood_group        TEXT NULL,
    genotype           TEXT NULL,
    allergies          TEXT NULL,
    chronic_conditions TEXT NULL,

    is_deceased   BOOLEAN NOT NULL DEFAULT false,
    deceased_at   DATE NULL,

    -- Captured explicitly: back-entry of a paper register is a real workflow,
    -- and registration_date is not the same thing as created_at.
    registration_date DATE NOT NULL,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    photo_document_id UUID NULL,

    -- ─── the nine sync columns ─────────────────────────────────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_patients_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_patients_gender
        CHECK (gender IN ('male', 'female', 'other', 'unknown')),
    CONSTRAINT chk_patients_marital
        CHECK (marital_status IS NULL OR marital_status IN
               ('single', 'married', 'divorced', 'widowed', 'separated', 'unknown')),
    CONSTRAINT chk_patients_age
        CHECK (age_years IS NULL OR (age_years >= 0 AND age_years <= 130)),
    CONSTRAINT chk_patients_names
        CHECK (length(btrim(first_name)) > 0 AND length(btrim(surname)) > 0)
);

-- The circle, closed once both tables exist.
ALTER TABLE households
    ADD CONSTRAINT fk_households_head_patient
    FOREIGN KEY (head_patient_id) REFERENCES patients(id);

CREATE UNIQUE INDEX uq_patients_number ON patients (patient_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_household ON patients (household_id) WHERE deleted_at IS NULL;
-- The index that makes Ctrl+K search fast. Search must query
-- `lower(surname) LIKE lower($1) || '%'` to hit it; a plain `=` cannot.
CREATE INDEX idx_patients_name ON patients (lower(surname), lower(first_name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_phone ON patients (phone) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_dob   ON patients (date_of_birth) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_server_rev ON patients (server_rev);
CREATE INDEX idx_patients_updated    ON patients (updated_at);

-- ─── sync scopes ───────────────────────────────────────────────────
-- Added with the tables, per D39: a scope row naming a table that does not
-- exist would make every pull attempt query it and 500.
INSERT INTO sync_table_scopes
    (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('households',
     '{front_desk,nurse,doctor,cashier,accountant,admin}',
     '{front_desk,admin}', 'lww',
     'Billing party. Read by cashier and accountant because balances are per household.'),
    ('patients', '{*}', '{front_desk,nurse,doctor,admin}', 'lww',
     'The registry. Every role can read it: patient lookup happens at every station.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM sync_table_scopes WHERE table_name IN ('patients', 'households');

-- Drop the circular FK first, then the tables in dependency order.
ALTER TABLE households DROP CONSTRAINT IF EXISTS fk_households_head_patient;
DROP TABLE IF EXISTS patients;
DROP TABLE IF EXISTS households;
-- +goose StatementEnd
