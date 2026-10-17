-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Sync infrastructure — SERVER-ONLY
--
--  The machinery behind POST /sync/push and GET /sync/pull. None of these
--  tables is ever replicated to a device; they are the SERVER's record of
--  what has been applied and what has changed. See
--  docs/01-DATA-MODEL.md §9 and docs/02-SYNC-PROTOCOL.md §5–§7.
--
--  None of them carries the nine sync columns. That is not an oversight:
--  those columns exist so a row can be replicated, and these rows must not
--  be. A replicated change log would be a second source of truth.
-- ════════════════════════════════════════════════════════════════════

-- ─── 1. Which tables sync, and to whom ─────────────────────────────
--
-- The single source of truth the sync engine enumerates. Adding a syncable
-- table means INSERTing a row here; no engine code changes.
--
-- pull_roles / push_roles hold ROLE NAMES, with '{*}' meaning every role.
-- Storing roles as an array rather than a join table keeps the hot path to
-- one lookup, and the list is tiny and rarely edited.
CREATE TABLE sync_table_scopes (
    table_name      TEXT PRIMARY KEY,
    sync_enabled    BOOLEAN NOT NULL DEFAULT true,
    pull_roles      TEXT[]  NOT NULL,
    push_roles      TEXT[]  NOT NULL,
    -- append_only: rows are facts, never edited (vitals, payments).
    -- lww:          last write wins, and the overwrite is RECORDED.
    conflict_policy TEXT    NOT NULL,
    retention_days  INTEGER NULL,
    notes           TEXT    NULL,

    CONSTRAINT chk_sync_scope_policy
        CHECK (conflict_policy IN ('append_only', 'lww')),
    CONSTRAINT chk_sync_scope_pull_roles
        CHECK (cardinality(pull_roles) > 0),
    CONSTRAINT chk_sync_scope_push_roles
        CHECK (cardinality(push_roles) > 0),
    CONSTRAINT chk_sync_scope_retention
        CHECK (retention_days IS NULL OR retention_days > 0)
);

-- ─── 2. Applied batches ────────────────────────────────────────────
--
-- id = the client's batch_id, so a retried batch hits the primary key and is
-- answered from the stored result rather than being applied twice. The
-- operation-level guarantee lives in sync_operations; this is the coarse one.
CREATE TABLE sync_batches (
    id                UUID PRIMARY KEY,
    device_id         UUID NOT NULL REFERENCES devices(id),
    user_id           UUID NOT NULL REFERENCES users(id),
    operation_count   INTEGER NOT NULL,
    accepted_count    INTEGER NOT NULL,
    rejected_count    INTEGER NOT NULL,
    received_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    client_sent_at    TIMESTAMPTZ NULL,
    highest_server_rev BIGINT NOT NULL,

    CONSTRAINT chk_sync_batch_counts CHECK (
        operation_count >= 0 AND accepted_count >= 0 AND rejected_count >= 0
        AND accepted_count + rejected_count <= operation_count
    )
);

CREATE INDEX idx_sync_batches_device ON sync_batches (device_id, received_at DESC);
CREATE INDEX idx_sync_batches_user   ON sync_batches (user_id, received_at DESC);

-- ─── 3. Per-operation results ──────────────────────────────────────
--
-- The real idempotency guarantee. A batch can be partially retried, and a
-- single operation replayed inside a NEW batch is still recognised here by
-- its operation_id.
CREATE TABLE sync_operations (
    id          UUID PRIMARY KEY,
    batch_id    UUID NOT NULL REFERENCES sync_batches(id) ON DELETE CASCADE,
    table_name  TEXT NOT NULL,
    row_id      UUID NOT NULL,
    operation   TEXT NOT NULL,
    result      TEXT NOT NULL,
    reason      TEXT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    server_rev  BIGINT NULL,

    CONSTRAINT chk_sync_op_operation
        CHECK (operation IN ('insert', 'update', 'delete')),
    CONSTRAINT chk_sync_op_result
        CHECK (result IN ('applied', 'duplicate', 'conflict_lww', 'rejected'))
);

CREATE INDEX idx_sync_operations_row    ON sync_operations (table_name, row_id);
CREATE INDEX idx_sync_operations_batch  ON sync_operations (batch_id);

