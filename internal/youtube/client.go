package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://www.googleapis.com/youtube/v3"

// Client wraps the YouTube Data API v3.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a YouTube API client with the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// get performs a GET request and returns the raw JSON as a map.
func (c *Client) get(ctx context.Context, endpoint string, params url.Values) (map[string]any, error) {
	params.Set("key", c.apiKey)
	reqURL := fmt.Sprintf("%s/%s?%s", baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result, nil
}

// GetChannel fetches a channel by ID or handle.
// If channelID starts with "handle:@", it uses the forHandle parameter.
func (c *Client) GetChannel(ctx context.Context, channelID string) (map[string]any, error) {
	params := url.Values{
		"part": {"snippet,statistics,contentDetails"},
	}
	if strings.HasPrefix(channelID, "handle:") {
		handle := strings.TrimPrefix(channelID, "handle:")
		handle = strings.TrimPrefix(handle, "@")
		params.Set("forHandle", handle)
	} else {
		params.Set("id", channelID)
	}
	return c.get(ctx, "channels", params)
}

// GetPlaylistItems fetches videos from a playlist (typically the uploads playlist).
func (c *Client) GetPlaylistItems(ctx context.Context, playlistID string, maxResults int, pageToken string) (map[string]any, error) {
	params := url.Values{
		"part":       {"snippet,contentDetails"},
		"playlistId": {playlistID},
		"maxResults": {fmt.Sprintf("%d", maxResults)},
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	return c.get(ctx, "playlistItems", params)
}

// GetVideos fetches video details by IDs (comma-separated, max 50).
func (c *Client) GetVideos(ctx context.Context, videoIDs string) (map[string]any, error) {
	params := url.Values{
		"part": {"snippet,statistics,contentDetails,liveStreamingDetails"},
		"id":   {videoIDs},
	}
	return c.get(ctx, "videos", params)
}

// GetCommentThreads fetches top-level comments for a video.
func (c *Client) GetCommentThreads(ctx context.Context, videoID string, maxResults int, pageToken string) (map[string]any, error) {
	params := url.Values{
		"part":       {"snippet,replies"},
		"videoId":    {videoID},
		"maxResults": {fmt.Sprintf("%d", maxResults)},
		"order":      {"relevance"},
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	return c.get(ctx, "commentThreads", params)
}
