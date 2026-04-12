package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	YouTubeAPIKey string
	DBDSN         string
	Channels      []Channel
}

type Channel struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

type channelsFile struct {
	Channels []Channel `json:"channels"`
}

// Load reads .env.local and channels.json to build the runtime config.
func Load(envPath, channelsPath string) (*Config, error) {
	if err := loadEnvFile(envPath); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY is not set")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?tls=true&parseTime=true",
		mustEnv("TIDB_USER"),
		mustEnv("TIDB_PASSWORD"),
		mustEnv("TIDB_HOST"),
		mustEnv("TIDB_PORT"),
		mustEnv("TIDB_DATABASE"),
	)

	channels, err := loadChannels(channelsPath)
	if err != nil {
		return nil, fmt.Errorf("load channels: %w", err)
	}

	return &Config{
		YouTubeAPIKey: apiKey,
		DBDSN:         dsn,
		Channels:      channels,
	}, nil
}

func loadChannels(path string) ([]Channel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f channelsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Channels, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("env %s is not set", key))
	}
	return v
}

func loadEnvFile(path string) error {
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
