package main
import (
	"crypto/ecdsa"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"time"
	"github.com/Cryptochain-VON/accounts/keystore"
	"github.com/Cryptochain-VON/common"
	"github.com/Cryptochain-VON/common/fdlimit"
	"github.com/Cryptochain-VON/consensus/ethash"
	"github.com/Cryptochain-VON/core"
	"github.com/Cryptochain-VON/core/types"
	"github.com/Cryptochain-VON/crypto"
	"github.com/Cryptochain-VON/eth"
	"github.com/Cryptochain-VON/eth/downloader"
	"github.com/Cryptochain-VON/log"
	"github.com/Cryptochain-VON/miner"
	"github.com/Cryptochain-VON/node"
	"github.com/Cryptochain-VON/p2p"
	"github.com/Cryptochain-VON/p2p/enode"
	"github.com/Cryptochain-VON/params"
)
func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(2048)
	faucets := make([]*ecdsa.PrivateKey, 128)
	for i := 0; i < len(faucets); i++ {
		faucets[i], _ = crypto.GenerateKey()
	}
	ethash.MakeDataset(1, filepath.Join(os.Getenv("HOME"), ".ethash"))
	genesis := makeGenesis(faucets)
	var (
		nodes  []*node.Node
		enodes []*enode.Node
	)
	for i := 0; i < 4; i++ {
		node, err := makeMiner(genesis)
		if err != nil {
			panic(err)
		}
		defer node.Close()
		for node.Server().NodeInfo().Ports.Listener == 0 {
			time.Sleep(250 * time.Millisecond)
		}
		for _, n := range enodes {
			node.Server().AddPeer(n)
		}
		nodes = append(nodes, node)
		enodes = append(enodes, node.Server().Self())
		store := node.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
		if _, err := store.NewAccount(""); err != nil {
			panic(err)
		}
	}
	time.Sleep(3 * time.Second)
	for _, node := range nodes {
		var ethereum *eth.Ethereum
		if err := node.Service(&ethereum); err != nil {
			panic(err)
		}
		if err := ethereum.StartMining(1); err != nil {
			panic(err)
		}
	}
	time.Sleep(3 * time.Second)
	nonces := make([]uint64, len(faucets))
	for {
		index := rand.Intn(len(faucets))
		var ethereum *eth.Ethereum
		if err := nodes[index%len(nodes)].Service(&ethereum); err != nil {
			panic(err)
		}
		tx, err := types.SignTx(types.NewTransaction(nonces[index], crypto.PubkeyToAddress(faucets[index].PublicKey), new(big.Int), 21000, big.NewInt(100000000000+rand.Int63n(65536)), nil), types.HomesteadSigner{}, faucets[index])
		if err != nil {
			panic(err)
		}
		if err := ethereum.TxPool().AddLocal(tx); err != nil {
			panic(err)
		}
		nonces[index]++
		if pend, _ := ethereum.TxPool().Stats(); pend > 2048 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
func makeGenesis(faucets []*ecdsa.PrivateKey) *core.Genesis {
	genesis := core.DefaultRopstenGenesisBlock()
	genesis.Difficulty = params.MinimumDifficulty
	genesis.GasLimit = 25000000
	genesis.Config.ChainID = big.NewInt(18)
	genesis.Config.EIP150Hash = common.Hash{}
	genesis.Alloc = core.GenesisAlloc{}
	for _, faucet := range faucets {
		genesis.Alloc[crypto.PubkeyToAddress(faucet.PublicKey)] = core.GenesisAccount{
			Balance: new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil),
		}
	}
	return genesis
}
func makeMiner(genesis *core.Genesis) (*node.Node, error) {
	datadir, _ := ioutil.TempDir("", "")
	config := &node.Config{
		Name:    "geth",
		Version: params.Version,
		DataDir: datadir,
		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    25,
		},
		NoUSB:             true,
		UseLightweightKDF: true,
	}
	stack, err := node.New(config)
	if err != nil {
		return nil, err
	}
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		return eth.New(ctx, &eth.Config{
			Genesis:         genesis,
			NetworkId:       genesis.Config.ChainID.Uint64(),
			SyncMode:        downloader.FullSync,
			DatabaseCache:   256,
			DatabaseHandles: 256,
			TxPool:          core.DefaultTxPoolConfig,
			GPO:             eth.DefaultConfig.GPO,
			Ethash:          eth.DefaultConfig.Ethash,
			Miner: miner.Config{
				GasFloor: genesis.GasLimit * 9 / 10,
				GasCeil:  genesis.GasLimit * 11 / 10,
				GasPrice: big.NewInt(1),
				Recommit: time.Second,
			},
		})
	}); err != nil {
		return nil, err
	}
	return stack, stack.Start()
}
