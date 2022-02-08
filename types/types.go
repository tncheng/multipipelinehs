package types

type View int
type Seq int

type NewViewType struct {
	View
	Seq
	ProposeType int
}

const (
	NoTimeout = iota
	TimeoutF
	TimeoutS
)

const (
	Continue = iota
	Drop
)

type SignalV struct {
	Ope  int
	View View
}

type StateContext struct {
	VoteType        int
	PreferredSeq    Seq
	SecPreferredSeq Seq
	LastVotedSeq    Seq
}
