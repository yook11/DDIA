# PostgreSQLの環境と掲示板テーブル

環境は `compose.yaml`、掲示板のテーブル定義は
[`migrations/001_board.sql`](migrations/001_board.sql) にある。
レプリケーションの実験手順は [SQLラボ](../../labs/postgres_replication/README.md) を参照。

## テーブルを作成・確認する

リポジトリのルートで実行する。Goのビルドやホストへのpsqlインストールは不要。

```sh
make db-up
make db-migrate
make db-board-test
make db-psql NODE=primary
```

psqlでテーブルと定義を確認できる。

```sql
\dt board.*
\d board.posts
```

`db-migrate` は常にprimaryへ管理用ユーザーで接続し、DDLを実行する。
作成したテーブルはPostgreSQLが2台のレプリカへ複製するため、レプリカにCREATE TABLEを発行しない。
現在の `lab.posts` と既存データは変更しない。

新しいデータボリュームでは `init/15-board.sql` が同じマイグレーションを初期化時に適用する。
起動済み・初期化済みのDBには `make db-migrate` で追加する。通常の `db-up` では既存DBへ新しいDDLを自動適用しない。

## データモデル

`board` は同じ `ddia` データベース内の名前空間（スキーマ）。別DBやパーティションではない。

| テーブル | 列 |
|---|---|
| `board.users` | `id`, `name`, `created_at` |
| `board.threads` | `id`, `title`, `author_id`, `created_at` |
| `board.posts` | `id`, `thread_id`, `author_id`, `body`, `reply_to`, `created_at` |

- 全IDは `bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY`。INSERT時にDBが採番し、`RETURNING id` で受け取る。
- `author_id` は `users.id`、`thread_id` は `threads.id`、`reply_to` は `posts.id` を参照する。
- 通常の投稿では `reply_to` をNULLにできる。それ以外の列は必須。表示名の重複は許容し、認証機能は追加していない。
- 参照先が存在しなければINSERTを拒否する。参照中のユーザー・スレッド・返信先を削除しても連鎖削除せず、外部キー違反になる。
- 返信先の存在を保証するが、同じスレッドへの返信に限定する追加ルールは設けていない。
- `created_at` はタイムゾーン付きの作成時刻。`posts(thread_id, id)` にスレッド単位の取得用インデックスがある。
- IDは投稿の識別子であり、連続性やコミット順を表さない。ロールバックでも番号が消費される。時刻やIDをLSNの代わりに反映確認へ使わない。

DDLと適用履歴は `ddia_admin` が所有する。`ddia_app` は掲示板データのSELECT/INSERT/UPDATE/DELETEと採番を利用できるが、テーブル作成や変更はできない。

## 書き込みの例

`make db-psql NODE=primary` のSQL操作用ユーザーで実行する。
以下は新しいユーザー・スレッド・投稿を実際に保存する例。事前の固定ユーザーやID指定は不要。

```sql
BEGIN;
INSERT INTO board.users (name)
VALUES ('alice') RETURNING id AS user_id \gset

INSERT INTO board.threads (title, author_id)
VALUES ('最初のスレッド', :user_id) RETURNING id AS thread_id \gset

INSERT INTO board.posts (thread_id, author_id, body)
VALUES (:thread_id, :user_id, 'こんにちは') RETURNING id AS post_id \gset
COMMIT;

SELECT p.id, t.title, u.name, p.body
FROM board.posts p
JOIN board.threads t ON t.id = p.thread_id
JOIN board.users u ON u.id = p.author_id
WHERE p.id = :post_id;
```

このSQLのGoからの実行、エンドポイント、セッションへのLSN保存は次の段階。
既存Goモデルの文字列IDやメモリ内採番は今回変更していないため、まだこのテーブルには接続されていない。

## マイグレーションとテスト

[`migrate.sql`](migrate.sql) はDDLと `ddia_migrations.applied` の適用記録を同じトランザクションでコミットする。
失敗するとまとめてロールバックする。並行した適用はトランザクション単位のロックで直列化する。
適用済みの `001_board` は再実行してもスキップし、テーブルやデータを上書きしない。
履歴なしに同名の `board` スキーマがある場合はエラーで停止する。既存データを消して強制適用しない。

今後定義を変更する場合は、適用済みの `001_board.sql` を書き換えず、次のSQLと適用処理を追加する。
通常の `db-down` はデータ保持。テーブル追加のためにボリューム削除・再初期化は不要。

`make db-board-test` は、SQL操作用ユーザーで以下を確認する。

- 3テーブルとIDENTITY列、作成日時、インデックス。
- ユーザー→スレッド→投稿→返信のINSERTとJOIN、UPDATE/DELETE。
- 不正な外部キー、必須値欠落、明示的なID指定を拒否すること。
- アプリ用ユーザーがDDLを実行できないこと。
- 期限付きのLSN確認後、両レプリカにもテーブルがあり、書き込みを拒否すること。

制約検証の行はトランザクションをロールバックして残さない。採番だけは進むことがある。
レプリカの適用停止・再開はこのテストでは行わない。停止中なら期限切れで失敗する。
既存の `make db-test` は引き続き `lab.posts` を使うレプリケーションラグ実験で、掲示板の制約テストとは独立している。
