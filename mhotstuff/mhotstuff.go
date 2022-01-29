package mhotstuff

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

type MHotStuff struct {
	node.Node
	election.Election
	pm              *pacemaker.Pacemaker
	lastVotedView   types.View
	lastVotedSeq    types.Seq
	preferredView   types.View
	preferredSeq    types.Seq
	highQC          *blockchain.QC
	highTC          *blockchain.TC
	secHighTC       *blockchain.TC
	secHighQC       *blockchain.QC
	highBlock       *blockchain.Block
	bc              *blockchain.BlockChain
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	proposeChan     chan types.SignalV
	selfVote        chan interface{}
	selfTmo         chan interface{}
	bufferedQCs     map[crypto.Identifier]*blockchain.QC
	bufferedBlocks  map[types.View]*blockchain.Block
	mu              sync.Mutex
}

func NewMHotStuff(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block, selfTmo chan interface{},
	selfVote chan interface{}) *MHotStuff {
	mhs := new(MHotStuff)
	mhs.Node = node
	mhs.Election = elec
	mhs.pm = pm
	mhs.bc = blockchain.NewBlockchain(config.GetConfig().N())
	mhs.bufferedBlocks = make(map[types.View]*blockchain.Block)
	mhs.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	mhs.highQC = &blockchain.QC{View: 0, Seq: 0}
	mhs.secHighQC = &blockchain.QC{View: 0, Seq: 0}
	mhs.highTC = nil
	mhs.secHighTC = nil
	mhs.highBlock = &blockchain.Block{Seq: 1}
	mhs.committedBlocks = committedBlocks
	mhs.forkedBlocks = forkedBlocks
	mhs.selfVote = selfVote
	mhs.selfTmo = selfTmo
	mhs.proposeChan = make(chan types.SignalV, 4)
	return mhs
}

