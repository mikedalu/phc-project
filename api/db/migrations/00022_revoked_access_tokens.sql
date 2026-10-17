-- +goose Up
-- +goose StatementBegin

-- ════════════════════════════════════════════════════════════════════
--  Access-token revocation.
--
--  Logout already kills the refresh-token FAMILY, so no new access token can be
--  minted. But a token issued a minute earlier stays valid until it expires,
--  which is the window this table closes: "I signed out" must mean signed out,
--  not "signed out in fifteen minutes".
--
--  The row is keyed by the JWT's `jti` and carries the token's own expiry, so
--  pruning is a single DELETE and the table can never grow past the number of
--  tokens issued in one TTL.
-- ════════════════════════════════════════════════════════════════════

CREATE TABLE revoked_access_tokens (
    jti        UUID PRIMARY KEY,
    -- Carried from the token, so pruning needs no other source of truth.
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason     TEXT NULL
);

CREATE INDEX idx_revoked_tokens_expiry ON revoked_access_tokens (expires_at);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS revoked_access_tokens;
-- +goose StatementEnd
