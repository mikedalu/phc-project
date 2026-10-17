-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  sync_probe — a DEVELOPMENT table. Not clinical data.
--
--  Sprint 2 must prove the sync engine end to end BEFORE any clinical
--  table exists, because building sync after the features means rewriting
--  every screen's data access. This table is the smallest thing that can be
--  written from two browsers, edited, deleted, and shown converging:
--  `id`, `title`, `body`, and the nine sync columns.
--
--  It is what `/dev/sync` writes to, and it is what the sprint demo uses
--  ("A says hello"). It ships in the schema because a demo table that only
--  exists in a developer's head cannot be demonstrated to an examiner.
--
--  Deleting it later is a one-migration change and nothing references it.
-- ════════════════════════════════════════════════════════════════════
CREATE TABLE sync_probe (
    id      UUID PRIMARY KEY,

    title   TEXT NOT NULL,
    body    TEXT NULL,

    -- ─── the nine sync columns (docs/01-DATA-MODEL.md §3) ──────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL REFERENCES users(id),
    updated_by  UUID        NOT NULL REFERENCES users(id),

    CONSTRAINT chk_sync_probe_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_sync_probe_title
        CHECK (length(btrim(title)) > 0)
);

CREATE INDEX idx_sync_probe_server_rev ON sync_probe (server_rev);
CREATE INDEX idx_sync_probe_updated    ON sync_probe (updated_at);

-- Every role may read and write the probe. It is a development table; giving
-- it a narrow role list would make the two-browser demo need two logins.
INSERT INTO sync_table_scopes
    (table_name, pull_roles, push_roles, conflict_policy, notes) VALUES
    ('sync_probe', '{*}', '{*}', 'lww',
     'Development probe table for demonstrating the sync engine. Not clinical.')
ON CONFLICT (table_name) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM sync_table_scopes WHERE table_name = 'sync_probe';
DROP TABLE IF EXISTS sync_probe;
-- +goose StatementEnd
