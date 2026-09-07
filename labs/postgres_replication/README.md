# PostgreSQLでレプリケーションラグを観察する

Goやシミュレーションを使わず、実際のPostgreSQLにSQLを発行するラボ。
今回は環境とSQL検証まで。掲示板アプリとの接続、セッション、読み取り先を選ぶRouterはまだ実装しない。

```text
INSERT → primary ── 非同期の物理ストリーミング複製 ──→ replica1（SELECT）
                └───────────────────────────────→ replica2（SELECT）
```

3台は同じデータのコピーを持つ。パーティショニングはしない。
WALの配送・適用はPostgreSQLが行い、Goの `sim` / `storage` / `transport` は通らない。
このラボではWAL適用を意図的に停止し、古い読み取りを確実に再現する。

掲示板用テーブルは別の `board` スキーマに置く。`make db-migrate` で追加、
`make db-board-test` で採番・制約・テーブルの複製を確認できる。
[掲示板のテーブル定義と操作例](../../infra/postgres/README.md)を参照。
このラボの `lab.posts` は引き続きSQL検証専用とし、Goアプリにはまだ接続しない。

## 起動と接続

Docker EngineとDocker Compose v2（`up --wait` 対応）が必要。
macOSではDocker Desktop等のエンジンを起動しておく。ホストのpsqlやGoのビルドは不要。
以下はリポジトリのルートで実行する。

```sh
make db-up
make db-status
make db-test
make db-psql NODE=primary
```

`db-up` は初期化済みのプライマリと、SQLを受け付ける2台のレプリカを確認して終了する。
各ノードの役割と `lab.posts` の存在をTCP接続で確認するため、初期化途中の一時サーバーは成功扱いにしない。
SQL受付は「最新の書き込みまで反映済み」と同義ではない。反映の確認はLSNで行う。

| NODE | ホストからの接続先 | 役割 |
|---|---|---|
| `primary` | `127.0.0.1:15432` | 書き込み・読み取り |
| `replica1` | `127.0.0.1:15433` | 読み取り専用 |
| `replica2` | `127.0.0.1:15434` | 読み取り専用 |

DB名は全ノード `ddia`。Compose内ではサービス名とポート `5432` で接続する。
ホストへの公開はloopbackのみ。プロジェクト名は `ddia-replication-lag` に固定し、
ネットワークと3つのデータボリュームを他のComposeプロジェクトから分離する。

| 用途 | ユーザー | パスワード |
|---|---|---|
| SQL操作（`db-psql` の既定） | `ddia_app` | `ddia_local_app` |
| 管理・適用制御（`ROLE=admin`） | `ddia_admin` | `ddia_local_admin` |
| レプリケーション接続 | `ddia_replicator` | `ddia_local_replication` |

**公開されたローカル学習専用の資格情報。実データ・本番・共有環境には使用しない。**
管理者はスーパーユーザー。SQL操作用ユーザーには管理・複製権限を与えず、複製ユーザーは通常のDB接続を許可しない。
TCP・UnixソケットともSCRAMパスワード認証を使う。TLSは今回構成しない。

```sh
make db-psql NODE=replica1
make db-psql NODE=replica1 ROLE=admin
make db-pause NODE=replica1
make db-resume NODE=replica1
make db-down
```

`db-down` はこの環境のコンテナとネットワークを停止・削除するが、データボリュームは保持する。
次の `db-up` は既存データから起動する。`NODE` は上表の3種類だけを許可し、primaryへの適用停止・再開は拒否する。
`db-test` の実行中に、別ターミナルから適用停止・再開やDDLを行わないこと。

## どのファイルを読むか

- [`../../infra/postgres/compose.yaml`](../../infra/postgres/compose.yaml): 3台の構成・ポート・PostgreSQL設定。
- [`../../infra/postgres/init/`](../../infra/postgres/init/): 初回のユーザー・テーブル・複製スロット作成。
- [`../../infra/postgres/replica-entrypoint.sh`](../../infra/postgres/replica-entrypoint.sh): 初回の `pg_basebackup` と既存データからの再起動。
- [`sql/`](sql/): ラッパーが実行するSQL。コンテナ内では `/opt/ddia-lab/sql/` にマウントされる。
- [`test.sh`](test.sh): 書き込み→停止中の古い読み取り→LSN到達→再開を確かめる自動実験。
- [`../../infra/postgres/common.sh`](../../infra/postgres/common.sh): 接続と期限付きポーリング。

