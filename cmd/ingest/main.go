// Package main is the batch ingest worker.
// It collects channels, videos, and comments from the YouTube API
// and writes them to TiDB.
//
// # 処理フロー
//
//	channels.json に登録されたチャンネルを順に処理する。
//
//	チャンネルごとに以下の順で YouTube Data API v3 を呼び出す:
//	  1. channels.list        — チャンネル情報取得          (1クォータ)
//	  2. playlistItems.list   — アップロード動画ID一覧取得   (1クォータ/ページ)
//	  3. videos.list          — 動画詳細取得 (50件バッチ)    (1クォータ/バッチ)
//	  4. commentThreads.list  — トップレベルコメント取得     (1クォータ/ページ)
//	                            返信は各スレッドに最大5件インラインで含まれる
//
// # フラグの役割
//
//	-max-videos  : 1チャンネルあたりの収集動画数上限。収集量を設計するためのパラメータ。
//	-max-comments: 1動画あたりのトップレベルコメント収集数上限。
//	-quota-limit : クォータ消費量がこの値に達したら処理を打ち切る安全弁。
//	               -max-videos/-max-comments で収集量を制御し、
//	               -quota-limit はバグや想定外データによるクォータ枯渇を防ぐ保険として機能する。
//	-dry-run     : API は叩くが DB 書き込みをスキップする。動作確認・デバッグ用。
//
// # クォータ消費の目安 (デフォルト設定: 10動画, 100コメント/動画)
//
//	channels.list       :  1
//	playlistItems.list  :  1  (10件なら1ページ)
//	videos.list         :  1  (10件なら1バッチ)
//	commentThreads.list : 10  (10動画 × 1ページ)
//	合計                : 約13クォータ  (YouTube API 1日上限: 10,000)
//
// # Usage
//
//	go run ./cmd/ingest                          # collect all channels
//	go run ./cmd/ingest -max-videos 5            # limit videos per channel
//	go run ./cmd/ingest -max-comments 50         # limit comments per video
//	go run ./cmd/ingest -quota-limit 1000        # stop when quota approaches limit
//	go run ./cmd/ingest -dry-run                 # fetch but don't write to DB
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/soramikumo/youtube-comment-analytics/internal/config"
	"github.com/soramikumo/youtube-comment-analytics/internal/db"
	"github.com/soramikumo/youtube-comment-analytics/internal/youtube"
)

