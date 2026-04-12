# データモデル設計（ドラフト）

このドキュメントは2部構成。

1. **第1部: YouTube Data API v3 の取得内容** — 事実ベース（API仕様）
2. **第2部: DBスキーマ設計** — 上記から導いた判断（議論の余地あり）

> ⚠️ ドラフト。フィールド名・型は実装時に必ず公式ドキュメントで再確認すること。
> 公式: https://developers.google.com/youtube/v3/docs

---

# 第1部: YouTube Data API v3 で取得できる内容

## 使うエンドポイント

| エンドポイント | 用途 | クォータ |
|---|---|---|
| `channels.list` | チャンネルのメタデータ取得 | 1 |
| `playlistItems.list` | チャンネルのアップロード動画一覧 | 1 |
| `videos.list` | 動画のメタデータ・統計 | 1 |
| `commentThreads.list` | 動画のトップレベルコメント+返信 | 1 |
| `comments.list` | 特定スレッドの全返信取得 | 1 |
| `search.list` | ⚠️ 100ユニット、極力使わない | 100 |

### `search.list` を避ける戦略
- チャンネルIDは**手動で集めて設定ファイルに書く**（または管理画面で登録）
- 動画一覧は `channels.list → contentDetails.relatedPlaylists.uploads → playlistItems.list` で取得（合計2ユニット）

---

## エンドポイント別レスポンス（重要フィールド抜粋）

### 1. `channels.list`

```http
GET /youtube/v3/channels?part=snippet,statistics,contentDetails&id=UCxxx
```

```json
{
  "items": [{
    "id": "UCxxxxxxxxxxxx",
    "snippet": {
      "title": "チャンネル名",
      "description": "...",
      "customUrl": "@handle",
      "publishedAt": "2020-01-01T00:00:00Z",
      "thumbnails": { "default": { "url": "..." }, "high": { "url": "..." } },
      "country": "JP"
    },
    "statistics": {
      "viewCount": "1234567",
      "subscriberCount": "10000",
      "videoCount": "120"
    },
    "contentDetails": {
      "relatedPlaylists": {
        "uploads": "UUxxxxxxxxxxxx"
      }
    }
  }]
}
```

**保存したいフィールド**: `id`, `title`, `description`, `customUrl`, `publishedAt`, `thumbnails.high.url`, `country`, `subscriberCount`, `videoCount`, `viewCount`, `uploadsPlaylistId`

---

### 2. `playlistItems.list`（チャンネルのアップロード一覧用）

```http
GET /youtube/v3/playlistItems?part=snippet,contentDetails&playlistId=UUxxx&maxResults=50
```

```json
{
  "items": [{
    "snippet": {
      "publishedAt": "...",
      "channelId": "UCxxx",
      "title": "...",
      "resourceId": { "videoId": "abc123" }
    },
    "contentDetails": {
      "videoId": "abc123",
      "videoPublishedAt": "..."
    }
  }],
  "nextPageToken": "..."
}
```

**保存したいフィールド**: `videoId`（リスト取得が目的、詳細は次の `videos.list` で）

---

### 3. `videos.list`

```http
GET /youtube/v3/videos?part=snippet,statistics,contentDetails,liveStreamingDetails&id=abc,def
```

```json
{
  "items": [{
    "id": "abc123",
    "snippet": {
      "publishedAt": "2026-04-01T12:00:00Z",
      "channelId": "UCxxx",
      "title": "動画タイトル",
      "description": "...",
      "thumbnails": { "high": { "url": "..." } },
      "tags": ["tag1", "tag2"],
      "categoryId": "20",
      "defaultLanguage": "ja",
      "liveBroadcastContent": "none"
    },
    "statistics": {
      "viewCount": "12345",
      "likeCount": "678",
      "commentCount": "90"
    },
    "contentDetails": {
      "duration": "PT3M21S"
    },
    "liveStreamingDetails": {
      "actualStartTime": "...",
      "actualEndTime": "...",
      "scheduledStartTime": "..."
    }
  }]
}
```

**保存したいフィールド**: `id`, `channelId`, `title`, `description`, `publishedAt`, `thumbnails.high.url`, `tags`, `categoryId`, `defaultLanguage`, `duration` (秒に変換), `viewCount`, `likeCount`, `commentCount`, `liveStreamingDetails`（あれば）

---

### 4. `commentThreads.list`

```http
GET /youtube/v3/commentThreads?part=snippet,replies&videoId=abc&maxResults=100
```

