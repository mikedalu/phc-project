-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Every write to a synced table must reach the change log.
--
--  ═══ THE BUG THIS FIXES ═══
--
--  The sync applier appended a change-log row for every operation it applied,
--  but the REST endpoints did NOT: they wrote the row and stopped. So a payment
--  taken at the cashier's desk was invisible to every ALREADY-BOOTSTRAPPED
--  device — those devices only ever saw it if they bootstrapped again, which is
--  a full resync, not a pull.
--
--  A trigger is the right fix because it cannot be forgotten. A new endpoint
--  that writes a synced table is covered the day it is written, and the next
--  person does not need to know that a change log exists.
--
--  Duplicate entries are possible: the sync path appends explicitly AND the
--  trigger fires on its UPDATE. That is harmless — the client upserts by id and
--  its cursor is monotonic — and cheaper than teaching every applier to suppress
--  a trigger.
-- ════════════════════════════════════════════════════════════════════

CREATE OR REPLACE FUNCTION sync_log_change() RETURNS trigger AS $$
DECLARE
    actor uuid;
BEGIN
    -- The constraint allows exactly 'upsert' and 'delete'. An INSERT and an
    -- UPDATE are the same thing to a replica: "this row changed, here it is".
    -- Using lower(TG_OP) failed that CHECK on the very first patient insert.
    IF TG_OP = 'DELETE' THEN
        actor := OLD.updated_by;
        INSERT INTO sync_change_log (table_name, row_id, operation, server_rev, changed_by)
        VALUES (TG_TABLE_NAME, OLD.id, 'delete', OLD.server_rev, actor);
        RETURN OLD;
    END IF;

    actor := NEW.updated_by;
    INSERT INTO sync_change_log (table_name, row_id, operation, server_rev, changed_by)
    VALUES (TG_TABLE_NAME, NEW.id, 'upsert', NEW.server_rev, actor);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- One trigger per synced table. Attach to every table that carries the nine sync
-- columns; sync_probe is included so the diagnostics table behaves like the rest.
DO $$
DECLARE
    t text;
    tables text[] := ARRAY[
        'sync_probe', 'patients', 'households', 'encounters', 'vitals',
        'diagnoses', 'lab_orders', 'lab_results',
        'tariffs', 'invoices', 'charges', 'payments', 'payment_allocations',
        'prescriptions', 'prescription_items', 'dispenses'
    ];
BEGIN
    FOREACH t IN ARRAY tables LOOP
        EXECUTE format(
            'DROP TRIGGER IF EXISTS trg_sync_log ON %I', t
        );
        EXECUTE format(
            'CREATE TRIGGER trg_sync_log AFTER INSERT OR UPDATE OR DELETE ON %I
             FOR EACH ROW EXECUTE FUNCTION sync_log_change()', t
        );
    END LOOP;
END $$;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    t text;
    tables text[] := ARRAY[
        'sync_probe', 'patients', 'households', 'encounters', 'vitals',
        'diagnoses', 'lab_orders', 'lab_results',
        'tariffs', 'invoices', 'charges', 'payments', 'payment_allocations',
        'prescriptions', 'prescription_items', 'dispenses'
    ];
BEGIN
    FOREACH t IN ARRAY tables LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS trg_sync_log ON %I', t);
    END LOOP;
END $$;
DROP FUNCTION IF EXISTS sync_log_change();
-- +goose StatementEnd
