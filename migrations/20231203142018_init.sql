-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sla_record (
    id BIGSERIAL PRIMARY KEY,
    datetime TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alias VARCHAR(255) NOT NULL,
    metric TEXT NOT NULL,
    slo FLOAT4 NOT NULL,
    value FLOAT4 NOT NULL,
    met BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE UNIQUE INDEX IF NOT EXISTS sla_record_alias_idx ON sla_record(alias);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sla_record;
-- +goose StatementEnd
