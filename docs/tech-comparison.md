# 技術選定 比較メモ

このプロジェクトで使うインフラ／マネージドサービスの選定理由と、検討した代替案を記録する。
**言語・フレームワーク（Go, Next.js 等）は対象外。**

判断軸はこの順番：

1. **無料枠で動くか**（個人開発・ポートフォリオ前提）
2. **将来の転職ポートフォリオで語れる技術か**（GCP / Cloud Run などスタンダードなもの）
3. **学習コストと運用負荷**

> ⚠️ 価格や無料枠は変動する。最新は必ず公式を確認すること。本ドキュメントは 2026-04 時点の調査結果。

---

## 1. フロントエンドホスティング

### 採用: **Vercel**

**理由**:
- Next.js の作者企業が運用しているので親和性が最高（ISR/SSG/Edge Functionsがゼロ設定）
- Hobbyプランが無料で個人開発に十分
- GitHub連携でPushすれば自動デプロイ、PRごとにPreview URL

### 比較表

| サービス | 無料枠 | Next.js親和性 | 強み | 弱み |
|---|---|---|---|---|
| **Vercel** ✅ | Hobby無料（100GB帯域/月、Functionsあり） | ★★★ | Next.js最適化、Edge Network、Preview URL | 商用利用は$20/月〜、Functions実行時間に制約 |
| Netlify | 無料（100GB帯域/月） | ★★ | Forms/Identityなど周辺機能、Hugoなど静的にも強い | Next.jsはVercelに一歩劣る |
| Cloudflare Pages | 無料（**帯域無制限**、ビルド500回/月） | ★★ | 帯域無制限、Workers連携、グローバルCDN | Next.js SSR/ISRに制約あり（Edge Runtime限定） |
| Railway | $5/月〜（無料枠は試用のみ） | ★ | フロント+API+DBを1箇所で、シンプル課金 | 無料枠なし、エッジ配信なし、Next.js最適化なし |
| AWS Amplify | 無料枠あり（15GB帯域/月） | ★★ | AWS統合、Cognitoなど認証統合 | 設定が複雑、Vercelより遅い |

### Railway を選ばなかった理由
- フロント専用なら Vercel の方がNext.jsで圧倒的に楽（ISR/Edge/Preview全部ゼロ設定）
- Railwayは「フロント+バック+DBを1箇所で」が強みだが、本プロジェクトはバックを Cloud Run、DB を TiDB に寄せているのでメリット薄
- 無料枠がない（$5/月の課金前提）

### 切り替えを検討するタイミング
- Vercel の Functions 実行時間制限（Hobby は10秒）に引っかかる重いSSRが必要になったとき
- 「フロント+バック+DBの全部入り構成」に方針転換したとき（Railwayが選択肢に上がる）
- 帯域コストが Vercel で問題になったとき → Cloudflare Pages

---

## 2. バックエンドホスティング

### 採用: **Google Cloud Run**

**理由**:
- リクエスト時のみ課金（アイドル時 \$0）→ 個人開発に最適
- 無料枠が広い（毎月200万リクエスト + 360,000 GB-秒のメモリ + 180,000 vCPU-秒）
- Goとの相性が良い（コンテナ起動が速い）

### 比較表

| サービス | 無料枠 | スケール | 強み | 弱み |
|---|---|---|---|---|
| **Cloud Run** ✅ | 毎月200万リクエスト無料 | 0→N自動 | GCP統合、コールドスタート速い、ポートフォリオ価値 | コールドスタートあり、リージョン制約 |
| Railway | $5/月〜 | 常時起動 | 設定がシンプル、UI良い、フロント+DBも同居可 | 常時起動課金、無料枠なし、スケールアウトに制約 |
| Render | 無料あり（750時間/月、ただしsleepあり） | 手動or自動 | UI良い、Postgres同梱で手軽 | 無料枠は15分sleep、寒いと遅い |
| Fly.io | 無料あり（共有CPU 3VM） | リージョン分散 | エッジ配信、低レイテンシ、グローバル | 設定が独自、UIは最小限 |
| AWS App Runner | 無料枠なし | 自動 | AWS統合 | 高い、設定が複雑 |
| Heroku | 無料枠廃止 | 手動 | 老舗、ドキュメント豊富 | 無料枠廃止後は割高 |

### Railway を選ばなかった理由
- バッチ収集ワーカーがメイン用途で、**24時間動かす必要がない**（夜間1回でOK）
- リクエスト課金の Cloud Run の方が圧倒的に安い（おそらく月$0で済む）

### 切り替えを検討するタイミング
- コールドスタートが致命的になったとき → Render の always-on または Railway
- AWS スタックに統一したくなったとき → ECS Fargate or App Runner
- リアルタイム処理（WebSocketなど）が必要になったとき → Fly.io

