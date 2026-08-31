# DDIA labs

[Designing Data-Intensive Applications](https://dataintensive.net/) を読みながら、本に出てくる異常を Go で再現する実験場。
動くものを作ることが目的ではなく、実際の状況を再現して、どう対処するのかを考えることが目的。

## 構成

掲示板が 1 つあり、それが複数のデータベースに分かれて保存され、複製が遅れる。
アプリは分割も遅延も知らない。ラボは章ごとの**シナリオと不変条件の検査**だけを持つ。

| パッケージ | 層 | 知ってはいけないもの |
|---|---|---|
| `app/` | **掲示板アプリ。** User / Thread / Post、セッション、エンドポイント、履歴、DB操作の境界 | レプリカ、遅延、分割、時計、乱数 |
| `database/` | **DBルーティング。** 書き込み・読み取りの最終接続先と複製先を決める | Record、配送時刻、アプリの操作履歴 |
| `storage/` | **データの保存状態。** Record、レプリカのログ、適用位置、未適用Record | 時計、乱数、ネットワーク、保存先の選び方 |
| `transport/` | **1宛先への配送。** メッセージ、遅延、喪失、順序 | 接続先や複製先の選び方、アプリ |
| `sim/` | **シミュレーション実装。** DB操作、仮想時計、乱数ストリーム、各層の組み立て | — |
| `labs/*/` | 章ごとのシナリオと検査 | — |

依存は一方向で循環しない。`database` は `app`、`storage` は `app` と `database`、
`transport` は `app`、`database`、`storage` を参照し、`sim` がそれらを組み立てる。
`app` は他のローカルパッケージをimportしないため、掲示板からクラスタや遅延には手が届かない。

## 1リクエストの境界

書き込みでは、エンドポイントがDBから受け取った位置をユーザーセッションへ記録する。

```
app.Application.Reply(session, ...)
  → app.Repository.AddPost
  → sim.Database
  → database.Router.RouteWrite
  → storage.Cluster.Append(Location, Record)
  → database.Replicator.Targets
  → transport.Send(Message{Destination: follower})
  ← app.WriteResult{Partition, Position}
  → session.RecordWrite
```

`Router`は候補ではなく最終的な`Location`を返す。読み書きの実行は`sim.Database`、
指定された場所の状態変更は`storage.Cluster`が担当する。

```
app.Application.ViewThread(session, ...)
  → sim.Database
  → database.ReadYourWritesRouter.RouteRead(requiredPosition)
  → 条件を満たすLocationを決定
  → storage.Cluster.Log(Location)
```

複製は`Replicator`が宛先を決め、`Transport`は指定された1宛先へ送るだけである。
`sim.Advance`が配送済みメッセージを取り出し、Storageへ適用する。

将来の実DBでは、Topologyを実ノードの状態へ接続し、Locationを接続プールへ対応づける。
シミュレーションと実DBで接続処理が違っても、Routerが「最終接続先を決める」という境界は変えない。

## 決めてあること

- **掲示板は 1 つ。板はない。** 分割はドメインではなくインフラ側の方針。
  異常を出すためにドメインを変える必要が出たら、層の切り方が間違っている。
- **乱数は層ごとに 1 本**（workload / delay / routing / fault）。1 本を共有すると、
  ルータを差し替えただけで遅延と負荷が変わり、対策の効果が測れなくなる。
- **`sim.Advance` 以外で複製を進めない。** アプリもテストも配送に触らない。
- **Storageは保存先を選ばない。Transportは複製先を増やさない。** どちらも明示されたLocationだけを扱う。
- **チェッカは履歴だけを見る。** 真値を知るためにアプリを歪めない。
- Phase 1 は決定的シミュレーション。**最終的には実際の並行処理に寄せる**（`sim` の差し替え）。

## ラボ

- [`labs/replication_lag/`](labs/replication_lag/) — 5.2 レプリケーションラグ

```sh
make test
```
