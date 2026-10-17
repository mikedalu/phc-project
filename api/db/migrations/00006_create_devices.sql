-- +goose Up
-- +goose StatementBegin

-- Registered workstations.
--
-- A device is a CREDENTIAL HOLDER, not merely a browser fingerprint. Each
-- workstation generates an ECDSA P-256 keypair on first run; the private key
-- stays in IndexedDB and the public key lives here. /sync/push requires BOTH a
-- valid JWT and a valid device signature, so a stolen token alone cannot inject
-- records. See docs/05-SECURITY.md §4.
--
-- Devices are revocable individually and immediately (`revoked_at`). With full
-- offline replication a browser holds every patient record in the clinic, so
-- losing a laptop must be recoverable WITHOUT locking the human out of their
-- account or invalidating every other workstation.
CREATE TABLE devices (
    id   UUID PRIMARY KEY,

    -- The user who REGISTERED this device, for the audit trail.
    -- NULLABLE on purpose: a workstation is shared in a PHC — the front desk,
    -- the cashier and the triage nurse may all log in on the same machine — so
    -- a device does not "belong" to one user. refresh_tokens carries the
    -- user↔device pairing that actually matters for sessions.
    --
    -- (An earlier draft had this NOT NULL, which created a circular FK with
    -- refresh_tokens: devices.user_id -> users, refresh_tokens.device_id ->
    -- devices. Making it nullable breaks the cycle without losing anything,
    -- because the registrant is recorded here and the session owner is
    -- recorded per token.)
    registered_by UUID REFERENCES users(id),

    name     TEXT NOT NULL,               -- human label: "Reception Desk 1"
    platform TEXT,                        -- e.g. "Windows 11 / Chrome 141"

    -- Base64url SPKI-encoded public key.
    -- ⚠️ SPKI, not raw: WebCrypto exports P-256 public keys as 'spki' and
    -- THROWS if you ask for 'raw'. See docs/05-SECURITY.md §4.2.
    public_key     TEXT NOT NULL,
    public_key_alg TEXT NOT NULL DEFAULT 'ES256',

    -- SHA-256 of the public key, shown to the admin so a human can compare it
    -- against what the device reports. Unique per facility-less deployment.
    fingerprint TEXT NOT NULL,

    app_version  TEXT,
    last_seen_at TIMESTAMPTZ,
    last_sync_cursor BIGINT NOT NULL DEFAULT 0,

    revoked_at     TIMESTAMPTZ,
    revoked_by     UUID REFERENCES users(id),
    revoked_reason TEXT,

    -- ─── sync column contract ──────────────────────────────────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NOT NULL,
    updated_by  UUID        NOT NULL,

    CONSTRAINT chk_devices_alg CHECK (public_key_alg IN ('ES256')),
    CONSTRAINT chk_devices_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    -- A revocation without attribution is useless in an audit.
    CONSTRAINT chk_devices_revoked_by
        CHECK ((revoked_at IS NULL) = (revoked_by IS NULL))
);

CREATE UNIQUE INDEX uq_devices_fingerprint ON devices (fingerprint);
CREATE INDEX idx_devices_server_rev  ON devices (server_rev);
CREATE INDEX idx_devices_active      ON devices (last_seen_at) WHERE revoked_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS devices;
-- +goose StatementEnd
