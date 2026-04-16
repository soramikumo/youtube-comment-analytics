package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Store wraps a sql.DB and provides upsert methods for each table.
type Store struct {
	db *sql.DB
}

// New opens a connection to TiDB and verifies it with a ping.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ChannelRow represents a row to upsert into channels.
type ChannelRow struct {
	ID                 string
	Title              string
	Description        string
	CustomURL          string
	Country            string
	ThumbnailURL       string
	UploadsPlaylistID  string
	SubscriberCount    int64
	VideoCount         int
	ViewCount          int64
	ChannelPublishedAt time.Time
}

func (s *Store) UpsertChannel(r *ChannelRow) error {
	_, err := s.db.Exec(`
		INSERT INTO channels (id, title, description, custom_url, country, thumbnail_url,
			uploads_playlist_id, subscriber_count, video_count, view_count,
			channel_published_at, first_seen_at, last_fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			title = VALUES(title),
			description = VALUES(description),
			custom_url = VALUES(custom_url),
			country = VALUES(country),
			thumbnail_url = VALUES(thumbnail_url),
			uploads_playlist_id = VALUES(uploads_playlist_id),
			subscriber_count = VALUES(subscriber_count),
			video_count = VALUES(video_count),
			view_count = VALUES(view_count),
			last_fetched_at = NOW()`,
		r.ID, r.Title, r.Description, r.CustomURL, r.Country, r.ThumbnailURL,
		r.UploadsPlaylistID, r.SubscriberCount, r.VideoCount, r.ViewCount,
		r.ChannelPublishedAt,
	)
	return err
}

// VideoRow represents a row to upsert into videos.
type VideoRow struct {
	ID               string
	ChannelID        string
	Title            string
	Description      string
	ThumbnailURL     string
	CategoryID       int
	DefaultLanguage  string
	DurationSeconds  int
	Tags             string // JSON string
	ViewCount        int64
	LikeCount        int64
	CommentCount     int64
	IsLiveArchive    bool
	LiveActualStart  *time.Time
	LiveActualEnd    *time.Time
	VideoPublishedAt time.Time
	CommentsDisabled bool
}

func (s *Store) UpsertVideo(r *VideoRow) error {
	_, err := s.db.Exec(`
		INSERT INTO videos (id, channel_id, title, description, thumbnail_url,
			category_id, default_language, duration_seconds, tags,
			view_count, like_count, comment_count,
			is_live_archive, live_actual_start_at, live_actual_end_at,
			video_published_at, first_seen_at, last_fetched_at, comments_disabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), ?)
		ON DUPLICATE KEY UPDATE
			title = VALUES(title),
			description = VALUES(description),
			thumbnail_url = VALUES(thumbnail_url),
			view_count = VALUES(view_count),
			like_count = VALUES(like_count),
			comment_count = VALUES(comment_count),
			last_fetched_at = NOW(),
			comments_disabled = VALUES(comments_disabled)`,
		r.ID, r.ChannelID, r.Title, r.Description, r.ThumbnailURL,
		r.CategoryID, r.DefaultLanguage, r.DurationSeconds, r.Tags,
		r.ViewCount, r.LikeCount, r.CommentCount,
		r.IsLiveArchive, r.LiveActualStart, r.LiveActualEnd,
		r.VideoPublishedAt, r.CommentsDisabled,
	)
	return err
}

// AuthorRow represents a row to upsert into authors.
type AuthorRow struct {
	ID              string
	DisplayName     string
	ProfileImageURL string
}

func (s *Store) UpsertAuthor(r *AuthorRow) error {
	_, err := s.db.Exec(`
		INSERT INTO authors (id, display_name, profile_image_url, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			display_name = VALUES(display_name),
			profile_image_url = VALUES(profile_image_url),
			last_seen_at = NOW()`,
		r.ID, r.DisplayName, r.ProfileImageURL,
	)
	return err
}

// CommentRow represents a row to upsert into comments.
type CommentRow struct {
	ID                     string
	VideoID                string
	AuthorID               string
	AuthorDisplayNameAtPost string
	TextOriginal           string
	LikeCount              int
	ReplyCount             int
	CommentPublishedAt     time.Time
	CommentUpdatedAt       time.Time
}

func (s *Store) UpsertComment(r *CommentRow) error {
	_, err := s.db.Exec(`
		INSERT INTO comments (id, video_id, author_id, author_display_name_at_post,
			text_original, like_count, reply_count,
			comment_published_at, comment_updated_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
			like_count = VALUES(like_count),
			reply_count = VALUES(reply_count),
			comment_updated_at = VALUES(comment_updated_at),
			fetched_at = NOW()`,
		r.ID, r.VideoID, r.AuthorID, r.AuthorDisplayNameAtPost,
		r.TextOriginal, r.LikeCount, r.ReplyCount,
		r.CommentPublishedAt, r.CommentUpdatedAt,
	)
	return err
}

// ReplyRow represents a row to upsert into replies.
type ReplyRow struct {
	ID                     string
	ParentCommentID        string
	VideoID                string
	AuthorID               string
	AuthorDisplayNameAtPost string
	TextOriginal           string
	LikeCount              int
	ReplyPublishedAt       time.Time
	ReplyUpdatedAt         time.Time
}

func (s *Store) UpsertReply(r *ReplyRow) error {
	_, err := s.db.Exec(`
		INSERT INTO replies (id, parent_comment_id, video_id, author_id,
			author_display_name_at_post, text_original, like_count,
			reply_published_at, reply_updated_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
			like_count = VALUES(like_count),
			reply_updated_at = VALUES(reply_updated_at),
			fetched_at = NOW()`,
		r.ID, r.ParentCommentID, r.VideoID, r.AuthorID,
		r.AuthorDisplayNameAtPost, r.TextOriginal, r.LikeCount,
		r.ReplyPublishedAt, r.ReplyUpdatedAt,
	)
	return err
}

// IngestRun tracks a batch collection run.
type IngestRun struct {
	ID            int64
	StartedAt     time.Time
	Status        string
	ChannelsCount int
	VideosCount   int
	CommentsCount int
	QuotaUsed     int
	ErrorMessage  string
}

func (s *Store) StartIngestRun() (*IngestRun, error) {
	result, err := s.db.Exec(`
		INSERT INTO ingest_runs (started_at, status) VALUES (NOW(), 'running')`)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &IngestRun{ID: id, StartedAt: time.Now(), Status: "running"}, nil
}

func (s *Store) FinishIngestRun(run *IngestRun) error {
	_, err := s.db.Exec(`
		UPDATE ingest_runs SET
			finished_at = NOW(),
			status = ?,
			channels_count = ?,
			videos_count = ?,
			comments_count = ?,
			quota_used = ?,
			error_message = ?
		WHERE id = ?`,
		run.Status, run.ChannelsCount, run.VideosCount, run.CommentsCount,
		run.QuotaUsed, run.ErrorMessage, run.ID,
	)
	return err
}
