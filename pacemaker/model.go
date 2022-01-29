package pacemaker

import (
	"github.com/tncheng/multipipelinehs/blockchain"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/types"
)

func NewTC(view types.View, seq types.Seq, requesters map[identity.NodeID]*blockchain.TMO) *blockchain.TC {
	// TODO: add crypto
	return &blockchain.TC{View: view, Seq: seq}
}
