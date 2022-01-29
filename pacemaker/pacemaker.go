package pacemaker

import (
	"sync"
	"time"

	"github.com/tncheng/multipipelinehs/blockchain"
	"github.com/tncheng/multipipelinehs/config"

	"github.com/tncheng/multipipelinehs/types"
)

type Pacemaker struct {
	curView           types.View
	curSeq            types.Seq
	newViewChan       chan types.NewViewType
	timeoutController *TimeoutController
	mu                sync.Mutex
}

func NewPacemaker(n int) *Pacemaker {
	pm := new(Pacemaker)
	pm.newViewChan = make(chan types.NewViewType, 100)
	pm.timeoutController = NewTimeoutController(n)
	return pm
}

func (p *Pacemaker) ProcessRemoteTmo(tmo *blockchain.TMO) (bool, *blockchain.TC, *blockchain.TC) {
	if tmo.View < p.curView {
		return false, nil, nil
	}
	return p.timeoutController.AddTmo(tmo)
}

// flag: 1 is normal advance view; 0 is timeout advance view
func (p *Pacemaker) AdvanceView(view types.View, seq types.Seq, pt int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if view < p.curView {
		return
	}
	p.curView = view + 1
	p.curSeq = seq + 2
	p.newViewChan <- types.NewViewType{View: view + 1, Seq: seq + 2, ProposeType: pt} // reset timer for the next view
}

func (p *Pacemaker) EnteringViewEvent() chan types.NewViewType {
	return p.newViewChan
}

func (p *Pacemaker) GetCurView() types.View {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.curView
}
func (p *Pacemaker) GetCurSeq() types.Seq {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.curSeq
}

func (p *Pacemaker) GetTimerForView() time.Duration {
	return time.Duration(config.GetConfig().Timeout) * time.Millisecond
}