func main() {
	maxVideos := flag.Int("max-videos", 10, "max videos to collect per channel")
	maxComments := flag.Int("max-comments", 100, "max top-level comments per video")
	quotaLimit := flag.Int("quota-limit", 9000, "stop when quota usage reaches this")
	dryRun := flag.Bool("dry-run", false, "fetch from API but skip DB writes")
	flag.Parse()

	// .env.local から API キーと DB 接続情報を、channels.json から対象チャンネル一覧を読み込む
	cfg, err := config.Load(".env.local", "channels.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := youtube.NewClient(cfg.YouTubeAPIKey)
	ctx := context.Background()

	// dry-run 時は DB 接続をスキップし、store は nil のまま各 Upsert 呼び出しを無視する
	var store *db.Store
	if !*dryRun {
		store, err = db.New(cfg.DBDSN)
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		defer store.Close()
	}

	// ingest_runs テーブルに実行ログを記録する（開始時刻・終了時刻・件数・クォータ消費量）
	var run *db.IngestRun
	if store != nil {
		run, err = store.StartIngestRun()
		if err != nil {
			log.Fatalf("start ingest run: %v", err)
		}
	}

	quota := 0
	totalChannels := 0
	totalVideos := 0
	totalComments := 0
	var lastErr error

	for _, ch := range cfg.Channels {
		// クォータ上限に達したらチャンネルループを打ち切る
		if quota >= *quotaLimit {
			log.Printf("⚠️  Quota limit reached (%d/%d), stopping", quota, *quotaLimit)
			break
		}

		log.Printf("📺 Processing channel: %s (%s)", ch.ID, ch.Note)

		// channels.json の id にはチャンネルID・@ハンドル・URL のいずれも指定可能。
		// resolveChannelInput で API クライアントが受け付ける形式に正規化する。
		resolvedID := resolveChannelInput(ch.ID)

		// 1. channels.list: チャンネルの基本情報と uploads プレイリスト ID を取得する
		chResp, err := client.GetChannel(ctx, resolvedID)
		quota++
		if err != nil {
			log.Printf("  ❌ channels.list: %v", err)
			lastErr = err
			continue
		}

		chRow := parseChannel(chResp)
		if chRow == nil {
			log.Printf("  ❌ channel not found")
			continue
		}

		// channels テーブルに UPSERT（同一 ID なら上書き更新）
		if store != nil {
			if err := store.UpsertChannel(chRow); err != nil {
				log.Printf("  ❌ upsert channel: %v", err)
				lastErr = err
				continue
			}
		}
		totalChannels++
		log.Printf("  ✅ Channel: %s (%s subscribers)", chRow.Title, fmtInt(chRow.SubscriberCount))

		// 2. playlistItems.list: uploads プレイリストから動画 ID 一覧を取得する。
		// チャンネルの全動画はこのプレイリストに格納されている（YouTube の仕様）。
		uploadsID := chRow.UploadsPlaylistID
		if uploadsID == "" {
			log.Printf("  ⚠️  No uploads playlist, skipping videos")
			continue
		}

		videoIDs, q := fetchVideoIDs(ctx, client, uploadsID, *maxVideos)
		quota += q
		log.Printf("  📋 Found %d videos (quota: +%d)", len(videoIDs), q)

		// 3. videos.list: 動画 ID を最大 50 件ずつバッチで詳細取得する。
		// 統計情報（再生数・コメント数）や再生時間などを取得するために別途呼び出しが必要。
		for i := 0; i < len(videoIDs); i += 50 {
			end := i + 50
			if end > len(videoIDs) {
				end = len(videoIDs)
			}
			batch := videoIDs[i:end]

			vidResp, err := client.GetVideos(ctx, strings.Join(batch, ","))
			quota++
			if err != nil {
				log.Printf("  ❌ videos.list: %v", err)
				lastErr = err
				continue
			}

			videos := parseVideos(vidResp)
			for _, v := range videos {
				// videos テーブルに UPSERT
				if store != nil {
					if err := store.UpsertVideo(v); err != nil {
						log.Printf("  ❌ upsert video %s: %v", v.ID, err)
						lastErr = err
						continue
					}
				}
				totalVideos++

				// コメントが無効化されている動画はスキップ
				if v.CommentsDisabled {
					log.Printf("  🚫 %s: comments disabled", v.Title)
					continue
				}

				if quota >= *quotaLimit {
					break
				}

				// 4. commentThreads.list: トップレベルコメントと返信を取得して DB に保存する
				nc, q := fetchComments(ctx, client, store, v.ID, *maxComments, *dryRun)
				quota += q
				totalComments += nc
				log.Printf("  💬 %s: %d comments (quota: +%d)", truncate(v.Title, 40), nc, q)
			}
		}
	}

	// ingest_runs テーブルの実行ログに集計結果と成否を書き込む
	if run != nil {
		run.ChannelsCount = totalChannels
		run.VideosCount = totalVideos
		run.CommentsCount = totalComments
		run.QuotaUsed = quota
		if lastErr != nil {
			run.Status = "failed"
			run.ErrorMessage = lastErr.Error()
		} else {
			run.Status = "success"
		}
		if err := store.FinishIngestRun(run); err != nil {
			log.Printf("⚠️  finish ingest run: %v", err)
		}
	}

	log.Println("========== Summary ==========")
	log.Printf("Channels: %d", totalChannels)
	log.Printf("Videos:   %d", totalVideos)
	log.Printf("Comments: %d (top-level + replies)", totalComments)
	log.Printf("Quota:    %d / %d", quota, *quotaLimit)
	if *dryRun {
		log.Println("Mode:     DRY RUN (no DB writes)")
	}
	if lastErr != nil {
		log.Printf("⚠️  Last error: %v", lastErr)
	}
}

// resolveChannelInput normalizes a channel input to an ID or handle: prefix.
// Supported formats:
//   - Channel ID: UCX6OQ3DkcsbYNE6H8uQQuVA
//   - Handle: @MrBeast
//   - URL: https://www.youtube.com/@MrBeast
//   - URL: https://www.youtube.com/channel/UCX6OQ3DkcsbYNE6H8uQQuVA
func resolveChannelInput(input string) string {
	input = strings.TrimSpace(input)

	// URL 形式: youtube.com/@ または youtube.com/channel/ を含む場合に分岐
	if strings.Contains(input, "youtube.com/") {
		// https://www.youtube.com/@handle
		if i := strings.Index(input, "/@"); i >= 0 {
			handle := strings.TrimRight(input[i+2:], "/")
			// クエリパラメータを除去
			if q := strings.IndexByte(handle, '?'); q >= 0 {
				handle = handle[:q]
			}
			return "handle:@" + handle
		}
		// https://www.youtube.com/channel/UCxxxx
		if i := strings.Index(input, "/channel/"); i >= 0 {
			id := strings.TrimRight(input[i+9:], "/")
			if q := strings.IndexByte(id, '?'); q >= 0 {
				id = id[:q]
			}
			return id
		}
	}

	// @ハンドル形式: GetChannel が受け付ける "handle:@xxx" プレフィックスに変換
	if strings.HasPrefix(input, "@") {
		return "handle:" + input
	}

	// それ以外はチャンネル ID としてそのまま渡す
	return input
}

// ---------- Fetch helpers ----------

// fetchVideoIDs は uploads プレイリストから動画 ID を最大 max 件取得する。
// playlistItems.list はページネーションで最大 50 件/ページのため、必要に応じてページをまたぐ。
func fetchVideoIDs(ctx context.Context, client *youtube.Client, playlistID string, max int) ([]string, int) {
	var ids []string
	quota := 0
	pageToken := ""

	for len(ids) < max {
		remaining := max - len(ids)
		perPage := 50
		if remaining < perPage {
			perPage = remaining
		}

		resp, err := client.GetPlaylistItems(ctx, playlistID, perPage, pageToken)
		quota++
		if err != nil {
			log.Printf("    playlistItems.list: %v", err)
			break
		}

		items := getItems(resp)
		for _, item := range items {
			cd := getMap(item, "contentDetails")
			vid := getString(cd, "videoId")
			if vid != "" {
				ids = append(ids, vid)
			}
		}

		// nextPageToken が空なら最終ページ
		next := getString(resp, "nextPageToken")
		if next == "" || len(ids) >= max {
			break
		}
		pageToken = next
	}
	return ids, quota
}

// fetchComments は指定動画のトップレベルコメントと返信を取得して DB に保存する。
// commentThreads.list のレスポンスにはトップレベルコメントごとに最大5件の返信がインラインで含まれる。
// コメント・返信それぞれの投稿者を authors テーブルにも UPSERT する（著者テーブルの正規化）。
func fetchComments(ctx context.Context, client *youtube.Client, store *db.Store, videoID string, maxComments int, dryRun bool) (int, int) {
	total := 0
	quota := 0
	pageToken := ""

	for total < maxComments {
		remaining := maxComments - total
		perPage := 100
		if remaining < perPage {
			perPage = remaining
		}

		resp, err := client.GetCommentThreads(ctx, videoID, perPage, pageToken)
		quota++
		if err != nil {
			// コメント無効化の場合は正常扱いで終了
			if strings.Contains(err.Error(), "commentsDisabled") || strings.Contains(err.Error(), "disabled") {
				return 0, quota
			}
			log.Printf("    commentThreads.list: %v", err)
			break
		}

		items := getItems(resp)
		for _, thread := range items {
			snippet := getMap(thread, "snippet")
			tlc := getMap(snippet, "topLevelComment") // トップレベルコメントオブジェクト
			tlcSnippet := getMap(tlc, "snippet")

			// トップレベルコメントの投稿者を authors テーブルに UPSERT
			authorID := extractAuthorID(tlcSnippet)
			if authorID != "" && !dryRun && store != nil {
				store.UpsertAuthor(&db.AuthorRow{
					ID:              authorID,
					DisplayName:     getString(tlcSnippet, "authorDisplayName"),
					ProfileImageURL: getString(tlcSnippet, "authorProfileImageUrl"),
				})
			}

			// トップレベルコメントを comments テーブルに UPSERT
			commentRow := &db.CommentRow{
				ID:                      getString(tlc, "id"),
				VideoID:                 videoID,
				AuthorID:                authorID,
				AuthorDisplayNameAtPost: getString(tlcSnippet, "authorDisplayName"),
				TextOriginal:            getString(tlcSnippet, "textOriginal"),
				LikeCount:               getInt(tlcSnippet, "likeCount"),
				ReplyCount:              getInt(snippet, "totalReplyCount"),
				CommentPublishedAt:      parseTime(getString(tlcSnippet, "publishedAt")),
				CommentUpdatedAt:        parseTime(getString(tlcSnippet, "updatedAt")),
			}
			if !dryRun && store != nil {
				store.UpsertComment(commentRow)
			}
			total++

			// インライン返信を replies テーブルに UPSERT
			// API は最大5件の返信をインラインで返す。5件超は別途 comments.list で取得が必要だが現時点では未対応。
			replies := getMap(thread, "replies")
			if replies != nil {
				replyComments, _ := replies["comments"].([]any)
				for _, rc := range replyComments {
					rcMap, _ := rc.(map[string]any)
					rcSnippet := getMap(rcMap, "snippet")

					replyAuthorID := extractAuthorID(rcSnippet)
					if replyAuthorID != "" && !dryRun && store != nil {
						store.UpsertAuthor(&db.AuthorRow{
							ID:              replyAuthorID,
							DisplayName:     getString(rcSnippet, "authorDisplayName"),
							ProfileImageURL: getString(rcSnippet, "authorProfileImageUrl"),
						})
					}

					replyRow := &db.ReplyRow{
						ID:                      getString(rcMap, "id"),
						ParentCommentID:         getString(rcSnippet, "parentId"),
						VideoID:                 getString(rcSnippet, "videoId"),
						AuthorID:                replyAuthorID,
						AuthorDisplayNameAtPost: getString(rcSnippet, "authorDisplayName"),
						TextOriginal:            getString(rcSnippet, "textOriginal"),
						LikeCount:               getInt(rcSnippet, "likeCount"),
						ReplyPublishedAt:        parseTime(getString(rcSnippet, "publishedAt")),
						ReplyUpdatedAt:          parseTime(getString(rcSnippet, "updatedAt")),
					}
					if !dryRun && store != nil {
						store.UpsertReply(replyRow)
					}
					total++
				}
			}
		}

		// nextPageToken が空なら最終ページ
		next := getString(resp, "nextPageToken")
		if next == "" || total >= maxComments {
			break
		}
		pageToken = next
	}
	return total, quota
}

// ---------- Parse helpers ----------

// parseChannel は channels.list のレスポンスから ChannelRow を生成する。
func parseChannel(resp map[string]any) *db.ChannelRow {
	items := getItems(resp)
	if len(items) == 0 {
		return nil
	}
	ch := items[0]
	snippet := getMap(ch, "snippet")
	stats := getMap(ch, "statistics")
	cd := getMap(ch, "contentDetails")
	rp := getMap(cd, "relatedPlaylists")

	return &db.ChannelRow{
		ID:                 getString(ch, "id"),
		Title:              getString(snippet, "title"),
		Description:        getString(snippet, "description"),
		CustomURL:          getString(snippet, "customUrl"),
		Country:            getString(snippet, "country"),
		ThumbnailURL:       getString(getMap(getMap(snippet, "thumbnails"), "high"), "url"),
		UploadsPlaylistID:  getString(rp, "uploads"), // 動画一覧取得に使うプレイリスト ID
		SubscriberCount:    getInt64(stats, "subscriberCount"),
		VideoCount:         getInt(stats, "videoCount"),
		ViewCount:          getInt64(stats, "viewCount"),
		ChannelPublishedAt: parseTime(getString(snippet, "publishedAt")),
	}
}

// parseVideos は videos.list のレスポンスから VideoRow のスライスを生成する。
func parseVideos(resp map[string]any) []*db.VideoRow {
	items := getItems(resp)
	var result []*db.VideoRow
	for _, item := range items {
		snippet := getMap(item, "snippet")
		stats := getMap(item, "statistics")
		cd := getMap(item, "contentDetails")

		// tags は可変長配列のため JSON 文字列として保存する
		tagsJSON := "null"
		if tags, ok := snippet["tags"]; ok {
			if b, err := jsonMarshal(tags); err == nil {
				tagsJSON = string(b)
			}
		}

		v := &db.VideoRow{
			ID:               getString(item, "id"),
			ChannelID:        getString(snippet, "channelId"),
			Title:            getString(snippet, "title"),
			Description:      getString(snippet, "description"),
			ThumbnailURL:     getString(getMap(getMap(snippet, "thumbnails"), "high"), "url"),
			CategoryID:       getInt(snippet, "categoryId"),
			DefaultLanguage:  getString(snippet, "defaultLanguage"),
			DurationSeconds:  parseDuration(getString(cd, "duration")), // ISO 8601 → 秒数に変換
			Tags:             tagsJSON,
			ViewCount:        getInt64(stats, "viewCount"),
			LikeCount:        getInt64(stats, "likeCount"),
			CommentCount:     getInt64(stats, "commentCount"),
			VideoPublishedAt: parseTime(getString(snippet, "publishedAt")),
		}

		// liveStreamingDetails が存在する場合はライブアーカイブとして扱う
		lsd := getMap(item, "liveStreamingDetails")
		if lsd != nil {
			v.IsLiveArchive = true
			if t := parseTimePtr(getString(lsd, "actualStartTime")); t != nil {
				v.LiveActualStart = t
			}
			if t := parseTimePtr(getString(lsd, "actualEndTime")); t != nil {
				v.LiveActualEnd = t
			}
		}

		result = append(result, v)
	}
	return result
}

// extractAuthorID はコメント snippet の authorChannelId.value を取り出す。
// YouTube API はチャンネル ID を {"value": "UCxxx"} という入れ子構造で返す。
func extractAuthorID(snippet map[string]any) string {
	aci := getMap(snippet, "authorChannelId")
	if aci == nil {
		return ""
	}
	return getString(aci, "value")
}

// ---------- Generic helpers ----------

// getItems は YouTube API レスポンスの "items" 配列を []map[string]any に変換する。
func getItems(resp map[string]any) []map[string]any {
	items, _ := resp["items"].([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func getMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// getInt は YouTube API が数値を float64 または string で返す両パターンに対応する。
func getInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func getInt64(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// parseDuration は ISO 8601 形式の動画時間（例: PT3M21S）を秒数に変換する。
func parseDuration(iso string) int {
	if !strings.HasPrefix(iso, "PT") {
		return 0
	}
	iso = strings.TrimPrefix(iso, "PT")
	total := 0

	if i := strings.Index(iso, "H"); i >= 0 {
		n, _ := strconv.Atoi(iso[:i])
		total += n * 3600
		iso = iso[i+1:]
	}
	if i := strings.Index(iso, "M"); i >= 0 {
		n, _ := strconv.Atoi(iso[:i])
		total += n * 60
		iso = iso[i+1:]
	}
	if i := strings.Index(iso, "S"); i >= 0 {
		n, _ := strconv.Atoi(iso[:i])
		total += n
	}
	return total
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func fmtInt(n int64) string {
	return fmt.Sprintf("%d", n)
}
