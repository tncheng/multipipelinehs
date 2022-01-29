package blockchain

import (
	"fmt"
	"sync"

	"github.com/tncheng/multipipelinehs/crypto"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/log"
	"github.com/tncheng/multipipelinehs/types"
)

type Vote struct {
	types.View
	types.Seq
	Voter   identity.NodeID
	BlockID crypto.Identifier
	crypto.Signature
}

type QC struct {
	Leader  identity.NodeID
	View    types.View
	Seq     types.Seq
	BlockID crypto.Identifier
	Signers []identity.NodeID
	crypto.AggSig
	crypto.Signature
}

type TMO struct {
	View   types.View
	FSeq   types.Seq
	SSeq   types.Seq
	NodeID identity.NodeID
	HighQC *QC
}

type TC struct {
	View types.View
	Seq  types.Seq
	crypto.AggSig
	crypto.Signature
}

type Quorum struct {
	total int
	votes map[crypto.Identifier]map[identity.NodeID]*Vote
	mu    sync.Mutex
}

func MakeVote(view types.View, seq types.Seq, voter identity.NodeID, id crypto.Identifier) *Vote {
	// TODO: uncomment the following
	sig, err := crypto.PrivSign(crypto.IDToByte(id), voter, nil)
	if err != nil {
		log.Fatalf("[%v] has an error when signing a vote", voter)
		return nil
	}
	// TODO: change seq to real seq in block and vote, note caculate from view.
	return &Vote{
		View:      view,
		Seq:       seq,
		Voter:     voter,
		BlockID:   id,
		Signature: sig,
	}
}

func NewQuorum(total int) *Quorum {
	return &Quorum{
		total: total,
		votes: make(map[crypto.Identifier]map[identity.NodeID]*Vote),
	}
}

func (q *Quorum) VotesExist(id crypto.Identifier) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.votes[id]
	return ok
}

func (q *Quorum) RemoveVote(id crypto.Identifier) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.votes, id)

}

// Add adds id to quorum ack records
func (q *Quorum) Add(vote *Vote) (bool, *QC) {
	if vote.View == 0 {
		log.Debugf("the special vote is activated")
		return true, &QC{View: 1, Seq: 1}
	}
	if q.superMajority(vote.BlockID) {
		return false, nil
	}
	q.mu.Lock()
	_, exist := q.votes[vote.BlockID]
	if !exist {
		//	first time of receiving the vote for this block
		q.votes[vote.BlockID] = make(map[identity.NodeID]*Vote)
	}
	q.votes[vote.BlockID][vote.Voter] = vote
	q.mu.Unlock()
	if q.superMajority(vote.BlockID) {
		aggSig, signers, err := q.getSigs(vote.BlockID)
		if err != nil {
			log.Warningf("cannot generate a valid qc, view: %v, block id: %x: %w", vote.View, vote.BlockID, err)
		}
		qc := &QC{
			View:    types.View(vote.Seq),
			Seq:     vote.Seq,
			BlockID: vote.BlockID,
			AggSig:  aggSig,
			Signers: signers,
		}
		return true, qc
	}
	return false, nil
}

// Super majority quorum satisfied
func (q *Quorum) superMajority(blockID crypto.Identifier) bool {
	return q.size(blockID) > q.total*2/3
}

// Size returns ack size for the block
func (q *Quorum) size(blockID crypto.Identifier) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.votes[blockID])
}

func (q *Quorum) getSigs(blockID crypto.Identifier) (crypto.AggSig, []identity.NodeID, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var sigs crypto.AggSig
	var signers []identity.NodeID
	_, exists := q.votes[blockID]
	if !exists {
		return nil, nil, fmt.Errorf("sigs does not exist, id: %x", blockID)
	}
	for _, vote := range q.votes[blockID] {
		sigs = append(sigs, vote.Signature)
		signers = append(signers, vote.Voter)
	}

	return sigs, signers, nil
}
