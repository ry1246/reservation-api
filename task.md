# reservation-api タスク

イベントの座席予約API。「同時に押されても二重予約しない」ことを、トランザクション・ロック・冪等性で保証する構成を体験する。Go は未経験なので、学びながら進める。1日1コミットを目安に小さく刻む。

## 決定済みの技術スタック

- 言語: Go（標準ライブラリ `net/http` 中心）
- DB: PostgreSQL（Docker / docker-compose）
- DBドライバ: pgx（`pgxpool`）
- マイグレーション: `.sql` ファイル
- 負荷テスト: k6（or hey）
- デプロイ: なし（ローカル完結）

## Phase 0: セットアップ（Day 1〜3）

- [x] `reservation-api/` 作成・`go mod init`
- [x] `.gitignore` 整備（`pgdata/`, `.env`, バイナリ）
- [x] `reservation-api/` で `git init`（独立リポジトリとして管理する）
- [x] 最初の commit
- [x] GitHub にリポジトリを作成し、`origin` を設定して push
- [x] Day 1: `docker-compose.yml`（PostgreSQL）作成・起動・`psql` で接続確認
- [x] Day 2: テーブル設計（`events` / `seats` / `reservations`）
- [x] Day 2: マイグレーションSQL作成・適用
- [x] Day 3: `main.go` に `/health` だけ返す最小HTTPサーバ

## Phase 1: 素朴なCRUD（整合性は考えない）

- [x] pgx で DB 接続（`pgxpool`、接続情報は環境変数）
- [x] `GET /events` イベント一覧
- [x] `POST /events` イベント作成（座席も生成）
- [x] `GET /events/{id}/seats` 座席一覧（空き状況つき）
- [x] `POST /reservations` 予約作成（この時点では素朴な実装）
- [x] `DELETE /reservations/{id}` 予約キャンセル
- [x] 入力バリデーションとエラーレスポンスの統一（JSON、ステータスコード）
- [x] ハンドラの基本テスト（`go test`、`httptest`）

## Phase 2: 二重予約を再現して潰す（本題）

- [x] 開発環境整備: `.env` の `DATABASE_URL` を `go run .` に自動で渡す（direnv の `dotenv` か Makefile の `make run`。毎回 `set -a; source .env` しなくて済むように）
- [x] k6 で同一座席へ同時リクエストを投げるスクリプト作成
- [x] 二重予約が実際に起きることを確認（Phase 1 の実装のバグを再現）
- [x] 対策A: DB のユニーク制約（座席ごとに有効な予約は1件）
- [x] 対策B: トランザクション + `SELECT ... FOR UPDATE`（悲観ロック）
- [x] 対策C: 楽観ロック（version カラム）
- [x] A/B/C を同じ負荷テストで比較し、結果（成功数・所要時間）を記録
- [x] トランザクション分離レベルの挙動を実験で確認（READ COMMITTED と SERIALIZABLE）
- [x] 冪等性キー（`Idempotency-Key` ヘッダ）で二重送信を防ぐ

## Phase 3: 仕上げ