-- ─── 4. The pull cursor ────────────────────────────────────────────
--
-- One row per affected record per change. `seq` IS the cursor the client
-- sends back as ?since=.
--
-- This table stores (table, row_id) and NOT the payload. The pull endpoint
-- re-reads the live row, so column-level redaction (stripping password_hash
-- from users, for example) happens in exactly one place.
CREATE TABLE sync_change_log (
    seq                BIGSERIAL PRIMARY KEY,
    table_name         TEXT NOT NULL,
    row_id             UUID NOT NULL,
    operation          TEXT NOT NULL,
    server_rev         BIGINT NOT NULL,
    changed_by         UUID NOT NULL REFERENCES users(id),
    device_id          UUID NULL REFERENCES devices(id),
    changed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    device_recorded_at TIMESTAMPTZ NULL,

    CONSTRAINT chk_change_log_operation
        CHECK (operation IN ('upsert', 'delete'))
);

-- The only index the pull query needs: seq is the cursor and the sort.
CREATE INDEX idx_change_log_pull ON sync_change_log (seq);

-- ─── 5. Recorded conflicts ─────────────────────────────────────────
--
-- The policy is automatic (LWW / append-only), but EVERY overwrite is
-- recorded. It costs almost nothing and buys two things: a defensible answer
-- to "what happened to my edit", and a debugging tool when two devices
-- behave oddly. The losing payload is KEPT, never discarded.
CREATE TABLE sync_conflicts (
    id               UUID PRIMARY KEY,
    table_name       TEXT NOT NULL,
    row_id           UUID NOT NULL,
    device_id        UUID NOT NULL REFERENCES devices(id),
    user_id          UUID NOT NULL REFERENCES users(id),
    conflict_type    TEXT NOT NULL,
    client_revision  BIGINT NULL,
    server_revision  BIGINT NULL,
    winning_payload  JSONB NULL,
    losing_payload   JSONB NULL,
    detected_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at      TIMESTAMPTZ NULL,
    reviewed_by      UUID NULL REFERENCES users(id),
    resolution       TEXT NULL,

    CONSTRAINT chk_sync_conflict_type CHECK (conflict_type IN (
        'lww_overwrite', 'unique_violation', 'stale_reference', 'constraint_violation'
    )),
    CONSTRAINT chk_sync_conflict_review
        CHECK ((reviewed_at IS NULL) = (reviewed_by IS NULL))
);

CREATE INDEX idx_sync_conflicts_unreviewed
    ON sync_conflicts (detected_at DESC)
    WHERE reviewed_at IS NULL;
CREATE INDEX idx_sync_conflicts_row ON sync_conflicts (table_name, row_id);

-- ─── 6. Per-device sync health ─────────────────────────────────────
--
-- What makes the admin's device-health screen possible, and what lets you
-- diagnose a workstation that has quietly stopped syncing. "Last seen" and
-- "pending operations" are the two fields that turn an abstract risk into an
-- operational one.
CREATE TABLE device_sync_state (
    device_id              UUID PRIMARY KEY REFERENCES devices(id),
    last_pull_cursor       BIGINT NOT NULL DEFAULT 0,
    last_push_at           TIMESTAMPTZ NULL,
    last_pull_at           TIMESTAMPTZ NULL,
    pending_operation_count INTEGER NOT NULL DEFAULT 0,
    last_error             TEXT NULL,
    last_error_at          TIMESTAMPTZ NULL,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── 7. Scope seeds ────────────────────────────────────────────────
--
-- ONLY tables that exist today. A scope row naming a table that does not
-- exist would make the pull loop try to query it and return a 500 — so each
-- future sprint adds its own row when its migration lands. That is also
-- CLAUDE.md rule 6: a new syncable table is not done until it is scoped.
--
-- The full planned set (patients, encounters, vitals, …) is listed in
-- docs/01-DATA-MODEL.md §9.1; copy the row from there as each table lands.
INSERT INTO sync_table_scopes
    (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('facilities', '{*}',   '{admin}', 'lww',
     'One configuration row: name, code, currency, timezone. Printed on receipts.'),
    ('users',      '{*}',   '{admin}', 'lww',
     'Names for display. password_hash is stripped by the pull builder, never by the client.'),
    ('user_roles', '{*}',   '{admin}', 'lww',
     'Clients need roles to make offline UI decisions; the server still re-checks every action.'),
    ('devices',    '{*}',   '{admin}', 'lww',
     'Needed so an offline client can show which workstation signed what.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
-- Reverse dependency order: change log references devices and users, conflicts
-- reference both, operations reference batches.
DROP TABLE IF EXISTS device_sync_state;
DROP TABLE IF EXISTS sync_conflicts;
DROP TABLE IF EXISTS sync_change_log;
DROP TABLE IF EXISTS sync_operations;
DROP TABLE IF EXISTS sync_batches;
DROP TABLE IF EXISTS sync_table_scopes;
-- +goose StatementEnd
