-- +goose Up
CREATE TABLE channels (
  id                   VARCHAR(32)   PRIMARY KEY,
  title                VARCHAR(255)  NOT NULL,
  description          TEXT          NULL,
  custom_url           VARCHAR(128)  NULL,
  country              VARCHAR(8)    NULL,
  thumbnail_url        VARCHAR(512)  NULL,
  uploads_playlist_id  VARCHAR(32)   NULL,
  subscriber_count     BIGINT        NULL,
  video_count          INT           NULL,
  view_count           BIGINT        NULL,
  channel_published_at DATETIME      NULL,
  first_seen_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_fetched_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  is_active            BOOLEAN       NOT NULL DEFAULT TRUE
);

-- +goose Down
DROP TABLE channels;
