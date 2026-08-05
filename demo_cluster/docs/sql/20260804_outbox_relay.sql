CREATE TABLE IF NOT EXISTS asset_operation (
    operation_id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL,
    operation_type TEXT NOT NULL,
    request_id TEXT NOT NULL,
    status TEXT NOT NULL,
    response_payload BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (user_id, operation_type, request_id)
);

CREATE TABLE IF NOT EXISTS asset_ledger (
    ledger_id BIGSERIAL PRIMARY KEY,
    operation_id UUID NOT NULL REFERENCES asset_operation(operation_id),
    user_id BIGINT NOT NULL,
    asset_kind TEXT NOT NULL,
    delta BIGINT NOT NULL CHECK (delta <> 0),
    balance_after BIGINT NOT NULL,
    reason TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (operation_id, asset_kind, reason)
);

CREATE INDEX IF NOT EXISTS idx_asset_ledger_player_time
    ON asset_ledger (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS domain_outbox (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
-- Run once in PostgreSQL before deploying the Game outbox relay.
-- The existing event_id primary key remains the event identity everywhere.

ALTER TABLE newsz_2024.domain_outbox
    ADD COLUMN IF NOT EXISTS locked_by TEXT,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_error TEXT;

-- Only PENDING rows are dispatch candidates.  This partial index means the
-- Relay does not scan the published history on every poll.
CREATE INDEX idx_domain_outbox_pending_dispatch_v2 
ON newsz_2024.domain_outbox (created_at ASC, next_attempt_at, locked_until) 
WHERE status = 'PENDING';

