# k3s-tailscale-daemonset

k3sクラスタ上でTailscaleのカスタムサブネットルーターを各ノードに設定するDaemonSet。

## 概要

このプロジェクトは、k3sクラスタの各ノードでTailscaleのサブネットルーティングを自動設定するためのKubernetes DaemonSetです。ConfigMapで指定したCIDRをTailscaleでadvertiseし、対応するip ruleを設定します。

### 主な機能

- Tailscaleの既存advertiseルートを保持しつつ、新規ルートを追加
- 指定CIDRに対するip ruleを自動設定（priority: 2500）
- 60秒ごとの自動reconcile
- Kubernetes probes用の組み込みヘルスチェックHTTPエンドポイント
- 設定可能なログレベルとreconcile間隔

## クイックスタート

### 前提条件

- k3sクラスタが稼働していること
- 各ノードでTailscaleが認証済みで動作していること
- kubectl CLIが設定済みであること

### デプロイ

1. ConfigMapを編集して、advertiseしたいCIDRを設定:

```bash
kubectl edit configmap tailscale-routes -n kube-system
```

または、k8s/configmap.yamlを編集:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-routes
  namespace: kube-system
data:
  config.yaml: |
    routes:
      - "192.168.0.0/16"
      - "172.16.0.0/12"
```

2. DaemonSetをデプロイ:

```bash
kubectl apply -k k8s/
```

または個別に適用:

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/daemonset.yaml
```

### 確認

DaemonSetの状態を確認:

```bash
kubectl get ds -n kube-system tailscale-subnet-router
kubectl logs -n kube-system ds/tailscale-subnet-router
```

ヘルスステータスを確認:

```bash
# クラスタ内からPodのヘルスを確認
kubectl exec -n kube-system ds/tailscale-subnet-router -- curl -s http://localhost:8080/healthz

# Probe失敗に関するPodイベントを確認
kubectl describe pod -n kube-system -l app=tailscale-subnet-router
```

## 設定

### ConfigMap形式

```yaml
routes:
  - "192.168.0.0/16"   # プライベートネットワーク
  - "172.16.0.0/12"    # プライベートネットワーク
```

### 環境変数

- `CONFIG_PATH`: 設定ファイルのパス（デフォルト: `/config/config.yaml`）
- `LOG_LEVEL`: ログレベル - DEBUG, INFO, WARN, ERROR（デフォルト: `INFO`）
- `RECONCILE_INTERVAL`: reconcile間隔（デフォルト: `60s`）
- `HEALTH_PORT`: ヘルスチェックHTTPサーバーのポート（デフォルト: `8080`）
- `HEALTH_TIMEOUT`: 異常と判定するまでのヘルスチェックタイムアウト（デフォルト: `90s`）

## アーキテクチャ

### コンポーネント

1. **Config Loader**: YAMLファイルから設定を読み込み
2. **Tailscale Manager**: `tailscale debug prefs`と`tailscale set`を使用してルートを管理
3. **IP Rule Manager**: netlinkライブラリでRPDBルールを管理
4. **Reconciler**: 定期的に設定を同期
5. **Health Server**: Kubernetes probes用の`/healthz`HTTPエンドポイント

### 動作フロー

1. 設定されたポートでヘルスチェックHTTPサーバーを起動
2. ConfigMapから設定を読み込み
3. 現在のTailscale advertiseルートを取得
4. 既存ルートと新規ルートをマージ
5. 変更がある場合はTailscaleに適用
6. 対応するip ruleを設定
7. 設定された間隔（デフォルト: 60秒）で繰り返し

## 開発

### 前提条件

ツールとタスクの管理には[mise](https://mise.jdx.dev/)をインストール:

```bash
curl https://mise.run | sh
```

### セットアップ

```bash
# 必要なツールをインストール
mise install

# 利用可能なタスクを一覧表示
mise tasks
```

### 共通タスク

```bash
# バイナリをビルド
mise run build

# テストを実行
mise run test

# リンティングを実行
mise run lint

# コードをフォーマット
mise run fmt

# go mod tidyを実行
mise run tidy

# Dockerイメージをビルド
mise run docker-build

# マルチアーキテクチャDockerイメージをビルド
mise run docker-buildx

# すべてのCIタスクを実行（fmt、lint、test、build）
mise run ci
```

## トラブルシューティング

### ログの確認

```bash
kubectl logs -n kube-system ds/tailscale-subnet-router -f
```

### Podの再起動

```bash
kubectl rollout restart -n kube-system ds/tailscale-subnet-router
```

### デバッグ

Pod内でコマンドを実行:

```bash
kubectl exec -it -n kube-system ds/tailscale-subnet-router -- sh
```

## ライセンス

MIT License

## 貢献

Issue報告やPull Requestは歓迎します。