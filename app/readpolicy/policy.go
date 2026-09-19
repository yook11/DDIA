package readpolicy

// Position は書き込みが記録された複製上の位置。
// 中身は接続先DBの実装(PostgreSQLならpg_lsn)が決め、app側は比較も解釈もしない。
// 空は「位置不明」。
type Position string

type Policy interface {
	isReadPolicy()
}

type ReadYourWrites struct {
	WritePosition Position
}
type Eventual struct{}

func (ReadYourWrites) isReadPolicy() {}
func (Eventual) isReadPolicy()       {}
