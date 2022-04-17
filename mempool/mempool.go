package mempool

import (
	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/message"
)

type MemPool struct {
	Buffer      []*message.Transaction
	bufferSize  int //number of byte of buffer
	deliverSize int
	*Backend
}

// NewTransactions creates a new memory pool for transactions.
func NewMemPool() *MemPool {

	mp := &MemPool{
		Backend:     NewBackend(config.GetConfig().MemSize),
		Buffer:      make([]*message.Transaction, 0),
		bufferSize:  0,
		deliverSize: config.GetConfig().DeliverSize,
	}

	return mp
}

func (mp *MemPool) addNew(tx *message.Transaction) (batch []*message.Transaction) {
	if !mp.Backend.insertBack(tx) {
		batch = nil
		return
	}
	if mp.bufferSize+len(tx.Command.Value) > mp.deliverSize {
		batch = mp.Buffer
		mp.Buffer = make([]*message.Transaction, 0)
		mp.bufferSize = 0
	} else {
		batch = nil
	}
	mp.Buffer = append(mp.Buffer, tx)
	mp.bufferSize += len(tx.Command.Value)
	return
}

func (mp *MemPool) addNewFromO(tx *message.Transaction) {
	mp.Backend.insertNoLimit(tx)
}

func (mp *MemPool) addOld(tx *message.Transaction) {
	mp.Backend.insertFront(tx)
}

func (mp *MemPool) remove(tx *message.Payload) {
	mp.Backend.remove(tx)
}
