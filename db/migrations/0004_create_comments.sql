-- +goose Up
CREATE TABLE comments (
  id                          VARCHAR(64)   PRIMARY KEY,
  video_id                    VARCHAR(16)   NOT NULL,
  author_id                   VARCHAR(32)   NULL,
  author_display_name_at_post VARCHAR(255)  NOT NULL,
  text_original               MEDIUMTEXT    NOT NULL,
  like_count                  INT           NOT NULL DEFAULT 0,
  reply_count                 INT           NOT NULL DEFAULT 0,
  comment_published_at        DATETIME      NOT NULL,
  comment_updated_at          DATETIME      NOT NULL,
  fetched_at                  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_video_published (video_id, comment_published_at DESC),
  INDEX idx_author (author_id)
);

-- +goose Down
DROP TABLE comments;
