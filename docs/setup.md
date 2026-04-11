# 環境構築ガイド

このドキュメントは、開発環境とクラウドサービスのセットアップ手順をまとめたものです。

---

## 0. ローカルツール

| ツール | 確認コマンド | インストール先 |
|---|---|---|
| Go 1.26+ | `go version` | https://go.dev/dl/ |
| Node.js 20+ | `node --version` | https://nodejs.org/ |
| Git | `git --version` | https://git-scm.com/ |
| Docker | `docker --version` | https://www.docker.com/ |
| gh CLI | `gh --version` | https://cli.github.com/ |
| gcloud CLI | `gcloud --version` | https://cloud.google.com/sdk/docs/install |

> 💡 Windowsでは `winget install <ID>` でまとめて入る:
> - `GoLang.Go`
> - `OpenJS.NodeJS.LTS`
> - `Git.Git`
> - `Docker.DockerDesktop`
> - `GitHub.cli`
> - `Google.CloudSDK`

---

## 1. GCP プロジェクト + YouTube Data API v3

YouTubeコメント収集に必須。**無料枠: 1日10,000ユニット**。

### 1-1. GCPプロジェクト作成

1. GCPコンソールにアクセス
   👉 https://console.cloud.google.com/
2. 上部のプロジェクト選択 → **「新しいプロジェクト」**
3. プロジェクト名: `youtube-comment-analytics`（任意）
4. 課金アカウント: **不要**（YouTube Data APIは無料枠で動く）
5. 作成 → プロジェクトID をメモ（例: `youtube-comment-analytics-12345`）

### 1-2. YouTube Data API v3 を有効化

1. APIライブラリにアクセス
   👉 https://console.cloud.google.com/apis/library/youtube.googleapis.com
2. 上部のプロジェクトが上で作ったものになっているか確認
3. **「有効にする」** をクリック

### 1-3. APIキーを発行

1. 認証情報ページ
   👉 https://console.cloud.google.com/apis/credentials
2. **「+ 認証情報を作成」 → 「APIキー」**
3. 発行されたキーをコピー
4. **「キーを制限」** をクリックして以下を設定:
   - **API の制限**: 「キーを制限」→ `YouTube Data API v3` のみ選択
   - **アプリケーションの制限**: いったん「なし」（後でIPで絞ってもよい）
5. 保存

### 1-4. ローカルへ保存

プロジェクトルートに `.env.local` を作成:

```env
YOUTUBE_API_KEY=AIzaSy...（コピーしたキー）
```

> ⚠️ `.env.local` は `.gitignore` 済み。絶対にコミットしないこと。

### 1-5. クォータの目安

| 操作 | 消費ユニット |
|---|---|
| `commentThreads.list`（1リクエスト=最大100件） | 1 |
| `videos.list` | 1 |
| `channels.list` | 1 |
| `search.list` | **100** ⚠️ |

→ `search.list` は高コスト。可能な限りキャッシュ。

クォータ確認:
👉 https://console.cloud.google.com/iam-admin/quotas

---

## 2. TiDB Cloud Starter

MySQL互換のサーバーレスDB。**無料枠: 5GB / 1クラスタ**。

### 2-1. アカウント作成

1. TiDB Cloud にアクセス
   👉 https://tidbcloud.com/
2. 「Sign up for free」 → GitHub または Google でログイン
3. メール認証を完了

### 2-2. クラスタ作成

1. ダッシュボード → **「Create Cluster」**
2. プラン: **Starter**（無料）を選択
3. 設定:
   - **Cloud Provider**: AWS（推奨）
   - **Region**: `Tokyo (ap-northeast-1)` または近いリージョン
   - **Cluster Name**: `youtube-comment-analytics`
4. **「Create」** をクリック（数十秒で起動）

### 2-3. 接続情報の取得

1. クラスタ一覧 → 作成したクラスタを開く
2. 右上の **「Connect」** をクリック
3. **「General」** タブで:
   - Connect With: **MySQL CLI** または **General**
   - Endpoint Type: **Public**
   - 初回は **「Generate Password」** でパスワード生成 → 必ずメモ

接続情報の例:
```
Host:     gateway01.ap-northeast-1.prod.aws.tidbcloud.com
Port:     4000
User:     xxxxxxxxxxxx.root
Password: （生成されたもの）
Database: test
```

### 2-4. ローカルへ保存

`.env.local` に追記:

```env
TIDB_HOST=gateway01.ap-northeast-1.prod.aws.tidbcloud.com
TIDB_PORT=4000
TIDB_USER=xxxxxxxxxxxx.root
TIDB_PASSWORD=（生成パスワード）
TIDB_DATABASE=youtube_analytics
TIDB_TLS=true
```

### 2-5. 接続テスト（Goから）

```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

dsn := fmt.Sprintf(
    "%s:%s@tcp(%s:%s)/%s?tls=true",
    user, pass, host, port, db,
)
db, err := sql.Open("mysql", dsn)
```

> ⚠️ TiDB Cloud Starter は **TLS必須**。`tls=true` パラメータを忘れずに。

---

## 3. GitHub リポジトリ

すでに `gh repo create` で作成済みの場合はスキップ。

```bash
gh auth login                                    # 初回のみ
gh repo create youtube-comment-analytics --public --source=. --remote=origin --push
```

---

## 4. Vercel（フロントデプロイ用、後回しでOK）

1. https://vercel.com/ にアクセス
2. GitHubでサインアップ
3. リポジトリ連携 → 自動デプロイ設定

---

## 5. Cloud Run（バックエンドデプロイ用、後回しでOK）

1. GCPコンソール → Cloud Run
2. APIを有効化
3. `gcloud run deploy` でデプロイ
4. 詳細は本実装フェーズで追記

---

## チェックリスト

- [ ] Goがインストール済み
- [ ] Node.jsがインストール済み
- [ ] GitHubリポジトリ作成済み
- [ ] GCPプロジェクト作成済み
- [ ] YouTube Data API v3 有効化済み
- [ ] APIキー発行済み（`.env.local`に保存）
- [ ] TiDB Cloud Starter クラスタ作成済み
- [ ] TiDB接続情報を `.env.local` に保存済み
- [ ] `.env.local` がコミットされていない（git status で確認）
