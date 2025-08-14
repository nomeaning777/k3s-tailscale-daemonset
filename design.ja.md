# システム設計書 — k3s-tailscale-daemonset

## 1. システム概要

k3sクラスタ上で動作するDaemonSetとして、各ノードにTailscaleのカスタムサブネットルーターを設定し、RPDBルールを管理するシステム。

### 主要機能
- Tailscaleの既存advertiseルートを保持しつつ、新規ルートを追加
- 指定CIDRに対するip ruleを自動設定（priority: 2500、table: main）
- 冪等性を保証し、定期的にreconcileを実行

## 2. アーキテクチャ

### 2.1 コンポーネント構成

```
┌─────────────────────────────────────┐
│         k3s Cluster                 │
│                                     │
│  ┌─────────────────────────────┐   │
│  │     ConfigMap               │   │
│  │   (routes configuration)    │   │
│  └──────────┬──────────────────┘   │
│             │ VolumeMount          │
│  ┌──────────▼──────────────────┐   │
│  │     DaemonSet Pod           │   │
│  │  ┌────────────────────┐     │   │
│  │  │  Go Application     │     │   │
│  │  │                     │     │   │
│  │  │  - Config Loader    │     │   │
│  │  │  - Route Manager    │     │   │
│  │  │  - Rule Manager     │     │   │
│  │  │  - Reconciler       │     │   │
│  │  └──────┬──────────────┘     │   │
│  │         │                    │   │
│  │  hostNetwork: true           │   │
│  │  hostPath: /var/run/tailscale│   │
│  └─────────┬────────────────────┘   │
│            │                        │
│  ┌─────────▼────────────────────┐   │
│  │     Node (Host)              │   │
│  │  - tailscaled socket         │   │
│  │  - iproute2 (ip rule)        │   │
│  └──────────────────────────────┘   │
└─────────────────────────────────────┘
```

### 2.2 データフロー

1. **設定読み込み**: ConfigMapからYAML設定をファイルとして読み込み
2. **現状取得**: `tailscale debug prefs`で既存のadvertiseルートを取得
3. **ルートマージ**: 既存ルートと新規ルートをマージ（重複除去）
4. **Tailscale設定**: `tailscale set --advertise-routes`で適用
5. **IP Rule設定**: 各CIDRに対して`ip rule add`を実行
6. **Reconcile**: 定期的（60秒ごと）に上記を繰り返し

## 3. 詳細設計

### 3.1 設定ファイル形式

```yaml
# /config/config.yaml (ConfigMapからマウント)
routes:
  - "10.0.0.0/8"
  - "192.168.0.0/16"
  - "172.16.0.0/12"
```

### 3.2 モジュール構成

#### 3.2.1 Main Loop
- 初期化処理
- Reconcileループの管理（60秒間隔）
- シグナルハンドリング（SIGTERM/SIGINT）

#### 3.2.2 Config Loader
- YAML設定ファイルの読み込み
- 設定値のバリデーション（CIDR形式チェック）

#### 3.2.3 Route Manager
- `tailscale debug prefs`の実行とJSONパース
- 既存advertiseルートの抽出
- 新規ルートとのマージ処理
- `tailscale set --advertise-routes`の実行

#### 3.2.4 Rule Manager
- 現在のip ruleの取得（`ip rule list`）
- 必要なルールの追加（`ip rule add from all to <CIDR> priority 2500 table main`）
- 不要なルールの削除（設定から削除されたCIDR）

#### 3.2.5 Reconciler
- Route ManagerとRule Managerの協調制御
- エラーハンドリングとリトライ
- ステータス管理

### 3.3 処理フロー

