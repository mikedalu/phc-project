-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  A starting price list.
--
--  Prices are BIGINT minor units (kobo). They are a SEED, not a truth: a real
--  PHC edits them. The scheme that matters here is that a charge SNAPSHOTS its
--  price, so changing a tariff never rewrites an invoice that was already
--  issued — which is why the table has effective_from/effective_to instead of an
--  UPDATE.
--
--  The admin user is the author of these rows so the audit trail is honest about
--  where they came from.
-- ════════════════════════════════════════════════════════════════════

INSERT INTO tariffs (id, code, name, category, unit_price_minor, unit, created_by, updated_by)
SELECT gen_random_uuid(), v.code, v.name, v.category, v.price, v.unit, u.id, u.id
  FROM (VALUES
    ('REG',        'New patient registration', 'registration',  100000::BIGINT, 'per_visit'),
    ('CONSULT',    'General consultation',     'consultation',  200000::BIGINT, 'per_visit'),
    ('CONSULT_ANC','Antenatal consultation',   'consultation',  150000::BIGINT, 'per_visit'),
    ('LAB_MP',     'Malaria parasite test',    'lab',           150000::BIGINT, 'per_test'),
    ('LAB_FBC',    'Full blood count',         'lab',           250000::BIGINT, 'per_test'),
    ('LAB_URIN',   'Urinalysis',               'lab',           100000::BIGINT, 'per_test'),
    ('LAB_RBS',    'Random blood sugar',       'lab',            80000::BIGINT, 'per_test'),
    ('LAB_WIDAL',  'Widal test',               'lab',           120000::BIGINT, 'per_test'),
    ('LAB_GXP',    'GeneXpert (TB)',           'lab',           500000::BIGINT, 'per_test'),
    ('DRUG_DISP',  'Dispensing fee',           'drug',           50000::BIGINT, 'per_visit'),
    ('WARD_DAY',   'Ward bed per day',         'ward',          300000::BIGINT, 'per_day'),
    ('PROC_DRESS', 'Wound dressing',           'procedure',     180000::BIGINT, 'per_visit')
  ) AS v(code, name, category, price, unit)
  CROSS JOIN (SELECT id FROM users WHERE staff_number = 'DEMO/STF/0001') AS u
ON CONFLICT (code) DO NOTHING;

-- ─── sync scopes ───────────────────────────────────────────────────
-- Already inserted by 00016. Nothing to add.

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM tariffs WHERE code IN
    ('REG','CONSULT','CONSULT_ANC','LAB_MP','LAB_FBC','LAB_URIN','LAB_RBS',
     'LAB_WIDAL','LAB_GXP','DRUG_DISP','WARD_DAY','PROC_DRESS');
-- +goose StatementEnd
