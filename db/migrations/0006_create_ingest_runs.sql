-- +goose Up
CREATE TABLE ingest_runs (
  id             BIGINT        AUTO_INCREMENT PRIMARY KEY,
  started_at     DATETIME      NOT NULL,
  finished_at    DATETIME      NULL,
  status         VARCHAR(16)   NOT NULL,
  channels_count INT           NOT NULL DEFAULT 0,
  videos_count   INT           NOT NULL DEFAULT 0,
  comments_count INT           NOT NULL DEFAULT 0,
  quota_used     INT           NOT NULL DEFAULT 0,
  error_message  TEXT          NULL
);

-- +goose Down
DROP TABLE ingest_runs;