```go
// 疑似コード
func reconcile() error {
    // 1. 設定読み込み
    config := loadConfig("/config/config.yaml")
    
    // 2. 現在のTailscaleルート取得
    currentRoutes := getTailscaleRoutes() // tailscale debug prefs
    
    // 3. ルートマージ
    mergedRoutes := mergeRoutes(currentRoutes, config.Routes)
    
    // 4. Tailscale設定更新
    if routesChanged(currentRoutes, mergedRoutes) {
        setTailscaleRoutes(mergedRoutes) // tailscale set --advertise-routes
    }
    
    // 5. IP Rule設定
    currentRules := getIPRules() // ip rule list
    for _, route := range config.Routes {
        if !ruleExists(currentRules, route) {
            addIPRule(route, 2500) // ip rule add
        }
    }
    
    // 6. 不要なルール削除
    removeObsoleteRules(currentRules, config.Routes)
    
    return nil
}
```

### 3.4 エラーハンドリング

- **設定ファイル不在**: エラーログ出力、次回reconcileで再試行
- **Tailscale接続エラー**: エラーログ出力、exponential backoffでリトライ
- **ip ruleエラー**: 個別にエラー処理、他のルールは継続処理
- **パニック**: recoverで捕捉、ログ出力後に処理継続

## 4. Kubernetes マニフェスト設計

### 4.1 DaemonSet

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: tailscale-subnet-router
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: tailscale-subnet-router
  template:
    metadata:
      labels:
        app: tailscale-subnet-router
    spec:
      hostNetwork: true
      containers:
      - name: subnet-router
        image: ghcr.io/nomeaning777/k3s-tailscale-daemonset:latest
        securityContext:
          capabilities:
            add: ["NET_ADMIN"]
        volumeMounts:
        - name: tailscale-socket
          mountPath: /var/run/tailscale
        - name: config
          mountPath: /config
        readinessProbe:
          exec:
            command: ["/app/healthcheck"]
          initialDelaySeconds: 10
          periodSeconds: 30
      volumes:
      - name: tailscale-socket
        hostPath:
          path: /var/run/tailscale
          type: Directory
      - name: config
        configMap:
          name: tailscale-routes
```

### 4.2 ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-routes
  namespace: kube-system
data:
  config.yaml: |
    routes:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
```

## 5. ビルド・デプロイ設計

### 5.1 Dockerイメージ

- ベースイメージ: `alpine:3.19`（軽量）
- 必要パッケージ: `tailscale`, `iproute2`
- マルチアーキテクチャ: linux/amd64, linux/arm64, linux/arm/v7

### 5.2 ビルドプロセス

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.19
RUN apk add --no-cache tailscale iproute2
COPY --from=builder /build/app /app/main
COPY --from=builder /build/healthcheck /app/healthcheck
ENTRYPOINT ["/app/main"]
```

## 6. ログ設計

### ログレベル
- **INFO**: 重要な操作（ルート追加、ip rule設定）
- **ERROR**: エラー発生時
- **DEBUG**: 環境変数`DEBUG=true`時のみ詳細ログ

### ログフォーマット
```
2024-01-15T10:30:45Z INFO: Applied advertise routes: [10.0.0.0/8, 192.168.0.0/16]
2024-01-15T10:30:46Z INFO: Added ip rule for 10.0.0.0/8 (priority: 2500)
2024-01-15T10:31:45Z INFO: Reconcile completed successfully
```

## 7. ヘルスチェック設計

### Readiness Probe
- 最後のreconcile成功から90秒以内であること
- Tailscaleソケットへの接続が可能であること
- 設定ファイルが読み込み可能であること

## 8. セキュリティ考慮事項

- **最小権限**: NET_ADMIN capabilityのみ付与
- **Secrets不要**: 認証情報は含まない
- **読み取り専用マウント**: ConfigMapは読み取り専用
- **ネームスペース分離**: kube-systemで実行

## 9. 運用考慮事項

### 監視
- Pod再起動回数の監視
- ログのエラーレート監視

### トラブルシューティング
- `kubectl logs -n kube-system ds/tailscale-subnet-router`でログ確認
- `kubectl exec`でPod内部の状態確認

### アップグレード
- ConfigMap更新後、Pod再起動で反映
- イメージ更新は`kubectl set image`で実施

## 10. 今後の拡張可能性

- Prometheusメトリクス露出
- Webhook通知
- 複数設定プロファイル対応
- CRD化による細かい制御