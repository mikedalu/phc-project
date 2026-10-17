-- +goose Up
-- +goose StatementBegin

-- Grant the bootstrap administrator its admin role.
--
-- WITHOUT THIS the seeded user exists but cannot authorise anything: every
-- capability check resolves against user_roles, so a user with no rows can log
-- in and then do nothing. This migration is what makes the first login usable.
--
-- Idempotent via NOT EXISTS rather than ON CONFLICT: the unique index is
-- partial (WHERE revoked_at IS NULL), and ON CONFLICT needs the full predicate
-- to target it — writing that correctly is more fragile than an explicit
-- existence check.
--
-- Self-granted: granted_by is the admin itself, because at bootstrap there is no
-- other actor. That is honest in the audit trail — "the bootstrap user granted
-- itself admin" is exactly what happened.
INSERT INTO user_roles (
    id, user_id, role, granted_at, granted_by,
    created_by, updated_by
)
SELECT
    '018f0000-0000-7000-8000-0000000000a5',
    u.id,
    'admin',
    now(),
    u.id,
    u.id,
    u.id
FROM users u
WHERE u.staff_number = 'DEMO/STF/0001'
  AND u.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM user_roles ur
      WHERE ur.user_id = u.id
        AND ur.role = 'admin'
        AND ur.revoked_at IS NULL
        AND ur.deleted_at IS NULL
  );

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM user_roles WHERE id = '018f0000-0000-7000-8000-0000000000a5';
-- +goose StatementEnd
