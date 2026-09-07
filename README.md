# DDIA labs

[Designing Data-Intensive Applications](https://dataintensive.net/) を読みながら、本に出てくる異常をシミュレーションと実DBで再現する実験場。
動くものを作ることが目的ではなく、実際の状況を再現して、どう対処するのかを考えることが目的。

## 実DBで始める: PostgreSQL 3台

プライマリ1台と読み取り専用レプリカ2台で、SQLによる書き込み・反映位置・古い読み取りを確認する。
パーティショニングは行わない。Dockerが動いていれば、ホストのpsqlやGoのビルドは不要。

```sh
make db-up
make db-test
make db-status
make db-psql NODE=primary
```

接続先は `127.0.0.1:15432`（primary）、`:15433`（replica1）、`:15434`（replica2）。
停止は `make db-down`（データ保持）。[接続情報・生のSQL・手動実験手順](labs/postgres_replication/README.md)を参照。
今回はDBとSQLまでで、既存の掲示板アプリやRouterには接続していない。

掲示板用の `board.users`・`board.threads`・`board.posts` は `make db-migrate` で作成し、
`make db-board-test` で採番・外部キー・権限・テーブルの複製を確認できる。
[テーブル定義とINSERTの例](infra/postgres/README.md)を参照。既存データの削除は不要。

## シミュレーションの構成

掲示板が 1 つあり、それが複数のデータベースに分かれて保存され、複製が遅れる。
アプリは分割も遅延も知らない。ラボは章ごとの**シナリオと不変条件の検査**だけを持つ。

| パッケージ | 層 | 知ってはいけないもの |
|---|---|---|
| `app/` | **実際の掲示板アプリ。** User / Thread / Post、HTTP、サービス、Repository | レプリカ、遅延、仮想時計、乱数 |
| `fake/database/` | **偽DBクラスタ。** ルーティング、保存ログ、適用位置、複製先 | 実PostgreSQL接続 |
| `fake/transport/` | **偽の複製配送。** メッセージ、遅延、喪失、順序 | 実ネットワーク |
| `fake/simulator/` | **実験実行。** 仮想セッション、履歴、時計、乱数、各層の組み立て | 実アプリのRepository |
| `fake/replication_lag/` | シミュレーションのシナリオと検査 | — |

依存は一方向で循環しない。`fake/database` はドメイン型だけを `app` から使い、
`fake/transport` は `fake/database`、`fake/simulator` がそれらを組み立てる。

## シミュレーションの1リクエストの境界

書き込みでは、エンドポイントがDBから受け取った位置をユーザーセッションへ記録する。

```
fake/simulator.Application.Reply(session, ...)
  → fake/simulator.Repository.AddPost
  → fake/simulator.Database
  → fake/database.Router.RouteWrite
  → fake/database.Cluster.Append(Location, Record)
  → fake/database.Replicator.Targets
  → fake/transport.Send(Message{Destination: follower})
  ← fake/database.WriteResult{Partition, Position}
  → session.RecordWrite
```

`Router`は候補ではなく最終的な`Location`を返す。読み書きの実行は`fake/simulator.Database`、
指定された場所の状態変更は`fake/database.Cluster`が担当する。

```
fake/simulator.Application.ViewThread(session, ...)
  → fake/simulator.Database
  → fake/database.ReadYourWritesRouter.RouteRead(requiredPosition)
  → 条件を満たすLocationを決定
  → fake/database.Cluster.Log(Location)
```

複製は`Replicator`が宛先を決め、`Transport`は指定された1宛先へ送るだけである。
`fake/simulator.Sim.Advance`が配送済みメッセージを取り出し、偽DBへ適用する。

将来の実DBでは、Topologyを実ノードの状態へ接続し、Locationを接続プールへ対応づける。
シミュレーションと実DBで接続処理が違っても、Routerが「最終接続先を決める」という境界は変えない。

## シミュレーションで決めてあること

- **掲示板は 1 つ。板はない。** 分割はドメインではなくインフラ側の方針。
  異常を出すためにドメインを変える必要が出たら、層の切り方が間違っている。
- **乱数は層ごとに 1 本**（workload / delay / routing / fault）。1 本を共有すると、
  ルータを差し替えただけで遅延と負荷が変わり、対策の効果が測れなくなる。
- **`fake/simulator.Sim.Advance` 以外で複製を進めない。** アプリもテストも配送に触らない。
- **Storageは保存先を選ばない。Transportは複製先を増やさない。** どちらも明示されたLocationだけを扱う。
- **チェッカは履歴だけを見る。** 真値を知るためにアプリを歪めない。
- Phase 1 は決定的シミュレーション。**最終的には実際の並行処理に寄せる**。

## ラボ

- [`fake/replication_lag/`](fake/replication_lag/) — 偽DBによる5.2 レプリケーションラグ
- [`labs/postgres_replication/`](labs/postgres_replication/) — 実PostgreSQLで同じデータの複製・適用位置をSQL確認（`make db-test`）

Goシミュレーションのテスト:

```sh
make test
```