func (mhs *MHotStuff) ProcessBlock(block *blockchain.Block) error {
	log.Debugf("[%v] is processing block from %v, view: %v, Seq: %v, qc's view:%v, id: %x,", mhs.ID(), block.Proposer.Node(), block.View, block.Seq, block.QC.View, block.ID)
	defer func() {
		log.Debugf("[%v] processing block over, view: %v, Seq: %v, id: %x", mhs.ID(), block.View, block.Seq, block.ID)
	}()

	curView := mhs.pm.GetCurView()
	if block.Proposer != mhs.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", mhs.ID())
		}
	}
	if block.View > curView+1 {
		//	buffer the block
		mhs.bufferedBlocks[block.View-1] = block
		log.Debugf("[%v] the block is buffered, id: %x", mhs.ID(), block.ID)
		return nil
	}
	if block.QC != nil {
		mhs.updateHighQC(block.QC)
	} else {
		return fmt.Errorf("the block should contain a QC")
	}
	// does not have to process the QC if the replica is the proposer
	if block.Proposer != mhs.ID() {
		mhs.processCertificate(block.QC)
	}

	// TODO: verify tc and tcs in block
	asCollector := block.TC == nil
	if !asCollector && block.Proposer != mhs.ID() {
		if curView <= block.TC.View {
			log.Debugf("validator advance view %v by tc", block.TC.View)
			mhs.pm.AdvanceView(block.TC.View, block.TC.Seq, types.TimeoutF)
		}
		var maxView types.View
		if block.TCs[1].View > block.TCs[0].View {
			maxView = block.TCs[1].View
		} else {
			maxView = block.TCs[0].View
		}
		if block.TC.View == maxView {
			mhs.updateHighTC(nil)
			mhs.updateHighTC(nil)
		}
	}

	curView = mhs.pm.GetCurView()
	if block.View < curView {
		log.Warningf("[%v] received a stale proposal from %v, block id: %x,current view:%v", mhs.ID(), block.Proposer, block.ID, curView)
		return nil
	}
	if !mhs.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}
	mhs.bc.AddBlock(block)

	// update highblock if the block's signature is valid
	mhs.updateHighBlock(block)

	// find next leader to collect vote for previous block
	nextView := curView + 1
	if mhs.IsLeader(mhs.ID(), nextView) {
		if asCollector {
			log.Debugf("[%v] send siganl to collector of view: %v", mhs.ID(), nextView)
			mhs.proposeChan <- types.SignalV{Ope: types.Continue, View: curView}
		} else {
			var maxTC *blockchain.TC
			if block.TCs[1].View > block.TCs[0].View {
				maxTC = block.TCs[1]
			} else {
				maxTC = block.TCs[0]
			}
			if curView <= maxTC.View {
				log.Debugf("[%v] tc advance view: %v", mhs.ID(), maxTC.View)
				mhs.pm.AdvanceView(maxTC.View, maxTC.Seq, types.TimeoutS)
				mhs.updateHighTC(block.TCs[0])
				mhs.updateHighTC(block.TCs[1])
			} else {
				log.Debugf("[%v] send signal to collector of view: %v", mhs.ID(), nextView)
				mhs.proposeChan <- types.SignalV{Ope: types.Continue, View: curView}
			}

		}

	}

	qc := block.QC
	if qc.Seq > 5 && qc.Seq+2 == block.Seq {
		ok, b, err := mhs.commitRule(qc)
		if !ok {
			log.Errorf("[%v] commit error:%v", mhs.ID(), err)
			return nil
		}
		// forked blocks are found when pruning
		committedBlocks, forkedBlocks, err := mhs.bc.CommitBlock(b.ID, mhs.pm.GetCurSeq())
		if err != nil {
			return fmt.Errorf("[%v] cannot commit blocks, %w", mhs.ID(), err)
		}
		for _, cBlock := range committedBlocks {
			mhs.committedBlocks <- cBlock
		}
		for _, fBlock := range forkedBlocks {
			mhs.forkedBlocks <- fBlock
		}
	}

	// process buffered QC
	qc, ok := mhs.bufferedQCs[block.ID]
	if ok {
		log.Debugf("[%v] run qc in buffer in view:%v, qc's view:%x", mhs.ID(), curView-1, qc.BlockID)
		mhs.processCertificate(qc)
		delete(mhs.bufferedQCs, block.ID)
	}

	shouldVote, err := mhs.votingRule(block)
	if err != nil {
		log.Errorf("[%v] cannot decide whether to vote the block, %w", mhs.ID(), err)
		return err
	}
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", mhs.ID(), block.ID)
		return nil
	}
	vote := blockchain.MakeVote(block.View, block.Seq, mhs.ID(), block.ID)
	// vote is sent to the next leader
	voteAggregator := mhs.FindLeaderFor(block.View + 2)
	log.Debugf("[%v] knows the leader of view: %v is: %v", mhs.ID(), block.View+2, voteAggregator)
	if voteAggregator == mhs.ID() {
		log.Debugf("[%v] vote is sent to itself, id: %x", mhs.ID(), vote.BlockID)
		mhs.selfVote <- *vote
	} else {
		log.Debugf("[%v] vote is sent to %v, id: %x", mhs.ID(), voteAggregator, vote.BlockID)
		mhs.Send(voteAggregator, vote)
	}

	b, ok := mhs.bufferedBlocks[block.View]
	if ok {
		_ = mhs.ProcessBlock(b)
		delete(mhs.bufferedBlocks, block.View)
	}
	return nil
}

func (mhs *MHotStuff) ProcessVote(vote *blockchain.Vote) {
	log.Debugf("[%v] is processing the vote, block id: %x", mhs.ID(), vote.BlockID)
	if vote.Voter != mhs.ID() {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Warningf("[%v] Error in verifying the signature in vote id: %x", mhs.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with invalid signature. vote id: %x", mhs.ID(), vote.BlockID)
			return
		}
	}
	isBuilt, qc := mhs.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] not sufficient votes to build a QC, block id: %x", mhs.ID(), vote.BlockID)
		return
	}
	log.Debugf("[%v] wait signal for process qc view:%v", mhs.ID(), qc.View)
	qc.Leader = mhs.ID()

	// wait for last view update
	// v := <-mhs.proposeChan

	for v := range mhs.proposeChan {
		if v.Ope == types.Continue {
			break
		} else {
			if v.View > qc.View {
				log.Debugf("[%v] drop qc for %v", mhs.ID(), qc.View)
				return
			} else {
				log.Debugf("[%v] skip invalid drop signal for view %v", mhs.ID(), qc.View)
				continue
			}
		}
	}

	// buffer the QC if the block has not been received
	_, err := mhs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		if qc.View != 1 {
			mhs.bufferedQCs[qc.BlockID] = qc
			return
		}
	}
	log.Debugf("[%v] got signal", mhs.ID())
	mhs.processCertificate(qc)
}