```json
{
  "items": [{
    "id": "Ugy...",
    "snippet": {
      "channelId": "UCxxx",
      "videoId": "abc123",
      "topLevelComment": {
        "id": "Ugy...",
        "snippet": {
          "authorDisplayName": "ユーザー名",
          "authorProfileImageUrl": "...",
          "authorChannelUrl": "http://www.youtube.com/channel/UCyyy",
          "authorChannelId": { "value": "UCyyy" },
          "textDisplay": "コメント本文（HTML）",
          "textOriginal": "コメント本文（プレーン）",
          "likeCount": 5,
          "publishedAt": "2026-04-01T13:00:00Z",
          "updatedAt": "2026-04-01T13:00:00Z"
        }
      },
      "totalReplyCount": 2,
      "isPublic": true
    },
    "replies": {
      "comments": [{
        "id": "Ugy...reply1",
        "snippet": {
          "authorDisplayName": "...",
          "authorChannelId": { "value": "UCzzz" },
          "textOriginal": "返信本文",
          "parentId": "Ugy...",
          "likeCount": 1,
          "publishedAt": "..."
        }
      }]
    }
  }],
  "nextPageToken": "..."
}
```

**保存したいフィールド**: コメントごとに `id`, `videoId`, `parentId`(返信の場合), `authorDisplayName`, `authorChannelId`, `textOriginal`, `likeCount`, `publishedAt`, `updatedAt`

**注意点**:
- `replies` は **5件まで** しか入らない。それ以上の返信が必要なら `comments.list?parentId=` で別途取得（追加クォータ）
- 取得できるのは**公開コメントのみ**。投稿者がコメント無効化していると `commentsDisabled` エラー

---

# 第2部: DBスキーマ設計（議論用ドラフト）

## 設計判断の論点

実装前に以下を決める必要があります。**それぞれ私の推奨を書きますが、議論してください。**

### 論点A: 主キーをYouTube IDにするか、サロゲート主キーにするか
- **推奨: YouTube IDをそのままPK**（`channel_id VARCHAR(32) PRIMARY KEY` 等）
- 理由: 自然キーが既に存在しユニーク、INSERT時の重複検出が `INSERT IGNORE` で済む、JOINも見やすい
- リスク: YouTube IDの上限長を仮定する必要がある（実際は最大でも30文字程度）

### 論点B: 著者（authors）を別テーブルに切り出すか、コメントに埋め込むか
- **採用: 別テーブル `authors` に切り出す**
- 検討した代替案: `comments` / `replies` に `author_display_name` 等を直接埋め込み

#### メリット（分離を選んだ理由）
1. **データ整合性**: 同一人物が100件コメントしたとき、名前変更が発生しても `authors.display_name` を1箇所更新するだけ。埋め込みだと100行に旧名・新名が混在し名寄せ不能になる
2. **検索性能**: 「この人の全コメント+全返信」を `WHERE author_id = 'UCxxx'` で取れる。IDベースなのでインデックスが確実に効き、名前変更にも影響しない。埋め込みだと名前文字列での検索になり、変更後に取りこぼす
3. **表示名の使い分け**: `authors.display_name`（常に最新）と `comments.author_display_name_at_post`（投稿時スナップショット）の両方を持てる。一覧では最新名、履歴では当時の名前、と用途で切り替え可能
4. **集計の起点になる**: 「投稿数ランキング」「特定ユーザーの活動履歴」など著者起点の分析がJOIN一発で書ける

#### デメリット（トレードオフ）
1. **テーブル数が増える**: スキーマが1テーブル分増える
2. **JOINが必要**: コメント表示時に `authors` へのJOINが発生する
3. **UPSERT処理**: バッチ収集時に `authors` への INSERT or UPDATE が毎回必要

#### 判断根拠
- デメリットはいずれも軽い。`authors` の行数はユニーク投稿者数（200万コメントでも数万〜数十万程度）なので、JOINもUPSERTも負荷は小さい
- コメント分析サイトという性質上、「誰が何件コメントしているか」「特定の人のコメント一覧」は確実にやるユースケース。埋め込みだとこれらが困難になる
- 埋め込みが有利なのは「著者で検索・集計する予定が一切ない」場合のみ。本プロジェクトでは該当しない

### 論点C: 返信コメントを別テーブルにするか、`comments` の自己参照にするか
- **採用: `replies` テーブルに分離**
- 検討した代替案: `comments` テーブルで `parent_comment_id` による自己参照

