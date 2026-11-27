# reservation-api

イベントの座席予約API（Go + PostgreSQL）
「同時にリクエストが来ても、同じ座席を二重予約しない」ことを、DBの制約・ロック・冪等性で保証する構成で体験するプロジェクト

## 構成

- `main.go` - APIサーバー本体
- `reservation.go` - 予約の作成・キャンセル（楽観ロック・冪等性キー）
- `expire.go` - 期限切れの予約を定期的に削除する goroutine
- `migrations/` - DBマイグレーション
- `loadtest/reserve.js` - k6の負荷テスト
- `docs/isolation-experiment.md` - トランザクション分離レベルの実験結果
- `docker-compose.yml` - ローカル開発用の環境定義

## 起動方法

### 前提

- Go（`go.mod` の `go` 行のバージョン。`ServeMux` のメソッド・パスパターンを使うため 1.22 以上が必要）
- Docker / Docker Compose
- k6（負荷テストを動かす場合）

### 手順

1. 環境変数ファイルを作る。

   ```
   cp .env.example .env
   ```

   `.env` の `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` と `DATABASE_URL` を、自分の環境に合わせて設定する。

2. PostgreSQL を起動する。

   ```
   docker compose up -d
   ```

3. マイグレーションを適用する（`migrations/*.up.sql` を番号順に）。

   ```
   docker exec -i reservation-db psql -U <POSTGRES_USER> -d <POSTGRES_DB> < migrations/001_create_tables.up.sql
   ```

   002 〜 005 も同様に実行する。

4. サーバーを起動する（`.env` は Makefile が読み込む）。

   ```
   make run
   ```

5. 動作確認。

   ```
   curl http://localhost:8080/health
   ```

## API

| メソッド | パス | 内容 |
| --- | --- | --- |
| GET | `/health` | ヘルスチェック |
| GET | `/events` | イベント一覧 |
| POST | `/events` | イベント作成（座席も生成） |
| GET | `/events/{id}/seats` | 座席一覧（空き状況つき） |
| POST | `/reservations` | 予約作成（`Idempotency-Key` ヘッダ対応） |
| DELETE | `/reservations/{id}` | 予約キャンセル |

予約作成のリクエスト例:

```
curl -X POST http://localhost:8080/reservations \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: abc-123' \
  -d '{"seat_id": 1, "user_name": "alice"}'
```

- 成功: `201`
- 座席がすでに予約済み、または同時更新: `409`
- 入力不正: `400`、座席が存在しない: `404`

## 二重予約の再現

対策前の実装（確認してから INSERT するだけ、ユニーク制約なし）で再現できる。

- 条件: k6 で 50 VU が同じ座席 `seat_id = 1` に 1 回ずつ同時に `POST /reservations`
- 実行:

  ```
  k6 run -e SEAT_ID=1 loadtest/reserve.js
  ```

- 結果（2026-11-09 実測）: `201` が 50 件、`409` が 0 件。DB には `seat_id = 1` の行が 50 件できた
- 確認SQL:

  ```sql
  SELECT seat_id, count(*) FROM reservations GROUP BY seat_id HAVING count(*) > 1;
  ```

- 原因: 「予約済みか確認 → INSERT」の間に隙間がある。複数のリクエストが同時に「空き」と判断して、全員が INSERT してしまう。ユニーク制約も無かった

## 対策

同じ負荷テスト（50 VU が同じ座席に 1 回ずつ POST）で、3つの方法を比べた。

### A: ユニーク制約

- 仕組み: `reservations.seat_id` に `UNIQUE` 制約をかける（`migrations/002_unique_reservation_seat.up.sql`）
- 防げる理由: 同じ `seat_id` の 2 件目の INSERT は、DBがエラー（SQLSTATE `23505`）で拒否する。アプリはそのエラーを `409` に変換して返す
- 実装コミット: `872930b`

### B: SELECT ... FOR UPDATE（悲観ロック）

- 仕組み: トランザクションの中で、座席の行を `SELECT ... FOR UPDATE` でロックしてから、予約済みか確認して INSERT する
- 防げる理由: 先に来たリクエストがコミットするまで、同じ座席を狙う他のリクエストはロックの解放を待つ。待たされたあとの確認では、すでに予約済みなので `409` になる
- 実装コミット: `6671131`

### C: 楽観ロック（version カラム）

- 仕組み: `seats.version` を読み、`UPDATE seats SET version = version + 1 WHERE id = $1 AND version = $2` で、読んだ値のままのときだけ更新する（`migrations/003_add_seat_version.up.sql`）
- 防げる理由: 同時に別のリクエストが先に更新していたら `version` が変わっていて、更新件数が 0 件になる。それを「同時に変更された」と判断して `409` を返す。ロックで待たず、衝突したら失敗させる
- 実装コミット: `5f37f02`（現在の `reservation.go` の実装）
- 注意: `002` を適用するとユニーク制約も入る。C だけの効果を測るときは、制約を一時的に外して測定した

### 冪等性キー・有効期限

- 冪等性キー:
  - `Idempotency-Key` ヘッダを付けると、同じキーでの再送は同じ結果を返す（応答に `Idempotency-Replayed: true` が付く）
  - クライアントの再送やネットワークの再試行で、二重に予約されるのを防ぐ
  - 同じキーで中身の違うリクエストを送ると `422` になる
  - キーと結果は `idempotency_keys` テーブルに保存する（`migrations/004_create_idempotency_keys.up.sql`）
- 有効期限:
  - 予約には `expires_at` が付く（作成から 15 分、`migrations/005_add_reservation_expires_at.up.sql`）
  - `runExpirer`（goroutine + ticker）が 30 秒ごとに、期限切れの予約を削除して座席を解放する

## 比較結果

- 条件: 50 VU / 50 回、同一座席 `SEAT_ID=1`、各対策 5 回測定（`iteration_duration`）。測定のたびに `DELETE FROM reservations; UPDATE seats SET version = 0;` でリセットした

| 対策 | 201 | 409 | avg | p(95) |
| --- | --- | --- | --- | --- |
| A: ユニーク制約 | 1 | 49 | 約 8.64ms | 約 11.14ms |
| B: FOR UPDATE | 1 | 49 | 約 14.00ms | 約 20.50ms |
| C: 楽観ロック | 1 | 49 | 約 7.94ms | 約 10.72ms |

- B は run 1 がウォームアップ前で遅い。run 1 を除く 4 回平均では avg 約 10.97ms、p(95) 約 17.37ms
- 所見: 3つとも二重予約を防げた（201 は常に 1 件）。差は数 ms で、5 回・50 件の規模では A と C の差は誤差の範囲とみられる。B が最も遅い傾向はあるが、順位づけは慎重に
- 分離レベル（READ COMMITTED / SERIALIZABLE）の実験結果は `docs/isolation-experiment.md` を参照

## テスト

```
make test              # go test ./...
go test -race ./...    # データ競合の検出つき
```
