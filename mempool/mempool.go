package mempool

import (
	"time"

	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/message"
)

type MemPool struct {
	*Backend
}

// NewTransactions creates a new memory pool for transactions.
func NewMemPool() *MemPool {
	mp := &MemPool{
		Backend: NewBackend(config.GetConfig().MemSize, config.GetConfig().Duplicate),
	}

	return mp
}

func (mp *MemPool) addNew(tx *message.Transaction) {
	tx.Timestamp = time.Now()
	mp.Backend.insertBack(tx)
	mp.Backend.addToBloom(tx.ID)
}

func (mp *MemPool) addOld(tx *message.Transaction) {
	mp.Backend.insertFront(tx)
}