#### メリット（分離を選んだ理由）
1. **クエリが明快**: トップレベルコメント一覧に `WHERE parent_comment_id IS NULL` が不要。テーブルを分けるだけで意図が明確
2. **テーブルサイズの分離**: 返信はコメントと同量〜数倍になりうる。テーブルが小さいほどインデックス効率が良い
3. **独立した拡張性**: 返信だけに付けたいカラム（`is_creator_reply` 等）をコメントテーブルを汚さず追加可能
4. **インデックス設計の自由度**: `video_id` を denormalize して直接持てば、JOINなしで「動画の全返信」を引ける

#### デメリット（トレードオフ）
1. **テーブル数が増える**: スキーマの見通しがやや複雑になる
2. **UNION が必要な場面**: 「この動画のコメント＋返信すべて」を1クエリで取るには `UNION ALL`
3. **INSERT先の分岐**: バッチ収集時に「コメントか返信か」の判定ロジックが必要（`parentId` の有無で判定するだけなので軽い）
4. **video_id の二重管理**: `comments` と `replies` 両方に `video_id` を持つ（denormalize のコスト）

#### 判断根拠
- YouTubeの返信は**最大500件/スレッド、ネスト深さは1**（返信への返信はない）。自己参照のメリットである再帰的ツリーが不要
- 将来のスケール（数百万〜数千万コメント）を見据えると、テーブルを分けておく方がパフォーマンス面で有利
- UNION が必要なクエリは限定的で、大半は「コメント一覧」「返信一覧」を独立で取る

### 論点D: 集計はDBで都度計算 vs 集計テーブルを別途持つ
- **推奨: 最初は都度SQL、必要になったら `daily_video_stats` のような集計テーブルを追加**
- 理由: TiDBはOLAP向きの分散SQL DBなので集計クエリが速い、最適化はあとから

### 論点E: 生APIレスポンスのJSONを保存するか
- **推奨: 当面しない**
- 理由: ストレージ消費が読めない、必要なときは再取得すればいい
- ただし `videos.tags` のような配列は JSON カラムで持つのが楽（TiDBはJSON型対応）

### 論点F: ユーザー（このシステムを使う人）テーブルをどう扱うか
- **推奨: テーブルは作らない、ただし将来追加しやすい構造にしておく**
- 「収集対象チャンネルをユーザーごとに持つ」「ログイン後に分析履歴を保存」などの機能が出てきたら追加
- 認証は OAuth (Google / GitHub) を想定しておけば、`users(id, provider, provider_user_id, ...)` の形になる

---

## スキーマ案（ドラフト）

