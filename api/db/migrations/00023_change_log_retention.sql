-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Retention: the change log had none, so it only ever grew.
--
--  Every synced table now carries a window. The values are a JUDGEMENT, not a
--  law:
--
--   • 30 days for clinical tables. A device offline for longer than a month
--     re-bootstraps instead of replaying — which is a full snapshot, not a
--     disaster, and it is the only way the log stays bounded.
--   • 7 days for sync_probe, which is diagnostics rather than a record.
--
--  Pruning is safe ONLY because the pull refuses a cursor older than the oldest
--  surviving row (`cursor_expired`) and the client re-bootstraps when it sees
--  that. Deleting without that guard would leave devices silently incomplete.
-- ════════════════════════════════════════════════════════════════════

UPDATE sync_table_scopes SET retention_days = 30 WHERE table_name IN (
    'patients', 'households', 'encounters', 'vitals', 'diagnoses',
    'lab_orders', 'lab_results',
    'tariffs', 'invoices', 'charges', 'payments', 'payment_allocations',
    'prescriptions', 'prescription_items', 'dispenses',
    'admissions', 'ward_charges', 'referrals'
);

UPDATE sync_table_scopes SET retention_days = 7 WHERE table_name = 'sync_probe';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
UPDATE sync_table_scopes SET retention_days = NULL;
-- +goose StatementEnd
