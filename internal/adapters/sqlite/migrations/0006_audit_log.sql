-- +goose Up
CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,
    occurred_at     TEXT NOT NULL,
    tenant_id       TEXT NOT NULL DEFAULT '',
    actor_id        TEXT NOT NULL DEFAULT '',
    actor_type      TEXT NOT NULL DEFAULT '',
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL DEFAULT '',
    resource_id     TEXT NOT NULL DEFAULT '',
    outcome         TEXT NOT NULL,
    ip              TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    metadata        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_audit_log_occurred_at ON audit_log(occurred_at DESC);
CREATE INDEX idx_audit_log_tenant_action ON audit_log(tenant_id, action, occurred_at DESC);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_id, occurred_at DESC);

-- +goose Down
DROP TABLE audit_log;
