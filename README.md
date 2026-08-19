# pf-reliability

P12 reliability-platform の製品リポジトリです。**学習用であり、本番インシデント管理や自動修復の置き換えではありません。** 訓練もアラート受信も **仮想メトリクス / 仮想インシデント** に閉じます。クラスタへの診断コマンド、自動 rollback、本番 EC の操作は実装していません。

いまのスライスは **インシデント CRUD + Ack/Resolve + タイムライン + HMAC Webhook（dedup）** です。オンコール、ランブック編集、訓練セッション採点は未着手です。

## 構成

| パス | 役割 |
| --- | --- |
| `apps/api` | Go API。サービスマスタ、インシデント、Webhook 受信 |
| `apps/web` | Next.js。インシデントボードと仮想メトリクス表示 |
| `packages/scenario` | bad-deploy シナリオの純関数。破壊的 I/O なし |
| `packages/openapi` | OpenAPI 3 |
| `deploy/` | 単体 Compose（API + Web。メモリストア） |

認証は `X-Dev-User-Sub`（P01 OIDC は未配線）。Webhook は HMAC-SHA256（`X-Signature-256: sha256=<hex>`）。統合シークレットは API 応答でマスクします。

状態機械: `triggered` → `acknowledged` → `resolved`（triggered から直接 resolve 可。resolved への ack は 409）。

## 単体デモ

```powershell
cd deploy
copy .env.example .env
docker compose up -d --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:3012 | インシデントボード |
| http://localhost:8012/health | API liveness |
| http://localhost:8012/ready | API readiness |
| http://localhost:8012/v1/virtual-metrics | 仮想メトリクス |

画面に「本番システムは操作しません」と出ます。擬似アラートボタンは inventory 5xx を **同じ ingest 経路** で起票するだけです。

### 同じイベントを 2 回送って 1 件

PowerShell:

```powershell
$secret = "dev-webhook-secret-not-for-prod"
$body = '{"dedup_key":"commerce-inventory-5xx","severity":"SEV2","service":"inventory","summary":"5xx ratio > 5%"}'
$hmac = [System.BitConverter]::ToString(
  [System.Security.Cryptography.HMACSHA256]::new(
    [Text.Encoding]::UTF8.GetBytes($secret)
  ).ComputeHash([Text.Encoding]::UTF8.GetBytes($body))
).Replace("-", "").ToLowerInvariant()
$headers = @{ "Content-Type" = "application/json"; "X-Signature-256" = "sha256=$hmac" }
Invoke-RestMethod -Method POST -Headers $headers -Body $body -Uri http://localhost:8012/v1/integrations/dev-inventory/events
Invoke-RestMethod -Method POST -Headers $headers -Body $body -Uri http://localhost:8012/v1/integrations/dev-inventory/events
Invoke-RestMethod http://localhost:8012/v1/incidents
```

署名無し・改ざんは 401。`event_id` を付けると同じイベントの再送は件数を増やしません。解決後の同一 `dedup_key` は新しいインシデントになります。

## テスト

```powershell
go test ./...
```

DB なし。状態機械は `apps/api/internal/incident`、HMAC は `internal/webhook` と httptest。シナリオ遷移は `packages/scenario`（scale では直らず rollback で回復）。

## 既知の制限

- メモリストア。Compose 再起動で消える。Postgres は未接続
- オンコール週次ローテーション、エスカレーション、Slack 通知なし
- ランブック CRUD と訓練セッション API なし（仮想メトリクスのプレビューのみ）
- 公開デモで他人をページングする機能はない

設計: `project/portfolio-plan/reliability-platform/DESIGN.md`
