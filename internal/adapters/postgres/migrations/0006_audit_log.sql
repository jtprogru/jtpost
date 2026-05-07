-- +goose Up
CREATE TABLE audit_log (
    id              UUID PRIMARY KEY,
    occurred_at     TIMESTAMPTZ NOT NULL,
    tenant_id       UUID,
    actor_id        UUID,
    actor_type      TEXT NOT NULL DEFAULT '',
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL DEFAULT '',
    resource_id     TEXT NOT NULL DEFAULT '',
    outcome         TEXT NOT NULL,
    ip              TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    metadata        JSONB NOT NULL DEFAULT '{}'::JSONB
);
CREATE INDEX idx_audit_log_occurred_at ON audit_log(occurred_at DESC);
CREATE INDEX idx_audit_log_tenant_action ON audit_log(tenant_id, action, occurred_at DESC);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_id, occurred_at DESC);

-- +goose Down
DROP TABLE audit_log;
