package replica

import (
	"encoding/gob"
	"fmt"
	"time"

	"go.uber.org/atomic"

	"github.com/tncheng/multipipelinehs/blockchain"
	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/election"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/log"
	"github.com/tncheng/multipipelinehs/mempool"
	"github.com/tncheng/multipipelinehs/message"
	"github.com/tncheng/multipipelinehs/mhotstuff"
	"github.com/tncheng/multipipelinehs/mthotstuff"
	"github.com/tncheng/multipipelinehs/node"
	"github.com/tncheng/multipipelinehs/pacemaker"
	"github.com/tncheng/multipipelinehs/types"
)

type Replica struct {
	node.Node
	Safety
	election.Election
	pd              *mempool.Producer
	pm              *pacemaker.Pacemaker
	start           chan bool // signal to start the node
	isStarted       atomic.Bool
	isByz           bool
	timer           *time.Timer // timeout for each view
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	eventChan       chan interface{}
	dualEventChan   chan interface{}
	newViewChan     chan types.NewViewType
	// pause           chan struct{} // pase main go routine

	/* for monitoring node statistics */
	thrus                string
	lastViewTime         time.Time
	startTime            time.Time
	tmpTime              time.Time
	voteStart            time.Time
	totalCreateDuration  time.Duration
	totalProcessDuration time.Duration
	totalProposeDuration time.Duration
	totalDelay           time.Duration
	totalRoundTime       time.Duration
	totalVoteTime        time.Duration
	totalBlockSize       int
	receivedNo           int
	roundNo              int
	voteNo               int
	totalCommittedTx     int
	latencyNo            int
	proposedNo           int
	processedNo          int
	committedNo          int
}

// NewReplica creates a new replica instance
func NewReplica(id identity.NodeID, alg string, isByz bool, txInterval int) *Replica {
	r := new(Replica)
	r.Node = node.NewNode(id, isByz, txInterval)
	if isByz {
		log.Infof("[%v] is Byzantine", r.ID())
	}
	if config.GetConfig().Master == "0" {
		r.Election = election.NewRotation(config.GetConfig().N())
	} else {
		r.Election = election.NewStatic(config.GetConfig().Master)
	}
	r.isByz = isByz
	r.pd = mempool.NewProducer()
	r.pm = pacemaker.NewPacemaker(config.GetConfig().N())
	r.start = make(chan bool)
	r.eventChan = make(chan interface{}, 4)
	r.dualEventChan = make(chan interface{}, 4)
	r.newViewChan = make(chan types.NewViewType)
	// r.pause = make(chan struct{})
	r.committedBlocks = make(chan *blockchain.Block, 100)
	r.forkedBlocks = make(chan *blockchain.Block, 100)
	r.Register(blockchain.Block{}, r.HandleBlock)
	r.Register(blockchain.Vote{}, r.HandleVote)
	r.Register(blockchain.TMO{}, r.HandleTmo)
	r.Register(message.Transaction{}, r.handleTxn)
	r.Register(message.Query{}, r.handleQuery)
	gob.Register(blockchain.Block{})
	gob.Register(blockchain.Vote{})
	gob.Register(blockchain.TC{})
	gob.Register(blockchain.TMO{})

	// Is there a better way to reduce the number of parameters?
	switch alg {
	case "mhotstuff":
		r.Safety = mhotstuff.NewMHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks, r.eventChan, r.dualEventChan)
	case "mthotstuff":
		log.Infof("[%v] activating 2-chain multipipeline hotstuff", r.ID())
		r.Safety = mthotstuff.NewMTchs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks, r.eventChan, r.dualEventChan)
	default:
		r.Safety = mhotstuff.NewMHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks, r.eventChan, r.dualEventChan)
	}
	return r
}

/* Message Handlers */

func (r *Replica) HandleBlock(block blockchain.Block) {
	r.receivedNo++
	r.startSignal()
	log.Debugf("[%v] received a block from %v, view is %v, id: %x, prevID: %x", r.ID(), block.Proposer, block.View, block.ID, block.PrevID)
	r.eventChan <- block
}

func (r *Replica) HandleVote(vote blockchain.Vote) {
	// view=vote.seq=vote.view+1
	if types.View(vote.Seq) < r.pm.GetCurView() {
		return
	}
	r.startSignal()
	log.Debugf("[%v] received a vote from %v, blockID is %x", r.ID(), vote.Voter, vote.BlockID)
	r.dualEventChan <- vote
}

