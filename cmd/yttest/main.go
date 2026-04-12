// Package main is a verification tool that hits each YouTube Data API v3
// endpoint and displays the results in a readable format.
//
// Usage:
//
//	go run ./cmd/yttest -channel UCX6OQ3DkcsbYNE6H8uQQuVA
//	go run ./cmd/yttest -channel https://www.youtube.com/@MrBeast
//	go run ./cmd/yttest -video dQw4w9WgXcQ
//	go run ./cmd/yttest -video https://www.youtube.com/watch?v=dQw4w9WgXcQ
//	go run ./cmd/yttest -channel UCxxx -all    (full pipeline)
//	go run ./cmd/yttest -channel UCxxx -raw    (dump raw JSON)
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/soramikumo/youtube-comment-analytics/internal/youtube"
)

func main() {
	channelArg := flag.String("channel", "", "YouTube channel ID or URL")
	videoArg := flag.String("video", "", "YouTube video ID or URL")
	all := flag.Bool("all", false, "Run full pipeline: channel → videos → comments")
	raw := flag.Bool("raw", false, "Dump raw JSON instead of formatted output")
	flag.Parse()

	if err := loadEnv(".env.local"); err != nil {
		log.Fatalf("load .env.local: %v", err)
	}

	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		log.Fatal("YOUTUBE_API_KEY is not set")
	}

	client := youtube.NewClient(apiKey)
	ctx := context.Background()

	// Parse URLs into IDs
	channelID := parseChannelArg(*channelArg)
	videoID := parseVideoArg(*videoArg)

	if channelID == "" && videoID == "" {
		printUsage()
		return
	}

	if *all && channelID != "" {
		if *raw {
			runAllRaw(ctx, client, channelID)
		} else {
			runAllFormatted(ctx, client, channelID)
		}
		return
	}

	if channelID != "" {
		showChannel(ctx, client, channelID, *raw)
	}
	if videoID != "" {
		showVideo(ctx, client, videoID, *raw)
	}
}

// ---------- Formatted output ----------

func showChannel(ctx context.Context, client *youtube.Client, id string, raw bool) {
	resp, err := client.GetChannel(ctx, id)
	if err != nil {
		log.Fatalf("channels.list: %v", err)
	}
	if raw {
		prettyPrint(resp)
		return
	}

	items := getItems(resp)
	if len(items) == 0 {
		fmt.Println("Channel not found.")
		return
	}
	ch := items[0]
	snippet := getMap(ch, "snippet")
	stats := getMap(ch, "statistics")

	fmt.Println("=== Channel ===")
	fmt.Printf("  ID:          %s\n", getString(ch, "id"))
	fmt.Printf("  Title:       %s\n", getString(snippet, "title"))
	fmt.Printf("  Handle:      %s\n", getString(snippet, "customUrl"))
	fmt.Printf("  Country:     %s\n", getString(snippet, "country"))
	fmt.Printf("  Published:   %s\n", getString(snippet, "publishedAt"))
	fmt.Printf("  Subscribers: %s\n", getString(stats, "subscriberCount"))
	fmt.Printf("  Videos:      %s\n", getString(stats, "videoCount"))
	fmt.Printf("  Views:       %s\n", getString(stats, "viewCount"))

	cd := getMap(ch, "contentDetails")
	rp := getMap(cd, "relatedPlaylists")
	uploadsID := getString(rp, "uploads")
	if uploadsID != "" {
		fmt.Printf("  Uploads PL:  %s\n", uploadsID)

		// Show latest 5 videos
		plResp, err := client.GetPlaylistItems(ctx, uploadsID, 5, "")
		if err != nil {
			log.Printf("  (playlistItems.list failed: %v)", err)
			return
		}
		plItems := getItems(plResp)
		if len(plItems) > 0 {
			fmt.Println("\n=== Latest Videos ===")
			for i, item := range plItems {
				s := getMap(item, "snippet")
				cd := getMap(item, "contentDetails")
				fmt.Printf("  %d. %s\n", i+1, getString(s, "title"))
				fmt.Printf("     ID: %s | Published: %s\n",
					getString(cd, "videoId"), getString(cd, "videoPublishedAt"))
			}
		}
	}
	fmt.Println()
}

