package bbs

// 履歴。チェッカが見ていい唯一のもの。
// invoke / ok の 2 点で記録するのは、実並行に移したとき呼び出しと復帰が別の時刻になるから。
// 今は同時刻だが、形を先に合わせておく。

type Phase int

const (
	Invoke Phase = iota
	Ok
)

type Event struct {
	T     int
	Phase Phase
	User  UserID
	Op    string

	// 書き込み時
	Wrote *Post

	// 読み取り時。Nodes はパーティションごとにどの複製を読んだか。
	// Thread は開いたスレ。Seen が空でも、何を見に行ったかが履歴に残る。
	Seen   []Post
	Nodes  map[int]int
	Thread ThreadID
}
