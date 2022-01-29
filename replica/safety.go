package replica

import (
	"github.com/tncheng/multipipelinehs/blockchain"
	"github.com/tncheng/multipipelinehs/message"
	"github.com/tncheng/multipipelinehs/types"
)

type Safety interface {
	ProcessBlock(block *blockchain.Block) error
	ProcessVote(vote *blockchain.Vote)
	ProcessRemoteTmo(tmo *blockchain.TMO)
	ProcessLocalTmo(view types.View, seq types.Seq)
	MakeProposal(newView types.NewViewType, payload []*message.Transaction) *blockchain.Block
	GetChainStatus() string
}
