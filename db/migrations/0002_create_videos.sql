-- +goose Up
CREATE TABLE videos (
  id                   VARCHAR(16)   PRIMARY KEY,
  channel_id           VARCHAR(32)   NOT NULL,
  title                VARCHAR(512)  NOT NULL,
  description          MEDIUMTEXT    NULL,
  thumbnail_url        VARCHAR(512)  NULL,
  category_id          INT           NULL,
  default_language     VARCHAR(16)   NULL,
  duration_seconds     INT           NULL,
  tags                 JSON          NULL,
  view_count           BIGINT        NULL,
  like_count           BIGINT        NULL,
  comment_count        BIGINT        NULL,
  is_live_archive      BOOLEAN       NOT NULL DEFAULT FALSE,
  live_actual_start_at DATETIME      NULL,
  live_actual_end_at   DATETIME      NULL,
  video_published_at   DATETIME      NOT NULL,
  first_seen_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_fetched_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  comments_disabled    BOOLEAN       NOT NULL DEFAULT FALSE,
  INDEX idx_channel_published (channel_id, video_published_at DESC),
  INDEX idx_published (video_published_at DESC)
);

-- +goose Down
DROP TABLE videos;
