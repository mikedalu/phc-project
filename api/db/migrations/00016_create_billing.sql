-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Sprint 6: money.
--
--  Three rules decide the shape of these tables:
--
--  1. Money is BIGINT MINOR UNITS. Never float, never numeric. 0.1 + 0.2 is
--     not 0.3, and a receipt that is off by a kobo is a receipt the cashier
--     cannot reconcile.
--  2. A payment carries a client-generated operation_id with a UNIQUE
--     constraint. That is the ONLY thing standing between a retried sync batch
--     and a patient being charged twice.
--  3. A BALANCE IS NEVER STORED. It is derived from charges minus allocations,
--     always. A stored balance is correct until the first concurrent write and
--     silently wrong for ever after.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE tariffs (
    id               UUID PRIMARY KEY,
    code             TEXT NOT NULL,
    name             TEXT NOT NULL,
    category         TEXT NOT NULL,
    unit_price_minor BIGINT NOT NULL,
    unit             TEXT NULL,
    is_active        BOOLEAN NOT NULL DEFAULT true,
    effective_from   DATE NOT NULL DEFAULT CURRENT_DATE,
    effective_to     DATE NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT uq_tariffs_code UNIQUE (code),
    CONSTRAINT chk_tariffs_price CHECK (unit_price_minor >= 0),
    CONSTRAINT chk_tariffs_category CHECK (category IN
        ('consultation', 'lab', 'drug', 'procedure', 'ward', 'registration', 'other'))
);

CREATE TABLE invoices (
    id             UUID PRIMARY KEY,
    invoice_number TEXT NOT NULL,
    patient_id     UUID NOT NULL REFERENCES patients(id),
    -- WHO PAYS. Defaults from the patient, may be moved to the family.
    household_id   UUID NULL REFERENCES households(id),
    encounter_id   UUID NULL REFERENCES encounters(id),
    invoice_type   TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'unpaid',
    -- The ONE permitted denormalisation: written in the same transaction as the
    -- charges it sums, and unaffected by payments. Do NOT add balance_minor.
    total_minor    BIGINT NOT NULL DEFAULT 0,
    issued_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    issued_by      UUID NOT NULL REFERENCES users(id),
    voided_at      TIMESTAMPTZ NULL,
    voided_by      UUID NULL REFERENCES users(id),
    void_reason    TEXT NULL,
    notes          TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_invoices_type CHECK (invoice_type IN
        ('consultation', 'lab', 'drug', 'ward', 'procedure', 'mixed')),
    CONSTRAINT chk_invoices_status CHECK (status IN
        ('unpaid', 'partially_paid', 'paid', 'void')),
    CONSTRAINT chk_invoices_total CHECK (total_minor >= 0)
);

