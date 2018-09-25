package plasma

import (
	"fmt"
	"math/big"
	"runtime"
	"sync"
	// "sync/atomic"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	// "github.com/ethereum/go-ethereum/consensus"
	"github.com/icstglobal/plasma/consensus"
	// "github.com/ethereum/go-ethereum/core"
	// "github.com/ethereum/go-ethereum/core/bloombits"
	// "github.com/ethereum/go-ethereum/core/rawdb"
	// "github.com/ethereum/go-ethereum/core/vm"
	// "github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/icstglobal/plasma/core"
	// "github.com/ethereum/go-ethereum/eth/filters"
	// "github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	// "github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	// "github.com/ethereum/go-ethereum/miner"
	// "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"path/filepath"
	// "github.com/icstglobal/plasma/network"
	// "github.com/ethereum/go-ethereum/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	// SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// Plasma implements the Ethereum full node service.
type Plasma struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the Plasma

	// Handlers
	txPool     *core.TxPool
	blockchain *core.BlockChain
	// protocolManager *ProtocolManager
	lesServer LesServer

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	// bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	// bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	// APIBackend *EthAPIBackend

	// miner     *miner.Miner
	operator *core.Operator
	gasPrice *big.Int
	operbase common.Address

	networkID uint64
	// netRPCService *ethapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and operbase)
}

func (s *Plasma) AddLesServer(ls LesServer) {
	s.lesServer = ls
	// ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new Plasma object (including the
// initialisation of the common Plasma object)
func New(config *Config) (*Plasma, error) {
	// if config.SyncMode == downloader.LightSync {
	// return nil, errors.New("can't run eth.Plasma in light sync mode, use les.LightEthereum")
	// }
	// if !config.SyncMode.IsValid() {
	// return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	// }
	chainDb, err := CreateDB(config, "chaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, _, genesisErr := core.SetupGenesisBlock(chainDb, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	// log.Info("Initialised chain configuration", "config", chainConfig)

	eth := &Plasma{
		config:      config,
		chainDb:     chainDb,
		chainConfig: chainConfig,
		// eventMux:       ctx.EventMux,
		// accountManager: ctx.AccountManager,
		// engine:         CreateConsensusEngine(ctx, &config.Ethash, chainConfig, chainDb),
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
		gasPrice:     config.GasPrice,
		operbase:     config.Operbase,
		// bloomRequests:  make(chan chan *bloombits.Retrieval),
		// bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	// log.Info("Initialising Plasma protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	var (
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, eth.chainConfig, eth.engine)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	// if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
	// log.Warn("Rewinding chain to upgrade configuration", "err", compat)
	// eth.blockchain.SetHead(compat.RewindTo)
	// rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	// }
	// eth.bloomIndexer.Start(eth.blockchain)

	if config.TxPool.Journal != "" {
		// config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	fmt.Printf("chainConfig: %v\n", eth.chainConfig)
	eth.txPool = core.NewTxPool(config.TxPool, eth.chainConfig, eth.blockchain)
	eth.operator = core.NewOperator(eth, nil)
	eth.operator.Start()

	// if eth.protocolManager, err = NewProtocolManager(eth.chainConfig, config.SyncMode, config.NetworkId, eth.eventMux, eth.txPool, eth.engine, eth.blockchain, chainDb); err != nil {
	// return nil, err
	// }
	// eth.miner = miner.New(eth, eth.chainConfig, eth.EventMux(), eth.engine)
	// eth.miner.SetExtra(makeExtraData(config.ExtraData))

	// eth.APIBackend = &EthAPIBackend{eth, nil}
	// gpoParams := config.GPO
	// if gpoParams.Default == nil {
	// gpoParams.Default = config.GasPrice
	// }
	// eth.APIBackend.gpo = gasprice.NewOracle(eth.APIBackend, gpoParams)

	return eth, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"geth",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(config *Config, name string) (ethdb.Database, error) {
	path := ""
	if filepath.IsAbs(name) {
		path = name
	}
	path = filepath.Join(config.DataDir, name)
	fmt.Printf("path, config.DataDir: %v %v\n", path, config.DataDir)

	db, err := ethdb.NewLDBDatabase(path, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	// if db, ok := db.(*ethdb.LDBDatabase); ok {
	// db.Meter("eth/db/chaindata/")
	// }
	return db, nil
}

// APIs return the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
// func (s *Plasma) APIs() []rpc.API {
// }

func (s *Plasma) Operbase() common.Address {
	s.lock.RLock()
	operbase := s.operbase
	s.lock.RUnlock()

	if operbase != (common.Address{}) {
		return operbase
	}

	panic("don't have operbase!")
	return common.Address{}
}

// SetEtherbase sets the mining reward address.
func (s *Plasma) SetEtherbase(operbase common.Address) {
	s.lock.Lock()
	s.operbase = operbase
	s.lock.Unlock()

	s.operator.SetOperbase(operbase)
}

func (s *Plasma) StartMining(local bool) error {
	eb := s.Operbase()
	nullAddr := common.Address{}
	if eb == nullAddr {
		log.Error("Cannot start mining without operbase")
		return fmt.Errorf("operbase missing")
	}
	// if local {
	// If local (CPU) mining is started, we can disable the transaction rejection
	// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
	// so none will ever hit this path, whereas marking sync done on CPU mining
	// will ensure that private networks work in single miner mode too.
	// atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	// }
	// go s.miner.Start(eb)
	return nil
}

// func (s *Plasma) StopMining()         { s.miner.Stop() }
// func (s *Plasma) IsMining() bool { return s.miner.Mining() }

// func (s *Plasma) Miner() *miner.Miner { return s.miner }

func (s *Plasma) AccountManager() *accounts.Manager { return s.accountManager }
func (s *Plasma) BlockChain() *core.BlockChain      { return s.blockchain }
func (s *Plasma) TxPool() *core.TxPool              { return s.txPool }
func (s *Plasma) EventMux() *event.TypeMux          { return s.eventMux }
func (s *Plasma) Engine() consensus.Engine          { return s.engine }
func (s *Plasma) ChainDb() ethdb.Database           { return s.chainDb }
func (s *Plasma) IsListening() bool                 { return true } // Always listening
// func (s *Plasma) EthVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Plasma) NetVersion() uint64 { return s.networkID }

// func (s *Plasma) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Plasma) Protocols() []p2p.Protocol {
	return nil
	// if s.lesServer == nil {
	// return s.protocolManager.SubProtocols
	// }
	// return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Plasma protocol implementation.
func (s *Plasma) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	// s.startBloomHandlers()

	// Start the RPC service
	// s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	// s.protocolManager.Start(maxPeers)
	// if s.lesServer != nil {
	// s.lesServer.Start(srvr)
	// }
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Plasma protocol.
func (s *Plasma) Stop() error {
	// s.bloomIndexer.Close()
	s.blockchain.Stop()
	// s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	// s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
