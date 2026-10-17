-- +goose Up
-- +goose StatementBegin

-- Rotating refresh tokens.
--
-- SERVER-ONLY. Deliberately has NO sync columns: refresh tokens must never
-- reach a client's replicated database, or an attacker with the laptop gets
-- every session's refresh token as well. This table is the one place a session
-- can be revoked from, so it stays on the server.
--
-- The device DOES cache its own refresh token in IndexedDB for offline use, but
-- that is the client's copy of its own token — not a replica of this table.
--
-- Design: docs/05-SECURITY.md §3.2.
CREATE TABLE refresh_tokens (
    id      UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,

    -- SHA-256 of the token. NEVER the token itself: a database dump must not
    -- yield usable sessions. SHA-256 is appropriate here (unlike passwords)
    -- because the token is 256 bits of randomness, not a low-entropy secret.
    token_hash TEXT NOT NULL,

    -- Rotation family. All tokens descended from one login share a family_id.
    -- Presenting an already-used token means the token leaked, so the whole
    -- family is revoked — logging out both the attacker and the legitimate user.
    -- That is the safe response: see "reuse detection" in §3.2.
    family_id UUID NOT NULL,

    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,

    revoked_reason TEXT CHECK (revoked_reason IS NULL OR revoked_reason IN (
        'rotated', 'reuse_detected', 'logout', 'device_revoked', 'admin', 'password_changed'
    )),

    -- Where the session came from, for the audit trail.
    ip         INET,
    user_agent TEXT,

    CONSTRAINT chk_refresh_tokens_expiry CHECK (expires_at > issued_at)
);

-- The lookup path for every refresh attempt.
CREATE UNIQUE INDEX uq_refresh_tokens_hash ON refresh_tokens (token_hash);

-- Reuse detection: find every token in a family to revoke them together.
CREATE INDEX idx_refresh_tokens_family ON refresh_tokens (family_id);

-- Session revocation for one user, and cleanup of expired rows.
CREATE INDEX idx_refresh_tokens_user_live
    ON refresh_tokens (user_id)
    WHERE revoked_at IS NULL AND used_at IS NULL;

CREATE INDEX idx_refresh_tokens_expiry ON refresh_tokens (expires_at);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd
