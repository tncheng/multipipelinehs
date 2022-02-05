package mthotstuff

import (
	"fmt"
	"sync"

	"github.com/tncheng/multipipelinehs/blockchain"
	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/crypto"
	"github.com/tncheng/multipipelinehs/election"
	"github.com/tncheng/multipipelinehs/log"
	"github.com/tncheng/multipipelinehs/message"
	"github.com/tncheng/multipipelinehs/node"
	"github.com/tncheng/multipipelinehs/pacemaker"
	"github.com/tncheng/multipipelinehs/types"
)

const FORK = "fork"

type MTchs struct {
	node.Node
	election.Election
	pm              *pacemaker.Pacemaker
	lastVotedView   types.View
	preferredView   types.View
	lastVotedSeq    types.Seq
	preferredSeq    types.Seq
	bc              *blockchain.BlockChain
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	proposeChan     chan types.SignalV
	selfVote        chan interface{}
	selfTmo         chan interface{}
	bufferedQCs     map[crypto.Identifier]*blockchain.QC
	bufferedBlocks  map[types.View]*blockchain.Block
	highQC          *blockchain.QC
	secHighQC       *blockchain.QC
	highTC          *blockchain.TC
	secHighTC       *blockchain.TC
	highBlock       *blockchain.Block

	mu sync.Mutex
}

func NewMTchs(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block,
	selfTmo chan interface{},
	selfVote chan interface{}) *MTchs {
	mth := new(MTchs)
	mth.Node = node
	mth.Election = elec
	mth.pm = pm
	mth.bc = blockchain.NewBlockchain(config.GetConfig().N())
	mth.bufferedBlocks = make(map[types.View]*blockchain.Block)
	mth.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	mth.highQC = &blockchain.QC{View: 0}
	mth.secHighQC = &blockchain.QC{View: 0, Seq: 0}
	mth.highTC = nil
	mth.secHighTC = nil
	mth.highBlock = &blockchain.Block{Seq: 1}
	mth.committedBlocks = committedBlocks
	mth.forkedBlocks = forkedBlocks
	mth.selfVote = selfVote
	mth.selfTmo = selfTmo
	mth.proposeChan = make(chan types.SignalV, 4)
	return mth
}

