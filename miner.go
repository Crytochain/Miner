package miner
import (
	"fmt"
	"math/big"
	"sync/atomic"
	"time"
	"github.com/Cryptochain-VON/common"
	"github.com/Cryptochain-VON/common/hexutil"
	"github.com/Cryptochain-VON/consensus"
	"github.com/Cryptochain-VON/core"
	"github.com/Cryptochain-VON/core/state"
	"github.com/Cryptochain-VON/core/types"
	"github.com/Cryptochain-VON/eth/downloader"
	"github.com/Cryptochain-VON/event"
	"github.com/Cryptochain-VON/log"
	"github.com/Cryptochain-VON/params"
)
type Backend interface {
	BlockChain() *core.BlockChain
	TxPool() *core.TxPool
}
type Config struct {
	Etherbase common.Address `toml:",omitempty"` 
	Notify    []string       `toml:",omitempty"` 
	ExtraData hexutil.Bytes  `toml:",omitempty"` 
	GasFloor  uint64         
	GasCeil   uint64         
	GasPrice  *big.Int       
	Recommit  time.Duration  
	Noverify  bool           
}
type Miner struct {
	mux      *event.TypeMux
	worker   *worker
	coinbase common.Address
	eth      Backend
	engine   consensus.Engine
	exitCh   chan struct{}
	canStart    int32 
	shouldStart int32 
}
func New(eth Backend, config *Config, chainConfig *params.ChainConfig, mux *event.TypeMux, engine consensus.Engine, isLocalBlock func(block *types.Block) bool) *Miner {
	miner := &Miner{
		eth:      eth,
		mux:      mux,
		engine:   engine,
		exitCh:   make(chan struct{}),
		worker:   newWorker(config, chainConfig, engine, eth, mux, isLocalBlock, true),
		canStart: 1,
	}
	go miner.update()
	return miner
}
func (miner *Miner) update() {
	events := miner.mux.Subscribe(downloader.StartEvent{}, downloader.DoneEvent{}, downloader.FailedEvent{})
	defer events.Unsubscribe()
	for {
		select {
		case ev := <-events.Chan():
			if ev == nil {
				return
			}
			switch ev.Data.(type) {
			case downloader.StartEvent:
				atomic.StoreInt32(&miner.canStart, 0)
				if miner.Mining() {
					miner.Stop()
					atomic.StoreInt32(&miner.shouldStart, 1)
					log.Info("Mining aborted due to sync")
				}
			case downloader.DoneEvent, downloader.FailedEvent:
				shouldStart := atomic.LoadInt32(&miner.shouldStart) == 1
				atomic.StoreInt32(&miner.canStart, 1)
				atomic.StoreInt32(&miner.shouldStart, 0)
				if shouldStart {
					miner.Start(miner.coinbase)
				}
				return
			}
		case <-miner.exitCh:
			return
		}
	}
}
func (miner *Miner) Start(coinbase common.Address) {
	atomic.StoreInt32(&miner.shouldStart, 1)
	miner.SetEtherbase(coinbase)
	if atomic.LoadInt32(&miner.canStart) == 0 {
		log.Info("Network syncing, will start miner afterwards")
		return
	}
	miner.worker.start()
}
func (miner *Miner) Stop() {
	miner.worker.stop()
	atomic.StoreInt32(&miner.shouldStart, 0)
}
func (miner *Miner) Close() {
	miner.worker.close()
	close(miner.exitCh)
}
func (miner *Miner) Mining() bool {
	return miner.worker.isRunning()
}
func (miner *Miner) HashRate() uint64 {
	if pow, ok := miner.engine.(consensus.PoW); ok {
		return uint64(pow.Hashrate())
	}
	return 0
}
func (miner *Miner) SetExtra(extra []byte) error {
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra exceeds max length. %d > %v", len(extra), params.MaximumExtraDataSize)
	}
	miner.worker.setExtra(extra)
	return nil
}
func (miner *Miner) SetRecommitInterval(interval time.Duration) {
	miner.worker.setRecommitInterval(interval)
}
func (miner *Miner) Pending() (*types.Block, *state.StateDB) {
	return miner.worker.pending()
}
func (miner *Miner) PendingBlock() *types.Block {
	return miner.worker.pendingBlock()
}
func (miner *Miner) SetEtherbase(addr common.Address) {
	miner.coinbase = addr
	miner.worker.setEtherbase(addr)
}
func (miner *Miner) EnablePreseal() {
	miner.worker.enablePreseal()
}
func (miner *Miner) DisablePreseal() {
	miner.worker.disablePreseal()
}
func (miner *Miner) SubscribePendingLogs(ch chan<- []*types.Log) event.Subscription {
	return miner.worker.pendingLogsFeed.Subscribe(ch)
}
