package blockchain

import (
	"time"

	"github.com/tncheng/multipipelinehs/crypto"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/message"
	"github.com/tncheng/multipipelinehs/types"
)

type Block struct {
	types.Seq
	types.View
	QC        *QC
	TC        *TC
	TCs       []*TC
	Proposer  identity.NodeID
	Timestamp time.Time
	Payload   []*message.Transaction
	PrevID    crypto.Identifier
	Sig       crypto.Signature
	ID        crypto.Identifier
	Ts        time.Duration
}

type rawBlock struct {
	types.Seq
	QC       *QC
	Proposer identity.NodeID
	Payload  []string
	PrevID   crypto.Identifier
	Sig      crypto.Signature
	ID       crypto.Identifier
}

// MakeBlock creates an unsigned block
func MakeBlock(view types.View, seq types.Seq, qc *QC, tc *TC,tcs []*TC, prevID crypto.Identifier, payload []*message.Transaction, proposer identity.NodeID) *Block {
	b := new(Block)
	b.View = view
	b.Seq = seq
	b.Proposer = proposer
	b.QC = qc
	b.TC = tc
	b.TCs=tcs
	b.Payload = payload
	b.PrevID = prevID
	b.makeID(proposer)
	return b
}

func (b *Block) makeID(nodeID identity.NodeID) {
	raw := &rawBlock{
		Seq:      b.Seq,
		QC:       b.QC,
		Proposer: b.Proposer,
		PrevID:   b.PrevID,
	}
	var payloadIDs []string
	for _, txn := range b.Payload {
		payloadIDs = append(payloadIDs, txn.ID)
	}
	raw.Payload = payloadIDs
	b.ID = crypto.MakeID(raw)
	// TODO: uncomment the following
	b.Sig, _ = crypto.PrivSign(crypto.IDToByte(b.ID), nodeID, nil)
}
