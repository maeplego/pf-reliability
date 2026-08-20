# pf-reliability

学習用の信頼性デモです。インシデントの起票・Ack・Resolve、タイムライン、HMAC webhook（重複排除）、仮想メトリクス、bad-deploy 訓練の採点までです。扱うのは **仮想のメトリクスとインシデント** だけです。クラスタへの診断コマンドや自動 rollback はありません。**本番インシデント管理の置き換えではありません。**

| ディレクトリ | 役割 |
| --- | --- |
| `apps/api` | Go API |
| `apps/web` | ボードと訓練画面 |
| `packages/scenario` | 訓練シナリオ（破壊的 I/O なし） |
| `deploy/` | API + Web + Postgres |

認証は開発ヘッダです。状態は `triggered` → `acknowledged` → `resolved`（triggered から直接 resolve 可。resolved への ack は 409）。

## 起動

```powershell
cd deploy
copy .env.example .env
docker compose up -d --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:3012 | インシデントボード |
| http://localhost:3012/training | 仮想訓練の採点 |
| http://localhost:8012/health | API |
| http://localhost:8012/v1/virtual-metrics | 仮想メトリクス |

画面に「本番システムは操作しません」と出ます。擬似アラートは、同じ取り込み経路で inventory 5xx を起票するだけです。同じ `dedup_key` を二度送ってもインシデントは 1 件です。署名無しは 401 です。

## テスト

```powershell
go test ./...
```

訓練では bad-deploy は rollback、noisy-neighbor は scale、dependency-timeout は escalate が正解です。ランブック CRUD、3 シナリオ、訓練履歴、架空オンコール一覧があります。Slack 通知はありません。

設計の詳細は [portfolio-plan](https://github.com/maeplego/portfolio-plan) の `portfolio-plan/reliability-platform/docs/` です。
