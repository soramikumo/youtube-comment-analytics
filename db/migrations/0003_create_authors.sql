-- +goose Up
CREATE TABLE authors (
  id                VARCHAR(32)   PRIMARY KEY,
  display_name      VARCHAR(255)  NOT NULL,
  profile_image_url VARCHAR(512)  NULL,
  first_seen_at     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE authors;
