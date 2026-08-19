# P12 reliability Kubernetes manifests

インシデント API + web。永続化はメモリ（platform DB `reliability` は予約済みで未接続）。overlay smoke は `RELIABILITY_DEV_AUTH`。単体 apply ではなく `pf-cloud-k8s` overlay `f-ops` から参照する。

Ingress（`pf-cloud-k8s`）:

| ホスト | Service | 用途 |
| --- | --- | --- |
| `reliability.localhost` | web:3012 | SRE デモ UI |
| `reliability-api.localhost` | api:8012 | REST |

Webhook 秘密は overlay の `p12-secrets`。製品 manifest に平文 Secret は置かない。

```powershell
cd ..\..\pf-cloud-k8s
.\scripts\cluster-smoke-f-ops.ps1
```