func showVideo(ctx context.Context, client *youtube.Client, id string, raw bool) {
	resp, err := client.GetVideos(ctx, id)
	if err != nil {
		log.Fatalf("videos.list: %v", err)
	}
	if raw {
		prettyPrint(resp)
	} else {
		items := getItems(resp)
		if len(items) == 0 {
			fmt.Println("Video not found.")
			return
		}
		v := items[0]
		snippet := getMap(v, "snippet")
		stats := getMap(v, "statistics")
		cd := getMap(v, "contentDetails")

		fmt.Println("=== Video ===")
		fmt.Printf("  ID:        %s\n", getString(v, "id"))
		fmt.Printf("  Title:     %s\n", getString(snippet, "title"))
		fmt.Printf("  Channel:   %s (%s)\n", getString(snippet, "channelTitle"), getString(snippet, "channelId"))
		fmt.Printf("  Published: %s\n", getString(snippet, "publishedAt"))
		fmt.Printf("  Duration:  %s\n", getString(cd, "duration"))
		fmt.Printf("  Views:     %s\n", getString(stats, "viewCount"))
		fmt.Printf("  Likes:     %s\n", getString(stats, "likeCount"))
		fmt.Printf("  Comments:  %s\n", getString(stats, "commentCount"))
	}

	// Show first 5 comments
	fmt.Println("\n=== Comments (first 5) ===")
	ctResp, err := client.GetCommentThreads(ctx, id, 5, "")
	if err != nil {
		fmt.Printf("  (commentThreads.list failed: %v)\n", err)
		fmt.Println("  Comments may be disabled for this video.")
		return
	}
	if raw {
		prettyPrint(ctResp)
		return
	}

	threads := getItems(ctResp)
	for i, thread := range threads {
		snippet := getMap(thread, "snippet")
		tlc := getMap(snippet, "topLevelComment")
		tlcSnippet := getMap(tlc, "snippet")
		replyCount := snippet["totalReplyCount"]

		fmt.Printf("  %d. @%s (likes: %s, replies: %v)\n",
			i+1,
			getString(tlcSnippet, "authorDisplayName"),
			formatNum(tlcSnippet["likeCount"]),
			formatNum(replyCount))
		text := getString(tlcSnippet, "textOriginal")
		if len(text) > 100 {
			text = text[:100] + "..."
		}
		fmt.Printf("     %s\n", text)
		fmt.Printf("     Posted: %s\n", getString(tlcSnippet, "publishedAt"))

		// Show replies if any
		replies := getMap(thread, "replies")
		if replies != nil {
			replyComments, _ := replies["comments"].([]any)
			for j, rc := range replyComments {
				rcMap, _ := rc.(map[string]any)
				rcSnippet := getMap(rcMap, "snippet")
				fmt.Printf("     └─ %d. @%s: %s\n",
					j+1,
					getString(rcSnippet, "authorDisplayName"),
					truncate(getString(rcSnippet, "textOriginal"), 80))
			}
		}
		fmt.Println()
	}
}

func runAllFormatted(ctx context.Context, client *youtube.Client, channelID string) {
	// 1. Channel info + latest videos
	showChannel(ctx, client, channelID, false)

	// 2. Get first video from the channel
	resp, err := client.GetChannel(ctx, channelID)
	if err != nil {
		log.Fatalf("channels.list: %v", err)
	}
	items := getItems(resp)
	if len(items) == 0 {
		return
	}
	cd := getMap(items[0], "contentDetails")
	rp := getMap(cd, "relatedPlaylists")
	uploadsID := getString(rp, "uploads")
	if uploadsID == "" {
		return
	}

	plResp, err := client.GetPlaylistItems(ctx, uploadsID, 1, "")
	if err != nil {
		return
	}
	plItems := getItems(plResp)
	if len(plItems) == 0 {
		return
	}
	firstVideoCD := getMap(plItems[0], "contentDetails")
	firstVideoID := getString(firstVideoCD, "videoId")

	fmt.Printf("--- Fetching video details and comments for: %s ---\n\n", firstVideoID)

	// 3. Video details + comments
	showVideo(ctx, client, firstVideoID, false)
}

