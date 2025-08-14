# 要件定義書 — *k3s-tailscale-daemonset*

## 1. 概要
個人プロジェクト（単独利用）として、k3s クラスタ上で **Tailscale overlay network mode** の制約（カスタムのサブネットルーターを設定不可）を回避し、**DaemonSet によりノード毎に Tailscale 設定を自動適用**してカスタムのサブネットルーター（以降 *custom subnet router*）を利用可能にする。

また、advertise したサブネットに対するトラフィックが **Linux の main ルーティングテーブルを経由**するように **RPDB（`ip rule`）のルールを自動付与**する。

> 設定値は **ConfigMap を VolumeMount してファイルとして読み込む**。Secrets は不要（秘密情報を含まない）。

## 2. 背景 / 課題
- k3s の Tailscale overlay network mode では *custom subnet router* を柔軟に設定できない。
- Tailscale が作成するルートが main 以外のテーブルや独自経路に載るため、ルーティング制御が難しいケースがある。
- 手作業による設定は再起動やノード追加のたびに必要で煩雑。

## 3. 目的（ゴール）
- DaemonSet が冪等的にノードへ Tailscale 設定を適用し、指定 CIDR を advertise できる。
- 指定 CIDR 宛トラフィックが `table main` を参照するように `ip rule` を作成・維持する。
- 再起動・ノード追加時にも自動再適用される。
- 個人利用前提で運用手間を最小化。

## 4. スコープ
### 対象範囲
- k3s 上で動作する DaemonSet の実装。
- 各ノードに対し `tailscale` CLI による設定適用、`ip rule` の追加、readiness チェック。
- 設定は ConfigMap をファイルとしてマウントして読み込む。

### 除外範囲
- 多ユーザ認可や RBAC の細分化。
- HA 構成や複雑なフェイルオーバ制御。
- Helm チャート配布。

## 5. 利用者要件
- 対象ユーザ: 作成者本人のみ。
- 運用: kubeconfig が使用可能で `kubectl apply` によるデプロイ。
- 監視: Pod ログ程度の最小限。

## 6. 機能要件
- Advertise Routes 管理（冪等適用、重複回避）
- RPDB ルールの自動追加と維持
- tailscaled ソケット接続（hostPath マウント）
- 起動時および再起動時の自動再適用
- readiness チェック
- 実行ログ出力
- マルチアーキテクチャ対応（amd64, arm64, arm/v7）

## 7. 非機能要件
- パフォーマンス: 起動時適用のみ、常駐負荷小
- 信頼性: 冪等性・再試行・安全側動作
- セキュリティ: Secrets 不要、`NET_ADMIN` 権限と hostNetwork 許可

## 8. 制約条件
- 対象環境: k3s (Linux, iproute2)
- 使用技術: Kubernetes DaemonSet, シェル/Go, iproute2, tailscale CLI
- 権限: `NET_ADMIN` capability, hostPath 許容
- アーキテクチャ: linux/amd64, linux/arm64, linux/arm/v7

## 9. 成果物
- manifests/（素の YAML）
- マルチアーキ対応コンテナイメージ
- サンプル ConfigMap
- README（ビルド・デプロイ手順）