func (mhs *MHotStuff) ProcessRemoteTmo(tmo *blockchain.TMO) {
	log.Debugf("[%v] is processing tmo from %v", mhs.ID(), tmo.NodeID)
	mhs.processCertificate(tmo.HighQC)
	isBuilt, ftc, stc := mhs.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		return
	}
	log.Debugf("[%v] a tc is built for seq %v, %v", mhs.ID(), ftc.View, stc.View)
	mhs.updateHighTC(ftc)
	mhs.updateHighTC(stc)
	mhs.processTC(ftc)
}

func (mhs *MHotStuff) ProcessLocalTmo(view types.View, seq types.Seq) {
	// mhs.pm.AdvanceView(view, seq)
	tmo := &blockchain.TMO{
		View:   view + 1,
		FSeq:   seq,
		SSeq:   seq + 1,
		NodeID: mhs.ID(),
		HighQC: mhs.GetHighQC(),
	}
	mhs.Broadcast(tmo)
	mhs.selfTmo <- *tmo
	// mhs.ProcessRemoteTmo(tmo)
}

func (mhs *MHotStuff) MakeProposal(view types.NewViewType, payload []*message.Transaction) *blockchain.Block {
	log.Debugf("[%v] make proposal for view:%v", mhs.ID(), view)
	var qc *blockchain.QC
	var tc *blockchain.TC
	var tcs []*blockchain.TC
	var preId crypto.Identifier
	switch view.ProposeType {
	case types.NoTimeout:
		qc = mhs.GetHighQC()
		preId = mhs.GetHighBlock().ID
	case types.TimeoutF:
		qc = mhs.GetSecHighQC()
		preId = mhs.GetHighQC().BlockID
		tc = mhs.GetSecHighTC()
		tcs = append(tcs, tc, mhs.GetHighTC())
		// drop qc when timeout
		log.Debugf("[%v] send drop signal for view:%v", mhs.ID(), view)
		mhs.proposeChan <- types.SignalV{Ope: types.Drop, View: view.View}
	case types.TimeoutS:
		qc = mhs.GetHighQC()
		preId = mhs.GetHighBlock().ID
		tc = mhs.GetHighTC()
		tcs = append(tcs, mhs.GetSecHighTC(), tc)
		mhs.updateHighTC(nil)
		mhs.updateHighTC(nil)
	}
	block := blockchain.MakeBlock(view.View, view.Seq, qc, tc, tcs, preId, payload, mhs.ID())
	return block
}

func (mhs *MHotStuff) forkChoice(flag int) *blockchain.QC {
	var choice *blockchain.QC
	if !mhs.IsByz() || config.GetConfig().Strategy != FORK {
		return mhs.GetHighQC()
	}
	//	create a fork by returning highQC's parent's QC
	parBlockID := mhs.GetHighQC().BlockID
	parBlock, err := mhs.bc.GetBlockByID(parBlockID)
	if err != nil {
		log.Warningf("cannot get parent block of block id: %x: %w", parBlockID, err)
	}
	if parBlock.QC.View < mhs.preferredView {
		choice = mhs.GetHighQC()
	} else {
		choice = parBlock.QC
	}
	// to simulate TC's view
	choice.View = mhs.pm.GetCurView() - 1
	return choice
}

func (mhs *MHotStuff) processTC(tc *blockchain.TC) {
	log.Debugf("[%v] processing tc, view:%v", mhs.ID(), tc.View)
	if types.View(tc.View) < mhs.pm.GetCurView() {
		return
	}
	mhs.pm.AdvanceView(tc.View, tc.Seq, types.TimeoutF)
}

func (mhs *MHotStuff) GetHighQC() *blockchain.QC {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	return mhs.highQC
}

func (mhs *MHotStuff) GetSecHighQC() *blockchain.QC {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	return mhs.secHighQC
}

func (mhs *MHotStuff) updateHighTC(tc *blockchain.TC) {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	mhs.secHighTC = mhs.highTC
	mhs.highTC = tc
}

func (mhs *MHotStuff) GetHighTC() *blockchain.TC {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	return mhs.highTC
}

func (mhs *MHotStuff) GetSecHighTC() *blockchain.TC {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	return mhs.secHighTC
}

func (mhs *MHotStuff) GetHighBlock() *blockchain.Block {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	return mhs.highBlock
}

