-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  The lab must be able to WRITE the orders it advances.
--
--  00014 gave lab_orders a push_role list of {doctor} only. That was wrong in a
--  way no unit test would catch: a lab scientist could collect a specimen and
--  record a result, and both writes were silently refused at the sync scope —
--  the result row itself landed (lab_results allows lab_scientist) while the
--  order stayed 'ordered' on the server for ever.
--
--  The symptom a clinician sees: the result exists, the worklist still says the
--  specimen is waiting to be collected.
-- ════════════════════════════════════════════════════════════════════

UPDATE sync_table_scopes
   SET push_roles = '{doctor,lab_scientist,nurse}',
       notes = 'Status advances (ordered -> collected -> resulted) by the people who do the work.'
 WHERE table_name = 'lab_orders';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
UPDATE sync_table_scopes
   SET push_roles = '{doctor}',
       notes = 'Status advances (ordered -> collected -> resulted), so LWW on the row is right.'
 WHERE table_name = 'lab_orders';
-- +goose StatementEnd