CREATE UNIQUE INDEX uq_invoices_number ON invoices (invoice_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_patient ON invoices (patient_id, issued_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_household ON invoices (household_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_unpaid ON invoices (issued_at) WHERE status IN ('unpaid', 'partially_paid');
CREATE INDEX idx_invoices_server_rev ON invoices (server_rev);

CREATE TABLE charges (
    id               UUID PRIMARY KEY,
    invoice_id       UUID NOT NULL REFERENCES invoices(id),
    patient_id       UUID NOT NULL REFERENCES patients(id),
    household_id     UUID NULL REFERENCES households(id),
    charge_type      TEXT NOT NULL,
    tariff_id        UUID NULL REFERENCES tariffs(id),
    description      TEXT NOT NULL,
    reference_id     UUID NULL,
    reference_type   TEXT NULL,
    quantity         NUMERIC(10,2) NOT NULL DEFAULT 1,
    -- SNAPSHOTTED: a later price change must not rewrite an issued invoice.
    unit_price_minor BIGINT NOT NULL,
    amount_minor     BIGINT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    posted_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    voided_at        TIMESTAMPTZ NULL,
    voided_by        UUID NULL REFERENCES users(id),
    void_reason      TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_charges_amount CHECK (amount_minor >= 0),
    CONSTRAINT chk_charges_qty CHECK (quantity > 0),
    CONSTRAINT chk_charges_status CHECK (status IN ('active', 'void')),
    CONSTRAINT chk_charges_type CHECK (charge_type IN
        ('consultation', 'lab', 'drug', 'ward', 'procedure', 'registration', 'other'))
);

CREATE INDEX idx_charges_invoice ON charges (invoice_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_charges_patient ON charges (patient_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_charges_household ON charges (household_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_charges_reference ON charges (reference_type, reference_id);
CREATE INDEX idx_charges_server_rev ON charges (server_rev);

CREATE TABLE payments (
    id               UUID PRIMARY KEY,
    -- The idempotency key. Client-generated, never reused.
    operation_id     UUID NOT NULL,
    receipt_number   TEXT NOT NULL,
    payer_patient_id UUID NULL REFERENCES patients(id),
    household_id     UUID NULL REFERENCES households(id),
    amount_minor     BIGINT NOT NULL,
    currency_code    CHAR(3) NOT NULL DEFAULT 'NGN',
    payment_method   TEXT NOT NULL DEFAULT 'cash',
    payment_reference TEXT NULL,
    status           TEXT NOT NULL DEFAULT 'posted',
    paid_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    received_by      UUID NOT NULL REFERENCES users(id),
    reversal_of_payment_id  UUID NULL REFERENCES payments(id),
    reversed_by_payment_id  UUID NULL REFERENCES payments(id),
    reversal_reason  TEXT NULL,
    notes            TEXT NULL,

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    -- THE constraint. A retried sync batch hits this and is reported as a
    -- duplicate instead of charging the patient twice.
    CONSTRAINT uq_payments_operation UNIQUE (operation_id),
    CONSTRAINT uq_payments_receipt UNIQUE (receipt_number),
    CONSTRAINT chk_payments_amount CHECK (amount_minor > 0),
    CONSTRAINT chk_payments_method CHECK (payment_method IN
        ('cash', 'transfer', 'pos_card', 'waiver')),
    CONSTRAINT chk_payments_status CHECK (status IN ('posted', 'reversed'))
);

CREATE INDEX idx_payments_household ON payments (household_id, paid_at DESC);
CREATE INDEX idx_payments_payer ON payments (payer_patient_id, paid_at DESC);
CREATE INDEX idx_payments_server_rev ON payments (server_rev);

CREATE TABLE payment_allocations (
    id           UUID PRIMARY KEY,
    payment_id   UUID NOT NULL REFERENCES payments(id),
    charge_id    UUID NOT NULL REFERENCES charges(id),
    amount_minor BIGINT NOT NULL,
    allocated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    allocated_by UUID NOT NULL REFERENCES users(id),

    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT uq_allocations_payment_charge UNIQUE (payment_id, charge_id),
    CONSTRAINT chk_allocations_amount CHECK (amount_minor > 0)
);

CREATE INDEX idx_allocations_charge ON payment_allocations (charge_id);
CREATE INDEX idx_allocations_server_rev ON payment_allocations (server_rev);

-- A lab order now knows its bill. Added here rather than in 00014 because the
-- tables it references did not exist then.
ALTER TABLE lab_orders
    ADD COLUMN invoice_id UUID NULL REFERENCES invoices(id),
    ADD COLUMN charge_id  UUID NULL REFERENCES charges(id);

-- ─── sync scopes ───────────────────────────────────────────────────
INSERT INTO sync_table_scopes (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('tariffs', '{*}', '{admin}', 'lww', 'Prices. Admin-only writes; everyone reads.'),
    ('invoices', '{*}', '{front_desk,cashier,doctor,admin}', 'lww', 'The bill.'),
    ('charges', '{*}', '{front_desk,cashier,doctor,admin}', 'lww', 'Bill lines, price snapshotted.'),
    ('payments', '{front_desk,cashier,accountant,admin}', '{cashier,admin}', 'append_only',
     'Money received. A reversal is a NEW row, never an edit.'),
    ('payment_allocations', '{front_desk,cashier,accountant,admin}', '{cashier,admin}', 'append_only',
     'Which charge a payment settled.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM sync_table_scopes WHERE table_name IN
    ('payment_allocations', 'payments', 'charges', 'invoices', 'tariffs');
ALTER TABLE lab_orders DROP COLUMN IF EXISTS charge_id, DROP COLUMN IF EXISTS invoice_id;
DROP TABLE IF EXISTS payment_allocations;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS charges;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS tariffs;
-- +goose StatementEnd
