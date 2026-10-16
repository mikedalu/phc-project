-- +goose Up
-- +goose StatementBegin

-- ═══════════════════════════════════════════════════════════════════
--  THIS DEPLOYMENT IS SINGLE-TENANT.
--
--  One installation serves exactly ONE health centre: its own code, its own
--  database, its own API. A second facility that wants this system deploys its
--  own copy. There is therefore no tenant column anywhere in the schema, and
--  `facility_id` appears on no clinical table.
--
--  FACILITIES IS A CONFIGURATION ROW, NOT A TENANT.
--  It holds the values that must appear on printed output and in money
--  formatting: the hospital name and address for receipt headers, the code that
--  prefixes every document number, the currency, and the timezone. It is
--  seeded once at install and is effectively immutable thereafter.
--
--  ⚠️  `code` is IMMUTABLE once any document has been issued.
--      It prefixes patient numbers, invoices, receipts, visit numbers and
--      referral numbers. Changing it would not renumber documents already
--      printed on thermal paper, so you would end up with two competing number
--      series and a uniqueness collision. The service layer must refuse to
--      update it; this comment is the second line of defence.
-- ═══════════════════════════════════════════════════════════════════
CREATE TABLE facilities (
    id                   UUID PRIMARY KEY,
    name                 TEXT NOT NULL,
    code                 TEXT NOT NULL UNIQUE,
    lga                  TEXT,
    state                TEXT,
    address              TEXT,
    phone                TEXT,
    currency_code        CHAR(3)  NOT NULL DEFAULT 'NGN',
    currency_minor_units SMALLINT NOT NULL DEFAULT 2,
    timezone             TEXT     NOT NULL DEFAULT 'Africa/Lagos',
    is_active            BOOLEAN  NOT NULL DEFAULT true,

    -- ─── sync column contract (docs/01-DATA-MODEL.md §3) ───────
    -- Identical to every other syncable table, minus facility_id (there is no
    -- parent organisation) and with nullable actors (see the note below).
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- NULLABLE on purpose. This row is inserted by the migration at install
    -- time, when no user exists yet to name as the actor. A NOT NULL actor here
    -- would make the very first insert impossible. Every other syncable table
    -- keeps created_by/updated_by NOT NULL, because by the time those rows are
    -- written a real user is always logged in.
    created_by  UUID,
    updated_by  UUID,

    CONSTRAINT chk_facilities_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_facilities_currency_units
        CHECK (currency_minor_units BETWEEN 0 AND 4),
    CONSTRAINT chk_facilities_code_upper
        CHECK (code = upper(code)),
    -- Facility codes appear inside document numbers such as
    -- URPHC/PAT/2026/00123. Keeping them short, uppercase and alphanumeric is
    -- what stops a code containing a "/" from making those numbers ambiguous.
    CONSTRAINT chk_facilities_code_charset
        CHECK (code ~ '^[A-Z0-9]{2,10}$')
);

-- Pull performance: GET /sync/pull walks server_rev.
CREATE INDEX idx_facilities_server_rev ON facilities (server_rev);

-- ★ SINGLE-TENANT GUARANTEE, ENFORCED BY THE DATABASE.
--
-- This is the constraint that makes "single tenant" a fact rather than a
-- convention. Without it, nothing stops a second row appearing — by a buggy
-- admin screen, a stray seed, or a hand-run INSERT — and the application would
-- then have an ambiguous "which facility am I?" question on every request.
--
-- The expression is a constant, so the unique index permits at most one row
-- satisfying it. A second INSERT fails with SQLSTATE 23505.
CREATE UNIQUE INDEX uq_facilities_single_org ON facilities ((true));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS facilities;
-- +goose StatementEnd