- [x] 予約の有効期限（仮押さえ）と期限切れの解放（goroutine / ticker で実装）
- [x] 並行処理のテスト（`go test -race`）
- [x] README に、再現手順と各対策の比較結果をまとめる
  - 対策前の再現結果（2026-11-09 実測）
    - 条件: k6 で 50 VU が同じ座席 `seat_id = 1` に 1 回ずつ同時に `POST /reservations`（`loadtest/reserve.js`）
    - 結果: 201 が 50 件、409 が 0 件。DB は `seat_id = 1` が 50 行（二重予約が発生）
    - 原因: `reservation.go` の「予約済みか確認 → INSERT」の間に隙間がある。ユニーク制約も無い
    - 確認SQL: `SELECT seat_id, count(*) FROM reservations GROUP BY seat_id HAVING count(*) > 1;`
  - 対策A（ユニーク制約）の結果（2026-11-12 実測）
    - 条件: 同上（`k6 run -e SEAT_ID=1 loadtest/reserve.js`、50 VU / 50 回）
    - 結果: 201 が 1 件、409 が 49 件。`seat_id = 1` は 1 行のみ（二重予約は発生しない）
  - 対策C（楽観ロック）の結果（2026-11-14 実測）
    - 条件: 同上（ユニーク制約を一時的に外して実施。`k6 run -e SEAT_ID=<id> loadtest/reserve.js`、50 VU / 50 回）
    - 結果: 201 が 1 件、409 が 49 件。`http_req_duration` は avg 25.9ms / p(95) 28.66ms、全体の所要時間は約 33ms（1502 req/s）
    - 補足: k6 の `http_req_failed` 98% は 409 を失敗扱いしているだけで、想定どおりの結果
  - A/B/C 比較用の測定（2026-11-14、各対策 5 回、毎回 `DELETE FROM reservations; UPDATE seats SET version = 0;` でリセット、`SEAT_ID=1`、50 VU / 50 回）
    - A（`872930b`、ユニーク制約あり）: 5 回とも 201 が 1 件、409 が 49 件
      - `http_req_duration` avg: 7.55 / 7.40 / 7.26 / 7.69 / 7.42 ms（平均 約 7.46ms）
      - `http_req_duration` p(95): 10.01 / 9.30 / 9.36 / 10.08 / 10.26 ms（平均 約 9.80ms）
    - B（`6671131`、ユニーク制約なし）: 5 回とも 201 が 1 件、409 が 49 件
      - `http_req_duration` avg: 24.96 / 9.98 / 9.76 / 9.54 / 9.94 ms（5 回平均 約 12.84ms、run 1 を除く 4 回平均 約 9.81ms）
      - `http_req_duration` p(95): 32.22 / 16.14 / 16.16 / 15.87 / 16.07 ms（5 回平均 約 19.29ms、run 1 を除く 4 回平均 約 16.06ms）
      - 注: run 1 だけ遅い。サーバ起動直後でウォームアップ（DB 接続プールの初期化など）の影響とみられる。A は事前に手動で 1 回流してから測ったため、この差は出ていない
    - C（`5f37f02`、ユニーク制約なし、ウォームアップ 1 回後に測定）: 5 回とも 201 が 1 件、409 が 49 件
      - `iteration_duration` avg: 7.87 / 7.84 / 7.86 / 8.43 / 7.69 ms（平均 約 7.94ms）
      - `iteration_duration` p(95): 10.44 / 10.58 / 10.75 / 11.36 / 10.47 ms（平均 約 10.72ms）
      - 注: 今回の C の出力には `http_req_duration` の行が含まれていない（grep の抜き出しから漏れた）。A/B/C を揃えて比べるため、`iteration_duration` で比較する
      - 注: 上の 2026-11-14 実測（avg 25.9ms）は起動直後の 1 回分で、ウォームアップなし。比較には使わない
  - A/B/C 比較表（`iteration_duration`、5 回平均。B は run 1 がウォームアップ前のため、除く 4 回平均も併記）
    - A（ユニーク制約）: 201=1 / 409=49、avg 約 8.64ms、p(95) 約 11.14ms
    - B（`FOR UPDATE`）: 201=1 / 409=49、avg 約 14.00ms（run 1 除く 4 回: 約 10.97ms）、p(95) 約 20.50ms（run 1 除く 4 回: 約 17.37ms）
    - C（楽観ロック）: 201=1 / 409=49、avg 約 7.94ms、p(95) 約 10.72ms
    - 所見: 3 つとも二重予約は防げた。差は数 ms で、50 件規模では小さい。B が最も遅く、A と C は近い。ただし 5 回・50 件では、A と C の差は誤差の範囲とみられるため、順位づけは慎重に
- [ ] `next-project-ideas.md` の 6 に実施済みを反映

## 次の候補（このプロジェクトの延長）

- 8: 可観測性（OpenTelemetry / Prometheus + Grafana / レート制限）を本APIに乗せる
- 7: イベント駆動のサービス間通信（NATS or Kafka / gRPC）
