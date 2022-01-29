package pacemaker

import (
	"sync"

	"github.com/tncheng/multipipelinehs/blockchain"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/types"
)

type TimeoutController struct {
	n        int                                                // the size of the network
	timeouts map[types.View]map[identity.NodeID]*blockchain.TMO // keeps track of timeout msgs
	mu       sync.Mutex
}

func NewTimeoutController(n int) *TimeoutController {
	tcl := new(TimeoutController)
	tcl.n = n
	tcl.timeouts = make(map[types.View]map[identity.NodeID]*blockchain.TMO)
	return tcl
}

func (tcl *TimeoutController) AddTmo(tmo *blockchain.TMO) (bool, *blockchain.TC, *blockchain.TC) {
	tcl.mu.Lock()
	defer tcl.mu.Unlock()
	// note that the tmo.SSeq=tmo.view, because thera are two block timeout
	if tcl.superMajority(tmo.View) {
		return false, nil, nil
	}
	_, exist := tcl.timeouts[tmo.View]
	if !exist {
		//	first time of receiving the timeout for this view
		tcl.timeouts[tmo.View] = make(map[identity.NodeID]*blockchain.TMO)
	}
	tcl.timeouts[tmo.View][tmo.NodeID] = tmo
	if tcl.superMajority(tmo.View) {
		return true, NewTC(types.View(tmo.FSeq), tmo.FSeq, tcl.timeouts[tmo.View]), NewTC(types.View(tmo.SSeq), tmo.SSeq, tcl.timeouts[tmo.View])
	}

	return false, nil, nil
}

func (tcl *TimeoutController) superMajority(view types.View) bool {
	return tcl.total(view) > tcl.n*2/3
}

func (tcl *TimeoutController) total(view types.View) int {
	return len(tcl.timeouts[view])
}
