# トランザクション分離レベルの実験

「確認 → INSERT」という同じ処理が、分離レベルによって二重予約になるかどうかを、2 つの `psql` セッションで手動で確かめた記録。

## 前提

- PostgreSQL 16（Docker）
- `reservations.seat_id` のユニーク制約は、実験のあいだ**外した**（外さないと、分離レベルと関係なく 2 件目の INSERT が弾かれるため）
  ```sql
  ALTER TABLE reservations DROP CONSTRAINT reservations_seat_id_key;
  ```
- 各実験の前に、予約を空にした
  ```sql
  DELETE FROM reservations;
  ```
- 対象は `seat_id = 1`。2 つのセッションを S1、S2 と呼ぶ

## 共通の手順

`BEGIN` の分離レベル指定だけを変えて、同じ順番で実行する。

| 順 | セッション | SQL |
|---|---|---|
| 1 | S1 | `BEGIN [ISOLATION LEVEL ...];` |
| 2 | S1 | `SELECT EXISTS (SELECT 1 FROM reservations WHERE seat_id = 1);` → `f` |
| 3 | S2 | `BEGIN [ISOLATION LEVEL ...];` |
| 4 | S2 | `SELECT EXISTS (SELECT 1 FROM reservations WHERE seat_id = 1);` → `f` |
| 5 | S1 | `INSERT INTO reservations (seat_id, user_name) VALUES (1, 's1');` |
| 6 | S1 | `COMMIT;` |
| 7 | S2 | `INSERT INTO reservations (seat_id, user_name) VALUES (1, 's2');` |
| 8 | S2 | `COMMIT;` |

順 4 の時点で、S2 は S1 の未コミットの INSERT を見られないので、まだ空きに見える。この「読んだ時点の前提」が、順 7 の時点では崩れている。

## 結果

| 実験 | 分離レベル | `seat_id = 1` の行数 | 結果 |
|---|---|---|---|
| 1 | READ COMMITTED（既定） | 2 | 二重予約になった |
| 2 | SERIALIZABLE | 1 | 防げた |
| 3 | REPEATABLE READ | TODO | TODO |
| 4 | SERIALIZABLE（Go + k6） | TODO | 未実施 |

### 実験 1: READ COMMITTED

`BEGIN;` のまま（既定）で実行した。エラーは出ず、S1 と S2 の両方が COMMIT まで成功して、2 行になった。Phase 1 のバグ（二重予約）と同じことが、手作業で再現できた。

### 実験 2: SERIALIZABLE

`BEGIN ISOLATION LEVEL SERIALIZABLE;` で実行した。S2 のどこかで次のエラーが出て、S1 の 1 行だけが残った。

```
ERROR:  could not serialize access due to read/write dependencies among transactions
HINT:  The transaction might succeed if retried.
```

これは SQLSTATE `40001`（serialization_failure）。失敗した側は `ROLLBACK;` が必要で、アプリは**リトライするか、409 / 503 を返す**必要がある。

- エラーが出た位置（INSERT か COMMIT か）: TODO

### 実験 3: REPEATABLE READ

`BEGIN ISOLATION LEVEL REPEATABLE READ;` で実行した。

- 期待: S1 と S2 が別々の新しい行を INSERT するだけで、書き込みの衝突とみなされないため、エラーは出ず、2 行になる（書き込みスキュー）。
- 実測: TODO

### 実験 4: Go + k6 で SERIALIZABLE

`experiment/serializable` ブランチで実施。`BeginTx` で `pgx.Serializable` を指定して、`40001` を 409 に変換した。

- 実測: TODO

## わかったこと

- READ COMMITTED では、「確認 → INSERT」は安全ではない。
- SERIALIZABLE は、ロックなしで「トランザクションを順番に実行したのと同じ結果」を保証する。代わりに、`40001` のリトライ（または 409 / 503）をアプリが担う必要がある。
- これは、楽観ロックで競合に負けた側が 409 を返す流れと似ている。
