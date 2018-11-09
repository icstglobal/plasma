package plasma

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"sync"

	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/event"
	"github.com/icstglobal/plasma/consensus"
	"github.com/icstglobal/plasma/core"
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/store"

	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	// SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

type Worker interface {
	Start()
	ProcessRemoteTxs(txs types.Transactions)
	ProcessRemoteBlock(block *types.Block)
	// SubscribeNewTxsCh(ch chan types.Transactions)
	SubscribeNewBlockCh(ch chan *types.Block)
	WriteBlock(block *types.Block) error
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
	chainDb store.Database // Block chain database

	eventMux *event.TypeMux
	engine   consensus.Engine

	// bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	// bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	// APIBackend *EthAPIBackend

	// miner     *miner.Miner
	worker   Worker // operator or validator
	operbase common.Address

	networkID uint64
	// netRPCService *ethapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and operbase)

	// chain
	rootchain *core.RootChain
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
	us := core.NewUTXOSet(chainDb)
	chainConfig, _, genesisErr := core.SetupGenesisBlock(chainDb, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	// log.Info("Initialised chain configuration", "config", chainConfig)

	pls := &Plasma{
		config:      config,
		chainDb:     chainDb,
		chainConfig: chainConfig,
		// eventMux:       ctx.EventMux,
		// engine:         CreateConsensusEngine(ctx, &config.Ethash, chainConfig, chainDb),
		shutdownChan: make(chan bool),
		networkID:    config.NetworkId,
		operbase:     config.Operbase,
		// bloomRequests:  make(chan chan *bloombits.Retrieval),
		// bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	// log.Info("Initialising Plasma protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	var (
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	pls.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, pls.chainConfig, pls.engine)
	if err != nil {
		return nil, err
	}
	signer := types.NewEIP155Signer(pls.chainConfig.ChainID)
	pls.blockchain.SetValidator(core.NewUtxoBlockValidator(signer, pls.blockchain, us))
	// Rewind the chain in case of an incompatible config upgrade.
	// if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
	// log.Warn("Rewinding chain to upgrade configuration", "err", compat)
	// pls.blockchain.SetHead(compat.RewindTo)
	// rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	// }
	// pls.bloomIndexer.Start(pls.blockchain)

	if config.TxPool.Journal != "" {
		// config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	fmt.Printf("chainConfig: %v\n", pls.chainConfig)

	log.Debug("try to init tx pool")
	txValidator := core.NewUtxoTxValidator(types.NewEIP155Signer(pls.chainConfig.ChainID), us)
	pls.txPool = core.NewTxPool(config.TxPool, pls.chainConfig, pls.blockchain, txValidator)

	// load from keystore
	keystoreDir := "./keystore"
	files, err := ioutil.ReadDir(keystoreDir)
	if err != nil {
		log.Error("keystore ioutil.ReadDir Error:", err)
	}

	var key *keystore.Key
	for _, file := range files {
		path := keystoreDir + "/" + file.Name()
		log.Debug("key path:", path, config.OperPwd)
		jsonStr, err := ioutil.ReadFile(path)
		pwd := config.OperPwd
		key, err = keystore.DecryptKey(jsonStr, pwd)

		if err != nil {
			log.Fatal(err)
		}
		break
	}
	if key == nil {
		return nil, fmt.Errorf("can not find operator key!")
	}

	//dial eth chain
	pls.rootchain, err = core.NewRootChain(config.ChainUrl, config.CxAbi, config.CxAddr, pls.chainDb)
	if err != nil {
		return nil, err
	}
	// new operator
	if config.IsOperator {
		pls.worker = core.NewOperator(pls.BlockChain(), pls.TxPool(), key.PrivateKey, us, pls.rootchain)
	} else {
		pls.worker = core.NewValidator(pls.BlockChain(), us, pls.rootchain)
	}

	pls.worker.Start()
	return pls, nil
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
func CreateDB(config *Config, name string) (store.Database, error) {
	path := ""
	if filepath.IsAbs(name) {
		path = name
	}
	path = filepath.Join(config.DataDir, name)
	log.Debugf("path, config.DataDir: %v %v\n", path, config.DataDir)

	db, err := store.NewLDBDatabase(path, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	// if db, ok := db.(*store.LDBDatabase); ok {
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

func (s *Plasma) BlockChain() *core.BlockChain { return s.blockchain }
func (s *Plasma) TxPool() *core.TxPool         { return s.txPool }

func (s *Plasma) EventMux() *event.TypeMux { return s.eventMux }
func (s *Plasma) Engine() consensus.Engine { return s.engine }
func (s *Plasma) ChainDb() store.Database  { return s.chainDb }
func (s *Plasma) IsListening() bool        { return true } // Always listening
// func (s *Plasma) EthVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Plasma) NetVersion() uint64 { return s.networkID }
func (s *Plasma) Config() *Config    { return s.config }

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

func (s *Plasma) ProcessRemoteTxs(txs types.Transactions) {
	s.worker.ProcessRemoteTxs(txs)
}

func (s *Plasma) SubscribeNewTxsCh(ch chan types.Transactions) {
	s.txPool.SubscribeNewTxsCh(ch)
}

func (s *Plasma) SubscribeNewBlockCh(ch chan *types.Block) {
	s.worker.SubscribeNewBlockCh(ch)
}

func (s *Plasma) WriteBlock(block *types.Block) error {
	return s.worker.WriteBlock(block)
}

func (s *Plasma) RootChain() *core.RootChain {
	return s.rootchain
}
