# go-rataliy_lib example

[English](README.md)

[go-rataliy_lib](https://github.com/Lapius7/go-rataliy_lib) を実際に動かせる
デモです。異なる制限を持つ2つのエンドポイントを、`Router` で振り分けます。

## 動かし方

```bash
git clone https://github.com/Lapius7/go-rataliy_lib
cd go-rataliy_lib/test
go run .
```

`:18181` でサーバーが起動し、以下の2つのエンドポイントが立ち上がります。

- `/hello` — token bucket、5リクエスト/分
- `/strict` — fixed window、2リクエスト/分

さらに `:18182` で、両方のLimiterの現在の状態を表示するライブダッシュボード
が起動します。ブラウザで `http://localhost:18182/` を開く、または背後の
JSONを `http://localhost:18182/api/snapshot` に`curl`してください。

## 試し方

別のターミナルで実行します。

```bash
# 最初の5回は成功、6回目はレート制限される
for i in $(seq 1 6); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18181/hello; done

# レート制限ヘッダーと429レスポンスを確認
curl -i http://localhost:18181/hello

# /strict は独自の、より厳しい制限を持つ
for i in $(seq 1 3); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18181/strict; done
```

すべてのレスポンスに `X-RateLimit-Limit` / `X-RateLimit-Remaining` /
`X-RateLimit-Reset` ヘッダーが付くので、残り回数が減っていく様子を確認できます。
拒否されたリクエストには `Retry-After` ヘッダーも付き、どれだけ待てばよいかが
わかります。ダッシュボードを更新する（または2秒ごとの自動更新を待つ）と、
キーごとに同じ数値が反映されているのを確認できます。

## 自分のチェックアウトを使う

このディレクトリの `go.mod` には親ディレクトリを指す `replace` が設定されて
いるため、`go run .` は常にローカルの `go-rataliy_lib` チェックアウトの内容を
使って動作します。ライブラリ自体を改造している場合に便利です。