func (mth *MTchs) ProcessBlock(block *blockchain.Block) error {
	log.Debugf("[%v] is processing block, view: %v, id: %x", mth.ID(), block.View, block.ID)
	defer func() {
		log.Debugf("[%v] processing block over, view: %v, Seq: %v, id: %x", mth.ID(), block.View, block.Seq, block.ID)
	}()
	curView := mth.pm.GetCurView()
	if block.Proposer != mth.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", mth.ID())
		}
	}
	if block.View > curView+1 {
		//	buffer the block
		mth.bufferedBlocks[block.View-1] = block
		log.Debugf("[%v] the block is buffered, view: %v, current view is: %v, id: %x", mth.ID(), block.View, curView, block.ID)
		return nil
	}
	if block.QC != nil {
		mth.updateHighQC(block.QC)
	} else {
		return fmt.Errorf("the block should contain a QC")
	}
	// does not have to process the QC if the replica is the proposer
	if block.Proposer != mth.ID() {
		mth.processCertificate(block.QC)
	}

	asCollector := block.TC == nil
	if !asCollector && block.Proposer != mth.ID() {
		var maxView types.View
		if block.TCs[1].View > block.TCs[0].View {
			maxView = block.TCs[1].View
		} else {
			maxView = block.TCs[0].View
		}
		if curView <= block.TC.View {
			log.Debugf("validator advance view %v by tc", block.TC.View)
			switch block.TC.View {
			case maxView - 1:
				mth.pm.AdvanceView(block.TC.View, block.TC.Seq, types.TimeoutF)
			case maxView:
				mth.pm.AdvanceView(block.TC.View, block.TC.Seq, types.TimeoutS)
				mth.updateHighTC(nil)
				mth.updateHighTC(nil)
			}
		}
	}

	curView = mth.pm.GetCurView()
	if block.View < curView {
		log.Warningf("[%v] received a stale proposal from %v, block view: %v, current view: %v, block id: %x", mth.ID(), block.Proposer, block.View, curView, block.ID)
		return nil
	}
	if !mth.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}
	mth.bc.AddBlock(block)

	// update highblock if the block's signature is valid
	mth.updateHighBlock(block)

	// find next leader to collect vote for previous block
	nextView := curView + 1
	if mth.IsLeader(mth.ID(), nextView) {
		if asCollector {
			log.Debugf("[%v] send siganl to collector of view: %v", mth.ID(), nextView)
			mth.proposeChan <- types.SignalV{Ope: types.Continue, View: curView}
		} else {
			var maxTC *blockchain.TC
			if block.TCs[1].View > block.TCs[0].View {
				maxTC = block.TCs[1]
			} else {
				maxTC = block.TCs[0]
			}
			if curView <= maxTC.View {
				log.Debugf("[%v] tc advance view: %v", mth.ID(), maxTC.View)
				mth.pm.AdvanceView(maxTC.View, maxTC.Seq, types.TimeoutS)
				mth.updateHighTC(block.TCs[0])
				mth.updateHighTC(block.TCs[1])
			} else {
				log.Debugf("[%v] send signal to collector of view: %v", mth.ID(), nextView)
				mth.proposeChan <- types.SignalV{Ope: types.Continue, View: curView}
			}

		}

	}
	// check commit rule
	qc := block.QC
	if qc.View > 3 && qc.Seq+2 == block.Seq {
		ok, b, _ := mth.commitRule(qc)
		if !ok {
			return nil
		}
		committedBlocks, forkedBlocks, err := mth.bc.CommitBlock(b.ID, mth.pm.GetCurSeq())
		if err != nil {
			return fmt.Errorf("[%v] cannot commit blocks", mth.ID())
		}
		for _, cBlock := range committedBlocks {
			mth.committedBlocks <- cBlock
		}
		for _, fBlock := range forkedBlocks {
			mth.forkedBlocks <- fBlock
		}
	}

	// process buffered QC
	qc, ok := mth.bufferedQCs[block.ID]
	if ok {
		log.Debugf("[%v] run qc in buffer in view:%v, qc's view:%x", mth.ID(), curView-1, qc.BlockID)
		mth.processCertificate(qc)
		delete(mth.bufferedQCs, block.ID)
	}

	shouldVote, err := mth.votingRule(block)
	if err != nil {
		log.Errorf("cannot decide whether to vote the block, %w", err)
		return err
	}
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", mth.ID(), block.ID)
		return nil
	}
	vote := blockchain.MakeVote(block.View, block.Seq, mth.ID(), block.ID)
	// vote to the next leader
	voteAggregator := mth.FindLeaderFor(block.View + 1)
	log.Debugf("[%v] knows the leader of view: %v is: %v", mth.ID(), block.View+2, voteAggregator)
	if voteAggregator == mth.ID() {
		log.Debugf("[%v] vote is sent to itself, id: %x", mth.ID(), vote.BlockID)
		mth.selfVote <- *vote
	} else {
		log.Debugf("[%v] vote is sent to %v, id: %x", mth.ID(), voteAggregator, vote.BlockID)
		mth.Send(voteAggregator, vote)
	}
	log.Debugf("[%v] vote is sent, id: %x", mth.ID(), vote.BlockID)

	b, ok := mth.bufferedBlocks[block.View]
	if ok {
		_ = mth.ProcessBlock(b)
		delete(mth.bufferedBlocks, block.View)
	}
	return nil
}

