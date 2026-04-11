// Package main provides a TiDB connection smoke test.
//
// Reads connection info from .env.local in the project root and runs:
//   - Open
//   - Ping
//   - SELECT VERSION()
//   - SHOW DATABASES (first few rows)
//
// Usage: go run ./cmd/dbtest
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func main() {
	if err := loadEnv(".env.local"); err != nil {
		log.Fatalf("load .env.local: %v", err)
	}

	cfg := mysql.NewConfig()
	cfg.User = mustEnv("TIDB_USER")
	cfg.Passwd = mustEnv("TIDB_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%s", mustEnv("TIDB_HOST"), mustEnv("TIDB_PORT"))
	cfg.DBName = mustEnv("TIDB_DATABASE")
	cfg.TLSConfig = "true"
	cfg.ParseTime = true
	cfg.Timeout = 10 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping: %v", err)
	}
	fmt.Println("✅ Ping OK")

	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		log.Fatalf("select version: %v", err)
	}
	fmt.Printf("✅ Server version: %s\n", version)

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		log.Fatalf("show databases: %v", err)
	}
	defer rows.Close()

	fmt.Println("✅ Databases:")
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatalf("scan: %v", err)
		}
		fmt.Printf("  - %s\n", name)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}
}

// loadEnv reads a simple KEY=VALUE file (one entry per line, # comments).
// Lines that already exist in the process environment are not overwritten.
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

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("env %s is not set", key)
	}
	return v
}