func (r *Replica) HandleTmo(tmo blockchain.TMO) {
	if tmo.View < r.pm.GetCurView() {
		return
	}
	log.Debugf("[%v] received a timeout from %v for view %v", r.ID(), tmo.NodeID, tmo.View)
	r.eventChan <- tmo
}

// handleQuery replies a query with the statistics of the node
func (r *Replica) handleQuery(m message.Query) {
	//realAveProposeTime := float64(r.totalProposeDuration.Milliseconds()) / float64(r.processedNo)
	//aveProcessTime := float64(r.totalProcessDuration.Milliseconds()) / float64(r.processedNo)
	//aveVoteProcessTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.roundNo)
	//aveBlockSize := float64(r.totalBlockSize) / float64(r.proposedNo)
	//requestRate := float64(r.pd.TotalReceivedTxNo()) / time.Now().Sub(r.startTime).Seconds()
	//committedRate := float64(r.committedNo) / time.Now().Sub(r.startTime).Seconds()
	//aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	//aveProposeTime := aveRoundTime - aveProcessTime - aveVoteProcessTime
	latency := float64(r.totalDelay.Milliseconds()) / float64(r.latencyNo)
	r.thrus += fmt.Sprintf("Time: %v s. Throughput: %v txs/s\n", time.Since(r.startTime).Seconds(), float64(r.totalCommittedTx)/time.Since(r.tmpTime).Seconds())
	r.totalCommittedTx = 0
	r.tmpTime = time.Now()
	status := fmt.Sprintf("Latency: %v\n%s", latency, r.thrus)
	//status := fmt.Sprintf("chain status is: %s\nCommitted rate is %v.\nAve. block size is %v.\nAve. trans. delay is %v ms.\nAve. creation time is %f ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nRequest rate is %f txs/s.\nAve. round time is %f ms.\nLatency is %f ms.\nThroughput is %f txs/s.\n", r.Safety.GetChainStatus(), committedRate, aveBlockSize, aveTransDelay, aveCreateDuration, aveProcessTime, aveVoteProcessTime, requestRate, aveRoundTime, latency, throughput)
	//status := fmt.Sprintf("Ave. actual proposing time is %v ms.\nAve. proposing time is %v ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nAve. block size is %v.\nAve. round time is %v ms.\nLatency is %v ms.\n", realAveProposeTime, aveProposeTime, aveProcessTime, aveVoteProcessTime, aveBlockSize, aveRoundTime, latency)
	m.Reply(message.QueryReply{Info: status})
}

func (r *Replica) handleTxn(m message.Transaction) {
	r.pd.AddTxn(&m)
	r.startSignal()
	// the first leader kicks off the protocol
	if r.pm.GetCurView() == 0 && r.IsLeader(r.ID(), 1) {
		log.Debugf("[%v] is going to kick off the protocol", r.ID())
		r.pm.AdvanceView(0, 0, types.NoTimeout)
		r.dualEventChan <- blockchain.Vote{View: 0, Voter: r.ID()}
	}
}

/* Processors */

func (r *Replica) processCommittedBlock(block *blockchain.Block) {
	var latencys time.Duration
	// if block.Proposer == r.ID() {
	for _, txn := range block.Payload {
		// only record the delay of transactions from the local memory pool
		delay := time.Since(txn.Timestamp)
		latencys += delay
		r.totalDelay += delay
		r.latencyNo++
	}

	// }
	r.committedNo++
	r.totalCommittedTx += len(block.Payload)

	log.Infof("[%v] the block is committed, No. of transactions: %v, view: %v, seq: %v, tx average latency: %.5f ms,current view: %v, id: %x", r.ID(), len(block.Payload), block.View, block.Seq, float64(latencys.Milliseconds())/float64(len(block.Payload)), r.pm.GetCurView(), block.ID)
}

func (r *Replica) processForkedBlock(block *blockchain.Block) {
	if block.Proposer == r.ID() {
		for _, txn := range block.Payload {
			// collect txn back to mem pool
			r.pd.CollectTxn(txn)
		}
	}
	log.Infof("[%v] the block is forked, No. of transactions: %v, view: %v, seq: %v, current view: %v, id: %x", r.ID(), len(block.Payload), block.View, block.Seq, r.pm.GetCurView(), block.ID)
}