func (mth *MTchs) ProcessVote(vote *blockchain.Vote) {
	log.Debugf("[%v] is processing the vote from %v, block id: %x", mth.ID(), vote.Voter, vote.BlockID)
	if mth.ID() != vote.Voter {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Fatalf("[%v] Error in verifying the signature in vote id: %x", mth.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with unvalid signature. vote id: %x", mth.ID(), vote.BlockID)
			return
		}
	}
	isBuilt, qc := mth.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] not sufficient votes to build a QC, block id: %x", mth.ID(), vote.BlockID)
		return
	}
	qc.Leader = mth.ID()

	for v := range mth.proposeChan {
		if v.Ope == types.Continue {
			break
		} else {
			if v.View > qc.View {
				log.Debugf("[%v] drop qc for %v", mth.ID(), qc.View)
			} else {
				log.Debugf("[%v] skip invalid drop signal for view %v", mth.ID(), qc.View)
				continue
			}
		}
	}

	_, err := mth.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		mth.bufferedQCs[qc.BlockID] = qc
		return
	}
	mth.processCertificate(qc)
}

func (mth *MTchs) ProcessRemoteTmo(tmo *blockchain.TMO) {
	log.Debugf("[%v] is processing tmo from %v", mth.ID(), tmo.NodeID)
	mth.processCertificate(tmo.HighQC)
	if tmo.View < mth.pm.GetCurView() {
		return
	}
	isBuilt, ftc, stc := mth.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		log.Debugf("[%v] not enough tc for %v", mth.ID(), tmo.View)
		return
	}
	log.Debugf("[%v] a tc is built for seq %v, %v", mth.ID(), ftc.View, stc.View)
	mth.updateHighTC(ftc)
	mth.updateHighTC(stc)
	mth.processTC(ftc)
}

func (mth *MTchs) ProcessLocalTmo(view types.View, seq types.Seq) {
	// mth.pm.AdvanceView(view + 1)
	tmo := &blockchain.TMO{
		View:   view + 1,
		FSeq:   seq,
		SSeq:   seq + 1,
		NodeID: mth.ID(),
		HighQC: mth.GetHighQC(),
	}
	mth.Broadcast(tmo)
	// mth.ProcessRemoteTmo(tmo)
	mth.selfTmo <- *tmo
}

func (mth *MTchs) MakeProposal(view types.NewViewType, payload []*message.Transaction) *blockchain.Block {
	log.Debugf("[%v] make proposal for view:%v", mth.ID(), view)
	var qc *blockchain.QC
	var tc *blockchain.TC
	var tcs []*blockchain.TC
	var preId crypto.Identifier
	switch view.ProposeType {
	case types.NoTimeout:
		qc = mth.GetHighQC()
		preId = mth.GetHighBlock().ID
	case types.TimeoutF:
		qc = mth.GetSecHighQC()
		preId = mth.GetHighQC().BlockID
		tc = mth.GetSecHighTC()
		tcs = append(tcs, tc, mth.GetHighTC())
		//drop qc when timeout
		log.Debugf("[%v] send drop signal for view:%v", mth.ID(), view)
		mth.proposeChan <- types.SignalV{Ope: types.Drop, View: view.View}
	case types.TimeoutS:
		qc = mth.GetHighQC()
		preId = mth.GetHighBlock().ID
		tc = mth.GetHighTC()
		tcs = append(tcs, mth.GetSecHighTC(), tc)
		mth.updateHighTC(nil)
		mth.updateHighTC(nil)
	}
	block := blockchain.MakeBlock(view.View, view.Seq, qc, tc, tcs, preId, payload, mth.ID())
	return block
}

func (mth *MTchs) forkChoice() *blockchain.QC {
	choice := mth.GetHighQC()
	// to simulate TC under forking attack
	choice.View = mth.pm.GetCurView() - 1
	return choice
}

func (mth *MTchs) processTC(tc *blockchain.TC) {
	log.Debugf("[%v] processing tc, view:%v", mth.ID(), tc.View)
	if tc.View < mth.pm.GetCurView() {
		return
	}
	mth.pm.AdvanceView(tc.View, tc.Seq, types.TimeoutF)
}

func (mth *MTchs) GetChainStatus() string {
	chainGrowthRate := mth.bc.GetChainGrowth()
	blockIntervals := mth.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", mth.ID(), mth.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (mth *MTchs) GetHighQC() *blockchain.QC {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	return mth.highQC
}

func (mth *MTchs) GetSecHighQC() *blockchain.QC {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	return mth.secHighQC
}

