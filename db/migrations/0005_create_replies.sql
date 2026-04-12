-- +goose Up
CREATE TABLE replies (
  id                          VARCHAR(64)   PRIMARY KEY,
  parent_comment_id           VARCHAR(64)   NOT NULL,
  video_id                    VARCHAR(16)   NOT NULL,
  author_id                   VARCHAR(32)   NULL,
  author_display_name_at_post VARCHAR(255)  NOT NULL,
  text_original               MEDIUMTEXT    NOT NULL,
  like_count                  INT           NOT NULL DEFAULT 0,
  reply_published_at          DATETIME      NOT NULL,
  reply_updated_at            DATETIME      NOT NULL,
  fetched_at                  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_parent  (parent_comment_id),
  INDEX idx_video   (video_id, reply_published_at DESC),
  INDEX idx_author  (author_id)
);

-- +goose Down
DROP TABLE replies;
