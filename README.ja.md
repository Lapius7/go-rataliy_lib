# go-rataliy_lib

[English](README.md)

[![CI](https://github.com/Lapius7/go-rataliy_lib/actions/workflows/ci.yml/badge.svg)](https://github.com/Lapius7/go-rataliy_lib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Lapius7/go-rataliy_lib.svg)](https://pkg.go.dev/github.com/Lapius7/go-rataliy_lib)
[![Go Report Card](https://goreportcard.com/badge/github.com/Lapius7/go-rataliy_lib)](https://goreportcard.com/report/github.com/Lapius7/go-rataliy_lib)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Go向けのHTTPレート制限ライブラリ。複数のアルゴリズムから選べて、`net/http`
ミドルウェアにそのまま差し込めます。コアパッケージは依存パッケージゼロです。

```go
import "github.com/Lapius7/go-rataliy_lib"
```

実際に動かせるサンプルが [`test/`](test/) にあります。リポジトリをcloneして
`go run` すれば、レート制限・レスポンスヘッダー・ルートごとのルール・
ライブダッシュボードをその場で確認できます。

## 目次

- [なぜ作ったか](#なぜ作ったか)
- [クイックスタート](#クイックスタート)
- [IPの代わりにAPIキーで制限する](#ipの代わりにapiキーで制限する)
- [ルートごとに違う制限をかける](#ルートごとに違う制限をかける)
- [ミドルウェアを使わずLimiterを直接使う](#ミドルウェアを使わずlimiterを直接使う)
- [後始末（クリーンシャットダウン）](#後始末クリーンシャットダウン)
- [ダッシュボード](#ダッシュボード)
- [アルゴリズム](#アルゴリズム)
- [ストレージ](#ストレージ)
- [既知の制約](#既知の制約)
- [FAQ](#faq)
- [コントリビュート](#コントリビュート)
- [ライセンス](#ライセンス)

## なぜ作ったか

`golang.org/x/time/rate` はtoken bucketの実装は提供してくれますが、HTTP
ハンドラへの組み込み、キー戦略の選択、429レスポンスの処理は自分で書く必要が
あります。go-rataliy_libはその面倒な部分を肩代わりし、トレードオフに応じて
3つのアルゴリズムから選べるようにしたものです。依存パッケージは増やしません。

## クイックスタート

```bash
go get github.com/Lapius7/go-rataliy_lib
```

```go
package main

import (
	"net/http"
	"time"

	"github.com/Lapius7/go-rataliy_lib"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", helloHandler)

	limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
		Rate:    60,             // 60回まで
		Per:     time.Minute,    // 1分間に
		KeyFunc: ratelimit.ByIP, // クライアントIPごとに制限
	})
	defer limiter.Close()

	http.ListenAndServe(":8080", limiter.Middleware(mux))
}
```

制限を超えたリクエストには `429 Too Many Requests` と `Retry-After`
ヘッダーが返ります。許可・拒否を問わず、すべてのレスポンスに
`X-RateLimit-Limit` / `X-RateLimit-Remaining` / `X-RateLimit-Reset`
ヘッダーが付与されるので、クライアント側で残りの利用回数を確認できます。

これは標準的な `http.Handler` ミドルウェアなので、ハンドラを受け取れる
任意のルーター・フレームワーク（Chi、Gorilla、または各フレームワーク独自の
ハンドラ型をラップする形）でそのまま使えます。

`ratelimit.New` は `Config.Rate` または `Config.Per` が正の値でない場合に
panicします。誤設定によって「何も制限されない」「すべてが弾かれる」と
いう静かな失敗を防ぐため、早期に検出して止める設計です。

## IPの代わりにAPIキーで制限する

```go
limiter := ratelimit.New(ratelimit.SlidingWindow, ratelimit.Config{
	Rate:    1000,
	Per:     time.Hour,
	KeyFunc: ratelimit.ByHeader("X-API-Key"),
})
```

`X-API-Key` ヘッダーが付いていないリクエストは、全員でひとつの予備バジェットを
共有します。ヘッダー無しのリクエストごとに無制限のバケットが与えられたり、
本当に空文字列のヘッダー値を送ってきたリクエストと衝突したりはしません。

## ルートごとに違う制限をかける

`Router` は `http.ServeMux` と同じパターン構文で、パターンごとに異なる
`Limiter` を振り分けます。マッチしないリクエストは無制限で素通りします。

```go
strict := ratelimit.New(ratelimit.FixedWindow, ratelimit.Config{Rate: 2, Per: time.Minute})
defer strict.Close()

rt := ratelimit.NewRouter()
rt.Handle("/admin/", strict)

http.ListenAndServe(":8080", rt.Middleware(mux))
```

## ミドルウェアを使わずLimiterを直接使う

```go
result := limiter.Allow("some-key")
if !result.Allowed {
	// result.RetryAfter だけ待つ、または拒否する
}
```

`Allow` は `Allowed` / `Remaining` / `RetryAfter` / `ResetAt` を持つ
`Result` を返します。ミドルウェアがレスポンスヘッダーに入れているのと
同じ情報です。

## 後始末（クリーンシャットダウン）

デフォルトのインメモリストアは、期限切れキーを掃除するバックグラウンド
goroutineを動かしています。使い終わったLimiterに対して `Limiter.Close()`
を呼ぶ（例: グレースフルシャットダウン時）と、そのgoroutineを停止できます。

## ダッシュボード

`Dashboard` は、各Limiterが追跡している全キーが**今この瞬間**どういう状態に
あるか（残り回数とリセット時刻）を、キー単位・Limiter単位で表示する、
自動更新のライブビューです。過去のリクエスト履歴やアクセスログではなく、
現在の状態を表示します。

```go
hello := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{Rate: 60, Per: time.Minute})
strict := ratelimit.New(ratelimit.FixedWindow, ratelimit.Config{Rate: 5, Per: time.Minute})

dashboard := ratelimit.NewDashboard(map[string]*ratelimit.Limiter{
	"hello":  hello,
	"strict": strict,
})

// アプリ本体のリスナーとは別の、独自のポートで動かす
go dashboard.ListenAndServe(":9090")
```

`http://localhost:9090/` を開けばHTMLビュー、`GET /api/snapshot` で同じ
データをJSONで取得できます（付属のページの代わりに、自分の監視システムに
組み込みたい場合に便利です）。専用ポートを持たせず既存のサーバーに組み込み
たい場合は `dashboard.Handler()` を使ってください。

ダッシュボードがキー一覧を表示できるのは、`Store`が列挙に対応している
Limiterだけです。デフォルトのインメモリストアは対応していますが、
`redisstore`は対応していません（[既知の制約](#既知の制約)参照）。非対応の
場合は空のテーブルではなく「このLimiterのストアはキー一覧表示に対応して
いません」と表示されます。

## アルゴリズム

| アルゴリズム      | 挙動                                                                      | キーあたりのメモリ |
|------------------|---------------------------------------------------------------------------|-----------------|
| `TokenBucket`    | バーストを滑らかにする。`Burst`（未指定時は`Rate`）まで短時間の急増を許容。 | 16バイト        |
| `SlidingWindow`  | 重み付き2窓カウンタによる近似的なスライディングウィンドウ。window境界での急激な変化が無い。 | 16バイト |
| `FixedWindow`    | 最もシンプルで軽量。window境界で鋭くリセットされるため、境界をまたぐと短時間で最大2倍のバーストを許容する場合がある。 | 12バイト |

どれを選ぶか迷ったら `TokenBucket` から始めてください。汎用的なAPIレート
制限における標準的な選択です。

## ストレージ

状態は `Store` インターフェースの背後に保持されます。

```go
type Store interface {
	Get(key string) (state []byte, ok bool)
	Set(key string, state []byte, ttl time.Duration)
}
```

デフォルトはインメモリストアです（期限切れキーをバックグラウンドgoroutineで
掃除します — 詳細は上記の[後始末（クリーンシャットダウン）](#後始末クリーンシャットダウン)
参照）。単一プロセスであれば十分ですが、**複数インスタンス間での協調は
行いません**。ロードバランサ配下で複数台動かす場合、各インスタンスが
独立して設定レートを適用するため、実質的な制限値はインスタンス数倍に
なります。

複数インスタンス間で制限を共有したい場合は、Redisを使った別モジュール
[`redisstore`](redisstore/) を使ってください。

```go
import (
	"github.com/Lapius7/go-rataliy_lib"
	"github.com/Lapius7/go-rataliy_lib/redisstore"
	"github.com/redis/go-redis/v9"
)

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
store := redisstore.New(client, "myapp:ratelimit:")

limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
	Rate:  60,
	Per:   time.Minute,
	Store: store,
})
```

`redisstore` を別のGoモジュールに分離しているのは、コアの
`go-rataliy_lib` パッケージを依存パッケージゼロのまま保つためです。
分散環境での制限が本当に必要な場合だけ `go-redis` を取り込む形になります。

## 既知の制約

公開前に必ず読んでください。いずれもバグではありませんが、利用環境に
よっては動作に影響する前提条件です。

- **キーの種類数に上限が無い（インメモリストア）。** デフォルトストアは
  追跡する異なるキーの数に上限を設けておらず、TTLベースの掃除しか
  行いません。信頼できないヘッダー（例: 偽装された`X-Forwarded-For`）を
  キーにしている場合、攻撃者が多数の異なるキーを高速に生成すると、各キーの
  TTLが切れるまでメモリ使用量が増え続ける可能性があります。`KeyFunc` が
  クライアントが制御できる値から導出される場合は、TTL（おおよそ
  `Config.Per`）を短くするか、独自の退避ポリシーを持つ `Store` を
  使ってください。
- **Redis障害時はフェイルオープンする。** `redisstore.Store.Get` は
  タイムアウトや接続切断を含む**すべての**Redisエラーを「キーが存在しない」
  と同じ扱いにします。Redisに到達できない間は、すべてのリクエストが新規
  バケットとして扱われ、許可されます。これはレート制限としては通常正しい
  トレードオフ（可用性を優先する）ですが、Redisの障害がレート制限を黙って
  無効化することを意味します。不正利用対策として制限に頼っている場合は、
  Redisの可用性を別途監視してください。
- **`ByIP` は `RemoteAddr` をそのまま信用します。** リバースプロキシ配下
  では、これはプロキシのアドレスであり、クライアントのものではありません
  ──プロキシの裏にいる全クライアントが1つのバケットを共有します。実際の
  クライアントIPが必要なら `ByHeader("X-Forwarded-For")`（または独自の
  `KeyFunc`）を使ってください。ただし、その場合はそのヘッダーが信頼できる
  プロキシによって設定されることがデプロイ環境で保証されている場合に限り
  使ってください（クライアントが直接設定できる場合、任意のIPを名乗って
  制限を回避できてしまいます）。
- **`SlidingWindow` は近似です。** 厳密なタイムスタンプのスライディング
  ログではなく、重み付き2窓カウンタによる近似です。一般的なAPI制限には
  十分な精度ですが、window境界付近では名目上のレートよりわずかに多く、
  または少なく許可することがあります。
- **独自の`Store`実装はバイト列をそのまま往復させる必要があります。**
  各アルゴリズムは状態を固定長の小さいバイナリblobとしてエンコードし、
  `Get`が返す値は以前の`Set`が書き込んだものと完全に一致する（または
  何も無い）ことを前提にしています。バイト列を切り詰めたり、再エンコード
  したりして変更する`Store`は、panicや不正な制限動作を引き起こします。
  `state`引数は解釈・再整形せず、そのまま保存・返却してください。
- **ダッシュボードには認証がありません。** `Dashboard.ListenAndServe`は
  アクセス制御を一切持たないHTTPサーバーを起動します。そのポートに到達
  できる人は誰でも、追跡中の全キー（クライアントIPやAPIキーヘッダーの値
  など）と各キーの現在のレート制限状態を見ることができます。信頼できない
  ネットワークにそのポートを公開しないでください。チーム外から到達可能に
  なり得る場合は、自前の認証やリバースプロキシの裏に置くか、localhost
  または内部インターフェースのみにバインドしてください。

## FAQ

**Gin / Echo / Chi / [フレームワーク] でも使えますか？**
はい。`Limiter.Middleware` と `Router.Middleware` は単純な
`func(http.Handler) http.Handler` です。標準ハンドラをラップできる
（またはラップ対象として公開している）フレームワークであれば、そのまま
使えます。

**gRPCや非HTTPの通信にもレート制限をかけられますか？**
ミドルウェア経由では無理ですが、`Limiter.Allow(key) Result` はHTTPに
依存していません。適切なキー（例: gRPCの`peer.FromContext`から取得した
ピアアドレス）を渡して直接呼び出し、`Result.Allowed` に応じて自分で処理
してください。

**`New` でエラーを返さずpanicするのはなぜですか？**
不正な`Config`（ゼロまたは負の`Rate`/`Per`）は実行時に回復すべき状況では
なく、範囲外のスライスインデックスと同じ種類のプログラミングエラーです。
トラフィックを受け付ける前の起動時にpanicさせることで、誤設定された
Limiterが本番環境で黙ってすべてを許可・拒否し続けるのを防ぎ、ミスを
即座に表面化させます。

## コントリビュート

IssueやPull Requestを歓迎します。PRを送る前に以下を実行してください。

```bash
go build ./... && go vet ./... && go test ./... -race
gofmt -l .   # 何も出力されないことを確認
```

`redisstore/` を変更した場合は、そのディレクトリ内（別モジュールです）でも
`go build ./...` と `go vet ./...` を実行してください。Redisを使う
統合テストはビルドタグで切り離されており、ローカルのRedisが必要です。

```bash
cd redisstore
go test -tags redis_integration ./...
```

## ライセンス

MIT — [LICENSE](LICENSE) を参照してください。
