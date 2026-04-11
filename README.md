# YouTube Comment Analytics

YouTubeコメントに特化した集計・分析OSS。特定界隈のチャンネルを対象にコメントを収集・可視化する。

## 概要

YouTube Data API v3 を用いて対象チャンネルのコメントを定期収集し、集計・分析結果をWebで可視化する。

## 技術スタック

| レイヤー | 技術 | 備考 |
|---|---|---|
| フロントエンド | Next.js / Vercel | 無料枠、React |
| バックエンド | Go / Cloud Run | コスト効率、GCP |
| データベース | TiDB Cloud Starter | MySQL互換、5GB無料 |
| データ取得 | YouTube Data API v3 | 1日10,000ユニット無料 |

## アーキテクチャ

```
[Next.js (Vercel)]
        |
        v
[Go API (Cloud Run)]
        |
        v
[TiDB Cloud]
        ^
        |
[Batch Collector (Cloud Run Jobs / Scheduler)]
        |
        v
[YouTube Data API v3]
```

## 設計上の制約

- YouTube Data API は **1日10,000ユニット** が無料枠上限
- コメント取得は 1リクエストあたり **最大100件**
- クォータを超過する場合は Google に追加申請が必要（自動課金では増えない）
- バッチは夜間に回してクォータを節約する想定

## ロードマップ

- [ ] 環境構築（GCP / TiDB Cloud / YouTube Data API）
- [ ] DBスキーマ設計（channels / videos / comments）
- [ ] バッチ収集ワーカー（Go）
- [ ] 集計API（Go / Cloud Run）
- [ ] フロントエンド（Next.js）
- [ ] Vercelデプロイ
- [ ] README整備・OSS公開

## 開発

環境構築手順は [docs/setup.md](docs/setup.md) を参照。

## ライセンス

MIT