func (mhs *MHotStuff) GetChainStatus() string {
	chainGrowthRate := mhs.bc.GetChainGrowth()
	blockIntervals := mhs.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", mhs.ID(), mhs.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (mhs *MHotStuff) updateHighQC(qc *blockchain.QC) {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	if qc.View > mhs.highQC.View {
		mhs.secHighQC = mhs.highQC
		mhs.highQC = qc
	}
}
func (mhs *MHotStuff) updateHighBlock(bk *blockchain.Block) {
	mhs.mu.Lock()
	defer mhs.mu.Unlock()
	if bk.View > mhs.highBlock.View {
		mhs.highBlock = bk
	}
}

func (mhs *MHotStuff) processCertificate(qc *blockchain.QC) {
	if qc.View < mhs.pm.GetCurView() {
		return
	}
	if qc.Leader != mhs.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if !quorumIsVerified {
			log.Warningf("[%v] received a quorum with invalid signatures", mhs.ID())
			return
		}
	}
	if mhs.IsByz() && config.GetConfig().Strategy == FORK && mhs.IsLeader(mhs.ID(), qc.View+1) {
		mhs.pm.AdvanceView(qc.View, qc.Seq, 1)
		return
	}
	log.Debugf("[%v] is processing a QC, qc.view: %v,qc.seq:%v,block id: %x", mhs.ID(), qc.View, qc.Seq, qc.BlockID)
	// err := mhs.updatePreferredView(qc)
	err := mhs.updatePreferredSeq(qc)
	if err != nil {
		mhs.bufferedQCs[qc.BlockID] = qc
		log.Debugf("[%v] a qc is buffered, view: %v, seq: %v id: %x", mhs.ID(), qc.View, qc.Seq, qc.BlockID)
		return
	}
	mhs.updateHighQC(qc)
	mhs.pm.AdvanceView(qc.View, qc.Seq, types.NoTimeout)

}

func (mhs *MHotStuff) votingRule(block *blockchain.Block) (bool, error) {
	if block.View < 5 {
		return true, nil
	}
	parentBlock, err := mhs.bc.GetBlockByID(block.QC.BlockID)
	if err != nil {
		return false, fmt.Errorf("cannot vote for block: %w", err)
	}
	if (block.Seq <= mhs.lastVotedSeq) || (parentBlock.Seq < mhs.preferredSeq) {
		return false, nil
	}
	return true, nil
}

func (mhs *MHotStuff) commitRule(qc *blockchain.QC) (bool, *blockchain.Block, error) {

	// commit b0, if there are three adjacent certified block, such as: b0 <- b1 <- b2
	b2, err := mhs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	b1, err := mhs.bc.GetBlockByID(b2.QC.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	b0, err := mhs.bc.GetBlockByID(b1.QC.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	// if ((b0.Seq + 2) == b1.Seq) && ((b1.Seq + 2) == qc.Seq) {
	// 	return true, b0, nil
	// }
	return true, b0, nil
}

func (mhs *MHotStuff) updateLastVotedView(targetView types.View) error {
	if targetView < mhs.lastVotedView {
		return fmt.Errorf("target view is lower than the last voted view")
	}
	mhs.lastVotedView = targetView
	return nil
}

func (mhs *MHotStuff) updateLastVotedSeq(targetSeq types.Seq) error {
	if targetSeq < mhs.lastVotedSeq {
		return fmt.Errorf("target seq is lower than last voted view")
	}
	mhs.lastVotedSeq = targetSeq
	return nil
}

func (mhs *MHotStuff) updatePreferredView(qc *blockchain.QC) error {
	// update 2-chain block if there are two adjacent block, such that: b0 <- b1
	if qc.View <= 4 {
		return nil
	}
	b1, err := mhs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}

	grandParentBlock, err := mhs.bc.GetBlockByID(b1.QC.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	if grandParentBlock.View > mhs.preferredView {
		mhs.preferredView = grandParentBlock.View
	}
	return nil
}

func (mhs *MHotStuff) updatePreferredSeq(qc *blockchain.QC) error {
	// update 2-chain block if there are two adjacent block, such that: b0 <- b1
	if qc.Seq <= 4 {
		return nil
	}
	b1, err := mhs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred seq: %w", err)
	}

	grandParentBlock, err := mhs.bc.GetBlockByID(b1.QC.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred seq: %w", err)
	}
	if grandParentBlock.Seq > mhs.preferredSeq {
		mhs.preferredSeq = grandParentBlock.Seq
	}
	return nil
}