func runAllRaw(ctx context.Context, client *youtube.Client, channelID string) {
	fmt.Println("========== 1. channels.list ==========")
	chResp, err := client.GetChannel(ctx, channelID)
	if err != nil {
		log.Fatalf("channels.list: %v", err)
	}
	prettyPrint(chResp)

	uploadsID := extractString(chResp, "items", 0, "contentDetails", "relatedPlaylists", "uploads")
	if uploadsID == "" {
		log.Fatal("could not extract uploads playlist ID")
	}

	fmt.Println("\n========== 2. playlistItems.list (first 5) ==========")
	plResp, err := client.GetPlaylistItems(ctx, uploadsID, 5, "")
	if err != nil {
		log.Fatalf("playlistItems.list: %v", err)
	}
	prettyPrint(plResp)

	firstVideoID := extractString(plResp, "items", 0, "contentDetails", "videoId")
	if firstVideoID == "" {
		log.Fatal("could not extract video ID")
	}

	fmt.Println("\n========== 3. videos.list ==========")
	vidResp, err := client.GetVideos(ctx, firstVideoID)
	if err != nil {
		log.Fatalf("videos.list: %v", err)
	}
	prettyPrint(vidResp)

	fmt.Println("\n========== 4. commentThreads.list (first 5) ==========")
	ctResp, err := client.GetCommentThreads(ctx, firstVideoID, 5, "")
	if err != nil {
		log.Printf("commentThreads.list: %v (comments may be disabled)", err)
	} else {
		prettyPrint(ctResp)
	}
}

// ---------- URL parsing ----------

func parseChannelArg(arg string) string {
	if arg == "" {
		return ""
	}
	// Already a channel ID
	if strings.HasPrefix(arg, "UC") && !strings.Contains(arg, "/") {
		return arg
	}
	// URL: https://www.youtube.com/@handle or /channel/UCxxx
	u, err := url.Parse(arg)
	if err != nil {
		return arg
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "channel" {
		return parts[1]
	}
	// @handle — can't resolve to channel ID without search API (100 units)
	// Return as-is and let the caller know
	if len(parts) >= 1 && strings.HasPrefix(parts[0], "@") {
		fmt.Printf("Note: handle %s detected. Use forHandle parameter.\n", parts[0])
		fmt.Printf("Looking up channel by handle...\n\n")
		return "handle:" + parts[0]
	}
	return arg
}

func parseVideoArg(arg string) string {
	if arg == "" {
		return ""
	}
	// Already a video ID (11 chars, no slash)
	if !strings.Contains(arg, "/") && !strings.Contains(arg, "?") {
		return arg
	}
	// URL: https://www.youtube.com/watch?v=xxx or https://youtu.be/xxx
	u, err := url.Parse(arg)
	if err != nil {
		return arg
	}
	// youtu.be/xxx
	if strings.Contains(u.Host, "youtu.be") {
		return strings.Trim(u.Path, "/")
	}
	// youtube.com/watch?v=xxx
	v := u.Query().Get("v")
	if v != "" {
		return v
	}
	return arg
}

// ---------- Helpers ----------

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

func formatNum(v any) string {
	switch n := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", n)
	case string:
		return n
	default:
		return fmt.Sprintf("%v", v)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func extractString(m map[string]any, keys ...any) string {
	var current any = m
	for _, key := range keys {
		switch k := key.(type) {
		case string:
			cm, ok := current.(map[string]any)
			if !ok {
				return ""
			}
			current = cm[k]
		case int:
			arr, ok := current.([]any)
			if !ok || k >= len(arr) {
				return ""
			}
			current = arr[k]
		}
	}
	s, _ := current.(string)
	return s
}

func prettyPrint(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func printUsage() {
	fmt.Println("YouTube API Test Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/yttest -channel <ID or URL>")
	fmt.Println("  go run ./cmd/yttest -video <ID or URL>")
	fmt.Println("  go run ./cmd/yttest -channel <ID or URL> -all")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./cmd/yttest -channel UCX6OQ3DkcsbYNE6H8uQQuVA")
	fmt.Println("  go run ./cmd/yttest -video https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	fmt.Println("  go run ./cmd/yttest -channel UCX6OQ3DkcsbYNE6H8uQQuVA -all")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -channel   Channel ID (UCxxx) or URL")
	fmt.Println("  -video     Video ID or URL (youtube.com/watch?v=xxx or youtu.be/xxx)")
	fmt.Println("  -all       Full pipeline: channel → latest video → comments")
	fmt.Println("  -raw       Dump raw JSON instead of formatted output")
}

func loadEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
	return s.Err()
}
