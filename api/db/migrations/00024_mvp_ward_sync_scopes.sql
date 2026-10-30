-- +goose Up
-- +goose StatementBegin
--
-- The REST ward endpoints allow doctors, nurses and admins to admit and accrue a
-- bed-day; the ward charge then creates an invoice and a charge line. An offline
-- device does the same work through sync, so the sync write scopes must match
-- the REST roles. Without this, a nurse's local invoice/charge payload was
-- rejected while the ward charge itself was accepted, leaving a billed night
-- with no bill.
--
UPDATE sync_table_scopes
   SET push_roles = '{doctor,nurse,admin}',
       notes = 'Status advances on discharge, so LWW on the header is right.'
 WHERE table_name = 'admissions';

UPDATE sync_table_scopes
   SET push_roles = '{doctor,nurse,cashier,admin}',
       notes = 'A night billed. The unique index on (admission_id, charge_date) is the double-billing guard.'
 WHERE table_name = 'ward_charges';

UPDATE sync_table_scopes
   SET push_roles = '{front_desk,cashier,doctor,nurse,admin}'
 WHERE table_name IN ('invoices', 'charges');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE sync_table_scopes
   SET push_roles = '{doctor,nurse}'
 WHERE table_name = 'admissions';

UPDATE sync_table_scopes
   SET push_roles = '{nurse,cashier,admin}'
 WHERE table_name = 'ward_charges';

UPDATE sync_table_scopes
   SET push_roles = '{front_desk,cashier,doctor,admin}'
 WHERE table_name IN ('invoices', 'charges');
-- +goose StatementEnd