func (mth *MTchs) updateHighQC(qc *blockchain.QC) {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	if qc.View > mth.highQC.View {
		mth.secHighQC = mth.highQC
		mth.highQC = qc
	}
}

func (mth *MTchs) updateHighTC(tc *blockchain.TC) {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	mth.secHighTC = mth.highTC
	mth.highTC = tc

}

func (mth *MTchs) updateHighBlock(bk *blockchain.Block) {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	if bk.View > mth.highBlock.View {
		mth.highBlock = bk
	}
}

func (mth *MTchs) GetHighTC() *blockchain.TC {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	return mth.highTC
}

func (mth *MTchs) GetSecHighTC() *blockchain.TC {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	return mth.secHighTC
}

func (mth *MTchs) GetHighBlock() *blockchain.Block {
	mth.mu.Lock()
	defer mth.mu.Unlock()
	return mth.highBlock
}

func (mth *MTchs) processCertificate(qc *blockchain.QC) {
	log.Debugf("[%v] is processing a QC, block id: %x", mth.ID(), qc.BlockID)
	if qc.View < mth.pm.GetCurView() {
		return
	}
	if qc.Leader != mth.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if !quorumIsVerified {
			log.Warningf("[%v] received a quorum with invalid signatures", mth.ID())
			return
		}
	}
	if mth.IsByz() && config.GetConfig().Strategy == FORK && mth.IsLeader(mth.ID(), qc.View+1) {
		mth.pm.AdvanceView(qc.View, qc.Seq, types.NoTimeout)
		return
	}
	err := mth.updatePreferredSeq(qc)
	if err != nil {
		mth.bufferedQCs[qc.BlockID] = qc
		log.Debugf("[%v] a qc is buffered, view: %v, id: %x", mth.ID(), qc.View, qc.BlockID)
		return
	}
	mth.updateHighQC(qc)
	mth.pm.AdvanceView(qc.View, qc.Seq, types.NoTimeout)
}

// TODO:voterule check
func (mth *MTchs) votingRule(block *blockchain.Block) (bool, error) {
	if block.View < 3 {
		return true, nil
	}
	parentBlock, err := mth.bc.GetBlockByID(block.QC.BlockID)
	if err != nil {
		return false, fmt.Errorf("cannot vote for block: %w", err)
	}
	if (block.Seq <= mth.lastVotedSeq) || (parentBlock.Seq < mth.preferredSeq) {
		return false, nil
	}
	return true, nil
}

func (mth *MTchs) commitRule(qc *blockchain.QC) (bool, *blockchain.Block, error) {
	// commit b0, if there are three adjacent certified block, such as: b0 <- b1
	b1, err := mth.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	b0, err := mth.bc.GetBlockByID(b1.QC.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	return true, b0, nil
}

func (mth *MTchs) updateLastVotedView(targetView types.View) error {
	if targetView < mth.lastVotedView {
		return fmt.Errorf("target view is lower than the last voted view")
	}
	mth.lastVotedView = targetView
	return nil
}

func (mth *MTchs) updatePreferredView(qc *blockchain.QC) error {
	if qc.View < 2 {
		return nil
	}
	_, err := mth.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	if qc.View > mth.preferredView {
		log.Debugf("[%v] preferred view has been updated to %v", mth.ID(), qc.View)
		mth.preferredView = qc.View
	}
	return nil
}

func (mth *MTchs) updatePreferredSeq(qc *blockchain.QC) error {
	// update 1-chain block if there are two adjacent block, such that: b0
	if qc.Seq <= 2 {
		return nil
	}
	_, err := mth.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred seq: %w", err)
	}
	if qc.Seq > mth.preferredSeq {
		mth.preferredSeq = qc.Seq
	}
	return nil
}

func (mth *MTchs) updateLastVotedSeq(targetSeq types.Seq) error {
	if targetSeq < mth.lastVotedSeq {
		return fmt.Errorf("target view is lower than the last voted view")
	}
	mth.lastVotedSeq = targetSeq
	return nil
}