## 生のSQLで追う

psqlでは `\q` で終了できる。既定のautocommitとREAD COMMITTEDを使い、
読み取りを古いREPEATABLE READトランザクションに閉じ込めない。

### 1. 役割と通常の複製を確認する

3台それぞれの `db-psql` で実行する。

```sql
SELECT pg_is_in_recovery();   -- primary: f、replica1/2: t
SHOW transaction_read_only;   -- primary: off、replica1/2: on
SELECT * FROM lab.posts ORDER BY id;
```

primaryの `ddia_app` 接続で投稿する。返されたIDを控え、両レプリカからそのIDをSELECTすると、
非同期に複製される様子を確認できる。直後にはまだ見えない場合がある。

```sql
INSERT INTO lab.posts (body) VALUES ('最初の投稿') RETURNING id;
```

### 2. replica1の適用を停止する

`make db-psql NODE=replica1 ROLE=admin` で実行する。

```sql
SELECT pg_wal_replay_pause();
SELECT pg_get_wal_replay_pause_state();
```

停止要求の成功だけでは不十分。状態が `paused` になるまで確認し、
`pause requested` のまま次へ進めない。30秒経っても止まらなければ実験を中断し、再開を試みる。
`make db-pause NODE=replica1` はこの確認まで自動で行う。
[PostgreSQLの適用制御仕様](https://www.postgresql.org/docs/18/functions-admin.html#FUNCTIONS-RECOVERY-CONTROL-TABLE)

### 3. primaryへコミットし、必要なWAL位置を取得する

primaryの `ddia_app` 接続で実行する。

```sql
BEGIN;
SET LOCAL synchronous_commit = on;
INSERT INTO lab.posts (body) VALUES ('replica1停止中の投稿') RETURNING id;
COMMIT;
SELECT pg_current_wal_lsn() AS required_lsn;
```

返された投稿IDと `required_lsn` を控える。位置を取得するのは **COMMIT後**。
この設定ではローカルのWAL永続化を待つが、`synchronous_standby_names` が空なのでレプリカの反映は待たない。
LSNは行番号ではなくWAL内の位置。今回のトークンは同じプライマリの他の書き込みも含み得る保守的な上限で、
当該トランザクション専用の正確なコミット位置ではない。
[WAL位置の関数](https://www.postgresql.org/docs/18/functions-admin.html#FUNCTIONS-ADMIN-BACKUP)

自動検証も同じ順番の [`sql/write.sql`](sql/write.sql) を使う。psqlからファイルで追う場合は以下でもよい。

```sql
\set body 'replica1停止中の投稿'
\i /opt/ddia-lab/sql/write.sql
```

### 4. 同じ投稿を3台で読む

各psqlで `123` を実際の投稿IDへ置き換える。

```sql
SELECT * FROM lab.posts WHERE id = 123;
```

primaryには存在し、停止中のreplica1には存在しない。
replica2では次のSQLの位置を実際の `required_lsn` へ置き換えて確認する。

```sql
SELECT pg_last_wal_receive_lsn() AS received_lsn,
       pg_last_wal_replay_lsn() AS applied_lsn;
SELECT COALESCE(pg_last_wal_replay_lsn() >= '0/12345678'::pg_lsn, false) AS caught_up;
```

`caught_up = t` になった後の新しいSELECTで投稿を読む。30秒経っても到達しなければ失敗として状態・ログを調べる。
比較は文字列や経過時間ではなく `pg_lsn` 型の大小で行う。
受信済みと適用済みは別。**読み取りに使う判定は `pg_last_wal_replay_lsn()`**。
停止中でも受信が進む場合があり、`received_lsn` だけでは投稿が見えることを確認できない。
[リカバリ位置の関数](https://www.postgresql.org/docs/18/functions-admin.html#FUNCTIONS-RECOVERY-INFO-TABLE)

### 5. replica1を必ず再開する

replica1の管理者接続で実行する。

```sql
SELECT pg_wal_replay_resume();
SELECT pg_get_wal_replay_pause_state(); -- not pausedになること
```

その後、手順4のLSN到達と投稿のSELECTをreplica1でも確認する。
`make db-resume NODE=replica1` も利用できる。最後に `make db-status` で両レプリカの状態を見る。

## 自動検証とデータ保持

`make db-test` は役割と読み取り専用制約、両レプリカへの通常複製、replica1停止中の可視性の差、
replica2の到達、replica1再開後の到達を検証する。判定は30秒の期限付きポーリングで、
0.2秒間隔で再確認する。固定の秒数を待っただけで成功にしない。SQLエラーは失敗になる。

自分が停止させたreplica1は、成功・失敗・INT/TERM/HUPでの中断時に再開を試みる。
SIGKILL、Docker停止、ホスト障害時には保証できない。失敗後は `db-status` を確認し、必要なら手動で再開する。
開始前から停止していたレプリカは変更せずテストを失敗させるため、先に手動で再開すること。
テストが追加した `db-test ...` の投稿は観察用に残す。既存の投稿は削除しない。

再起動・保持の確認例:

```sh
make db-test
make db-psql NODE=primary   # SELECT * FROM lab.posts ORDER BY id; でIDと本文を控える
make db-down
make db-up
make db-psql NODE=primary   # 同じIDと本文が残っていることを確認
make db-test               # 同じデータ上で再実行できる
```

## 初期化とトラブルシュート

公式 `postgres:18.6-bookworm` を共通使用する。PostgreSQL 18に合わせ、
ボリュームは `/var/lib/postgresql`、PGDATAは `/var/lib/postgresql/18/docker`。
初回だけbase backupを取得し、`standby.signal` と専用スロット名を保存する。
各スロットは作成時からWALを確保し、同時base backup中に開始位置を失わないようにする。
[公式イメージのデータ配置](https://github.com/docker-library/docs/blob/master/postgres/README.md)

初期化完了マーカーがない・PG_VERSIONが違う・不完全なbackup等はエラーで停止する。
再起動で勝手に消去・再コピーしない。`pg_basebackup --no-clean` で失敗時のファイルも残す。
初期化SQLは初回専用なので、SQLやパスワード設定を編集するだけでは既存DBは更新されない。

```sh
docker compose --project-name ddia-replication-lag --file infra/postgres/compose.yaml ps -a
docker compose --project-name ddia-replication-lag --file infra/postgres/compose.yaml logs --tail 100 primary replica1 replica2
```

WAL適用を長時間停止したまま書き込みを続けると、特にレプリカ側で未適用WALが蓄積する。
スロット保持の `max_slot_wal_keep_size=256MB` はディスク全体の厳密な上限ではない。
チェックポイント時に必要WALが削除されてスロットが使えなくなると、再開だけでは復旧せず再構築が必要になる場合がある。
実験後は早めに再開するか、環境全体を `db-down` で停止する。
[スロットとWAL保持の設定](https://www.postgresql.org/docs/18/runtime-config-replication.html)

### 明示的に全データを初期化し直す場合だけ

以下は通常の起動・停止に不要。**このラボの全投稿を失う操作で、バックアップなしでは復元できない。**
残したい投稿や失敗時の調査データを退避し、対象を確認したうえで利用者が明示的に実行する。
`docker volume prune` や他プロジェクトを含む一括削除は使わない。

```sh
make db-down
docker volume inspect ddia-replication-lag_primary-data ddia-replication-lag_replica1-data ddia-replication-lag_replica2-data
# 3つともLabelsのcom.docker.compose.projectがddia-replication-lagであることを確認。
docker volume rm ddia-replication-lag_primary-data ddia-replication-lag_replica1-data ddia-replication-lag_replica2-data
make db-up
```

## 次の段階

次はGoから3つの接続先へSQLを発行し、コミット後のLSNをセッションに記録する。
読み取り時はレプリカの適用LSNと比較し、追いついた接続先を選ぶ処理を学ぶ。
今回は読み取りルーティング、自動フェイルオーバー・Raft・アプリ独自の複製は追加していない。
LSN比較は同じプライマリの複製関係を前提とし、昇格や別クラスタ間の比較は対象外。
