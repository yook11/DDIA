# 5.2 レプリケーションラグ

掲示板の下でレプリカが遅れる。異常は手で再演せず、普通に投稿して普通に読んだら壊れる形で出す。

配管は `fake` 配下にある。ここに置くのは
**このラボのシナリオと不変条件の検査だけ**。

## 再現したい異常

| 不変条件 | シナリオ | 必要なもの |
|---|---|---|
| リードアフターライト | レスを書いてスレを開き直すと自分のレスがない | `simulator.Session.RequiredPositions` |
| 単調読み取り | リロードでレス数が 50 → 45 に減る | 複数レプリカ、ランダムなルーティング |
| 一貫性のあるプレフィックス | 「そうだね」が元の質問より先に見える | `Post.ReplyTo` **かつ** `database.ByPost` + `GET /recent` |

3 つ目は分割していないと原理的に出ない。同じパーティションの中では順序が保たれるため
（`database.TestReplicaAppliesInOrder`）。対策は「因果のあるデータを同じパーティションに置く」
＝ `fake/database.ByThread` に戻すこと。

## 差し替え点

| 何を差し替える | 何が変わる |
|---|---|
| `simulator.Repository` | 偽アプリから見たDB操作の境界 |
| `database.LeaderRouter` / `RandomRouter` / `ReadYourWritesRouter` | DB位置の選び方。同じ検査が fail ⇄ pass で反転する |
| `database.Partitioner` | 書き込み先パーティション。異常3が出るかどうか |
| `database.Replicator` | どのフォロワーへ複製するか |
| `transport.Delay` | 遅延の性質。`Slow` で 1 台だけ恒常的に遅くできる |

## まだ無いもの

- `StickyRouter`。単調読み取りの対策。`fake/database/router.go` に足す
- 一貫性のあるプレフィックスの検査
- ack、ノード死、分断。それを壊す不変条件がテーマになってから載せる

```sh
make test
```
