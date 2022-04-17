package socket

import (
	"sync"
	"time"

	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/log"
	"github.com/tncheng/multipipelinehs/transport"
	"github.com/tncheng/multipipelinehs/utils"
)

type TxShare interface {
	BroadcastTX(interface{})
	RecvTX() interface{}
}

type txShare struct {
	id        identity.NodeID
	addresses map[identity.NodeID]string
	nodes     map[identity.NodeID]transport.Transport
	lock      sync.RWMutex
}

func NewTxShare(id identity.NodeID, addrs map[identity.NodeID]string) TxShare {
	txshare := &txShare{
		id:        id,
		addresses: addrs,
		nodes:     make(map[identity.NodeID]transport.Transport),
	}
	txshare.nodes[id] = transport.NewTransport(addrs[id])
	txshare.nodes[id].Listen()
	return txshare
}

func (ts *txShare) send(to identity.NodeID, m interface{}) {
	ts.lock.RLock()
	t, exist := ts.nodes[to]
	ts.lock.RUnlock()
	if !exist {
		ts.lock.RLock()
		address, ok := ts.addresses[to]
		ts.lock.RUnlock()
		if !ok {
			log.Errorf("socket does not have address of node %s", to)
			return
		}
		t = transport.NewTransport(address)
		err := utils.Retry(t.Dial, 100, time.Duration(50)*time.Millisecond)
		if err != nil {
			panic(err)
		}
		ts.lock.Lock()
		ts.nodes[to] = t
		ts.lock.Unlock()
	}
	t.Send(m)
}

func (ts *txShare) BroadcastTX(m interface{}) {
	for id := range ts.addresses {
		if id == ts.id {
			continue
		}
		ts.send(id, m)
	}
}

func (ts *txShare) RecvTX() interface{} {
	ts.lock.RLock()
	t := ts.nodes[ts.id]
	ts.lock.RUnlock()
	for {
		m := t.Recv()
		return m
	}
}
