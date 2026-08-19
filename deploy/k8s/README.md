# Kubernetes マニフェスト（P12 reliability）

インシデント API と Web です。このフォルダだけを apply しないでください。起動は [pf-cloud-k8s](https://github.com/maeplego/pf-cloud-k8s) の ops overlay からです。

| ホスト | 用途 |
| --- | --- |
| `reliability.localhost` | UI |
| `reliability-api.localhost` | REST |

Webhook 秘密は overlay の Secret です。製品マニフェストに平文は置きません。