```sql
-- =====================================================================
-- channels: 収集対象チャンネル
-- =====================================================================
CREATE TABLE channels (
  id                   VARCHAR(32)   PRIMARY KEY,            -- YouTube channel ID (UCxxx...)
  title                VARCHAR(255)  NOT NULL,
  description          TEXT          NULL,
  custom_url           VARCHAR(128)  NULL,
  country              VARCHAR(8)    NULL,
  thumbnail_url        VARCHAR(512)  NULL,
  uploads_playlist_id  VARCHAR(32)   NULL,
  subscriber_count     BIGINT        NULL,
  video_count          INT           NULL,
  view_count           BIGINT        NULL,
  channel_published_at DATETIME      NULL,
  -- メタデータ
  first_seen_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_fetched_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  is_active            BOOLEAN       NOT NULL DEFAULT TRUE   -- 収集対象から外したらFALSE
);

-- =====================================================================
-- videos: 動画
-- =====================================================================
CREATE TABLE videos (
  id                   VARCHAR(16)   PRIMARY KEY,            -- YouTube video ID
  channel_id           VARCHAR(32)   NOT NULL,
  title                VARCHAR(512)  NOT NULL,
  description          MEDIUMTEXT    NULL,
  thumbnail_url        VARCHAR(512)  NULL,
  category_id          INT           NULL,
  default_language     VARCHAR(16)   NULL,
  duration_seconds     INT           NULL,                   -- ISO8601をパースして秒に
  tags                 JSON          NULL,                   -- ["tag1","tag2"]
  view_count           BIGINT        NULL,
  like_count           BIGINT        NULL,
  comment_count        BIGINT        NULL,
  is_live_archive      BOOLEAN       NOT NULL DEFAULT FALSE,
  live_actual_start_at DATETIME      NULL,
  live_actual_end_at   DATETIME      NULL,
  video_published_at   DATETIME      NOT NULL,
  first_seen_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_fetched_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  comments_disabled    BOOLEAN       NOT NULL DEFAULT FALSE,
  INDEX idx_channel_published (channel_id, video_published_at DESC),
  INDEX idx_published (video_published_at DESC)
);

-- =====================================================================
-- authors: コメント投稿者（YouTubeチャンネルID単位で正規化）
-- =====================================================================
CREATE TABLE authors (
  id                   VARCHAR(32)   PRIMARY KEY,            -- YouTube channel ID of the commenter
  display_name         VARCHAR(255)  NOT NULL,               -- 最終観測時の表示名
  profile_image_url    VARCHAR(512)  NULL,
  first_seen_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================================
-- comments: トップレベルコメント（返信は replies テーブルに分離）
-- =====================================================================
CREATE TABLE comments (
  id                          VARCHAR(64)   PRIMARY KEY,     -- YouTube comment ID
  video_id                    VARCHAR(16)   NOT NULL,
  author_id                   VARCHAR(32)   NULL,             -- authors.id（匿名コメントもありうるので NULL 可）
  author_display_name_at_post VARCHAR(255)  NOT NULL,         -- スナップショット（後で名前変わっても残す）
  text_original               MEDIUMTEXT    NOT NULL,
  like_count                  INT           NOT NULL DEFAULT 0,
  reply_count                 INT           NOT NULL DEFAULT 0, -- API の totalReplyCount
  comment_published_at        DATETIME      NOT NULL,
  comment_updated_at          DATETIME      NOT NULL,
  fetched_at                  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_video_published (video_id, comment_published_at DESC),
  INDEX idx_author (author_id)
);

-- =====================================================================
-- replies: 返信コメント（comments から分離、video_id を denormalize）
-- =====================================================================
CREATE TABLE replies (
  id                          VARCHAR(64)   PRIMARY KEY,     -- YouTube comment ID (reply)
  parent_comment_id           VARCHAR(64)   NOT NULL,        -- comments.id への参照
  video_id                    VARCHAR(16)   NOT NULL,        -- denormalized（JOINなしで動画単位検索）
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

-- =====================================================================
-- ingest_runs: バッチ収集の実行ログ（クォータ追跡 + デバッグ用）
-- =====================================================================
CREATE TABLE ingest_runs (
  id              BIGINT        AUTO_INCREMENT PRIMARY KEY,
  started_at      DATETIME      NOT NULL,
  finished_at     DATETIME      NULL,
  status          VARCHAR(16)   NOT NULL,                    -- 'running','success','failed'
  channels_count  INT           NOT NULL DEFAULT 0,
  videos_count    INT           NOT NULL DEFAULT 0,
  comments_count  INT           NOT NULL DEFAULT 0,
  quota_used      INT           NOT NULL DEFAULT 0,
  error_message   TEXT          NULL
);
```

---

## 設計の意図・メモ

- **`comments.id` / `replies.id` の長さ**: YouTube comment ID は実測で40〜50文字。余裕を見て VARCHAR(64)
- **`videos.id` の長さ**: YouTube video ID は11文字固定。VARCHAR(16) は余裕
- **`channels.id` の長さ**: 24文字（`UC` + 22文字）。VARCHAR(32)
- **`replies.video_id` の denormalize**: 正規化なら `comments` 経由でJOINだが、返信が増えるとJOINコストが大きい。video_id を両テーブルに持たせることで「動画の全返信」をインデックス一発で取得可能
- **外部キー制約は張らない**: TiDB Cloud Starter ではFK制約のサポートが限定的（バージョンによる）。アプリ層でJOIN整合性を担保する
- **`fetched_at` / `last_fetched_at` を全テーブルに**: 増分更新の判定に必須
- **`first_seen_at` を持つ**: 「いつから観測してるか」の質問に答えられる
- **`is_active` フラグ**: 収集対象から外す論理削除（物理削除しない方が分析に便利）

---

## 将来拡張の余地

- `users` テーブル: 収集対象をユーザー単位で管理する場合（OAuth前提）
- `user_channels` テーブル: 「どのユーザーがどのチャンネルを登録したか」のM:N
- `daily_video_stats`: 日次集計のサマリ（クエリが遅くなったら）
- `comment_sentiment`: 感情分析結果（NLPを足すなら）
- `comment_topics`: トピックモデリング結果

---

## マイグレーション戦略

- ツール: **`golang-migrate`** または **`pressly/goose`**（Goエコシステム標準）
- ファイル配置: `db/migrations/0001_init.up.sql` / `0001_init.down.sql`
- 実行: バッチワーカーの起動時に最新マイグレーションを適用 + CIで `migrate up` を強制実行

> どっちのツールにするかは実装フェーズで決める。個人的には goose の方がGoバイナリに埋め込みやすい
