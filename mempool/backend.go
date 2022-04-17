package mempool

import (
	"container/list"
	"sync"

	"github.com/tncheng/multipipelinehs/message"
)

type Backend struct {
	order         map[string]*list.Element
	txns          *list.List
	limit         int
	totalReceived int64
	*BloomFilter
	mu *sync.Mutex
}

func NewBackend(limit int) *Backend {
	var mu sync.Mutex
	return &Backend{
		order:       make(map[string]*list.Element, limit),
		txns:        list.New(),
		BloomFilter: NewBloomFilter(),
		mu:          &mu,
		limit:       limit,
	}
}

func (b *Backend) insertBack(txn *message.Transaction) bool {
	if txn == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size()+1 > b.limit {
		return false
	}
	b.totalReceived++
	e := b.txns.PushBack(txn)
	b.order[txn.ID] = e
	return true
}

func (b *Backend) insertNoLimit(tx *message.Transaction) {
	if tx == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.totalReceived++
	e := b.txns.PushBack(tx)
	b.order[tx.ID] = e
}

func (b *Backend) remove(txn *message.Payload) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.order[txn.ID]
	if !ok {
		return
	}
	b.txns.Remove(e)
	delete(b.order, txn.ID)
}

func (b *Backend) insertFront(txn *message.Transaction) {
	if txn == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.txns.PushFront(txn)
	b.order[txn.ID] = e
}

func (b *Backend) size() int {
	return b.txns.Len()
}

func (b *Backend) front() *message.Transaction {
	if b.size() == 0 {
		return nil
	}
	ele := b.txns.Front()
	val, ok := ele.Value.(*message.Transaction)
	if !ok {
		return nil
	}
	b.txns.Remove(ele)
	delete(b.order, val.ID)
	return val
}

func (b *Backend) some(n int) []*message.Payload {
	var batchSize int
	b.mu.Lock()
	defer b.mu.Unlock()
	batchSize = b.size()
	if batchSize >= n {
		batchSize = n
	}
	batch := make([]*message.Payload, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		tx := b.front()
		txp := &message.Payload{
			ID:        tx.ID,
			Digest:    make([]byte, 16),
			Timestamp: tx.Timestamp,
		}
		batch = append(batch, txp)
	}
	return batch
}

func (b *Backend) addToBloom(id string) {
	b.Add(id)
}