---

## 3. データベース

### 採用: **TiDB Cloud Starter**

**理由**:
- **5GB の無料枠**（200万コメント想定でも余裕）
- **MySQL互換**で `go-sql-driver/mysql` でそのまま接続可能
- 分散SQL DBという設計を学べる（HTAPで分析も得意）
- スケールが必要になったときの上限が高い（PB級）

### 比較表

| サービス | 無料枠 | 互換性 | 強み | 弱み |
|---|---|---|---|---|
| **TiDB Cloud Starter** ✅ | 5GB / 5万RU/日 | MySQL 8.0互換 | 分散SQL、無料枠広い、HTAPで分析も得意 | 知名度が低い、学習リソース少なめ |
| Neon | 無料枠あり（0.5GB、Computeは300時間/月） | PostgreSQL | ブランチング、サーバーレス、コールドスタート速い | 無料枠が小さい、PostgreSQL前提 |
| Supabase | 無料枠あり（500MB DB + Auth + Storage） | PostgreSQL | Auth/Storage/Realtime統合、BaaSとして万能 | 無料枠が小さい、機能が広すぎてDBに集中しづらい |
| Cloud SQL (GCP) | **無料枠なし**（最小$10/月〜） | MySQL/PostgreSQL | GCP統合、Cloud Runと同VPC | 無料枠なし、スケールに弱い |
| Turso | 無料枠あり（9GB） | SQLite互換 | エッジに分散配置、低レイテンシ | SQLite前提、複雑なSQLに弱い |

### Supabase を選ばなかった理由
- PostgreSQLベースで悪くないが、**500MBの無料枠が200万コメントには足りない**（コメント1件あたり数KBとして概算）
- Auth/Storage機能が今回不要

### Neon を選ばなかった理由
- 無料枠が0.5GBと小さい
- PostgreSQLの分析機能はTiDBの方が強い（HTAP）

### 切り替えを検討するタイミング
- TiDB Cloudの料金体系が変わったとき → Supabase または Neon に縮小して逃げる
- フルマネージドの認証が必要になったとき → Supabase 併用
- エッジで読み取りが必要になったとき → Turso 併用

---

## 4. データソース

### 採用: **YouTube Data API v3**

**代替なし。**YouTubeの公式コメント取得APIはこれ一択。

### 制約のメモ

| 項目 | 制約 |
|---|---|
| 無料クォータ | **1日10,000ユニット** |
| `commentThreads.list` | 1ユニット / リクエスト（最大100件取得） |
| `videos.list` | 1ユニット |
| `search.list` | **100ユニット** ⚠️（高コスト、極力避ける） |
| クォータ追加 | Google申請が必要（自動課金では増えない） |

### 設計上の影響
- バッチ収集は **夜間まとめて** が前提
- `search.list` を多用すると即上限に達するので、**チャンネルIDは事前に手動で集める**
- コメント取得は `commentThreads.list` のページネーションで網羅

### 非公式代替（参考、今回は使わない）
- スクレイピング（yt-dlp 等）→ 利用規約違反のリスクあり、ポートフォリオでは選ばない
- サードパーティAPI（SocialBlade等）→ 高額、データの粒度が不足

---

## 5. CI/CD

### 採用: **GitHub Actions**

**理由**:
- リポジトリと同じGitHub内で完結
- パブリックリポジトリは無料・無制限
- マーケットプレイスのActionsが豊富

### 比較表（簡易）

| サービス | 無料枠 | 強み | 弱み |
|---|---|---|---|
| **GitHub Actions** ✅ | パブリック無制限、プライベートは2,000分/月 | リポジトリ統合、Actions豊富 | プライベートだと制限あり |
| CircleCI | 無料あり（6,000分/月、ただし制約多） | 並列実行が速い | UIが古い、設定がやや複雑 |
| Cloud Build (GCP) | 120分/日無料 | GCPと統合、Cloud Runデプロイがスムーズ | GitHub Actionsより独自色強い |

将来 Cloud Run へのデプロイを GitHub Actions から行う構成で問題なし。Cloud Build は使わない予定。

---

## まとめ

| レイヤー | 採用 | 主な代替候補 |
|---|---|---|
| フロント | Vercel | Cloudflare Pages |
| バック | Cloud Run | Render / Fly.io |
| DB | TiDB Cloud Starter | Supabase / Neon |
| データ取得 | YouTube Data API v3 | （代替なし） |
| CI/CD | GitHub Actions | Cloud Build |

すべて **「無料枠で動く」+「ポートフォリオで説明できる」** を基準に選定。
方針が変わったら本ドキュメントを更新する。