func (r *Replica) processNewView(newView types.NewViewType) {
	log.Debugf("[%v] is processing new view: %v, leader is %v", r.ID(), newView, r.FindLeaderFor(newView.View))
	if !r.IsLeader(r.ID(), newView.View) {
		return
	}
	r.proposeBlock(newView)
}

func (r *Replica) proposeBlock(view types.NewViewType) {
	createStart := time.Now()
	block := r.Safety.MakeProposal(view, r.pd.GeneratePayload())
	r.totalBlockSize += len(block.Payload)
	r.proposedNo++
	createEnd := time.Now()
	createDuration := createEnd.Sub(createStart)
	block.Timestamp = time.Now()
	r.totalCreateDuration += createDuration
	//TODO:check the order of self propose and broadcast
	r.eventChan <- *block
	r.Broadcast(block)
	// r.Safety.ProcessBlock(block)
	r.voteStart = time.Now()
}

// ListenLocalEvent listens new view and timeout events
func (r *Replica) ListenLocalEvent() {
	r.lastViewTime = time.Now()
	r.timer = time.NewTimer(r.pm.GetTimerForView())
	for {
		r.timer.Reset(r.pm.GetTimerForView())
	L:
		for {
			select {
			case viewType := <-r.pm.EnteringViewEvent():
				if viewType.View >= 2 {
					r.totalVoteTime += time.Since(r.voteStart)
				}
				// measure round time
				now := time.Now()
				lasts := now.Sub(r.lastViewTime)
				r.totalRoundTime += lasts
				r.roundNo++
				r.lastViewTime = now
				r.newViewChan <- viewType
				log.Debugf("[%v] the last view lasts %v milliseconds, current view: %v", r.ID(), lasts.Milliseconds(), viewType.View)
				break L
			case <-r.timer.C:
				r.Safety.ProcessLocalTmo(r.pm.GetCurView(), r.pm.GetCurSeq())
				break L
			}
		}
	}
}

// ListenCommittedBlocks listens committed blocks and forked blocks from the protocols
func (r *Replica) ListenCommittedBlocks() {
	for {
		select {
		case committedBlock := <-r.committedBlocks:
			r.processCommittedBlock(committedBlock)
		case forkedBlock := <-r.forkedBlocks:
			r.processForkedBlock(forkedBlock)
		}
	}
}

func (r *Replica) startSignal() {
	if !r.isStarted.Load() {
		r.startTime = time.Now()
		r.tmpTime = time.Now()
		log.Debugf("[%v] is boosting", r.ID())
		r.isStarted.Store(true)
		r.start <- true
	}
}

// Start starts event loop
func (r *Replica) Start() {
	go r.Run()
	// wait for the start signal
	<-r.start
	go r.ListenLocalEvent()
	go r.ListenCommittedBlocks()

	go func() {
		for {
			event := <-r.dualEventChan
			switch v := event.(type) {
			case blockchain.Vote:
				startProcessTime := time.Now()
				r.Safety.ProcessVote(&v)
				processingDuration := time.Since(startProcessTime)
				r.totalVoteTime += processingDuration
				r.voteNo++

			}
		}
	}()

	go func() {
		for {
			newView := <-r.newViewChan
			r.processNewView(newView)
		}
	}()

	for r.isStarted.Load() {
		event := <-r.eventChan
		switch v := event.(type) {
		// case types.View:
		// 	r.processNewView(v)
		case blockchain.Block:
			// if r.IsLeader(r.ID(), r.pm.GetCurView()) {
			// 	<-r.pause
			// }

			startProcessTime := time.Now()
			r.totalProposeDuration += startProcessTime.Sub(v.Timestamp)
			_ = r.Safety.ProcessBlock(&v)
			r.totalProcessDuration += time.Since(startProcessTime)
			r.voteStart = time.Now()
			r.processedNo++

		// case blockchain.Vote:
		// 	startProcessTime := time.Now()
		// 	r.Safety.ProcessVote(&v)
		// 	processingDuration := time.Now().Sub(startProcessTime)
		// 	r.totalVoteTime += processingDuration
		// 	r.voteNo++
		case blockchain.TMO:
			r.Safety.ProcessRemoteTmo(&v)
		}
	}
}
