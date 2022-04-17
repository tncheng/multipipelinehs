package mempool

import (
	"strconv"
	"time"

	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/db"
	"github.com/tncheng/multipipelinehs/identity"
	"github.com/tncheng/multipipelinehs/log"
	"github.com/tncheng/multipipelinehs/message"
	"github.com/tncheng/multipipelinehs/socket"
)

type Producer struct {
	id             identity.NodeID
	mempool        *MemPool
	txShare        socket.TxShare
	rate           int
	virtualC       int
	payloadSize    int
	activateSignal chan struct{}
}

func NewProducer(id identity.NodeID, signal chan struct{}) *Producer {
	return &Producer{
		id:             id,
		mempool:        NewMemPool(),
		txShare:        socket.NewTxShare(id, config.GetConfig().MpAddrs),
		rate:           config.GetConfig().Rate,
		virtualC:       config.GetConfig().VirtualC,
		payloadSize:    config.GetConfig().PayloadSize,
		activateSignal: signal,
	}
}

func (pd *Producer) Run() {
	go pd.simulateTx()
	go pd.recv()
}

func (pd *Producer) recv() {
	for {
		m := pd.txShare.RecvTX()
		switch txs := m.(type) {
		case []*message.Transaction:
			for _, tx := range txs {
				pd.processTxsByO(tx)
			}
		}
	}
}

func (pd *Producer) simulateTx() {
	time.Sleep(10 * time.Second)
	base := string(pd.id)
	count := 0
	interval := time.Duration((1000000 / pd.rate) * int(time.Microsecond))
	close(pd.activateSignal)
	log.Debugf("%v close channel to activate ", pd.id)
	for {
		time.Sleep(interval)
		for i := 0; i < pd.virtualC; i++ {
			start := time.Now()
			tx := message.Transaction{Command: db.Command{Key: db.Key(count), Value: make(db.Value, pd.payloadSize)}, Timestamp: start, ID: base + strconv.Itoa(count)}
			count++
			pd.AddTxn(&tx)
		}
	}
}

func (pd *Producer) GeneratePayload() []*message.Payload {
	return pd.mempool.some(config.Configuration.BSize)
}

func (pd *Producer) AddTxn(txn *message.Transaction) {
	batch := pd.mempool.addNew(txn)
	if batch != nil {
		pd.txShare.BroadcastTX(batch)
	}
}

func (pd *Producer) processTxsByO(tx *message.Transaction) {
	pd.mempool.addNewFromO(tx)
}

func (pd *Producer) CleanTxn(txn *message.Payload) {
	pd.mempool.remove(txn)
}

func (pd *Producer) CollectTxn(txn *message.Transaction) {
	pd.mempool.addOld(txn)
}

func (pd *Producer) TotalReceivedTxNo() int64 {
	return pd.mempool.totalReceived
}
